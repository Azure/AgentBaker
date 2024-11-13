package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
	"github.com/Azure/agentbakere2e/config"
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

	go func() {
		// block until signal is received
		<-ch
		fmt.Println("Received cancellation signal, gracefully shutting down the test suite. Cancel again to force exit.")
		cancel()

		// block until second signal is received
		<-ch
		fmt.Println("Received second cancellation signal, forcing exit.")
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

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	ctx := newTestCtx(t)
	cleanTestDir(t)
	ensureResourceGroupOnce(ctx)
	maybeSkipScenario(ctx, t, s)
	s.PrepareRuntime(ctx, t)
	createAndValidateVM(ctx, t, s)
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

func createAndValidateVM(ctx context.Context, t *testing.T, scenario *Scenario) {
	rid, _ := scenario.VHD.VHDResourceID(ctx, t)

	t.Logf("running scenario %q with image %q in aks cluster %q", t.Name(), rid, *scenario.Runtime.Cluster.Model.ID)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)

	vmssName := getVmssName(t)
	createVMSS(ctx, t, vmssName, scenario, privateKeyBytes, publicKeyBytes)

	t.Logf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
	nodeName := validateNodeHealth(ctx, t, scenario.Runtime.Cluster.Kube, vmssName, scenario.Tags.Airgap)

	// skip when outbound type is block as the wasm will create pod from gcr, however, network isolated cluster scenario will block egress traffic of gcr.
	// TODO(xinhl): add another way to validate
	if scenario.Runtime.NBC != nil && scenario.Runtime.NBC.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi && scenario.Runtime.NBC.OutboundType != datamodel.OutboundTypeBlock && scenario.Runtime.NBC.OutboundType != datamodel.OutboundTypeNone {
		validateWasm(ctx, t, scenario.Runtime.Cluster.Kube, nodeName)
	}
	if scenario.Runtime.AKSNodeConfig != nil && scenario.Runtime.AKSNodeConfig.WorkloadRuntime == nbcontractv1.WorkloadRuntime_WASM_WASI {
		validateWasm(ctx, t, scenario.Runtime.Cluster.Kube, nodeName)
	}

	t.Logf("node %s is ready, proceeding with validation commands...", vmssName)

	vmPrivateIP, err := getVMPrivateIPAddress(ctx, *scenario.Runtime.Cluster.Model.Properties.NodeResourceGroup, vmssName)
	require.NoError(t, err)

	require.NoError(t, err, "get vm private IP %v", vmssName)
	err = runLiveVMValidators(ctx, t, vmssName, vmPrivateIP, string(privateKeyBytes), scenario)
	require.NoError(t, err)

	t.Logf("node %s bootstrapping succeeded!", vmssName)
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
