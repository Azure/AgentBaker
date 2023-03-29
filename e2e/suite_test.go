package e2e_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/barkimedes/go-deepcopy"
	"k8s.io/apimachinery/pkg/util/wait"
)

func Test_All(t *testing.T) {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	ctx := context.Background()

	t.Parallel()

	suiteConfig, err := newSuiteConfig()
	if err != nil {
		t.Fatal(err)
	}

	if suiteConfig.testsToRun != nil {
		tests := []string{}
		for testName := range suiteConfig.testsToRun {
			if _, ok := cases[testName]; !ok {
				t.Fatalf("unrecognized E2E test case: %q", testName)
			} else {
				tests = append(tests, testName)
			}
		}
		t.Logf("Will run the following Agentbaker E2E tests: %s", strings.Join(tests, ", "))
	} else {
		t.Logf("Running all Agentbaker E2E tests...")
	}

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		t.Fatal(err)
	}

	if err := setupCluster(ctx, t, cloud, suiteConfig.location, suiteConfig.resourceGroupName, suiteConfig.clusterName); err != nil {
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

	var clusterParams map[string]string
	err = wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, t, kube)
		if err != nil {
			t.Logf("error extracting cluster parameters: %q", err)
			return false, nil
		}
		clusterParams = params
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := createClusterParamsDir(); err != nil {
		t.Fatal(err)
	}

	t.Logf("dumping cluster parameters to local directory: %s", clusterParamsDir)
	if err := dumpFileMapToDir(clusterParamsDir, clusterParams); err != nil {
		t.Error("error dumping cluster parameters", err)
	}

	baseConfig, err := getBaseBootstrappingConfig(ctx, t, cloud, suiteConfig, clusterParams)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range cases {
		if suiteConfig.testsToRun != nil && !suiteConfig.testsToRun[name] {
			continue
		}

		tc := tc
		caseName := name
		copied, err := deepcopy.Anything(baseConfig)
		if err != nil {
			t.Error(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		if tc.bootstrapConfigMutator != nil {
			tc.bootstrapConfigMutator(t, nbc)
		}

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			caseLogsDir, err := createVMLogsDir(caseName)
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
				poller, err := cloud.vmssClient.BeginDelete(ctx, agentbakerTestClusterMCResourceGroupName, vmssName, nil)
				if err != nil {
					t.Error("error deleting vmss", vmssName, err)
					return
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					t.Error("error polling deleting vmss", vmssName, err)
				}
				t.Log("finished deleting vmss", vmssName)
			}

			defer cleanupVMSS()

			privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair(r)
			if err != nil {
				t.Error(err)
				return
			}

			err = createVMSSWithPayload(ctx, publicKeyBytes, cloud, suiteConfig.location, vmssName, subnetID, base64EncodedCustomData, cseCmd, tc.vmConfigMutator)
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

			// Perform posthoc log extraction when the VMSS creation succeeded, or failed due to a CSE error
			if vmssSucceeded || isCSEError {
				debug := func() {
					err := wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
						t.Log("attempting to extract VM logs")

						logFiles, err := extractLogsFromVM(ctx, t, cloud, kube, suiteConfig.subscription, vmssName, string(privateKeyBytes))
						if err != nil {
							t.Logf("error extracting VM logs: %q", err)
							return false, nil
						}

						t.Logf("dumping VM logs to local directory: %s", caseLogsDir)
						if err = dumpFileMapToDir(caseLogsDir, logFiles); err != nil {
							t.Logf("error extracting VM logs: %q", err)
							return false, nil
						}

						return true, nil
					})
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
