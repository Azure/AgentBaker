package e2e_test

import (
	"context"
	"fmt"
	"log"
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

	scenarios := scenario.InitScenarioTable(suiteConfig.scenariosToRun)

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureResourceGroup(ctx, cloud, suiteConfig.resourceGroupName); err != nil {
		t.Fatal(err)
	}

	clusterConfigs, err := getInitialClusterConfigs(ctx, cloud, suiteConfig.resourceGroupName)
	if err != nil {
		t.Fatal(err)
	}

	if err := createMissingClusters(ctx, r, cloud, suiteConfig, scenarios, &clusterConfigs); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		scenario := scenario

		clusterConfig, err := chooseCluster(ctx, r, cloud, suiteConfig, scenario, clusterConfigs)
		if err != nil {
			t.Fatal(err)
		}

		clusterName := *clusterConfig.cluster.Name
		log.Printf("chose cluster: %q", clusterName)

		baseConfig, err := getBaseNodeBootstrappingConfiguration(ctx, cloud, suiteConfig, clusterConfig.parameters)
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
				clusterConfig: clusterConfig,
				cloud:         cloud,
				suiteConfig:   suiteConfig,
				scenario:      scenario,
				nbc:           nbc,
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

	fmt.Println("lecimy tutaj private key:" + string(privateKeyBytes))
	vmssSucceeded := true
	vmssName, vmssModel, _, err := bootstrapVMSS(ctx, t, r, opts, publicKeyBytes)
	//if cleanupVMSS != nil {
	//	defer cleanupVMSS()
	//}
	if err != nil {
		vmssSucceeded = false
		if !isVMExtensionProvisioningError(err) {
			t.Fatal("Encountered an unknown error while creating VM:", err)
		}
		log.Println("VM was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...")
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

	// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
	defer func() {
		err := pollExtractVMLogs(ctx, vmssName, vmPrivateIP, privateKeyBytes, opts)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		log.Println("vmss creation succeded, proceeding with node readiness and pod checks...")
		nodeName, err := validateNodeHealth(ctx, opts.clusterConfig.kube, vmssName)
		if err != nil {
			t.Fatal(err)
		}

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			log.Println("wasm scenario: running wasm validation...")
			if err := ensureWasmRuntimeClasses(ctx, opts.clusterConfig.kube); err != nil {
				t.Fatalf("unable to ensure wasm RuntimeClasses: %s", err)
			}
			if err := validateWasm(ctx, opts.clusterConfig.kube, nodeName, string(privateKeyBytes)); err != nil {
				t.Fatalf("unable to validate wasm: %s", err)
			}
		}

		log.Println("node is ready, proceeding with validation commands...")

		err = runLiveVMValidators(ctx, vmssName, vmPrivateIP, string(privateKeyBytes), opts)
		if err != nil {
			t.Fatalf("VM validation failed: %s", err)
		}

		log.Println("node bootstrapping succeeded!")
	}
}
