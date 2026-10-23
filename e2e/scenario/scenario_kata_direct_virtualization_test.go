package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func TestRequireKataGalleryFeatures(t *testing.T) {
	features := []*armcompute.GalleryImageFeature{
		{Name: to.Ptr("VirtualizationType"), Value: to.Ptr("Direct")},
		{Name: to.Ptr("DirectVirtualizationSchedulerType"), Value: to.Ptr("GuestManaged")},
		{Name: to.Ptr("DiskControllerTypes"), Value: to.Ptr("SCSI,NVMe")},
	}
	definition := &armcompute.GalleryImage{Properties: &armcompute.GalleryImageProperties{Features: features}}
	require.NoError(t, requireKataGalleryFeatures(definition))
	features[1].Value = to.Ptr("AzureManaged")
	require.ErrorContains(t, requireKataGalleryFeatures(definition), "GuestManaged")
	features[1].Value = to.Ptr("GuestManaged")
	features[0].StartsAtVersion = to.Ptr("99.0.0")
	require.ErrorContains(t, requireKataGalleryFeatures(definition), "definition-wide")
	features[0].StartsAtVersion = nil
	definition.Properties.Features = append(features, features[0])
	require.ErrorContains(t, requireKataGalleryFeatures(definition), "exactly one")
	definition.Properties.Features = features[1:]
	require.ErrorContains(t, requireKataGalleryFeatures(definition), "VirtualizationType")
	require.Error(t, requireKataGalleryFeatures(nil))
}

func TestKataDirectVirtualizationConfiguration(t *testing.T) {
	old := config.Config
	config.Config = config.DefaultConfiguration()
	t.Cleanup(func() { config.Config = old })
	require.Contains(t, kataDirectVirtualization.SkipIf(t.Context()), "KATA_DIRECT_VIRTUALIZATION_VM_SKU")
	config.Config.KataDirectVirtualizationVMSKU = "test-direct-sku"
	require.Empty(t, kataDirectVirtualization.SkipIf(t.Context()))
	config.Config.TestPreProvision = true
	require.Contains(t, kataDirectVirtualization.SkipIf(t.Context()), "recaptured")
	config.Config.TestPreProvision = false
	require.Same(t, config.VHDAzureLinuxV3Gen2Kata, kataDirectVirtualization.VHD)
	nbc := &datamodel.NodeBootstrappingConfiguration{
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
		ContainerService: &datamodel.ContainerService{Properties: &datamodel.Properties{
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{{}},
		}},
	}
	kataDirectVirtualization.BootstrapConfigMutator(nil, nbc)
	require.Equal(t, "test-direct-sku", nbc.AgentPoolProfile.VMSize)
	require.Equal(t, nbc.AgentPoolProfile.VMSize, nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize)
	for _, nvme := range []bool{true, false} {
		vmss := &armcompute.VirtualMachineScaleSet{SKU: &armcompute.SKU{}, Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{DiffDiskSettings: &armcompute.DiffDiskSettings{}},
				},
			},
		}}
		configureKataDirectVMSS(vmss, nbc.AgentPoolProfile.VMSize, nvme)
		require.Equal(t, nbc.AgentPoolProfile.VMSize, *vmss.SKU.Name)
		require.Nil(t, vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings)
		if nvme {
			require.Equal(t, "NVMe", *vmss.Properties.VirtualMachineProfile.StorageProfile.DiskControllerType)
		} else {
			require.Equal(t, "SCSI", *vmss.Properties.VirtualMachineProfile.StorageProfile.DiskControllerType)
		}
	}
}
