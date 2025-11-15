package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	logf                        = toolkit.Logf
	log                         = toolkit.Log
	SSHKeyPrivate, SSHKeyPublic = mustGetNewRSAKeyPair()
)

// it's important to share context between tests to allow graceful shutdown
// cancellation signal can be sent before a test starts, without shared context such test will miss the signal
var testCtx = setupSignalHandler()

// setupSignalHandler handles OS signals to gracefully shutdown the test suite
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	red := func(text string) string {
		return "\033[31m" + text + "\033[0m"
	}

	go func() {
		// block until signal is received
		<-ch
		fmt.Println(red("Received cancellation signal, gracefully shutting down the test suite. Deleting Azure Resources. Cancel again to force exit. (Created Azure resources will not be deleted in this case)"))
		cancel()

		// block until second signal is received
		<-ch
		// This DefaultLocation is used on purpose.
		msg := constructErrorMessage(config.Config.SubscriptionID, config.Config.DefaultLocation)
		fmt.Println(red(msg))
		os.Exit(1)
	}()
	return ctx
}

func constructErrorMessage(subscriptionID, location string) string {
	return fmt.Sprintf("Received second cancellation signal, forcing exit.\nPlease check https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/overview and delete any resources created by the test suite", subscriptionID, config.ResourceGroupName(location))
}

func newTestCtx(t testing.TB) context.Context {
	if testCtx.Err() != nil {
		t.Skip("test suite is shutting down")
	}
	ctx, cancel := context.WithTimeout(testCtx, config.Config.TestTimeout)
	t.Cleanup(cancel)
	// T should be used only for logging, not for assertions or any other logic
	ctx = toolkit.ContextWithT(ctx, t)
	return ctx
}

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	// Special case for testing VHD caching. Not used by default.
	if config.Config.TestPreProvision || s.VHDCaching {
		t.Run("VHDCreation", func(t *testing.T) {
			t.Parallel()
			runScenarioWithPreProvision(t, s)
		})
	} else {
		// Default path
		runScenario(t, s)
	}

}

func runScenarioWithPreProvision(t *testing.T, original *Scenario) {
	// This is hard to understand. Some functional magic is used to run the original scenario in two stages.
	// 1. Stage 1: Run the original scenario with pre-provisioning enabled, but skip the main validation and validate only pre-provisioning.
	// 2. Create a new Image from the VMSS created in Stage 1
	// 3. Stage 2: Run the original scenario again, but this time using the custom VHD created in a previous step, with validators,
	// The goal here is to test pre-provisioning logic on the variety of existing scenarios
	firstStage := copyScenario(original)
	var customVHD *config.Image

	// Mutate the copy for pre-provisioning
	firstStage.Config.SkipDefaultValidation = true
	firstStage.Config.Validator = func(ctx context.Context, stage1 *Scenario) {
		if stage1.IsWindows() {
			ValidateFileExists(ctx, stage1, "C:\\AzureData\\base_prep.complete")
			ValidateFileDoesNotExist(ctx, stage1, "C:\\AzureData\\provision.complete")
			ValidateWindowsServiceIsNotRunning(ctx, stage1, "kubelet")
			ValidateWindowsServiceIsRunning(ctx, stage1, "containerd")
		} else {
			ValidateFileExists(ctx, stage1, "/etc/containerd/config.toml")
			ValidateFileExists(ctx, stage1, "/opt/azure/containers/base_prep.complete")
			ValidateFileDoesNotExist(ctx, stage1, "/opt/azure/containers/provision.complete")
			ValidateSystemdUnitIsRunning(ctx, stage1, "containerd")
			ValidateSystemdUnitIsNotRunning(ctx, stage1, "kubelet")
		}
		t.Log("=== Creating VHD Image ===")
		customVHD = CreateImage(ctx, stage1)
		customVHDJSON, _ := json.MarshalIndent(customVHD, "", "  ")
		t.Logf("Created custom VHD image: %s", string(customVHDJSON))
	}
	firstStage.Config.VMConfigMutator = func(vmss *armcompute.VirtualMachineScaleSet) {
		if original.VMConfigMutator != nil {
			original.VMConfigMutator(vmss)
		}
		if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
			vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
		}
	}
	if original.BootstrapConfigMutator != nil {
		firstStage.BootstrapConfigMutator = func(nbc *datamodel.NodeBootstrappingConfiguration) {
			original.BootstrapConfigMutator(nbc)
			nbc.PreProvisionOnly = true
		}
	}
	if original.AKSNodeConfigMutator != nil {
		firstStage.AKSNodeConfigMutator = func(nodeconfig *aksnodeconfigv1.Configuration) {
			original.AKSNodeConfigMutator(nodeconfig)
			nodeconfig.PreProvisionOnly = true
		}
	}

	runScenario(t, firstStage)

	if t.Failed() {
		return
	}

	// Create a new subtest to avoid conflicts with previous steps (log output folder is based on the test name)
	t.Run("VMProvision", func(t *testing.T) {
		t.Parallel()
		secondStageScenario := copyScenario(original)
		secondStageScenario.Description = "Stage 2: Create VMSS from captured VHD via SIG"
		secondStageScenario.Config.VHD = customVHD
		secondStageScenario.Config.Validator = func(ctx context.Context, s *Scenario) {
			// This validators are used when running all scenarios in "VHD Caching" mode, which is usually done manually
			if s.IsWindows() {
				ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
			} else {
				ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
			}
			if original.Config.Validator != nil {
				original.Config.Validator(ctx, s)
			}
		}
		runScenario(t, secondStageScenario)
	})
}

