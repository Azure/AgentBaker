package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/toolkit"
)

const (
	// cseEventsDir is the directory where CSE task timing events are stored on the VM.
	cseEventsDir = "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
	// provisionJSONPath is the path to the provision.json file with overall boot timing.
	provisionJSONPath = "/var/log/azure/aks/provision.json"
)

// CSETaskTiming represents the timing of a single CSE task.
type CSETaskTiming struct {
	TaskName  string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Message   string
}

// CSEProvisionTiming represents the overall provisioning timing from provision.json.
type CSEProvisionTiming struct {
	ExitCode              string `json:"ExitCode"`
	ExecDuration          string `json:"ExecDuration"`
	KernelStartTime       string `json:"KernelStartTime"`
	CloudInitLocalStart   string `json:"CloudInitLocalStartTime"`
	CloudInitStart        string `json:"CloudInitStartTime"`
	CloudFinalStart       string `json:"CloudFinalStartTime"`
	CSEStartTime          string `json:"CSEStartTime"`
	GuestAgentStartTime   string `json:"GuestAgentStartTime"`
	SystemdSummary        string `json:"SystemdSummary"`
	BootDatapoints        json.RawMessage `json:"BootDatapoints"`
}

// CSETimingReport holds all parsed timing data from a VM.
type CSETimingReport struct {
	Tasks      []CSETaskTiming
	Provision  *CSEProvisionTiming
	taskIndex  map[string]*CSETaskTiming
}

// cseEventJSON matches the JSON structure written by logs_to_events.
type cseEventJSON struct {
	Timestamp   string `json:"Timestamp"`
	OperationId string `json:"OperationId"`
	TaskName    string `json:"TaskName"`
	EventLevel  string `json:"EventLevel"`
	Message     string `json:"Message"`
}

// GetTask returns the timing for a specific task, or nil if not found.
func (r *CSETimingReport) GetTask(name string) *CSETaskTiming {
	if r.taskIndex == nil {
		r.taskIndex = make(map[string]*CSETaskTiming, len(r.Tasks))
		for i := range r.Tasks {
			r.taskIndex[r.Tasks[i].TaskName] = &r.Tasks[i]
		}
	}
	return r.taskIndex[name]
}

// TotalCSEDuration returns the duration of the cse_start task if present.
func (r *CSETimingReport) TotalCSEDuration() time.Duration {
	if t := r.GetTask("AKS.CSE.cse_start"); t != nil {
		return t.Duration
	}
	return 0
}

// LogReport logs all task timings to the test logger.
func (r *CSETimingReport) LogReport(ctx context.Context, t interface{ Logf(string, ...any) }) {
	t.Logf("=== CSE Task Timing Report ===")
	t.Logf("%-60s %12s %12s", "Task", "Duration", "Start→End")
	t.Logf("%s", strings.Repeat("-", 90))

	sorted := make([]CSETaskTiming, len(r.Tasks))
	copy(sorted, r.Tasks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.Before(sorted[j].StartTime)
	})

	for _, task := range sorted {
		t.Logf("%-60s %10.2fs   %s → %s",
			task.TaskName,
			task.Duration.Seconds(),
			task.StartTime.Format("15:04:05.000"),
			task.EndTime.Format("15:04:05.000"),
		)
	}

	if total := r.TotalCSEDuration(); total > 0 {
		t.Logf("%s", strings.Repeat("-", 90))
		t.Logf("%-60s %10.2fs", "TOTAL (cse_start)", total.Seconds())
	}

	if r.Provision != nil {
		t.Logf("\n=== Provision Summary ===")
		t.Logf("ExitCode: %s, ExecDuration: %ss", r.Provision.ExitCode, r.Provision.ExecDuration)
		t.Logf("KernelStart: %s, CSEStart: %s, GuestAgent: %s",
			r.Provision.KernelStartTime, r.Provision.CSEStartTime, r.Provision.GuestAgentStartTime)
	}
}

