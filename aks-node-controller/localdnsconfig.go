package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	akslivepatchingv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	localDNSLivePatchingComponentName = "localDNS"
	defaultLocalDNSCorefilePath       = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
	localDNSHostsFilePath             = "/etc/localdns/hosts"
	localDNSAgentPoolLabel            = "kubernetes.azure.com/agentpool"
	localDNSLPSALPNProto              = "aks-live-patching"
	localDNSALPNH2Proto               = "h2"
)

type localDNSConfigFetcher func(context.Context) (string, error)

type localDNSConfigOutcome string

const (
	outcomeLocalDNSConfigApplied        localDNSConfigOutcome = "applied"
	outcomeLocalDNSConfigAlreadyCurrent localDNSConfigOutcome = "alreadyCurrent"
	outcomeLocalDNSConfigNotFound       localDNSConfigOutcome = "notFound"
	outcomeLocalDNSConfigNoCorefileData localDNSConfigOutcome = "noCorefileData"
	outcomeLocalDNSConfigFailed         localDNSConfigOutcome = "failed"
)

type localDNSConfigPayload struct {
	Corefile           string                             `json:"corefile"`
	CorefileBase64     string                             `json:"corefileBase64"`
	CorefileBase64Alt  string                             `json:"corefile_base64"`
	CoreFile           string                             `json:"coreFile"`
	LocalDNSProfile    json.RawMessage                    `json:"localDnsProfile"`
	LocalDNSProfileAlt json.RawMessage                    `json:"local_dns_profile"`
	AgentPools         map[string]localDNSAgentPoolConfig `json:"agentPools"`
	Profiles           map[string]localDNSAgentPoolConfig `json:"profiles"`
}

type localDNSAgentPoolConfig struct {
	CorefileVersion    string          `json:"corefileVersion"`
	ConfigChecksum     string          `json:"configChecksum"`
	Corefile           string          `json:"corefile"`
	CorefileBase64     string          `json:"corefileBase64"`
	CorefileBase64Alt  string          `json:"corefile_base64"`
	CoreFile           string          `json:"coreFile"`
	LocalDNSProfile    json.RawMessage `json:"localDnsProfile"`
	LocalDNSProfileAlt json.RawMessage `json:"local_dns_profile"`
}

type localDNSCorefileUpdate struct {
	corefile       string
	desiredVersion string
	hasCorefile    bool
}

func (a *App) runApplyLocalDNSConfigCommand(ctx context.Context, configPath string, outputPath string, writer io.Writer) error {
	config, err := readLocalDNSConfigInput(configPath)
	if err != nil {
		return err
	}
	outcome, err := a.applyLocalDNSConfig(ctx, config, outputPath)
	if writer != nil {
		_, _ = fmt.Fprintf(writer, "%s\n", outcome)
	}
	if err != nil {
		return err
	}
	return nil
}

func readLocalDNSConfigInput(configPath string) (string, error) {
	if configPath == "" || configPath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading localDNS config from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("reading localDNS config %s: %w", configPath, err)
	}
	return string(data), nil
}

func (a *App) applyLocalDNSConfig(ctx context.Context, config string, outputPath string) (localDNSConfigOutcome, error) {
	return a.fetchAndApplyLocalDNSConfigWithFetcher(ctx, outputPath, func(context.Context) (string, error) {
		return config, nil
	})
}

func (a *App) runFetchLocalDNSConfigCommand(ctx context.Context, outputPath string) (err error) {
	slog.Info("aks-node-controller fetch-localdns-config started", "outputPath", outputPath)
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("fetch-localdns-config panicked (fail-open)", "panic", r)
			if a.eventLogger != nil {
				a.eventLogger.LogEvent("FetchLocalDNSConfig",
					fmt.Sprintf("fetch-localdns-config outcome=%s panic=%v", outcomeLocalDNSConfigFailed, r),
					helpers.EventLevelError, startTime, time.Now())
			}
			err = nil
		}
	}()

	outcome, err := a.fetchAndApplyLocalDNSConfig(ctx, outputPath)
	level := helpers.EventLevelInformational
	if outcome == outcomeLocalDNSConfigFailed {
		level = helpers.EventLevelError
	}
	message := fmt.Sprintf("fetch-localdns-config outcome=%s", outcome)
	if err != nil {
		message = fmt.Sprintf("%s error=%s", message, err.Error())
		slog.Warn("fetch-localdns-config completed with error (fail-open)", "outcome", outcome, "error", err)
	} else {
		slog.Info("fetch-localdns-config completed", "outcome", outcome)
	}
	if a.eventLogger != nil {
		a.eventLogger.LogEvent("FetchLocalDNSConfig", message, level, startTime, time.Now())
	}
	// The bootstrap shell caller is intentionally fail-open, so it cannot use the
	// process exit status to distinguish an applied config from a fallback. Keep
	// the machine-readable outcome as the final stdout line.
	fmt.Fprintln(os.Stdout, outcome)
	return nil
}

