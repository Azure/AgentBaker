package e2e

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	listVMSSNetworkInterfaceURLTemplate      = "https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%d/networkInterfaces?api-version=2018-10-01"
	loadBalancerBackendAddressPoolIDTemplate = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/kubernetes/backendAddressPools/aksOutboundBackendPool"
	vmssNamePrefix                           = "abe2e"
)

func createVMSS(ctx context.Context, t *testing.T, vmssName string, opts *scenarioRunOpts, privateKeyBytes []byte, publicKeyBytes []byte) *armcompute.VirtualMachineScaleSet {
	t.Logf("creating VMSS %q in resource group %q", vmssName, *opts.clusterConfig.Model.Properties.NodeResourceGroup)
	nodeBootstrapping, err := getNodeBootstrapping(ctx, opts.nbc)
	require.NoError(t, err)

	model := getBaseVMSSModel(vmssName, string(publicKeyBytes), nodeBootstrapping.CustomData, nodeBootstrapping.CSE, opts)

	isAzureCNI, err := opts.clusterConfig.IsAzureCNI()
	require.NoError(t, err, vmssName, opts)

	if isAzureCNI {
		err = addPodIPConfigsForAzureCNI(&model, vmssName, opts)
		require.NoError(t, err)
	}

	opts.scenario.PrepareVMSSModel(ctx, t, &model)

	operation, err := config.Azure.VMSS.BeginCreateOrUpdate(
		ctx,
		*opts.clusterConfig.Model.Properties.NodeResourceGroup,
		vmssName,
		model,
		nil,
	)
	skipTestIfSKUNotAvailableErr(t, err)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupVMSS(ctx, t, vmssName, opts, privateKeyBytes)
	})

	vmssResp, err := operation.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	// fail test, but continue to extract debug information
	require.NoError(t, err, "create vmss %q, check %s for vm logs", vmssName, testDir(t))
	return &vmssResp.VirtualMachineScaleSet
}

func skipTestIfSKUNotAvailableErr(t *testing.T, err error) {
	// sometimes the SKU is not available and we can't do anything. Skip the test in this case.
	var respErr *azcore.ResponseError
	if config.Config.SkipTestsWithSKUCapacityIssue &&
		errors.As(err, &respErr) &&
		respErr.StatusCode == 409 &&
		respErr.ErrorCode == "SkuNotAvailable" {
		t.Skip("skipping scenario SKU not available", t.Name(), err)
	}
}

func cleanupVMSS(ctx context.Context, t *testing.T, vmssName string, opts *scenarioRunOpts, privateKeyBytes []byte) {
	// original context can be cancelled, but we still want to collect the logs
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	defer deleteVMSS(t, ctx, vmssName, opts, privateKeyBytes)

	vmPrivateIP, err := getVMPrivateIPAddress(ctx, *opts.clusterConfig.Model.Properties.NodeResourceGroup, vmssName)
	require.NoError(t, err)

	require.NoError(t, err, "get vm private IP %v", vmssName)
	logFiles, err := extractLogsFromVM(ctx, t, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
	require.NoError(t, err, "extract logs from vm %v", vmssName)

	err = dumpFileMapToDir(t, logFiles)
	require.NoError(t, err, "dump file map to dir %v", vmssName)

}

func deleteVMSS(t *testing.T, ctx context.Context, vmssName string, opts *scenarioRunOpts, privateKeyBytes []byte) {
	if config.Config.KeepVMSS {
		t.Logf("vmss %q will be retained for debugging purposes, please make sure to manually delete it later", vmssName)
		if err := writeToFile(t, "sshkey", string(privateKeyBytes)); err != nil {
			t.Logf("failed to write retained vmss %s private ssh key to disk: %s", vmssName, err)
		}
		return
	}
	_, err := config.Azure.VMSS.BeginDelete(ctx, *opts.clusterConfig.Model.Properties.NodeResourceGroup, vmssName, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
		ForceDeletion: to.Ptr(true),
	})
	if err != nil {
		t.Logf("failed to delete vmss %q: %s", vmssName, err)
		return
	}
	t.Logf("vmss %q deleted successfully", vmssName)
}

// Adds additional IP configs to the passed in vmss model based on the chosen cluster's setting of "maxPodsPerNode",
// as we need be able to allow AKS to allocate an additional IP config for each pod running on the given node.
// Additional info: https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni
func addPodIPConfigsForAzureCNI(vmss *armcompute.VirtualMachineScaleSet, vmssName string, opts *scenarioRunOpts) error {
	maxPodsPerNode, err := opts.clusterConfig.MaxPodsPerNode()
	if err != nil {
		return fmt.Errorf("failed to read agentpool MaxPods value from chosen cluster model: %w", err)
	}

	var podIPConfigs []*armcompute.VirtualMachineScaleSetIPConfiguration
	for i := 1; i <= maxPodsPerNode; i++ {
		ipConfig := &armcompute.VirtualMachineScaleSetIPConfiguration{
			Name: to.Ptr(fmt.Sprintf("%s%d", vmssName, i)),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				Subnet: &armcompute.APIEntityReference{
					ID: to.Ptr(opts.clusterConfig.SubnetID),
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

func getVMPrivateIPAddress(ctx context.Context, mcResourceGroupName, vmssName string) (string, error) {
	pl := config.Azure.Core.Pipeline()
	url := fmt.Sprintf(listVMSSNetworkInterfaceURLTemplate,
		config.Config.SubscriptionID,
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
func getNewRSAKeyPair() (privatePEMBytes []byte, publicKeyBytes []byte, e error) {
	privateKey, err := rsa.GenerateKey(crand.Reader, 4096)
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

func getVmssName(t *testing.T) string {
	name := fmt.Sprintf("%s-%s-%s-%s", vmssNamePrefix, time.Now().Format(time.DateOnly), randomLowercaseString(4), t.Name())
	// delete invalid characters like _ and /
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "Test", "")
	// truncate to 57 characters, as AKS has a limit of 64 characters for VM names
	// an additional prefix is generated for VM name
	if len(name) > 57 { // a limit for VMSS name
		name = name[:57]
	}
	// AKS converts VM names to lowercase at some stage, avoid potential matching issues
	name = strings.ToLower(name)
	return name
}

func getBaseVMSSModel(name, sshPublicKey, customData, cseCmd string, opts *scenarioRunOpts) armcompute.VirtualMachineScaleSet {
	return armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(config.Config.Location),
		SKU: &armcompute.SKU{
			Name:     to.Ptr("Standard_D2ds_v5"),
			Capacity: to.Ptr[int64](1),
		},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			Overprovision: to.Ptr(false),
			UpgradePolicy: &armcompute.UpgradePolicy{
				Mode: to.Ptr(armcompute.UpgradeModeAutomatic),
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
															config.Config.SubscriptionID,
															*opts.clusterConfig.Model.Properties.NodeResourceGroup,
														),
													),
												},
											},
											Subnet: &armcompute.APIEntityReference{
												ID: to.Ptr(opts.clusterConfig.SubnetID),
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
