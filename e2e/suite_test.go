package e2e_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
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
			scenario.Config.BootstrapConfigMutator(t, nbc)
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

	vmssModel, cleanupVMSS, err := bootstrapVMSS(ctx, t, r, publicKeyBytes, opts)
	defer cleanupVMSS()
	isCSEError := isVMExtensionProvisioningError(err)
	vmssSucceeded := true
	if err != nil {
		vmssSucceeded = false
		if !isCSEError {
			t.Fatal("Encountered an unknown error while creating VM:", err)
		}
		t.Log("VM was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...")
	}

	if err := writeToFile(filepath.Join(opts.loggingDir, "vmssId.txt"), *vmssModel.ID); err != nil {
		t.Fatal("failed to write vmss resource ID to disk", err)
	}

	// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
	if vmssSucceeded || isCSEError {
		debug := func() {
			err := pollExtractVMLogs(ctx, t, *vmssModel.Name, privateKeyBytes, opts)
			if err != nil {
				t.Fatal(err)
			}
		}
		defer debug()
	}

	// Only perform node readiness/pod-related checks when VMSS creation succeeded
	if vmssSucceeded {
		t.Log("vmss creation succeded, proceeding with node readiness and pod checks...")
		if err = validateNodeHealth(ctx, t, opts.kube, *vmssModel.Name); err != nil {
			t.Fatalf("node health validation vailed: %s", err)
		}

		t.Logf("node is ready, proceeding with validation commands...")

		err := runLiveVMValidators(ctx, t, *vmssModel.Name, string(privateKeyBytes), opts)
		if err != nil {
			t.Fatalf("VM validation failed: %s", err)
		}

		t.Log("node bootstrapping succeeded!")
	}
}

func bootstrapVMSS(ctx context.Context, t *testing.T, r *mrand.Rand, publicKeyBytes []byte, opts *scenarioRunOpts) (*armcompute.VirtualMachineScaleSet, func(), error) {
	nodeBootstrapping, err := getNodeBootstrapping(ctx, opts.nbc)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get node bootstrapping: %s", err)
	}

	vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
	t.Logf("vmss name: %q", vmssName)

	cleanupVMSS := func() {
		t.Log("deleting vmss", vmssName)
		poller, err := opts.cloud.vmssClient.BeginDelete(ctx, *opts.chosenCluster.Properties.NodeResourceGroup, vmssName, nil)
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
		return nil, nil, fmt.Errorf("unable to create VMSS with payload: %s", err)
	}

	return vmssModel, cleanupVMSS, nil
}

func getNodeBootstrapping(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return nil, err
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, nbc)
	if err != nil {
		return nil, err
	}
	return nodeBootstrapping, nil
}

func validateNodeHealth(ctx context.Context, t *testing.T, kube *kubeclient, vmssName string) error {
	nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
	if err != nil {
		return fmt.Errorf("error waiting for node ready: %s", err)
	}

	err = ensureTestNginxPod(ctx, kube, nodeName)
	if err != nil {
		return fmt.Errorf("error waiting for pod ready: %s", err)
	}

	err = waitUntilPodDeleted(ctx, kube, nodeName)
	if err != nil {
		return fmt.Errorf("error waiting pod deleted: %s", err)
	}

	return nil
}
