package e2e

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
)

func TestPartitionEntriesKeepsRegisteredScenariosUnchanged(t *testing.T) {
	entries := []scenarioEntry{
		{name: "Excluded", scenario: &Scenario{Name: "Excluded"}},
		{name: "Kept", scenario: &Scenario{Name: "Kept"}},
	}

	runnable, filtered, err := partitionEntries(entries, tagFilter{skip: "Name=Excluded"})
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 1 || runnable[0].name != "Kept" {
		t.Fatalf("unexpected runnable entries: %+v", runnable)
	}
	if len(filtered) != 1 || filtered[0].Name != "Excluded" || filtered[0].Status != statusSkipped {
		t.Fatalf("unexpected filtered results: %+v", filtered)
	}
	if !strings.HasPrefix(filtered[0].Attempts[0].Message, "filtered: ") {
		t.Fatalf("filtered reason lost its prefix: %q", filtered[0].Attempts[0].Message)
	}
	for _, entry := range entries {
		if entry.scenario.Tags != (Tags{}) {
			t.Fatalf("filtering mutated the registered scenario: %+v", entry.scenario.Tags)
		}
	}
}

func TestPartitionEntriesRejectsInvalidFilters(t *testing.T) {
	entries := []scenarioEntry{{name: "Only", scenario: &Scenario{Name: "Only"}}}
	for _, filter := range []tagFilter{{run: "not-a-pair"}, {skip: "unknownKey=true"}} {
		if _, _, err := partitionEntries(entries, filter); err == nil {
			t.Fatalf("invalid filter %+v was accepted", filter)
		}
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
		"e2e", "run", "--log-dir", t.TempDir(), "--junit-file", junitFile, "--tags", "Name=DoesNotExist", "Ubuntu2204",
	})

	if code != exitUsage {
		t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no scenarios matched the configured filters") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
	if azureInitProbe() != before {
		t.Fatal("configuration was initialized before the filters were evaluated")
	}
	report, err := os.ReadFile(junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte(`<skipped message="filtered:`)) {
		t.Fatalf("JUnit report dropped the filtered scenario:\n%s", report)
	}
}

func TestAppFailsFastOnInvalidTagFilter(t *testing.T) {
	restoreRunnerConfig(t)
	before := azureInitProbe()

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	code := app.Run(context.Background(), []string{
		"e2e", "run", "--log-dir", t.TempDir(), "--tags", "not-a-pair", "Ubuntu2204",
	})

	if code != exitFailure {
		t.Fatalf("run returned %d, want %d; stderr: %s", code, exitFailure, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid filter format") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
	if azureInitProbe() != before {
		t.Fatal("configuration was initialized before the filters were validated")
	}
}

func restoreRunnerConfig(t *testing.T) {
	t.Helper()
	saved := *config.Config
	t.Cleanup(func() { *config.Config = saved })
}
