package localdns

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
)

// fetchConfigEventTask is the guest agent event task for `localdns fetch-config`.
const fetchConfigEventTask = "FetchLocalDNSConfig"

// EventLogger is the part of helpers.EventLogger the commands use.
type EventLogger interface {
	LogEvent(taskName, message string, eventLevel helpers.EventLevel, startTime, endTime time.Time)
}

// FetchParams are the inputs to FetchConfig.
type FetchParams struct {
	// Fetch gets the localDNS payload from LPS.
	Fetch Fetcher
	// NotPublished reports whether a Fetch error means LPS has no localDNS config for this node,
	// which is the notFound outcome rather than a failure.
	NotPublished func(error) bool
	// Events receives the command's outcome event. Optional.
	Events EventLogger
	// OutputPath is where the Corefile is written; empty uses DefaultCorefilePath.
	OutputPath string
	Sources    Sources
	Stdout     io.Writer
}

// FetchConfig is `localdns fetch-config`: it fetches this node's config from LPS during bootstrap
// and updates the Corefile. It fails open: it always returns nil, so it never blocks provisioning.
// There is no Node object before the node joins, so the agent pool comes from the provisioning
// files. Reaching LPS needs those same files, so a node without them never gets this far.
func FetchConfig(ctx context.Context, p FetchParams) (err error) {
	slog.Info("aks-node-controller localdns fetch-config started", "outputPath", p.OutputPath)
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("localdns fetch-config panicked (fail-open)", "panic", r)
			if p.Events != nil {
				p.Events.LogEvent(fetchConfigEventTask,
					fmt.Sprintf("localdns fetch-config outcome=%s panic=%v", OutcomeFailed, r),
					helpers.EventLevelError, startTime, time.Now())
			}
			err = nil
		}
	}()

	result, err := fetchAndApply(ctx, p.Fetch, p.NotPublished, p.OutputPath, "", p.Sources)
	outcome := result.Outcome
	level := helpers.EventLevelInformational
	if outcome == OutcomeFailed {
		level = helpers.EventLevelError
	}
	message := fmt.Sprintf("localdns fetch-config outcome=%s", outcome)
	if err != nil {
		message = fmt.Sprintf("%s error=%s", message, err.Error())
		slog.Warn("localdns fetch-config completed with error (fail-open)", "outcome", outcome, "error", err)
	} else {
		slog.Info("localdns fetch-config completed", "outcome", outcome)
	}
	if p.Events != nil {
		p.Events.LogEvent(fetchConfigEventTask, message, level, startTime, time.Now())
	}
	// The bootstrap shell caller is intentionally fail-open, so it cannot use the
	// process exit status to distinguish a rendered config from a fallback. Keep
	// the machine-readable outcome as the final stdout line.
	for _, line := range result.Lines() {
		_, _ = fmt.Fprintln(p.Stdout, line)
	}
	return nil
}

// ApplyParams are the inputs to ApplyConfig.
type ApplyParams struct {
	// ConfigPath is the payload file; empty or "-" reads stdin.
	ConfigPath string
	// OutputPath is where the Corefile is written; empty uses DefaultCorefilePath.
	OutputPath string
	// AgentPool is the node's kubernetes.azure.com/agentpool label as the reconciler read it off the
	// Node object. It selects this node's entry on every provisioning mode, including classic CSE,
	// where neither provisioning file the agent pool is otherwise read from exists. Empty falls back
	// to those files.
	AgentPool string
	Sources   Sources
	Stdout    io.Writer
}

// ApplyConfig is `localdns apply-config`: it applies a payload the knead reconciler already has in
// hand at runtime. The outcome is always the last line of output, and on failure the reason is
// printed before it as "error: <reason>", since the CLI discards returned errors and the localDNS
// handler reports this reason as the component's message in live-patching-status.
func ApplyConfig(ctx context.Context, p ApplyParams) error {
	result := Result{Outcome: OutcomeFailed}
	config, err := readConfig(p.ConfigPath)
	if err == nil {
		result, err = fetchAndApply(ctx, func(context.Context) (string, error) {
			return config, nil
		}, nil, p.OutputPath, p.AgentPool, p.Sources)
	}
	if p.Stdout != nil {
		writeResult(p.Stdout, result, err)
	}
	return err
}
