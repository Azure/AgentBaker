package config

import "strings"

// E2E scenarios pin themselves to one of these regions. Reference these constants from
// scenarios rather than writing region literals, so this list stays the single definition
// of where E2E runs instead of a copy that can drift.
const (
	RegionEastUS         = "eastus"
	RegionSouthCentralUS = "southcentralus"
	RegionSouthEastAsia  = "southeastasia"
	RegionUAENorth       = "uaenorth"
	RegionWestUS2        = "westus2"
	RegionWestUS3        = "westus3"
)

// linuxE2ERegions is the set of regions every shared Linux gallery image version is
// replicated to.
//
// PublishingProfile.TargetRegions is full desired state: an update replaces the region list
// rather than appending to it. Writers that each add only their own region compute different
// desired states from possibly stale reads, so a concurrent write can silently drop another
// writer's region. Replicating to a fixed set instead means every writer - across goroutines,
// test processes, and pipelines - submits an identical desired state, so a lost update loses
// nothing and no locking or cross-process coordination is needed.
var linuxE2ERegions = []string{
	RegionEastUS,
	RegionSouthCentralUS,
	RegionSouthEastAsia,
	RegionUAENorth,
	RegionWestUS2,
	RegionWestUS3,
}

// windowsE2ERegions is deliberately smaller: no Windows scenario pins a location, so they all
// run in the default region. Windows images are by far the largest, and replicating them to
// regions no Windows test uses would cost storage for nothing. Convergence still holds because
// every writer of a Windows image computes this same set.
var windowsE2ERegions = []string{
	RegionWestUS3,
}

// E2ERegions returns the regions scenarios using this image may run in, and therefore the
// regions the image is replicated to. It backs both the replication set and scenario
// validation, so the two cannot disagree.
func (i *Image) E2ERegions() []string {
	base := linuxE2ERegions
	if i.OS == OSWindows {
		base = windowsE2ERegions
	}

	regions := make([]string, 0, len(base)+1)
	regions = append(regions, base...)
	// A local run may point E2E at a region no scenario names. CI never overrides it, so
	// every pipeline still computes the same set.
	if location := NormalizeRegion(Config.DefaultLocation); location != "" && !containsRegion(base, location) {
		regions = append(regions, location)
	}
	return regions
}

// replicationRegions returns the regions this image version should be replicated to.
func (i *Image) replicationRegions(location string) []string {
	if i.Ephemeral {
		// Created and deleted by a single test, so it has exactly one writer: there is no
		// lost update to defend against and no reason to pay to replicate a throwaway
		// captured OS image to regions nothing will read it from.
		return []string{location}
	}
	return append(i.E2ERegions(), location)
}

// SupportsE2ERegion reports whether scenarios using this image may run in the given region.
func (i *Image) SupportsE2ERegion(location string) bool {
	return containsRegion(i.E2ERegions(), NormalizeRegion(location))
}

// E2ERegionsVarName names the list a new region must be added to, for error messages.
func (i *Image) E2ERegionsVarName() string {
	if i.OS == OSWindows {
		return "windowsE2ERegions"
	}
	return "linuxE2ERegions"
}

func containsRegion(regions []string, normalized string) bool {
	for _, region := range regions {
		if region == normalized {
			return true
		}
	}
	return false
}

// NormalizeRegion converts a region name to the compact lowercase form ARM uses in
// resource IDs, so "West US 2", "westus2" and "WestUS2" compare equal.
func NormalizeRegion(location string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(location), " ", ""))
}
