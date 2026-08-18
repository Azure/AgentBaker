package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratedRegionsUpToDate is the drift gate: if a scenario is pinned to a new region and
// nobody reran the generator, the replication set would silently stop covering that region
// and the scenario would fail with GalleryImageNotFound. Failing here instead is much cheaper.
func TestGeneratedRegionsUpToDate(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving e2e module root: %v", err)
	}

	regions, err := scan(root)
	if err != nil {
		t.Fatalf("scanning scenarios: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, generatedFileName))
	if err != nil {
		t.Fatalf("reading %s: %v", generatedFileName, err)
	}

	if want := render(regions); !bytes.Equal(got, want) {
		t.Fatalf("%s is out of date; run `make generate-e2e-regions`", generatedFileName)
	}
}

func TestScanFindsPinnedRegions(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving e2e module root: %v", err)
	}

	regions, err := scan(root)
	if err != nil {
		t.Fatalf("scanning scenarios: %v", err)
	}

	// Regions reach a scenario as a struct field and as a helper argument. Assert one of
	// each so a regression in either path is caught.
	for _, want := range []string{"westus2", "uaenorth"} {
		if !contains(regions.linux, want) {
			t.Errorf("scan did not find region %q pinned by a scenario; got %v", want, regions.linux)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// A region that cannot be resolved statically would never reach the generated list, so the
// scenario would run somewhere the image is not replicated. Both ways a region reaches a
// Scenario must therefore be rejected when they are not a config.Region* constant.
func TestScanRejectsUnresolvableRegions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scenario string
	}{
		{
			name:     "literal in Location field",
			scenario: `func TestX(t *testing.T) { RunScenario(t, &Scenario{Location: "northeurope"}) }`,
		},
		{
			name: "literal passed to a helper's location parameter",
			scenario: `func TestX(t *testing.T) { helper(t, "northeurope") }
func helper(t *testing.T, location string) { RunScenario(t, &Scenario{Location: location}) }`,
		},
		{
			name:     "non-constant expression",
			scenario: `func TestX(t *testing.T) { RunScenario(t, &Scenario{Location: os.Getenv("REGION")}) }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := stubModule(t, tc.scenario)
			if _, err := scan(root); err == nil {
				t.Fatal("expected scan to reject a region it cannot resolve, got nil error")
			}
		})
	}
}

func TestScanAcceptsRegionConstants(t *testing.T) {
	root := stubModule(t, `func TestX(t *testing.T) { helper(t, config.RegionWestUS2) }
func helper(t *testing.T, location string) { RunScenario(t, &Scenario{Location: location}) }`)

	regions, err := scan(root)
	if err != nil {
		t.Fatalf("scan rejected a valid region constant: %v", err)
	}
	for _, want := range []string{"westus2", "westus3"} {
		if !contains(regions.linux, want) {
			t.Errorf("expected %q in %v", want, regions.linux)
		}
	}
}

// stubModule writes a minimal e2e tree so scan can be exercised without the real scenarios.
func stubModule(t *testing.T, scenario string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	regions := `package config

const (
	RegionWestUS2 = "westus2"
	RegionWestUS3 = "westus3"
)

const BaselineRegion = RegionWestUS3
`
	if err := os.WriteFile(filepath.Join(root, regionsFileName), []byte(regions), 0o644); err != nil {
		t.Fatalf("writing regions.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "scenario_test.go"), []byte("package e2e\n\n"+scenario+"\n"), 0o644); err != nil {
		t.Fatalf("writing scenario_test.go: %v", err)
	}
	return root
}
