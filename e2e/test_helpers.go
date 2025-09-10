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
	ctrruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	logf = toolkit.Logf
	log  = toolkit.Log
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

func newTestCtx(t *testing.T) context.Context {
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
	if config.Config.TestPreProvision {
		t.Run("Original", func(t *testing.T) {
			t.Parallel()
			runScenario(t, s)
		})
		t.Run("FirstStage", func(t *testing.T) {
			t.Parallel()
			runScenarioWithPreProvision(t, s)
		})
	} else {
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
		t.Log("=== Stage 1 validation complete, proceeding to Stage 2 ===")
		customVHD = CreateImage(ctx, stage1)
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

	t.Run("SecondStage", func(t *testing.T) {
		t.Parallel()
		secondStageScenario := copyScenario(original)
		secondStageScenario.Description = "Stage 2: Create VMSS from captured VHD via SIG"
		secondStageScenario.Config.VHD = customVHD
		secondStageScenario.Config.Validator = func(ctx context.Context, s *Scenario) {
			if s.IsWindows() {
				ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
				ValidateWindowsServiceIsRunning(ctx, s, "kubelet")
			} else {
				ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
				ValidateSystemdUnitIsRunning(ctx, s, "kubelet")
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

func runScenario(t *testing.T, s *Scenario) {
	if s.Location == "" {
		s.Location = config.Config.DefaultLocation
	}

	s.Location = strings.ToLower(s.Location)
	ctx := newTestCtx(t)
	_, err := CachedEnsureResourceGroup(ctx, s.Location)
	require.NoError(t, err)
	_, err = CachedCreateVMManagedIdentity(ctx, s.Location)
	require.NoError(t, err)
	s.T = t
	ctrruntimelog.SetLogger(zap.New())

	maybeSkipScenario(ctx, t, s)

	cluster, err := s.Config.Cluster(ctx, s.Location)
	require.NoError(s.T, err, "failed to get cluster")
	// in some edge cases cluster cache is broken and nil cluster is returned
	// need to find the root cause and fix it, this should help to catch such cases
	require.NotNil(t, cluster)
	s.Runtime = &ScenarioRuntime{
		Cluster: cluster,
	}
	// use shorter timeout for faster feedback on test failures
	ctx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutVMSS)
	defer cancel()
	prepareAKSNode(ctx, s)

	t.Logf("Choosing the private ACR %q for the vm validation", config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location))
	validateVM(ctx, s)
}

func prepareAKSNode(ctx context.Context, s *Scenario) {
	s.Runtime.VMSSName = generateVMSSName(s)
	if (s.BootstrapConfigMutator == nil) == (s.AKSNodeConfigMutator == nil) {
		s.T.Fatalf("exactly one of BootstrapConfigMutator or AKSNodeConfigMutator must be set")
	}

	nbc := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)

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
	var err error
	s.Runtime.SSHKeyPrivate, s.Runtime.SSHKeyPublic, err = getNewRSAKeyPair()
	publicKeyData := datamodel.PublicKey{KeyData: string(s.Runtime.SSHKeyPublic)}

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
	createVMSS(ctx, s)

	err = getCustomScriptExtensionStatus(ctx, s)
	require.NoError(s.T, err)

	s.T.Logf("vmss %s creation succeeded", s.Runtime.VMSSName)

	if !s.Config.SkipDefaultValidation {
		vmssCreatedAt := time.Now()         // Record the start time
		creationElapse := time.Since(start) // Calculate the elapsed time
		s.Runtime.KubeNodeName = s.Runtime.Cluster.Kube.WaitUntilNodeReady(ctx, s.T, s.Runtime.VMSSName)
		readyElapse := time.Since(vmssCreatedAt) // Calculate the elapsed time
		totalElapse := time.Since(start)
		s.T.Logf("node %s is ready", s.Runtime.VMSSName)
		toolkit.LogDuration(ctx, totalElapse, 3*time.Minute, fmt.Sprintf("Node %s took %s to be created and %s to be ready", s.Runtime.VMSSName, toolkit.FormatDuration(creationElapse), toolkit.FormatDuration(readyElapse)))
	}

	s.Runtime.VMPrivateIP, err = getVMPrivateIPAddress(ctx, s)
	require.NoError(s.T, err, "failed to get VM private IP address")
}

func maybeSkipScenario(ctx context.Context, t *testing.T, s *Scenario) {
	s.Tags.Name = t.Name()
	s.Tags.OS = string(s.VHD.OS)
	s.Tags.Arch = s.VHD.Arch
	s.Tags.ImageName = s.VHD.Name
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

	vhd, err := CachedPrepareVHD(ctx, GetVHDRequest{
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
	t.Logf("VHD: %q, TAGS %+v", vhd, s.Tags)
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
	err := uploadSSHKey(ctx, s)
	require.NoError(s.T, err)

	if !s.Config.SkipDefaultValidation {
		ValidateNodeCanRunAPod(ctx, s)
		switch s.VHD.OS {
		case config.OSWindows:
			// TODO: validate something
		default:
			ValidateCommonLinux(ctx, s)
		}
	}

	// test-specific validation
	if s.Config.Validator != nil {
		s.Config.Validator(ctx, s)
	}
	s.T.Log("validation succeeded")
}

func getCustomScriptExtensionStatus(ctx context.Context, s *Scenario) error {
	pager := config.Azure.VMSSVM.NewListPager(*s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get VMSS instances: %v", err)
		}

		for _, vmInstance := range page.Value {
			instanceViewResp, err := config.Azure.VMSSVM.GetInstanceView(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *vmInstance.InstanceID, nil)
			if err != nil {
				return fmt.Errorf("failed to get instance view for VM %s: %v", *vmInstance.InstanceID, err)
			}
			for _, extension := range instanceViewResp.Extensions {
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
							return fmt.Errorf("Parse CSE message with error, error %w", err)
						}
						if resp.ExitCode != "0" {
							return fmt.Errorf("vmssCSE %s, output=%s, error=%s, cse output: %s", resp.ExitCode, resp.Output, resp.Error, *status.Message)
						}
						return nil
					}
				}
			}
		}
	}
	return fmt.Errorf("failed to get CSE output.")
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

func CreateImage(ctx context.Context, s *Scenario) *config.Image {
	s.T.Log("Generalizing VM")
	if s.IsLinux() {
		execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo waagent -deprovision", 0, "Failed to deprovision the VM for image creation")
	} else {
		runPoller, err := config.Azure.VMSSVM.BeginRunCommand(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, "0", armcompute.RunCommandInput{
			CommandID: to.Ptr("RunPowerShellScript"),
			Script: []*string{to.Ptr(`if(Test-Path C:\system32\Sysprep\unattend.xml) {
Remove-Item C:\system32\Sysprep\unattend.xml -Force
};
C:\Windows\System32\Sysprep\Sysprep.exe /oobe /generalize /mode:vm /quiet /quit;`)},
		}, nil)
		require.NoError(s.T, err, "Failed to run command on Windows VM for image creation")

		runResp, err := runPoller.PollUntilDone(ctx, nil)
		require.NoError(s.T, err, "Failed to run command on Windows VM for image creation")
		respJson, _ := runResp.MarshalJSON()
		s.T.Logf("Run command output: %s", string(respJson))
	}

	vm, err := config.Azure.VMSSVM.Get(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, "0", &armcompute.VirtualMachineScaleSetVMsClientGetOptions{})
	require.NoError(s.T, err, "Failed to get VMSS VM for image creation")

	s.T.Log("Deallocating VMSS VM...")
	poll, err := config.Azure.VMSSVM.BeginDeallocate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, "0", nil)
	require.NoError(s.T, err, "Failed to begin deallocate")
	_, err = poll.PollUntilDone(ctx, nil)
	require.NoError(s.T, err, "Failed to deallocate")

	// Create version using smaller integers that fit within Azure's limits
	// Use Unix timestamp for guaranteed uniqueness in concurrent runs
	now := time.Now()
	nanos := now.UnixNano()
	// Take last 9 digits to ensure it fits in 32-bit integer range
	patchVersion := nanos % 1000000000
	version := fmt.Sprintf("1.%s.%d", now.Format("20060102"), patchVersion)

	// Get the OS disk resource ID directly from the VM
	diskName := *vm.Properties.StorageProfile.OSDisk.Name
	diskResourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s",
		config.Config.SubscriptionID, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, diskName)

	return CreateSIGImageVersionFromDisk(
		ctx,
		s,
		version,
		diskResourceID,
	)
}

