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
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	loadBalancerBackendAddressPoolIDTemplate = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/kubernetes/backendAddressPools/aksOutboundBackendPool"
)

func compileAndUploadAKSNodeController(ctx context.Context, arch string) (string, error) {
	binary, err := compileAKSNodeController(ctx, arch)
	if err != nil {
		return "", err
	}
	uniqueSuffix := randomLowercaseString(6)
	blobPath := fmt.Sprintf("%s/aks-node-controller-%s", time.Now().UTC().Format("2006-01-02-15-04-05"), uniqueSuffix)
	logf(ctx, "uploading aks-node-controller binary to blob path %s", blobPath)
	url, err := config.Azure.UploadAndGetSignedLink(ctx, blobPath, binary)
	if err != nil {
		return "", fmt.Errorf("failed to upload aks-node-controller binary: %w", err)
	}
	return url, nil
}

// compileAndUploadAKSNodeController compiles the aks-node-controller binary for the given architecture.
func compileAKSNodeController(ctx context.Context, arch string) (*os.File, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("failed to find go binary in PATH: %w", err)
	}
	binName := "aks-node-controller-" + arch
	cmd := exec.CommandContext(ctx, goBin, "build", "-o", binName, "-v")
	cmd.Dir = filepath.Join("..", "aks-node-controller")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH="+arch,
	)
	logf(ctx, "compiling aks-node-controller: %q", cmd.String())
	log, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to compile aks-node-controller: %s", string(log))
	}
	f, err := os.Open(filepath.Join("..", "aks-node-controller", binName))
	if err != nil {
		return nil, fmt.Errorf("failed to open compiled aks-node-controller binary: %w", err)
	}
	return f, nil
}

func ConfigureAndCreateVMSS(ctx context.Context, s *Scenario) *armcompute.VirtualMachineScaleSet {
	model := createVMMSModel(ctx, s)

	vmss, err := CreateVMSSWithRetry(ctx, s, model)
	s.T.Cleanup(func() {
		cleanupVMSS(ctx, s)
	})
	skipTestIfSKUNotAvailableErr(s.T, err)
	// fail test, but continue to extract debug information
	require.NoError(s.T, err, "create vmss %q, check %s for vm logs", s.Runtime.VMSSName, testDir(s.T))

	return vmss
}

func createVMMSModel(ctx context.Context, s *Scenario) armcompute.VirtualMachineScaleSet {
	cluster := s.Runtime.Cluster
	var nodeBootstrapping *datamodel.NodeBootstrapping
	ab, err := agent.NewAgentBaker()
	require.NoError(s.T, err)
	var cse, customData string
	if s.Runtime.AKSNodeConfig != nil {
		if !config.Config.DisableScriptLessCompilation {
			s.Runtime.AKSNodeConfig.AksNodeControllerUrl, err = CachedCompileAndUploadAKSNodeController(ctx, s.VHD.Arch)
			require.NoError(s.T, err)
		}
		s.T.Logf("creating VMSS %q with AKSNodeConfigMutator in resource group %s", s.Runtime.VMSSName, *cluster.Model.Properties.NodeResourceGroup)
		cse = nodeconfigutils.CSE
		customData, err = nodeconfigutils.CustomData(s.Runtime.AKSNodeConfig)
		require.NoError(s.T, err)
	} else {
		s.T.Logf("creating VMSS %q with BootstrapConfigMutator/NBC in resource group %s", s.Runtime.VMSSName, *cluster.Model.Properties.NodeResourceGroup)
		nodeBootstrapping, err = ab.GetNodeBootstrapping(ctx, s.Runtime.NBC)
		require.NoError(s.T, err)
		cse = nodeBootstrapping.CSE
		customData = nodeBootstrapping.CustomData
	}

	// These two links are really for local development
	if config.Config.IsLocalBuild() {
		s.T.Logf(
			"VMSS portal link: https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/overview",
			config.Config.SubscriptionID,
			*cluster.Model.Properties.NodeResourceGroup,
			s.Runtime.VMSSName,
		)
		s.T.Logf(
			"Managed cluster portal link: https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s/overview",
			config.Config.SubscriptionID,
			*cluster.Model.Properties.NodeResourceGroup,
			*cluster.Model.Name,
		)
	}

	model := getBaseVMSSModel(s, customData, cse, s.Location)
	if s.Tags.NonAnonymousACR {
		// add acr pull identity
		userAssignedIdentity := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s",
			config.Config.SubscriptionID,
			config.ResourceGroupName(s.Location),
			config.VMIdentityName,
		)
		model.Identity = &armcompute.VirtualMachineScaleSetIdentity{
			Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssignedUserAssigned),
			UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
				userAssignedIdentity: {},
			},
		}
	}

	isAzureCNI, err := cluster.IsAzureCNI()
	require.NoError(s.T, err, "checking if cluster is using Azure CNI")

	if isAzureCNI {
		err = addPodIPConfigsForAzureCNI(&model, s.Runtime.VMSSName, cluster)
		require.NoError(s.T, err)
	}

	s.PrepareVMSSModel(ctx, s.T, &model)
	return model
}

