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
	ExitCode            string          `json:"ExitCode"`
	ExecDuration        string          `json:"ExecDuration"`
	KernelStartTime     string          `json:"KernelStartTime"`
	CloudInitLocalStart string          `json:"CloudInitLocalStartTime"`
	CloudInitStart      string          `json:"CloudInitStartTime"`
	CloudFinalStart     string          `json:"CloudFinalStartTime"`
	CSEStartTime        string          `json:"CSEStartTime"`
	GuestAgentStartTime string          `json:"GuestAgentStartTime"`
	SystemdSummary      string          `json:"SystemdSummary"`
	BootDatapoints      json.RawMessage `json:"BootDatapoints"`
}

// CSETimingReport holds all parsed timing data from a VM.
type CSETimingReport struct {
	Tasks     []CSETaskTiming
	Provision *CSEProvisionTiming
	taskIndex map[string]*CSETaskTiming
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
func (r *CSETimingReport) LogReport(_ context.Context, t interface{ Logf(string, ...any) }) {
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

	result, err := execScriptOnVm(ctx, s, s.Runtime.VM, "sudo cat /var/log/azure/cluster-provision.log")
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster-provision.log: %w", err)
	}

	var parseErrors int
	for _, line := range strings.Split(result.stdout, "\n") {
		if !strings.Contains(line, " echo ") ||
			!strings.Contains(line, `"TaskName"`) ||
			!strings.Contains(line, "AKS.CSE.") {
			continue
		}

		// Bash xtrace prints each word as a separately quoted shell argument:
		// + echo '{' '"Timestamp":' '"2026-07-17' '02:22:57.206",' ...
		// Removing those trace-only single quotes reconstructs the JSON fields.
		normalized := strings.ReplaceAll(line, "'", "")
		startTimestamp := extractXtraceJSONField(normalized, "Timestamp")
		endTimestamp := extractXtraceJSONField(normalized, "OperationId")
		taskName := extractXtraceJSONField(normalized, "TaskName")
		if startTimestamp == "" || endTimestamp == "" || !strings.HasPrefix(taskName, "AKS.CSE.") {
			parseErrors++
			continue
		}

		startTime, err := parseCSETimestamp(startTimestamp)
		if err != nil {
			parseErrors++
			s.T.Logf("WARNING: failed to parse CSE start timestamp for task %s: %v", taskName, err)
			continue
		}
		endTime, err := parseCSETimestamp(endTimestamp)
		if err != nil {
			parseErrors++
			s.T.Logf("WARNING: failed to parse CSE end timestamp for task %s: %v", taskName, err)
			continue
		}

		report.Tasks = append(report.Tasks, CSETaskTiming{
			TaskName:  taskName,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
		})
	}

	if parseErrors > 0 {
		s.T.Logf("WARNING: %d CSE timing lines in cluster-provision.log could not be parsed", parseErrors)
	}
	if len(report.Tasks) == 0 {
		return report, fmt.Errorf("no CSE task timings were parsed from cluster-provision.log (%d parse errors)", parseErrors)
	}

	provResult, err := execScriptOnVm(ctx, s, s.Runtime.VM, fmt.Sprintf("sudo cat %s", provisionJSONPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", provisionJSONPath, err)
	}

	var prov CSEProvisionTiming
	if err := json.Unmarshal([]byte(strings.TrimSpace(provResult.stdout)), &prov); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", provisionJSONPath, err)
	}
	report.Provision = &prov

	cseStart, err := parseProvisionTimestamp(prov.CSEStartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSEStartTime from %s: %w", provisionJSONPath, err)
	}
	execDuration, err := time.ParseDuration(prov.ExecDuration + "s")
	if err != nil {
		return nil, fmt.Errorf("failed to parse ExecDuration %q from %s: %w", prov.ExecDuration, provisionJSONPath, err)
	}
	report.Tasks = append(report.Tasks, CSETaskTiming{
		TaskName:  "AKS.CSE.cse_start",
		StartTime: cseStart,
		EndTime:   cseStart.Add(execDuration),
		Duration:  execDuration,
	})

	return report, nil
}

func extractXtraceJSONField(line, field string) string {
	fieldStart := strings.Index(line, `"`+field+`":`)
	if fieldStart == -1 {
		return ""
	}
	valueStart := strings.Index(line[fieldStart+len(field)+3:], `"`)
	if valueStart == -1 {
		return ""
	}
	valueStart += fieldStart + len(field) + 4
	valueEnd := strings.Index(line[valueStart:], `"`)
	if valueEnd == -1 {
		return ""
	}
	return line[valueStart : valueStart+valueEnd]
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

func parseProvisionTimestamp(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"Mon Jan _2 15:04:05 MST 2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse provision timestamp %q", s)
}

// CSETimingThresholds defines maximum acceptable durations for CSE tasks.
type CSETimingThresholds struct {
	// TaskThresholds maps task name suffixes to maximum duration.
	// Task names are matched by suffix to allow flexible matching
	// (e.g., "installDebPackageFromFile" matches "AKS.CSE.installkubelet.installDebPackageFromFile").
	TaskThresholds map[string]time.Duration

	// TotalCSEThreshold is the maximum acceptable total CSE duration.
	TotalCSEThreshold time.Duration

	// DefaultTaskThreshold is the threshold applied to any task that exceeds it
	// but has no specific entry in TaskThresholds. This ensures that ALL slow tasks
	// appear as sub-tests in ADO Pipeline Analytics, even newly added ones.
	// Tasks below this threshold are silently skipped.
	// Set to 0 to disable dynamic tracking.
	DefaultTaskThreshold time.Duration
}

