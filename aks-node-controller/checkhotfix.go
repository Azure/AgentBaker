package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

// check-hotfix reads the hotfix pointer from the live-patching-service (LPS) over the
// IMDS-attested SNI path that is reachable pre-kubelet, then writes it to the same path
// download-hotfix already reads. download-hotfix then re-resolves the pointer against the
// node's baked ANC version and keeps its unchanged patch-only, strictly-higher gating.
// check-hotfix only fetches and stages the pointer; it never installs anything and never
// blocks provisioning (fail-open).
const (
	// lpsSNIHost is the live-patching-service SNI/Host that the kube-api-proxy envoy on the
	// apiserver front routes to the LPS backend. The TLS handshake pins this as ServerName
	// while the TCP connection is forced to the apiserver FQDN (the curl --resolve trick),
	// giving the faithful end-to-end path node -> SNI(lpsSNIHost) -> envoy -> LPS.
	lpsSNIHost = "aks-security-patch.data.mcr.microsoft.com"

	// lpsAPIServerPort is the HTTPS port the apiserver front (and thus the LPS path) listens on.
	lpsAPIServerPort = "443"

	// lpsHotfixPath is the LPS route serving the base->hotfix pointer map.
	//
	// TODO(provisioning-hotfix): this route and its response schema are a planned-maintenance
	// LPS-endpoint deliverable that is NOT finalized yet. The connectivity prototype only
	// proved reachability of the LPS read path (/v1/packages). Replace this placeholder with
	// the real route once the LPS endpoint contract is published. The expected response body
	// is the {"hotfixes":{"<YYYYMM.DD base>":"<YYYYMM.DD.PATCH>"}} JSON object that parses
	// directly into the shared hotfixConfig type (see parseHotfixConfig).
	lpsHotfixPath = "/v1/anc-hotfix"

	// imdsAttestedDocURL returns the IMDS attested-data document, whose signature is used as
	// the LPS Authorization token. IMDS is reachable pre-kubelet (the same primitive Secure
	// TLS Bootstrap uses), so this works before any kube credential exists.
	imdsAttestedDocURL = "http://169.254.169.254/metadata/attested/document?api-version=2025-04-07"

	// lpsFetchTimeout caps the LPS round-trip so a hung/slow endpoint never delays provisioning.
	lpsFetchTimeout = 10 * time.Second
	// imdsTimeout bounds the IMDS attested-document fetch.
	imdsTimeout = 5 * time.Second
)

// NOTE: the IMDS attested-token fetch and the SNI-pinned/forced-dial HTTP client below are
// duplicated from the check-lps connectivity prototype. When the shared LPS client lands,
// de-duplicate these helpers (fetchIMDSAttestedToken, buildLPSHTTPClient, App.attestedToken)
// into that single client and have check-hotfix consume it.

// checkHotfixOutcome is the telemetry taxonomy emitted under TaskName "CheckHotfix".
type checkHotfixOutcome string

const (
	// outcomeLPSRead: LPS pointer fetched + parsed OK and a hotfix entry matched this node's base.
	outcomeLPSRead checkHotfixOutcome = "lpsRead"
	// outcomeNoHotfixForBase: LPS read OK but no entry matched this node's YYYYMM.DD base.
	outcomeNoHotfixForBase checkHotfixOutcome = "noHotfixForBase"
	// outcomeNotEnrolled: the LPS was reachable and the request was well-formed, but it has
	// no hotfix for this node yet (HTTP 401/403/404). The LPS authorizes in two stages -
	// IMDS attestation, then agent-pool authorization - so a node whose pool is not yet
	// enrolled in live-patching gets a 401, and 403/404 likewise mean nothing is published
	// for this node. This is the expected steady state on a freshly enabled cluster, so it is
	// a benign no-op: no overlay is staged and it is never treated as a failure.
	outcomeNotEnrolled checkHotfixOutcome = "notEnrolled"
	// outcomeCustomDataFallback: LPS read failed; the embedded customdata pointer was used.
	outcomeCustomDataFallback checkHotfixOutcome = "customDataFallback"
	// outcomeFailed: everything failed; nothing was staged. Provisioning still proceeds (exit 0).
	outcomeFailed checkHotfixOutcome = "failed"
	// outcomeDisabled: the AKSNodeConfig enable_provisioning_hotfix field is not true (false/unset),
	// so check-hotfix no-ops without any remote hotfix call. Provisioning proceeds (exit 0).
	outcomeDisabled checkHotfixOutcome = "disabled"
)

