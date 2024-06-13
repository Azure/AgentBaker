package e2e

import (
	"context"
	"log"
	mrand "math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/barkimedes/go-deepcopy"
)

func Test_All(t *testing.T) {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	ctx := context.Background()
	t.Parallel()

	if err := createE2ELoggingDir(); err != nil {
		t.Fatal(err)
	}

	scenarios, err := scenario.GetScenariosForSuite(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) < 1 {
		t.Fatal("at least one scenario must be selected to run the e2e suite")
	}

	cloud, err := newAzureClient(config.Subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureResourceGroup(ctx, cloud); err != nil {
		t.Fatal(err)
	}

	clusterConfigs, err := getInitialClusterConfigs(ctx, cloud, config.ResourceGroupName)
	if err != nil {
		t.Fatal(err)
	}

	if err := createMissingClusters(ctx, r, cloud, scenarios, &clusterConfigs); err != nil {
		t.Fatal(err)
	}

	for _, e2eScenario := range scenarios {
		e2eScenario := e2eScenario

		clusterConfig, err := chooseCluster(ctx, r, cloud, e2eScenario, clusterConfigs)
		if err != nil {
			t.Fatal(err)
		}

		clusterName := *clusterConfig.cluster.Name
		log.Printf("chose cluster: %q", clusterName)

		baseNodeBootstrappingConfig, err := getBaseNodeBootstrappingConfiguration(clusterConfig.parameters)
		if err != nil {
			t.Fatal(err)
		}

		copied, err := deepcopy.Anything(baseNodeBootstrappingConfig)
		if err != nil {
			t.Error(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		e2eScenario.PrepareNodeBootstrappingConfiguration(nbc)

		t.Run(e2eScenario.Name, func(t *testing.T) {
			t.Parallel()

			loggingDir, err := createVMLogsDir(e2eScenario.Name)
			if err != nil {
				t.Fatal(err)
			}

			runScenario(ctx, t, &scenarioRunOpts{
				clusterConfig: clusterConfig,
				cloud:         cloud,
				scenario:      e2eScenario,
				nbc:           nbc,
				loggingDir:    loggingDir,
			})
		})
	}
}

func runScenario(ctx context.Context, t *testing.T, opts *scenarioRunOpts) {
	// need to create a new rand object for each goroutine since mrand.Rand is not thread-safe
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))

	privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair(r)
	if err != nil {
		t.Error(err)
		return
	}

	vmssName := getVmssName(r)
	log.Printf("creating and bootstrapping vmss: %q", vmssName)

	vmssSucceeded := true
	vmssModel, cleanupVMSS, err := bootstrapVMSS(ctx, t, r, vmssName, opts, publicKeyBytes)
	if !config.KeepVMSS && cleanupVMSS != nil {
		defer cleanupVMSS()
	}
	if err != nil {
		vmssSucceeded = false
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
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		log.Printf("vmss %s creation succeeded, proceeding with node readiness and pod checks...", vmssName)
		nodeName, err := validateNodeHealth(ctx, opts.clusterConfig.kube, vmssName)
		if err != nil {
			t.Fatal(err)
		}

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			log.Printf("wasm scenario: running wasm validation on %s...", vmssName)
			if err := ensureWasmRuntimeClasses(ctx, opts.clusterConfig.kube); err != nil {
				t.Fatalf("unable to ensure wasm RuntimeClasses on %s: %s", vmssName, err)
			}
			if err := validateWasm(ctx, opts.clusterConfig.kube, nodeName, string(privateKeyBytes)); err != nil {
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
