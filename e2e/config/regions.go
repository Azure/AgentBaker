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

// e2eRegions is the set of regions every E2E gallery image version is replicated to.
//
// PublishingProfile.TargetRegions is full desired state: an update replaces the region
// list rather than appending to it. Writers that each add only their own region compute
// different desired states from possibly stale reads, so a concurrent write can silently
// drop another writer's region. Replicating to this fixed set instead means every writer -
// across goroutines, test processes, and pipelines - submits an identical desired state,
// so a lost update loses nothing and no locking or cross-process coordination is needed.
var e2eRegions = []string{
	RegionEastUS,
	RegionSouthCentralUS,
	RegionSouthEastAsia,
	RegionUAENorth,
	RegionWestUS2,
	RegionWestUS3,
}

// E2EReplicationRegions returns the regions E2E images are replicated to. DefaultLocation is
// included so a local run with a custom E2E_LOCATION still works; CI never overrides it, so
// every pipeline computes the same set.
func E2EReplicationRegions() []string {
	regions := make([]string, 0, len(e2eRegions)+1)
	regions = append(regions, e2eRegions...)
	if location := NormalizeRegion(Config.DefaultLocation); location != "" && !containsRegion(e2eRegions, location) {
		regions = append(regions, location)
	}
	return regions
}

// IsE2ERegion reports whether scenarios may run in the given region.
func IsE2ERegion(location string) bool {
	return containsRegion(E2EReplicationRegions(), NormalizeRegion(location))
}

// replicationRegions returns the regions this image version should be replicated to.
func (i *Image) replicationRegions(location string) []string {
	if i.Ephemeral {
		// Created and deleted by a single test, so it has exactly one writer: there is no
		// lost update to defend against and no reason to pay to replicate a throwaway
		// captured OS image to regions nothing will read it from.
		return []string{location}
	}
	return append(E2EReplicationRegions(), location)
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
