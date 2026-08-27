package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"k8s.io/apimachinery/pkg/util/wait"
)

func runScenarioFlow(ctx context.Context, name string, logger toolkit.Logger, s *Scenario) error {
	if config.Config.TestPreProvision || s.VHDCaching {
		return runScenarioWithPreProvision(ctx, name, logger, s)
	}
	if config.Config.DisableScriptless || scriptlessUnsupported(s) {
		return runScenario(ctx, name, logger, s)
	}

	if s.Runtime == nil {
		s.Runtime = &ScenarioRuntime{}
	}
	s.Runtime.EnableScriptlessNBCCSECmd = true
	return runScenario(ctx, name, logger, s)
}

func scriptlessUnsupported(s *Scenario) bool {
	return s.IsWindows() || len(s.Config.CustomDataWriteFiles) > 0 || s.VHDCaching || config.Config.TestPreProvision || s.VHD.Distro == datamodel.AKSAzureLinuxV2Gen2
}

func runScenarioWithPreProvision(ctx context.Context, name string, logger toolkit.Logger, original *Scenario) error {
	// This is hard to understand. Some functional magic is used to run the original scenario in two stages.
	// 1. Stage 1: Run the original scenario with pre-provisioning enabled, but skip the main validation and validate only pre-provisioning.
	// 2. Create a new Image from the VMSS created in Stage 1
	// 3. Stage 2: Run the original scenario again, but this time using the custom VHD created in a previous step, with validators,
	// The goal here is to test pre-provisioning logic on the variety of existing scenarios
	firstStage := freshScenario(original)
	var customVHD *config.Image

	// Mutate the copy for pre-provisioning
	firstStage.Config.SkipDefaultValidation = true
	firstStage.Config.Validator = func(ctx context.Context, stage1 *Scenario) error {
		var validationErr error
		if stage1.IsWindows() {
			validationErr = errors.Join(
				ValidateFileExists(ctx, stage1, "C:\\AzureData\\base_prep.complete"),
				ValidateFileDoesNotExist(ctx, stage1, "C:\\AzureData\\provision.complete"),
				ValidateWindowsServiceIsNotRunning(ctx, stage1, "kubelet"),
				ValidateWindowsServiceIsRunning(ctx, stage1, "containerd"),
			)
		} else {
			validationErr = errors.Join(
				ValidateFileExists(ctx, stage1, "/etc/containerd/config.toml"),
				ValidateFileExists(ctx, stage1, "/opt/azure/containers/base_prep.complete"),
				ValidateFileDoesNotExist(ctx, stage1, "/opt/azure/containers/provision.complete"),
				ValidateSystemdUnitIsRunning(ctx, stage1, "containerd"),
				ValidateSystemdUnitIsNotRunning(ctx, stage1, "kubelet"),
			)
		}
		if validationErr != nil {
			return validationErr
		}
		toolkit.Log(ctx, "=== Creating VHD Image ===")
		var err error
		customVHD, err = CreateImage(ctx, stage1)
		if err != nil {
			return err
		}
		customVHDJSON, _ := json.MarshalIndent(customVHD, "", "  ")
		toolkit.Logf(ctx, "Created custom VHD image: %s", string(customVHDJSON))
		cleanupBastionTunnel(firstStage.Runtime.VM.SSHClient)
		firstStage.Runtime.VM.SSHClient = nil
		return nil
	}

	firstStage.Config.VMConfigMutator = func(vmss *armcompute.VirtualMachineScaleSet) {
		if original.VMConfigMutator != nil {
			original.VMConfigMutator(vmss)
		}
		if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
			vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
		}
	}
	if original.BootstrapConfigMutator != nil || original.BootstrapConfigMutatorWithError != nil || original.PreProvisionBootstrapConfigMutator != nil {
		firstStage.BootstrapConfigMutator = nil
		firstStage.BootstrapConfigMutatorWithError = func(ctx context.Context, cluster *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
			if original.BootstrapConfigMutator != nil {
				original.BootstrapConfigMutator(cluster, nbc)
			}
			if original.BootstrapConfigMutatorWithError != nil {
				if err := original.BootstrapConfigMutatorWithError(ctx, cluster, nbc); err != nil {
					return err
				}
			}
			nbc.PreProvisionOnly = true
			nbc.EnableScriptlessNBCCSECmd = false
			// Bake-stage-only mutation: lets a scenario deliberately diverge bake-time
			// state from provision-time state (e.g. a stale sentinel bootstrap token).
			if original.PreProvisionBootstrapConfigMutator != nil {
				original.PreProvisionBootstrapConfigMutator(cluster, nbc)
			}
			return nil
		}
	}
	if original.AKSNodeConfigMutator != nil {
		firstStage.AKSNodeConfigMutator = func(cluster *Cluster, nodeconfig *aksnodeconfigv1.Configuration) {
			original.AKSNodeConfigMutator(cluster, nodeconfig)
			nodeconfig.PreProvisionOnly = true
		}
	}

	err := runScenario(ctx, name, logger, firstStage)
	original.checks = append(original.checks, firstStage.checks...)
	if err != nil {
		return err
	}

	secondStageScenario := freshScenario(original)
	secondStageScenario.Description = "Stage 2: Create VMSS from captured VHD via SIG"
	secondStageScenario.Config.VHD = customVHD
	secondStageScenario.Config.Validator = func(ctx context.Context, s *Scenario) error {
		var markerErr error
		if s.IsWindows() {
			markerErr = ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
		} else {
			markerErr = ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
		}
		if markerErr != nil {
			return markerErr
		}
		if original.Config.Validator != nil {
			return original.Config.Validator(ctx, s)
		}
		return nil
	}
	err = runScenario(ctx, name+"/VMProvision", logger, secondStageScenario)
	original.checks = append(original.checks, secondStageScenario.checks...)
	return err
}

// Keep attempt-owned cleanup across VHD-caching stages.
func freshScenario(s *Scenario) *Scenario {
	copied := *s
	copied.Runtime = nil
	copied.Logger = nil
	copied.testName = ""
	copied.failed = false
	copied.checks = nil
	return &copied
}

