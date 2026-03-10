package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
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
	toolkit.Logf(ctx, "uploading aks-node-controller binary to blob path %s", blobPath)
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
	toolkit.Logf(ctx, "compiling aks-node-controller: %q", cmd.String())
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

func ConfigureAndCreateVMSS(ctx context.Context, s *Scenario) (*ScenarioVM, error) {
	vm, err := CreateVMSSWithRetry(ctx, s)
	skipTestIfSKUNotAvailableErr(s.T, err)

	return vm, err
}

// CustomDataWithHack is similar to nodeconfigutils.CustomData, but it uses a hack to run new aks-node-controller binary
// Original aks-node-controller isn't run because it fails systemd check validating aks-node-controller-config.json exists
// check aks-node-controller.service for details
// a new binary is downloaded from the given URL and run with provision command
func CustomDataWithHack(s *Scenario, binaryURL string) (string, error) {
	cloudConfigTemplate := `#cloud-config
write_files:
- path: /opt/azure/containers/aks-node-controller-config-hack.json
  permissions: "0755"
  owner: root
  content: !!binary |
   %s
runcmd:
 - mkdir -p /opt/azure/bin
 - curl -fSL "%s" -o /opt/azure/bin/aks-node-controller-hack
 - chmod +x /opt/azure/bin/aks-node-controller-hack
 - /opt/azure/bin/aks-node-controller-hack provision --provision-config=/opt/azure/containers/aks-node-controller-config-hack.json &
`
	if s.VHD.Flatcar {
		cloudConfigTemplate = `#cloud-config
write_files:
- path: /opt/azure/containers/aks-node-controller-config-hack.json
  permissions: "0755"
  owner: root
  content: !!binary |
   %s
- path: /opt/azure/bin/run-aks-node-controller-hack.sh
  permissions: "0755"
  owner: root
  content: |
    #!/bin/bash
    set -euo pipefail
    mkdir -p /opt/azure/bin
    curl -fSL "%s" -o /opt/azure/bin/aks-node-controller-hack
    chmod +x /opt/azure/bin/aks-node-controller-hack
    /opt/azure/bin/aks-node-controller-hack provision --provision-config=/opt/azure/containers/aks-node-controller-config-hack.json
# Flatcar specific configuration. It supports only a subset of cloud-init features https://github.com/flatcar/coreos-cloudinit/blob/main/Documentation/cloud-config.md#coreos-parameters
coreos:
  units:
    - name: aks-node-controller-hack.service
      command: start
      content: |
        [Unit]
        Description=Downloads and runs the AKS node controller hack
        After=network-online.target
        Wants=network-online.target
        [Service]
        Type=oneshot
        ExecStart=/opt/azure/bin/run-aks-node-controller-hack.sh
        [Install]
        WantedBy=multi-user.target
`
	}

	aksNodeConfigJSON, err := nodeconfigutils.MarshalConfigurationV1(s.Runtime.AKSNodeConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}
	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	customDataYAML := fmt.Sprintf(cloudConfigTemplate, encodedAksNodeConfigJSON, binaryURL)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

func createVMSSModel(ctx context.Context, s *Scenario) armcompute.VirtualMachineScaleSet {
	cluster := s.Runtime.Cluster
	var nodeBootstrapping *datamodel.NodeBootstrapping
	ab, err := agent.NewAgentBaker()
	require.NoError(s.T, err)
	var cse, customData string
	if s.Runtime.AKSNodeConfig != nil {
		cse = nodeconfigutils.CSE
		customData = func() string {
			if config.Config.DisableScriptLessCompilation {
				data, err := nodeconfigutils.CustomData(s.Runtime.AKSNodeConfig)
				require.NoError(s.T, err, "failed to generate custom data from AKSNodeConfig")
				return data
			}
			binaryURL, err := CachedCompileAndUploadAKSNodeController(ctx, s.VHD.Arch)
			require.NoError(s.T, err, "failed to compile and upload aks-node-controller binary")
			data, err := CustomDataWithHack(s, binaryURL)
			require.NoError(s.T, err, "failed to generate custom data from AKSNodeConfig with hack")
			return data
		}()

	} else {
		nodeBootstrapping, err = ab.GetNodeBootstrapping(ctx, s.Runtime.NBC)
		require.NoError(s.T, err)
		cse = nodeBootstrapping.CSE
		customData = nodeBootstrapping.CustomData
		if s.Runtime.NBC.EnableScriptlessCSECmd {
			// Validate that the custom data doesn't contain any script content,
			// which indicates that the scriptless CSE is working as intended
			decodedCustomData, err := base64.StdEncoding.DecodeString(customData)
			require.NoError(s.T, err, "failed to decode custom data")
			reader, err := gzip.NewReader(bytes.NewReader(decodedCustomData))
			require.NoError(s.T, err, "failed to create gzip reader")
			result, err := io.ReadAll(reader)
			require.NoError(s.T, err, "failed to read gzip data")
			reader.Close()
			require.Contains(s.T, string(result), "/opt/azure/containers/scriptless-cse-overrides.txt", "custom data contains other script content, but scriptless CSE CMD is enabled")
		}
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

	model := getBaseVMSSModel(s, customData, cse)

	// always assign the kubelet and e2e VM identities to the VMSS
	model.Identity = &armcompute.VirtualMachineScaleSetIdentity{
		Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssignedUserAssigned),
		UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
			*s.Runtime.Cluster.KubeletIdentity.ResourceID:  {},
			config.Config.VMIdentityResourceID(s.Location): {},
		},
	}

	isAzureCNI, err := cluster.IsAzureCNI()
	require.NoError(s.T, err, "checking if cluster is using Azure CNI")

	if isAzureCNI {
		err = addPodIPConfigsForAzureCNI(&model, s.Runtime.VMSSName, cluster)
		require.NoError(s.T, err)
	}

	s.PrepareVMSSModel(ctx, s.T, &model)

	if s.Config.UseNVMe {
		model.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings.Placement = to.Ptr(armcompute.DiffDiskPlacementNvmeDisk)
	}
	return model
}

