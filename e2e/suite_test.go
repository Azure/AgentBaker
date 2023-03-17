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
			baker := agent.InitializeTemplateGenerator()
			base64EncodedCustomData := baker.GetNodeBootstrappingPayload(nbc)
			cseCmd := baker.GetNodeBootstrappingCmd(nbc)

			vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))

			t.Logf("vmss name: %q", vmssName)

			cleanup := func() {
				poller, err := cloud.vmssClient.BeginDelete(ctx, suiteConfig.resourceGroupName, vmssName, nil)
				if err != nil {
					t.Error(err)
					return
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					t.Error(err)
				}
			}

			defer cleanup()

			sshPrivateKeyBytes, err := createVMSSWithPayload(ctx, r, cloud, suiteConfig.location, suiteConfig.resourceGroupName, vmssName, subnetID, base64EncodedCustomData, cseCmd, tc.vmConfigMutator)
			if err != nil {
				t.Error(err)
				return
			}

			debug := func() {
				_, err = extractLogsFromVM(ctx, t, cloud, kube, suiteConfig.subscription, suiteConfig.resourceGroupName, suiteConfig.clusterName, vmssName, string(sshPrivateKeyBytes))
				if err != nil {
					t.Error(err)
				}
			}
			defer debug()

			nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
			if err != nil {
				t.Fatal(err)
				return
			}

			err = ensureTestNginxpod(ctx, kube, nodeName)
			if err != nil {
				t.Fatal(err)
			}

			err = ensurePodDeleted(ctx, kube, nodeName)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
