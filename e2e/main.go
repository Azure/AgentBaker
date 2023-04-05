package main

import (
	"context"
	"fmt"
	"log"
	mrand "math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/barkimedes/go-deepcopy"
	"k8s.io/apimachinery/pkg/util/wait"
)

func main() {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	ctx := context.Background()

	suiteConfig, err := newSuiteConfig()
	if err != nil {
		log.Fatal(err)
	}

	if suiteConfig.testsToRun != nil {
		tests := []string{}
		for testName := range suiteConfig.testsToRun {
			if _, ok := cases[testName]; !ok {
				log.Fatalf("unrecognized E2E test case: %q", testName)
			} else {
				tests = append(tests, testName)
			}
		}
		log.Printf("Will run the following Agentbaker E2E tests: %s", strings.Join(tests, ", "))
		return
	} else {
		log.Printf("Running all Agentbaker E2E tests...")
		return
	}

	cloud, err := newAzureClient(suiteConfig.subscription)
	if err != nil {
		log.Fatal(err)
	}

	if err := setupCluster(ctx, cloud, suiteConfig.location, suiteConfig.resourceGroupName, suiteConfig.clusterName); err != nil {
		log.Fatal(err)
	}

	subnetID, err := getClusterSubnetID(ctx, cloud, suiteConfig.location, suiteConfig.resourceGroupName, suiteConfig.clusterName)
	if err != nil {
		log.Fatal(err)
	}

	kube, err := getClusterKubeClient(ctx, cloud, suiteConfig)
	if err != nil {
		log.Fatal(err)
	}

	if err := ensureDebugDaemonset(ctx, kube, suiteConfig.resourceGroupName, suiteConfig.clusterName); err != nil {
		log.Fatal(err)
	}

	var clusterParams map[string]string
	err = wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, kube)
		if err != nil {
			log.Printf("error extracting cluster parameters: %q", err)
			return false, nil
		}
		clusterParams = params
		return true, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := createE2ELoggingDir(); err != nil {
		log.Fatal(err)
	}

	baseConfig, err := getBaseBootstrappingConfig(ctx, cloud, suiteConfig, clusterParams)
	if err != nil {
		log.Fatal(err)
	}

	for name, tc := range cases {
		if suiteConfig.testsToRun != nil && !suiteConfig.testsToRun[name] {
			continue
		}

		tc := tc
		caseName := name
		copied, err := deepcopy.Anything(baseConfig)
		if err != nil {
			log.Fatal(err)
			continue
		}
		nbc := copied.(*datamodel.NodeBootstrappingConfiguration)

		if tc.bootstrapConfigMutator != nil {
			tc.bootstrapConfigMutator(nbc)
		}

		caseLogsDir, err := createVMLogsDir(caseName)
		if err != nil {
			log.Fatal(err)
		}

		ab, err := agent.NewAgentBaker()
		if err != nil {
			log.Fatal(err)
		}
		nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), nbc)
		if err != nil {
			log.Fatal(err)
		}
		base64EncodedCustomData := nodeBootstrapping.CustomData
		cseCmd := nodeBootstrapping.CSE

		vmssName := fmt.Sprintf("abtest%s", randomLowercaseString(r, 4))
		log.Printf("vmss name: %q", vmssName)
		return

		cleanupVMSS := func() {
			log.Printf("deleting vmss: %s", vmssName)
			poller, err := cloud.vmssClient.BeginDelete(ctx, agentbakerTestClusterMCResourceGroupName, vmssName, nil)
			if err != nil {
				log.Fatal("error deleting vmss", vmssName, err)
				return
			}
			_, err = poller.PollUntilDone(ctx, nil)
			if err != nil {
				log.Fatal("error polling deleting vmss", vmssName, err)
			}
			log.Printf("finished deleting vmss: %s", vmssName)
		}

		defer cleanupVMSS()

		privateKeyBytes, publicKeyBytes, err := getNewRSAKeyPair(r)
		if err != nil {
			log.Fatal(err)
			return
		}

		vmssModel, err := createVMSSWithPayload(ctx, publicKeyBytes, cloud, suiteConfig.location, vmssName, subnetID, base64EncodedCustomData, cseCmd, tc.vmConfigMutator)
		isCSEError := isVMExtensionProvisioningError(err)
		vmssSucceeded := true
		if err != nil {
			vmssSucceeded = false
			if isCSEError {
				log.Fatal("VM was unable to be provisioned due to a CSE error, will still atempt to extract provisioning logs...", err)
			} else {
				log.Fatal("Encountered an unknown error while creating VM", err)
			}
		}

		if err := writeToFile(filepath.Join(caseLogsDir, "vmssId.txt"), *vmssModel.ID); err != nil {
			log.Fatal("failed to write vmss resource ID to disk", err)
		}

		// Perform posthoc log extraction when the VMSS creation succeeded, or failed due to a CSE error
		if vmssSucceeded || isCSEError {
			debug := func() {
				err := wait.PollImmediateWithContext(ctx, 15*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
					log.Printf("attempting to extract VM logs")

					logFiles, err := extractLogsFromVM(ctx, cloud, kube, suiteConfig.subscription, vmssName, string(privateKeyBytes))
					if err != nil {
						log.Printf("error extracting VM logs: %q", err)
						return false, nil
					}

					log.Printf("dumping VM logs to local directory: %s", caseLogsDir)

					if err = dumpFileMapToDir(caseLogsDir, logFiles); err != nil {
						log.Printf("error extracting VM logs: %q", err)
						return false, nil
					}

					return true, nil
				})
				if err != nil {
					log.Fatal(err)
				}
			}
			defer debug()
		}

		// Only perform node readiness/pod-related checks when VMSS creation succeeded
		if vmssSucceeded {
			log.Printf("vmss creation succeded, proceeding with node readiness and pod checks...")

			nodeName, err := waitUntilNodeReady(ctx, kube, vmssName)
			if err != nil {
				log.Fatal("error waiting for node ready", err)
			}

			err = ensureTestNginxPod(ctx, kube, nodeName)
			if err != nil {
				log.Fatal("error waiting for pod ready", err)
			}

			err = waitUntilPodDeleted(ctx, kube, nodeName)
			if err != nil {
				log.Fatal("error waiting for pod deleted", err)
			}

			log.Printf("node bootstrapping succeeded!")
		}
	}

	fmt.Println("done")
}
