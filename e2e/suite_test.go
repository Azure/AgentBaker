package e2e_test

import (
	"context"
	"flag"
	"fmt"
	mrand "math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/barkimedes/go-deepcopy"
)

var e2eMode string

func init() {
	flag.StringVar(&e2eMode, "e2eMode", "", "specify mode for e2e tests - 'coverage' or 'validation' - default: 'validation'")
}

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

	scenarios := scenario.InitScenarioTable(t, suiteConfig.scenariosToRun)

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureResourceGroup(ctx, t, cloud, suiteConfig.resourceGroupName); err != nil {
		t.Fatal(err)
	}

	clusterConfigs, err := getInitialClusterConfigs(ctx, t, cloud, suiteConfig.resourceGroupName)
	if err != nil {
		t.Fatal(err)
	}

	if err := createMissingClusters(ctx, t, r, cloud, suiteConfig, scenarios, &clusterConfigs); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		scenario := scenario

		chosenConfig := mustChooseCluster(ctx, t, r, cloud, suiteConfig, scenario, clusterConfigs)

		clusterName := *chosenConfig.cluster.Name
		t.Logf("chose cluster: %q", clusterName)

		baseConfig, err := getBaseNodeBootstrappingConfiguration(ctx, t, cloud, suiteConfig, chosenConfig.parameters)
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
				clusterConfig: chosenConfig,
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

	vmssName, vmssModel, cleanupVMSS, err := bootstrapVMSS(ctx, t, r, opts, publicKeyBytes)
	if cleanupVMSS != nil {
		defer cleanupVMSS()
	}
	isCSEError := isVMExtensionProvisioningError(err)
	vmssSucceeded := true
	if err != nil {
		vmssSucceeded = false
		if !isCSEError {
			t.Fatal("Encountered an unknown error while creating VM:", err)
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

	// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
	if vmssSucceeded || isCSEError {
		debug := func() {
			err := pollExtractVMLogs(ctx, t, vmssName, vmPrivateIP, privateKeyBytes, opts)
			if err != nil {
				t.Fatal(err)
			}
		}
		defer debug()
	}

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		t.Log("vmss creation succeded, proceeding with node readiness and pod checks...")
		nodeName, err := validateNodeHealth(ctx, t, opts.clusterConfig.kube, vmssName)
		if err != nil {
			t.Fatal(err)
		}

		if opts.nbc.AgentPoolProfile.WorkloadRuntime == datamodel.WasmWasi {
			t.Log("wasm scenario: running wasm validation...")
			if err := ensureWasmRuntimeClasses(ctx, opts.clusterConfig.kube); err != nil {
				t.Fatalf("unable to ensure wasm RuntimeClasses: %s", err)
			}
			if err := validateWasm(ctx, opts.clusterConfig.kube, nodeName, string(privateKeyBytes)); err != nil {
				t.Fatalf("unable to validate wasm: %s", err)
			}
		}

		t.Logf("node is ready, proceeding with validation commands...")

		err = runLiveVMValidators(ctx, t, *vmssModel.Name, vmPrivateIP, string(privateKeyBytes), opts)
		if err != nil {
			t.Fatalf("VM validation failed: %s", err)
		}

		t.Log("node bootstrapping succeeded!")
	}
}

func bootstrapVMSS(ctx context.Context, t *testing.T, r *mrand.Rand, opts *scenarioRunOpts, publicKeyBytes []byte) (string, *armcompute.VirtualMachineScaleSet, func(), error) {
	nodeBootstrappingFn := getNodeBootstrappingFn(e2eMode)
	nodeBootstrapping, err := nodeBootstrappingFn(ctx, opts.nbc)
	if err != nil {
		return "", nil, nil, fmt.Errorf("unable to get node bootstrapping: %w", err)
	}

	vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
	t.Logf("vmss name: %q", vmssName)

	cleanupVMSS := func() {
		t.Log("deleting vmss", vmssName)
		poller, err := opts.cloud.vmssClient.BeginDelete(ctx, *opts.clusterConfig.cluster.Properties.NodeResourceGroup, vmssName, nil)
		if err != nil {
			t.Error("error deleting vmss", vmssName, err)
			return
		}
		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			t.Error("error polling deleting vmss", vmssName, err)
		}
		t.Logf("finished deleting vmss %q", vmssName)
	}

	vmssModel, err := createVMSSWithPayload(ctx, nodeBootstrapping.CustomData, nodeBootstrapping.CSE, vmssName, publicKeyBytes, opts)
	if err != nil {
		return "", nil, nil, fmt.Errorf("unable to create VMSS with payload: %w", err)
	}

	return vmssName, vmssModel, cleanupVMSS, nil
}

func validateNodeHealth(ctx context.Context, t *testing.T, kube *kubeclient, vmssName string) (string, error) {
	nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
	if err != nil {
		return "", fmt.Errorf("error waiting for node ready: %w", err)
	}

	nginxPodName, err := ensureTestNginxPod(ctx, kube, nodeName)
	if err != nil {
		return "", fmt.Errorf("error waiting for pod ready: %w", err)
	}

	err = waitUntilPodDeleted(ctx, kube, nginxPodName)
	if err != nil {
		return "", fmt.Errorf("error waiting pod deleted: %w", err)
	}

	return nodeName, nil
}
