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

// A Region* constant that is missing from e2eRegions is a region scenarios can pin but images
// are never replicated to, which is exactly the GalleryImageNotFound failure this package
// exists to prevent. Keeping the two in sync is the only maintenance this design needs.
func TestRegionsAreConsistent(t *testing.T) {
	source, err := os.ReadFile("regions.go")
	if err != nil {
		t.Fatalf("reading regions.go: %v", err)
	}

	declared := regexp.MustCompile(`(?m)^\tRegion\w+\s+= "([a-z0-9]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(declared) == 0 {
		t.Fatal("found no Region* constants in regions.go; has the declaration style changed?")
	}
	for _, match := range declared {
		if !slices.Contains(e2eRegions, match[1]) {
			t.Errorf("region %q is declared but missing from e2eRegions", match[1])
		}
	}
	if len(e2eRegions) != len(declared) {
		t.Errorf("e2eRegions has %d entries but %d Region* constants are declared", len(e2eRegions), len(declared))
	}
}

// Scenarios must reference the constants, otherwise a region can reach a scenario without
// reaching e2eRegions.
func TestScenariosDoNotPinRegionLiterals(t *testing.T) {
	files, err := filepath.Glob("../*_test.go")
	if err != nil {
		t.Fatalf("listing scenario files: %v", err)
	}

	literal := regexp.MustCompile(`(?m)^\s*Location:\s*"([^"]*)"`)
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, match := range literal.FindAllStringSubmatch(string(source), -1) {
			t.Errorf("%s pins Location: %q; use a config.Region* constant instead", filepath.Base(path), match[1])
		}
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
	if got := (&Image{OS: OSWindows}).replicationRegions(); !slices.Equal(got, []string{RegionWestUS3}) {
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
	got := missingRegions(existing, []string{RegionWestUS2, RegionEastUS})
	if !slices.Equal(got, []string{RegionEastUS}) {
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
