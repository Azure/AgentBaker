package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
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

var scenarioOnce sync.Once

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	ctx := newTestCtx(t)
	scenarioOnce.Do(func() {
		err := ensureResourceGroup(ctx)
		if err != nil {
			panic(err)
		}
	})
	maybeSkipScenario(ctx, t, s)

	model, err := s.Cluster(ctx, t)
	require.NoError(t, err, "creating AKS cluster")

	nbc, err := s.PrepareNodeBootstrappingConfiguration(model.NodeBootstrappingConfiguration)
	require.NoError(t, err)

	executeScenario(ctx, t, &scenarioRunOpts{
		clusterConfig: model,
		scenario:      s,
		nbc:           nbc,
	})
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

func executeScenario(ctx context.Context, t *testing.T, opts *scenarioRunOpts) {
	rid, _ := opts.scenario.VHD.VHDResourceID(ctx, t)
	t.Logf("running scenario %q with image %q in aks cluster %q", t.Name(), rid, *opts.clusterConfig.Model.ID)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)

	vmssName := getVmssName(t)
	createVMSS(ctx, t, vmssName, opts, privateKeyBytes, publicKeyBytes)

	t.Logf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
	nodeName := validateNodeHealth(ctx, t, opts.clusterConfig.Kube, vmssName)

	if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
		validateWasm(ctx, t, opts.clusterConfig.Kube, nodeName)
	}

	t.Logf("node %s is ready, proceeding with validation commands...", vmssName)

	vmPrivateIP, err := getVMPrivateIPAddress(ctx, *opts.clusterConfig.Model.Properties.NodeResourceGroup, vmssName)
	require.NoError(t, err)

	require.NoError(t, err, "get vm private IP %v", vmssName)
	err = runLiveVMValidators(ctx, t, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
	require.NoError(t, err)

	t.Logf("node %s bootstrapping succeeded!", vmssName)
}

func getExpectedPackageVersions(packageName, distro, release string) []string {
	var expectedVersions []string
	// since we control this json, we assume its going to be properly formatted here
	jsonBytes, _ := os.ReadFile("../parts/linux/cloud-init/artifacts/components.json")
	packages := gjson.GetBytes(jsonBytes, fmt.Sprintf("Packages.#(name=%s).downloadURIs", packageName))

	for _, packageItem := range packages.Array() {
		versions := packageItem.Get(fmt.Sprintf("%s.%s.versions", distro, release))
		if !versions.Exists() {
			versions = packageItem.Get(fmt.Sprintf("%s.current.versions", distro))
		}
		if !versions.Exists() {
			versions = packageItem.Get("default.current.versions")
		}
		if versions.Exists() {
			for _, version := range versions.Array() {
				expectedVersions = append(expectedVersions, version.String())
			}
		}
	}
	return expectedVersions
}
