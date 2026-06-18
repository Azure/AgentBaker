package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

const (
	// lpsClusterIP is the well-known kubernetes Service ClusterIP for the apiserver.
	// Reaching it requires kube-proxy to have programmed the iptables/IPVS DNAT rules,
	// which has not happened pre-kubelet. This is the "through kube-proxy" probe path.
	lpsClusterIP = "10.0.0.1"
	// lpsAPIServerPort is the HTTPS port the apiserver listens on.
	lpsAPIServerPort = "443"
	// lpsProbePath is the unauthenticated apiserver health endpoint used for the probe.
	lpsProbePath = "/healthz"
	// lpsProbeTimeout bounds each individual probe so the diagnostic never delays
	// provisioning noticeably, even when an endpoint is unreachable.
	lpsProbeTimeout = 5 * time.Second
)

// probeResult captures the outcome of a single connectivity probe for logging.
type probeResult struct {
	// label is the verbose probe name used by the primary slog channel
	// (e.g. "clusterip-via-kube-proxy", "fqdn-direct").
	label string
	// target is the short probe name used by the secondary stdout marker
	// (e.g. "clusterip", "fqdn").
	target     string
	url        string
	reachable  bool
	statusCode int
	latency    time.Duration
	err        error
}

// checkLPS probes apiserver connectivity two ways, pre-kubelet, and logs which works:
//
//  1. ClusterIP (10.0.0.1) — the "through kube-proxy" path. Expected to FAIL pre-kubelet
//     because kube-proxy has not yet programmed the Service ClusterIP DNAT rules.
//  2. apiserver FQDN — the "direct" path. Expected to WORK pre-kubelet because it resolves
//     via DNS to the apiserver load balancer and bypasses kube-proxy entirely.
//
// This empirically validates that the Provisioning-Hotfix Option 4 (LPS IMDS endpoint)
// read path is reachable pre-kubelet via the direct path. It is purely diagnostic and is
// fail-open: it always returns nil and never blocks provisioning.
func (a *App) checkLPS(ctx context.Context, provisionConfigPath string) error {
	// Probe #1: ClusterIP through kube-proxy.
	clusterIPURL := fmt.Sprintf("https://%s:%s%s", lpsClusterIP, lpsAPIServerPort, lpsProbePath)
	a.logProbeResult(a.probeEndpoint(ctx, "clusterip-via-kube-proxy", "clusterip", clusterIPURL))

	// Probe #2: direct apiserver FQDN, sourced from the provision config.
	// TODO(nbc-cmd mode): NBC_CMD provisioning does not pass a provision-config file, so the
	// FQDN probe is skipped in that mode. Add NBC_CMD_PATH FQDN sourcing if/when needed.
	fqdn := apiServerFQDNFromConfig(provisionConfigPath)
	if fqdn == "" {
		slog.Warn("check-lps could not determine apiserver FQDN, skipping direct probe",
			"provisionConfigPath", provisionConfigPath)
		return nil
	}
	fqdnURL := fmt.Sprintf("https://%s:%s%s", fqdn, lpsAPIServerPort, lpsProbePath)
	a.logProbeResult(a.probeEndpoint(ctx, "fqdn-direct", "fqdn", fqdnURL))

	return nil
}

// apiServerFQDNFromConfig reads the apiserver FQDN from the provision config file.
// Returns "" when the path is empty, unreadable, or does not contain an apiserver name.
func apiServerFQDNFromConfig(provisionConfigPath string) string {
	if provisionConfigPath == "" {
		return ""
	}
	data, err := os.ReadFile(provisionConfigPath)
	if err != nil {
		slog.Warn("check-lps failed to read provision config", "path", provisionConfigPath, "error", err)
		return ""
	}
	config, err := nodeconfigutils.UnmarshalConfigurationV1(data)
	if err != nil {
		// Best-effort: UnmarshalConfigurationV1 still returns a (partial) config on
		// non-fatal errors such as unknown fields, so fall through and use what we got.
		slog.Warn("check-lps encountered an error parsing provision config (continuing)",
			"path", provisionConfigPath, "error", err)
	}
	if config == nil {
		return ""
	}
	return config.GetApiServerConfig().GetApiServerName()
}

// probeEndpoint performs a single HTTPS GET against url and returns the outcome.
// TLS verification is intentionally skipped: pre-kubelet there is no established CA
// trust, and the probe only cares about reachability (a completed TLS handshake plus any
// HTTP response — including 401/403 — counts as reachable).
func (a *App) probeEndpoint(ctx context.Context, label, target, url string) probeResult {
	client := a.probeClient()

	ctx, cancel := context.WithTimeout(ctx, lpsProbeTimeout)
	defer cancel()

	result := probeResult{label: label, target: target, url: url}
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.latency = time.Since(start)
		result.err = err
		return result
	}

	resp, err := client.Do(req)
	result.latency = time.Since(start)
	if err != nil {
		result.err = err
		return result
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	result.reachable = true
	result.statusCode = resp.StatusCode
	return result
}

// probeClient returns the injected HTTP client when set, otherwise a default client with
// a bounded timeout and TLS verification disabled (reachability-only probe).
func (a *App) probeClient() *http.Client {
	if a.httpProbeClient != nil {
		return a.httpProbeClient
	}
	return &http.Client{
		Timeout: lpsProbeTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // reachability-only pre-kubelet probe, no trust assumptions
		},
	}
}

// logProbeResult emits a probe outcome on two channels (PoC option C):
//
//   - PRIMARY/authoritative: structured slog JSON to the logger configured in main.go,
//     which writes to /var/log/azure/aks-node-controller.log (a file ANC opens directly,
//     independent of how the systemd oneshot routes stdout). Messages
//     "check-lps probe reachable"/"check-lps probe unreachable" with field probe=<label>.
//   - SECONDARY/convenience: a fixed-format "check-lps:" marker line to stdout (target=<short>),
//     a bonus if it lands in the CSE output log or journal.
func (a *App) logProbeResult(r probeResult) {
	if r.reachable {
		slog.Info("check-lps probe reachable",
			"probe", r.label,
			"url", r.url,
			"statusCode", r.statusCode,
			"latencyMs", r.latency.Milliseconds())
	} else {
		slog.Info("check-lps probe unreachable",
			"probe", r.label,
			"url", r.url,
			"latencyMs", r.latency.Milliseconds(),
			"error", errString(r.err))
	}

	//nolint:forbidigo // secondary convenience marker; primary sink is the slog log file above
	_, _ = fmt.Fprintf(a.probeOut(),
		"check-lps: target=%s url=%s success=%t http_status=%d latency_ms=%d err=%q\n",
		r.target, r.url, r.reachable, r.statusCode, r.latency.Milliseconds(), errString(r.err))
}

// probeOut returns the injected probe marker writer when set, otherwise os.Stdout.
func (a *App) probeOut() io.Writer {
	if a.probeLogWriter != nil {
		return a.probeLogWriter
	}
	return os.Stdout
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
