package config

import (
	"slices"
	"strings"
)

// e2eRegions is every region E2E runs in. Scenarios name their region inline; add it here too
// when a scenario needs a new one, otherwise images are never replicated there and the
// scenario fails. TestScenarioRegionsAreReplicated catches the common case of forgetting.
var e2eRegions = []string{
	"eastus",
	"southcentralus",
	"southeastasia",
	"uaenorth",
	"westus2",
	"westus3", // Config.DefaultLocation: where scenarios that name no region run
}

// replicationRegions returns the regions an image version is replicated to.
//
// The result deliberately does not depend on the region the caller wants. TargetRegions is
// full desired state, so a writer that appends only its own region computes a different
// desired state from every other writer and its update can drop their regions - which is what
// made scenarios fail with GalleryImageNotFound. A fixed set makes every writer submit the
// same list, so a lost update loses nothing and no locking or merging is needed.
func (i *Image) replicationRegions() []string {
	switch {
	case i.Ephemeral:
		// Throwaway image created and deleted by one test: single writer, and no reason to
		// pay to replicate it anywhere.
		return nil
	case i.OS == OSWindows:
		// No Windows scenario names a region, and Windows images are large enough that
		// replicating them where no Windows test looks is pure cost.
		return []string{"westus3"}
	default:
		return e2eRegions
	}
}

// SupportsE2ERegion reports whether scenarios using this image are replicated to location.
func (i *Image) SupportsE2ERegion(location string) bool {
	return i.Ephemeral || slices.Contains(i.replicationRegions(), NormalizeRegion(location))
}

// NormalizeRegion converts a region name to the compact lowercase form ARM uses in resource
// IDs, so "West US 2", "westus2" and "WestUS2" compare equal.
func NormalizeRegion(location string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(location), " ", ""))
}