func CreateVMSSWithRetry(ctx context.Context, s *Scenario) (*ScenarioVM, error) {
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
		vm, err := CreateVMSS(ctx, s, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup)
		if err == nil {
			return vm, nil
		}

		// not a retryable error
		if !retryOn(err) {
			return vm, err
		}

		if attempt >= maxAttempts {
			return vm, fmt.Errorf("failed to create VMSS after %d retries: %w", maxAttempts, err)
		}

		toolkit.Logf(ctx, "failed to create VMSS: %v, attempt: %v, retrying in %v", err, attempt, delay)
		select {
		case <-ctx.Done():
			return vm, err
		case <-time.After(delay):
		}
	}
}

func CreateVMSS(ctx context.Context, s *Scenario, resourceGroupName string) (*ScenarioVM, error) {
	defer toolkit.LogStepCtxf(ctx, "creating VMSS %s", s.Runtime.VMSSName)()
	vm := &ScenarioVM{}
	operation, err := config.Azure.VMSS.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		s.Runtime.VMSSName,
		createVMSSModel(ctx, s),
		nil,
	)
	if err != nil {
		return vm, err
	}
	// We want to generate SSH instructions as soon as possible, so we can debug CSE issues
	// Wait for VMSS VM to appear before extracting the private IP
	vm.VM, err = waitForVMSSVM(ctx, s)
	if err != nil {
		return vm, fmt.Errorf("failed to wait for VMSS VM: %w", err)
	}

	vm.PrivateIP, err = getPrivateIPFromVMSSVM(ctx, resourceGroupName, s.Runtime.VMSSName, *vm.VM.InstanceID)
	if err != nil {
		return vm, fmt.Errorf("failed to get VM private IP address: %w", err)
	}

	s.T.Cleanup(func() {
		defer cleanupBastionTunnel(vm.SSHClient)
		cleanupVMSS(ctx, s, vm)
	})

	result := "SSH Instructions: (may take a few minutes for the VM to be ready for SSH)\n========================\n"
	if config.Config.KeepVMSS {
		s.T.Logf("VM will be preserved after the test finishes, PLEASE MANUALLY DELETE THE VMSS. Set KEEP_VMSS=false to delete it automatically after the test finishes\n")
	} else {
		s.T.Logf("VM will be automatically deleted after the test finishes, to preserve it for debugging purposes set KEEP_VMSS=true or pause the test with a breakpoint before the test finishes or failed\n")
	}
	// We combine the az aks get credentials in the same line so we don't overwrite the user's kubeconfig.
	result += fmt.Sprintf(`az network bastion ssh --target-resource-id "%s" --name "%s-bastion" --resource-group %s --auth-type ssh-key --username azureuser --ssh-key %s`, *vm.VM.ID, *s.Runtime.Cluster.Model.Name, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, config.VMSSHPrivateKeyFileName) + "\n"
	s.T.Log(result)

	vmssResp, err := operation.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if !s.Config.SkipSSHConnectivityValidation {
		var bastErr error
		vm.SSHClient, bastErr = DialSSHOverBastion(ctx, s.Runtime.Cluster.Bastion, vm.PrivateIP, config.VMSSHPrivateKey)
		if bastErr != nil {
			return vm, fmt.Errorf("failed to start bastion tunnel: %w", bastErr)
		}
	}
	if err != nil {
		return vm, err
	}

	// Wait for VM to be in "Running" power state before proceeding
	err = waitForVMRunningState(ctx, s, vm.VM)
	if err != nil {
		return vm, fmt.Errorf("failed to wait for VM to reach running state: %w", err)
	}

	return &ScenarioVM{
		VMSS:      &vmssResp.VirtualMachineScaleSet,
		PrivateIP: vm.PrivateIP,
		VM:        vm.VM,
		SSHClient: vm.SSHClient,
	}, nil
}

