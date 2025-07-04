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

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	ctrruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
		fmt.Println(red("Received cancellation signal, gracefully shutting down the test suite. Cancel again to force exit. (Created Azure resources will not be deleted in this case)"))
		cancel()

		// block until second signal is received
		<-ch
		msg := fmt.Sprintf("Received second cancellation signal, forcing exit.\nPlease check https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/overview and delete any resources created by the test suite", config.Config.SubscriptionID, config.ResourceGroupName)
		fmt.Println(red(msg))
		os.Exit(1)
	}()
	return ctx
}

func newTestCtx(t *testing.T) context.Context {
	if testCtx.Err() != nil {
		t.Skip("test suite is shutting down")
	}
	ctx, cancel := context.WithTimeout(testCtx, config.Config.TestTimeout)
	t.Cleanup(cancel)
	return ctx
}

func mustNoError(err error) {
	if err != nil {
		panic(err)
	}
}

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	if config.Config.TestPreProvision {
		// Run two versions of the same scenario, one with pre-provisioning enabled and one without.
		// In parallel
		t.Run("Original", func(t *testing.T) {
			t.Parallel()
			runScenario(t, s)
		})
		t.Run("PreProvision", func(t *testing.T) {
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
	runScenario(t, &Scenario{
		Description: original.Description,
		Tags:        original.Tags,
		Config: Config{
			Cluster:               original.Cluster,
			VHD:                   original.VHD,
			SkipDefaultValidation: true, // Skip default validation, VM isn't ready at stage 1
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				original.VMConfigMutator(vmss)
				// Configure VM to use persistent disk instead of ephemeral for image capture
				if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
					vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
				}
			},
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				original.BootstrapConfigMutator(nbc)
				nbc.PreProvisionOnly = true
			},
			Validator: func(ctx context.Context, stage1 *Scenario) {
				if stage1.IsWindows() {
					ValidateFileExists(ctx, stage1, "C:\\AzureData\\preprovision.complete")
					ValidateFileDoesNotExist(ctx, stage1, "C:\\AzureData\\provision.complete")
					ValidateWindowsServiceIsNotRunning(ctx, stage1, "kubelet")
				} else {
					ValidateFileExists(ctx, stage1, "/etc/containerd/config.toml")
					ValidateSystemdUnitIsRunning(ctx, stage1, "containerd")
					ValidateFileExists(ctx, stage1, "/opt/azure/containers/preprovision.complete")
					ValidateFileDoesNotExist(ctx, stage1, "/opt/azure/containers/provision.complete")
				}

				t.Log("=== Stage 1 validation complete, proceeding to Stage 2 ===")

				// VM is automatically deleted after the test.
				// We run Subtest in the Validator to capture VHD before it's deleted
				customVHD := CreateImage(ctx, stage1)
				stage1SSHKeys := stage1.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys

				// Create a subtest so RunScenario won't fail on t.Parallel()
				t.Run("SecondStage", func(t *testing.T) {
					// Run Stage 2 scenario using the custom VHD
					// RunScenario fails due to the running of the t.Parallel() for the second time
					runScenario(t, &Scenario{
						Description: "Stage 2: Create VMSS from captured VHD via SIG",
						Tags:        stage1.Tags,
						Config: Config{
							Cluster:               stage1.Config.Cluster,
							VHD:                   customVHD,
							SkipDefaultValidation: original.Config.SkipDefaultValidation,
							BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
								if nbc.ContainerService.Properties.LinuxProfile != nil {
									nbc.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = stage1SSHKeys
								}
							},
							VMConfigMutator: original.VMConfigMutator,
							Validator: func(ctx context.Context, s *Scenario) {
								// Stage 2 validation: Verify kubelet is now working
								if s.IsWindows() {
									ValidateFileExists(ctx, s, "C:\\AzureData\\preprovision.complete") // Test with known existing file first
									ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
									ValidateWindowsServiceIsRunning(ctx, s, "kubelet")
								} else {
									ValidateFileHasContent(ctx, s, "/var/log/azure/cluster-provision.log", "Running in kubelet-only mode")
									ValidateSystemdUnitIsRunning(ctx, s, "kubelet")
									ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
								}
								original.Config.Validator(ctx, s)
							},
						},
					})
				})
			},
		},
	})
}

