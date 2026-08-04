package config

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
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
