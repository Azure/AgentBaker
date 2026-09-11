package scenario

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateCSETimings(t *testing.T) {
	thresholds := CSETimingThresholds{
		TotalCSEThreshold: 3 * time.Second,
		TaskThresholds: map[string]time.Duration{
			"installContainerRuntime": 2 * time.Second,
		},
	}
	for _, tc := range []struct {
		name        string
		total       time.Duration
		install     time.Duration
		wantFailure string
	}{
		{name: "within limits", total: 3 * time.Second, install: 2 * time.Second},
		{name: "total exceeds limit", total: 4 * time.Second, install: 2 * time.Second, wantFailure: "TotalCSEDuration"},
		{name: "task exceeds limit", total: 3 * time.Second, install: 3 * time.Second, wantFailure: "Task_installContainerRuntime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Scenario{
				Logger: discardLogger{},
				Runtime: &ScenarioRuntime{
					CSETimingReport: &CSETimingReport{Tasks: []CSETaskTiming{
						{TaskName: "AKS.CSE.cse_start", Duration: tc.total},
						{TaskName: "AKS.CSE.installContainerRuntime", Duration: tc.install},
					}},
				},
			}
			report, err := ValidateCSETimings(t.Context(), s, thresholds)
			require.Same(t, s.Runtime.CSETimingReport, report)
			if tc.wantFailure == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Len(t, s.adoTestCases, 2)
			for _, result := range s.adoTestCases {
				require.Equal(t, "e2e.cse", result.ClassName)
				if result.Name == tc.wantFailure {
					require.NotEmpty(t, result.Message)
				} else {
					require.Empty(t, result.Message)
				}
			}
		})
	}
}