func runScenario(t *testing.T, s *Scenario) {
	s.T = t
	ctx := newTestCtx(t)
	ctrruntimelog.SetLogger(zap.New())

	maybeSkipScenario(ctx, t, s)
	cluster, err := s.Config.Cluster(ctx, s.T)
	require.NoError(s.T, err)
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

	t.Logf("Choosing the private ACR %q for the vm validation", config.GetPrivateACRName(s.Tags.NonAnonymousACR))
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
		// Pass the cloned nbc to BootstrapConfigMutator so that it can mutate the properties but not affecting the original one.
		// Without this, it will cause a race condition when running multiple tests in parallel.
		s.BootstrapConfigMutator(nbc)
		s.Runtime.NBC = nbc
	}
	if s.AKSNodeConfigMutator != nil {
		nodeconfig := nbcToAKSNodeConfigV1(nbc)
		// Pass the cloned clonedNodeConfig to AKSNodeConfigMutator so that it can mutate the properties but not affecting the original one.
		// Without this, it will cause a race condition when running multiple tests in parallel.
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
		toolkit.LogDuration(totalElapse, 3*time.Minute, fmt.Sprintf("Node %s took %s to be created and %s to be ready\n", s.Runtime.VMSSName, creationElapse, readyElapse))
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

	vhd, err := s.VHD.VHDResourceID(ctx, t)
	if err != nil {
		if config.Config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			t.Skipf("skipping scenario %q: could not find image for VHD %s due to %s", t.Name(), s.VHD.Distro, err)
		} else {
			t.Fatalf("failing scenario %q: could not find image for VHD %s due to %s", t.Name(), s.VHD.Distro, err)
		}
	}
	t.Logf("VHD: %q, TAGS %+v", vhd, s.Tags)
}

func getWindowsEnvVarForName(vhd *config.Image) string {
	return strings.TrimPrefix(vhd.Name, "windows-")
}

func getServercoreImagesForVHD(vhd *config.Image) []string {
	return getWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", getWindowsEnvVarForName(vhd))
}

func getNanoserverImagesForVhd(vhd *config.Image) []string {
	return getWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", getWindowsEnvVarForName(vhd))
}

func ValidateNodeCanRunAPod(ctx context.Context, s *Scenario) {
	if s.IsWindows() {
		serverCorePods := getServercoreImagesForVHD(s.VHD)
		for i, pod := range serverCorePods {
			ValidatePodRunning(ctx, s, podWindows(s, fmt.Sprintf("servercore%d", i), pod))
		}

		nanoServerPods := getNanoserverImagesForVhd(s.VHD)
		for i, pod := range nanoServerPods {
			ValidatePodRunning(ctx, s, podWindows(s, fmt.Sprintf("nanoserver%d", i), pod))
		}
	} else {
		ValidatePodRunning(ctx, s, podHTTPServerLinux(s))
	}
}

func validateVM(ctx context.Context, s *Scenario) {
	// the instructions belows expects the SSH key to be uploaded to the user pool VM.
	// which happens as a side-effect of execCommandOnVMForScenario, it's ugly but works.
	// maybe we should use a single ssh key per cluster, but need to be careful with parallel test runs.
	err := uploadSSHKey(ctx, s)
	if err != nil {
		s.T.Logf("failed to upload SSH key: %v", err)
	}

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

func getExpectedPackageVersions(packageName, distro, release string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here
	jsonBytes, _ := os.ReadFile("../parts/common/components.json")
	packages := gjson.GetBytes(jsonBytes, fmt.Sprintf("Packages.#(name=%s).downloadURIs", packageName))

	for _, packageItem := range packages.Array() {
		// check if versionsV2 exists
		if packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release)).Exists() {
			versions := packageItem.Get(fmt.Sprintf("%s.%s.versionsV2", distro, release))
			for _, version := range versions.Array() {
				// get versions.latestVersion and append to expectedVersions
				expectedVersions = append(expectedVersions, version.Get("latestVersion").String())
				// get versions.previousLatestVersion (if exists) and append to expectedVersions
				if version.Get("previousLatestVersion").Exists() {
					expectedVersions = append(expectedVersions, version.Get("previousLatestVersion").String())
				}
			}
		}
	}
	return expectedVersions
}

func getWindowsContainerImages(containerName string, windowsVersion string) []string {
	return toolkit.Map(getWindowsContainerImageTags(containerName, windowsVersion), func(tag string) string {
		return strings.Replace(containerName, "*", tag, 1)
	})
}

