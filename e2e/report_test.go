package e2e

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReportsSuiteDurationExcludesChildChecks(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")

	attemptDuration := 10 * time.Second
	checkDuration := 4 * time.Second

	e := &executor{
		opts: runOptions{junitFile: junitPath},
		results: []scenarioResult{
			{
				Name:   "Scenario1",
				Status: statusPassed,
				Attempts: []attemptResult{
					{
						Attempt:  1,
						Status:   statusPassed,
						Duration: attemptDuration,
						Checks: []scenarioCheck{
							{Name: "TotalCSEDuration", Duration: checkDuration},
						},
					},
				},
			},
		},
	}

	if err := e.writeReports(nil); err != nil {
		t.Fatalf("writeReports() returned error: %v", err)
	}

	data, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatalf("failed to read JUnit report: %v", err)
	}

	var report junitSuites
	if err := xml.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed to unmarshal JUnit report: %v", err)
	}

	if len(report.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(report.Suites))
	}
	suite := report.Suites[0]

	if len(suite.Cases) != 2 {
		t.Fatalf("expected 2 cases (parent + child), got %d", len(suite.Cases))
	}

	assertJUnitTime(t, suite.Cases[0].Time, attemptDuration)
	assertJUnitTime(t, suite.Cases[1].Time, checkDuration)
	assertJUnitTime(t, suite.Time, attemptDuration)
}

func assertJUnitTime(t *testing.T, got string, want time.Duration) {
	t.Helper()
	gotDuration, err := time.ParseDuration(got + "s")
	if err != nil {
		t.Fatalf("failed to parse JUnit time %q: %v", got, err)
	}
	if diff := gotDuration - want; diff < -time.Millisecond || diff > time.Millisecond {
		t.Fatalf("got time %s, want %s", gotDuration, want)
	}
}
