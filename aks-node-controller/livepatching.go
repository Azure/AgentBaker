package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// livepatching is a generic, component-agnostic client for the live-patching service.
//
// It is the plain transport counterpart to check-hotfix: both call the same GetComponentConfig
// RPC, but where check-hotfix pins the component to aksNodeController and layers domain policy on
// top (pointer parsing, cold-start fallback, staging, and a fail-open exit-0 contract), this
// command takes the component name from the caller, writes the returned config verbatim to stdout,
// and reports failure through a non-zero exit code.
//
// That difference is deliberate: check-hotfix runs inside provisioning, where blocking the node is
// worse than missing a hotfix, so it must never fail. livepatching is an operator/diagnostic and
// scripting entrypoint for any component (securityPatch, localDNS, ...), where silently printing
// nothing and exiting 0 would be indistinguishable from an empty config.

// runLivePatchingCommand fetches the live-patching config for component and writes it to out.
//
// The config is written verbatim, with no trailing newline added, so consumers can pipe it
// straight into a JSON parser. An empty config is written as nothing and is not an error: the
// service may legitimately publish an empty blob for a registered component.
//
// Unlike check-hotfix this is NOT fail-open. Every failure -- including the benign-for-hotfix
// codes that mapGRPCError folds into errLPSUnavailable -- is returned to the caller and becomes a
// non-zero exit code, because a caller asking for one component's config needs to distinguish
// "the service published nothing for you" from "here is the config".
func (a *App) runLivePatchingCommand(ctx context.Context, out io.Writer, component string) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return fmt.Errorf("livepatching requires a component argument")
	}

	slog.Info("aks-node-controller livepatching started", "component", component)

	config, err := a.fetchComponentConfig(ctx, component)
	if err != nil {
		slog.Error("aks-node-controller livepatching failed", "component", component, "error", err)
		return fmt.Errorf("fetching live-patching config for component %q: %w", component, err)
	}

	if _, werr := out.Write(config); werr != nil {
		return fmt.Errorf("writing live-patching config for component %q: %w", component, werr)
	}

	slog.Info("aks-node-controller livepatching finished", "component", component, "bytes", len(config))
	return nil
}

// fetchComponentConfig returns the raw live-patching config bytes for component. Tests inject
// livePatchingFetcher to supply a canned config or error without networking; otherwise it goes out
// over the shared gRPC transport.
func (a *App) fetchComponentConfig(ctx context.Context, component string) ([]byte, error) {
	if a.livePatchingFetcher != nil {
		return a.livePatchingFetcher(ctx, component)
	}
	return a.fetchComponentConfigOverGRPC(ctx, component, "livepatching")
}