// TODO: expand this logic to support linux container images as well
func getWindowsContainerImageTags(containerName string, windowsVersion string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here
	jsonBytes, _ := os.ReadFile("../parts/common/components.json")

	containerImages := gjson.GetBytes(jsonBytes, "ContainerImages") //fmt.Sprintf("ContainerImages", containerName))

	for _, containerImage := range containerImages.Array() {
		imageDownloadUrl := containerImage.Get("downloadURL").String()
		if strings.EqualFold(imageDownloadUrl, containerName) {
			packages := containerImage.Get("windowsVersions")
			//t.Logf("got packages: %s", packages.String())

			for _, packageItem := range packages.Array() {
				// check if versionsV2 exists
				if packageItem.Get("windowsSkuMatch").Exists() {
					windowsSkuMatch := packageItem.Get("windowsSkuMatch").String()
					matched, err := filepath.Match(windowsSkuMatch, windowsVersion)
					if matched && err == nil {

						// get versions.latestVersion and append to expectedVersions
						expectedVersions = append(expectedVersions, packageItem.Get("latestVersion").String())
						// get versions.previousLatestVersion (if exists) and append to expectedVersions
						if packageItem.Get("previousLatestVersion").Exists() {
							expectedVersions = append(expectedVersions, packageItem.Get("previousLatestVersion").String())
						}
					}
				} else {
					// get versions.latestVersion and append to expectedVersions
					expectedVersions = append(expectedVersions, packageItem.Get("latestVersion").String())
					// get versions.previousLatestVersion (if exists) and append to expectedVersions
					if packageItem.Get("previousLatestVersion").Exists() {
						expectedVersions = append(expectedVersions, packageItem.Get("previousLatestVersion").String())
					}
				}
			}
		}
	}

	return expectedVersions
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

	diskName := *vm.Properties.StorageProfile.OSDisk.Name
	snapshotName := fmt.Sprintf("snap-%s", time.Now().Format("20060102-150405"))
	s.T.Logf("Creating snapshot '%s' from disk '%s'...", snapshotName, diskName)

	snapshotPoller, err := config.Azure.Snapshots.BeginCreateOrUpdate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, snapshotName,
		armcompute.Snapshot{
			Location: to.Ptr("westus3"),
			Properties: &armcompute.SnapshotProperties{
				CreationData: &armcompute.CreationData{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionCopy),
					SourceResourceID: to.Ptr(fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s",
						config.Config.SubscriptionID, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, diskName)),
				},
			},
		}, nil)
	require.NoError(s.T, err, "Failed to begin snapshot creation")

	s.T.Cleanup(func() {
		s.T.Logf("Cleaning up snapshot '%s'...", snapshotName)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = config.Azure.Snapshots.BeginDelete(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, snapshotName, nil)
	})

	snapshot, err := snapshotPoller.PollUntilDone(ctx, nil)
	require.NoError(s.T, err, "Failed to create snapshot for disk creation")

	imageName := "image-" + snapshotName
	s.T.Logf("Creating image '%s' from snapshot '%s'...", imageName, snapshotName)
	imagePoller, err := config.Azure.Images.BeginCreateOrUpdate(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, imageName, armcompute.Image{
		Location: vm.Location,
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType: func() *armcompute.OperatingSystemTypes {
						if s.IsWindows() {
							return to.Ptr(armcompute.OperatingSystemTypesWindows)
						}
						return to.Ptr(armcompute.OperatingSystemTypesLinux)
					}(),
					OSState: to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
					Snapshot: &armcompute.SubResource{
						ID: snapshot.ID,
					},
				},
			},
		},
	}, nil)
	require.NoError(s.T, err, "Failed to begin create image snapshot")
	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = config.Azure.Images.BeginDelete(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, imageName, nil)
	})

	image, err := imagePoller.PollUntilDone(ctx, nil)
	require.NoError(s.T, err, "Failed to create image snapshot")

	return &config.Image{
		Name:   imageName,
		OS:     s.Config.VHD.OS,
		Arch:   s.Config.VHD.Arch,
		Distro: s.Config.VHD.Distro,
		Gallery: &config.Gallery{
			SubscriptionID:    config.Config.SubscriptionID,
			ResourceGroupName: *s.Runtime.Cluster.Model.Properties.NodeResourceGroup,
			Name:              "managed-disks", // Special marker for managed disks
		},
		Version: *image.ID, // Store the managed image ID in Version field
	}
}
