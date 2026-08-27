package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

// check-hotfix reads the hotfix pointer from the live-patching-service (LPS) over the
// IMDS-attested gRPC path that is reachable pre-kubelet, then writes it to the same path
// download-hotfix already reads. download-hotfix then re-resolves the pointer against the
// node's baked ANC version and keeps its unchanged patch-only, strictly-higher gating.
// check-hotfix only fetches and stages the pointer; it never installs anything and never
// blocks provisioning (fail-open).
//
// The wire transport is gRPC (GetComponentConfig) and lives in checkhotfix_grpc.go; this file
// holds the transport-independent domain logic: the IMDS attested-token fetch, node-config
// endpoint resolution, the benign-vs-fatal taxonomy, and the fail-open fetch/parse/stage
// workflow.
const (
	defaultNBCCmdPath = "/opt/azure/containers/aks-node-controller-nbc-cmd.sh"

	// lpsALPNProto is the ALPN protocol the kube-api-proxy envoy uses to route the TLS connection
	// to the live-patching-service (LPS) gRPC backend. The client dials the apiserver FQDN as SNI
	// (riding the existing apiserver egress rule) and advertises this ALPN value; envoy then
	// forwards the stream to LPS. It is the ALPN-based successor to the legacy special-SNI route.
	lpsALPNProto = "aks-live-patching"

	// alpnH2Proto is the HTTP/2 ALPN identifier gRPC negotiates over TLS. We advertise it alongside
	// lpsALPNProto so envoy routes on the custom proto while the gRPC transport still negotiates h2.
	// grpc-go's credentials.NewTLS appends "h2" to NextProtos itself, but listing it explicitly keeps
	// the on-wire ALPN set self-documenting.
	alpnH2Proto = "h2"

	// lpsAPIServerPort is the HTTPS port the apiserver front (and thus the LPS path) listens on.
	// Envoy forwards the matched ALPN stream to the LPS backend internally; the client only ever
	// dials the apiserver front here.
	lpsAPIServerPort = "443"

	// imdsAttestedDocURL returns the IMDS attested-data document, whose signature is used as
	// the LPS Authorization token. IMDS is reachable pre-kubelet (the same primitive Secure
	// TLS Bootstrap uses), so this works before any kube credential exists.
	imdsAttestedDocURL = "http://169.254.169.254/metadata/attested/document?api-version=2025-04-07"
)

// Timeout tuning for the IMDS and LPS fetches. The generic transport/retry mechanics live in
// the common package (common.NewBaseTransport + common.RetryStringFetch); these constants are
// the domain-specific budgets this command layers on top.
const (
	// lpsFetchTimeout bounds the whole LPS gRPC round-trip (cold connect + RPC, no retry). It is a
	// balance, not merely "tight": too short misses a hotfix from a slow-but-reachable LPS (we fall
	// back to the possibly-stale cold-start pointer); too long delays provisioning when the front
	// is unreachable. 3s is a starting point to tune from production: the CheckHotfix event records
	// outcome + durationMs, so a too-tight deadline surfaces as customDataFallback/failed events
	// with durationMs near this budget. Revisit with real traffic (private-cluster fronts may be
	// slower).
	lpsFetchTimeout = 3 * time.Second // overall gRPC round-trip (cold connect + RPC)

	// IMDS timeouts. IMDS is a link-local endpoint (169.254.169.254) that is normally
	// near-instant, so these are tighter than the LPS knobs. imdsFetchTimeout bounds a single
	// attempt.
	imdsDialTimeout = 1 * time.Second
	// imdsTLSHandshakeTimeout is inert in practice: IMDS is a plain-HTTP endpoint
	// (http://169.254.169.254), so the transport never enters the TLS handshake path and this
	// timer is never armed. It is kept nonzero only so the shared transport builder stays
	// uniform across the LPS (HTTPS) and IMDS callers, and to remain defensive if IMDS is ever
	// pointed at an HTTPS endpoint.
	imdsTLSHandshakeTimeout   = 1 * time.Second
	imdsResponseHeaderTimeout = 1 * time.Second
	imdsFetchTimeout          = 2 * time.Second

	// imdsMaxAttempts is the total number of IMDS attempts (1 initial + retries). IMDS is
	// local and usually reliable, so a single quick retry is enough to smooth a one-off blip
	// without materially adding to provisioning latency.
	imdsMaxAttempts = 2
)

