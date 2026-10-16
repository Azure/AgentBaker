package scenario

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

var cachedKataDirectSKU = cachedFunc(func(ctx context.Context, req VMSizeSKURequest) (*armcompute.ResourceSKU, error) {
	return config.Azure.RequireDirectVirtualizationSKU(ctx, req.Location, req.VMSize)
})

// A separate node is necessary: the ordinary Kata scenario continues to cover a
// nested-virtualization VM, while this one must use an official direct-virt SKU.
var kataDirectVirtualization = Register(&Scenario{
	Name:        "AzureLinuxV3Gen2Kata_DirectVirtualization",
	Description: "Validates Direct/GuestManaged gallery features and BusyBox VM isolation under both Kata handlers on a direct-virtualization SKU",
	Tags:        Tags{Kata: true},
	SkipIf: func(context.Context) string {
		if config.Config.TestPreProvision {
			return "direct virtualization scenario requires the PR-built image, not a separately recaptured preprovision image"
		}
		if strings.TrimSpace(config.Config.KataDirectVirtualizationVMSKU) == "" {
			return "set KATA_DIRECT_VIRTUALIZATION_VM_SKU or --kata-direct-virtualization-vm-sku to an available L1VH SKU"
		}
		return ""
	},
	Config: Config{
		VHD: config.VHDAzureLinuxV3Gen2Kata,
		Cluster: func(ctx context.Context, request ClusterRequest) (*Cluster, error) {
			// Check capability and the selected image before creating cluster infrastructure.
			if _, err := cachedKataDirectSKU(ctx, VMSizeSKURequest{Location: request.Location, VMSize: config.Config.KataDirectVirtualizationVMSKU}); err != nil {
				return nil, err
			}
			imageID, err := CachedPrepareVHD(ctx, GetVHDRequest{Image: *config.VHDAzureLinuxV3Gen2Kata, Location: request.Location})
			if err != nil {
				return nil, err
			}
			if err := validateKataGalleryFeatures(ctx, string(imageID)); err != nil {
				return nil, err
			}
			return ClusterKubenet(ctx, request)
		},
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.DisableUnattendedUpgrades = false
			nbc.AgentPoolProfile.VMSize = config.Config.KataDirectVirtualizationVMSKU
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = config.Config.KataDirectVirtualizationVMSKU
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			sku, err := cachedKataDirectSKU(ctx, VMSizeSKURequest{Location: *vmss.Location, VMSize: config.Config.KataDirectVirtualizationVMSKU})
			if err != nil {
				return err
			}
			configureKataDirectVMSS(vmss, config.Config.KataDirectVirtualizationVMSKU, config.SkuSupportsNVMe(sku))
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			if s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VMSS == nil || s.Runtime.VM.VMSS.SKU == nil || s.Runtime.VM.VMSS.SKU.Name == nil || !strings.EqualFold(*s.Runtime.VM.VMSS.SKU.Name, config.Config.KataDirectVirtualizationVMSKU) {
				return fmt.Errorf("Kata direct virtualization VMSS did not use the requested SKU")
			}
			return ValidateKataWorkloads(ctx, s)
		},
	},
})

func configureKataDirectVMSS(vmss *armcompute.VirtualMachineScaleSet, size string, nvme bool) {
	vmss.SKU.Name = to.Ptr(size)
	storage := vmss.Properties.VirtualMachineProfile.StorageProfile
	// Avoid requiring local/ephemeral OS disk capacity on the preview SKU.
	storage.OSDisk.DiffDiskSettings = nil
	storage.DiskControllerType = to.Ptr("SCSI")
	if nvme {
		storage.DiskControllerType = to.Ptr("NVMe")
	}
}

func validateKataGalleryFeatures(ctx context.Context, imageID string) error {
	definition, err := config.Azure.GetKataImageDefinition(ctx, imageID)
	if err != nil {
		return err
	}
	if err := requireKataGalleryFeatures(definition); err != nil {
		return fmt.Errorf("Kata image %s: %w", imageID, err)
	}
	logging.Logf(ctx, "Kata image %s advertises VirtualizationType=Direct and DirectVirtualizationSchedulerType=GuestManaged", imageID)
	return nil
}

func requireKataGalleryFeatures(definition *armcompute.GalleryImage) error {
	if definition == nil || definition.Properties == nil {
		return fmt.Errorf("missing gallery image definition properties")
	}
	for name, value := range map[string]string{"VirtualizationType": "Direct", "DirectVirtualizationSchedulerType": "GuestManaged"} {
		matches := 0
		for _, feature := range definition.Properties.Features {
			if feature != nil && feature.Name != nil && strings.EqualFold(*feature.Name, name) {
				if feature.Value == nil || *feature.Value != value || (feature.StartsAtVersion != nil && *feature.StartsAtVersion != "") {
					return fmt.Errorf("expected definition-wide %s=%s", name, value)
				}
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("expected exactly one %s=%s feature, got %d", name, value, matches)
		}
	}
	return nil
}
