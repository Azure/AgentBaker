package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	aksnodeconfigv1 "github.com/Azure/agentbaker/pkg/proto/aksnodeconfig/v1"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/assert"
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
		fmt.Println(red("Received cancellation signal, gracefully shutting down the test suite. Cancel again to force exit."))
		cancel()

		// block until second signal is received
		<-ch
		fmt.Println(red("Received second cancellation signal, forcing exit."))
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
	fmt.Printf("using E2E environment configuration:\n%s\n", config.Config)
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
	s.PrepareRuntime(ctx, t)
	createAndValidateVM(ctx, s)
}

func maybeSkipScenario(ctx context.Context, t *testing.T, s *Scenario) {
	s.Tags.Name = t.Name()
	s.Tags.OS = s.VHD.OS
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
			t.Skipf("skipping scenario %q: could not find image", t.Name())
		} else {
			t.Fatalf("could not find image for %q: %s", t.Name(), err)
		}
	}
	t.Logf("running scenario %q with vhd: %q, tags %+v", t.Name(), vhd, s.Tags)
}

func createAndValidateVM(ctx context.Context, s *Scenario) {
	rid, _ := s.VHD.VHDResourceID(ctx, s.T)

	s.T.Logf("running scenario %q with image %q in aks cluster %q", s.T.Name(), rid, *s.Runtime.Cluster.Model.ID)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(s.T, err)

	createVMSS(ctx, s.T, s.Runtime.VMSSName, s, privateKeyBytes, publicKeyBytes)

	err = getCustomScriptExtensionStatus(ctx, s.T, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName)
	require.NoError(s.T, err)

	s.T.Logf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", s.Runtime.VMSSName)
	nodeName := s.validateNodeHealth(ctx)

	// skip when outbound type is block as the wasm will create pod from gcr, however, network isolated cluster scenario will block egress traffic of gcr.
	// TODO(xinhl): add another way to validate
	if s.Runtime.NBC != nil && s.Runtime.NBC.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi && s.Runtime.NBC.OutboundType != datamodel.OutboundTypeBlock && s.Runtime.NBC.OutboundType != datamodel.OutboundTypeNone {
		validateWasm(ctx, s.T, s.Runtime.Cluster.Kube, nodeName)
	}
	if s.Runtime.AKSNodeConfig != nil && s.Runtime.AKSNodeConfig.WorkloadRuntime == aksnodeconfigv1.WorkloadRuntime_WASM_WASI {
		validateWasm(ctx, s.T, s.Runtime.Cluster.Kube, nodeName)
	}

	s.T.Logf("node %s is ready, proceeding with validation commands...", s.Runtime.VMSSName)

	vmPrivateIP, err := getVMPrivateIPAddress(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName)

	require.NoError(s.T, err, "get vm private IP %v", s.Runtime.VMSSName)
	err = runLiveVMValidators(ctx, s.T, s.Runtime.VMSSName, vmPrivateIP, string(privateKeyBytes), s)
	require.NoError(s.T, err)

	s.T.Logf("node %s bootstrapping succeeded!", s.Runtime.VMSSName)
}

func getExpectedPackageVersions(packageName, distro, release string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here
	jsonBytes, _ := os.ReadFile("../parts/linux/cloud-init/artifacts/components.json")
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

func getCustomScriptExtensionStatus(ctx context.Context, t *testing.T, resourceGroupName, vmssName string) error {
	pager := config.Azure.VMSSVM.NewListPager(resourceGroupName, vmssName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get VMSS instances: %v", err)
		}

		for _, vmInstance := range page.Value {
			instanceViewResp, err := config.Azure.VMSSVM.GetInstanceView(ctx, resourceGroupName, vmssName, *vmInstance.InstanceID, nil)
			if err != nil {
				return fmt.Errorf("failed to get instance view for VM %s: %v", *vmInstance.InstanceID, err)
			}
			for _, extension := range instanceViewResp.Extensions {
				for _, status := range extension.Statuses {
					resp, err := parseLinuxCSEMessage(*status)
					if err != nil {
						return fmt.Errorf("Parse CSE message with error, error %w", err)
					}
					if resp.ExitCode != "0" {
						return fmt.Errorf("vmssCSE %s, output=%s, error=%s", resp.ExitCode, resp.Output, resp.Error)
					}
					t.Logf("CSE completed successfully with exit code 0, cse output: %s", *status.Message)
					return nil
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