// Helper to deep copy a Scenario (implement as needed for your struct)
func copyScenario(s *Scenario) *Scenario {
	// Implement deep copy logic for Scenario and its fields
	// This is a placeholder; you may need to copy nested structs and slices
	copied := *s
	copied.Config = s.Config // If Config is a struct, deep copy its fields as well
	return &copied
}

func runScenario(t testing.TB, s *Scenario) {
	t = toolkit.WithTestLogger(t)
	if s.Location == "" {
		s.Location = config.Config.DefaultLocation
	}

	s.Location = strings.ToLower(s.Location)

	if s.K8sSystemPoolSKU == "" {
		s.K8sSystemPoolSKU = config.Config.DefaultVMSKU
	}

	ctx := newTestCtx(t)
	_, err := CachedEnsureResourceGroup(ctx, s.Location)
	require.NoError(t, err)
	_, err = CachedCreateVMManagedIdentity(ctx, s.Location)
	require.NoError(t, err)
	s.T = t
	ctrruntimelog.SetLogger(zap.New())

	maybeSkipScenario(ctx, t, s)

	cluster, err := s.Config.Cluster(ctx, ClusterRequest{
		Location:         s.Location,
		K8sSystemPoolSKU: s.K8sSystemPoolSKU,
	})

	require.NoError(s.T, err, "failed to get cluster")
	// in some edge cases cluster cache is broken and nil cluster is returned
	// need to find the root cause and fix it, this should help to catch such cases
	require.NotNil(t, cluster)
	s.Runtime = &ScenarioRuntime{
		Cluster:  cluster,
		VMSSName: generateVMSSName(s),
	}

	// use shorter timeout for faster feedback on test failures
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutVMSS)
	defer cancel()
	s.Runtime.VM = prepareAKSNode(ctx, s)

	t.Logf("Choosing the private ACR %q for the vm validation", config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location))
	validateVM(ctx, s)
}

