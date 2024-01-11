package e2e_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"golang.org/x/crypto/ssh"
)

const (
	vmssNameTemplate                         = "abtest%s"
	listVMSSNetworkInterfaceURLTemplate      = "https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%d/networkInterfaces?api-version=2018-10-01"
	loadBalancerBackendAddressPoolIDTemplate = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/kubernetes/backendAddressPools/aksOutboundBackendPool"
)

func bootstrapVMSS(ctx context.Context, t *testing.T, r *mrand.Rand, vmssName string, opts *scenarioRunOpts, publicKeyBytes []byte) (*armcompute.VirtualMachineScaleSet, func(), error) {
	nodeBootstrapping, err := getNodeBootstrapping(ctx, opts.nbc)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get node bootstrapping: %w", err)
	}

	cleanupVMSS := func() {
		log.Printf("deleting vmss %q", vmssName)
		if _, err := pollVMSSOperation(ctx, vmssName, pollVMSSOperationOpts{
			pollingInterval: to.Ptr(deleteVMSSPollInterval),
			pollingTimeout:  to.Ptr(deleteVMSSPollingTimeout),
		}, func() (Poller[armcompute.VirtualMachineScaleSetsClientDeleteResponse], error) {
			return opts.cloud.vmssClient.BeginDelete(ctx, *opts.clusterConfig.cluster.Properties.NodeResourceGroup, vmssName, nil)
		}); err != nil {
			t.Errorf("encountered an error while waiting for deletion of vmss %q: %s", vmssName, err)
		}
		log.Printf("finished deleting vmss %q", vmssName)
	}

	vmssModel, err := createVMSSWithPayload(ctx, nodeBootstrapping.CustomData, nodeBootstrapping.CSE, vmssName, publicKeyBytes, opts)
	if err != nil {
		return nil, cleanupVMSS, fmt.Errorf("unable to create VMSS with payload: %w", err)
	}

	return vmssModel, cleanupVMSS, nil
}

func createVMSSWithPayload(ctx context.Context, customData, cseCmd, vmssName string, publicKeyBytes []byte, opts *scenarioRunOpts) (*armcompute.VirtualMachineScaleSet, error) {
	model := getBaseVMSSModel(vmssName, string(publicKeyBytes), customData, cseCmd, opts)

	if opts.suiteConfig.BuildID != "" {
		if model.Tags == nil {
			model.Tags = map[string]*string{}
		}
		model.Tags[buildIDTagKey] = &opts.suiteConfig.BuildID
	}

	isAzureCNI, err := opts.clusterConfig.isAzureCNI()
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether chosen cluster uses Azure CNI from cluster model: %w", err)
	}

	if isAzureCNI {
		if err := addPodIPConfigsForAzureCNI(&model, vmssName, opts); err != nil {
			return nil, fmt.Errorf("failed to create pod IP configs for azure CNI scenario: %w", err)
		}
	}

	if err := opts.scenario.PrepareVMSSModel(&model); err != nil {
		return nil, fmt.Errorf("unable to prepare model for VMSS %q: %w", vmssName, err)
	}

	createVMSSCtx, cancel := context.WithTimeout(ctx, vmssClientCreateVMSSPollingTimeout)
	defer cancel()

	vmssResp, err := pollVMSSOperation(createVMSSCtx, vmssName, pollVMSSOperationOpts{
		pollUntilDone: &runtime.PollUntilDoneOptions{
			Frequency: vmssClientCreateVMSSPollInterval,
		},
	},
		func() (Poller[armcompute.VirtualMachineScaleSetsClientCreateOrUpdateResponse], error) {
			return opts.cloud.vmssClient.BeginCreateOrUpdate(
				ctx,
				*opts.clusterConfig.cluster.Properties.NodeResourceGroup,
				vmssName,
				model,
				nil,
			)
		})
	if err != nil {
		return nil, fmt.Errorf("unable to create VMSS %q: %w", vmssName, err)
	}

	return &vmssResp.VirtualMachineScaleSet, nil
}

// Adds additional IP configs to the passed in vmss model based on the chosen cluster's setting of "maxPodsPerNode",
// as we need be able to allow AKS to allocate an additional IP config for each pod running on the given node.
// Additional info: https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni
func addPodIPConfigsForAzureCNI(vmss *armcompute.VirtualMachineScaleSet, vmssName string, opts *scenarioRunOpts) error {
	maxPodsPerNode, err := opts.clusterConfig.maxPodsPerNode()
	if err != nil {
		return fmt.Errorf("failed to read agentpool MaxPods value from chosen cluster model: %w", err)
	}

	var podIPConfigs []*armcompute.VirtualMachineScaleSetIPConfiguration
	for i := 1; i <= maxPodsPerNode; i++ {
		ipConfig := &armcompute.VirtualMachineScaleSetIPConfiguration{
			Name: to.Ptr(fmt.Sprintf("%s%d", vmssName, i)),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				Subnet: &armcompute.APIEntityReference{
					ID: to.Ptr(opts.clusterConfig.subnetId),
				},
			},
		}
		podIPConfigs = append(podIPConfigs, ipConfig)
	}
	vmssNICConfig, err := getVMSSNICConfig(vmss)
	if err != nil {
		return fmt.Errorf("unable to get vmss nic: %w", err)
	}
	vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations =
		append(vmssNICConfig.Properties.IPConfigurations, podIPConfigs...)
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

	privateIP, err := getPrivateIP(instanceNICResult)
	if err != nil {
		return "", err
	}

	return privateIP, nil
}

