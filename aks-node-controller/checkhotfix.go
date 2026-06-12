package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	yaml "gopkg.in/yaml.v3"
)

// check-hotfix reads the live-patching-controller's hotfix pointer from the
// kube-system/anc-hotfix-version ConfigMap and writes it to the same path
// download-hotfix already reads. download-hotfix then re-resolves the pointer
// against the node's baked ANC version and keeps its unchanged patch-only,
// strictly-higher gating. check-hotfix only fetches and stages the pointer; it
// never installs anything and never blocks provisioning (fail-open).
const (
	// hotfixConfigMapNamespace and hotfixConfigMapName identify the ConfigMap the
	// live-patching-controller publishes the base->hotfix pointer map into.
	hotfixConfigMapNamespace = "kube-system"
	hotfixConfigMapName      = "anc-hotfix-version"

	// hotfixConfigMapDataKey is the documented .data key holding the full
	// {"hotfixes":{...}} JSON object. When absent we fall back to the single/only
	// entry in .data (see parseConfigMapHotfixConfig). Per-base keys are not used.
	hotfixConfigMapDataKey = "hotfixes.json"

	// kubeCACertPath is the cluster CA written by cse_config.sh; we trust it for the
	// apiserver TLS handshake.
	kubeCACertPath = "/etc/kubernetes/certs/ca.crt"
	// bootstrapKubeconfigPath and kubeconfigPath are the on-node kubeconfigs written
	// by cse_config.sh configureKubeletAndKubectl; used only by the secondary fallback.
	bootstrapKubeconfigPath = "/var/lib/kubelet/bootstrap-kubeconfig"
	kubeconfigPath          = "/var/lib/kubelet/kubeconfig"

	// apiServerHTTPS is the standard apiserver port used for the FQDN derived from the AKSNodeConfig.
	apiServerHTTPSPort = "443"

	// configMapFetchTimeout caps the apiserver round-trip so a hung/slow apiserver
	// never delays provisioning.
	configMapFetchTimeout = 10 * time.Second
)

// checkHotfixOutcome is the telemetry taxonomy emitted under TaskName "CheckHotfix".
type checkHotfixOutcome string

const (
	// outcomeConfigMapRead: ConfigMap fetched + parsed OK and a hotfix entry matched this node's base.
	outcomeConfigMapRead checkHotfixOutcome = "configMapRead"
	// outcomeNoHotfixForBase: ConfigMap read OK but no entry matched this node's YYYYMM.DD base.
	outcomeNoHotfixForBase checkHotfixOutcome = "noHotfixForBase"
	// outcomeCustomDataFallback: ConfigMap read failed; the embedded customdata pointer was used.
	outcomeCustomDataFallback checkHotfixOutcome = "customDataFallback"
	// outcomeFailed: everything failed; nothing was staged. Provisioning still proceeds (exit 0).
	outcomeFailed checkHotfixOutcome = "failed"
)

// k8sConfigMap is the minimal shape of a Kubernetes ConfigMap GET response. ConfigMap
// .data values are strings, so the hotfix pointer is a JSON object encoded as a string.
type k8sConfigMap struct {
	Data map[string]string `json:"data"`
}

// runCheckHotfixCommand is the cli Action for `check-hotfix`. It ALWAYS returns nil so
// provisioning is never blocked: any error (404, 403, timeout, parse failure) is logged,
// emitted as telemetry, and swallowed. Internal helpers return errors for testability only.
func (a *App) runCheckHotfixCommand(ctx context.Context) error {
	slog.Info("aks-node-controller check-hotfix started")
	startTime := time.Now()

	outcome, err := a.checkHotfix(ctx)

	endTime := time.Now()
	level := helpersEventLevel(outcome)
	message := fmt.Sprintf("check-hotfix outcome=%s", outcome)
	if err != nil {
		message = fmt.Sprintf("%s error=%s", message, err.Error())
		slog.Warn("check-hotfix completed with error (fail-open)", "outcome", outcome, "error", err)
	} else {
		slog.Info("check-hotfix completed", "outcome", outcome)
	}
	if a.eventLogger != nil {
		a.eventLogger.LogEvent("CheckHotfix", message, level, startTime, endTime)
	}

	// Fail-open: never propagate an error so the cli exit code stays 0.
	return nil
}

