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
	"net"
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

	// lpsSNIHost is the live-patching-service (LPS) SNI/Host the kube-api-proxy envoy on
	// the apiserver front routes to the LPS backend. This is the faithful end-to-end path:
	// node -> SNI(lpsSNIHost) -> kube-api-proxy envoy -> live-patching-service.
	lpsSNIHost = "aks-security-patch.data.mcr.microsoft.com"
	// lpsPackagesPath is the LPS read endpoint exercised by the faithful probe (same path
	// used by mariner-package-update.sh's custom-patching flow).
	lpsPackagesPath = "/v1/packages"
	// imdsAttestedDocURL returns the IMDS attested-data document, whose signature is used
	// as the LPS Authorization token. IMDS is reachable pre-kubelet (same primitive Secure
	// TLS Bootstrap uses), so this works before any kube credential exists.
	imdsAttestedDocURL = "http://169.254.169.254/metadata/attested/document?api-version=2025-04-07"
	// imdsTimeout bounds the IMDS attested-document fetch.
	imdsTimeout = 5 * time.Second
)

// probeResult captures the outcome of a single connectivity probe for logging.
type probeResult struct {
	label      string
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
	a.logProbeResult(a.probeEndpoint(ctx, "clusterip-via-kube-proxy", clusterIPURL))

	// Probe #2: direct apiserver FQDN, sourced from the provision config.
	fqdn := apiServerFQDNFromConfig(provisionConfigPath)
	if fqdn == "" {
		slog.Warn("check-lps could not determine apiserver FQDN, skipping direct probe",
			"provisionConfigPath", provisionConfigPath)
		return nil
	}
	fqdnURL := fmt.Sprintf("https://%s:%s%s", fqdn, lpsAPIServerPort, lpsProbePath)
	a.logProbeResult(a.probeEndpoint(ctx, "fqdn-direct", fqdnURL))

	// Probe #3 (faithful): the real LPS path. SNI/Host = lpsSNIHost, but dialed at the
	// apiserver FQDN/LB (the --resolve trick), authenticated with an IMDS attested-data
	// token, and verified against the cluster CA from the provision config. Expected to be
	// reachable/200 via the FQDN path when LPS is enabled on the cluster.
	a.logProbeResult(a.probeLPSFaithful(ctx, "lps-sni-fqdn", fqdn, provisionConfigPath))

	// Probe #4 (faithful, via kube-proxy): same LPS request dialed at the ClusterIP.
	// Expected to be unreachable pre-kubelet because kube-proxy has not programmed the
	// Service DNAT rules yet. Kept for fault isolation, mirroring the transport probes.
	a.logProbeResult(a.probeLPSFaithful(ctx, "lps-sni-clusterip", lpsClusterIP, provisionConfigPath))

	return nil
}

// probeLPSFaithful issues the faithful LPS request (GET lpsSNIHost/v1/packages) with the
// TLS ServerName pinned to lpsSNIHost but the TCP connection forced to dialHost (the
// apiserver FQDN or ClusterIP). It attaches the IMDS attested-data token as the
// Authorization header and verifies the server certificate against the cluster CA sourced
// from the provision config (falling back to InsecureSkipVerify, logged, when unavailable).
func (a *App) probeLPSFaithful(ctx context.Context, label, dialHost, provisionConfigPath string) probeResult {
	url := fmt.Sprintf("https://%s:%s%s", lpsSNIHost, lpsAPIServerPort, lpsPackagesPath)
	result := probeResult{label: label, url: url}
	start := time.Now()

	token, err := a.attestedToken(ctx)
	if err != nil {
		result.latency = time.Since(start)
		result.err = fmt.Errorf("imds attested token: %w", err)
		return result
	}

	client, caSource := a.lpsProbeClient(dialHost, provisionConfigPath)
	slog.Info("check-lps faithful probe TLS trust source", "probe", label, "caSource", caSource)

	ctx, cancel := context.WithTimeout(ctx, lpsProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.latency = time.Since(start)
		result.err = err
		return result
	}
	req.Header.Set("Authorization", token)

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

// attestedToken returns the IMDS attested-data signature used as the LPS Authorization
// token. The fetch is overridable for testing via App.fetchAttestedToken.
func (a *App) attestedToken(ctx context.Context) (string, error) {
	if a.fetchAttestedToken != nil {
		return a.fetchAttestedToken(ctx)
	}
	return fetchIMDSAttestedToken(ctx)
}

// fetchIMDSAttestedToken queries IMDS for the attested-data document and returns its
// signature, mirroring mariner-package-update.sh's custom-patching flow.
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

// lpsProbeClient builds the HTTP client for the faithful LPS probe: TLS ServerName pinned
// to lpsSNIHost, the TCP dial forced to dialHost:443, and RootCAs from the cluster CA in
// the provision config. It returns the client and a string describing the TLS trust source
// ("injected", "provision-config", or "insecure-skip-verify"). When App.httpProbeClient is
// set (tests), it is returned as-is.
func (a *App) lpsProbeClient(dialHost, provisionConfigPath string) (*http.Client, string) {
	if a.httpProbeClient != nil {
		return a.httpProbeClient, "injected"
	}

	tlsConfig := &tls.Config{ServerName: lpsSNIHost}
	caSource := "insecure-skip-verify"
	if pool := caPoolFromConfig(provisionConfigPath); pool != nil {
		tlsConfig.RootCAs = pool
		caSource = "provision-config"
	} else {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // CA unavailable pre-provision; fall back to reachability-only
	}

	dialAddr := net.JoinHostPort(dialHost, lpsAPIServerPort)
	dialer := &net.Dialer{Timeout: lpsProbeTimeout}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		// Force the connection to dialHost regardless of the SNI/Host (lpsSNIHost) in the
		// request URL — the curl --resolve equivalent.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dialAddr)
		},
	}
	return &http.Client{Timeout: lpsProbeTimeout, Transport: transport}, caSource
}

// caPoolFromConfig builds an x509 cert pool from the cluster CA carried in the provision
// config (Configuration.KubernetesCaCert, base64-encoded PEM). Returns nil when the path
// is empty/unreadable or the CA is missing/invalid, so callers can fall back gracefully.
// This is deliberately sourced from config rather than /etc/kubernetes/certs/ca.crt, which
// is only written during provision and is absent when check-lps runs pre-provision.
func caPoolFromConfig(provisionConfigPath string) *x509.CertPool {
	if provisionConfigPath == "" {
		return nil
	}
	data, err := os.ReadFile(provisionConfigPath)
	if err != nil {
		return nil
	}
	config, _ := nodeconfigutils.UnmarshalConfigurationV1(data)
	if config == nil {
		return nil
	}
	encoded := config.GetKubernetesCaCert()
	if encoded == "" {
		return nil
	}
	pemBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil
	}
	return pool
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
func (a *App) probeEndpoint(ctx context.Context, label, url string) probeResult {
	client := a.probeClient()

	ctx, cancel := context.WithTimeout(ctx, lpsProbeTimeout)
	defer cancel()

	result := probeResult{label: label, url: url}
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

// logProbeResult emits a structured log line describing a probe outcome.
func (a *App) logProbeResult(r probeResult) {
	if r.reachable {
		slog.Info("check-lps probe reachable",
			"probe", r.label,
			"url", r.url,
			"statusCode", r.statusCode,
			"latencyMs", r.latency.Milliseconds())
		return
	}
	slog.Info("check-lps probe unreachable",
		"probe", r.label,
		"url", r.url,
		"latencyMs", r.latency.Milliseconds(),
		"error", errString(r.err))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
