package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	akslivepatchingv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/internal/localdns"
	"google.golang.org/grpc/metadata"
)

// runApplyLocalDNSConfigCommand applies a payload the reconciler already has in hand. agentPool is
// the node's kubernetes.azure.com/agentpool label as the reconciler read it off the Node object; it
// selects this node's entry on every provisioning mode, including classic CSE, where neither
// provisioning file the agent pool is otherwise read from exists. Empty falls back to those files.
//
// Run discards returned errors, so the reason is also printed: see localdns.WriteResult.
func (a *App) runApplyLocalDNSConfigCommand(ctx context.Context, configPath, outputPath, agentPool string, writer io.Writer) error {
	result := localdns.Result{Outcome: localdns.OutcomeFailed}
	config, err := localdns.ReadConfig(configPath)
	if err == nil {
		result, err = localdns.Apply(ctx, func(context.Context) (string, error) {
			return config, nil
		}, outputPath, agentPool, a.localDNSSources())
	}
	if writer != nil {
		localdns.WriteResult(writer, result, err)
	}
	return err
}

func (a *App) runFetchLocalDNSConfigCommand(ctx context.Context, outputPath string) (err error) {
	slog.Info("aks-node-controller localdns fetch-config started", "outputPath", outputPath)
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("localdns fetch-config panicked (fail-open)", "panic", r)
			if a.eventLogger != nil {
				a.eventLogger.LogEvent("FetchLocalDNSConfig",
					fmt.Sprintf("localdns fetch-config outcome=%s panic=%v", localdns.OutcomeFailed, r),
					helpers.EventLevelError, startTime, time.Now())
			}
			err = nil
		}
	}()

	// There is no Node object before the node joins, so the agent pool comes from the provisioning
	// files. Reaching LPS needs those same files, so a node without them never gets this far.
	result, err := localdns.Apply(ctx, a.fetchLocalDNSConfig, outputPath, "", a.localDNSSources())
	outcome := result.Outcome
	level := helpers.EventLevelInformational
	if outcome == localdns.OutcomeFailed {
		level = helpers.EventLevelError
	}
	message := fmt.Sprintf("localdns fetch-config outcome=%s", outcome)
	if err != nil {
		message = fmt.Sprintf("%s error=%s", message, err.Error())
		slog.Warn("localdns fetch-config completed with error (fail-open)", "outcome", outcome, "error", err)
	} else {
		slog.Info("localdns fetch-config completed", "outcome", outcome)
	}
	if a.eventLogger != nil {
		a.eventLogger.LogEvent("FetchLocalDNSConfig", message, level, startTime, time.Now())
	}
	// The bootstrap shell caller is intentionally fail-open, so it cannot use the
	// process exit status to distinguish a rendered config from a fallback. Keep
	// the machine-readable outcome as the final stdout line.
	for _, line := range result.Lines() {
		fmt.Fprintln(os.Stdout, line)
	}
	return nil
}

// localDNSSources points the render at this node's provisioning files, through the same path
// overrides check-hotfix uses.
func (a *App) localDNSSources() localdns.Sources {
	return localdns.Sources{
		NodeConfigPath:    a.getNodeConfigPath(),
		BakedCorefilePath: a.localDNSBakedCorefilePath,
		HostsFilePath:     a.localDNSHostsFilePath,
		NBCCmdVars: func() (map[string]string, string, error) {
			path := a.getNBCCmdPath()
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, path, fmt.Errorf("reading nbc-cmd %s: %w", path, err)
			}
			return parseEnvVarsFromNBCCmdContent(string(raw)), path, nil
		},
	}
}

// fetchLocalDNSConfig fetches the payload from LPS. LPS answering that it has nothing for this node
// is reported as localdns.ErrNotPublished, the notFound outcome.
func (a *App) fetchLocalDNSConfig(ctx context.Context) (string, error) {
	fetch := a.fetchLocalDNSConfigFromLPS
	if a.fetchLocalDNSConfigFn != nil {
		fetch = a.fetchLocalDNSConfigFn
	}
	config, err := fetch(ctx)
	if isLPSUnavailable(err) {
		return "", fmt.Errorf("%w: %w", localdns.ErrNotPublished, err)
	}
	return config, err
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
	ctx, cancel := context.WithTimeout(ctx, lpsFetchTimeout)
	defer cancel()

	// Share the check-hotfix dialer rather than building a second TLS config: it targets the same
	// apiserver FQDN with the same ALPN protocols and the same cluster CA, so LPS presents a
	// certificate issued for the dialed hostname and grpc-go's standard verification covers both
	// the chain and the hostname. Reusing it also picks up grpc.WithNoProxy(), which this path was
	// missing, and keeps the two LPS clients from drifting.
	conn, err := a.dialLPSGRPC(fqdn, caPEM)
	if err != nil {
		return "", fmt.Errorf("creating LPS client: %w", err)
	}
	defer conn.Close()

	client := akslivepatchingv1.NewLivePatchingServiceClient(conn)
	rpcCtx := metadata.AppendToOutgoingContext(ctx, "authorization", token)
	resp, err := client.GetComponentConfig(rpcCtx, &akslivepatchingv1.GetComponentConfigRequest{
		ComponentName: localdns.ComponentName,
	})
	if err != nil {
		// mapGRPCError is the shared LPS classifier: it folds NotFound/PermissionDenied/
		// Unauthenticated into errLPSUnavailable -- which fetchLocalDNSConfig tests via
		// isLPSUnavailable -- while keeping the gRPC code and message, and passes non-status
		// errors through untouched. %w keeps that chain intact through this wrap.
		return "", fmt.Errorf("get %s component config: %w",
			localdns.ComponentName, mapGRPCError(err))
	}
	return resp.GetConfig(), nil
}