func runScenarioCleanup(ctx context.Context, cleanup *scenarioCleanup) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scenarioCleanupTimeout)
	defer cancel()
	if err := cleanup.runCleanups(cleanupCtx); err != nil {
		return fmt.Errorf("scenario cleanup failed: %w", err)
	}
	return nil
}

func addFailure(runErr, failure error) error {
	if failure == nil {
		return runErr
	}
	if runErr == nil {
		return failure
	}
	// Preserve the original result text, but unwrap only the added failure so
	// a skip followed by a cleanup or logging failure is classified as a failure.
	return fmt.Errorf("%v; %w", runErr, failure)
}

func markScenarioOutcome(s *Scenario, runErr error, recovered any) {
	if recovered != nil {
		s.failed = true
		panic(recovered)
	}
	var skip *skipError
	s.failed = runErr != nil && !errors.As(runErr, &skip)
}

func runScenario(ctx context.Context, name string, logger toolkit.Logger, s *Scenario) (runErr error) {
	s.testName = name
	s.Logger = logger
	if s.Location == "" {
		s.Location = config.Config.DefaultLocation
	}

	s.Location = strings.ToLower(s.Location)

	if s.K8sSystemPoolSKU == "" {
		s.K8sSystemPoolSKU = config.Config.DefaultVMSKU
	}

	ctx = toolkit.ContextWithLogger(ctx, s.Logger)
	defer func() {
		markScenarioOutcome(s, runErr, recover())
	}()
	if err := maybeSkipScenario(ctx, name, s); err != nil {
		return err
	}

	if _, err := CachedEnsureResourceGroup(ctx, s.Location); err != nil {
		return fmt.Errorf("ensure resource group: %w", err)
	}
	if _, err := CachedCreateVMManagedIdentity(ctx, s.Location); err != nil {
		return fmt.Errorf("create VM managed identity: %w", err)
	}
	defer toolkit.LogStep(s.Logger, "running scenario")()

	cluster, err := s.Config.Cluster(ctx, ClusterRequest{
		Location:         s.Location,
		K8sSystemPoolSKU: s.K8sSystemPoolSKU,
	})
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// in some edge cases cluster cache is broken and nil cluster is returned
	// need to find the root cause and fix it, this should help to catch such cases
	if cluster == nil || cluster.Model == nil || cluster.Model.Name == nil || cluster.Model.Location == nil || cluster.Model.Properties == nil {
		return fmt.Errorf("cluster cache returned an incomplete cluster")
	}

	// Log cluster identity for debugging
	clusterName := *cluster.Model.Name
	clusterLocation := *cluster.Model.Location
	resourceGroup := config.ResourceGroupName(clusterLocation)
	subscriptionID := config.Config.SubscriptionID
	s.Logger.Logf("using cluster %s in rg=%s sub=%s", clusterName, resourceGroup, subscriptionID)
	s.Logger.Logf("portal: https://portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s/overview",
		subscriptionID, resourceGroup, clusterName)

	if s.Runtime == nil {
		s.Runtime = &ScenarioRuntime{}
	}
	s.Runtime.Cluster = cluster
	s.Runtime.VMSize = config.Config.DefaultVMSKU
	s.Runtime.VMSSName = generateVMSSName(s)

	testKube, err := cluster.NewKubeclientForTest()
	if err != nil {
		return fmt.Errorf("creating per-test kubeclient: %w", err)
	}
	s.Runtime.Kube = testKube

	// use shorter timeout for faster feedback on test failures
	vmssCtx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutVMSS)
	defer cancel()
	s.Runtime.VM, err = prepareAKSNode(vmssCtx, s)
	if s.ExpectedError != "" {
		return assert.ErrorContains(err, s.ExpectedError)
	}
	if err != nil {
		return err
	}

	s.Logger.Logf("Choosing the private ACR %q for the vm validation", config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location))

	return validateVM(vmssCtx, s)
}