func (a *App) fetchAndApplyLocalDNSConfig(ctx context.Context, outputPath string) (localDNSConfigOutcome, error) {
	return a.fetchAndApplyLocalDNSConfigWithFetcher(ctx, outputPath, a.fetchLocalDNSConfig)
}

func (a *App) fetchAndApplyLocalDNSConfigWithFetcher(ctx context.Context, outputPath string, fetcher localDNSConfigFetcher) (localDNSConfigOutcome, error) {
	if outputPath == "" {
		outputPath = defaultLocalDNSCorefilePath
	}
	config, err := fetcher(ctx)
	if err != nil {
		if isLPSUnavailable(err) {
			return outcomeLocalDNSConfigNotFound, nil
		}
		return outcomeLocalDNSConfigFailed, err
	}
	update, err := a.localDNSCorefileUpdateFromConfig(config)
	if err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	versionPath := localDNSCorefileVersionPath(outputPath)
	if !update.hasCorefile {
		return outcomeLocalDNSConfigNoCorefileData, nil
	}
	current, readErr := os.ReadFile(outputPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return outcomeLocalDNSConfigFailed, fmt.Errorf("reading localDNS corefile %s: %w", outputPath, readErr)
	}
	contentMatches := readErr == nil && bytes.Equal(current, []byte(update.corefile))
	if update.desiredVersion != "" {
		currentVersion, err := readLocalDNSCorefileVersion(versionPath)
		if err != nil {
			return outcomeLocalDNSConfigFailed, err
		}
		if currentVersion == update.desiredVersion && contentMatches {
			return outcomeLocalDNSConfigAlreadyCurrent, nil
		}
	} else if contentMatches {
		return outcomeLocalDNSConfigAlreadyCurrent, nil
	}
	if err := writeLocalDNSCorefile(outputPath, update.corefile); err != nil {
		return outcomeLocalDNSConfigFailed, err
	}
	if update.desiredVersion != "" {
		if err := writeLocalDNSCorefileVersion(versionPath, update.desiredVersion); err != nil {
			return outcomeLocalDNSConfigFailed, err
		}
	}
	return outcomeLocalDNSConfigApplied, nil
}

func (a *App) fetchLocalDNSConfig(ctx context.Context) (string, error) {
	if a.fetchLocalDNSConfigFn != nil {
		return a.fetchLocalDNSConfigFn(ctx)
	}
	return a.fetchLocalDNSConfigFromLPS(ctx)
}

func (a *App) fetchLocalDNSConfigFromLPS(ctx context.Context) (string, error) {
	fqdn, caPEM, err := a.lpsTargetFromNodeConfig()
	if err != nil {
		return "", fmt.Errorf("resolving LPS endpoint from node config: %w", err)
	}
	token, err := a.attestedToken(ctx)
	if err != nil {
		return "", fmt.Errorf("imds attested token: %w", err)
	}
	rootCAs, err := certPoolFromPEM(caPEM)
	if err != nil {
		return "", err
	}
	host := fqdn
	if h, _, splitErr := net.SplitHostPort(fqdn); splitErr == nil {
		host = h
	}
	target := net.JoinHostPort(host, lpsAPIServerPort)
	tlsConfig := &tls.Config{
		MinVersion:            tls.VersionTLS12,
		RootCAs:               rootCAs,
		NextProtos:            []string{localDNSLPSALPNProto, localDNSALPNH2Proto},
		InsecureSkipVerify:    true, //nolint:gosec // SNI stays on the apiserver FQDN for ALPN routing; chain and hostname are verified below.
		VerifyPeerCertificate: localDNSVerifyChainAgainstPool(rootCAs, host),
	}
	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return "", fmt.Errorf("creating LPS client: %w", err)
	}
	defer conn.Close()

	client := akslivepatchingv1.NewLivePatchingServiceClient(conn)
	rpcCtx := metadata.AppendToOutgoingContext(ctx, "authorization", token)
	resp, err := client.GetComponentConfig(rpcCtx, &akslivepatchingv1.GetComponentConfigRequest{
		ComponentName: localDNSLivePatchingComponentName,
	})
	if err != nil {
		if localDNSLPSUnavailable(status.Code(err)) {
			return "", errLPSUnavailable
		}
		return "", fmt.Errorf("get %s component config: %w", localDNSLivePatchingComponentName, err)
	}
	return resp.GetConfig(), nil
}