// waitForVMRunningState polls until the VM reaches "Running" power state or the timeout elapses.
func waitForVMRunningState(ctx context.Context, s *Scenario, vmssVM *armcompute.VirtualMachineScaleSetVM) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	ticker := time.NewTicker(config.Config.DefaultPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		// Get the updated VM with instance view to check power state
		vm, err := config.Azure.VMSSVM.Get(ctxTimeout, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *vmssVM.InstanceID, &armcompute.VirtualMachineScaleSetVMsClientGetOptions{
			Expand: to.Ptr(armcompute.InstanceViewTypesInstanceView),
		})

		if err == nil {
			// Check if the VM has instance view and statuses
			if vm.Properties != nil && vm.Properties.InstanceView != nil && vm.Properties.InstanceView.Statuses != nil {
				for _, status := range vm.Properties.InstanceView.Statuses {
					if status.Code != nil && strings.HasPrefix(*status.Code, "PowerState/") {
						powerState := strings.TrimPrefix(*status.Code, "PowerState/")
						if powerState == "running" {
							toolkit.Logf(ctxTimeout, "VM reached running state")
							*vmssVM = vm.VirtualMachineScaleSetVM
							return nil
						}
						toolkit.Logf(ctxTimeout, "VM is in power state: %s, waiting for running state...", powerState)
					}
				}
			}
		}

		if err != nil {
			lastErr = err
		}

		select {
		case <-ctxTimeout.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for VM to reach running state: %w", lastErr)
			}
			return fmt.Errorf("timeout waiting for VM to reach running state")
		case <-ticker.C:
		}
	}
}

// waitForVMSSVM polls until a VMSS VM instance appears with network profile or the timeout elapses.
func waitForVMSSVM(ctx context.Context, s *Scenario) (*armcompute.VirtualMachineScaleSetVM, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ticker := time.NewTicker(config.Config.DefaultPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		pager := config.Azure.VMSSVM.NewListPager(*s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, &armcompute.VirtualMachineScaleSetVMsClientListOptions{
			Expand: to.Ptr("instanceView"),
		})

		if pager.More() {
			page, err := pager.NextPage(ctxTimeout)
			if err == nil && len(page.Value) > 0 {
				vmssVM := page.Value[0]
				// Verify it has network profile
				if vmssVM.Properties != nil && vmssVM.Properties.NetworkProfile != nil {
					return vmssVM, nil
				}
			}
			if err != nil {
				lastErr = err
			}
		}

		select {
		case <-ctxTimeout.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("timeout waiting for VMSS VM: %w", lastErr)
			}
			return nil, fmt.Errorf("timeout waiting for VMSS VM")
		case <-ticker.C:
		}
	}
}

