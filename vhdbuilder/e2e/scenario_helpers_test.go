package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestMain(m *testing.M) {
	log.Printf("using E2E environment configuration:\n%s\n", config.Config)
	// clean up logs from previous run
	if _, err := os.Stat("scenario-logs"); err == nil {
		_ = os.RemoveAll("scenario-logs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := ensureResourceGroup(ctx)
	mustNoError(err)
	_, err = config.Azure.CreateVMManagedIdentity(ctx)
	mustNoError(err)
	m.Run()
}

func mustNoError(err error) {
	if err != nil {
		panic(err)
	}
}

func RunScenario(t *testing.T, s *Scenario) {
	s.T = t
	t.Parallel()
	ctx := newTestCtx(t)
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
	if s.VHD.OS == config.OSWindows {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = windowsCSE(ctx, s.T)
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

	createVMSS(ctx, s)

	err = getCustomScriptExtensionStatus(ctx, s)
	require.NoError(s.T, err)
	s.T.Logf("vmss %s creation succeeded", s.Runtime.VMSSName)

	s.Runtime.KubeNodeName = s.Runtime.Cluster.Kube.WaitUntilNodeReady(ctx, s.T, s.Runtime.VMSSName)
	s.T.Logf("node %s is ready", s.Runtime.VMSSName)

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
			t.Skipf("skipping scenario %q: could not find image for VHD %s due to %s", t.Name(), s.VHD.String(), err)
		} else {
			t.Fatalf("could not find image for %q (VHD %s): %s", t.Name(), s.VHD.String(), err)
		}
	}
	t.Logf("VHD: %q, TAGS %+v", vhd, s.Tags)
}

func validateVM(ctx context.Context, s *Scenario) {
	ValidatePodRunning(ctx, s)

	// skip when outbound type is block as the wasm will create pod from gcr, however, network isolated cluster scenario will block egress traffic of gcr.
	// TODO(xinhl): add another way to validate
	if s.Runtime.NBC != nil && s.Runtime.NBC.AgentPoolProfile != nil && s.Runtime.NBC.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi && s.Runtime.NBC.OutboundType != datamodel.OutboundTypeBlock && s.Runtime.NBC.OutboundType != datamodel.OutboundTypeNone {
		ValidateWASM(ctx, s, s.Runtime.KubeNodeName)
	}
	if s.Runtime.AKSNodeConfig != nil && s.Runtime.AKSNodeConfig.WorkloadRuntime == aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_WASM_WASI {
		ValidateWASM(ctx, s, s.Runtime.KubeNodeName)
	}

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