// checkHotfixOutcome is the telemetry taxonomy emitted under TaskName "CheckHotfix".
type checkHotfixOutcome string

const (
	// outcomeLPSRead: LPS pointer fetched + parsed OK and a hotfix entry matched this node's base.
	outcomeLPSRead checkHotfixOutcome = "lpsRead"
	// outcomeNoHotfixForBase: LPS read OK but no entry matched this node's YYYYMM.DD base.
	outcomeNoHotfixForBase checkHotfixOutcome = "noHotfixForBase"
	// outcomeNoHotfixAvailable: the LPS was reachable and the request was well-formed, but it
	// returned no hotfix for this node (gRPC Unauthenticated/PermissionDenied/NotFound). This
	// simply means the LPS has nothing published for this node yet. It is the expected steady
	// state, so it is a benign no-op: no overlay is staged and it is never treated as a failure.
	outcomeNoHotfixAvailable checkHotfixOutcome = "noHotfixAvailable"
	// outcomeCustomDataFallback: LPS read failed; the embedded customdata pointer was used.
	outcomeCustomDataFallback checkHotfixOutcome = "customDataFallback"
	// outcomeFailed: everything failed; nothing was staged. Provisioning still proceeds (exit 0).
	outcomeFailed checkHotfixOutcome = "failed"
)

// errLPSUnavailable is a benign sentinel meaning "no hotfix is available for this node yet"
// rather than a failure. It is returned (wrapped with the specific gRPC code for diagnosis) for
// the gRPC Unauthenticated/PermissionDenied/NotFound codes, all of which mean the LPS is reachable
// but has nothing published for this node. check-hotfix must not classify it as a hard failure or
// retry it. A plain sentinel suffices because the code is only carried in the message, never read
// programmatically; classification is by identity via isLPSUnavailable.
var errLPSUnavailable = errors.New("LPS has no hotfix available for this node")

// isLPSUnavailable reports whether err is a benign "nothing for this node yet" LPS response.
func isLPSUnavailable(err error) bool {
	return errors.Is(err, errLPSUnavailable)
}

// shouldColdStartFallback reports whether a failed LPS fetch should fall back to the embedded
// cold-start pointer. Fallback is only appropriate when the LPS could not be reached or talked
// to: transport/pre-network errors (no gRPC status) and server-side conditions
// (Unavailable/DeadlineExceeded/Internal). A reachable LPS that returns an authoritative client
// rejection (InvalidArgument/ResourceExhausted) is not a reason to stage a possibly-stale
// cold-start pointer. See mapGRPCError for the code -> fallbackAllowed classification.
func shouldColdStartFallback(err error) bool {
	var grpcErr *lpsGRPCStatusError
	if errors.As(err, &grpcErr) {
		return grpcErr.fallbackAllowed
	}
	return true
}

// runCheckHotfixCommand is the cli Action for `check-hotfix`. It ALWAYS returns nil so
// provisioning is never blocked: any error (404, 403, timeout, parse failure) is logged,
// emitted as telemetry, and swallowed. Internal helpers return errors for testability only.
func (a *App) runCheckHotfixCommand(ctx context.Context) (err error) {
	slog.Info("aks-node-controller check-hotfix started")
	startTime := time.Now()

	// Fail-open hardening: a panic anywhere in the check-hotfix workflow must not crash the
	// process. The wrapper runs check-hotfix before the customdata (cold-start) route, so a
	// crash here could otherwise prevent that route from completing. Recover, emit failed
	// telemetry, and return nil so provisioning proceeds.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("check-hotfix panicked (fail-open)", "panic", r)
			if a.eventLogger != nil {
				a.eventLogger.LogEvent("CheckHotfix",
					fmt.Sprintf("check-hotfix outcome=%s panic=%v", outcomeFailed, r),
					helpers.EventLevelError, startTime, time.Now())
			}
			err = nil
		}
	}()

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

	data, fetchErr := a.fetchHotfix(ctx)
	if fetchErr != nil {
		return a.handleFetchError(hotfixPath, fetchErr)
	}

	cfg, err := parseHotfixConfig(data)
	if err != nil {
		return outcomeFailed, fmt.Errorf("parsing LPS hotfix pointer: %w", err)
	}

	// An empty served hotfixes map means the LPS is reachable but has nothing published for
	// this node's fleet yet - the expected steady state while the component exists but carries
	// no entries. Treat it exactly like a benign NotFound: do NOT write, leaving the shared
	// on-disk pointer (which cloud-init populated with version/scripts_version) intact, and
	// report the benign no-op outcome. Writing an empty map here would clobber those fields and
	// silently disable the cloud-init provisioning-hotfix path. This also makes a 200-"{}"
	// response behave identically to a 404, and folds in the empty-body-is-benign parse case.
	if len(cfg.Hotfixes) == 0 {
		slog.Info("LPS returned no hotfixes for this node; leaving existing pointer intact (fail-open)")
		return outcomeNoHotfixAvailable, nil
	}

	// stagedHotfixConfig is the exact shape writeHotfixConfig merges in (map-only; the legacy
	// Version field is dropped). Basing both the write and the telemetry decision on this same
	// value keeps the reported outcome consistent with what download-hotfix will actually read:
	// a pointer with no entry for this node's base stages nothing resolvable, so it must report
	// noHotfixForBase, not LPSRead.
	staged := hotfixConfig{Hotfixes: cfg.Hotfixes}

	if err := writeHotfixConfig(hotfixPath, staged); err != nil {
		return outcomeFailed, fmt.Errorf("writing hotfix config: %w", err)
	}

	// Report whether this node's base actually has a pointer in the staged config.
	// download-hotfix still performs the authoritative patch-only-strictly-higher gating;
	// this is telemetry only.
	if staged.resolveVersion(Version) == "" {
		return outcomeNoHotfixForBase, nil
	}
	return outcomeLPSRead, nil
}