// getPrivateIPFromVMSSVM extracts the private IP address from a VMSS VM by querying its network interfaces.
func getPrivateIPFromVMSSVM(ctx context.Context, resourceGroup, vmssName, instanceID string) (string, error) {
	// Query the network interface to get the IP configuration
	pager := config.Azure.NetworkInterfaces.NewListVirtualMachineScaleSetVMNetworkInterfacesPager(
		resourceGroup,
		vmssName,
		instanceID,
		nil,
	)

	if !pager.More() {
		return "", fmt.Errorf("no network interfaces found for VMSS VM")
	}

	page, err := pager.NextPage(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	if len(page.Value) == 0 {
		return "", fmt.Errorf("no network interfaces found")
	}

	nic := page.Value[0]
	if nic.Properties == nil || nic.Properties.IPConfigurations == nil || len(nic.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("network interface has no IP configurations")
	}

	ipConfig := nic.Properties.IPConfigurations[0]
	if ipConfig.Properties == nil || ipConfig.Properties.PrivateIPAddress == nil {
		return "", fmt.Errorf("IP configuration has no private IP address")
	}

	return *ipConfig.Properties.PrivateIPAddress, nil
}

func skipTestIfSKUNotAvailableErr(t testing.TB, err error) {
	if !config.Config.SkipTestsWithSKUCapacityIssue {
		return
	}
	var respErr *azcore.ResponseError
	if !errors.As(err, &respErr) || respErr.StatusCode != 409 {
		return
	}
	// sometimes the SKU is not available and we can't do anything. Skip the test in this case.
	if respErr.ErrorCode == "SkuNotAvailable" {
		t.Skip("skipping scenario SKU not available", t.Name(), err)
	}
	// sometimes the SKU quota is exceeded and we can't do anything. Skip the test in this case.
	if respErr.ErrorCode == "OperationNotAllowed" &&
		strings.Contains(respErr.Error(), "exceeding approved") &&
		strings.Contains(respErr.Error(), "quota") {
		t.Skip("skipping scenario SKU quota exceeded", t.Name(), err)
	}
}

func cleanupVMSS(ctx context.Context, s *Scenario, vm *ScenarioVM) {
	// original context can be cancelled, but we still want to collect the logs
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancel()
	defer deleteVMSS(ctx, s)
	extractLogsFromVM(ctx, s, vm)
}

func extractLogsFromVM(ctx context.Context, s *Scenario, vm *ScenarioVM) {
	if s.IsWindows() {
		extractLogsFromVMWindows(ctx, s)
	} else {
		err := extractLogsFromVMLinux(ctx, s, vm)
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

func extractLogsFromVMLinux(ctx context.Context, s *Scenario, vm *ScenarioVM) error {
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
		"waagent.log":                      "sudo cat /var/log/waagent.log",
		"aks-node-controller.log":          "sudo cat /var/log/azure/aks-node-controller.log",
		"aks-node-controller-config.json":  "sudo cat /opt/azure/containers/aks-node-controller-config.json", // Only available in Scriptless.

		// Only available in Scriptless. By default, e2e enables aks-node-controller-hack, so this is the actual config used. Only in e2e. Not used in production.
		"aks-node-controller-config-hack.json": "sudo cat /opt/azure/containers/aks-node-controller-config-hack.json",
		"syslog":                               "sudo cat /var/log/" + syslogHandle,
		"journalctl":                           "sudo journalctl --boot=0 --no-pager",
		"azure.json":                           "sudo cat /etc/kubernetes/azure.json",
	}
	if s.SecureTLSBootstrappingEnabled() {
		commandList["secure-tls-bootstrap.log"] = "sudo cat /var/log/azure/aks/secure-tls-bootstrap.log"
	}

	isAzureCNI, err := s.Runtime.Cluster.IsAzureCNI()
	if err == nil && isAzureCNI {
		commandList["azure-vnet.log"] = "sudo cat /var/log/azure-vnet.log"
		commandList["azure-vnet-ipam.log"] = "sudo cat /var/log/azure-vnet-ipam.log"
	}

	var logFiles = map[string]string{}
	for file, sourceCmd := range commandList {
		execResult, err := execScriptOnVm(ctx, s, vm, sourceCmd)
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

	s.T.Logf("##vso[task.logissue type=warning;]Storage account %s (%s) in Azure portal: %s", config.Config.BlobStorageAccount(), blobPrefix, azurePortalURL)

	runCommandTimeout := int32((20 * time.Minute).Seconds())

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
		if err := writeToFile(s.T, "sshkey", string(config.VMSSHPrivateKey)); err != nil {
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

func generateVMSSNameLinux(t testing.TB) string {
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

func getBaseVMSSModel(s *Scenario, customData, cseCmd string) armcompute.VirtualMachineScaleSet {
	model := armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(s.Location),
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
									KeyData: to.Ptr(string(config.VMSSHPublicKey)),
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
							Option:    to.Ptr(armcompute.DiffDiskOptionsLocal),
							Placement: to.Ptr(armcompute.DiffDiskPlacementResourceDisk),
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
						TypeHandlerVersion:      to.Ptr("2.1"),
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