func CreateVMSSWithRetry(ctx context.Context, s *Scenario, parameters armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error) {
	delay := 5 * time.Second
	retryOn := func(err error) bool {
		var respErr *azcore.ResponseError
		// AllocationFailed sometimes happens for exotic SKUs (new GPUs) with limited availability, sometimes retrying helps
		// It's not a quota issue
		return errors.As(err, &respErr) && respErr.StatusCode == 200 && respErr.ErrorCode == "AllocationFailed"
	}

	maxAttempts := 10
	attempt := 0

	for {
		attempt++
		vmss, err := CreateVMSS(ctx, s, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, parameters)
		if err == nil {
			logf(ctx, "created VMSS %s in resource group %s", s.Runtime.VMSSName, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup)
			return vmss, nil
		}

		// not a retryable error
		if !retryOn(err) {
			return nil, err
		}

		if attempt >= maxAttempts {
			return nil, fmt.Errorf("failed to create VMSS after %d retries: %w", maxAttempts, err)
		}

		logf(ctx, "failed to create VMSS: %v, attempt: %v, retrying in %v", err, attempt, delay)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(delay):
		}
	}

}

func CreateVMSS(ctx context.Context, s *Scenario, resourceGroupName string, vmssName string, parameters armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error) {
	operation, err := config.Azure.VMSS.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		vmssName,
		parameters,
		nil,
	)
	if err != nil {
		return nil, err
	}
	// We want to generate SSH instructions as soon as possible, so we can debug CSE issues
	s.Runtime.VMPrivateIP, err = waitForVMPrivateIP(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM private IP address: %w", err)
	}
	err = uploadSSHKey(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to upload ssh key: %w", err)
	}

	vmssResp, err := operation.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, err
	}
	return &vmssResp.VirtualMachineScaleSet, nil
}

// waitForVMPrivateIP polls until a private IP is available or the timeout elapses.
func waitForVMPrivateIP(ctx context.Context, s *Scenario) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ticker := time.NewTicker(config.Config.DefaultPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		ip, err := getVMPrivateIPAddress(ctxTimeout, s)
		if err == nil && ip != "" {
			return ip, nil
		}
		lastErr = err
		select {
		case <-ctxTimeout.Done():
			return "", fmt.Errorf("timeout waiting for private IP: %w", lastErr)
		case <-ticker.C:
		}
	}
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

func cleanupVMSS(ctx context.Context, s *Scenario) {
	// original context can be cancelled, but we still want to collect the logs
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancel()
	defer deleteVMSS(ctx, s)
	extractLogsFromVM(ctx, s)
}