// checkHotfix performs the fetch/parse/stage workflow and reports a telemetry outcome.
// It is fail-open by contract: the only caller (runCheckHotfixCommand) swallows the error.
func (a *App) checkHotfix(ctx context.Context) (checkHotfixOutcome, error) {
	hotfixPath := a.hotfixVersionPath
	if hotfixPath == "" {
		hotfixPath = defaultHotfixVersionPath
	}

	data, fetchErr := a.fetchHotfixConfigMap(ctx)
	if fetchErr != nil {
		// ConfigMap read failed: fall back to the pointer embedded in the node config
		// (cold-start path). See coldStartHotfixConfig for the contract TODO.
		slog.Warn("failed to read anc-hotfix-version ConfigMap, attempting cold-start fallback",
			"error", fetchErr)
		cfg, ok, coldErr := a.coldStartHotfixConfig()
		if coldErr != nil {
			return outcomeFailed, fmt.Errorf("configmap fetch failed (%v) and cold-start fallback failed: %w", fetchErr, coldErr)
		}
		if !ok {
			return outcomeFailed, fmt.Errorf("configmap fetch failed and no cold-start pointer present: %w", fetchErr)
		}
		if err := writeHotfixConfig(hotfixPath, cfg); err != nil {
			return outcomeFailed, fmt.Errorf("writing cold-start hotfix config: %w", err)
		}
		return outcomeCustomDataFallback, nil
	}

	cfg, err := parseConfigMapHotfixConfig(data)
	if err != nil {
		return outcomeFailed, fmt.Errorf("parsing anc-hotfix-version ConfigMap: %w", err)
	}

	if err := writeHotfixConfig(hotfixPath, cfg); err != nil {
		return outcomeFailed, fmt.Errorf("writing hotfix config: %w", err)
	}

	// Report whether this node's base actually has a pointer. download-hotfix still
	// performs the authoritative patch-only-strictly-higher gating; this is telemetry only.
	if cfg.resolveVersion(Version) == "" {
		return outcomeNoHotfixForBase, nil
	}
	return outcomeConfigMapRead, nil
}

// helpersEventLevel maps a check-hotfix outcome to a guest-agent event level. Only the
// terminal "failed" outcome is reported as an error; the rest are informational because
// the command is fail-open and provisioning continues regardless.
func helpersEventLevel(outcome checkHotfixOutcome) helpers.EventLevel {
	if outcome == outcomeFailed {
		return helpers.EventLevelError
	}
	return helpers.EventLevelInformational
}

// fetchHotfixConfigMap returns the raw ConfigMap GET body. Tests inject
// checkHotfixConfigMapFetcher to supply canned bytes or errors without networking.
func (a *App) fetchHotfixConfigMap(ctx context.Context) ([]byte, error) {
	if a.checkHotfixConfigMapFetcher != nil {
		return a.checkHotfixConfigMapFetcher(ctx)
	}
	return a.fetchConfigMapFromAPIServer(ctx)
}

// apiServerCreds holds the endpoint and credentials needed to reach the apiserver.
type apiServerCreds struct {
	// server is the apiserver host[:port] without scheme.
	server string
	// token, when set, is sent as an Authorization: Bearer header.
	token string
	// caPEM is the cluster CA used to verify the apiserver certificate.
	caPEM []byte
	// clientCertPEM/clientKeyPEM, when both set, enable client-cert mTLS.
	clientCertPEM []byte
	clientKeyPEM  []byte
}