func prepareAKSNode(ctx context.Context, s *Scenario) *ScenarioVM {
	if (s.BootstrapConfigMutator == nil) == (s.AKSNodeConfigMutator == nil) {
		s.T.Fatalf("exactly one of BootstrapConfigMutator or AKSNodeConfigMutator must be set")
	}

	var err error
	nbc, err := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)
	require.NoError(s.T, err)

	if s.IsWindows() {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/"
	}

	if s.BootstrapConfigMutator != nil {
		s.BootstrapConfigMutator(nbc)
		s.Runtime.NBC = nbc
	}
	if s.AKSNodeConfigMutator != nil {
		nodeconfig := nbcToAKSNodeConfigV1(nbc)
		s.AKSNodeConfigMutator(nodeconfig)
		s.Runtime.AKSNodeConfig = nodeconfig
	}
	publicKeyData := datamodel.PublicKey{KeyData: string(SSHKeyPublic)}

	// check it all.
	if s.Runtime.NBC != nil && s.Runtime.NBC.ContainerService != nil && s.Runtime.NBC.ContainerService.Properties != nil && s.Runtime.NBC.ContainerService.Properties.LinuxProfile != nil {
		if s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys == nil {
			s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{}
		}
		// Windows fetches SSH keys from the linux profile and replaces any existing SSH keys with these. So we have to set
		// the Linux SSH keys for Windows SSH to work. Yeah. I find it odd too.
		s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = append(s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys, publicKeyData)
	}

	require.NoError(s.T, err)

	start := time.Now() // Record the start time
	scenarioVM := ConfigureAndCreateVMSS(ctx, s)

	err = getCustomScriptExtensionStatus(s, scenarioVM.VM)
	require.NoError(s.T, err)

	if !s.Config.SkipDefaultValidation {
		vmssCreatedAt := time.Now()         // Record the start time
		creationElapse := time.Since(start) // Calculate the elapsed time
		scenarioVM.KubeName = s.Runtime.Cluster.Kube.WaitUntilNodeReady(ctx, s.T, s.Runtime.VMSSName)
		readyElapse := time.Since(vmssCreatedAt) // Calculate the elapsed time
		totalElapse := time.Since(start)
		toolkit.LogDuration(ctx, totalElapse, 3*time.Minute, fmt.Sprintf("Node %s took %s to be created and %s to be ready", s.Runtime.VMSSName, toolkit.FormatDuration(creationElapse), toolkit.FormatDuration(readyElapse)))
	}

	return scenarioVM
}

func maybeSkipScenario(ctx context.Context, t testing.TB, s *Scenario) {
	s.Tags.Name = t.Name()
	s.Tags.OS = string(s.VHD.OS)
	s.Tags.Arch = s.VHD.Arch
	s.Tags.ImageName = s.VHD.Name
	s.Tags.VHDCaching = s.VHDCaching
	if s.AKSNodeConfigMutator != nil {
		s.Tags.Scriptless = true
	}

	if config.Config.TagsToRun != "" {
		matches, err := s.Tags.MatchesFilters(config.Config.TagsToRun)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if !matches {
			t.Skipf("skipping scenario %q: scenario tags %+v does not match filter %q", t.Name(), s.Tags, config.Config.TagsToRun)
		}
	}

	if config.Config.TagsToSkip != "" {
		matches, err := s.Tags.MatchesAnyFilter(config.Config.TagsToSkip)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if matches {
			t.Skipf("skipping scenario %q: scenario tags %+v matches filter %q", t.Name(), s.Tags, config.Config.TagsToSkip)
		}
	}

	_, err := CachedPrepareVHD(ctx, GetVHDRequest{
		Image:    *s.VHD,
		Location: s.Location,
	})
	if err != nil {
		if config.Config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			t.Skipf("skipping scenario %q: could not find image for VHD %s due to %s", t.Name(), s.VHD.Distro, err)
		} else {
			t.Fatalf("failing scenario %q: could not find image for VHD %s due to %s", t.Name(), s.VHD.Distro, err)
		}
	}
	t.Logf("TAGS %+v", s.Tags)
}

func ValidateNodeCanRunAPod(ctx context.Context, s *Scenario) {
	if s.IsWindows() {
		serverCorePods := components.GetServercoreImagesForVHD(s.VHD)
		for i, pod := range serverCorePods {
			ValidatePodRunning(ctx, s, podWindows(s, fmt.Sprintf("servercore%d", i), pod))
		}

		nanoServerPods := components.GetNanoserverImagesForVhd(s.VHD)
		for i, pod := range nanoServerPods {
			ValidatePodRunning(ctx, s, podWindows(s, fmt.Sprintf("nanoserver%d", i), pod))
		}
	} else {
		ValidatePodRunning(ctx, s, podHTTPServerLinux(s))
	}
}

func validateVM(ctx context.Context, s *Scenario) {
	if !s.Config.SkipSSHConnectivityValidation {
		err := validateSSHConnectivity(ctx, s)
		require.NoError(s.T, err)
	}

	if !s.Config.SkipDefaultValidation {
		ValidateNodeCanRunAPod(ctx, s)
		switch s.VHD.OS {
		case config.OSWindows:
			ValidateCommonWindows(ctx, s)
		default:
			ValidateCommonLinux(ctx, s)
		}
	}

	// test-specific validation
	if s.Config.Validator != nil {
		s.Config.Validator(ctx, s)
	}
	if s.T.Failed() {
		s.T.Log("VM validation failed")
	} else {
		s.T.Log("VM validation succeeded")
	}
}

