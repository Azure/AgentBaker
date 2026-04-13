package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
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
// Returns an error if no tasks could be parsed, since an empty report would make
// regression detection ineffective.
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
	var parseErrors int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event cseEventJSON
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			parseErrors++
			s.T.Logf("WARNING: failed to unmarshal CSE event JSON: %v (line: %.100s)", err, line)
			continue
		}
		if event.TaskName == "" || event.Timestamp == "" || event.OperationId == "" {
			continue
		}

		startTime, err := parseCSETimestamp(event.Timestamp)
		if err != nil {
			parseErrors++
			s.T.Logf("WARNING: failed to parse CSE start timestamp for task %s: %v", event.TaskName, err)
			continue
		}
		endTime, err := parseCSETimestamp(event.OperationId)
		if err != nil {
			parseErrors++
			s.T.Logf("WARNING: failed to parse CSE end timestamp for task %s: %v", event.TaskName, err)
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

	if parseErrors > 0 {
		s.T.Logf("WARNING: %d CSE event parse errors encountered", parseErrors)
	}
	if len(report.Tasks) == 0 {
		return report, fmt.Errorf("no CSE task timings were parsed from %d event lines (%d parse errors)", len(lines), parseErrors)
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

// ValidateCSETimings extracts CSE task timings from the VM, logs them, and validates
// against thresholds. Each threshold check runs as a t.Run() sub-test so that ADO
// Pipeline Analytics (via gotestsum → JUnit XML → PublishTestResults) can track
// individual CSE task pass/fail and duration trends over time.
func ValidateCSETimings(ctx context.Context, s *Scenario, thresholds CSETimingThresholds) *CSETimingReport {
	s.T.Helper()
	defer toolkit.LogStep(s.T, "validating CSE task timings")()

	// Type-assert to *testing.T so we can use t.Run() for sub-tests.
	// This is safe: E2E scenarios always run under *testing.T.
	tRunner, ok := s.T.(*testing.T)
	if !ok {
		s.T.Fatalf("ValidateCSETimings requires *testing.T for sub-test support, got %T", s.T)
	}

	report, err := ExtractCSETimings(ctx, s)
	if err != nil {
		s.T.Fatalf("failed to extract CSE timings: %v", err)
	}

	// Always log the full timing report
	report.LogReport(ctx, s.T)

	// Fail if no tasks were parsed — an empty report makes regression detection ineffective
	if len(report.Tasks) == 0 {
		s.T.Fatalf("no CSE task timings were parsed; cannot validate performance thresholds")
	}

	// Fail if the critical cse_start task is missing
	if report.GetTask("AKS.CSE.cse_start") == nil {
		s.T.Errorf("expected AKS.CSE.cse_start task not found in timing report")
	}

	// Validate total CSE duration as a sub-test for ADO tracking
	if thresholds.TotalCSEThreshold > 0 {
		tRunner.Run("TotalCSEDuration", func(t *testing.T) {
			totalDuration := report.TotalCSEDuration()
			t.Logf("total CSE duration: %s (threshold: %s)", totalDuration, thresholds.TotalCSEThreshold)
			if totalDuration > thresholds.TotalCSEThreshold {
				toolkit.LogDuration(ctx, totalDuration, thresholds.TotalCSEThreshold,
					fmt.Sprintf("CSE total duration %s exceeds threshold %s", totalDuration, thresholds.TotalCSEThreshold))
				t.Errorf("CSE total duration %s exceeds threshold %s", totalDuration, thresholds.TotalCSEThreshold)
			}
		})
	}

	// Validate individual task thresholds — each as a sub-test for ADO tracking.
	// ADO Test Analytics will show per-task pass/fail trends and flag regressions.
	for _, task := range report.Tasks {
		for suffix, maxDuration := range thresholds.TaskThresholds {
			if strings.HasSuffix(task.TaskName, suffix) {
				task := task
				suffix := suffix
				maxDuration := maxDuration
				tRunner.Run(fmt.Sprintf("Task_%s", suffix), func(t *testing.T) {
					t.Logf("task %s duration: %s (threshold: %s)", task.TaskName, task.Duration, maxDuration)
					if task.Duration > maxDuration {
						toolkit.LogDuration(ctx, task.Duration, maxDuration,
							fmt.Sprintf("CSE task %s took %s (threshold: %s)", task.TaskName, task.Duration, maxDuration))
						t.Errorf("CSE task %s took %s, exceeds threshold %s", task.TaskName, task.Duration, maxDuration)
					}
				})
				break
			}
		}
	}

	return report
}
