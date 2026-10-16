package scenario

import (
	"context"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
)

const (
	cvmLocation = "westus"
	cvmSNPSKU   = "Standard_DC2ads_v5"
	cvmTDXSKU   = "Standard_DC2eds_v6"
)

type cvmPlatform string

const (
	cvmPlatformSEVSNP cvmPlatform = "sevsnp"
	cvmPlatformTDX    cvmPlatform = "tdx"
)

var (
	_ = Register(newCVMScenario(
		"Ubuntu2004_CVM_SNP",
		"Tests that an Ubuntu 20.04 Confidential VM node can be provisioned, pass standard node validation, and attest as SEV-SNP",
		config.VHDUbuntu2004CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2404_CVM_SNP",
		"Tests that an Ubuntu 24.04 Confidential VM node can be provisioned, pass standard node validation, and attest as SEV-SNP",
		config.VHDUbuntu2404CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
	))
	_ = Register(newCVMScenario(
		"AzureLinuxV3_CVM_SNP",
		"Tests that an Azure Linux V3 Confidential VM node can be provisioned, pass standard node validation, and attest as SEV-SNP",
		config.VHDAzureLinuxV3CVMGen2,
		ClusterKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
	))
	_ = Register(newCVMScenario(
		"AzureContainerLinux_CVM_SNP",
		"Tests that an Azure Container Linux Confidential VM node can be provisioned, pass standard node validation, and attest as SEV-SNP",
		config.VHDACLCVMGen2,
		ClusterKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2604Minimal_CVM_SNP",
		"Tests that an Ubuntu 26.04 minimal Confidential VM node can be provisioned, pass standard node validation, and attest as SEV-SNP",
		config.VHDUbuntu2604MinimalCVMGen2Containerd,
		ClusterLatestKubernetesVersionKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
		armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2604Minimal_CVM_TDX",
		"Tests that an Ubuntu 26.04 minimal Confidential VM node can be provisioned, pass standard node validation, and attest as TDX",
		config.VHDUbuntu2604MinimalCVMGen2Containerd,
		ClusterLatestKubernetesVersionKubenet,
		cvmTDXSKU,
		cvmPlatformTDX,
		true,
		armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
	))
)

func newCVMScenario(
	name string,
	description string,
	vhd *config.Image,
	cluster func(context.Context, ClusterRequest) (*Cluster, error),
	vmSize string,
	platform cvmPlatform,
	useNVMe bool,
	securityEncryptionType armcompute.SecurityEncryptionTypes,
) *Scenario {
	return &Scenario{
		Name:        name,
		Description: description,
		Location:    cvmLocation,
		Tags: Tags{
			CVM: true,
		},
		Config: Config{
			Cluster: cluster,
			VHD:     vhd,
			UseNVMe: useNVMe,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				configureConfidentialVM(vmss, vmSize, securityEncryptionType)
			},
			Validator: validateCVM(platform, securityEncryptionType),
		},
	}
}

func configureConfidentialVM(
	vmss *armcompute.VirtualMachineScaleSet,
	vmSize string,
	securityEncryptionType armcompute.SecurityEncryptionTypes,
) {
	if vmss.SKU == nil {
		vmss.SKU = &armcompute.SKU{}
	}
	vmss.SKU.Name = to.Ptr(vmSize)

	if vmss.Properties == nil {
		vmss.Properties = &armcompute.VirtualMachineScaleSetProperties{}
	}
	if vmss.Properties.VirtualMachineProfile == nil {
		vmss.Properties.VirtualMachineProfile = &armcompute.VirtualMachineScaleSetVMProfile{}
	}
	profile := vmss.Properties.VirtualMachineProfile
	if profile.SecurityProfile == nil {
		profile.SecurityProfile = &armcompute.SecurityProfile{}
	}
	profile.SecurityProfile.SecurityType = to.Ptr(armcompute.SecurityTypesConfidentialVM)
	profile.SecurityProfile.EncryptionAtHost = to.Ptr(true)
	if profile.SecurityProfile.UefiSettings == nil {
		profile.SecurityProfile.UefiSettings = &armcompute.UefiSettings{}
	}
	profile.SecurityProfile.UefiSettings.SecureBootEnabled = to.Ptr(true)
	profile.SecurityProfile.UefiSettings.VTpmEnabled = to.Ptr(true)

	if profile.StorageProfile == nil {
		profile.StorageProfile = &armcompute.VirtualMachineScaleSetStorageProfile{}
	}
	if profile.StorageProfile.OSDisk == nil {
		profile.StorageProfile.OSDisk = &armcompute.VirtualMachineScaleSetOSDisk{}
	}
	if profile.StorageProfile.OSDisk.ManagedDisk == nil {
		profile.StorageProfile.OSDisk.ManagedDisk = &armcompute.VirtualMachineScaleSetManagedDiskParameters{}
	}
	if profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile == nil {
		profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile = &armcompute.VMDiskSecurityProfile{}
	}
	profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType =
		to.Ptr(securityEncryptionType)
}