func prepareAKSNode(ctx context.Context, s *Scenario) (*ScenarioVM, error) {
	defer toolkit.LogStep(s.Logger, "preparing AKS node")()

	nbc, err := getBaseNBC(ctx, s.Runtime.Cluster, s.VHD)
	if err != nil {
		return nil, fmt.Errorf("get base node bootstrapping configuration: %w", err)
	}

	if !config.Config.DisableScriptless {
		nbc.EnableScriptlessCSECmd = true
	}
	if s.Runtime != nil && s.Runtime.EnableScriptlessNBCCSECmd {
		nbc.EnableScriptlessNBCCSECmd = true
		nbc.EnableScriptlessCSECmd = false
	}

	if s.IsWindows() {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/"
	}

	s.Runtime.NBC = nbc
	if s.BootstrapConfigMutator != nil {
		s.BootstrapConfigMutator(s.Runtime.Cluster, nbc)
	}
	if s.BootstrapConfigMutatorWithError != nil {
		if err := s.BootstrapConfigMutatorWithError(ctx, s.Runtime.Cluster, nbc); err != nil {
			return nil, fmt.Errorf("mutate bootstrap configuration: %w", err)
		}
	}
	if s.AKSNodeConfigMutator != nil {
		nodeconfig, err := nbcToAKSNodeConfigV1(nbc)
		if err != nil {
			return nil, fmt.Errorf("convert NBC to AKS node config: %w", err)
		}
		s.AKSNodeConfigMutator(s.Runtime.Cluster, nodeconfig)
		s.Runtime.AKSNodeConfig = nodeconfig

		aksNodeConfigJSON, err := nodeconfigutils.MarshalConfigurationV1(nodeconfig)
		if err != nil {
			return nil, fmt.Errorf("marshal AKS node config: %w", err)
		}
		s.Runtime.NBC.AKSNodeConfigJSON = string(aksNodeConfigJSON)

		nbc.EnableScriptlessCSECmd = false

		// for scriptless phase 2.5, we are using nbc cse cmd for provisioning but passing aksnodeconfig and nbc cse cmd to compare env variables
		// scriptless tag means provisioning with aksnodeconfig is used
		if !config.Config.DisableScriptless && !s.Tags.Scriptless &&
			(s.BootstrapConfigMutator != nil || s.BootstrapConfigMutatorWithError != nil) {
			nbc.EnableScriptlessNBCCSECmd = true
		}
	}

	publicKeyData := datamodel.PublicKey{KeyData: string(config.VMSSHPublicKey)}

	// check it all.
	if s.Runtime.NBC != nil && s.Runtime.NBC.ContainerService != nil && s.Runtime.NBC.ContainerService.Properties != nil && s.Runtime.NBC.ContainerService.Properties.LinuxProfile != nil {
		if s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys == nil {
			s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{}
		}
		// Windows fetches SSH keys from the linux profile and replaces any existing SSH keys with these. So we have to set
		// the Linux SSH keys for Windows SSH to work. Yeah. I find it odd too.
		s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = append(s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys, publicKeyData)
	}

	gen2Only, err := CachedIsVMSizeGen2Only(ctx, VMSizeSKURequest{
		Location: s.Location,
		VMSize:   s.Runtime.VMSize,
	})
	if err != nil {
		return nil, fmt.Errorf("checking if VM size %q supports only Gen2: %w", s.Runtime.VMSize, err)
	}
	if gen2Only && s.Config.VHD.UnsupportedGen2 {
		s.Logger.Logf("VM size %q only supports Gen2 hypervisor but image does not, falling back to vm size that supports Gen1 %q", s.Runtime.VMSize, config.DefaultV5VMSKU)
		s.Runtime.VMSize = config.DefaultV5VMSKU
		nbc.AgentPoolProfile.VMSize = config.DefaultV5VMSKU
	}
	supportsNVMe, err := CachedVMSizeSupportsNVMe(ctx, VMSizeSKURequest{
		Location: s.Location,
		VMSize:   s.Runtime.VMSize,
	})
	if err != nil {
		return nil, fmt.Errorf("checking if VM size %q supports only NVMe: %w", s.Runtime.VMSize, err)
	}
	if supportsNVMe {
		if s.Config.VHD.UnsupportedNVMe {
			s.Logger.Logf("VM size %q supports NVMe disk controller but image does not support NVMe, falling back to vm size that supports SCSI %q", s.Runtime.VMSize, config.DefaultV5VMSKU)
			s.Runtime.VMSize = config.DefaultV5VMSKU
			nbc.AgentPoolProfile.VMSize = config.DefaultV5VMSKU
		} else {
			s.Config.UseNVMe = true
		}
	}

	start := time.Now() // Record the start time
	scenarioVM, err := ConfigureAndCreateVMSS(ctx, s)
	// Expected failures are checked by the runner; cleanup still collects debug information.
	if s.ExpectedError != "" {
		return scenarioVM, err
	}
	if err != nil {
		return scenarioVM, fmt.Errorf("create vmss %q, check %s for vm logs: %w", s.Runtime.VMSSName, testDir(s.testName), err)
	}
	if scenarioVM == nil || scenarioVM.VM == nil {
		return nil, fmt.Errorf("create vmss %q returned an incomplete VM", s.Runtime.VMSSName)
	}

	if err := getCustomScriptExtensionStatus(s, scenarioVM.VM); err != nil {
		return scenarioVM, err
	}

	if !s.Config.SkipDefaultValidation {
		vmssCreatedAt := time.Now()         // Record the start time
		creationElapse := time.Since(start) // Calculate the elapsed time
		scenarioVM.KubeName, err = s.Runtime.Kube.WaitUntilNodeReady(ctx, s.Logger, s.Runtime.VMSSName)
		if err != nil {
			return scenarioVM, err
		}
		readyElapse := time.Since(vmssCreatedAt) // Calculate the elapsed time
		totalElapse := time.Since(start)
		toolkit.LogDuration(ctx, totalElapse, 3*time.Minute, fmt.Sprintf("Node %s took %s to be created and %s to be ready", s.Runtime.VMSSName, creationElapse, readyElapse))
	}

	return scenarioVM, nil
}

func maybeSkipScenario(ctx context.Context, name string, s *Scenario) error {
	s.Tags = scenarioTags(s)

	_, err := CachedPrepareVHD(ctx, GetVHDRequest{
		Image:    *s.VHD,
		Location: s.Location,
	})
	if err != nil {
		if config.Config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			return &skipError{message: fmt.Sprintf("scenario %q image for VHD %s was not found: %s", name, s.VHD.Distro, err)}
		}
		return fmt.Errorf("failing scenario %q: could not find image for VHD %s: %w", name, s.VHD.Distro, err)
	}
	s.Logger.Logf("TAGS %+v", s.Tags)
	return nil
}

func ValidateNodeCanRunAPod(ctx context.Context, s *Scenario) error {
	var errs []error
	numberRetries := 3
	if s.IsWindows() {
		serverCorePods := components.GetServercoreImagesForVHD(s.VHD)
		for i, pod := range serverCorePods {
			errs = append(errs, ValidatePodRunningWithRetry(ctx, s, debugPodWindows(s, fmt.Sprintf("servercore%d", i), pod), numberRetries))
		}

		nanoServerPods := components.GetNanoserverImagesForVhd(s.VHD)
		for i, pod := range nanoServerPods {
			errs = append(errs, ValidatePodRunningWithRetry(ctx, s, debugPodWindows(s, fmt.Sprintf("nanoserver%d", i), pod), numberRetries))
		}
	} else {
		errs = append(errs, ValidatePodRunningWithRetry(ctx, s, podHTTPServerLinux(s), numberRetries))
	}
	return errors.Join(errs...)
}