// fetchConfigMapFromAPIServer performs the real network GET against the apiserver. It
// resolves credentials (primary: AKSNodeConfig bootstrap token + FQDN; secondary: on-node
// kubeconfigs), builds a short-timeout TLS client trusting the cluster CA, and returns the
// raw response body. Non-2xx responses are surfaced as errors so the caller fails open.
func (a *App) fetchConfigMapFromAPIServer(ctx context.Context) ([]byte, error) {
	creds, err := a.resolveAPIServerCreds()
	if err != nil {
		return nil, fmt.Errorf("resolving apiserver credentials: %w", err)
	}

	client, err := buildAPIServerHTTPClient(creds)
	if err != nil {
		return nil, fmt.Errorf("building apiserver http client: %w", err)
	}

	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/configmaps/%s",
		creds.server, hotfixConfigMapNamespace, hotfixConfigMapName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if creds.token != "" {
		req.Header.Set("Authorization", "Bearer "+creds.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("apiserver returned status %d for %s", resp.StatusCode, url)
	}
	return body, nil
}

// resolveAPIServerCreds gathers apiserver endpoint + credentials.
//
// Primary (per design 2.1b): the bootstrap token + apiserver FQDN come from the
// AKSNodeConfig that ANC already parses, with the cluster CA from kubeCACertPath. This is
// the same auth pattern as the previous anc-hotfix-svc proxy, minus the proxy.
//
// Secondary (client-cert mode): parse the on-node bootstrap-kubeconfig then kubeconfig for
// server, certificate-authority, and client-certificate/client-key or token.
func (a *App) resolveAPIServerCreds() (apiServerCreds, error) {
	if creds, err := a.credsFromNodeConfig(); err == nil {
		return creds, nil
	} else {
		slog.Info("primary apiserver creds (node config) unavailable, trying kubeconfig fallback", "error", err)
	}
	return credsFromKubeconfigs(bootstrapKubeconfigPath, kubeconfigPath)
}

// credsFromNodeConfig builds creds from the AKSNodeConfig (apiserver FQDN + TLS bootstrap
// token) and the on-node cluster CA file.
func (a *App) credsFromNodeConfig() (apiServerCreds, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return apiServerCreds{}, fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, err := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if err != nil {
		// Forward-compatible parse: unknown fields are discarded, so a non-nil error here
		// means the document was unusable. Continue to evaluate what we did parse.
		slog.Info("node config parsed with errors, continuing with partial config", "error", err)
	}
	server := strings.TrimSpace(cfg.GetApiServerConfig().GetApiServerName())
	if server == "" {
		return apiServerCreds{}, fmt.Errorf("node config has no api_server_config.api_server_name")
	}
	token := strings.TrimSpace(cfg.GetBootstrappingConfig().GetTlsBootstrappingToken())
	if token == "" {
		return apiServerCreds{}, fmt.Errorf("node config has no bootstrapping_config.tls_bootstrapping_token")
	}
	caPEM, err := os.ReadFile(kubeCACertPath)
	if err != nil {
		return apiServerCreds{}, fmt.Errorf("reading cluster CA %s: %w", kubeCACertPath, err)
	}
	return apiServerCreds{
		server: ensurePort(server, apiServerHTTPSPort),
		token:  token,
		caPEM:  caPEM,
	}, nil
}

// kubeconfig is the minimal YAML shape we parse from the on-node kubeconfigs.
type kubeconfig struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthority     string `yaml:"certificate-authority"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token                 string `yaml:"token"`
			ClientCertificate     string `yaml:"client-certificate"`
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKey             string `yaml:"client-key"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// credsFromKubeconfigs tries each kubeconfig path in order and returns the first that
// yields a usable server endpoint. It supports both token and client-cert auth, and both
// file-path and inline base64 (-data) forms for CA and client credentials.
func credsFromKubeconfigs(paths ...string) (apiServerCreds, error) {
	var lastErr error
	for _, p := range paths {
		creds, err := credsFromKubeconfig(p)
		if err != nil {
			lastErr = err
			continue
		}
		return creds, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no kubeconfig paths provided")
	}
	return apiServerCreds{}, fmt.Errorf("no usable kubeconfig credentials: %w", lastErr)
}

func credsFromKubeconfig(path string) (apiServerCreds, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return apiServerCreds{}, fmt.Errorf("reading kubeconfig %s: %w", path, err)
	}
	return parseKubeconfigCreds(raw)
}

