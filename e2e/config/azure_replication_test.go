package config

import (
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

// TestFindRegionalReplicationStatus exercises the location-matching logic used to
// extract the per-region replication state from a SIG image version's ReplicationStatus
// summary. Region names from ARM may be in either the "WestUS 2" form or the "westus2"
// form, and may include arbitrary casing; the lookup must normalize both sides.
func TestFindRegionalReplicationStatus(t *testing.T) {
	completed := armcompute.ReplicationStateCompleted
	replicating := armcompute.ReplicationStateReplicating

	status := &armcompute.ReplicationStatus{
		Summary: []*armcompute.RegionalReplicationStatus{
			{Region: to.Ptr("East US"), State: &completed, Progress: to.Ptr(int32(100))},
			{Region: to.Ptr("West US 2"), State: &replicating, Progress: to.Ptr(int32(40))},
			{Region: to.Ptr("uaenorth"), State: &completed, Progress: to.Ptr(int32(100))},
			nil, // tolerate nil entries
			{Region: nil, State: &completed},
		},
	}

	tests := []struct {
		name       string
		status     *armcompute.ReplicationStatus
		location   string
		wantFound  bool
		wantState  armcompute.ReplicationState
		wantRegion string
	}{
		{name: "nil status", status: nil, location: "westus2"},
		{name: "exact lowercase normalized match (with space in summary)", status: status, location: "westus2", wantFound: true, wantState: replicating, wantRegion: "West US 2"},
		{name: "uppercase input match (with space in summary)", status: status, location: "EASTUS", wantFound: true, wantState: completed, wantRegion: "East US"},
		{name: "input with embedded spaces", status: status, location: "east us", wantFound: true, wantState: completed, wantRegion: "East US"},
		{name: "summary already normalized", status: status, location: "uaenorth", wantFound: true, wantState: completed, wantRegion: "uaenorth"},
		{name: "missing region returns nil", status: status, location: "centralus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findRegionalReplicationStatus(tt.status, tt.location)
			if !tt.wantFound {
				if got != nil {
					t.Fatalf("expected nil, got region %q state %v", *got.Region, *got.State)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected to find region %q, got nil", tt.wantRegion)
			}
			if got.Region == nil || *got.Region != tt.wantRegion {
				t.Errorf("region mismatch: want %q got %v", tt.wantRegion, got.Region)
			}
			if got.State == nil || *got.State != tt.wantState {
				t.Errorf("state mismatch: want %v got %v", tt.wantState, got.State)
			}
		})
	}
}

func regionNames(regions []*armcompute.TargetRegion) []string {
	names := make([]string, 0, len(regions))
	for _, region := range regions {
		names = append(names, NormalizeRegion(*region.Name))
	}
	slices.Sort(names)
	return names
}

func TestMergeTargetRegions(t *testing.T) {
	existing := []*armcompute.TargetRegion{
		{Name: to.Ptr("West US 2")},
		{Name: to.Ptr("eastus")},
	}

	merged, missing := mergeTargetRegions(existing, []string{"westus2", "EastUS", "uaenorth", " southeastasia ", ""})

	require.Equal(t, []string{"southeastasia", "uaenorth"}, missing)
	require.Equal(t, []string{"eastus", "southeastasia", "uaenorth", "westus2"}, regionNames(merged))
	// Existing entries must be passed through untouched so replica counts and storage
	// account types configured outside E2E are not rewritten.
	require.Equal(t, "West US 2", *merged[0].Name)
	require.Len(t, existing, 2, "input slice must not be mutated")

	_, missing = mergeTargetRegions(merged, []string{"westus2", "uaenorth"})
	require.Empty(t, missing, "merge must be idempotent once all regions are present")
}

// TestMergeTargetRegionsConverges is the property the whole design rests on: TargetRegions is
// full desired state, so a lost update is only harmful if writers disagree. Two writers that
// start from different stale snapshots but merge the same desired set produce the same result,
// so whichever write lands last is still correct.
func TestMergeTargetRegionsConverges(t *testing.T) {
	desired := []string{"eastus", "westus2", "uaenorth"}

	// Writer A read the version before anyone had replicated.
	a, _ := mergeTargetRegions(nil, desired)
	// Writer B read it after some other writer had already added westus2.
	b, _ := mergeTargetRegions([]*armcompute.TargetRegion{{Name: to.Ptr("westus2")}}, desired)

	require.Equal(t, regionNames(a), regionNames(b))
}

func TestE2ERegionsPerImageOS(t *testing.T) {
	original := Config.DefaultLocation
	t.Cleanup(func() { Config.DefaultLocation = original })
	Config.DefaultLocation = RegionWestUS3

	linux := &Image{OS: OSUbuntu}
	require.Equal(t, linuxE2ERegions, linux.E2ERegions())
	require.True(t, linux.SupportsE2ERegion(RegionUAENorth))

	// No Windows scenario pins a location, so Windows images must not be fanned out to
	// regions no Windows test uses - they are the largest images in the gallery.
	windows := &Image{OS: OSWindows}
	require.Equal(t, []string{RegionWestUS3}, windows.E2ERegions())
	require.False(t, windows.SupportsE2ERegion(RegionUAENorth))
	require.True(t, windows.SupportsE2ERegion("West US 3"))

	// A custom local E2E_LOCATION must still work, without duplicating a known region.
	Config.DefaultLocation = "North Europe"
	require.Contains(t, windows.E2ERegions(), "northeurope")
	require.True(t, windows.SupportsE2ERegion("northeurope"))
	Config.DefaultLocation = RegionWestUS3
	require.Equal(t, windowsE2ERegions, windows.E2ERegions())
}

// TestReplicationRegionsForEphemeralImage guards the cost/blast-radius carve-out: image
// versions captured at runtime for a single test have exactly one writer and are deleted on
// cleanup, so they must not be fanned out to the shared E2E region set.
func TestReplicationRegionsForEphemeralImage(t *testing.T) {
	shared := &Image{OS: OSUbuntu}
	require.Equal(t, append(shared.E2ERegions(), RegionWestUS2), shared.replicationRegions(RegionWestUS2))

	ephemeral := &Image{OS: OSUbuntu, Ephemeral: true}
	require.Equal(t, []string{RegionWestUS2}, ephemeral.replicationRegions(RegionWestUS2))
}

func TestHasTargetRegion(t *testing.T) {
	regions := []*armcompute.TargetRegion{{Name: to.Ptr("West US 2")}, nil, {}}
	require.True(t, hasTargetRegion(regions, "westus2"))
	require.True(t, hasTargetRegion(regions, "West US 2"))
	require.False(t, hasTargetRegion(regions, "eastus"))
	require.False(t, hasTargetRegion(nil, "westus2"))
}