func validateVM(ctx context.Context, s *Scenario) error {
	defer toolkit.LogStep(s.Logger, "validating VM")()
	if !s.Config.SkipSSHConnectivityValidation {
		if err := validateSSHConnectivity(ctx, s); err != nil {
			return err
		}
	}

	// Extract CSE timing events immediately after SSH is available, before other
	// validators run. The Guest Agent periodically sweeps the events directory,
	// so we must read events before the delay from pod scheduling and validation.
	// Only runs for CSE perf test scenarios (gated by EagerCSETimingExtraction).
	if s.EagerCSETimingExtraction {
		report, err := ExtractCSETimings(ctx, s)
		if err == nil && len(report.Tasks) > 0 {
			s.Runtime.CSETimingReport = report
		}
	}

	var errs []error
	if !s.Config.SkipDefaultValidation {
		errs = append(errs, ValidateNodeCanRunAPod(ctx, s))
		switch s.VHD.OS {
		case config.OSWindows:
			errs = append(errs, ValidateCommonWindows(ctx, s))
		default:
			errs = append(errs, ValidateCommonLinux(ctx, s))
		}
	}

	// test-specific validation
	if s.Config.Validator != nil {
		errs = append(errs, s.Config.Validator(ctx, s))
	}
	err := errors.Join(errs...)
	if err != nil {
		s.Logger.Log("VM validation failed")
	} else {
		s.Logger.Log("VM validation succeeded")
	}
	return err
}

func getCustomScriptExtensionStatus(s *Scenario, vmssVM *armcompute.VirtualMachineScaleSetVM) error {
	if vmssVM == nil || vmssVM.Properties == nil {
		return fmt.Errorf("VMSS VM is missing properties")
	}
	// Re-fetch the VM with instance view to ensure we have fresh extension status data.
	// The VM object passed in may have been fetched before the CSE finished executing,
	// so the extension status message could be empty or stale.
	if vmssVM.InstanceID != nil {
		// Bounded fresh context (matches other diagnostic/cleanup paths in this file):
		// this re-fetch collects post-mortem CSE status, so it should complete even if
		// the caller's ctx was cancelled (e.g., test timeout), but must not hang the
		// suite indefinitely on a stalled ARM call.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		freshVM, err := config.Azure.VMSSVM.Get(ctx,
			*s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
			s.Runtime.VMSSName,
			*vmssVM.InstanceID,
			&armcompute.VirtualMachineScaleSetVMsClientGetOptions{
				Expand: to.Ptr(armcompute.InstanceViewTypesInstanceView),
			})
		if err == nil && freshVM.Properties != nil && freshVM.Properties.InstanceView != nil {
			vmssVM.Properties.InstanceView = freshVM.Properties.InstanceView
		} else if err != nil {
			s.Logger.Logf("warning: failed to re-fetch VM instance view for CSE status: %v", err)
		}
	}

	if vmssVM.Properties.InstanceView == nil {
		return fmt.Errorf("VMSS VM is missing instance view")
	}
	for _, extension := range vmssVM.Properties.InstanceView.Extensions {
		// Only process the CSE extension, skip other extensions (e.g., ManagedIdentity)
		// whose empty status messages would overwrite the actual CSE output file.
		// The extension name in InstanceView is typically "vmssCSE" (matching the resource name)
		// but may also appear as the handler type. Match on known CSE identifiers.
		if extension.Name == nil {
			continue
		}
		name := strings.ToLower(*extension.Name)
		isCSE := name == "vmsscse" ||
			strings.Contains(name, "customscript") ||
			strings.Contains(name, "aksnode")
		if !isCSE {
			continue
		}
		for _, status := range extension.Statuses {
			if status == nil {
				continue
			}
			if s.IsWindows() {
				// Save the CSE output for Windows VMs for better troubleshooting.
				// Only write when the message has actual content to avoid overwriting
				// with an empty file from a status entry that has no output.
				if status.Message != nil && *status.Message != "" {
					logDir := testDir(s.testName)
					if err := os.MkdirAll(logDir, 0755); err == nil {
						logFile := filepath.Join(logDir, "windows-cse-output.log")
						err = os.WriteFile(logFile, []byte(*status.Message), 0644)
						if err != nil {
							s.Logger.Logf("failed to save Windows CSE output to %s: %v", logFile, err)
						} else {
							s.Logger.Logf("saved Windows CSE output to %s (%d bytes)", logFile, len(*status.Message))
						}
					}
				}

				if status.Code == nil || !strings.EqualFold(*status.Code, "ProvisioningState/succeeded") {
					return fmt.Errorf("failed to get CSE output, error: %s", *status.Message)
				}
				return nil

			} else {
				resp, err := parseLinuxCSEMessage(*status)
				if err != nil {
					return fmt.Errorf("parse CSE message with error, error %w", err)
				}
				if resp.ExitCode != "0" {
					return fmt.Errorf("vmssCSE %s, output=%s, error=%s, cse output: %s", resp.ExitCode, resp.Output, resp.Error, *status.Message)
				}
				return nil
			}
		}
	}
	extensionsJSON, _ := json.MarshalIndent(vmssVM.Properties.InstanceView.Extensions, "", "  ")
	return fmt.Errorf("failed to get CSE output, VM extensions: %s", string(extensionsJSON))
}

func parseLinuxCSEMessage(status armcompute.InstanceViewStatus) (*datamodel.CSEStatus, error) {
	if status.Code == nil || status.Message == nil {
		return nil, datamodel.NewError(datamodel.InvalidCSEMessage, "No valid Status code or Message provided from cse extension")
	}

	start := strings.Index(*status.Message, "[stdout]") + len("[stdout]")
	end := strings.Index(*status.Message, "[stderr]")

	var linuxExtensionExitCodeStrRegex = regexp.MustCompile(linuxExtensionExitCodeStr)
	var linuxExtensionErrorCodeRegex = regexp.MustCompile(extensionErrorCodeRegex)
	extensionFailed := linuxExtensionErrorCodeRegex.MatchString(*status.Code)
	if end <= start {
		return nil, fmt.Errorf("Parse CSE failed with error cannot find [stdout] and [stderr], raw CSE Message: %s, delete vm: %t", *status.Message, extensionFailed)
	}
	rawInstanceViewInfo := (*status.Message)[start:end]
	// Parse CSE message
	var cseStatus datamodel.CSEStatus
	err := json.Unmarshal([]byte(rawInstanceViewInfo), &cseStatus)
	if err != nil {
		exitCodeMatch := linuxExtensionExitCodeStrRegex.FindStringSubmatch(*status.Message)
		if len(exitCodeMatch) > 1 && extensionFailed {
			// Failed but the format is not expected.
			cseStatus.ExitCode = exitCodeMatch[1]
			cseStatus.Error = *status.Message
			return &cseStatus, nil
		}
		return nil, fmt.Errorf("Parse CSE Json failed with error: %s, raw CSE Message: %s, delete vm: %t", err, *status.Message, extensionFailed)
	}
	if cseStatus.ExitCode == "" {
		return nil, fmt.Errorf("CSE Json does not contain exit code, raw CSE Message: %s", *status.Message)
	}
	return &cseStatus, nil
}