// parseKubeconfigCreds extracts apiserver creds from kubeconfig YAML. It uses the first
// cluster/user (the on-node kubeconfigs written by cse_config.sh each contain exactly one).
func parseKubeconfigCreds(raw []byte) (apiServerCreds, error) {
	var kc kubeconfig
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return apiServerCreds{}, fmt.Errorf("parsing kubeconfig YAML: %w", err)
	}
	if len(kc.Clusters) == 0 {
		return apiServerCreds{}, fmt.Errorf("kubeconfig has no clusters")
	}
	cluster := kc.Clusters[0].Cluster
	server := strings.TrimSpace(cluster.Server)
	if server == "" {
		return apiServerCreds{}, fmt.Errorf("kubeconfig cluster has no server")
	}
	creds := apiServerCreds{server: stripScheme(server)}

	if data := strings.TrimSpace(cluster.CertificateAuthorityData); data != "" {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return apiServerCreds{}, fmt.Errorf("decoding certificate-authority-data: %w", err)
		}
		creds.caPEM = decoded
	} else if file := strings.TrimSpace(cluster.CertificateAuthority); file != "" {
		pem, err := os.ReadFile(file)
		if err != nil {
			return apiServerCreds{}, fmt.Errorf("reading certificate-authority %s: %w", file, err)
		}
		creds.caPEM = pem
	}

	if len(kc.Users) > 0 {
		user := kc.Users[0].User
		creds.token = strings.TrimSpace(user.Token)

		certPEM, err := pemFromFileOrData(user.ClientCertificate, user.ClientCertificateData)
		if err != nil {
			return apiServerCreds{}, fmt.Errorf("loading client certificate: %w", err)
		}
		keyPEM, err := pemFromFileOrData(user.ClientKey, user.ClientKeyData)
		if err != nil {
			return apiServerCreds{}, fmt.Errorf("loading client key: %w", err)
		}
		creds.clientCertPEM = certPEM
		creds.clientKeyPEM = keyPEM
	}
	return creds, nil
}

// pemFromFileOrData returns PEM bytes from inline base64 data when present, else from the
// referenced file path, else nil (the field was unset).
func pemFromFileOrData(file, data string) ([]byte, error) {
	if d := strings.TrimSpace(data); d != "" {
		decoded, err := base64.StdEncoding.DecodeString(d)
		if err != nil {
			return nil, fmt.Errorf("decoding base64 data: %w", err)
		}
		return decoded, nil
	}
	if f := strings.TrimSpace(file); f != "" {
		return os.ReadFile(f)
	}
	return nil, nil
}

