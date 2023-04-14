package e2e_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	mrand "math/rand"

	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"golang.org/x/crypto/ssh"
)

// Returns a newly generated RSA public/private key pair with the private key in PEM format.
func getNewRSAKeyPair(r *mrand.Rand) (privatePEMBytes []byte, publicKeyBytes []byte, e error) {
	privateKey, err := rsa.GenerateKey(r, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rsa private key: %q", err)
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate rsa private key: %q", err)
	}

	publicRsaKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert private to public key: %q", err)
	}

	publicKeyBytes = ssh.MarshalAuthorizedKey(publicRsaKey)

	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEMBytes = pem.EncodeToMemory(&privBlock)

	return
}

func createVMSSWithPayload(ctx context.Context, customData, cseCmd, vmssName string, publicKeyBytes []byte, opts *scenarioRunOpts) (*armcompute.VirtualMachineScaleSet, error) {
	model := getBaseVMSSModel(vmssName, opts.suiteConfig.location, *opts.chosenCluster.Properties.NodeResourceGroup, opts.subnetID, string(publicKeyBytes), customData, cseCmd)

	isAzureCNI, err := opts.isChosenClusterAzureCNI()
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether chosen cluster uses Azure CNI from cluster model: %s", err)
	}

	if isAzureCNI {
		if err := addPodIPConfigsForAzureCNI(&model, vmssName, opts); err != nil {
			return nil, fmt.Errorf("failed to create pod IP configs for azure CNI scenario: %s", err)
		}
	}

	if opts.scenario.VMConfigMutator != nil {
		opts.scenario.VMConfigMutator(&model)
	}

	pollerResp, err := opts.cloud.vmssClient.BeginCreateOrUpdate(
		ctx,
		*opts.chosenCluster.Properties.NodeResourceGroup,
		vmssName,
		model,
		nil,
	)
	if err != nil {
		return nil, err
	}

	vmssResp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &vmssResp.VirtualMachineScaleSet, nil
}

// Adds additional IP configs to the passed in vmss model based on the chosen cluster's setting of "maxPodsPerNode",
// as we need be able to allow AKS to allocate an additional IP config for each pod running on the given node.
// Additional info: https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni
func addPodIPConfigsForAzureCNI(vmss *armcompute.VirtualMachineScaleSet, vmssName string, opts *scenarioRunOpts) error {
	maxPodsPerNode, err := opts.chosenClusterMaxPodsPerNode()
	if err != nil {
		return fmt.Errorf("failed to read agentpool MaxPods value from chosen cluster model: %s", err)
	}

	var podIPConfigs []*armcompute.VirtualMachineScaleSetIPConfiguration
	for i := 1; i <= maxPodsPerNode; i++ {
		ipConfig := &armcompute.VirtualMachineScaleSetIPConfiguration{
			Name: to.Ptr(fmt.Sprintf("%s%d", vmssName, i)),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				Subnet: &armcompute.APIEntityReference{
					ID: to.Ptr(opts.subnetID),
				},
			},
		}
		podIPConfigs = append(podIPConfigs, ipConfig)
	}
	vmssNICConfig := vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0]
	ipConfigs := append(vmssNICConfig.Properties.IPConfigurations, podIPConfigs...)
	vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations = ipConfigs
	return nil
}

func getVMPrivateIPAddress(ctx context.Context, cloud *azureClient, subscription, mcResourceGroupName, vmssName string) (string, error) {
	pl := cloud.coreClient.Pipeline()
	url := fmt.Sprintf(listVMSSNetworkInterfaceURLTemplate,
		subscription,
		mcResourceGroupName,
		vmssName,
		0,
	)
	req, err := runtime.NewRequest(ctx, "GET", url)
	if err != nil {
		return "", err
	}

	resp, err := pl.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var instanceNICResult listVMSSVMNetworkInterfaceResult

	if err := json.Unmarshal(respBytes, &instanceNICResult); err != nil {
		return "", err
	}

	privateIP := instanceNICResult.Value[0].Properties.IPConfigurations[0].Properties.PrivateIPAddress
	return privateIP, nil
}

func getBaseVMSSModel(name, location, mcResourceGroupName, subnetID, sshPublicKey, customData, cseCmd string) armcompute.VirtualMachineScaleSet {
	return armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(location),
		SKU: &armcompute.SKU{
			Name:     to.Ptr("Standard_DS2_v2"),
			Capacity: to.Ptr[int64](1),
		},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			Overprovision: to.Ptr(false),
			UpgradePolicy: &armcompute.UpgradePolicy{
				Mode: to.Ptr(armcompute.UpgradeModeManual),
			},
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				ExtensionProfile: &armcompute.VirtualMachineScaleSetExtensionProfile{
					Extensions: []*armcompute.VirtualMachineScaleSetExtension{
						{
							Name: to.Ptr("vmssCSE"),
							Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
								Publisher:               to.Ptr("Microsoft.Azure.Extensions"),
								Type:                    to.Ptr("CustomScript"),
								TypeHandlerVersion:      to.Ptr("2.0"),
								AutoUpgradeMinorVersion: to.Ptr(true),
								Settings:                map[string]interface{}{},
								ProtectedSettings: map[string]interface{}{
									"commandToExecute": cseCmd,
								},
							},
						},
					},
				},
				OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.Ptr(name),
					AdminUsername:      to.Ptr("azureuser"),
					CustomData:         &customData,
					LinuxConfiguration: &armcompute.LinuxConfiguration{
						SSH: &armcompute.SSHConfiguration{
							PublicKeys: []*armcompute.SSHPublicKey{
								{
									KeyData: to.Ptr(sshPublicKey),
									Path:    to.Ptr("/home/azureuser/.ssh/authorized_keys"),
								},
							},
						},
					},
				},
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &armcompute.ImageReference{
						ID: to.Ptr(scenario.DefaultImageVersionIDs["ubuntu1804"]),
					},
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{
						CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
						DiskSizeGB:   to.Ptr(int32(512)),
						OSType:       to.Ptr(armcompute.OperatingSystemTypesLinux),
					},
				},
				NetworkProfile: &armcompute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: []*armcompute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: to.Ptr(name),
							Properties: &armcompute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:            to.Ptr(true),
								EnableIPForwarding: to.Ptr(true),
								IPConfigurations: []*armcompute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: to.Ptr(fmt.Sprintf("%s0", name)),
										Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
											Primary: to.Ptr(true),
											LoadBalancerBackendAddressPools: []*armcompute.SubResource{
												{
													ID: to.Ptr(
														fmt.Sprintf(
															"/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/kubernetes/backendAddressPools/aksOutboundBackendPool",
															mcResourceGroupName,
														),
													),
												},
											},
											Subnet: &armcompute.APIEntityReference{
												ID: to.Ptr(subnetID),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
