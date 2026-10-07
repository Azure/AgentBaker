package scenario

import (
	"context"
	"errors"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
)

// The ARM64 FIPS image requires a separate node from the existing x64 FIPS scenario.
var _ = Register(&Scenario{
	Name:        "ACL_ARM64_FIPS_TrustedLaunch",
	Description: "Bootstraps the ACL ARM64 FIPS image with Trusted Launch and verifies runtime FIPS",
	Tags:        Tags{VMSeriesCoverageTest: true},
	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDACLArm64Gen2FIPSTL,
		UseNVMe: true,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.IsARM64 = true
			nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_v6"
			nbc.AgentPoolProfile.LocalDNSProfile = nil
			nbc.ContainerService.Properties.AgentPoolProfiles[0].LocalDNSProfile = nil
		},
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			return errors.Join(
				ValidateFileHasContent(ctx, s, "/etc/os-release", "ID=azurelinux"),
				ValidateFileHasContent(ctx, s, "/etc/os-release", "VARIANT_ID=azurecontainerlinux"),
				ValidateACLFIPSEnabled(ctx, s),
				ValidateFIPSProvider(ctx, s),
			)
		},
	},
})