// lpsUnavailableError marks a benign LPS response that means "no hotfix is available for this
// node yet" rather than a failure. It is returned for HTTP 401 (agent pool not enrolled in
// live-patching), 403, and 404. On a freshly LPS-enabled cluster the enrollment map is empty,
// so 401 is the EXPECTED steady-state response until a pool is actually enrolled; check-hotfix
// must not classify it as a hard failure or retry it.
type lpsUnavailableError struct {
	statusCode int
}

func (e *lpsUnavailableError) Error() string {
	return fmt.Sprintf("LPS has no hotfix available for this node (status %d)", e.statusCode)
}

// isLPSUnavailable reports whether err is a benign "nothing for this node yet" LPS response.
func isLPSUnavailable(err error) bool {
	var u *lpsUnavailableError
	return errors.As(err, &u)
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
	// Single source of truth: the enable_provisioning_hotfix contract field on the AKSNodeConfig.
	// When it is not true (false or unset), no-op without any remote hotfix call. The wrapper
	// calls check-hotfix unconditionally, so this Go gate is what keeps disabled nodes inert.
	if !a.provisioningHotfixEnabled() {
		slog.Info("check-hotfix disabled: enable_provisioning_hotfix is not true; skipping hotfix pointer fetch")
		return outcomeDisabled, nil
	}

	hotfixPath := a.hotfixVersionPath
	if hotfixPath == "" {
		hotfixPath = defaultHotfixVersionPath
	}

	data, fetchErr := a.fetchHotfix(ctx)
	if fetchErr != nil {
		if isLPSUnavailable(fetchErr) {
			// The LPS is reachable but has nothing for this node yet (e.g. the agent pool is
			// not enrolled in live-patching). This is the expected steady state, not a
			// failure: stage no overlay (download-hotfix keeps whatever pointer it had) and
			// report a benign outcome. Fail-open.
			slog.Info("LPS reports no hotfix available for this node (fail-open)", "reason", fetchErr)
			return outcomeNotEnrolled, nil
		}
		// LPS read failed to reach/talk to the endpoint: fall back to the pointer embedded
		// in the node config (cold-start path). See coldStartHotfixConfig for the contract TODO.
		slog.Warn("failed to read hotfix pointer from LPS, attempting cold-start fallback",
			"error", fetchErr)
		cfg, ok, coldErr := a.coldStartHotfixConfig()
		if coldErr != nil {
			return outcomeFailed, fmt.Errorf("LPS fetch failed (%v) and cold-start fallback failed: %w", fetchErr, coldErr)
		}
		if !ok {
			return outcomeFailed, fmt.Errorf("LPS fetch failed and no cold-start pointer present: %w", fetchErr)
		}
		if err := writeHotfixConfig(hotfixPath, cfg); err != nil {
			return outcomeFailed, fmt.Errorf("writing cold-start hotfix config: %w", err)
		}
		return outcomeCustomDataFallback, nil
	}

	cfg, err := parseHotfixConfig(data)
	if err != nil {
		return outcomeFailed, fmt.Errorf("parsing LPS hotfix pointer: %w", err)
	}

	if err := writeHotfixConfig(hotfixPath, cfg); err != nil {
		return outcomeFailed, fmt.Errorf("writing hotfix config: %w", err)
	}

	// Report whether this node's base actually has a pointer. download-hotfix still
	// performs the authoritative patch-only-strictly-higher gating; this is telemetry only.
	if cfg.resolveVersion(Version) == "" {
		return outcomeNoHotfixForBase, nil
	}
	return outcomeLPSRead, nil
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

// fetchHotfix returns the raw LPS response body. Tests inject checkHotfixFetcher to supply
// canned pointer JSON or errors without networking.
func (a *App) fetchHotfix(ctx context.Context) ([]byte, error) {
	if a.checkHotfixFetcher != nil {
		return a.checkHotfixFetcher(ctx)
	}
	return a.fetchHotfixFromLPS(ctx)
}

// fetchHotfixFromLPS performs the real network GET against the LPS over the IMDS-attested
// SNI path. It sources the apiserver FQDN and cluster CA from the AKSNodeConfig that ANC
// already parses, pins the TLS ServerName to lpsSNIHost while forcing the TCP dial to the
// FQDN, attaches the IMDS attested-data token as the Authorization header, and returns the
// raw response body. A 2xx returns the body; 401/403/404 are surfaced as a benign
// lpsUnavailableError ("nothing for this node yet"); any other non-2xx is a hard error so the
// caller falls back / fails open.
func (a *App) fetchHotfixFromLPS(ctx context.Context) ([]byte, error) {
	fqdn, caPEM, err := a.lpsTargetFromNodeConfig()
	if err != nil {
		return nil, fmt.Errorf("resolving LPS endpoint from node config: %w", err)
	}

	token, err := a.attestedToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("imds attested token: %w", err)
	}

	client, caSource, err := buildLPSHTTPClient(fqdn, caPEM)
	if err != nil {
		return nil, fmt.Errorf("building LPS http client: %w", err)
	}
	// caSource records whether the server cert is verified against the provision-config CA
	// or (when that CA is unavailable) trust was skipped; surface it for diagnosis.
	slog.Info("check-hotfix LPS TLS trust source", "caSource", caSource, "dialHost", fqdn)

	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	url := fmt.Sprintf("https://%s:%s%s", lpsSNIHost, lpsAPIServerPort, lpsHotfixPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", token)
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
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusNotFound:
		// Benign: reachable LPS with nothing for this node yet (e.g. pool not enrolled).
		// Surfaced as a typed error so the caller treats it as a no-op, not a failure.
		return nil, &lpsUnavailableError{statusCode: resp.StatusCode}
	default:
		return nil, fmt.Errorf("LPS returned status %d for %s", resp.StatusCode, url)
	}
}