func localDNSLPSUnavailable(code codes.Code) bool {
	switch code {
	case codes.NotFound, codes.PermissionDenied, codes.Unauthenticated:
		return true
	default:
		return false
	}
}

func localDNSVerifyChainAgainstPool(pool *x509.CertPool, serverName string) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("server presented no certificates")
		}
		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse server certificate: %w", err)
		}
		intermediates := x509.NewCertPool()
		for _, raw := range rawCerts[1:] {
			cert, err := x509.ParseCertificate(raw)
			if err != nil {
				return fmt.Errorf("failed to parse intermediate certificate: %w", err)
			}
			intermediates.AddCert(cert)
		}
		if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, Intermediates: intermediates, DNSName: serverName}); err != nil {
			return fmt.Errorf("server certificate verification failed: %w", err)
		}
		return nil
	}
}

func certPoolFromPEM(caPEM []byte) (*x509.CertPool, error) {
	if len(caPEM) == 0 {
		return nil, fmt.Errorf("cluster CA unavailable from provision-config; refusing to fetch over unverified TLS")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse cluster CA PEM")
	}
	return pool, nil
}

func (a *App) localDNSCorefileUpdateFromConfig(config string) (localDNSCorefileUpdate, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return localDNSCorefileUpdate{}, nil
	}

	var payload localDNSConfigPayload
	if err := json.Unmarshal([]byte(config), &payload); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS LPS config: %w", err)
	}

	selected, found, err := a.selectLocalDNSAgentPoolConfig(payload)
	if err != nil || !found {
		return localDNSCorefileUpdate{}, err
	}

	update := localDNSCorefileUpdate{
		desiredVersion: firstNonEmpty(selected.CorefileVersion, selected.ConfigChecksum),
	}
	return a.localDNSCorefileUpdateFromAgentPoolConfig(selected, update)
}

func (a *App) selectLocalDNSAgentPoolConfig(payload localDNSConfigPayload) (localDNSAgentPoolConfig, bool, error) {
	selected := localDNSAgentPoolConfig{
		Corefile:           payload.Corefile,
		CorefileBase64:     payload.CorefileBase64,
		CorefileBase64Alt:  payload.CorefileBase64Alt,
		CoreFile:           payload.CoreFile,
		LocalDNSProfile:    payload.LocalDNSProfile,
		LocalDNSProfileAlt: payload.LocalDNSProfileAlt,
	}
	if len(payload.AgentPools) == 0 && len(payload.Profiles) == 0 {
		return selected, true, nil
	}

	agentPool, err := a.nodeAgentPoolName()
	if err != nil {
		return localDNSAgentPoolConfig{}, false, err
	}
	if selected, ok := payload.AgentPools[agentPool]; ok {
		return selected, true, nil
	}
	if selected, ok := payload.Profiles[agentPool]; ok {
		return selected, true, nil
	}
	return localDNSAgentPoolConfig{}, false, nil
}

func (a *App) localDNSCorefileUpdateFromAgentPoolConfig(selected localDNSAgentPoolConfig, update localDNSCorefileUpdate) (localDNSCorefileUpdate, error) {
	switch {
	case strings.TrimSpace(selected.Corefile) != "":
		update.corefile = selected.Corefile
		update.hasCorefile = true
		return update, nil
	case strings.TrimSpace(selected.CoreFile) != "":
		update.corefile = selected.CoreFile
		update.hasCorefile = true
		return update, nil
	case strings.TrimSpace(selected.CorefileBase64) != "":
		return update.withCorefileBase64(selected.CorefileBase64)
	case strings.TrimSpace(selected.CorefileBase64Alt) != "":
		return update.withCorefileBase64(selected.CorefileBase64Alt)
	}
	profileJSON := selected.LocalDNSProfile
	if len(profileJSON) == 0 {
		profileJSON = selected.LocalDNSProfileAlt
	}
	if len(profileJSON) == 0 {
		if selected.CorefileVersion != "" || selected.ConfigChecksum != "" {
			slog.Info("localDNS LPS config has only version/checksum; Corefile content is required for bootstrap mutation",
				"corefileVersion", selected.CorefileVersion, "configChecksum", selected.ConfigChecksum)
		}
		return update, nil
	}
	profile := &aksnodeconfigv1.LocalDnsProfile{}
	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOptions.Unmarshal(profileJSON, profile); err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("parsing localDNS profile: %w", err)
	}
	if !profile.GetEnableLocalDns() {
		return update, nil
	}
	nodeConfig, err := a.nodeConfigWithLocalDNSProfile(profile)
	if err != nil {
		return localDNSCorefileUpdate{}, err
	}
	includeHostsPlugin := a.includeHostsPluginForCorefile(profile)
	if includeHostsPlugin {
		if _, statErr := os.Stat(localDNSHostsFilePath); statErr != nil {
			includeHostsPlugin = false
		}
	}
	corefile, err := parser.GenerateLocalDNSCorefileFromAKSNodeConfig(nodeConfig, includeHostsPlugin)
	if err != nil {
		return localDNSCorefileUpdate{}, err
	}
	update.corefile = corefile
	update.hasCorefile = true
	return update, nil
}

