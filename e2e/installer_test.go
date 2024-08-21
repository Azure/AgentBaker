package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstaller(t *testing.T) {
	t.Parallel()
	ctx := newTestCtx(t)
	err := ensureResourceGroup(ctx)
	require.NoError(t, err)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)
	vmssName := getVmssName(t)
	vm := createInstallerVMSSS(ctx, t, vmssName, privateKeyBytes, publicKeyBytes)
	t.Logf("VMSS %q created", *vm.ID)
}

func installerCSE(ctx context.Context, t *testing.T) string {
	return `curl https://www.google.com > /var/log/azure/cluster-provision-cse-output.log`
}

func createInstallerVMSSS(ctx context.Context, t *testing.T, vmssName string, privateKeyBytes, publicKeyBytes []byte) *armcompute.VirtualMachineScaleSet {
	clusterConfig, err := ClusterKubenet(ctx, t)
	require.NoError(t, err)
	t.Logf("creating VMSS %q in resource group %q", vmssName, *clusterConfig.Model.Properties.NodeResourceGroup)
	script := installerCSE(ctx, t)
	model := getBaseVMSSModel(vmssName, string(publicKeyBytes), "", script, clusterConfig)
	imageID, err := config.VHDUbuntu2204Gen2Containerd.VHDResourceID(ctx, t)
	require.NoError(t, err)
	model.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
		ID: to.Ptr(string(imageID)),
	}

	operation, err := config.Azure.VMSS.BeginCreateOrUpdate(
		ctx,
		*clusterConfig.Model.Properties.NodeResourceGroup,
		vmssName,
		model,
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupVMSS(ctx, t, vmssName, clusterConfig, privateKeyBytes)
	})

	vmssResp, err := operation.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
		Frequency: 10 * time.Second,
	})
	// fail test, but continue to extract debug information
	require.NoError(t, err, "create vmss %q, check %s for vm logs", vmssName, testDir(t))
	return &vmssResp.VirtualMachineScaleSet
}