func getCustomScriptExtensionStatus(s *Scenario, vmssVM *armcompute.VirtualMachineScaleSetVM) error {
	for _, extension := range vmssVM.Properties.InstanceView.Extensions {
		for _, status := range extension.Statuses {
			if s.IsWindows() {
				// Save the CSE output for Windows VMs for better troubleshooting
				if status.Message != nil {
					logDir := filepath.Join("scenario-logs", s.T.Name())
					if err := os.MkdirAll(logDir, 0755); err == nil {
						logFile := filepath.Join(logDir, "windows-cse-output.log")
						err = os.WriteFile(logFile, []byte(*status.Message), 0644)
						if err != nil {
							s.T.Logf("failed to save Windows CSE output to %s: %v", logFile, err)
						} else {
							s.T.Logf("saved Windows CSE output to %s", logFile)
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

func createVMExtensionLinuxAKSNode(location *string) (*armcompute.VirtualMachineScaleSetExtension, error) {
	// Default to "westus" if location is nil.
	region := "westus"
	if location != nil {
		region = *location
	}

	extensionName := "Compute.AKS.Linux.AKSNode"
	publisher := "Microsoft.AKS"

	// NOTE (@surajssd): If this is gonna be called multiple times, then find a way to cache the latest version.
	extensionVersion, err := config.Azure.GetLatestVMExtensionImageVersion(context.TODO(), region, extensionName, publisher)
	if err != nil {
		return nil, fmt.Errorf("getting latest VM extension image version: %v", err)
	}

	return &armcompute.VirtualMachineScaleSetExtension{
		Name: to.Ptr(extensionName),
		Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
			Publisher:          to.Ptr(publisher),
			Type:               to.Ptr(extensionName),
			TypeHandlerVersion: to.Ptr(extensionVersion),
		},
	}, nil
}

// RunCommand executes a command on the VMSS VM with instance ID "0" and returns the raw JSON response from Azure
// Unlike default approach, it doesn't use SSH and uses Azure tooling
// This approach is generally slower, but it works even if SSH is not available
func RunCommand(ctx context.Context, s *Scenario, command string) (armcompute.RunCommandResult, error) {
	s.T.Helper()
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		logf(ctx, "Command %q took %s", command, toolkit.FormatDuration(elapsed))
	}()

	runPoller, err := config.Azure.VMSSVM.BeginRunCommand(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, armcompute.RunCommandInput{
		CommandID: func() *string {
			if s.IsWindows() {
				return to.Ptr("RunPowerShellScript")
			}
			return to.Ptr("RunShellScript")
		}(),
		Script: []*string{to.Ptr(command)},
	}, nil)
	if err != nil {
		return armcompute.RunCommandResult{}, fmt.Errorf("failed to run command on Windows VM for image creation: %w", err)
	}

	runResp, err := runPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return runResp.RunCommandResult, fmt.Errorf("failed to run command on Windows VM for image creation: %w", err)
	}
	return runResp.RunCommandResult, err
}

func CreateImage(ctx context.Context, s *Scenario) *config.Image {
	if s.IsWindows() {
		s.T.Log("Running sysprep on Windows VM...")
		res, err := RunCommand(ctx, s, `C:\Windows\System32\Sysprep\Sysprep.exe /oobe /generalize /mode:vm /quiet /quit;`)
		resJson, _ := json.MarshalIndent(res, "", "  ")
		s.T.Logf("Sysprep result: %s", string(resJson))
		require.NoErrorf(s.T, err, "failed to run sysprep on Windows VM for image creation")
	}

	vm, err := config.Azure.VMSSVM.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, &armcompute.VirtualMachineScaleSetVMsClientGetOptions{})
	require.NoError(s.T, err, "Failed to get VMSS VM for image creation")

	s.T.Log("Deallocating VMSS VM...")
	poll, err := config.Azure.VMSSVM.BeginDeallocate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, nil)
	require.NoError(s.T, err, "Failed to begin deallocate")
	_, err = poll.PollUntilDone(ctx, nil)
	require.NoError(s.T, err, "Failed to deallocate")

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
func CreateSIGImageVersionFromDisk(ctx context.Context, s *Scenario, version string, diskResourceID string) *config.Image {
	startTime := time.Now()
	defer func() {
		s.T.Logf("Created SIG image version %s from disk %s in %s", version, diskResourceID, toolkit.FormatDuration(time.Since(startTime)))
	}()
	rg := config.ResourceGroupName(s.Location)
	gallery, err := CachedCreateGallery(ctx, CreateGalleryRequest{
		ResourceGroup: rg,
		Location:      s.Location,
	})
	require.NoError(s.T, err, "failed to create or get gallery")

	image, err := CachedCreateGalleryImage(ctx, CreateGalleryImageRequest{
		ResourceGroup:    rg,
		GalleryName:      *gallery.Name,
		Location:         s.Location,
		Arch:             s.VHD.Arch,
		Windows:          s.IsWindows(),
		HyperVGeneration: s.Runtime.VM.VM.Properties.InstanceView.HyperVGeneration,
	})
	require.NoError(s.T, err, "failed to create or get gallery image")

	s.T.Logf("Created gallery image: %s", *image.ID)

	// Create the image version directly from the disk
	s.T.Logf("Creating gallery image version: %s in %s", version, *image.ID)
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
	require.NoError(s.T, err, "Failed to create gallery image version")

	_, err = createVersionOp.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	require.NoError(s.T, err, "Failed to complete gallery image version creation")

	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		config.Azure.DeleteSIGImageVersion(ctx, rg, *gallery.Name, *image.Name, version)
	})
	customVHD := *s.Config.VHD
	customVHD.Name = *image.Name // Use the architecture-specific image name
	customVHD.Gallery = &config.Gallery{
		SubscriptionID:    config.Config.SubscriptionID,
		ResourceGroupName: rg,
		Name:              *gallery.Name,
	}
	customVHD.Version = version

	return &customVHD
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
	s.T.Logf("SSH connectivity validation will retry for up to %s if reboot-related errors are encountered", s.Config.WaitForSSHAfterReboot)
	startTime := time.Now()
	var lastSSHError error

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, s.Config.WaitForSSHAfterReboot, true, func(ctx context.Context) (bool, error) {
		err := attemptSSHConnection(ctx, s)
		if err == nil {
			elapsed := time.Since(startTime)
			s.T.Logf("SSH connectivity established after %s", toolkit.FormatDuration(elapsed))
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
			s.T.Logf("Detected reboot-related SSH error, will retry: %v", err)
			return false, nil // Continue polling
		}

		// Not a reboot error, fail immediately
		return false, err
	})

	// If we timed out while retrying reboot-related errors, provide a better error message
	if err != nil && lastSSHError != nil {
		elapsed := time.Since(startTime)
		return fmt.Errorf("SSH connection failed after waiting %s for node to reboot and come back up. Last SSH error: %w", toolkit.FormatDuration(elapsed), lastSSHError)
	}

	return err
}

