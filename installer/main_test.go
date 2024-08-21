package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	subscriptionID    = "82acd5bb-4206-47d4-9c12-a65db028483d"
	resourceGroupName = "akhantimirov-test"
	location          = "westus3"
	SIGImageID        = "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2"
	vnetName          = "installer-test-vnet"
	subnetName        = "installer-test-subnet"
	vmNamePrefix      = "installer-test-vm-"
	sshPublicKeyPath  = "/Users/r2k1/.ssh/id_rsa.pub"
	sshPrivateKeyPath = "/Users/r2k1/.ssh/id_rsa"
	localBinaryPath   = "installer"
	remoteBinaryPath  = "/home/azureuser/installer"
	publicIPName      = "installer-test-ip"
	nsgName           = "installer-test-nsg"
)

var (
	defaultPollingOptions = &runtime.PollUntilDoneOptions{
		Frequency: 1 * time.Second,
	}
)

func TestInstaller(t *testing.T) {
	client := NewAzureClient(t)
	client.EnsureResourceGroup(context.Background())
	ip := client.CreateVM(context.Background(), client.CreateVNet(context.Background()))
	// Compile installer
	compileInstaller(t)
	sshClient := createSSHClient(t, ip, "azureuser")
	CopyAndExecuteBinary(t, sshClient, localBinaryPath, remoteBinaryPath)
}

func createSSHClient(t *testing.T, vmIP, username string) *ssh.Client {
	key, err := os.ReadFile(sshPrivateKeyPath)
	require.NoError(t, err)

	signer, err := ssh.ParsePrivateKey(key)
	require.NoError(t, err)

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", vmIP+":22", config)
	require.NoError(t, err)

	return client
}

func compileInstaller(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", localBinaryPath, "-v")
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=amd64",
	)
	err := cmd.Run()
	require.NoError(t, err)
	t.Logf("Compiled %s", localBinaryPath)
}

func CopyAndExecuteBinary(t *testing.T, sshClient *ssh.Client, localBinaryPath, remoteBinaryPath string) {

	sftpClient, err := sftp.NewClient(sshClient)
	require.NoError(t, err)
	dstFile, err := sftpClient.Create(remoteBinaryPath)
	require.NoError(t, err)
	defer dstFile.Close()
	content, err := os.ReadFile(localBinaryPath)
	require.NoError(t, err)
	_, err = dstFile.Write(content)
	require.NoError(t, err)
	t.Logf("Copied %s to %s", localBinaryPath, remoteBinaryPath)
	require.NoError(t, err)
	execSSH(t, sshClient, "chmod +x "+remoteBinaryPath)
	execSSH(t, sshClient, remoteBinaryPath)
}

func execSSH(t *testing.T, sshClient *ssh.Client, cmd string) string {
	session, err := sshClient.NewSession()
	require.NoError(t, err)
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	t.Logf("Output: %s", output)
	t.Logf("Executed %s", cmd)
	require.NoError(t, err)
	return string(output)
}

type AzureClient struct {
	Credential *azidentity.DefaultAzureCredential
	T          *testing.T
}

func NewAzureClient(t *testing.T) *AzureClient {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err)

	return &AzureClient{
		Credential: cred,
		T:          t,
	}
}

func (a *AzureClient) EnsureResourceGroup(ctx context.Context) {
	client, err := armresources.NewResourceGroupsClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)
	_, err = client.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(location),
	}, nil)
	require.NoError(a.T, err)
	a.T.Logf("Created resource group %s", resourceGroupName)
}

func (a *AzureClient) CreateVNet(ctx context.Context) string {
	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)
	subnetClient, err := armnetwork.NewSubnetsClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)

	vnetPoller, err := vnetClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					to.Ptr("10.0.0.0/16"),
				},
			},
		},
	}, nil)
	require.NoError(a.T, err)
	_, err = vnetPoller.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)

	poller, err := subnetClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, subnetName, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.0.0/24"),
		},
	}, nil)
	require.NoError(a.T, err)

	subnetResp, err := poller.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)

	a.T.Logf("Created VNet %s with subnet %s", vnetName, subnetName)

	return *subnetResp.ID
}