// handleFetchError maps a failed LPS fetch to a check-hotfix outcome. It is fail-open: benign
// 401/403/404 is a no-op, a reachable client error (non-benign 4xx) fails without staging a
// possibly-stale pointer, and only an unreachable/5xx LPS falls back to the cold-start pointer.
func (a *App) handleFetchError(hotfixPath string, fetchErr error) (checkHotfixOutcome, error) {
	if isLPSUnavailable(fetchErr) {
		// The LPS is reachable but has no hotfix published for this node (HTTP 401/403/404).
		// This is the expected steady state, not a failure: stage no overlay (download-hotfix
		// keeps whatever pointer it had) and report a benign outcome. Fail-open.
		slog.Info("LPS reports no hotfix available for this node (fail-open)", "reason", fetchErr)
		return outcomeNoHotfixAvailable, nil
	}
	if !shouldColdStartFallback(fetchErr) {
		// The LPS was reachable but rejected the request with a non-benign 4xx (e.g. 400/429).
		// The server is authoritative here, so do NOT stage the (possibly stale) cold-start
		// pointer; fail-open without a fallback.
		slog.Warn("LPS returned a client error; not falling back to cold-start pointer (fail-open)",
			"error", fetchErr)
		return outcomeFailed, fmt.Errorf("LPS fetch failed with a client error, not falling back: %w", fetchErr)
	}
	// LPS could not be reached or talked to (transport failure / 5xx): fall back to the pointer
	// embedded in the node config (cold-start path). See coldStartHotfixConfig for the contract TODO.
	slog.Warn("failed to reach LPS, attempting cold-start fallback", "error", fetchErr)
	cfg, ok, coldErr := a.coldStartHotfixConfig()
	if coldErr != nil {
		return outcomeFailed, fmt.Errorf("LPS fetch failed (%w) and cold-start fallback failed: %w", fetchErr, coldErr)
	}
	if !ok {
		return outcomeFailed, fmt.Errorf("LPS fetch failed and no cold-start pointer present: %w", fetchErr)
	}
	if err := writeHotfixConfig(hotfixPath, cfg); err != nil {
		return outcomeFailed, fmt.Errorf("writing cold-start hotfix config: %w", err)
	}
	return outcomeCustomDataFallback, nil
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

// fetchHotfix returns the raw LPS response bytes. Tests inject checkHotfixFetcher to supply
// canned pointer JSON or errors without networking; otherwise it fetches over gRPC
// (GetComponentConfig), the sole live-patching-service transport.
func (a *App) fetchHotfix(ctx context.Context) ([]byte, error) {
	if a.checkHotfixFetcher != nil {
		return a.checkHotfixFetcher(ctx)
	}
	return a.fetchHotfixOverGRPC(ctx)
}

// lpsTargetFromNodeConfig reads the apiserver FQDN (the forced dial target) and the cluster
// CA (TLS trust) from the AKSNodeConfig. If the config file is absent (e.g. Staging Phase 2
// where AKSNodeConfigJSON is empty), it falls back to the existing nbc-cmd.sh file, which
// contains the same values as command-scoped environment variables.
//
// check-hotfix runs before the provisioning scripts (cse_config.sh), so the on-node decoded
// CA file (/etc/kubernetes/certs/ca.crt) does not exist yet -- it is written later during
// provisioning. Depending on the Scriptless phase, either the node config or nbc-cmd carries
// the CA as base64-encoded PEM (the same value cse_config.sh later decodes into that file).
func (a *App) lpsTargetFromNodeConfig() (string, []byte, error) {
	path := a.getNodeConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("node config not found, trying nbc-cmd.sh fallback", "path", path)
			return a.lpsTargetFromNBCCmd()
		}
		return "", nil, fmt.Errorf("reading node config %s: %w", path, err)
	}
	cfg, perr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if perr != nil {
		// Forward-compatible parse: unknown fields are discarded, so a non-nil error here
		// means some fields were unusable. Continue with whatever parsed.
		slog.Info("node config parsed with errors, continuing with partial config", "error", perr)
	}
	if cfg == nil {
		if perr != nil {
			return "", nil, fmt.Errorf("node config %s could not be parsed: %w", path, perr)
		}
		return "", nil, fmt.Errorf("node config %s could not be parsed", path)
	}

	fqdn := cfg.GetApiServerConfig().GetApiServerName()
	if fqdn == "" {
		if perr != nil {
			// A required field is missing and the unmarshal reported an error; surface the
			// original parse failure as the root cause rather than masking it.
			return "", nil, fmt.Errorf("node config has no api_server_config.api_server_name (parse error: %w)", perr)
		}
		return "", nil, fmt.Errorf("node config has no api_server_config.api_server_name")
	}

	var caPEM []byte
	if caB64 := cfg.GetKubernetesCaCert(); caB64 != "" {
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
// signature, the same primitive Secure TLS Bootstrap and the custom-patching flow use. IMDS
// is local and usually reliable, so it makes up to imdsMaxAttempts attempts (one quick retry)
// to smooth a one-off blip; each attempt is independently bounded by imdsFetchTimeout.
func fetchIMDSAttestedToken(ctx context.Context) (string, error) {
	return common.RetryStringFetch(ctx, imdsMaxAttempts, fetchIMDSAttestedTokenOnce)
}

// fetchIMDSAttestedTokenOnce performs a single IMDS attested-document GET and returns the
// document signature.
func fetchIMDSAttestedTokenOnce(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, imdsFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imdsAttestedDocURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata", "true")

	// IMDS (169.254.169.254) is a link-local endpoint that must never be routed through an
	// HTTP(S) proxy. The shared base transport disables proxying (unlike the default client,
	// which honors HTTP(S)_PROXY env vars), matching the shell implementation.
	imdsClient := &http.Client{
		Timeout: imdsFetchTimeout,
		Transport: common.NewBaseTransport(common.HTTPTransportOptions{
			DialTimeout:           imdsDialTimeout,
			TLSHandshakeTimeout:   imdsTLSHandshakeTimeout,
			ResponseHeaderTimeout: imdsResponseHeaderTimeout,
		}),
	}
	resp, err := imdsClient.Do(req)
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

// parseHotfixConfig extracts the hotfix pointer from an LPS response body. The body is the
// {"hotfixes":{...}} JSON object that unmarshals DIRECTLY into the shared 2.1a hotfixConfig,
// so check-hotfix and download-hotfix share ONE identical data contract.
//
// An empty body is treated as a benign zero-value config (no hotfix), NOT an error: a
// conformant LPS serves "{}" for an empty map, but a successful RPC with an unset config
// string (proto3 default "") means the same thing - the service is reachable and has nothing
// for this node. Erroring there would surface a false "failed" outcome on an otherwise OK
// call. This mirrors download-hotfix's readHotfixConfig, which also maps empty -> zero value.
func parseHotfixConfig(data []byte) (hotfixConfig, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return hotfixConfig{}, nil
	}
	var cfg hotfixConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return hotfixConfig{}, fmt.Errorf("unmarshaling hotfix pointer JSON: %w", err)
	}
	return cfg, nil
}

