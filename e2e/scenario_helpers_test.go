package e2e

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var once sync.Once

func RunScenario(t *testing.T, s *Scenario) {
	// without this, the test will not be able to catch the interrupt signal
	// and will not be able to clean up the resources or flush the logs
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	once.Do(func() {
		if err := createE2ELoggingDir(); err != nil {
			panic(err)
		}

		if err := ensureResourceGroup(ctx); err != nil {
			panic(err)
		}
	})
	t.Parallel()
	model, err := s.Cluster(ctx, t)
	require.NoError(t, err)
	maybeSkipScenario(ctx, t, s)
	loggingDir, err := createVMLogsDir(t.Name())
	require.NoError(t, err)
	nbc, err := s.PrepareNodeBootstrappingConfiguration(model.NodeBootstrappingConfiguration)
	require.NoError(t, err)

	executeScenario(ctx, t, &scenarioRunOpts{
		clusterConfig: model,
		scenario:      s,
		nbc:           nbc,
		loggingDir:    loggingDir,
	})
}

func maybeSkipScenario(ctx context.Context, t *testing.T, s *Scenario) {
	s.Tags.OS = s.VHD.OS
	s.Tags.Arch = s.VHD.Arch
	s.Tags.ImageName = s.VHD.Name
	t.Logf("running scenario %q with tags %+v", t.Name(), s.Tags)
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

	_, err := s.VHD.VHDResourceID(ctx, t)
	if err != nil {
		if config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			t.Skipf("skipping scenario %q: could not find image", t.Name())
		} else {
			t.Fatalf("could not find image for %q: %s", t.Name(), err)
		}
	}
}

func executeScenario(ctx context.Context, t *testing.T, opts *scenarioRunOpts) {
	rid, _ := opts.scenario.VHD.VHDResourceID(ctx, t)
	t.Logf("running scenario %q with image %q in aks cluster %q", t.Name(), rid, *opts.clusterConfig.Model.ID)

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)

	vmssName := getVmssName(t)

	vmssSucceeded := true
	_, err = bootstrapVMSS(ctx, t, vmssName, opts, privateKeyBytes, publicKeyBytes)
	if err != nil {
		vmssSucceeded = false
		if !isVMExtensionProvisioningError(err) {
			t.Fatalf("creating VMSS %s: %v", vmssName, err)
		}
		t.Logf("vm %s was unable to be provisioned due to a CSE error, will still attempt to extract provisioning logs...\n", vmssName)
		t.Fail()
	}

	vmPrivateIP, err := pollGetVMPrivateIP(ctx, t, vmssName, opts)
	require.NoError(t, err)

	// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
	t.Cleanup(func() {
		// original context can be cancelled, so create a new one
		err := pollExtractVMLogs(context.WithoutCancel(ctx), t, vmssName, vmPrivateIP, privateKeyBytes, opts)
		require.NoError(t, err)
	})

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		t.Logf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
		nodeName, err := validateNodeHealth(ctx, opts.clusterConfig.Kube, vmssName)
		require.NoError(t, err)

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			t.Logf("wasm scenario: running wasm validation on %s...", vmssName)
			err = ensureWasmRuntimeClasses(ctx, opts.clusterConfig.Kube)
			require.NoError(t, err)
			err = validateWasm(ctx, t, opts.clusterConfig.Kube, nodeName, string(privateKeyBytes))
			require.NoError(t, err)
		}

		t.Logf("node %s is ready, proceeding with validation commands...", vmssName)

		err = runLiveVMValidators(ctx, t, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
		require.NoError(t, err)

		t.Logf("node %s bootstrapping succeeded!", vmssName)
	}
}