func extractLogsFromVM(ctx context.Context, s *Scenario) {
	if s.IsWindows() {
		extractLogsFromVMWindows(ctx, s)
	} else {
		err := extractLogsFromVMLinux(ctx, s)
		if err != nil {
			s.T.Logf("failed to extract logs from VM: %s", err)
		} else {
			s.T.Logf("extracted VM logs to %s", testDir(s.T))
		}
		err = extractBootDiagnostics(ctx, s)
		if err != nil {
			s.T.Logf("failed to extract boot diagnostics from VM: %s", err)
		}
	}
}

func extractBootDiagnostics(ctx context.Context, s *Scenario) error {
	// Only extract boot diagnostics for Linux VMs
	if s.VHD.OS == config.OSWindows {
		return nil
	}

	pager := config.Azure.VMSSVM.NewListPager(*s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get VMSS instances: %v", err)
		}

		for _, vmInstance := range page.Value {
			// Get boot diagnostics data
			bootDiagResp, err := config.Azure.VMSSVM.RetrieveBootDiagnosticsData(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *vmInstance.InstanceID, nil)
			if err != nil {
				return fmt.Errorf("failed to get boot diagnostics for VM %s: %v", *vmInstance.InstanceID, err)
			}

			if bootDiagResp.SerialConsoleLogBlobURI == nil {
				continue
			}
			if *bootDiagResp.SerialConsoleLogBlobURI == "" {
				continue
			}
			// Save serial console log if available
			logFile := fmt.Sprintf("serial-console-vm-%s.log", *vmInstance.InstanceID)
			attempts := 0
			for {
				if attempts >= 3 {
					s.T.Logf("failed to download serial console log for VM %s after 3 attempts", *vmInstance.InstanceID)
					break
				}
				attempts++

				httpClient := config.NewHttpClient()
				resp, err := httpClient.Get(*bootDiagResp.SerialConsoleLogBlobURI)
				if err != nil {
					s.T.Logf("failed to download serial console log for VM %s: %v", *vmInstance.InstanceID, err)
					continue
				}
				body := resp.Body
				defer body.Close()

				contents, err := io.ReadAll(body)
				if err != nil {
					s.T.Logf("failed to read serial console log for VM %s: %v", *vmInstance.InstanceID, err)
					continue
				}
				if err := writeToFile(s.T, logFile, string(contents)); err != nil {
					s.T.Logf("failed to write serial console log for VM %s: %v", *vmInstance.InstanceID, err)
					continue
				}
				break
			}
		}
	}
	return nil
}

func extractLogsFromVMLinux(ctx context.Context, s *Scenario) error {
	privateIP, err := getVMPrivateIPAddress(ctx, s)
	if err != nil {
		return fmt.Errorf("failed to get VM private IP address: %w", err)
	}

	syslogHandle := "syslog"
	if s.VHD.OS == config.OSMariner || s.VHD.OS == config.OSAzureLinux {
		syslogHandle = "messages"
	}

	commandList := map[string]string{
		"cluster-provision.log":            "sudo cat /var/log/azure/cluster-provision.log",
		"kubelet.log":                      "sudo journalctl -u kubelet",
		"aks-log-collector.log":            "sudo journalctl -u aks-log-collector",
		"cluster-provision-cse-output.log": "sudo cat /var/log/azure/cluster-provision-cse-output.log",
		"sysctl-out.log":                   "sudo sysctl -a",
		"aks-node-controller.log":          "sudo cat /var/log/azure/aks-node-controller.log",
		"syslog":                           "sudo cat /var/log/" + syslogHandle,
	}
	if s.VHD.OS == config.OSFlatcar {
		commandList["journald"] = "sudo journalctl --boot=0 --no-pager"
	}

	pod, err := s.Runtime.Cluster.Kube.GetHostNetworkDebugPod(ctx)
	if err != nil {
		return fmt.Errorf("failed to get host network debug pod: %w", err)
	}

	var logFiles = map[string]string{}
	for file, sourceCmd := range commandList {
		execResult, err := execBashCommandOnVM(ctx, s, privateIP, pod.Name, sourceCmd)
		if err != nil {
			s.T.Logf("error executing %s: %s", sourceCmd, err)
			continue
		}
		logFiles[file] = execResult.String()
	}
	err = dumpFileMapToDir(s.T, logFiles)
	if err != nil {
		return fmt.Errorf("failed to dump log files: %w", err)
	}
	return nil
}

