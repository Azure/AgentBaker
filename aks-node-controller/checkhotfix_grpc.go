package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"

	lpsv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// check-hotfix gRPC transport.
//
// The live-patching service is a gRPC server exposing a single GetComponentConfig RPC that returns
// an opaque, per-component config blob. check-hotfix requests the ANC component config, maps the
// gRPC status codes onto the benign-vs-fatal taxonomy, and preserves the fail-open contract.

const (
	// ancComponentName is the component name check-hotfix requests from the live-patching service.
	// It must match, byte for byte, the component key the service registers for aks-node-controller.
	// The service side registers this component with the lowerCamelCase key "aksNodeController"
	// (matching its existing "securityPatch"/"localDNS" components), so the consumer requests the
	// same string here.
	ancComponentName = "aksNodeController"

	// lpsAttestedMetadataKey is the gRPC request-metadata key carrying the IMDS attested-data
	// signature that authenticates the node to the live-patching service. The service reads it from
	// the "authorization" metadata header (per the live-patching client example / contract),
	// mirroring the embargoed HTTP service's header-based auth.
	lpsAttestedMetadataKey = "authorization"
)

// lpsGRPCStatusError is a non-benign gRPC failure from the live-patching service. fallbackAllowed
// mirrors the HTTP path's 5xx-vs-4xx split: a transient/unavailable/internal server condition
// permits the cold-start fallback, while an authoritative client rejection does not.
type lpsGRPCStatusError struct {
	code            codes.Code
	fallbackAllowed bool
}

func (e *lpsGRPCStatusError) Error() string {
	return fmt.Sprintf("LPS gRPC call failed with code %s", e.code)
}

// fetchHotfixOverGRPC performs the GetComponentConfig call against the live-patching service: it
// resolves the apiserver FQDN + cluster CA from the available node bootstrap input, carries the
// IMDS attested-data document in gRPC metadata, and returns the opaque response bytes for the
// shared parse/stage path.
// The gRPC status is mapped onto the benign-vs-fatal taxonomy so handleFetchError is unchanged.
func (a *App) fetchHotfixOverGRPC(ctx context.Context) ([]byte, error) {
	fqdn, caPEM, err := a.lpsTargetFromNodeConfig()
	if err != nil {
		return nil, fmt.Errorf("resolving LPS endpoint: %w", err)
	}

	token, err := a.attestedToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("imds attested token: %w", err)
	}

	conn, err := a.dialLPSGRPC(fqdn, caPEM)
	if err != nil {
		return nil, fmt.Errorf("dialing LPS gRPC: %w", err)
	}
	defer conn.Close()
	slog.Info("check-hotfix LPS gRPC dial", "dialHost", fqdn, "alpn", lpsALPNProto, "component", ancComponentName)

	// Bound the whole round-trip (cold connect + RPC); on expiry we fail open to the cold-start
	// pointer. See the lpsFetchTimeout const doc for the deadline tradeoff.
	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	// Carry the attested-data document that authenticates the node in request metadata.
	ctx = metadata.AppendToOutgoingContext(ctx, lpsAttestedMetadataKey, token)

	client := lpsv1.NewLivePatchingServiceClient(conn)
	resp, err := client.GetComponentConfig(ctx, &lpsv1.GetComponentConfigRequest{ComponentName: ancComponentName})
	if err != nil {
		return nil, mapGRPCError(err)
	}
	// The shared live-patching contract carries the component config as a JSON-encoded UTF-8
	// string; the parse/stage path operates on bytes, so convert without interpreting.
	return []byte(resp.GetConfig()), nil
}

// dialLPSGRPC builds the gRPC client connection to the live-patching service: it dials the cluster
// apiserver FQDN:443 (riding the existing apiserver egress rule) and advertises the live-patching
// ALPN protocol so the kube-api-proxy envoy routes the stream to the LPS backend. The LPS server
// presents a certificate issued for the same apiserver FQDN, so grpc-go's standard TLS verification
// validates both the cluster CA chain and the dialed hostname. The connection is lazy; the per-RPC
// context deadline bounds connect + call. Tests inject grpcDialContext to reach an in-process
// (bufconn) server.
//
// The cluster CA is REQUIRED: without it the server certificate cannot be verified, so rather than
// weaken TLS we return an error and the caller fails open (nothing staged).
func (a *App) dialLPSGRPC(fqdn string, caPEM []byte) (*grpc.ClientConn, error) {
	if a.grpcDialContext != nil {
		return grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(a.grpcDialContext),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if len(caPEM) == 0 {
		return nil, fmt.Errorf("cluster CA unavailable from provision-config; refusing to fetch over unverified TLS")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse cluster CA PEM")
	}

	// api_server_name may already carry a port (e.g. "host:443"); normalize to a bare hostname for
	// JoinHostPort so it does not produce an invalid "[host:443]:443" address.
	host := fqdn
	if h, _, splitErr := net.SplitHostPort(fqdn); splitErr == nil {
		host = h
	}

	// Advertise the live-patching ALPN protocol (the value envoy's kube-api-proxy filter chain
	// matches to route the stream to LPS) plus "h2" for the gRPC HTTP/2 handshake. grpc-go derives
	// the TLS server name from target, so the certificate is verified against the apiserver FQDN.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
		NextProtos: []string{lpsALPNProto, alpnH2Proto},
	}

	target := net.JoinHostPort(host, lpsAPIServerPort)
	return grpc.NewClient(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		// The API server endpoint is already an explicitly trusted cluster address. Bypass
		// environment proxies so private-connect IPs do not require a matching NO_PROXY entry.
		grpc.WithNoProxy(),
	)
}

// mapGRPCError classifies a gRPC error from GetComponentConfig into the existing check-hotfix
// taxonomy:
//   - Unauthenticated/PermissionDenied/NotFound -> benign errLPSUnavailable (the service is
//     reachable but has nothing published for this node yet; a no-op, never a failure);
//   - InvalidArgument/ResourceExhausted -> authoritative client rejection, no cold-start fallback;
//   - everything else (Unavailable/DeadlineExceeded/Internal/Unknown, and non-status transport
//     errors) -> fallback-eligible, so the cold-start pointer may be staged.
func mapGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok || st == nil {
		// Not a gRPC status (e.g. a raw transport error): unreachable/unknown, so cold-start
		// fallback is allowed (shouldColdStartFallback default).
		return err
	}
	//exhaustive:ignore // benign/authoritative codes handled explicitly; all others fall through to fallback-eligible default.
	switch st.Code() {
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return fmt.Errorf("%w (code %s)", errLPSUnavailable, st.Code())
	case codes.InvalidArgument, codes.ResourceExhausted:
		return &lpsGRPCStatusError{code: st.Code(), fallbackAllowed: false}
	default:
		return &lpsGRPCStatusError{code: st.Code(), fallbackAllowed: true}
	}
}
