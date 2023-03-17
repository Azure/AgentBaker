package e2e_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
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
		t.Log("error dumping cluster parameters")
		t.Error(err)
	}

	baseConfig, err := getBaseBootstrappingConfig(ctx, t, cloud, suiteConfig, clusterParams)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range cases {
		tc := tc
		copied, err := deepcopy.Anything(baseConfig)
		if err != nil {
			t.Error(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		if tc.bootstrapConfigMutator != nil {
			tc.bootstrapConfigMutator(t, nbc)
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			caseLogsDir, err := createVMLogsDir(name)
			if err != nil {
				t.Fatal(err)
			}

			baker := agent.InitializeTemplateGenerator()
			base64EncodedCustomData := baker.GetNodeBootstrappingPayload(nbc)
			cseCmd := baker.GetNodeBootstrappingCmd(nbc)

			vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
			t.Logf("vmss name: %q", vmssName)

			cleanupVMSS := func() {
				t.Log("deleting vmss", vmssName)
				poller, err := cloud.vmssClient.BeginDelete(ctx, agentbakerTestResourceGroupName, vmssName, nil)
				if err != nil {
					t.Log("error deleting vmss", vmssName)
					t.Error(err)
					return
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					t.Log("error polling deleting vmss", vmssName)
					t.Error(err)
				}
				t.Log("finished deleting vmss", vmssName)
			}

			defer cleanupVMSS()

			sshPrivateKeyBytes, err := createVMSSWithPayload(ctx, r, cloud, suiteConfig.location, vmssName, subnetID, base64EncodedCustomData, cseCmd, tc.vmConfigMutator)
			if err != nil {
				t.Error(err)
				return
			}

			debug := func() {
				t.Log(" extracting VM logs")
				logFiles, err := extractLogsFromVM(ctx, t, cloud, kube, suiteConfig.subscription, vmssName, string(sshPrivateKeyBytes))
				if err != nil {
					t.Log("error extracting VM logs")
					t.Error(err)
				}

				t.Logf("dumping VM logs to local directory: %s", caseLogsDir)
				if err = dumpFileMapToDir(caseLogsDir, logFiles); err != nil {
					t.Log("error dumping VM logs")
					t.Error(err)
				}
			}
			defer debug()

			nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
			if err != nil {
				t.Log("error waiting for node ready")
				t.Fatal(err)
				return
			}

			err = ensureTestNginxPod(ctx, kube, nodeName)
			if err != nil {
				t.Log("error waiting for pod ready")
				t.Fatal(err)
			}

			err = ensurePodDeleted(ctx, kube, nodeName)
			if err != nil {
				t.Log("error waiting for pod deleted")
				t.Error(err)
			}
		})
	}
}