func addVMExtensionToVMSS(properties *armcompute.VirtualMachineScaleSetProperties, extension *armcompute.VirtualMachineScaleSetExtension) *armcompute.VirtualMachineScaleSetProperties {
	if properties == nil {
		properties = &armcompute.VirtualMachineScaleSetProperties{}
	}

	if properties.VirtualMachineProfile == nil {
		properties.VirtualMachineProfile = &armcompute.VirtualMachineScaleSetVMProfile{}
	}

	if properties.VirtualMachineProfile.ExtensionProfile == nil {
		properties.VirtualMachineProfile.ExtensionProfile = &armcompute.VirtualMachineScaleSetExtensionProfile{}
	}

	if properties.VirtualMachineProfile.ExtensionProfile.Extensions == nil {
		properties.VirtualMachineProfile.ExtensionProfile.Extensions = []*armcompute.VirtualMachineScaleSetExtension{}
	}

	// NOTE: This is not checking if we are adding a duplicate extension.
	properties.VirtualMachineProfile.ExtensionProfile.Extensions = append(properties.VirtualMachineProfile.ExtensionProfile.Extensions, extension)
	return properties
}

func addTrustedLaunchToVMSS(properties *armcompute.VirtualMachineScaleSetProperties) *armcompute.VirtualMachineScaleSetProperties {
	if properties == nil {
		properties = &armcompute.VirtualMachineScaleSetProperties{}
	}

	if properties.VirtualMachineProfile == nil {
		properties.VirtualMachineProfile = &armcompute.VirtualMachineScaleSetVMProfile{}
	}

	if properties.VirtualMachineProfile.SecurityProfile == nil {
		properties.VirtualMachineProfile.SecurityProfile = &armcompute.SecurityProfile{}
	}

	properties.VirtualMachineProfile.SecurityProfile.SecurityType = to.Ptr(armcompute.SecurityTypesTrustedLaunch)
	if properties.VirtualMachineProfile.SecurityProfile.UefiSettings == nil {
		properties.VirtualMachineProfile.SecurityProfile.UefiSettings = &armcompute.UefiSettings{}
	}
	properties.VirtualMachineProfile.SecurityProfile.UefiSettings.SecureBootEnabled = to.Ptr(true)
	properties.VirtualMachineProfile.SecurityProfile.UefiSettings.VTpmEnabled = to.Ptr(true)

	return properties
}

func createVMExtensionLinuxAKSNode(ctx context.Context, location *string) (*armcompute.VirtualMachineScaleSetExtension, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	region := config.Config.DefaultLocation
	if location != nil {
		region = *location
	}

	const fallbackExtensionVersion = "1.413"
	extensionName := "Compute.AKS.Linux.AKSNode"
	publisher := "Microsoft.AKS"
	extensionVersion, err := CachedGetLatestVMExtensionImageVersion(ctx, GetLatestExtensionVersionRequest{
		Location:  region,
		ExtType:   extensionName,
		Publisher: publisher,
	})
	if err != nil {
		toolkit.Logf(ctx, "warning: failed to get latest VM extension version, falling back to %s: %v", fallbackExtensionVersion, err)
		extensionVersion = fallbackExtensionVersion
	}
	toolkit.Logf(ctx, "Using VM extension version %s for extension type %s in region %s", extensionVersion, extensionName, region)

	return &armcompute.VirtualMachineScaleSetExtension{
		Name: to.Ptr(extensionName),
		Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
			Publisher:          to.Ptr(publisher),
			Type:               to.Ptr(extensionName),
			TypeHandlerVersion: to.Ptr(extensionVersion),
		},
	}, nil
}

// RunCommand executes a script on the VMSS VM with the configured instance ID via the
// Azure VMSS RunCommand v2 API (VirtualMachineRunCommand resource). This is the API
// already used by production aks-rp PIS code; using it here keeps test and production
// on the same surface and avoids the v1 RunCommand extension's failure modes
// (e.g. the Microsoft.CPlat.Core/RunCommandWindows "Keyset does not exist" error
// fixed by ADO PR https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp/pullrequest/15721814).
//
// Unlike SSH-based exec, this works even when WinRM/SSH are unavailable (e.g. mid-sysprep).
// It is generally slower than SSH because each call creates a VirtualMachineRunCommand
// resource on the VM and waits for it to provision.
func RunCommand(ctx context.Context, s *Scenario, command string) (armcompute.VirtualMachineRunCommandInstanceView, error) {
	if s.Runtime == nil || s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil ||
		s.Runtime.VM == nil || s.Runtime.VM.VM == nil || s.Runtime.VM.VM.InstanceID == nil {
		return armcompute.VirtualMachineRunCommandInstanceView{}, fmt.Errorf("scenario runtime is incomplete for RunCommand")
	}
	rg := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	instanceID := *s.Runtime.VM.VM.InstanceID
	// VirtualMachineRunCommand resources persist on the VM until explicitly deleted;
	// use a unique name per call so concurrent / repeated calls don't collide, and
	// best-effort delete it on the way out below.
	runCommandName := fmt.Sprintf("e2e-runcmd-%d", time.Now().UnixNano())
	start := time.Now()
	defer func() {
		toolkit.Logf(ctx, "RunCommand %s took %s", runCommandName, time.Since(start))
	}()

	runCmd := armcompute.VirtualMachineRunCommand{
		Location: to.Ptr(s.Location),
		Properties: &armcompute.VirtualMachineRunCommandProperties{
			Source: &armcompute.VirtualMachineRunCommandScriptSource{
				Script: to.Ptr(command),
			},
			AsyncExecution: to.Ptr(false),
		},
	}

	poller, err := config.Azure.VMSSVMRunCommands.BeginCreateOrUpdate(ctx, rg, s.Runtime.VMSSName, instanceID, runCommandName, runCmd, nil)
	if err != nil {
		return armcompute.VirtualMachineRunCommandInstanceView{}, fmt.Errorf("failed to start RunCommand on VMSS VM: %w", err)
	}
	defer func() {
		// Best-effort cleanup: VirtualMachineRunCommand resources persist on the VM
		// otherwise and can accumulate across many RunCommand calls in a single run.
		// Use a fresh context so we still clean up if the caller's ctx is cancelled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, derr := config.Azure.VMSSVMRunCommands.BeginDelete(cleanupCtx, rg, s.Runtime.VMSSName, instanceID, runCommandName, nil); derr != nil {
			toolkit.Logf(ctx, "best-effort RunCommand %s delete failed: %v", runCommandName, derr)
		}
	}()
	if _, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions()); err != nil {
		return armcompute.VirtualMachineRunCommandInstanceView{}, fmt.Errorf("failed to wait for RunCommand on VMSS VM: %w", err)
	}

	// CreateOrUpdate (PUT) never includes InstanceView in the response — it's only
	// returned by Get when $expand=instanceView is set. Fetch it explicitly so we
	// get stdout/stderr/exit code.
	getResp, err := config.Azure.VMSSVMRunCommands.Get(ctx, rg, s.Runtime.VMSSName, instanceID, runCommandName, &armcompute.VirtualMachineScaleSetVMRunCommandsClientGetOptions{
		Expand: to.Ptr("instanceView"),
	})
	if err != nil {
		return armcompute.VirtualMachineRunCommandInstanceView{}, fmt.Errorf("failed to get RunCommand instance view: %w", err)
	}
	if getResp.Properties == nil || getResp.Properties.InstanceView == nil {
		return armcompute.VirtualMachineRunCommandInstanceView{}, errors.New("RunCommand result missing instance view")
	}
	view := *getResp.Properties.InstanceView
	return view, runCommandScriptError(view)
}