// attemptSSHConnection performs a single SSH connectivity check
func attemptSSHConnection(ctx context.Context, s *Scenario) error {
	connectionTest := fmt.Sprintf("%s echo 'SSH_CONNECTION_OK'", sshString(s.Runtime.VM.PrivateIP))
	connectionResult, err := execOnPrivilegedPod(ctx, s.Runtime.Cluster.Kube, defaultNamespace, s.Runtime.Cluster.DebugPod.Name, connectionTest)

	if err != nil || !strings.Contains(connectionResult.stdout.String(), "SSH_CONNECTION_OK") {
		output := ""
		if connectionResult != nil {
			output = connectionResult.String()
		}

		return fmt.Errorf("SSH connection to %s failed: %s: %s", s.Runtime.VM.PrivateIP, err, output)
	}

	s.T.Logf("SSH connectivity to %s verified successfully", s.Runtime.VM.PrivateIP)
	return nil
}

func runScenarioGPUNPD(t *testing.T, vmSize, location, k8sSystemPoolSKU string) *Scenario {
	t.Helper()
	return &Scenario{
		Description:      fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2404 VHD can be properly bootstrapped and NPD tests are valid", vmSize),
		Location:         location,
		K8sSystemPoolSKU: k8sSystemPoolSKU,
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)

				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")

				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// First, ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)

				// Then validate NPD configuration and GPU monitoring
				ValidateNPDGPUCountPlugin(ctx, s)
				ValidateNPDGPUCountCondition(ctx, s)
				ValidateNPDGPUCountAfterFailure(ctx, s)

				// Validate the if IB NPD is reporting the flapping condition
				ValidateNPDIBLinkFlappingCondition(ctx, s)
				ValidateNPDIBLinkFlappingAfterFailure(ctx, s)
			},
		}}
}
