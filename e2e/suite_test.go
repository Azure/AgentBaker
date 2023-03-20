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
	scenarioTable := scenario.InitScenarioTable()
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	ctx := context.Background()

	t.Parallel()

	suiteConfig, err := newSuiteConfig()
	if err != nil {
		t.Fatal(err)
	}

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := setupCluster(ctx, cloud, suiteConfig.location, suiteConfig.resourceGroupName, suiteConfig.clusterName); err != nil {
		t.Fatal(err)
	}

	subnetID, err := getClusterSubnetID(ctx, cloud, suiteConfig.location, suiteConfig.resourceGroupName, suiteConfig.clusterName)
	if err != nil {
		t.Fatal(err)
	}

	kube, err := getClusterKubeClient(ctx, cloud, suiteConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureDebugDaemonset(ctx, kube, suiteConfig.resourceGroupName, suiteConfig.clusterName); err != nil {
		t.Fatal(err)
	}

	clusterParams, err := extractClusterParameters(ctx, t, kube)
	if err != nil {
		t.Fatal(err)
	}

	if err := createClusterParamsDir(); err != nil {
		t.Fatal(err)
	}

	t.Logf("dumping cluster parameters to local directory: %s", clusterParamsDir)
	if err := dumpFileMapToDir(clusterParamsDir, clusterParams); err != nil {
		t.Log("error dumping cluster parameters:")
		t.Error(err)
	}

	baseConfig, err := getBaseBootstrappingConfig(ctx, t, cloud, suiteConfig, clusterParams)
	if err != nil {
		t.Fatal(err)
	}

	for name, scenario := range scenarioTable {
		scenario := scenario
		copied, err := deepcopy.Anything(baseConfig)
		if err != nil {
			t.Error(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		if scenario.ScenarioConfig.BootstrapConfigMutator != nil {
			scenario.ScenarioConfig.BootstrapConfigMutator(t, nbc)
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			t.Logf("Running scenario %q: %q", scenario.Name, scenario.Description)

			caseLogsDir, err := createVMLogsDir(scenario.Name)
			if err != nil {
				t.Fatal(err)
			}

			baker := agent.InitializeTemplateGenerator()
			base64EncodedCustomData := baker.GetNodeBootstrappingPayload(nbc)
			cseCmd := baker.GetNodeBootstrappingCmd(nbc)

			vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
			t.Logf("[scenario/%s] vmss name: %q", scenario.Name, vmssName)

			cleanupVMSS := func() {
				t.Logf("[scenario/%s] deleting vmss %q", scenario.Name, vmssName)
				poller, err := cloud.vmssClient.BeginDelete(ctx, agentbakerTestResourceGroupName, vmssName, nil)
				if err != nil {
					t.Logf("[scenario/%s] error deleting vmss %q", scenario.Name, vmssName)
					t.Error(err)
					return
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					t.Logf("[scenario/%s] error polling deleting vmss %q", scenario.Name, vmssName)
					t.Error(err)
				}
				t.Logf("[scenario/%s] finished deleting vmss %q", scenario.Name, vmssName)
			}

			defer cleanupVMSS()

			sshPrivateKeyBytes, err := createVMSSWithPayload(ctx, r, cloud, suiteConfig.location, vmssName, subnetID, base64EncodedCustomData, cseCmd, scenario.ScenarioConfig.VMConfigMutator)
			if err != nil {
				t.Error(err)
				return
			}

			debug := func() {
				t.Logf("[scenario/%s] extracting VM logs", scenario.Name)
				logFiles, err := extractLogsFromVM(ctx, t, cloud, kube, suiteConfig.subscription, vmssName, string(sshPrivateKeyBytes))
				if err != nil {
					t.Logf("[scenario/%s] error extracting VM logs:", scenario.Name)
					t.Error(err)
				}

				t.Logf("dumping VM logs to local directory: %s", caseLogsDir)
				if err = dumpFileMapToDir(caseLogsDir, logFiles); err != nil {
					t.Logf("[scenario/%s] error dumping VM logs:", scenario.Name)
					t.Error(err)
				}
			}
			defer debug()

			nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
			if err != nil {
				t.Logf("[scenario/%s] error waiting for node ready:", scenario.Name)
				t.Fatal(err)
				return
			}

			err = ensureTestNginxPod(ctx, kube, nodeName)
			if err != nil {
				t.Logf("[scenario/%s] error waiting for pod ready:", scenario.Name)
				t.Fatal(err)
			}

			err = ensurePodDeleted(ctx, kube, nodeName)
			if err != nil {
				t.Logf("[scenario/%s] error waiting for pod deleted:", scenario.Name)
				t.Error(err)
			}
		})
	}
}
