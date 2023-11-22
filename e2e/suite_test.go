package e2e_test

import (
	"context"
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

	log.Printf("suite config:\n%s", suiteConfig.String())

	if err := createE2ELoggingDir(); err != nil {
		t.Fatal(err)
	}

	scenarios := scenario.InitScenarioTable(suiteConfig.scenariosToRun, suiteConfig.scenariosToExclude)
	if len(scenarios) < 1 {
		t.Fatal("at least one scenario must be selected to run the e2e suite")
	}

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureResourceGroup(ctx, cloud, suiteConfig); err != nil {
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

	vmssName := getVmssName(r)
	log.Printf("vmss name: %q", vmssName)

	vmssSucceeded := true
	vmssModel, cleanupVMSS, err := bootstrapVMSS(ctx, t, r, vmssName, opts, publicKeyBytes)
	if !opts.suiteConfig.keepVMSS && cleanupVMSS != nil {
		defer cleanupVMSS()
	}
	if err != nil {
		vmssSucceeded = false
		if !isVMExtensionProvisioningError(err) {
			t.Fatalf("encountered an unknown error while creating VM: %s", err)
		}
		log.Println("vm was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...")
	}

	if vmssModel != nil {
		if err := writeToFile(filepath.Join(opts.loggingDir, "vmssId.txt"), *vmssModel.ID); err != nil {
			t.Fatalf("failed to write vmss resource ID to disk: %s", err)
		}
	} else {
		log.Printf("WARNING: bootstrapped vmss model was nil for %s", vmssName)
	}

	if opts.suiteConfig.keepVMSS {
		defer func() {
			log.Printf("vmss %q will be retained for debugging purposes, please make sure to manually delete it later", vmssName)
			if vmssModel != nil {
				log.Printf("retained vmss resource ID: %q", *vmssModel.ID)
			} else {
				log.Printf("WARNING: model of retained vmss %q is nil", vmssName)
			}
			if err := writeToFile(filepath.Join(opts.loggingDir, "sshkey"), string(privateKeyBytes)); err != nil {
				t.Fatalf("failed to write retained vmss %q private ssh key to disk: %s", vmssName, err)
			}
		}()
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
			t.Fatalf("vm validation failed: %s", err)
		}

		log.Println("node bootstrapping succeeded!")
	} else {
		t.Fatal("vmss was unable to be properly created and bootstrapped")
	}
}
