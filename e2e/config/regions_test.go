package config

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func withDefaultLocation(t *testing.T, location string) {
	t.Helper()
	previous := Config.DefaultLocation
	Config.DefaultLocation = location
	t.Cleanup(func() { Config.DefaultLocation = previous })
}

// The whole point of the fixed region set is that the desired state does not depend on the
// region the caller happens to need. If it ever does again, concurrent writers can clobber
// each other's regions and GalleryImageNotFound comes back.
func TestReplicationRegionsDoNotDependOnCallerRegion(t *testing.T) {
	withDefaultLocation(t, RegionWestUS3)

	image := &Image{OS: OSUbuntu}
	first := image.replicationRegions()
	second := image.replicationRegions()

	if len(first) != len(second) {
		t.Fatalf("replicationRegions is not deterministic: %v vs %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("replicationRegions is not deterministic: %v vs %v", first, second)
		}
	}

	for _, region := range []string{RegionWestUS2, RegionEastUS, RegionUAENorth} {
		if !slicesContains(first, region) {
			t.Errorf("region %q pinned by a scenario is missing from the replication set %v", region, first)
		}
	}
	if !slicesContains(first, RegionWestUS3) {
		t.Errorf("default location westus3 missing from replication set %v", first)
	}
}

func TestReplicationRegionsWindowsExcludesLinuxOnlyRegions(t *testing.T) {
	withDefaultLocation(t, RegionWestUS3)

	regions := (&Image{OS: OSWindows}).replicationRegions()
	if len(regions) != 1 || regions[0] != BaselineRegion {
		t.Fatalf("expected Windows images to replicate only to the baseline region, got %v", regions)
	}
}

func TestReplicationRegionsEphemeralStaysLocal(t *testing.T) {
	withDefaultLocation(t, RegionWestUS3)

	if regions := (&Image{OS: OSUbuntu, Ephemeral: true}).replicationRegions(); len(regions) != 0 {
		t.Fatalf("expected ephemeral images to opt out of the shared replication set, got %v", regions)
	}
}

// E2E_LOCATION is per-process, so if it could widen the replication set two pipelines would
// compute different desired states and clobber each other - the original bug, across
// processes instead of goroutines. An unsupported region must be rejected, not accommodated.
func TestReplicationRegionsIgnoreDefaultLocationOverride(t *testing.T) {
	withDefaultLocation(t, "northeurope")

	regions := (&Image{OS: OSUbuntu}).replicationRegions()
	if slicesContains(regions, "northeurope") {
		t.Fatalf("E2E_LOCATION must not widen the replication set, got %v", regions)
	}
	if !slicesContains(regions, BaselineRegion) {
		t.Fatalf("baseline region missing from replication set %v", regions)
	}
	if (&Image{OS: OSUbuntu}).SupportsE2ERegion("northeurope") {
		t.Fatal("an unlisted region must be reported as unsupported so the run fails fast")
	}
}

func TestBaselineRegionIsAlwaysReplicated(t *testing.T) {
	for _, os := range []OS{OSUbuntu, OSWindows, OSMariner, OSAzureLinux} {
		image := &Image{OS: os}
		if !slicesContains(image.replicationRegions(), BaselineRegion) {
			t.Errorf("%s images must replicate to the baseline region, got %v", os, image.replicationRegions())
		}
	}
}

func TestSupportsE2ERegionNormalizesARMFormatting(t *testing.T) {
	withDefaultLocation(t, RegionWestUS3)
	image := &Image{OS: OSUbuntu}

	for _, location := range []string{"westus2", "West US 2", "WestUS2", " westus2 "} {
		if !image.SupportsE2ERegion(location) {
			t.Errorf("SupportsE2ERegion(%q) = false, want true", location)
		}
	}
	if image.SupportsE2ERegion("northeurope") {
		t.Error("SupportsE2ERegion(\"northeurope\") = true, want false")
	}
	if !(&Image{OS: OSUbuntu, Ephemeral: true}).SupportsE2ERegion("northeurope") {
		t.Error("ephemeral images should be usable in whatever region created them")
	}
}

func TestMissingRegions(t *testing.T) {
	existing := []*armcompute.TargetRegion{
		{Name: to.Ptr("West US 2")},
		nil,
		{Name: nil},
		{Name: to.Ptr("eastus")},
	}

	missing := missingRegions(existing, []string{"westus2", "eastus", "uaenorth"})
	if len(missing) != 1 || missing[0] != "uaenorth" {
		t.Fatalf("missingRegions = %v, want [uaenorth]", missing)
	}

	if got := missingRegions(existing, []string{"westus2"}); len(got) != 0 {
		t.Fatalf("missingRegions on a fully replicated image = %v, want none", got)
	}
}

func TestNormalizeRegion(t *testing.T) {
	for input, want := range map[string]string{
		"West US 2": "westus2",
		"WESTUS2":   "westus2",
		"  eastus ": "eastus",
		"":          "",
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
		{"wrapped conflict", fmt.Errorf("update: %w", &azcore.ResponseError{StatusCode: http.StatusConflict}), true},
		{"not found", &azcore.ResponseError{StatusCode: http.StatusNotFound}, false},
		{"plain error", errors.New("boom"), false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGalleryUpdateConflict(tc.err); got != tc.want {
				t.Fatalf("isGalleryUpdateConflict(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