func (a *AzureClient) CreateVM(ctx context.Context, subnetID string) string {
	publicIP := a.EnsurePublicIP(ctx)
	nsgID := a.EnsureNSG(ctx)
	nic := a.EnsureNIC(ctx, subnetID, publicIP, nsgID)
	a.EnsureVM(ctx, nic)

	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)
	publicIPD, err := publicIPClient.Get(context.Background(), resourceGroupName, publicIPName, nil)
	require.NoError(a.T, err)

	return *publicIPD.PublicIPAddress.Properties.IPAddress
}

func (a *AzureClient) EnsureVM(ctx context.Context, nic armnetwork.InterfacesClientCreateOrUpdateResponse) {
	sshPublicKey, err := os.ReadFile(sshPublicKeyPath)
	require.NoError(a.T, err)

	// Cloud-init script to install and enable SSHD
	customData := `#cloud-config
package_update: true
packages:
	- openssh-server
runcmd:
	- systemctl enable ssh
	- systemctl start ssh
`

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_D2ds_v5")),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					ID: to.Ptr(SIGImageID),
				},
			},
			OSProfile: &armcompute.OSProfile{
				AdminUsername: to.Ptr("azureuser"),
				AdminPassword: to.Ptr("Password@123"), // Use a strong password or SSH key
				ComputerName:  to.Ptr("installer-test-vm"),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(false),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/home/azureuser/.ssh/authorized_keys"),
								KeyData: to.Ptr(string(sshPublicKey)),
							},
						},
					},
				},
				CustomData: to.Ptr(base64.StdEncoding.EncodeToString([]byte(customData))),
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: nic.ID,
					},
				},
			},
		},
	}
	vmName := vmNamePrefix + hash(a.T, vmParams)

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)
	_, err = vmClient.Get(context.Background(), resourceGroupName, vmName, nil)
	if err == nil {
		a.T.Logf("VM %s already exists", vmName)
		return
	}
	poller, err := vmClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, vmParams, nil)
	require.NoError(a.T, err)

	_, err = poller.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)

	a.T.Logf("Created VM %s", vmName)
}

func (a *AzureClient) EnsureNIC(ctx context.Context, subnetID string, publicIP armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, nsgID string) armnetwork.InterfacesClientCreateOrUpdateResponse {
	// Create NIC
	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)
	nicResp, err := nicClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmNamePrefix+"-nic", armnetwork.Interface{
		Location: to.Ptr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr(vmNamePrefix + "-ipconfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: &subnetID,
						},
						PublicIPAddress: &armnetwork.PublicIPAddress{
							ID: publicIP.ID,
						},
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: &nsgID,
			},
		},
	}, nil)
	require.NoError(a.T, err)

	nicRespFinal, err := nicResp.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)
	a.T.Logf("Created NIC %s", vmNamePrefix+"-nic")
	return nicRespFinal
}

func (a *AzureClient) EnsurePublicIP(ctx context.Context) armnetwork.PublicIPAddressesClientCreateOrUpdateResponse {
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)

	publicIPPoller, err := publicIPClient.BeginCreateOrUpdate(ctx, resourceGroupName, publicIPName, armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
		},
	}, nil)
	require.NoError(a.T, err)
	publicIP, err := publicIPPoller.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)
	a.T.Logf("Created public IP %s", publicIPName)
	return publicIP
}

func (a *AzureClient) EnsureNSG(ctx context.Context) string {
	nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, a.Credential, nil)
	require.NoError(a.T, err)

	nsgPoller, err := nsgClient.BeginCreateOrUpdate(ctx, resourceGroupName, nsgName, armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("AllowSSH"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						SourcePortRange:          to.Ptr("*"),
						DestinationPortRange:     to.Ptr("22"),
						SourceAddressPrefix:      to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
						Priority:                 to.Ptr[int32](100),
					},
				},
			},
		},
	}, nil)
	require.NoError(a.T, err)

	nsg, err := nsgPoller.PollUntilDone(ctx, defaultPollingOptions)
	require.NoError(a.T, err)
	a.T.Logf("Created NSG %s", nsgName)
	return *nsg.ID
}

func hash(t *testing.T, obj any) string {
	jsonData, err := json.Marshal(obj)
	require.NoError(t, err)
	hasher := sha256.New()
	_, err = hasher.Write(jsonData)
	require.NoError(t, err)
	hexHash := hex.EncodeToString(hasher.Sum(nil))
	return hexHash[:5]
}
