package e2e_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
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

	for _, scenario := range scenarioTable {
		scenario := scenario

		chosenCluster := chooseCompatibleCluster(scenario, clusters)
		chosenClusterExists := chosenCluster != nil
		if !chosenClusterExists {
			t.Logf("could not find test cluster able to run scenario %q, creating a new one...", scenario.Name)
			newCluster := getBaseClusterModel(
				fmt.Sprintf(testClusterNameTemplate, randomLowercaseString(r, 5)),
				suiteConfig.location,
			)
			chosenCluster = &newCluster
			scenario.ScenarioConfig.ClusterMutator(chosenCluster)
		}

		if err := ensureCluster(ctx, t, cloud, suiteConfig.location, suiteConfig.resourceGroupName, chosenCluster, !chosenClusterExists); err != nil {
			t.Fatal(err)
		}

		clusterName := *chosenCluster.Name
		t.Logf("chosen cluster name: %q", clusterName)

		subnetID, err := getClusterSubnetID(ctx, cloud, suiteConfig.location, *chosenCluster.Properties.NodeResourceGroup, clusterName)
		if err != nil {
			t.Fatal(err)
		}

		kube, err := getClusterKubeClient(ctx, cloud, suiteConfig.resourceGroupName, clusterName)
		if err != nil {
			t.Fatal(err)
		}

		if err := ensureDebugDaemonset(ctx, kube); err != nil {
			t.Fatal(err)
		}

		clusterParams, err := pollExtractClusterParameters(ctx, t, kube)
		if err != nil {
			t.Fatal(err)
		}

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

		if scenario.ScenarioConfig.BootstrapConfigMutator != nil {
			scenario.ScenarioConfig.BootstrapConfigMutator(t, nbc)
		}

		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()

			caseLogsDir, err := createVMLogsDir(scenario.Name)
			if err != nil {
				t.Fatal(err)
			}

			ab, err := agent.NewAgentBaker()
			if err != nil {
				t.Fatal(err)
			}
			nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), nbc)
			if err != nil {
				t.Fatal(err)
			}
			base64EncodedCustomData := nodeBootstrapping.CustomData
			cseCmd := nodeBootstrapping.CSE

			vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
			t.Logf("vmss name: %q", vmssName)

			cleanupVMSS := func() {
				t.Log("deleting vmss", vmssName)
				poller, err := cloud.vmssClient.BeginDelete(ctx, *chosenCluster.Properties.NodeResourceGroup, vmssName, nil)
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

			defer cleanupVMSS()

			privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair(r)
			if err != nil {
				t.Error(err)
				return
			}

			err = createVMSSWithPayload(ctx, publicKeyBytes, cloud, suiteConfig.location, *chosenCluster.Properties.NodeResourceGroup, vmssName, subnetID, base64EncodedCustomData, cseCmd, scenario.VMConfigMutator)
			isCSEError := isVMExtensionProvisioningError(err)
			vmssSucceeded := true
			if err != nil {
				vmssSucceeded = false
				if isCSEError {
					t.Error("VM was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...", err)
				} else {
					t.Fatal("Encountered an unknown error while creating VM", err)
				}
			}

			// Perform posthoc log extraction when the VMSS creation succeeded or failed due to a CSE error
			if vmssSucceeded || isCSEError {
				debug := func() {
					err := pollExtractVMLogs(ctx, t, cloud, kube, suiteConfig, *chosenCluster.Properties.NodeResourceGroup, vmssName, caseLogsDir, privateKeyBytes)
					if err != nil {
						t.Fatal(err)
					}
				}
				defer debug()
			}

			// Only perform node readiness/pod-related checks when VMSS creation succeeded
			if vmssSucceeded {
				t.Log("vmss creation succeded, proceeding with node readiness and pod checks...")

				nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
				if err != nil {
					t.Fatal("error waiting for node ready", err)
				}

				err = ensureTestNginxPod(ctx, kube, nodeName)
				if err != nil {
					t.Fatal("error waiting for pod ready", err)
				}

				err = waitUntilPodDeleted(ctx, kube, nodeName)
				if err != nil {
					t.Fatal("error waiting for pod deleted", err)
				}

				t.Log("node bootstrapping succeeded!")
			}
		})
	}
}