// runCommandScriptError converts a RunCommand instance view into an error if the
// script itself failed. The ARM CreateOrUpdate operation reports success as long as
// the extension was able to run the script — a non-zero exit, throw, or timeout
// inside the script lives in ExecutionState / ExitCode and must be converted to an error. See:
// https://learn.microsoft.com/en-us/azure/virtual-machines/windows/run-command-managed
// ("InstanceView.ExecutionState: Status of user's Run Command script. ...
//
//	ProvisioningState: Status of general extension provisioning end to end").
func runCommandScriptError(view armcompute.VirtualMachineRunCommandInstanceView) error {
	state := armcompute.ExecutionStateUnknown
	if view.ExecutionState != nil {
		state = *view.ExecutionState
	}
	exitCode := int32(0)
	if view.ExitCode != nil {
		exitCode = *view.ExitCode
	}
	if state == armcompute.ExecutionStateSucceeded && exitCode == 0 {
		return nil
	}
	output := ""
	if view.Output != nil {
		output = strings.TrimSpace(*view.Output)
	}
	stderr := ""
	if view.Error != nil {
		stderr = strings.TrimSpace(*view.Error)
	}
	msg := ""
	if view.ExecutionMessage != nil {
		msg = strings.TrimSpace(*view.ExecutionMessage)
	}
	return fmt.Errorf("RunCommand script failed: executionState=%s exitCode=%d message=%q stdout=%q stderr=%q",
		state, exitCode, msg, output, stderr)
}

// windowsSysprepScript runs Sysprep /generalize on the test VM. It pre-emptively drops
// any SysPrepExternal\Generalize provider entry pointing at VMAgentDisabler.dll: when
// the DLL can't be loaded, Sysprep stalls past our vmssCtx deadline. The same workaround
// has lived in vhdbuilder/packer/windows/sysprep.ps1 since 2020 (PR #429).
//
// Pre-cleanup of C:\Windows\Panther and unattend.xml follows
// https://learn.microsoft.com/en-us/azure/virtual-machines/generalize, which notes
// that stale Panther logs can cause Sysprep to fail and that custom answer files
// aren't supported in this step. The ImageState poll handles Server 2022 where
// Sysprep /quit can return before background SetupHost.exe finishes generalizing.
const windowsSysprepScript = `
$ErrorActionPreference = 'Stop'

# Best-effort: drop broken SysPrepExternal\Generalize providers that point at
# VMAgentDisabler.dll. When the DLL can't be loaded Sysprep /generalize hangs
# ~14m. Registry cleanup failures are logged but must not abort sysprep itself.
try {
    $path = 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\SysPrepExternal\Generalize'
    if (Test-Path $path) {
        foreach ($name in (Get-Item -Path $path).Property) {
            $value = Get-ItemPropertyValue -Path $path -Name $name
            if ($value -like '*VMAgentDisabler.dll*') {
                Write-Host "Removing broken generalize provider $name -> $value"
                Remove-ItemProperty -Path $path -Name $name
            }
        }
    }
} catch {
    Write-Warning "Failed to clean SysPrepExternal\Generalize entries: $_"
}

# Per https://learn.microsoft.com/en-us/azure/virtual-machines/generalize:
# stale Panther logs can cause Sysprep to fail; custom unattend files unsupported.
Remove-Item "$env:SystemRoot\Panther" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:SystemRoot\System32\Sysprep\unattend.xml" -Force -ErrorAction SilentlyContinue

# /quit (not /shutdown) so RunCommand can return; deallocate happens separately.
# $LASTEXITCODE isn't reliable after Sysprep.exe /quit — sysprep launches a
# background SetupHost.exe and returns before generalization completes. The
# ImageState poll below is the authoritative success signal (same as
# vhdbuilder/packer/windows/sysprep.ps1).
& "$env:SystemRoot\System32\Sysprep\Sysprep.exe" /oobe /generalize /mode:vm /quiet /quit

# On Server 2022, sysprep /quit can return before background SetupHost.exe
# finishes generalizing. Wait for the registry state to confirm before letting
# the caller deallocate and capture the disk. Same pattern as
# vhdbuilder/packer/windows/sysprep.ps1 (in-tree since 2020).
$deadline = (Get-Date).AddMinutes(10)
$last = $null
while ($true) {
    $state = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\State').ImageState
    if ($state -ne $last) {
        Write-Host "ImageState=$state"
        $last = $state
    }
    if ($state -eq 'IMAGE_STATE_GENERALIZE_RESEAL_TO_OOBE') { break }
    if ((Get-Date) -gt $deadline) {
        throw "Sysprep did not reach IMAGE_STATE_GENERALIZE_RESEAL_TO_OOBE within 10 minutes (last state: $state)"
    }
    Start-Sleep -Seconds 10
}
`

