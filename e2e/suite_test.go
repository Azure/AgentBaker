package e2e_test

import (
	"context"
	mrand "math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/barkimedes/go-deepcopy"
)

func Test_All(t *testing.T) {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	ctx := context.Background()
	t.Parallel()

	suiteConfig, err := newSuiteConfig()
	if err != nil {
		t.Fatal(err)
	}

	if err := createE2ELoggingDir(); err != nil {
		t.Fatal(err)
	}

	scenarioTable := scenario.InitScenarioTable(t, suiteConfig.scenariosToRun)

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureResourceGroup(ctx, t, cloud, suiteConfig.resourceGroupName); err != nil {
		t.Fatal(err)
	}

	clusters, err := listClusters(ctx, t, cloud, suiteConfig.resourceGroupName)
	if err != nil {
		t.Fatal(err)
	}

	paramCache := paramCache{}

	for _, scenario := range scenarioTable {
		scenario := scenario

		kube, cluster, clusterParams, subnetID := mustChooseCluster(ctx, t, r, cloud, suiteConfig, scenario, &clusters, paramCache)

		clusterName := *cluster.Name
		t.Logf("chose cluster: %q", clusterName)

		baseConfig, err := getBaseNodeBootstrappingConfiguration(ctx, t, cloud, suiteConfig, clusterParams)
		if err != nil {
			t.Fatal(err)
		}

		copied, err := deepcopy.Anything(baseConfig)
		if err != nil {
			t.Error(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		if scenario.Config.BootstrapConfigMutator != nil {
			scenario.Config.BootstrapConfigMutator(nbc)
		}

		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()

			caseLogsDir, err := createVMLogsDir(scenario.Name)
			if err != nil {
				t.Fatal(err)
			}

			opts := &scenarioRunOpts{
				cloud:         cloud,
				kube:          kube,
				suiteConfig:   suiteConfig,
				scenario:      scenario,
				chosenCluster: cluster,
				nbc:           nbc,
				subnetID:      subnetID,
				loggingDir:    caseLogsDir,
			}

			runScenario(ctx, t, r, opts)
		})
	}
}

func runScenario(ctx context.Context, t *testing.T, r *mrand.Rand, opts *scenarioRunOpts) {
	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair(r)
	if err != nil {
		t.Error(err)
		return
	}

	privateKey := string(privateKeyBytes)

	vmssName, vmssModel, cleanupVMSS, err := bootstrapVMSS(ctx, t, r, publicKeyBytes, opts)
	if cleanupVMSS != nil {
		defer cleanupVMSS()
	}
	isCSEError := isVMExtensionProvisioningError(err)
	vmssSucceeded := true
	if err != nil {
		vmssSucceeded = false
		if !isCSEError {
			t.Fatalf("Encountered an unknown error while creating VM: %s", err)
		}
		t.Log("VM was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...")
	}

	if vmssModel != nil {
		if err := writeToFile(filepath.Join(opts.loggingDir, "vmssId.txt"), *vmssModel.ID); err != nil {
			t.Fatal("failed to write vmss resource ID to disk", err)
		}
	}

	vmPrivateIP, err := pollGetVMPrivateIP(ctx, vmssName, opts)
	if err != nil {
		t.Fatalf("failed to get VM private IP: %s", err)
	}

	defer func() {
		err := pollExtractVMLogs(ctx, vmssName, privateKey, vmPrivateIP, opts)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		t.Log("vmss creation succeded, proceeding with node readiness and pod checks...")
		nodeName, err := validateNodeHealth(ctx, t, opts.kube, vmssName)
		if err != nil {
			t.Fatalf("unable to validate node health: %s", err)
		}

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			t.Log("wasm scenario: running wasm validation...")
			if err := ensureWasmRuntimeClasses(ctx, opts.kube); err != nil {
				t.Fatalf("unable to ensure wasm RuntimeClasses: %s", err)
			}
			if err := validateWasm(ctx, opts.kube, nodeName, privateKey, vmPrivateIP); err != nil {
				t.Fatalf("unable to validate wasm: %s", err)
			}
		}

		t.Logf("node is ready, proceeding with validation commands...")
		if err := runLiveVMValidators(ctx, t, vmssName, privateKey, vmPrivateIP, opts); err != nil {
			t.Fatalf("VM validation failed: %s", err)
		}

		t.Log("node bootstrapping succeeded!")
	}
}