func execBashCommandOnVM(ctx context.Context, s *Scenario, vmPrivateIP, jumpboxPodName, command string) (*podExecResult, error) {
	script := Script{
		interpreter: Bash,
		script:      command,
	}
	return execScriptOnVm(ctx, s, vmPrivateIP, jumpboxPodName, script)
}

const uploadLogsPowershellScript = `
param(
    [string]$arg1,
    [string]$arg2,
    [string]$arg3
)

Invoke-WebRequest -UseBasicParsing https://aka.ms/downloadazcopy-v10-windows -OutFile azcopy.zip
Expand-Archive azcopy.zip
cd .\azcopy\*
$env:AZCOPY_AUTO_LOGIN_TYPE="MSI"
$env:AZCOPY_MSI_RESOURCE_STRING=$arg3
C:\k\debug\collect-windows-logs.ps1
$CollectedLogs=(Get-ChildItem . -Filter "*_logs.zip" -File)[0].Name
.\azcopy.exe copy $CollectedLogs "$arg1/collected-node-logs.zip"
.\azcopy.exe copy "C:\azuredata\CustomDataSetupScript.log" "$arg1/cse.log"
.\azcopy.exe copy "C:\AzureData\provision.complete" "$arg1/provision.complete"
.\azcopy.exe copy "C:\k\kubelet.err.log" "$arg1/kubelet.err.log"
.\azcopy.exe copy "C:\k\containerd.err.log" "$arg1/containerd.err.log"

# Collect network configuration information
ipconfig /all > network_config.txt
Get-NetIPConfiguration -Detailed >> network_config.txt
Get-NetAdapter | Format-Table -AutoSize >> network_config.txt
Get-DnsClientServerAddress >> network_config.txt
Get-NetRoute >> network_config.txt
Get-NetNat >> network_config.txt
Get-NetIPAddress >> network_config.txt
Get-NetNeighbor >> network_config.txt
Get-NetConnectionProfile >> network_config.txt
hnsdiag list networks >> network_config.txt
hnsdiag list endpoints >> network_config.txt
.\azcopy.exe copy "network_config.txt" "$arg1/network_config.txt"
`