// coldStartHotfixConfig reads a LENIENT top-level "hotfixes" object from the AKSNodeConfig
// JSON (aks-node-controller-config.json) as a best-effort cold-start fallback, used only when
// the LPS endpoint could not be reached or talked to (transport failure / 5xx). A benign
// 401/403/404 is NOT a cold-start: the LPS authoritatively has nothing for this node, so that
// path stages no overlay.
//
// This top-level key is out-of-contract: AKSNodeConfig has no typed "hotfixes" field and none
// is planned, so nothing populates it in practice - this read is a no-op on every real node and
// never fails provisioning. The authoritative provisioning-time pointer is the customdata-
// delivered aks-node-controller-hotfix.json (defaultHotfixVersionPath), which download-hotfix
// consumes directly. This lenient read is therefore a candidate for removal once confirmed
// unused; it is retained here only to preserve the pre-existing fail-open fallback behavior.
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

// writeHotfixConfig stages the LPS-served hotfixes map to the path download-hotfix reads.
// It is a read-modify-write: the shared file at defaultHotfixVersionPath is ALSO written by
// cloud-init (from hotfix_generate.py) carrying "version" and "scripts_version", which
// download-hotfix reads to drive the binary and CSE-scripts hotfix respectively. To avoid
// clobbering those fields, this preserves whatever the existing file holds and replaces only
// the "hotfixes" map with the served one. The write is atomic (temp file + rename) so a
// concurrent reader never sees a partial file.
func writeHotfixConfig(path string, cfg hotfixConfig) error {
	// Read-modify-write: carry forward cloud-init's version/scripts_version. If the existing
	// file is missing or unreadable we proceed with empty carry-over fields (best-effort,
	// fail-open); it was already unusable to download-hotfix in that case.
	existing, err := readHotfixConfig(path)
	if err != nil {
		slog.Warn("could not read existing hotfix pointer; version/scripts_version will not be preserved",
			"path", path, "error", err)
		existing = &hotfixConfig{}
	}

	// Normalize nil -> empty map so the staged file always has a stable top-level "hotfixes"
	// key, i.e. {"hotfixes":{}} rather than {} for an empty map. hotfixConfig.Hotfixes carries
	// omitempty for its own read path, so it must not be reused here.
	hotfixes := cfg.Hotfixes
	if hotfixes == nil {
		hotfixes = map[string]string{}
	}
	out := struct {
		Version        string            `json:"version,omitempty"`
		ScriptsVersion string            `json:"scripts_version,omitempty"`
		Hotfixes       map[string]string `json:"hotfixes"`
	}{
		Version:        existing.Version,
		ScriptsVersion: existing.ScriptsVersion,
		Hotfixes:       hotfixes,
	}
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
	// CreateTemp defaults to 0600, but the hotfix file may already exist on disk with a
	// different mode (e.g. 0644, from older provisioning mechanisms), so rewriting it
	// must not silently tighten the mode. Preserve the existing file's mode when
	// present, otherwise match the 0644 contract.
	fileMode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		fileMode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpPath, fileMode); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting mode on temp file %s: %w", tmpPath, err)
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