// ValidateCSETimings extracts CSE task timings from the VM, logs them, and validates
// against thresholds. Each threshold check runs as a t.Run() sub-test so that ADO
// Pipeline Analytics (via gotestsum → JUnit XML → PublishTestResults) can track
// individual CSE task pass/fail and duration trends over time.
func ValidateCSETimings(ctx context.Context, s *Scenario, thresholds CSETimingThresholds) *CSETimingReport {
	s.T.Helper()
	defer toolkit.LogStep(s.T, "validating CSE task timings")()

	// Unwrap the underlying *testing.T from the toolkit logger wrapper
	// so we can use t.Run() for sub-tests (ADO Pipeline Analytics tracking).
	tRunner := toolkit.UnwrapTestingT(s.T)
	if tRunner == nil {
		s.T.Fatalf("ValidateCSETimings requires *testing.T for sub-test support, got %T", s.T)
	}

	// Use pre-cached report if available (extracted eagerly before GA swept events).
	// Fall back to live extraction if no cached report exists.
	report := s.Runtime.CSETimingReport
	if report == nil {
		var err error
		report, err = ExtractCSETimings(ctx, s)
		if err != nil {
			s.T.Fatalf("failed to extract CSE timings: %v", err)
			return nil
		}
	}

	// Always log the full timing report
	report.LogReport(ctx, s.T)

	// Fail if no tasks were parsed — an empty report makes regression detection ineffective.
	if len(report.Tasks) == 0 {
		s.T.Fatalf("no CSE task timings were parsed; cannot validate performance thresholds")
		return nil
	}

	// Fail if the critical cse_start task is missing — without it TotalCSEDuration()
	// returns 0 and the total duration threshold check would silently pass.
	if report.GetTask("AKS.CSE.cse_start") == nil {
		s.T.Fatalf("AKS.CSE.cse_start task not found in timing report; cannot validate total CSE duration")
		return nil
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
	// Sort suffixes by length descending so longer (more specific) suffixes match first,
	// making matching deterministic when multiple suffixes could match the same task.
	sortedSuffixes := make([]string, 0, len(thresholds.TaskThresholds))
	for suffix := range thresholds.TaskThresholds {
		sortedSuffixes = append(sortedSuffixes, suffix)
	}
	sort.Slice(sortedSuffixes, func(i, j int) bool {
		return len(sortedSuffixes[i]) > len(sortedSuffixes[j])
	})

	matchedTasks := make(map[string]bool)
	matchedSuffixes := make(map[string]bool)
	for _, task := range report.Tasks {
		for _, suffix := range sortedSuffixes {
			maxDuration := thresholds.TaskThresholds[suffix]
			if strings.HasSuffix(task.TaskName, suffix) {
				matchedTasks[task.TaskName] = true
				matchedSuffixes[suffix] = true
				task := task
				suffix := suffix
				maxDuration := maxDuration
				// Include sanitized task name to avoid collisions when multiple tasks match different suffixes
				shortTask := task.TaskName
				if idx := strings.LastIndex(shortTask, "."); idx >= 0 {
					shortTask = shortTask[idx+1:]
				}
				testName := suffix
				if shortTask != suffix {
					testName = fmt.Sprintf("%s/%s", shortTask, suffix)
				}
				tRunner.Run(fmt.Sprintf("Task_%s", testName), func(t *testing.T) {
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

	// Log warnings for configured threshold suffixes that didn't match any task.
	// This helps detect task renames/removals without hard-failing, since some tasks
	// only fire on specific install paths (cached vs full) or OS variants.
	for _, suffix := range sortedSuffixes {
		if !matchedSuffixes[suffix] {
			s.T.Logf("⚠️  threshold suffix %q did not match any CSE task — task may not fire on this install path, or may have been renamed", suffix)
		}
	}

	// Dynamic tracking: create sub-tests for any CSE task that exceeds DefaultTaskThreshold
	// but wasn't matched by a specific threshold above. This ensures newly added CSE tasks
	// automatically appear in ADO Pipeline Analytics without code changes.
	// Skip cse_start (validated by TotalCSEThreshold) and non-CSE events (e.g., AKS.Runtime.*).
	if thresholds.DefaultTaskThreshold > 0 {
		for _, task := range report.Tasks {
			if matchedTasks[task.TaskName] {
				continue
			}
			if task.TaskName == "AKS.CSE.cse_start" {
				continue
			}
			if !strings.HasPrefix(task.TaskName, "AKS.CSE.") {
				continue
			}
			if task.Duration < thresholds.DefaultTaskThreshold {
				continue
			}
			task := task
			// Extract short name: "AKS.CSE.foo.bar" → "bar", or use full name if no dots
			shortName := task.TaskName
			if idx := strings.LastIndex(shortName, "."); idx >= 0 {
				shortName = shortName[idx+1:]
			}
			defaultThreshold := thresholds.DefaultTaskThreshold
			tRunner.Run(fmt.Sprintf("Task_%s", shortName), func(t *testing.T) {
				t.Logf("task %s duration: %s (default threshold: %s — no specific threshold configured)",
					task.TaskName, task.Duration, defaultThreshold)
				if task.Duration > defaultThreshold {
					t.Errorf("CSE task %s took %s, exceeds default threshold %s (consider adding a specific threshold)",
						task.TaskName, task.Duration, defaultThreshold)
				}
			})
		}
	}

	return report
}