func (u localDNSCorefileUpdate) withCorefileBase64(v string) (localDNSCorefileUpdate, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
	if err != nil {
		return localDNSCorefileUpdate{}, fmt.Errorf("decoding localDNS corefileBase64: %w", err)
	}
	if len(strings.TrimSpace(string(decoded))) == 0 {
		return u, nil
	}
	u.corefile = string(decoded)
	u.hasCorefile = true
	return u, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (a *App) nodeAgentPoolName() (string, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("node config not found, trying nbc-cmd.sh fallback", "path", path)
			return a.nodeAgentPoolNameFromNBCCmd()
		}
		return "", fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		return "", fmt.Errorf("node config %s could not be parsed", path)
	}
	agentPool := cfg.GetKubeletConfig().GetKubeletNodeLabels()[localDNSAgentPoolLabel]
	if agentPool == "" {
		return "", fmt.Errorf("node config has no %s kubelet node label", localDNSAgentPoolLabel)
	}
	return agentPool, nil
}

func (a *App) nodeConfigWithLocalDNSProfile(profile *aksnodeconfigv1.LocalDnsProfile) (*aksnodeconfigv1.Configuration, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("node config not found, trying nbc-cmd.sh fallback", "path", path)
			return a.nodeConfigWithLocalDNSProfileFromNBCCmd(profile)
		}
		return nil, fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		return nil, fmt.Errorf("node config %s could not be parsed", path)
	}
	cfg.LocalDnsProfile = profile
	return cfg, nil
}

func writeLocalDNSCorefile(path string, corefile string) error {
	if strings.TrimSpace(corefile) == "" {
		return fmt.Errorf("localDNS corefile is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".localdns-corefile-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.WriteString(tmp, corefile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func localDNSCorefileVersionPath(corefilePath string) string {
	return corefilePath + ".version"
}

func readLocalDNSCorefileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading localDNS corefile version %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func writeLocalDNSCorefileVersion(path string, version string) error {
	if strings.TrimSpace(version) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(version)+"\n"), 0600)
}

// nodeAgentPoolNameFromNBCCmd reads the agent pool name from the existing nbc-cmd.sh file.
// This is the fallback path when the full AKSNodeConfig JSON is not present (e.g. Staging
// Phase 2), mirroring lpsTargetFromNBCCmd. The pool name is carried in KUBELET_NODE_LABELS
// as the kubernetes.azure.com/agentpool label.
func (a *App) nodeAgentPoolNameFromNBCCmd() (string, error) {
	vars, path, err := a.nbcCmdEnvVars()
	if err != nil {
		return "", err
	}
	labels := vars["KUBELET_NODE_LABELS"]
	if labels == "" {
		return "", fmt.Errorf("nbc-cmd %s has no KUBELET_NODE_LABELS", path)
	}
	agentPool := labelValueFromCSV(labels, localDNSAgentPoolLabel)
	if agentPool == "" {
		return "", fmt.Errorf("nbc-cmd %s KUBELET_NODE_LABELS has no %s label", path, localDNSAgentPoolLabel)
	}
	slog.Info("loaded agent pool from nbc-cmd.sh fallback", "agentPool", agentPool, "path", path)
	return agentPool, nil
}

// nodeConfigWithLocalDNSProfileFromNBCCmd builds the minimal Configuration needed to render the
// LocalDNS Corefile when the full AKSNodeConfig JSON is absent (e.g. Staging Phase 2).
//
// The Corefile template reads exactly one field off Configuration outside of LocalDnsProfile:
// ClusterConfig.ClusterNetworkConfig.CoreDnsServiceIp. That value is NOT carried as a standalone
// variable in nbc-cmd.sh -- note that --cluster-dns points at the localdns cluster listener
// (169.254.10.11), not at the CoreDNS service. It is, however, already baked into the Corefile
// that bootstrap rendered into LOCALDNS_COREFILE_BASE, so recover it from there.
func (a *App) nodeConfigWithLocalDNSProfileFromNBCCmd(profile *aksnodeconfigv1.LocalDnsProfile) (*aksnodeconfigv1.Configuration, error) {
	vars, path, err := a.nbcCmdEnvVars()
	if err != nil {
		return nil, err
	}
	coreDNSServiceIP, err := coreDNSServiceIPFromBootstrapCorefiles(vars)
	if err != nil {
		return nil, fmt.Errorf("nbc-cmd %s: %w", path, err)
	}
	slog.Info("loaded CoreDNS service IP from nbc-cmd.sh fallback", "coreDnsServiceIp", coreDNSServiceIP, "path", path)
	return &aksnodeconfigv1.Configuration{
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
				CoreDnsServiceIp: coreDNSServiceIP,
			},
		},
		LocalDnsProfile: profile,
	}, nil
}

