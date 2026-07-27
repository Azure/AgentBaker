package helpers

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// kubeletActiveFlagsTaskName is the event TaskName for the kubelet active flags telemetry.
	// Scriptless (aks-node-controller) and shell CSE use different TaskName values; keep this in sync
	// with the query patterns documented in the PR description.
	kubeletActiveFlagsTaskName = "ensureKubelet.kubeletActiveFlags"

	// kubeletFlagsPollMaxAttempts is how many times to poll journalctl for FLAG lines.
	kubeletFlagsPollMaxAttempts = 10

	// kubeletFlagsPollInterval is the sleep between poll attempts.
	kubeletFlagsPollInterval = 2 * time.Second

	// maxMessageBytes is the Context1 size cap in GuestAgentGenericLogs (3 KiB hard truncation
	// verified empirically; we cap at 3000 to leave margin).
	maxMessageBytes = 3000

	// kubeletConfigFilePath is the default path where AgentBaker writes the KubeletConfiguration.
	kubeletConfigFilePath = "/etc/default/kubeletconfig.json"
)

// KubeletActiveFlagsPayload is the JSON structure emitted as the event Message.
type KubeletActiveFlagsPayload struct {
	Found          bool              `json:"found"`
	UsesConfigFile bool              `json:"uses_config_file"`
	ConfigPath     string            `json:"config_path"`
	FlagCount      int               `json:"flag_count"`
	Flags          map[string]string `json:"flags"`
	ConfigFile     any               `json:"config_file"`
}

// getKubeletTrackedFlags returns the curated set of CLI flags to surface individually.
func getKubeletTrackedFlags() []string {
	return []string{
		"config", "cgroup-driver", "kube-reserved", "kube-reserved-cgroup",
		"system-reserved", "enforce-node-allocatable", "max-pods",
		"rotate-certificates", "rotate-server-certificates", "tls-cipher-suites",
		"container-runtime-endpoint", "pod-infra-container-image", "resolv-conf",
		"feature-gates", "cloud-provider", "protect-kernel-defaults",
		"streaming-connection-idle-timeout", "node-status-update-frequency",
		"image-gc-high-threshold", "image-gc-low-threshold", "eviction-hard",
	}
}

// EmitKubeletActiveFlagsEvent reads kubelet's active startup flags from journalctl,
// reads the config file if present, and emits a structured guest agent event.
// Best-effort: never returns an error that should block provisioning.
func (l *EventLogger) EmitKubeletActiveFlagsEvent() {
	payload := buildKubeletFlagsPayload()
	messageBytes := marshalWithSizeGuard(payload)
	if messageBytes == nil {
		return
	}
	now := time.Now()
	l.LogEvent(kubeletActiveFlagsTaskName, string(messageBytes), EventLevelInformational, now, now)
}

// buildKubeletFlagsPayload assembles the payload from journalctl FLAG lines and config file.
func buildKubeletFlagsPayload() KubeletActiveFlagsPayload {
	flags := pollKubeletFlags()
	payload := KubeletActiveFlagsPayload{
		Flags:      make(map[string]string),
		ConfigFile: map[string]any{},
	}
	if len(flags) == 0 {
		return payload
	}

	payload.Found = true
	payload.FlagCount = len(flags)

	if configVal, ok := flags["config"]; ok && configVal != "" {
		payload.UsesConfigFile = true
		payload.ConfigPath = configVal
	}

	for _, name := range getKubeletTrackedFlags() {
		if val, ok := flags[name]; ok {
			payload.Flags[name] = val
		}
	}

	payload.ConfigFile = readKubeletConfigFile(payload.ConfigPath, payload.UsesConfigFile)
	return payload
}

// readKubeletConfigFile reads and parses the kubelet config JSON file.
func readKubeletConfigFile(configPath string, usesConfigFile bool) any {
	if !usesConfigFile {
		return map[string]any{}
	}
	if configPath == "" {
		configPath = kubeletConfigFilePath
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]any{}
	}
	var content any
	if json.Unmarshal(data, &content) != nil {
		return map[string]any{}
	}
	return content
}

// marshalWithSizeGuard marshals the payload, dropping config_file if it exceeds the size cap.
func marshalWithSizeGuard(payload KubeletActiveFlagsPayload) []byte {
	messageBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal kubelet active flags payload", "error", err)
		return nil
	}
	if len(messageBytes) <= maxMessageBytes {
		return messageBytes
	}

	slog.Warn("kubelet active flags payload exceeded size cap, dropping config_file",
		"size", len(messageBytes), "cap", maxMessageBytes)
	payload.ConfigFile = "truncated:exceeded_size_cap"
	messageBytes, err = json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal truncated kubelet active flags payload", "error", err)
		return nil
	}
	return messageBytes
}

// pollKubeletFlags polls journalctl for kubelet's FLAG lines, retrying until they appear.
// Returns a map of flag-name -> value.
func pollKubeletFlags() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(kubeletFlagsPollMaxAttempts)*kubeletFlagsPollInterval)
	defer cancel()
	for i := 0; i < kubeletFlagsPollMaxAttempts; i++ {
		flags := readKubeletFlagsFromJournal(ctx)
		if len(flags) > 0 {
			return flags
		}
		if ctx.Err() != nil {
			break
		}
		time.Sleep(kubeletFlagsPollInterval)
	}
	return nil
}

// readKubeletFlagsFromJournal runs journalctl and parses FLAG lines.
func readKubeletFlagsFromJournal(ctx context.Context) map[string]string {
	// #nosec G204 -- fixed command, no user input
	cmd := exec.CommandContext(ctx, "journalctl", "-u", "kubelet", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	flags := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		idx := strings.Index(line, "FLAG: --")
		if idx < 0 {
			continue
		}
		flagStr := line[idx+len("FLAG: --"):]
		// flagStr is like: name="value"
		name, value, found := strings.Cut(flagStr, "=")
		if !found {
			continue
		}
		// Strip surrounding quotes
		value = strings.Trim(value, "\"")
		// De-duplicate: keep first occurrence
		if _, exists := flags[name]; !exists {
			flags[name] = value
		}
	}
	return flags
}
