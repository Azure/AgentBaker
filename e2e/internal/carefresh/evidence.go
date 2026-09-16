// Package carefresh validates independent evidence from disposable scenario nodes.
package carefresh

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxJitter = 5 * time.Minute
	// Allow acquisition/locking plus a 90s restart and two full CRI polls:
	// interrupted-process readiness may be checked before retrying recovery.
	RefreshTimeout = MaxJitter + 15*time.Minute
	HelperTimeout  = RefreshTimeout + time.Minute
	FixtureTimeout = 2*RefreshTimeout + 3*time.Minute
	HealthTimeout  = 2*HelperTimeout + 13*time.Minute
)

// SnapshotCommand only reads node state. In particular it does not source the
// script under test to select or hash the trust bundle, nor trust its readiness
// marker instead of checking the real CRI.
const SnapshotCommand = `
set -euo pipefail
bundle=/etc/ssl/certs/ca-certificates.crt
if [ ! -s "$bundle" ]; then bundle=/etc/pki/tls/certs/ca-bundle.crt; fi
test -s "$bundle"
printf 'BundlePath=%s\nBundleSHA256=' "$bundle"
sha256sum "$bundle" | cut -d' ' -f1
for service in kubelet containerd; do
  printf '[%s]\n' "$service"
  systemctl show "$service" --property=Id --property=ActiveState --property=SubState \
    --property=MainPID --property=ExecMainStartTimestampMonotonic --property=InvocationID --property=NRestarts
done
crictl --runtime-endpoint unix:///run/containerd/containerd.sock info |
  jq -e '.status.conditions | any(.type == "RuntimeReady" and .status == true)' >/dev/null
`

type ServiceIdentity struct {
	PID, Started, Restarts uint64
	InvocationID           string
}

type Snapshot struct {
	BundlePath, BundleSHA256 string
	Kubelet, Containerd      ServiceIdentity
}

var (
	hashPattern       = regexp.MustCompile(`^[a-f0-9]{64}$`)
	invocationPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

func ParseSnapshot(output string) (Snapshot, error) {
	sections := map[string]map[string]string{"": {}, "kubelet": {}, "containerd": {}}
	section := ""
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		if line == "[kubelet]" || line == "[containerd]" {
			section = strings.Trim(line, "[]")
			if seen[section] {
				return Snapshot{}, fmt.Errorf("duplicate service section %q", section)
			}
			seen[section] = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		_, duplicate := sections[section][key]
		if !ok || duplicate {
			return Snapshot{}, fmt.Errorf("invalid or duplicate snapshot property %q", key)
		}
		sections[section][key] = value
	}
	s := Snapshot{BundlePath: sections[""]["BundlePath"], BundleSHA256: sections[""]["BundleSHA256"]}
	if (s.BundlePath != "/etc/ssl/certs/ca-certificates.crt" && s.BundlePath != "/etc/pki/tls/certs/ca-bundle.crt") ||
		!hashPattern.MatchString(s.BundleSHA256) || len(sections[""]) != 2 {
		return Snapshot{}, fmt.Errorf("invalid independent trust bundle evidence")
	}
	var err error
	if s.Kubelet, err = parseService("kubelet", sections["kubelet"]); err != nil {
		return Snapshot{}, err
	}
	s.Containerd, err = parseService("containerd", sections["containerd"])
	return s, err
}

func parseService(name string, properties map[string]string) (ServiceIdentity, error) {
	s := ServiceIdentity{InvocationID: properties["InvocationID"]}
	if len(properties) != 7 || properties["Id"] != name+".service" ||
		properties["ActiveState"] != "active" || properties["SubState"] != "running" ||
		!invocationPattern.MatchString(s.InvocationID) || s.InvocationID == strings.Repeat("0", 32) {
		return s, fmt.Errorf("missing live %s service identity", name)
	}
	for key, dest := range map[string]*uint64{
		"MainPID": &s.PID, "ExecMainStartTimestampMonotonic": &s.Started, "NRestarts": &s.Restarts,
	} {
		n, err := strconv.ParseUint(properties[key], 10, 64)
		if err != nil || (key != "NRestarts" && n == 0) {
			return s, fmt.Errorf("invalid %s %s", name, key)
		}
		*dest = n
	}
	return s, nil
}

func Result(output string) (string, error) {
	result := ""
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "CA_REFRESH_RESULT=") {
			continue
		}
		if result != "" {
			return "", fmt.Errorf("multiple CA refresh result markers")
		}
		result = strings.TrimPrefix(line, "CA_REFRESH_RESULT=")
		switch result {
		case "unchanged", "restarted", "recovered", "inactive":
		default:
			return "", fmt.Errorf("invalid CA refresh result marker")
		}
	}
	if result == "" {
		return "", fmt.Errorf("missing CA refresh result marker")
	}
	return result, nil
}

// ValidateTransition is for healthy, already-running scenario nodes. Pending
// failure recovery is covered by production unit tests, not silently accepted
// as idempotence when these successful refreshes observe identical trust.
func ValidateTransition(before, after Snapshot, result string) error {
	if before.BundlePath != after.BundlePath {
		return fmt.Errorf("selected system trust bundle changed")
	}
	if before.Kubelet != after.Kubelet {
		return fmt.Errorf("kubelet restarted during CA refresh")
	}
	if before.BundleSHA256 == after.BundleSHA256 {
		if result != "unchanged" || before.Containerd != after.Containerd {
			return fmt.Errorf("unchanged trust requires unchanged containerd and result (got %q)", result)
		}
		return nil
	}
	if result != "restarted" && result != "recovered" {
		return fmt.Errorf("changed trust requires a recovered runtime instance (got %q)", result)
	}
	return ValidateRestart(before, after)
}

func ValidateRestart(before, after Snapshot) error {
	if before.Kubelet != after.Kubelet {
		return fmt.Errorf("kubelet restarted during CA refresh")
	}
	if before.Containerd.PID == after.Containerd.PID ||
		before.Containerd.Started >= after.Containerd.Started ||
		before.Containerd.InvocationID == after.Containerd.InvocationID {
		return fmt.Errorf("changed trust did not produce a new containerd PID/start/invocation")
	}
	if after.Containerd.Restarts != 0 {
		return fmt.Errorf("containerd unexpectedly auto-restarted while recovering")
	}
	return nil
}

const FixtureReportPrefix = "CA_FIXTURE_REPORT="

// The runner compares both reported states with its own node observations.
// Reporting the final identity lets it catch later, unauthorized restarts
// during survivor, DNS/HTTP and new-workload checks.
type FixtureReport struct {
	Before, After Snapshot
}

func ParseFixtureReport(output string) (FixtureReport, error) {
	var report FixtureReport
	found := false
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, FixtureReportPrefix) {
			continue
		}
		if found {
			return report, fmt.Errorf("duplicate fixture report")
		}
		found = true
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, FixtureReportPrefix)), &report); err != nil {
			return report, fmt.Errorf("invalid fixture report: %w", err)
		}
	}
	if !found || !hashPattern.MatchString(report.Before.BundleSHA256) || !hashPattern.MatchString(report.After.BundleSHA256) {
		return report, fmt.Errorf("missing fixture trust/service evidence")
	}
	return report, nil
}

func ValidateStable(before, after Snapshot) error {
	if before != after {
		return fmt.Errorf("unexpected trust/service change outside coordinated refresh: before=%+v after=%+v", before, after)
	}
	return nil
}
