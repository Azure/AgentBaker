package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionScenariosKeepsRegisteredScenariosUnchanged(t *testing.T) {
	scenarios := []*scenario.Scenario{
		{Name: "Excluded"},
		{Name: "Kept"},
	}

	runnable, filtered, err := partitionScenarios(scenarios, tagFilter{skip: "Name=Excluded"})
	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, "Kept", runnable[0].Name)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Excluded", filtered[0].Name)
	assert.Equal(t, statusSkipped, filtered[0].Status)
	require.Len(t, filtered[0].Attempts, 1)
	assert.True(t, strings.HasPrefix(filtered[0].Attempts[0].Message, "filtered: "), "filtered reason lost its prefix: %q", filtered[0].Attempts[0].Message)
	for _, scenario := range scenarios {
		assert.Empty(t, scenario.Tags, "filtering mutated the registered scenario")
	}
}

func TestPartitionScenariosAcceptsLegacyTestNameFilter(t *testing.T) {
	scenarios := []*scenario.Scenario{{Name: "AzureLinuxV2"}, {Name: "Ubuntu2204"}}

	runnable, filtered, err := partitionScenarios(scenarios, tagFilter{run: "Name=Test_AzureLinuxV2"})

	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, "AzureLinuxV2", runnable[0].Name)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Ubuntu2204", filtered[0].Name)
}