// Returns a newly generated RSA public/private key pair with the private key in PEM format.
func getNewRSAKeyPair(r *mrand.Rand) (privatePEMBytes []byte, publicKeyBytes []byte, e error) {
	privateKey, err := rsa.GenerateKey(r, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rsa private key: %w", err)
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate rsa private key: %w", err)
	}

	publicRsaKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert private to public key: %w", err)
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

func getVmssName(r *mrand.Rand) string {
	return fmt.Sprintf(vmssNameTemplate, randomLowercaseString(r, 4))
}

func getBaseVMSSModel(name, sshPublicKey, customData, cseCmd string, opts *scenarioRunOpts) armcompute.VirtualMachineScaleSet {
	return armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(opts.suiteConfig.Location),
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
						ID: to.Ptr(string(scenario.BaseVHDCatalog.Ubuntu1804.Gen2Containerd.ResourceID)),
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
															loadBalancerBackendAddressPoolIDTemplate,
															opts.suiteConfig.Subscription,
															*opts.clusterConfig.cluster.Properties.NodeResourceGroup,
														),
													),
												},
											},
											Subnet: &armcompute.APIEntityReference{
												ID: to.Ptr(opts.clusterConfig.subnetId),
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

func getPrivateIP(res listVMSSVMNetworkInterfaceResult) (string, error) {
	if len(res.Value) > 0 {
		v := res.Value[0]
		if len(v.Properties.IPConfigurations) > 0 {
			ipconfig := v.Properties.IPConfigurations[0]
			return ipconfig.Properties.PrivateIPAddress, nil
		}
	}
	return "", fmt.Errorf("unable to extract private IP address from listVMSSNetworkInterfaceResult:\n%+v", res)
}

func getVMSSNICConfig(vmss *armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSetNetworkConfiguration, error) {
	if vmss != nil && vmss.Properties != nil &&
		vmss.Properties.VirtualMachineProfile != nil && vmss.Properties.VirtualMachineProfile.NetworkProfile != nil {
		networkProfile := vmss.Properties.VirtualMachineProfile.NetworkProfile
		if len(networkProfile.NetworkInterfaceConfigurations) > 0 {
			return networkProfile.NetworkInterfaceConfigurations[0], nil
		}
	}
	return nil, fmt.Errorf("unable to extract vmss nic info, vmss model or vmss model properties were nil/empty:\n%+v", vmss)
}

type listVMSSVMNetworkInterfaceResult struct {
	Value []struct {
		Name       string `json:"name,omitempty"`
		ID         string `json:"id,omitempty"`
		Properties struct {
			ProvisioningState string `json:"provisioningState,omitempty"`
			IPConfigurations  []struct {
				Name       string `json:"name,omitempty"`
				ID         string `json:"id,omitempty"`
				Properties struct {
					ProvisioningState         string `json:"provisioningState,omitempty"`
					PrivateIPAddress          string `json:"privateIPAddress,omitempty"`
					PrivateIPAllocationMethod string `json:"privateIPAllocationMethod,omitempty"`
					PublicIPAddress           struct {
						ID string `json:"id,omitempty"`
					} `json:"publicIPAddress,omitempty"`
					Subnet struct {
						ID string `json:"id,omitempty"`
					} `json:"subnet,omitempty"`
					Primary                         bool   `json:"primary,omitempty"`
					PrivateIPAddressVersion         string `json:"privateIPAddressVersion,omitempty"`
					LoadBalancerBackendAddressPools []struct {
						ID string `json:"id,omitempty"`
					} `json:"loadBalancerBackendAddressPools,omitempty"`
					LoadBalancerInboundNatRules []struct {
						ID string `json:"id,omitempty"`
					} `json:"loadBalancerInboundNatRules,omitempty"`
				} `json:"properties,omitempty"`
			} `json:"ipConfigurations,omitempty"`
			DNSSettings struct {
				DNSServers               []interface{} `json:"dnsServers,omitempty"`
				AppliedDNSServers        []interface{} `json:"appliedDnsServers,omitempty"`
				InternalDomainNameSuffix string        `json:"internalDomainNameSuffix,omitempty"`
			} `json:"dnsSettings,omitempty"`
			MacAddress                  string `json:"macAddress,omitempty"`
			EnableAcceleratedNetworking bool   `json:"enableAcceleratedNetworking,omitempty"`
			EnableIPForwarding          bool   `json:"enableIPForwarding,omitempty"`
			NetworkSecurityGroup        struct {
				ID string `json:"id,omitempty"`
			} `json:"networkSecurityGroup,omitempty"`
			Primary        bool `json:"primary,omitempty"`
			VirtualMachine struct {
				ID string `json:"id,omitempty"`
			} `json:"virtualMachine,omitempty"`
		} `json:"properties,omitempty"`
	} `json:"value,omitempty"`
}