// getNBCCmdPath returns the injectable nbc-cmd.sh path, defaulting to the standard location.
func (a *App) getNBCCmdPath() string {
	if a.nbcCmdPath != "" {
		return a.nbcCmdPath
	}
	return defaultNBCCmdPath
}

// lpsTargetFromNBCCmd reads the apiserver FQDN and cluster CA from the existing nbc-cmd.sh file.
// This is the fallback path when the full AKSNodeConfig JSON is not present (e.g. Staging
// Phase 2). Reuse the parser used by env comparison for the flattened NBC command.
func (a *App) lpsTargetFromNBCCmd() (string, []byte, error) {
	path := a.getNBCCmdPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading nbc-cmd %s: %w", path, err)
	}

	vars := parseEnvVarsFromNBCCmdContent(string(raw))
	fqdn := vars["API_SERVER_NAME"]
	if fqdn == "" {
		return "", nil, fmt.Errorf("nbc-cmd %s has no API_SERVER_NAME", path)
	}

	var caPEM []byte
	if caB64 := vars["KUBE_CA_CRT"]; caB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(caB64)
		if err != nil {
			return "", nil, fmt.Errorf("decoding nbc-cmd KUBE_CA_CRT: %w", err)
		}
		caPEM = decoded
	}

	slog.Info("loaded LPS target from nbc-cmd.sh fallback", "fqdn", fqdn, "path", path)
	return fqdn, caPEM, nil
}
