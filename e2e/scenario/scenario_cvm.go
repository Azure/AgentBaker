package scenario

import (
	"context"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
)

const (
	cvmLocation = "westeurope"
	cvmSNPSKU   = "Standard_DC2ads_v5"
	cvmTDXSKU   = "Standard_DC2eds_v6"
)

var (
	_ = Register(newCVMScenario(
		"Ubuntu2004_CVM_SNP",
		"Tests that an Ubuntu 20.04 Confidential VM node can be provisioned and pass standard node validation",
		config.VHDUbuntu2004CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		false,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2404_CVM_SNP",
		"Tests that an Ubuntu 24.04 Confidential VM node can be provisioned and pass standard node validation",
		config.VHDUbuntu2404CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		false,
	))
	_ = Register(newCVMScenario(
		"AzureLinuxV3_CVM_SNP",
		"Tests that an Azure Linux V3 Confidential VM node can be provisioned and pass standard node validation",
		config.VHDAzureLinuxV3CVMGen2,
		ClusterKubenet,
		cvmSNPSKU,
		false,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2604Minimal_CVM_SNP",
		"Tests that an Ubuntu 26.04 minimal Confidential VM node can be provisioned on SEV-SNP and pass standard node validation",
		config.VHDUbuntu2604MinimalCVMGen2Containerd,
		ClusterLatestKubernetesVersionKubenet,
		cvmSNPSKU,
		false,
	))
	_ = Register(newCVMScenario(
		"Ubuntu2604Minimal_CVM_TDX",
		"Tests that an Ubuntu 26.04 minimal Confidential VM node can be provisioned on TDX and pass standard node validation",
		config.VHDUbuntu2604MinimalCVMGen2Containerd,
		ClusterLatestKubernetesVersionKubenet,
		cvmTDXSKU,
		true,
	))
)

func newCVMScenario(
	name string,
	description string,
	vhd *config.Image,
	cluster func(context.Context, ClusterRequest) (*Cluster, error),
	vmSize string,
	useNVMe bool,
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
				configureConfidentialVM(vmss, vmSize)
			},
		},
	}
}

func configureConfidentialVM(vmss *armcompute.VirtualMachineScaleSet, vmSize string) {
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
		to.Ptr(armcompute.SecurityEncryptionTypesVMGuestStateOnly)
}