// nbcCmdEnvVars reads and parses the command-scoped environment variables out of nbc-cmd.sh.
func (a *App) nbcCmdEnvVars() (map[string]string, string, error) {
	path := a.getNBCCmdPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("reading nbc-cmd %s: %w", path, err)
	}
	return parseEnvVarsFromNBCCmdContent(string(raw)), path, nil
}

// labelValueFromCSV extracts a label value from a comma-separated "key=value" list.
// KUBELET_NODE_LABELS may repeat a key; the values agree, so the first match wins.
func labelValueFromCSV(labels string, key string) string {
	for _, pair := range strings.Split(labels, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// coreDNSServiceIPFromBootstrapCorefiles recovers the CoreDNS service IP from a Corefile that
// bootstrap already rendered. In the generated Corefile the cluster.local server block forwards
// to the CoreDNS service, so the forward target there is the value we need.
func coreDNSServiceIPFromBootstrapCorefiles(vars map[string]string) (string, error) {
	for _, name := range []string{"LOCALDNS_COREFILE_BASE", "LOCALDNS_COREFILE_WITH_HOSTS", "LOCALDNS_GENERATED_COREFILE"} {
		encoded := strings.TrimSpace(vars[name])
		if encoded == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			slog.Info("bootstrap corefile variable is not valid base64, skipping", "variable", name, "error", err)
			continue
		}
		if ip := coreDNSServiceIPFromCorefile(string(decoded)); ip != "" {
			return ip, nil
		}
	}
	return "", fmt.Errorf("no bootstrap corefile carried a cluster.local forward target")
}

// coreDNSServiceIPFromCorefile returns the forward target of the cluster.local server block.
func coreDNSServiceIPFromCorefile(corefile string) string {
	inClusterLocalBlock := false
	for _, line := range strings.Split(corefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "{") && strings.HasPrefix(trimmed, "cluster.local:") {
			inClusterLocalBlock = true
			continue
		}
		if !inClusterLocalBlock {
			continue
		}
		if fields := strings.Fields(trimmed); len(fields) >= 3 && fields[0] == "forward" && fields[1] == "." {
			if ip := net.ParseIP(fields[2]); ip != nil {
				return fields[2]
			}
			return ""
		}
		if trimmed == "}" {
			inClusterLocalBlock = false
		}
	}
	return ""
}

// includeHostsPluginForCorefile decides whether the re-rendered Corefile keeps the hosts plugin
// block.
//
// The hosts plugin is currently driven by a control-plane toggle that bootstrap resolves into
// SHOULD_ENABLE_HOSTS_PLUGIN (see withLocalDNSHostsPlugin on the RP side); the LPS LocalDnsProfile
// carries enableHostsPlugin off HostsPluginConfig, which is not populated on the HCP proto yet and
// therefore reads false. Honouring the profile alone would silently drop the hosts block from a
// node that booted with it, so prefer the bootstrap-resolved value and fall back to the profile.
func (a *App) includeHostsPluginForCorefile(profile *aksnodeconfigv1.LocalDnsProfile) bool {
	vars, path, err := a.nbcCmdEnvVars()
	if err != nil {
		slog.Info("could not read nbc-cmd.sh for hosts plugin state, using LPS profile value", "error", err)
		return profile.GetEnableHostsPlugin()
	}
	switch strings.TrimSpace(vars["SHOULD_ENABLE_HOSTS_PLUGIN"]) {
	case "true":
		return true
	case "false":
		return false
	}
	slog.Info("nbc-cmd.sh has no SHOULD_ENABLE_HOSTS_PLUGIN, using LPS profile value", "path", path)
	return profile.GetEnableHostsPlugin()
}
