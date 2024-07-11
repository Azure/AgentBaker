package e2e

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/barkimedes/go-deepcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := createE2ELoggingDir(); err != nil {
		panic(err)
	}

	if err := ensureResourceGroup(context.Background()); err != nil {
		panic(err)
	}
	m.Run()

}

func RunScenario(t *testing.T, s *Scenario) {
	t.Parallel()
	ctx := context.Background()
	model, err := s.Cluster(ctx)
	require.NoError(t, err)
	maybeSkipScenario(t, s)
	setupAndRunScenario(ctx, t, s, model)
}

func maybeSkipScenario(t *testing.T, s *Scenario) {
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

	rid, err := s.VHDSelector()
	if err != nil {
		if config.IgnoreScenariosWithMissingVHD && errors.Is(err, config.ErrNotFound) {
			t.Skipf("skipping scenario %q: could not find image", t.Name())
		} else {
			t.Fatalf("could not find image for %q: %s", t.Name(), err)
		}
	}
	t.Logf("running scenario %q with image %q", t.Name(), rid)
}

func setupAndRunScenario(ctx context.Context, t *testing.T, e2eScenario *Scenario, clusterConfig *cluster.Cluster) {
	log.Printf("chose cluster: %q", *clusterConfig.Model.ID)

	clusterParams, err := pollExtractClusterParameters(ctx, clusterConfig.Kube)
	require.NoError(t, err)

	baseNodeBootstrappingConfig, err := getBaseNodeBootstrappingConfiguration(clusterParams)
	require.NoError(t, err)

	copied, err := deepcopy.Anything(baseNodeBootstrappingConfig)
	require.NoError(t, err)
	nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

	e2eScenario.PrepareNodeBootstrappingConfiguration(nbc)

	loggingDir, err := createVMLogsDir(t.Name())
	require.NoError(t, err)

	executeScenario(ctx, t, &scenarioRunOpts{
		clusterConfig: clusterConfig,
		scenario:      e2eScenario,
		nbc:           nbc,
		loggingDir:    loggingDir,
	})
}

func executeScenario(ctx context.Context, t *testing.T, opts *scenarioRunOpts) {
	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair()
	assert.NoError(t, err)

	vmssName := getVmssName()
	log.Printf("creating and bootstrapping vmss: %q", vmssName)

	vmssSucceeded := true
	vmssModel, err := bootstrapVMSS(ctx, t, vmssName, opts, publicKeyBytes)
	if err != nil {
		vmssSucceeded = false
		if config.SkipTestsWithSKUCapacityIssue {
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 409 && respErr.ErrorCode == "SkuNotAvailable" {
				t.Skip("skipping scenario SKU not available", t.Name(), err)
			}
		}

		if !isVMExtensionProvisioningError(err) {
			t.Fatalf("encountered an unknown error while creating VM %s: %v", vmssName, err)
		}
		log.Printf("vm %s was unable to be provisioned due to a CSE error, will still attempt to extract provisioning logs...\n", vmssName)
	}

	if config.KeepVMSS {
		defer func() {
			log.Printf("vmss %q will be retained for debugging purposes, please make sure to manually delete it later", vmssName)
			if vmssModel != nil {
				log.Printf("retained vmss %s resource ID: %q", vmssName, *vmssModel.ID)
			} else {
				log.Printf("WARNING: model of retained vmss %q is nil", vmssName)
			}
			if err := writeToFile(filepath.Join(opts.loggingDir, "sshkey"), string(privateKeyBytes)); err != nil {
				t.Fatalf("failed to write retained vmss %s private ssh key to disk: %s", vmssName, err)
			}
		}()
	} else {
		if vmssModel != nil {
			if err := writeToFile(filepath.Join(opts.loggingDir, "vmssId.txt"), *vmssModel.ID); err != nil {
				t.Fatalf("failed to write vmss %s resource ID to disk: %s", vmssName, err)
			}
		} else {
			log.Printf("WARNING: bootstrapped vmss model was nil for %s", vmssName)
		}
	}

	vmPrivateIP, err := pollGetVMPrivateIP(ctx, vmssName, opts)
	if err != nil {
		t.Fatalf("failed to get VM %s private IP: %s", vmssName, err)
	}

	// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
	defer func() {
		err := pollExtractVMLogs(ctx, vmssName, vmPrivateIP, privateKeyBytes, opts)
		require.NoError(t, err)
	}()

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		log.Printf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
		nodeName, err := validateNodeHealth(ctx, opts.clusterConfig.Kube, vmssName)
		if err != nil {
			t.Fatal(err)
		}

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			log.Printf("wasm scenario: running wasm validation on %s...", vmssName)
			if err := ensureWasmRuntimeClasses(ctx, opts.clusterConfig.Kube); err != nil {
				t.Fatalf("unable to ensure wasm RuntimeClasses on %s: %s", vmssName, err)
			}
			if err := validateWasm(ctx, opts.clusterConfig.Kube, nodeName, string(privateKeyBytes)); err != nil {
				t.Fatalf("unable to validate wasm on %s: %s", vmssName, err)
			}
		}

		log.Printf("node %s is ready, proceeding with validation commands...", vmssName)

		err = runLiveVMValidators(ctx, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
		if err != nil {
			t.Fatalf("vm %s validation failed: %s", vmssName, err)
		}

		log.Printf("node %s bootstrapping succeeded!", vmssName)
	} else {
		t.Fatalf("vmss %s was unable to be properly created and bootstrapped", vmssName)
	}
}