func CreateImage(ctx context.Context, s *Scenario) (*config.Image, error) {
	if s.Runtime == nil || s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil ||
		s.Runtime.VM == nil || s.Runtime.VM.VM == nil || s.Runtime.VM.VM.InstanceID == nil {
		return nil, fmt.Errorf("scenario runtime is incomplete for image creation")
	}
	if s.IsWindows() {
		s.Logger.Log("Running sysprep on Windows VM...")
		res, err := RunCommand(ctx, s, windowsSysprepScript)
		var stdout, stderr string
		if res.Output != nil {
			stdout = strings.TrimSpace(*res.Output)
		}
		if res.Error != nil {
			stderr = strings.TrimSpace(*res.Error)
		}
		s.Logger.Logf("Sysprep stdout: %s", stdout)
		if stderr != "" {
			s.Logger.Logf("Sysprep stderr: %s", stderr)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to run sysprep on Windows VM for image creation: %w", err)
		}
	}

	vm, err := config.Azure.VMSSVM.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, &armcompute.VirtualMachineScaleSetVMsClientGetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get VMSS VM for image creation: %w", err)
	}
	if vm.Properties == nil || vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.OSDisk == nil ||
		vm.Properties.StorageProfile.OSDisk.ManagedDisk == nil || vm.Properties.StorageProfile.OSDisk.ManagedDisk.ID == nil {
		return nil, fmt.Errorf("VMSS VM is missing its managed OS disk ID")
	}

	s.Logger.Log("Deallocating VMSS VM...")
	poll, err := config.Azure.VMSSVM.BeginDeallocate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to begin deallocate: %w", err)
	}
	_, err = poll.PollUntilDone(ctx, config.PollUntilDoneOptions())
	if err != nil {
		return nil, fmt.Errorf("Failed to deallocate: %w", err)
	}

	// Create version using smaller integers that fit within Azure's limits
	// Use Unix timestamp for guaranteed uniqueness in concurrent runs
	// Take last 9 digits to ensure it fits in 32-bit integer range
	now := time.Now().UTC()
	patchVersion := now.UnixNano() % 1000000000
	version := fmt.Sprintf("1.%s.%d", now.Format("20060102"), patchVersion)

	return CreateSIGImageVersionFromDisk(
		ctx,
		s,
		version,
		*vm.Properties.StorageProfile.OSDisk.ManagedDisk.ID,
	)
}