// extractLogsFromVMWindows runs a script on windows VM to collect logs and upload them to a blob storage
// it then lists the blobs in the container and prints the content of each blob
func extractLogsFromVMWindows(ctx context.Context, s *Scenario) {
	if !s.T.Failed() {
		s.T.Logf("skipping logs extraction from windows VM, as the test didn't fail")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	pager := config.Azure.VMSSVM.NewListPager(*s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, nil)
	page, err := pager.NextPage(ctx)
	if err != nil {
		s.T.Logf("failed to list VMSS instances: %s", err)
		return
	}
	if len(page.Value) == 0 {
		s.T.Logf("no VMSS instances found")
		return
	}
	instanceID := *page.Value[0].InstanceID
	blobPrefix := s.Runtime.VMSSName
	blobUrl := config.Config.BlobStorageAccountURL() + "/" + config.Config.BlobContainer + "/" + blobPrefix

	client := config.Azure.VMSSVMRunCommands

	// Invoke the RunCommand on the VMSS instance
	s.T.Logf("uploading windows logs to blob storage at %s, may take a few minutes", blobUrl)

	azurePortalURL := fmt.Sprintf(
		"https://portal.azure.com/?feature.customportal=false#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s/containersList",
		config.Config.SubscriptionID,
		config.ResourceGroupName(s.Location),
		config.Config.BlobStorageAccount(),
	)

	s.T.Logf("Storage account %s in Azure portal: %s", blobPrefix, azurePortalURL)
	s.T.Logf("##vso[task.logissue type=warning;]Storage account %s in Azure portal: %s", blobPrefix, azurePortalURL)

	runCommandTimeout := int32((20 * time.Minute).Seconds())
	s.T.Logf("run command timeout: %d", runCommandTimeout)

	pollerResp, err := client.BeginCreateOrUpdate(
		ctx,
		*s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
		s.Runtime.VMSSName,
		instanceID,
		"RunPowerShellScript",
		armcompute.VirtualMachineRunCommand{
			Properties: &armcompute.VirtualMachineRunCommandProperties{
				TimeoutInSeconds: to.Ptr(runCommandTimeout), // 20 minutes should be enough
				Source: &armcompute.VirtualMachineRunCommandScriptSource{
					//CommandID: to.Ptr("RunPowerShellScript"),
					Script: to.Ptr(uploadLogsPowershellScript),
				},
				Parameters: []*armcompute.RunCommandInputParameter{
					{
						Name:  to.Ptr("arg1"),
						Value: to.Ptr(blobUrl),
					},
					{
						Name:  to.Ptr("arg2"),
						Value: to.Ptr(s.Runtime.VMSSName),
					},
					{
						Name:  to.Ptr("arg3"),
						Value: to.Ptr(config.Config.VMIdentityResourceID(s.Location)),
					},
				},
			},
		},
		nil,
	)
	require.NoError(s.T, err, "failed to initiate run command on VMSS instance %s", instanceID)

	// Poll the result until the operation is completed
	runCommandResp, err := pollerResp.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	require.NoError(s.T, err, "failed to poll run command on VMSS instance %s", instanceID)

	respJSON, _ := json.MarshalIndent(runCommandResp, "", "  ")
	s.T.Logf("run command executed successfully:\n%s", respJSON)

	s.T.Logf("uploaded logs to %s", blobUrl)

	downloadBlob := func(blobSuffix string) {
		fileName := filepath.Join(testDir(s.T), blobSuffix)
		err := os.MkdirAll(testDir(s.T), 0755)
		if err != nil {
			s.T.Logf("failed to create directory %q: %s", testDir(s.T), err)
			return
		}
		file, err := os.Create(fileName)
		if err != nil {
			s.T.Logf("failed to create file %q: %s", fileName, err)
			return
		}
		// NOTE, read after write is possible, list blobs is eventually consistent and may fail
		_, err = config.Azure.Blob.DownloadFile(ctx, config.Config.BlobContainer, blobPrefix+"/"+blobSuffix, file, nil)
		if err != nil {
			s.T.Logf("failed to download collected logs: %s", err)
			err = os.Remove(file.Name())
			if err != nil {
				s.T.Logf("failed to remove file: %s", err)
			}
			return
		}
	}
	downloadBlob("collected-node-logs.zip")
	downloadBlob("cse.log")
	downloadBlob("provision.complete")
	downloadBlob("network_config.txt")
	s.T.Logf("logs collected to %s", testDir(s.T))
}

func deleteVMSS(ctx context.Context, s *Scenario) {
	// original context can be cancelled, but we still want to delete the VM
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	if config.Config.KeepVMSS {
		s.T.Logf("vmss %q will be retained for debugging purposes, please make sure to manually delete it later", s.Runtime.VMSSName)
		if err := writeToFile(s.T, "sshkey", string(s.Runtime.SSHKeyPrivate)); err != nil {
			s.T.Logf("failed to write retained vmss %s private ssh key to disk: %s", s.Runtime.VMSSName, err)
		}
		return
	}
	_, err := config.Azure.VMSS.BeginDelete(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
		ForceDeletion: to.Ptr(true),
	})
	if err != nil {
		s.T.Logf("failed to delete vmss %q: %s", s.Runtime.VMSSName, err)
		return
	}
	s.T.Logf("vmss %q deleted successfully", s.Runtime.VMSSName)
}

