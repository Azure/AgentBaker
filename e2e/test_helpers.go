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
	"sync"
	"syscall"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/barkimedes/go-deepcopy"
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

func newTestCtx(t *testing.T, location string) context.Context {
	if testCtx.Err() != nil {
		msg := constructErrorMessage(config.Config.SubscriptionID, location)
		t.Skip("test suite is shutting down: " + msg)
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

// Global state to track which locations have been initialized
var (
	// Track which locations have been initialized
	initializedLocations = make(map[string]bool)
	// Mutex to protect the map access
	locationMutex sync.Mutex
)

// ensureLocationInitialized ensures that both resource group and managed identity
// are created for a location, but only runs once per location across all tests
func ensureLocationInitialized(ctx context.Context, t *testing.T, location string) {
	locationMutex.Lock()
	defer locationMutex.Unlock()

	// Check if this location has already been initialized
	if initializedLocations[location] {
		t.Logf("Location %s is already initialized, skipping", location)
		return
	}

	// Initialize the location
	err := ensureResourceGroup(ctx, location)
	mustNoError(err)
	_, err = config.Azure.CreateVMManagedIdentity(ctx, location)
	mustNoError(err)

	// Mark this location as initialized
	initializedLocations[location] = true
}

func RunScenario(t *testing.T, s *Scenario) {
	s.T = t
	t.Parallel()

	if s.Location == "" {
		s.Location = config.Config.DefaultLocation
	}

	ctx := newTestCtx(t, s.Location)
	ensureLocationInitialized(ctx, t, s.Location)

	ctrruntimelog.SetLogger(zap.New())

	maybeSkipScenario(ctx, t, s)

	cluster, err := s.Config.Cluster(ctx, s.Location, s.T)
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

	t.Logf("Choosing the private ACR %q for the vm validation", config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location))
	validateVM(ctx, s)
}

func prepareAKSNode(ctx context.Context, s *Scenario) {
	s.Runtime.VMSSName = generateVMSSName(s)
	if (s.BootstrapConfigMutator == nil) == (s.AKSNodeConfigMutator == nil) {
		s.T.Fatalf("exactly one of BootstrapConfigMutator or AKSNodeConfigMutator must be set")
	}

	nbc := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)

	if s.VHD.OS == config.OSWindows {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/"
	}

	if s.BootstrapConfigMutator != nil {
		// deep copy the nbc so that we can mutate it without affecting the original
		clonedNbc, err := deepcopy.Anything(nbc)
		if err != nil {
			s.T.Fatalf("failed to deep copy node config: %v", err)
		}

		// Pass the cloned nbc to BootstrapConfigMutator so that it can mutate the properties but not affecting the original one.
		// Without this, it will cause a race condition when running multiple tests in parallel.
		s.BootstrapConfigMutator(clonedNbc.(*datamodel.NodeBootstrappingConfiguration))
		s.Runtime.NBC = clonedNbc.(*datamodel.NodeBootstrappingConfiguration)
	}
	if s.AKSNodeConfigMutator != nil {
		nodeconfig := nbcToAKSNodeConfigV1(nbc)

		// deep copy the node config so that we can mutate it without affecting the original
		clonedNodeConfig, err := deepcopy.Anything(nodeconfig)
		if err != nil {
			s.T.Fatalf("failed to deep copy node config: %v", err)
		}

		// Pass the cloned clonedNodeConfig to AKSNodeConfigMutator so that it can mutate the properties but not affecting the original one.
		// Without this, it will cause a race condition when running multiple tests in parallel.
		s.AKSNodeConfigMutator(clonedNodeConfig.(*aksnodeconfigv1.Configuration))
		s.Runtime.AKSNodeConfig = clonedNodeConfig.(*aksnodeconfigv1.Configuration)
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
	vmssCreatedAt := time.Now()         // Record the start time
	creationElapse := time.Since(start) // Calculate the elapsed time

	s.T.Logf("vmss %s creation succeeded", s.Runtime.VMSSName)

	s.Runtime.KubeNodeName = s.Runtime.Cluster.Kube.WaitUntilNodeReady(ctx, s.T, s.Runtime.VMSSName)
	readyElapse := time.Since(vmssCreatedAt) // Calculate the elapsed time
	totalElapse := time.Since(start)
	s.T.Logf("node %s is ready", s.Runtime.VMSSName)

	toolkit.LogDuration(totalElapse, 3*time.Minute, fmt.Sprintf("Node %s took %s to be created and %s to be ready\n", s.Runtime.VMSSName, toolkit.FormatDuration(creationElapse), toolkit.FormatDuration(readyElapse)))

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

	vhd, err := s.VHD.VHDResourceID(ctx, t, s.Location)
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

func validateNodeCanRunAPod(ctx context.Context, s *Scenario) {
	if s.VHD.OS == config.OSWindows {
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
	validateNodeCanRunAPod(ctx, s)

	switch s.VHD.OS {
	case config.OSWindows:
		// TODO: validate something
	default:
		ValidateCommonLinux(ctx, s)
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
					if s.VHD.OS == config.OSWindows {
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

func GetKubeletVersionByMinorVersion(minorVersion string) string {
	allCachedKubeletVersions := getExpectedPackageVersions("kubernetes-binaries", "default", "current")
	rightVersions := toolkit.Filter(allCachedKubeletVersions, func(v string) bool { return strings.HasPrefix(v, minorVersion) })
	rightVersion := toolkit.Reduce(rightVersions, "", func(sum string, next string) string {
		if sum == "" {
			return next
		}
		if next > sum {
			return next
		}
		return sum
	})
	return rightVersion
}

func RemoveLeadingV(version string) string {
	if len(version) > 0 && version[0] == 'v' {
		return version[1:]
	}
	return version
}
