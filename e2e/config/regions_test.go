package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// A scenario naming a region that images are not replicated to would fail at runtime, so the
// inline regions are checked against e2eRegions here instead. Regions passed to a helper
// rather than set on the struct are not visible to this scan; maybeSkipScenario catches those
// when the scenario runs.
func TestScenarioRegionsAreReplicated(t *testing.T) {
	files, err := filepath.Glob("../*_test.go")
	if err != nil {
		t.Fatalf("listing scenario files: %v", err)
	}

	pinned := regexp.MustCompile(`(?m)^\s*Location:\s*"([^"]+)"`)
	var found int
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, match := range pinned.FindAllStringSubmatch(string(source), -1) {
			found++
			if !slices.Contains(e2eRegions, NormalizeRegion(match[1])) {
				t.Errorf("%s runs a scenario in %q, which is missing from e2eRegions in config/regions.go", filepath.Base(path), match[1])
			}
		}
	}
	if found == 0 {
		t.Fatal("found no scenario regions to check; has the Location field changed shape?")
	}
}

// The whole point of the fixed set is that desired state does not depend on the caller. If it
// ever does again, concurrent writers can clobber each other's regions.
func TestReplicationRegionsDoNotDependOnCaller(t *testing.T) {
	image := &Image{OS: OSUbuntu}
	if got := image.replicationRegions(); !slices.Equal(got, e2eRegions) {
		t.Fatalf("expected the fixed set %v, got %v", e2eRegions, got)
	}

	previous := Config.DefaultLocation
	Config.DefaultLocation = "northeurope"
	t.Cleanup(func() { Config.DefaultLocation = previous })

	// E2E_LOCATION is per-process, so letting it widen the set would make two pipelines
	// compute different desired states - the original bug, across processes.
	if got := image.replicationRegions(); !slices.Equal(got, e2eRegions) {
		t.Fatalf("E2E_LOCATION must not change the replication set, got %v", got)
	}
	if image.SupportsE2ERegion("northeurope") {
		t.Error("an unlisted region must be unsupported so the run fails fast")
	}
}

func TestReplicationRegionsByImageKind(t *testing.T) {
	if got := (&Image{OS: OSWindows}).replicationRegions(); !slices.Equal(got, []string{"westus3"}) {
		t.Errorf("Windows images should replicate only to the default location, got %v", got)
	}
	if got := (&Image{OS: OSUbuntu, Ephemeral: true}).replicationRegions(); len(got) != 0 {
		t.Errorf("ephemeral images should opt out of the shared set, got %v", got)
	}
	if !(&Image{OS: OSUbuntu, Ephemeral: true}).SupportsE2ERegion("northeurope") {
		t.Error("ephemeral images should be usable in any region")
	}
}

func TestSupportsE2ERegionNormalizesARMFormatting(t *testing.T) {
	image := &Image{OS: OSUbuntu}
	for _, location := range []string{"westus2", "West US 2", "WestUS2", " westus2 "} {
		if !image.SupportsE2ERegion(location) {
			t.Errorf("expected %q to be recognised as westus2", location)
		}
	}
}

func TestMissingRegions(t *testing.T) {
	existing := []*armcompute.TargetRegion{
		{Name: to.Ptr("West US 2")}, // ARM returns display names, not the compact form
		nil,
		{Name: nil},
	}
	got := missingRegions(existing, []string{"westus2", "eastus"})
	if !slices.Equal(got, []string{"eastus"}) {
		t.Fatalf("expected only eastus to be missing, got %v", got)
	}
	if got := missingRegions(nil, nil); len(got) != 0 {
		t.Fatalf("expected no missing regions, got %v", got)
	}
}

func TestNormalizeRegion(t *testing.T) {
	for input, want := range map[string]string{
		"westus2": "westus2", "West US 2": "westus2", "WESTUS2": "westus2", " westus2 ": "westus2", "": "",
	} {
		if got := NormalizeRegion(input); got != want {
			t.Errorf("NormalizeRegion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsGalleryUpdateConflict(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"conflict", &azcore.ResponseError{StatusCode: http.StatusConflict}, true},
		{"precondition failed", &azcore.ResponseError{StatusCode: http.StatusPreconditionFailed}, true},
		{"wrapped", fmt.Errorf("update: %w", &azcore.ResponseError{StatusCode: http.StatusConflict}), true},
		{"not found", &azcore.ResponseError{StatusCode: http.StatusNotFound}, false},
		{"plain", errors.New("boom"), false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGalleryUpdateConflict(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
