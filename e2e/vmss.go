package e2e_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	mrand "math/rand"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"golang.org/x/crypto/ssh"
)

func createVMSSWithPayload(ctx context.Context, r *mrand.Rand, cloud *azureClient, location, resourceGroupName, name, subnetID string, customData, cseCmd string, mutator func(*armcompute.VirtualMachineScaleSet)) (sshPrivateKey []byte, e error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(r, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to create rsa private key: %q", err)
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate private key: %q", err)
	}

	publicRsaKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert private to public key: %q", err)
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEM := pem.EncodeToMemory(&privBlock)

	model := getBaseVMSSModel(name, location, subnetID, string(pubKeyBytes), customData, cseCmd)

	if mutator != nil {
		mutator(&model)
	}

	pollerResp, err := cloud.vmssClient.BeginCreateOrUpdate(
		ctx,
		agentbakerTestResourceGroupName,
		name,
		model,
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	_ = res

	return privatePEM, nil
}

func getBaseVMSSModel(name, location, subnetID, sshPublicKey, customData, cseCmd string) armcompute.VirtualMachineScaleSet {
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
						ID: to.Ptr("/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2/versions/1.1677169694.31375"),
						// 	Offer:     to.Ptr("0001-com-ubuntu-server-jammy"),
						// 	Publisher: to.Ptr("Canonical"),
						// 	SKU:       to.Ptr("22_04-lts-gen2"),
						// 	Version:   to.Ptr("latest"),
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
										Name: to.Ptr(name),
										Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
											LoadBalancerBackendAddressPools: []*armcompute.SubResource{
												{
													ID: to.Ptr("/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/MC_agentbaker-e2e-tests_agentbaker-e2e-test-cluster_eastus/providers/Microsoft.Network/loadBalancers/kubernetes/backendAddressPools/aksOutboundBackendPool"),
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
