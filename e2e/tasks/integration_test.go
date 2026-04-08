package tasks

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Spec example task definitions ---
// Mirrors the complete example from the design spec:
// CreateRG → CreateVNet → CreateSubnet → CreateCluster → RunTests → Teardown

type createRGOutput struct {
	RGName string
}

type createRG struct {
	Output createRGOutput
}

func (t *createRG) Do(ctx context.Context) error {
	t.Output.RGName = "my-rg"
	return nil
}

type createVNet struct {
	Deps struct {
		RG *createRG
	}
	Output struct {
		VNetID string
	}
}

func (t *createVNet) Do(ctx context.Context) error {
	t.Output.VNetID = fmt.Sprintf("%s-vnet", t.Deps.RG.Output.RGName)
	return nil
}

type createSubnet struct {
	Deps struct {
		VNet *createVNet
	}
	Output struct {
		SubnetID string
	}
}

func (t *createSubnet) Do(ctx context.Context) error {
	t.Output.SubnetID = fmt.Sprintf("%s-subnet", t.Deps.VNet.Output.VNetID)
	return nil
}

type createCluster struct {
	Deps struct {
		RG     *createRG
		Subnet *createSubnet
	}
	Output struct {
		ClusterID string
	}
}

func (t *createCluster) Do(ctx context.Context) error {
	t.Output.ClusterID = fmt.Sprintf("cluster-in-%s-%s",
		t.Deps.RG.Output.RGName,
		t.Deps.Subnet.Output.SubnetID)
	return nil
}

type runTests struct {
	Deps struct {
		Cluster *createCluster
	}
	Output struct {
		Passed bool
	}
}

func (t *runTests) Do(ctx context.Context) error {
	t.Output.Passed = true
	return nil
}

type teardown struct {
	Deps struct {
		RG    *createRG
		Tests *runTests
	}
	Output struct {
		TornDown bool
	}
}

func (t *teardown) Do(ctx context.Context) error {
	t.Output.TornDown = true
	return nil
}

// --- Integration tests ---

func TestIntegration_SpecExample(t *testing.T) {
	// Wire up the full DAG from the spec:
	//
	//   CreateRG ──┬── CreateVNet ── CreateSubnet ──┐
	//              │                                 │
	//              ├──────────────── CreateCluster ──┘
	//              │                       │
	//              │                   RunTests
	//              │                       │
	//              └──────────────── Teardown
	rg := &createRG{}
	vnet := &createVNet{}
	vnet.Deps.RG = rg
	subnet := &createSubnet{}
	subnet.Deps.VNet = vnet
	cluster := &createCluster{}
	cluster.Deps.RG = rg
	cluster.Deps.Subnet = subnet
	tests := &runTests{}
	tests.Deps.Cluster = cluster
	td := &teardown{}
	td.Deps.RG = rg
	td.Deps.Tests = tests

	err := Execute(context.Background(), Config{}, td)
	require.NoError(t, err)

	// Verify all outputs propagated correctly
	assert.Equal(t, "my-rg", rg.Output.RGName)
	assert.Equal(t, "my-rg-vnet", vnet.Output.VNetID)
	assert.Equal(t, "my-rg-vnet-subnet", subnet.Output.SubnetID)
	assert.Equal(t, "cluster-in-my-rg-my-rg-vnet-subnet", cluster.Output.ClusterID)
	assert.True(t, tests.Output.Passed)
	assert.True(t, td.Output.TornDown)
}

func TestIntegration_SpecExample_WithMaxConcurrency(t *testing.T) {
	rg := &createRG{}
	vnet := &createVNet{}
	vnet.Deps.RG = rg
	subnet := &createSubnet{}
	subnet.Deps.VNet = vnet
	cluster := &createCluster{}
	cluster.Deps.RG = rg
	cluster.Deps.Subnet = subnet
	tests := &runTests{}
	tests.Deps.Cluster = cluster
	td := &teardown{}
	td.Deps.RG = rg
	td.Deps.Tests = tests

	err := Execute(context.Background(), Config{MaxConcurrency: 1}, td)
	require.NoError(t, err)

	assert.Equal(t, "cluster-in-my-rg-my-rg-vnet-subnet", cluster.Output.ClusterID)
	assert.True(t, td.Output.TornDown)
}

