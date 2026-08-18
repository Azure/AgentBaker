package config

import "strings"

//go:generate go run ../hack/genregions -write

// Region names E2E scenarios may pin themselves to.
//
// Scenarios must reference these constants instead of writing region literals. That keeps
// this file the single place a region is spelled out, and lets e2e/hack/genregions find every
// region E2E uses so the replication set below cannot silently fall behind the scenarios.
const (
	RegionEastUS         = "eastus"
	RegionSouthCentralUS = "southcentralus"
	RegionSouthEastAsia  = "southeastasia"
	RegionUAENorth       = "uaenorth"
	RegionWestUS2        = "westus2"
	RegionWestUS3        = "westus3"
)

// BaselineRegion is always replicated to, whatever the scenarios pin. It is the default value
// of Config.DefaultLocation, so scenarios that pin no region at all land here.
//
// The generator writes it into both lists rather than replicationRegions reading
// Config.DefaultLocation at runtime: E2E_LOCATION is per-process, so deriving the set from it
// would make two pipelines compute different desired states and reintroduce the lost update
// this whole file exists to prevent.
const BaselineRegion = RegionWestUS3

// replicationRegions returns the fixed set of regions an image version is replicated to.
//
// It deliberately ignores the region the caller actually needs, and that is the entire point.
// A gallery image version's PublishingProfile.TargetRegions is full desired state: an update
// replaces the region list rather than appending to it. The original code read the live
// version, appended only the caller's own region and wrote the result back, so two scenarios
// preparing the same image for different regions computed *different* desired states from the
// same read and the second write silently dropped the first writer's region. The test whose
// region was dropped then failed with GalleryImageNotFound.
//
// The returned set depends only on the image, never on the caller, the process or the
// environment. Every writer - across goroutines, test processes and pipelines - therefore
// submits an identical desired state. Interleaved reads and writes converge on the same
// result, so a lost update loses nothing and no locking, merging or conflict resolution is
// needed. The cost is replicating to a handful of regions a given run may not use, which is
// far cheaper than a flaky suite.
//
// Windows uses the smaller windowsScenarioRegions set: no Windows scenario pins a location,
// so they all run in BaselineRegion, and Windows images are by far the largest - replicating
// them where no Windows test looks would cost storage for nothing.
func (i *Image) replicationRegions() []string {
	if i.Ephemeral {
		// Created from a disk and deleted by the single test that made it, so it has exactly
		// one writer: there is no lost update to defend against, and no reason to pay to
		// replicate a throwaway image to regions nothing will ever read it from.
		return nil
	}
	if i.OS == OSWindows {
		return windowsScenarioRegions
	}
	return scenarioRegions
}

// SupportsE2ERegion reports whether scenarios using this image are replicated to location.
func (i *Image) SupportsE2ERegion(location string) bool {
	if i.Ephemeral {
		return true
	}
	return slicesContains(i.replicationRegions(), NormalizeRegion(location))
}

// E2ERegionsVarName names the generated list a new region must be added to, for error
// messages that tell the reader how to fix the failure.
func (i *Image) E2ERegionsVarName() string {
	if i.OS == OSWindows {
		return "windowsScenarioRegions"
	}
	return "scenarioRegions"
}

// NormalizeRegion converts a region name to the compact lowercase form ARM uses in resource
// IDs, so "West US 2", "westus2" and "WestUS2" all compare equal.
func NormalizeRegion(location string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(location), " ", ""))
}

func slicesContains(regions []string, normalized string) bool {
	for _, region := range regions {
		if region == normalized {
			return true
		}
	}
	return false
}
