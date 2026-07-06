// Package common holds low-level primitives shared across the aks-node-controller commands.
//
// The HTTP helpers here are the reusable transport/retry mechanics for the check-hotfix
// network calls (the IMDS attested-token GET and the live-patching-service hotfix-pointer GET)
// and are intentionally domain-agnostic: callers supply their own timeouts, TLS trust, dial
// override, overall deadline, and retry policy. Endpoint identity and timeout tuning live with
// the caller, not here.
//
// TODO(provisioning-hotfix): when the check-lps connectivity client (lps.go) lands in main,
// have it consume NewBaseTransport too so there is a single canonical HTTP client.
package common

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HTTPTransportOptions configures the base transport built by NewBaseTransport.
type HTTPTransportOptions struct {
	// DialTimeout bounds the TCP connect.
	DialTimeout time.Duration
	// TLSHandshakeTimeout bounds the TLS handshake (ignored for plain-HTTP endpoints).
	TLSHandshakeTimeout time.Duration
	// ResponseHeaderTimeout bounds the wait for response headers after the request is written.
	ResponseHeaderTimeout time.Duration
	// TLSConfig is the TLS client config; nil for plain-HTTP endpoints (e.g. IMDS).
	TLSConfig *tls.Config
	// DialAddrOverride forces every dial to this host:port regardless of the URL host (the
	// curl --resolve equivalent used by the LPS SNI-pin path). Empty means dial the URL host.
	DialAddrOverride string
}

// NewBaseTransport builds an *http.Transport with fail-fast connect/handshake/response
// timeouts and proxying disabled. Proxying is disabled unconditionally: the LPS client forces
// its dial to the apiserver front (a proxy CONNECT would defeat the --resolve pin) and IMDS is
// a link-local endpoint that must never be proxied. Retry and overall-deadline policy are
// layered by callers.
func NewBaseTransport(opts HTTPTransportOptions) *http.Transport {
	dialer := &net.Dialer{Timeout: opts.DialTimeout}
	return &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       opts.TLSConfig,
		TLSHandshakeTimeout:   opts.TLSHandshakeTimeout,
		ResponseHeaderTimeout: opts.ResponseHeaderTimeout,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if opts.DialAddrOverride != "" {
				addr = opts.DialAddrOverride
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
}

// RetryStringFetch calls fn up to maxAttempts times and returns the first success. It stops
// early if the context is done, since a retry cannot then succeed. It does NOT sleep between
// attempts: each attempt is already bounded by its own timeout and the provisioning path wants
// fail-fast behavior. Used for the IMDS attested-token fetch (one quick retry).
func RetryStringFetch(ctx context.Context, maxAttempts int, fn func(context.Context) (string, error)) (string, error) {
	// Normalize to at least one attempt so a zero/negative maxAttempts cannot silently return
	// ("", nil) -- that would make a misconfiguration look like a successful empty fetch.
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		v, err := fn(ctx)
		if err == nil {
			return v, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
		if attempt < maxAttempts {
			slog.Warn("fetch attempt failed, retrying", "attempt", attempt, "maxAttempts", maxAttempts, "error", err)
		}
	}
	return "", lastErr
}