// CreateSIGImageVersionFromDisk creates a new SIG image version directly from a VM disk
func CreateSIGImageVersionFromDisk(ctx context.Context, s *Scenario, version string, diskResourceID string) *config.Image {
	rg := config.ResourceGroupName(s.Location)
	gallery, err := CachedCreateGallery(ctx, CreateGalleryRequest{
		ResourceGroup: rg,
		Location:      s.Location,
	})
	require.NoError(s.T, err, "failed to create or get gallery")

	image, err := CachedCreateGalleryImage(ctx, CreateGalleryImageRequest{
		ResourceGroup: rg,
		GalleryName:   *gallery.Name,
		Location:      s.Location,
		Arch:          s.VHD.Arch,
		Windows:       s.IsWindows(),
	})
	require.NoError(s.T, err, "failed to create or get gallery image")

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
	// Create a new VHD config for Stage2 (ignore lock copying warning for now)
	customVHD := *s.Config.VHD
	customVHD.Name = *image.Name // Use the architecture-specific image name
	customVHD.Gallery = &config.Gallery{
		SubscriptionID:    config.Config.SubscriptionID,
		ResourceGroupName: rg,
		Name:              *gallery.Name,
	}
	customVHD.Version = version

	s.T.Logf("Created SIG image version: %s", version)

	return &customVHD
}

func validateSSHConnectivity(ctx context.Context, s *Scenario) error {
	connectionTest := fmt.Sprintf("%s echo 'SSH_CONNECTION_OK'", sshString(s.Runtime.VMPrivateIP))
	connectionResult, err := execOnPrivilegedPod(ctx, s.Runtime.Cluster.Kube, defaultNamespace, s.Runtime.Cluster.DebugPod.Name, connectionTest)

	if err != nil || !strings.Contains(connectionResult.stdout.String(), "SSH_CONNECTION_OK") {
		stderr := ""
		if connectionResult != nil {
			stderr = connectionResult.stderr.String()
		}

		return fmt.Errorf("SSH connection to %s failed: %v\nStderr: %s", s.Runtime.VMPrivateIP, err, stderr)
	}

	s.T.Logf("SSH connectivity to %s verified successfully", s.Runtime.VMPrivateIP)
	return nil
}