func TestIntegration_TransitiveDependencyAccess(t *testing.T) {
	// Verify that a task can read transitive dependencies through Deps chains
	// as described in the spec's "Accessing Transitive Dependencies" section.
	rg := &createRG{}
	vnet := &createVNet{}
	vnet.Deps.RG = rg
	subnet := &createSubnet{}
	subnet.Deps.VNet = vnet
	cluster := &createCluster{}
	cluster.Deps.RG = rg
	cluster.Deps.Subnet = subnet

	err := Execute(context.Background(), Config{}, cluster)
	require.NoError(t, err)

	// Access transitive dep: cluster -> subnet -> vnet -> rg
	rgName := cluster.Deps.Subnet.Deps.VNet.Deps.RG.Output.RGName
	assert.Equal(t, "my-rg", rgName)
}

// failingRunTests simulates a test failure mid-pipeline
type failingRunTests struct {
	Deps struct {
		Cluster *createCluster
	}
}

func (t *failingRunTests) Do(ctx context.Context) error {
	return fmt.Errorf("tests failed: 2 of 10 scenarios failed")
}

type teardownAfterFail struct {
	Deps struct {
		RG    *createRG
		Tests *failingRunTests
	}
	Output struct{ TornDown bool }
}

func (t *teardownAfterFail) Do(ctx context.Context) error {
	t.Output.TornDown = true
	return nil
}

func TestIntegration_MidPipelineFailure_CancelDependents(t *testing.T) {
	rg := &createRG{}
	vnet := &createVNet{}
	vnet.Deps.RG = rg
	subnet := &createSubnet{}
	subnet.Deps.VNet = vnet
	cluster := &createCluster{}
	cluster.Deps.RG = rg
	cluster.Deps.Subnet = subnet
	failTests := &failingRunTests{}
	failTests.Deps.Cluster = cluster

	td := &teardownAfterFail{}
	td.Deps.RG = rg
	td.Deps.Tests = failTests

	err := Execute(context.Background(), Config{OnError: CancelDependents}, td)
	require.Error(t, err)

	var dagErr *DAGError
	require.True(t, errors.As(err, &dagErr))

	// Upstream tasks should have succeeded
	assert.Equal(t, Succeeded, dagErr.Results[rg].Status)
	assert.Equal(t, Succeeded, dagErr.Results[vnet].Status)
	assert.Equal(t, Succeeded, dagErr.Results[subnet].Status)
	assert.Equal(t, Succeeded, dagErr.Results[cluster].Status)

	// failTests should have failed
	assert.Equal(t, Failed, dagErr.Results[failTests].Status)
	assert.Contains(t, dagErr.Results[failTests].Err.Error(), "tests failed")

	// teardown should be skipped since it depends on failTests
	assert.Equal(t, Skipped, dagErr.Results[td].Status)

	// Outputs of successful tasks should still be populated
	assert.Equal(t, "my-rg", rg.Output.RGName)
	assert.Equal(t, "cluster-in-my-rg-my-rg-vnet-subnet", cluster.Output.ClusterID)
}

func TestIntegration_TwoIndependentSubgraphs_SharedTask(t *testing.T) {
	// Two independent pipelines share CreateRG.
	// Both should complete, CreateRG should execute only once.
	rg := &createRG{}

	vnet1 := &createVNet{}
	vnet1.Deps.RG = rg
	vnet2 := &createVNet{}
	vnet2.Deps.RG = rg

	subnet1 := &createSubnet{}
	subnet1.Deps.VNet = vnet1
	subnet2 := &createSubnet{}
	subnet2.Deps.VNet = vnet2

	// Both subnets are roots; they share rg
	err := Execute(context.Background(), Config{}, subnet1, subnet2)
	require.NoError(t, err)

	assert.Equal(t, "my-rg", rg.Output.RGName)
	assert.Equal(t, "my-rg-vnet", vnet1.Output.VNetID)
	assert.Equal(t, "my-rg-vnet", vnet2.Output.VNetID)
	assert.Equal(t, "my-rg-vnet-subnet", subnet1.Output.SubnetID)
	assert.Equal(t, "my-rg-vnet-subnet", subnet2.Output.SubnetID)
}

func TestIntegration_EmptyGraph(t *testing.T) {
	err := Execute(context.Background(), Config{})
	require.NoError(t, err)
}