// lpsTargetFromNodeConfig reads the apiserver FQDN (the forced dial target) and the cluster
// CA (TLS trust) from the AKSNodeConfig.
//
// check-hotfix runs before the provisioning scripts (cse_config.sh), so the on-node decoded
// CA file (/etc/kubernetes/certs/ca.crt) does not exist yet -- it is written later during
// provisioning. The node config is the only credential source guaranteed to be present at
// this point and it carries the CA as base64-encoded PEM (the same value cse_config.sh later
// decodes into that file).
func (a *App) lpsTargetFromNodeConfig() (fqdn string, caPEM []byte, err error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		// Forward-compatible parse: unknown fields are discarded, so a non-nil error here
		// means some fields were unusable. Continue with whatever parsed.
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		return "", nil, fmt.Errorf("node config %s could not be parsed", path)
	}

	fqdn = stripScheme(strings.TrimSpace(cfg.GetApiServerConfig().GetApiServerName()))
	if fqdn == "" {
		return "", nil, fmt.Errorf("node config has no api_server_config.api_server_name")
	}

	if caB64 := strings.TrimSpace(cfg.GetKubernetesCaCert()); caB64 != "" {
		decoded, derr := base64.StdEncoding.DecodeString(caB64)
		if derr != nil {
			return "", nil, fmt.Errorf("decoding node config kubernetes_ca_cert: %w", derr)
		}
		caPEM = decoded
	}
	return fqdn, caPEM, nil
}

// attestedToken returns the IMDS attested-data signature used as the LPS Authorization
// token. The fetch is overridable for testing via App.fetchAttestedToken.
func (a *App) attestedToken(ctx context.Context) (string, error) {
	if a.fetchAttestedToken != nil {
		return a.fetchAttestedToken(ctx)
	}
	return fetchIMDSAttestedToken(ctx)
}

// fetchIMDSAttestedToken queries IMDS for the attested-data document and returns its
// signature, the same primitive Secure TLS Bootstrap and the custom-patching flow use.
func fetchIMDSAttestedToken(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, imdsTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imdsAttestedDocURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("imds returned status %d", resp.StatusCode)
	}
	var doc struct {
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Signature == "" {
		return "", fmt.Errorf("imds attested document had empty signature")
	}
	return doc.Signature, nil
}

