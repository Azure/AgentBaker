package e2e

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReportsSuiteDurationExcludesADOTestCases(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")

	attemptDuration := 10 * time.Second
	checkDuration := 4 * time.Second

	results := []scenarioResult{
		{
			Name:   "Scenario1",
			Status: statusPassed,
			Attempts: []attemptResult{
				{
					Attempt:  1,
					Status:   statusPassed,
					Duration: attemptDuration,
					ADOTestCases: []adoTestCase{
						{Name: "TotalCSEDuration", ClassName: "e2e.cse", Duration: checkDuration},
					},
				},
			},
		},
	}

	require.NoError(t, writeJUnitReport(junitPath, results))

	data, err := os.ReadFile(junitPath)
	require.NoError(t, err, "failed to read JUnit report")

	var report junitSuites
	require.NoError(t, xml.Unmarshal(data, &report), "failed to unmarshal JUnit report")

	require.Len(t, report.Suites, 1)
	suite := report.Suites[0]

	require.Len(t, suite.Cases, 2, "expected scenario and ADO test case")

	assertJUnitTime(t, suite.Cases[0].Time, attemptDuration)
	assertJUnitTime(t, suite.Cases[1].Time, checkDuration)
	assertJUnitTime(t, suite.Time, attemptDuration)
}

func assertJUnitTime(t *testing.T, got string, want time.Duration) {
	t.Helper()
	gotDuration, err := time.ParseDuration(got + "s")
	require.NoError(t, err, "failed to parse JUnit time %q", got)
	assert.InDelta(t, want, gotDuration, float64(time.Millisecond))
}
