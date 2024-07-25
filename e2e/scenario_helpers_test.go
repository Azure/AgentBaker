package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	// without this, the test will not be able to catch the interrupt signal
	// and will not be able to clean up the resources or flush the logs
	// TODO: this isn't ideal, as the test can be started after the signal is sent so it will not be caught
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	t.Cleanup(cancel)
	ctx, cancel = context.WithTimeout(ctx, config.TestTimeout)
	t.Cleanup(cancel)
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
	s.Tags.OS = s.VHD.OS
	s.Tags.Arch = s.VHD.Arch
	s.Tags.ImageName = s.VHD.Name
	if config.TagsToRun != "" {
		matches, err := s.Tags.MatchesFilters(config.TagsToRun)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if !matches {
			t.Skipf("skipping scenario %q: scenario tags %+v does not match filter %q", t.Name(), s.Tags, config.TagsToRun)
		}
	}

	if config.TagsToSkip != "" {
		matches, err := s.Tags.MatchesAnyFilter(config.TagsToSkip)
		if err != nil {
			t.Fatalf("could not match tags for %q: %s", t.Name(), err)
		}
		if matches {
			t.Skipf("skipping scenario %q: scenario tags %+v matches filter %q", t.Name(), s.Tags, config.TagsToSkip)
		}
	}

	vhd, err := s.VHD.VHDResourceID(ctx, t)
	if err != nil {
		if config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
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
		t.Logf("wasm scenario: running wasm validation on %s...", vmssName)
		err = ensureWasmRuntimeClasses(ctx, opts.clusterConfig.Kube)
		require.NoError(t, err)
		err = validateWasm(ctx, t, opts.clusterConfig.Kube, nodeName, string(privateKeyBytes))
		require.NoError(t, err)
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
	jsonBytes, _ := os.ReadFile("../vhdbuilder/packer/components.json")
	versions := gjson.GetBytes(jsonBytes, fmt.Sprintf("Packages.#(name=%s).downloadURIs", packageName)).Get(fmt.Sprintf("%s.%s.versions", distro, release)).Array()
	for _, version := range versions {
		expectedVersions = append(expectedVersions, version.String())
	}
	return expectedVersions
}