// buildAPIServerHTTPClient builds an *http.Client trusting the cluster CA (and presenting a
// client cert when provided), with a short timeout so provisioning is never delayed.
func buildAPIServerHTTPClient(creds apiServerCreds) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(creds.caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(creds.caPEM) {
			return nil, fmt.Errorf("failed to parse cluster CA PEM")
		}
		tlsConfig.RootCAs = pool
	}
	if len(creds.clientCertPEM) > 0 && len(creds.clientKeyPEM) > 0 {
		cert, err := tls.X509KeyPair(creds.clientCertPEM, creds.clientKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("loading client key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{
		Timeout:   configMapFetchTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

// parseConfigMapHotfixConfig extracts the hotfix pointer from a ConfigMap GET body. Q1
// decision: .data holds the full {"hotfixes":{...}} JSON object under a SINGLE key. We
// prefer the documented key name (hotfixConfigMapDataKey); if absent we use the single/only
// entry. The value unmarshals DIRECTLY into the shared 2.1a hotfixConfig, so check-hotfix
// and download-hotfix share ONE identical parser and data contract.
func parseConfigMapHotfixConfig(data []byte) (hotfixConfig, error) {
	var cm k8sConfigMap
	if err := json.Unmarshal(data, &cm); err != nil {
		return hotfixConfig{}, fmt.Errorf("unmarshaling ConfigMap: %w", err)
	}
	if len(cm.Data) == 0 {
		return hotfixConfig{}, fmt.Errorf("ConfigMap has no data")
	}

	value, ok := cm.Data[hotfixConfigMapDataKey]
	if !ok {
		if len(cm.Data) != 1 {
			return hotfixConfig{}, fmt.Errorf("ConfigMap data has no %q key and %d entries (expected exactly 1)",
				hotfixConfigMapDataKey, len(cm.Data))
		}
		for _, v := range cm.Data {
			value = v
		}
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return hotfixConfig{}, fmt.Errorf("ConfigMap hotfix entry is empty")
	}

	var cfg hotfixConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return hotfixConfig{}, fmt.Errorf("unmarshaling hotfix pointer JSON: %w", err)
	}
	return cfg, nil
}

// coldStartHotfixConfig reads a LENIENT top-level "hotfixes" object from the AKSNodeConfig
// JSON. This is the PoC cold-start fallback used only when the ConfigMap read fails.
//
// TODO(2.1b): There is no formalized AKSNodeConfig contract field for the embedded pointer
// yet — the absvc/aks-rp side that would populate a typed field is not built. Once that
// contract exists, replace this lenient top-level read with the typed field and drop the
// permissive JSON shape. Until then we read it best-effort and never fail provisioning.
func (a *App) coldStartHotfixConfig() (hotfixConfig, bool, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hotfixConfig{}, false, nil
		}
		return hotfixConfig{}, false, fmt.Errorf("reading node config %s: %w", path, err)
	}

	// Lenient parse: the AKSNodeConfig is protojson, but the cold-start pointer is an
	// out-of-contract top-level object, so parse it permissively with encoding/json.
	var lenient struct {
		Hotfixes map[string]string `json:"hotfixes"`
	}
	if err := json.Unmarshal(raw, &lenient); err != nil {
		return hotfixConfig{}, false, fmt.Errorf("parsing cold-start hotfixes from node config: %w", err)
	}
	if len(lenient.Hotfixes) == 0 {
		return hotfixConfig{}, false, nil
	}
	return hotfixConfig{Hotfixes: lenient.Hotfixes}, true, nil
}

// writeHotfixConfig writes the resolved config to the path download-hotfix reads, in the
// exact {"hotfixes":{...}} shape so download-hotfix re-resolves and applies its unchanged
// gating. The write is atomic (temp file + rename) so a concurrent reader never sees a
// partial file.
func writeHotfixConfig(path string, cfg hotfixConfig) error {
	// Only persist the map shape; the legacy Version field is intentionally omitted so the
	// on-disk contract matches what the live-patching-controller ConfigMap publishes.
	out := hotfixConfig{Hotfixes: cfg.Hotfixes}
	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling hotfix config: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".aks-node-controller-hotfix-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, path, err)
	}
	slog.Info("staged hotfix pointer for download-hotfix", "path", path)
	return nil
}

// getNodeConfigPath returns the injectable node-config path, defaulting to the standard
// AKSNodeConfig location that ANC already reads.
func (a *App) getNodeConfigPath() string {
	if a.nodeConfigPath != "" {
		return a.nodeConfigPath
	}
	return nodeconfigutils.AKSNodeConfigFilePath
}

// ensurePort appends ":<port>" to host when it has no port. IPv6 literals already in
// bracketed host:port form ("[::1]:443") are left unchanged.
func ensurePort(host, port string) string {
	host = stripScheme(strings.TrimSpace(host))
	if host == "" {
		return host
	}
	// Already has a port (account for IPv6 "[..]:p").
	if i := strings.LastIndex(host, ":"); i != -1 && !strings.Contains(host[i+1:], "]") {
		return host
	}
	return host + ":" + port
}

// stripScheme removes a leading https:// or http:// scheme from a server URL.
func stripScheme(server string) string {
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")
	return strings.TrimRight(server, "/")
}