// ExtractCSETimings SSHes into the scenario VM and extracts all CSE task timings.
func ExtractCSETimings(ctx context.Context, s *Scenario) (*CSETimingReport, error) {
	report := &CSETimingReport{}

	// Read all event JSON files from the CSE events directory
	listCmd := fmt.Sprintf("sudo find %s -name '*.json' -exec cat {} \\;", cseEventsDir)
	result, err := execScriptOnVm(ctx, s, s.Runtime.VM, listCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSE events: %w", err)
	}

	// Each line is a separate JSON object (one per event file)
	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event cseEventJSON
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.TaskName == "" || event.Timestamp == "" || event.OperationId == "" {
			continue
		}

		startTime, err := parseCSETimestamp(event.Timestamp)
		if err != nil {
			continue
		}
		endTime, err := parseCSETimestamp(event.OperationId)
		if err != nil {
			continue
		}

		report.Tasks = append(report.Tasks, CSETaskTiming{
			TaskName:  event.TaskName,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
			Message:   event.Message,
		})
	}

	// Read provision.json for overall boot timing
	provResult, err := execScriptOnVm(ctx, s, s.Runtime.VM, fmt.Sprintf("sudo cat %s", provisionJSONPath))
	if err == nil && provResult.stdout != "" {
		var prov CSEProvisionTiming
		if json.Unmarshal([]byte(strings.TrimSpace(provResult.stdout)), &prov) == nil {
			report.Provision = &prov
		}
	}

	return report, nil
}

// parseCSETimestamp parses the timestamp format used by logs_to_events: "YYYY-MM-DD HH:MM:SS.mmm"
func parseCSETimestamp(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse CSE timestamp %q", s)
}

// CSETimingThresholds defines maximum acceptable durations for CSE tasks.
type CSETimingThresholds struct {
	// TaskThresholds maps task name suffixes to maximum duration.
	// Task names are matched by suffix to allow flexible matching
	// (e.g., "installDebPackageFromFile" matches "AKS.CSE.installkubelet.installDebPackageFromFile").
	TaskThresholds map[string]time.Duration

	// TotalCSEThreshold is the maximum acceptable total CSE duration.
	TotalCSEThreshold time.Duration
}

// ValidateCSETimings extracts CSE task timings from the VM, logs them, and validates against thresholds.
func ValidateCSETimings(ctx context.Context, s *Scenario, thresholds CSETimingThresholds) *CSETimingReport {
	s.T.Helper()
	defer toolkit.LogStep(s.T, "validating CSE task timings")()

	report, err := ExtractCSETimings(ctx, s)
	if err != nil {
		s.T.Fatalf("failed to extract CSE timings: %v", err)
	}

	// Always log the full timing report
	report.LogReport(ctx, s.T)

	// Validate total CSE duration
	if thresholds.TotalCSEThreshold > 0 {
		totalDuration := report.TotalCSEDuration()
		if totalDuration > thresholds.TotalCSEThreshold {
			toolkit.LogDuration(ctx, totalDuration, thresholds.TotalCSEThreshold,
				fmt.Sprintf("CSE total duration %s exceeds threshold %s", totalDuration, thresholds.TotalCSEThreshold))
			s.T.Errorf("CSE total duration %s exceeds threshold %s", totalDuration, thresholds.TotalCSEThreshold)
		}
	}

	// Validate individual task thresholds
	for _, task := range report.Tasks {
		for suffix, maxDuration := range thresholds.TaskThresholds {
			if strings.HasSuffix(task.TaskName, suffix) {
				if task.Duration > maxDuration {
					toolkit.LogDuration(ctx, task.Duration, maxDuration,
						fmt.Sprintf("CSE task %s took %s (threshold: %s)", task.TaskName, task.Duration, maxDuration))
					s.T.Errorf("CSE task %s took %s, exceeds threshold %s", task.TaskName, task.Duration, maxDuration)
				}
				break
			}
		}
	}

	return report
}