// buildLPSHTTPClient builds the HTTP client for the LPS fetch: TLS ServerName pinned to
// lpsSNIHost, the TCP dial forced to dialHost:443 (the curl --resolve equivalent), and
// RootCAs from the cluster CA. It returns the client and a string describing the TLS trust
// source ("provision-config" or "insecure-skip-verify"), with a short timeout so
// provisioning is never delayed. When the CA is unavailable the client falls back to
// skipping verification (reachability-only), mirroring the connectivity prototype; the
// staged pointer is non-authoritative anyway because download-hotfix re-resolves and gates.
func buildLPSHTTPClient(dialHost string, caPEM []byte) (*http.Client, string, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: lpsSNIHost}
	caSource := "insecure-skip-verify"
	if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, "", fmt.Errorf("failed to parse cluster CA PEM")
		}
		tlsConfig.RootCAs = pool
		caSource = "provision-config"
	} else {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // CA unavailable pre-provision; pointer is re-resolved and gated by download-hotfix
	}

	dialAddr := net.JoinHostPort(dialHost, lpsAPIServerPort)
	dialer := &net.Dialer{Timeout: lpsFetchTimeout}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		// Force the connection to dialHost regardless of the SNI/Host (lpsSNIHost) in the
		// request URL -- the curl --resolve equivalent.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dialAddr)
		},
	}
	return &http.Client{Timeout: lpsFetchTimeout, Transport: transport}, caSource, nil
}

// parseHotfixConfig extracts the hotfix pointer from an LPS response body. The body is the
// {"hotfixes":{...}} JSON object that unmarshals DIRECTLY into the shared 2.1a hotfixConfig,
// so check-hotfix and download-hotfix share ONE identical data contract.
func parseHotfixConfig(data []byte) (hotfixConfig, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return hotfixConfig{}, fmt.Errorf("LPS response body is empty")
	}
	var cfg hotfixConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return hotfixConfig{}, fmt.Errorf("unmarshaling hotfix pointer JSON: %w", err)
	}
	return cfg, nil
}

// coldStartHotfixConfig reads a LENIENT top-level "hotfixes" object from the AKSNodeConfig
// JSON. This is the PoC cold-start fallback used only when the LPS endpoint could not be
// reached or talked to (transport failure / 5xx). A benign 401/403/404 is NOT a cold-start:
// the LPS authoritatively has nothing for this node, so that path stages no overlay.
//
// TODO(provisioning-hotfix): There is no formalized AKSNodeConfig contract field for the
// embedded pointer yet - the control-plane side that would populate a typed field is not
// built. Once that contract exists, replace this lenient top-level read with the typed field
// and drop the permissive JSON shape. Until then we read it best-effort and never fail
// provisioning.
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
	// on-disk contract matches what the live-patching-service publishes.
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

// provisioningHotfixEnabled reports whether the AKSNodeConfig enable_provisioning_hotfix
// contract field is explicitly true. It is the single source of truth for whether
// check-hotfix does any work. Any read/parse problem yields false (default-off, fail-open):
// a node that cannot prove the feature is on is treated as off, and provisioning still
// proceeds because the caller swallows all errors.
func (a *App) provisioningHotfixEnabled() bool {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Info("check-hotfix gate: cannot read node config, treating as disabled", "path", path, "error", err)
		return false
	}
	cfg, err := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if err != nil {
		// Forward-compatible parse: unknown fields are discarded. A non-nil error means the
		// document was unusable, so fall back to disabled.
		slog.Info("check-hotfix gate: node config parsed with errors, evaluating partial config", "error", err)
	}
	return cfg.GetEnableProvisioningHotfix()
}

// getNodeConfigPath returns the injectable node-config path, defaulting to the standard
// AKSNodeConfig location that ANC already reads.
func (a *App) getNodeConfigPath() string {
	if a.nodeConfigPath != "" {
		return a.nodeConfigPath
	}
	return nodeconfigutils.AKSNodeConfigFilePath
}

// stripScheme removes a leading https:// or http:// scheme from a server URL.
func stripScheme(server string) string {
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")
	return strings.TrimRight(server, "/")
}