// CreateSIGImageVersionFromDisk creates a new SIG image version directly from a VM disk
func CreateSIGImageVersionFromDisk(ctx context.Context, s *Scenario, version string, diskResourceID string) (*config.Image, error) {
	startTime := time.Now()
	defer func() {
		s.Logger.Logf("Created SIG image version %s from disk %s in %s", version, diskResourceID, time.Since(startTime))
	}()
	if s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VM == nil ||
		s.Runtime.VM.VM.Properties == nil || s.Runtime.VM.VM.Properties.InstanceView == nil {
		return nil, fmt.Errorf("scenario runtime is missing VM instance metadata for image creation")
	}
	if s.Config.VHD == nil {
		return nil, fmt.Errorf("scenario VHD is nil")
	}
	rg := config.ResourceGroupName(s.Location)
	gallery, err := CachedCreateGallery(ctx, CreateGalleryRequest{
		ResourceGroup: rg,
		Location:      s.Location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or get gallery: %w", err)
	}
	if gallery.Name == nil {
		return nil, fmt.Errorf("failed to create or get gallery: no gallery name returned")
	}

	image, err := CachedCreateGalleryImage(ctx, CreateGalleryImageRequest{
		ResourceGroup:    rg,
		GalleryName:      *gallery.Name,
		Location:         s.Location,
		Arch:             s.VHD.Arch,
		Windows:          s.IsWindows(),
		HyperVGeneration: s.Runtime.VM.VM.Properties.InstanceView.HyperVGeneration,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or get gallery image: %w", err)
	}
	if image.ID == nil || image.Name == nil {
		return nil, fmt.Errorf("failed to create or get gallery image: incomplete image metadata returned")
	}

	s.Logger.Logf("Created gallery image: %s", *image.ID)

	// Create the image version directly from the disk
	s.Logger.Logf("Creating gallery image version: %s in %s", version, *image.ID)
	createVersionOp, err := config.Azure.GalleryImageVersions.BeginCreateOrUpdate(ctx, rg, *gallery.Name, *image.Name, version, armcompute.GalleryImageVersion{
		Location: to.Ptr(s.Location),
		Properties: &armcompute.GalleryImageVersionProperties{
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				OSDiskImage: &armcompute.GalleryOSDiskImage{
					Source: &armcompute.GalleryDiskImageSource{
						ID: to.Ptr(diskResourceID),
					},
				},
			},
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				ReplicationMode: to.Ptr(armcompute.ReplicationModeShallow),
				TargetRegions: []*armcompute.TargetRegion{
					{
						Name:                 to.Ptr(s.Location),
						RegionalReplicaCount: to.Ptr[int32](1),
						StorageAccountType:   to.Ptr(armcompute.StorageAccountTypePremiumLRS),
					},
				},
				ReplicaCount: to.Ptr[int32](1),
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create gallery image version: %w", err)
	}

	_, err = createVersionOp.PollUntilDone(ctx, config.PollUntilDoneOptions())
	if err != nil {
		return nil, fmt.Errorf("Failed to complete gallery image version creation: %w", err)
	}

	s.Cleanup(func(ctx context.Context) error {
		config.Azure.DeleteSIGImageVersion(ctx, rg, *gallery.Name, *image.Name, version)
		return nil
	})
	customVHD := *s.Config.VHD
	customVHD.Name = *image.Name // Use the architecture-specific image name
	customVHD.Gallery = &config.Gallery{
		SubscriptionID:    config.Config.SubscriptionID,
		ResourceGroupName: rg,
		Name:              *gallery.Name,
	}
	customVHD.Version = version

	return &customVHD, nil
}

// isRebootRelatedSSHError checks if the error is related to a system reboot
func isRebootRelatedSSHError(err error, stderr string) bool {
	if err == nil {
		return false
	}

	rebootIndicators := []string{
		"System is going down",
		"pam_nologin",
		"Connection closed by",
		"Connection refused",
		"Connection timed out",
	}

	errMsg := err.Error()
	for _, indicator := range rebootIndicators {
		if strings.Contains(errMsg, indicator) || strings.Contains(stderr, indicator) {
			return true
		}
	}
	return false
}

func validateSSHConnectivity(ctx context.Context, s *Scenario) error {
	// If WaitForSSHAfterReboot is not set, use the original single-attempt behavior
	if s.Config.WaitForSSHAfterReboot == 0 {
		return attemptSSHConnection(ctx, s)
	}

	// Retry logic with exponential backoff for scenarios that may reboot
	s.Logger.Logf("SSH connectivity validation will retry for up to %s if reboot-related errors are encountered", s.Config.WaitForSSHAfterReboot)
	startTime := time.Now()
	var lastSSHError error

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, s.Config.WaitForSSHAfterReboot, true, func(ctx context.Context) (bool, error) {
		err := attemptSSHConnection(ctx, s)
		if err == nil {
			elapsed := time.Since(startTime)
			s.Logger.Logf("SSH connectivity established after %s", elapsed)
			return true, nil
		}

		// Save the last error for better error messages
		lastSSHError = err

		// Extract stderr from the error
		stderr := ""
		if strings.Contains(err.Error(), "Stderr:") {
			parts := strings.Split(err.Error(), "Stderr:")
			if len(parts) > 1 {
				stderr = parts[1]
			}
		}

		// Check if this is a reboot-related error
		if isRebootRelatedSSHError(err, stderr) {
			s.Logger.Logf("Detected reboot-related SSH error, will retry: %v", err)
			return false, nil // Continue polling
		}

		// Not a reboot error, fail immediately
		return false, err
	})

	// If we timed out while retrying reboot-related errors, provide a better error message
	if err != nil && lastSSHError != nil {
		elapsed := time.Since(startTime)
		return fmt.Errorf("SSH connection failed after waiting %s for node to reboot and come back up. Last SSH error: %w", elapsed, lastSSHError)
	}

	return err
}

// attemptSSHConnection performs a single SSH connectivity check
func attemptSSHConnection(ctx context.Context, s *Scenario) error {
	var connectionResult *podExecResult
	var err error
	connectionResult, err = runSSHCommand(ctx, s.Runtime.VM.SSHClient, "echo 'SSH_CONNECTION_OK'", s.IsWindows())

	if err != nil {
		return fmt.Errorf("SSH connection to %s failed: %s", s.Runtime.VM.PrivateIP, err)
	}

	if !strings.Contains(connectionResult.stdout, "SSH_CONNECTION_OK") {
		return fmt.Errorf("SSH_CONNECTION_OK not found on %s: %s", s.Runtime.VM.PrivateIP, connectionResult.String())
	}

	s.Logger.Logf("SSH connectivity to %s verified successfully", s.Runtime.VM.PrivateIP)
	return nil
}

func runScenarioUbuntu2404GPUNPD(name, vmSize, location, k8sSystemPoolSKU string) *Scenario {
	return &Scenario{
		Name:             name,
		Description:      fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2404 VHD can be properly bootstrapped and NPD tests are valid", vmSize),
		Location:         location,
		K8sSystemPoolSKU: k8sSystemPoolSKU,
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
				vmss.SKU.Name = to.Ptr(vmSize)

				extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
				if err != nil {
					return fmt.Errorf("creating AKS VM extension: %w", err)
				}

				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
				return nil
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				// First, ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				if err := errors.Join(
					ValidateNvidiaModProbeInstalled(ctx, s),
					ValidateKubeletHasNotStopped(ctx, s),
					ValidateServicesDoNotRestartKubelet(ctx, s),
				); err != nil {
					return err
				}

				// Then validate NPD configuration and GPU monitoring
				if err := ValidateNPDGPUCountPlugin(ctx, s); err != nil {
					return err
				}
				if err := ValidateNPDGPUCountCondition(ctx, s); err != nil {
					return err
				}
				if err := ValidateNPDGPUCountAfterFailure(ctx, s); err != nil {
					return err
				}

				// Validate the if IB NPD is reporting the flapping condition
				if err := ValidateNPDIBLinkFlappingCondition(ctx, s); err != nil {
					return err
				}
				return ValidateNPDIBLinkFlappingAfterFailure(ctx, s)
			},
		}}
}

func vmSKUGeneration(sku string) (int, error) {
	// Extract the generation number from the SKU string (e.g., "Standard_D2s_v3" -> 3)
	sku = strings.ToLower(sku)
	idx := strings.LastIndex(sku, "_v")
	if idx < 0 {
		return 0, fmt.Errorf("invalid SKU format: %s", sku)
	}
	gen, err := strconv.Atoi(sku[idx+2:])
	if err != nil {
		return 0, fmt.Errorf("SKU %q has non-numeric generation suffix: %w", sku, err)
	}
	return gen, nil
}

func ensureMinVMGeneration(minSku string) string {
	// Ensure that the VM SKU used is at least the minimum generation required for the test
	// Get the minimum generation for the specified SKU
	defaultGen, err := vmSKUGeneration(config.Config.DefaultVMSKU)
	if err != nil {
		panic(fmt.Sprintf("Warning: No minimum generation found for SKU %s", config.Config.DefaultVMSKU))
	}
	minGen, err := vmSKUGeneration(minSku)
	if err != nil {
		panic(fmt.Sprintf("Warning: No minimum generation found for SKU %s", minSku))
	}
	if defaultGen < minGen {
		return minSku
	} else {
		return config.Config.DefaultVMSKU
	}
}