func TestACLIPEOptInRequiresOnlyUnfilteredACL(t *testing.T) {
	cases := []struct {
		name, mode, filter, want string
		selectors                []string
		child                    bool
	}{
		{name: "ordinary run selects all"},
		{name: "ordinary run without ACL", selectors: []string{"ACL_CustomCA"}},
		{name: "off selects only ACL", mode: "off", selectors: []string{"ACL"}},
		{name: "audit selects only ACL", mode: "audit", selectors: []string{"ACL"}},
		{name: "off rejects all Linux", mode: "off", want: "requires exactly the ACL scenario"},
		{name: "audit rejects all Linux", mode: "audit", want: "requires exactly the ACL scenario"},
		{name: "off rejects other ACL scenario", mode: "off", selectors: []string{"ACL", "ACL_CustomCA"}, want: "requires exactly the ACL scenario"},
		{name: "audit rejects other ACL scenario", mode: "audit", selectors: []string{"ACL", "ACL_AzureCNI"}, want: "requires exactly the ACL scenario"},
		{name: "off rejects nested ACL", mode: "off", selectors: []string{"ACL"}, child: true, want: "requires exactly the ACL scenario"},
		{name: "off missing ACL", mode: "off", selectors: []string{"ACL_CustomCA"}, want: "requires exactly the ACL scenario"},
		{name: "audit filtered ACL", mode: "audit", selectors: []string{"ACL"}, filter: "Name=ACL", want: "not filtered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry := []*scenario.Scenario{
				{Name: "ACL"}, {Name: "ACL_CustomCA"}, {Name: "ACL_AzureCNI"},
				{Name: "ACL_NetworkIsolatedCluster_NonAnonymousACR"}, {Name: "Ubuntu2204"},
			}
			if tc.child {
				registry = append(registry, &scenario.Scenario{Name: "ACL/child"})
			}
			scenarios := selectScenarios(registry, tc.selectors)
			err := requireACLIPEOnly(scenarios, tc.mode)
			if err == nil {
				runnable, _, partitionErr := partitionScenarios(scenarios, tagFilter{skip: tc.filter})
				require.NoError(t, partitionErr)
				err = requireACLIPEOnly(runnable, tc.mode)
			}
			if tc.want == "" {
				require.NoError(t, err)
				if tc.mode != "" {
					require.Len(t, scenarios, 1)
					require.Equal(t, "ACL", scenarios[0].Name)
				}
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestACLIPEOptInRequiresPassingMeasurements(t *testing.T) {
	firstBoot := func(mode string) scenario.Measurement {
		return scenario.Measurement{Name: "ACL_IPE_FirstBoot_" + mode}
	}
	auditDeny := scenario.Measurement{Name: "ACL_IPE_AuditDeny"}
	tests := []struct {
		name, mode, want string
		results          []scenarioResult
	}{
		{name: "ordinary run may skip ACL", results: []scenarioResult{{Name: "ACL", Status: statusSkipped}}},
		{name: "off passes", mode: "off", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed, ADOTestCases: []scenario.Measurement{firstBoot("off")}}}}}},
		{name: "audit passes", mode: "audit", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed, ADOTestCases: []scenario.Measurement{firstBoot("audit"), auditDeny}}}}}},
		{name: "off absent", mode: "off", want: "no ACL scenario result", results: []scenarioResult{{Name: "ACL_CustomCA", Status: statusPassed}}},
		{name: "off skipped missing image", mode: "off", want: "ACL scenario skipped", results: []scenarioResult{{Name: "ACL", Status: statusSkipped,
			Attempts: []attemptResult{{Status: statusSkipped, Message: "image aclgen2TL missing"}}}}},
		{name: "audit filtered", mode: "audit", want: "ACL scenario skipped", results: []scenarioResult{{Name: "ACL", Status: statusSkipped,
			Attempts: []attemptResult{{Status: statusSkipped, Message: "filtered: Name=ACL"}}}}},
		{name: "off failed missing image", mode: "off", want: "ACL scenario failed", results: []scenarioResult{{Name: "ACL", Status: statusFailed,
			Attempts: []attemptResult{{Status: statusFailed, Message: "image aclgen2TL missing"}}}}},
		{name: "off missing first boot", mode: "off", want: "ACL_IPE_FirstBoot_off", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed}}}}},
		{name: "off wrong mode", mode: "off", want: "ACL_IPE_FirstBoot_off", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed, ADOTestCases: []scenario.Measurement{firstBoot("audit")}}}}}},
		{name: "audit missing deny", mode: "audit", want: "ACL_IPE_AuditDeny", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed, ADOTestCases: []scenario.Measurement{firstBoot("audit")}}}}}},
		{name: "off failed measurement", mode: "off", want: "ACL_IPE_FirstBoot_off", results: []scenarioResult{{Name: "ACL", Status: statusPassed,
			Attempts: []attemptResult{{Status: statusPassed, ADOTestCases: []scenario.Measurement{{Name: firstBoot("off").Name, Message: "policy failed"}}}}}}},
		{name: "off flaky final pass", mode: "off", results: []scenarioResult{{Name: "ACL", Status: statusFlaky,
			Attempts: []attemptResult{{Status: statusFailed}, {Status: statusPassed, ADOTestCases: []scenario.Measurement{firstBoot("off")}}}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireACLIPEPassed(tc.results, tc.mode)
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestAppACLIPEOptInFailsBeforeInitializationWhenACLExcluded(t *testing.T) {
	for _, tc := range []struct {
		name, mode, skip, want string
		selectors              []string
	}{
		{name: "off missing selector", mode: "off", selectors: []string{"ACL_CustomCA"}, want: "requires exactly the ACL scenario"},
		{name: "audit filtered", mode: "audit", selectors: []string{"ACL"}, skip: "Name=ACL", want: "not filtered"},
		{name: "off rejects all", mode: "off", want: "requires exactly the ACL scenario"},
		{name: "audit rejects other ACL", mode: "audit", selectors: []string{"ACL", "ACL_CustomCA"}, want: "requires exactly the ACL scenario"},
		{name: "invalid mode", mode: "enforce", selectors: []string{"ACL"}, want: "must be off or audit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreRunnerConfig(t)
			t.Setenv("ACL_IPE_EXPECTED_MODE", tc.mode)
			before := azureInitProbe()
			var stderr bytes.Buffer
			args := []string{"e2e", "run", "--log-dir", t.TempDir(), "--skip-tags", tc.skip}
			args = append(args, tc.selectors...)
			code := NewApp(&bytes.Buffer{}, &stderr).Run(t.Context(), args)
			require.NotEqual(t, exitSuccess, code)
			require.Contains(t, stderr.String(), tc.want)
			require.Equal(t, before, azureInitProbe())
		})
	}
}

func TestFilterReasonIsConcise(t *testing.T) {
	s := &scenario.Scenario{
		Name: "Windows2025Gen2_McrChinaCloud_Windows",
		Tags: scenario.Tags{OS: "windows", MockAzureChinaCloud: true},
	}
	for _, test := range []struct {
		name   string
		filter tagFilter
		want   string
	}{
		{"run", tagFilter{run: "os=linux"}, `filtered: does not match run filter "os=linux"`},
		{"skip", tagFilter{skip: "os=windows,gpu=true"}, `filtered: matches skip filter "os=windows,gpu=true"`},
		{"included", tagFilter{run: "os=windows"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			reason, err := filterReason(s.Name, s, test.filter)
			require.NoError(t, err)
			assert.Equal(t, test.want, reason)
		})
	}
}

func TestPartitionScenariosRejectsInvalidFilters(t *testing.T) {
	scenarios := []*scenario.Scenario{{Name: "Only"}}
	for _, filter := range []tagFilter{{run: "not-a-pair"}, {skip: "unknownKey=true"}} {
		_, _, err := partitionScenarios(scenarios, filter)
		require.Error(t, err, "invalid filter %+v was accepted", filter)
	}
}

// azureInitProbe returns a value that changes whenever config.Initialize runs.
func azureInitProbe() string {
	return config.VMSSHPrivateKeyFileName
}

func TestAppFailsBeforeInitializationWhenFiltersMatchNothing(t *testing.T) {
	restoreRunnerConfig(t)
	before := azureInitProbe()
	junitFile := filepath.Join(t.TempDir(), "report.xml")

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	code := app.Run(context.Background(), []string{
		"e2e", "run", "--log-dir", t.TempDir(), "--junit-file", junitFile, "--tags", "Name=DoesNotExist", "Ubuntu2204_CustomLinuxOSConfig_Taints_ANC",
	})

	assert.Equal(t, exitUsage, code, "stderr: %s", stderr.String())
	assert.Contains(t, stderr.String(), "no scenarios matched the configured filters")
	assert.Equal(t, before, azureInitProbe(), "configuration was initialized before the filters were evaluated")
	report, err := os.ReadFile(junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `<skipped message="filtered:`, "JUnit report dropped the filtered scenario")
}

func TestAppFailsFastOnInvalidTagFilter(t *testing.T) {
	restoreRunnerConfig(t)
	before := azureInitProbe()

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	code := app.Run(context.Background(), []string{
		"e2e", "run", "--log-dir", t.TempDir(), "--tags", "not-a-pair", "Ubuntu2204_CustomLinuxOSConfig_Taints_ANC",
	})

	assert.Equal(t, exitFailure, code, "stderr: %s", stderr.String())
	assert.Contains(t, stderr.String(), "invalid filter format")
	assert.Equal(t, before, azureInitProbe(), "configuration was initialized before the filters were validated")
}

func restoreRunnerConfig(t *testing.T) {
	t.Helper()
	saved := *config.Config
	t.Cleanup(func() { *config.Config = saved })
}

func TestMatchFiltersPreservesTagPolicy(t *testing.T) {
	tags := scenario.Tags{Name: "Ubuntu2204", OS: "linux", GPU: true}
	for _, test := range []struct {
		filter string
		all    bool
		match  bool
	}{
		{filter: "Name=Other,Name=Test_Ubuntu2204", all: true, match: true},
		{filter: "Name=Ubuntu2204,GPU=false", all: true, match: false},
		{filter: "Name=Ubuntu2204,GPU=false", all: false, match: true},
		{filter: " os = LINUX , gpu = true ", all: true, match: true},
		{filter: "", all: true, match: true},
	} {
		t.Run(test.filter, func(t *testing.T) {
			got, err := matchFilters(tags, test.filter, test.all)
			require.NoError(t, err)
			assert.Equal(t, test.match, got)
		})
	}
	_, err := matchFilters(tags, "GPU=invalid", true)
	require.ErrorContains(t, err, "invalid boolean")
}