// Adds additional IP configs to the passed in vmss model based on the chosen cluster's setting of "maxPodsPerNode",
// as we need be able to allow AKS to allocate an additional IP config for each pod running on the given node.
// Additional info: https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni
func addPodIPConfigsForAzureCNI(vmss *armcompute.VirtualMachineScaleSet, vmssName string, cluster *Cluster) error {
	maxPodsPerNode, err := cluster.MaxPodsPerNode()
	if err != nil {
		return fmt.Errorf("failed to read agentpool MaxPods value from chosen cluster model: %w", err)
	}

	var podIPConfigs []*armcompute.VirtualMachineScaleSetIPConfiguration
	for i := 1; i <= maxPodsPerNode; i++ {
		ipConfig := &armcompute.VirtualMachineScaleSetIPConfiguration{
			Name: to.Ptr(fmt.Sprintf("%s%d", vmssName, i)),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				Subnet: &armcompute.APIEntityReference{
					ID: to.Ptr(cluster.SubnetID),
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

func getVMPrivateIPAddress(ctx context.Context, s *Scenario) (string, error) {
	pager := config.Azure.NetworkInterfaces.NewListVirtualMachineScaleSetVMNetworkInterfacesPager(
		*s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
		s.Runtime.VMSSName,
		"0", // VM instance index
		nil,
	)

	if !pager.More() {
		return "", fmt.Errorf("no network interfaces found for VMSS %s", s.Runtime.VMSSName)
	}

	page, err := pager.NextPage(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces for VMSS %s: %w", s.Runtime.VMSSName, err)
	}

	if len(page.Value) == 0 {
		return "", fmt.Errorf("no network interfaces found for VMSS %s", s.Runtime.VMSSName)
	}

	networkInterface := page.Value[0]
	if networkInterface.Properties == nil || networkInterface.Properties.IPConfigurations == nil || len(networkInterface.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("no IP configurations found for network interface %s", *networkInterface.ID)
	}

	ipConfig := networkInterface.Properties.IPConfigurations[0]
	if ipConfig.Properties == nil || ipConfig.Properties.PrivateIPAddress == nil {
		return "", fmt.Errorf("no private IP address found for IP configuration %s", *ipConfig.ID)
	}

	return *ipConfig.Properties.PrivateIPAddress, nil
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

func generateVMSSNameLinux(t *testing.T) string {
	name := fmt.Sprintf("%s-%s-%s", randomLowercaseString(4), time.Now().Format(time.DateOnly), t.Name())
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "Test", "")
	name = strings.ToLower(name)
	if len(name) > 57 { // a limit for VMSS name
		name = name[:57]
	}
	return name
}

func generateVMSSNameWindows() string {
	// windows has a limit of 9 characters for VMSS name
	// and doesn't allow "-"
	return fmt.Sprintf("win%s", randomLowercaseString(4))
}

func generateVMSSName(s *Scenario) string {
	if s.IsWindows() {
		return generateVMSSNameWindows()
	}
	return generateVMSSNameLinux(s.T)
}

func getBaseVMSSModel(s *Scenario, customData, cseCmd, location string) armcompute.VirtualMachineScaleSet {
	model := armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(location),
		SKU: &armcompute.SKU{
			Name:     to.Ptr(config.Config.DefaultVMSKU),
			Capacity: to.Ptr[int64](1),
		},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			Overprovision: to.Ptr(false),
			UpgradePolicy: &armcompute.UpgradePolicy{
				Mode: to.Ptr(armcompute.UpgradeModeAutomatic),
			},
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				DiagnosticsProfile: &armcompute.DiagnosticsProfile{
					BootDiagnostics: &armcompute.BootDiagnostics{
						Enabled: to.Ptr(true),
					},
				},
				OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.Ptr(s.Runtime.VMSSName),
					AdminUsername:      to.Ptr("azureuser"),
					CustomData:         &customData,
					LinuxConfiguration: &armcompute.LinuxConfiguration{
						SSH: &armcompute.SSHConfiguration{
							PublicKeys: []*armcompute.SSHPublicKey{
								{
									KeyData: to.Ptr(string(s.Runtime.SSHKeyPublic)),
									Path:    to.Ptr("/home/azureuser/.ssh/authorized_keys"),
								},
							},
						},
					},
				},
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{
						CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
						DiskSizeGB:   to.Ptr(int32(50)),
						OSType:       to.Ptr(armcompute.OperatingSystemTypesLinux),
						Caching:      to.Ptr(armcompute.CachingTypesReadOnly),
						DiffDiskSettings: &armcompute.DiffDiskSettings{
							Option: to.Ptr(armcompute.DiffDiskOptionsLocal),
						},
					},
				},
				NetworkProfile: &armcompute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: []*armcompute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: to.Ptr(s.Runtime.VMSSName),
							Properties: &armcompute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:            to.Ptr(true),
								EnableIPForwarding: to.Ptr(true),
								IPConfigurations: []*armcompute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: to.Ptr(fmt.Sprintf("%s0", s.Runtime.VMSSName)),
										Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
											Primary:                 to.Ptr(true),
											PrivateIPAddressVersion: to.Ptr(armcompute.IPVersionIPv4),
											LoadBalancerBackendAddressPools: []*armcompute.SubResource{
												{
													ID: to.Ptr(
														fmt.Sprintf(
															loadBalancerBackendAddressPoolIDTemplate,
															config.Config.SubscriptionID,
															*s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
														),
													),
												},
											},
											Subnet: &armcompute.APIEntityReference{
												ID: to.Ptr(s.Runtime.Cluster.SubnetID),
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

	if cseCmd != "" {
		model.Properties.VirtualMachineProfile.ExtensionProfile = &armcompute.VirtualMachineScaleSetExtensionProfile{
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
		}
	}
	if s.IsWindows() {
		model.Identity = &armcompute.VirtualMachineScaleSetIdentity{
			Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssignedUserAssigned),
			UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
				config.Config.VMIdentityResourceID(s.Location): {},
			},
		}
		model.Properties.VirtualMachineProfile.StorageProfile.OSDisk.OSType = to.Ptr(armcompute.OperatingSystemTypesWindows)
		model.Properties.VirtualMachineProfile.OSProfile.LinuxConfiguration = nil
		model.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Publisher = to.Ptr("Microsoft.Compute")
		model.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Type = to.Ptr("CustomScriptExtension")
		model.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.TypeHandlerVersion = to.Ptr("1.10")
		model.Properties.VirtualMachineProfile.OSProfile.AdminUsername = to.Ptr("azureuser")
		model.Properties.VirtualMachineProfile.OSProfile.AdminPassword = to.Ptr(generateWindowsPassword())
	}
	return model
}

func generateWindowsPassword() string {
	if config.Config.WindowsAdminPassword != "" {
		return config.Config.WindowsAdminPassword
	}
	return randomStringWithDigitsAndSymbols(16)
}

func randomStringWithDigitsAndSymbols(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	const symbols = "!@#$%^&*()-_+="
	b := make([]byte, length)

	for i := range b {
		if i < (length - 2) { // Ensure at least 2 characters are symbols
			b[i] = charset[randomInt(len(charset))]
		} else if i == (length - 2) {
			b[i] = digits[randomInt(len(digits))]
		} else {
			b[i] = symbols[randomInt(len(symbols))]
		}
	}
	return string(b)
}

func randomInt(bound int) int {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(bound)))
	if err != nil {
		panic(err) // Intentionally panic for simplicity; handle errors as needed
	}
	return int(n.Int64())
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
