package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/go-autorest/autorest/azure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Polling intervals
	createVMSSPollingInterval               = 15 * time.Second
	execOnVMPollInterval                    = 10 * time.Second
	execOnPodPollInterval                   = 10 * time.Second
	extractClusterParametersPollInterval    = 10 * time.Second
	extractVMLogsPollInterval               = 10 * time.Second
	getVMPrivateIPAddressPollInterval       = 5 * time.Second
	waitUntilPodRunningPollInterval         = 5 * time.Second
	waitUntilPodDeletedPollInterval         = 5 * time.Second
	waitUntilClusterNotCreatingPollInterval = 10 * time.Second

	// Polling timeouts
	createVMSSPollingTimeout               = 10 * time.Minute
	execOnVMPollingTimeout                 = 3 * time.Minute
	execOnPodPollingTimeout                = 2 * time.Minute
	extractClusterParametersPollingTimeout = 3 * time.Minute
	extractVMLogsPollingTimeout            = 5 * time.Minute
	getVMPrivateIPAddressPollingTimeout    = 1 * time.Minute
	waitUntilPodRunningPollingTimeout      = 3 * time.Minute
	waitUntilPodDeletedPollingTimeout      = 1 * time.Minute

	vmssRetryAttempts = 3
	vmssRetrySleep    = 5 * time.Second
)

func pollExecOnVM(ctx context.Context, kube *kubeclient, vmPrivateIP, jumpboxPodName string, sshPrivateKey, command string, isShellBuiltIn bool) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollImmediateWithContext(ctx, execOnVMPollInterval, execOnVMPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := execOnVM(ctx, kube, vmPrivateIP, jumpboxPodName, sshPrivateKey, command, isShellBuiltIn)
		if err != nil {
			log.Printf("unable to execute command on VM: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
			return false, nil
		}

		// this denotes a retriable SSH failure
		if res.exitCode == "255" {
			return false, nil
		}

		execResult = res
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return execResult, nil
}

func pollExecOnPod(ctx context.Context, kube *kubeclient, namespace, podName, command string) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollImmediateWithContext(ctx, execOnPodPollInterval, execOnPodPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := execOnPod(ctx, kube, namespace, podName, append(bashCommandArray(), command))
		if err != nil {
			log.Printf("unable to execute command on pod: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
			return false, nil
		}

		execResult = res
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return execResult, nil
}

// Wraps extractClusterParameters in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractClusterParameters(ctx context.Context, kube *kubeclient) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollImmediateWithContext(ctx, extractClusterParametersPollInterval, extractClusterParametersPollingTimeout, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, kube)
		if err != nil {
			log.Printf("error extracting cluster parameters: %s", err)
			return false, nil
		}
		clusterParams = params
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return clusterParams, nil
}

// Wraps exctracLogsFromVM and dumpFileMapToDir in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractVMLogs(ctx context.Context, vmssName, privateIP string, privateKeyBytes []byte, opts *scenarioRunOpts) error {
	err := wait.PollImmediateWithContext(ctx, extractVMLogsPollInterval, extractVMLogsPollingTimeout, func(ctx context.Context) (bool, error) {
		log.Println("attempting to extract VM logs")

		logFiles, err := extractLogsFromVM(ctx, vmssName, privateIP, string(privateKeyBytes), opts)
		if err != nil {
			log.Printf("error extracting VM logs: %q", err)
			return false, nil
		}

		log.Printf("dumping VM logs to local directory: %s", opts.loggingDir)
		if err = dumpFileMapToDir(opts.loggingDir, logFiles); err != nil {
			log.Printf("error extracting VM logs: %q", err)
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func pollGetVMPrivateIP(ctx context.Context, vmssName string, opts *scenarioRunOpts) (string, error) {
	var vmPrivateIP string
	err := wait.PollImmediateWithContext(ctx, getVMPrivateIPAddressPollInterval, getVMPrivateIPAddressPollingTimeout, func(ctx context.Context) (bool, error) {
		pip, err := getVMPrivateIPAddress(ctx, opts.cloud, opts.suiteConfig.Subscription, *opts.clusterConfig.cluster.Properties.NodeResourceGroup, vmssName)
		if err != nil {
			log.Printf("encountered an error while getting VM private IP address: %s", err)
			return false, nil
		}
		vmPrivateIP = pip
		return true, nil
	})

	if err != nil {
		return "", err
	}

	return vmPrivateIP, nil
}

func waitForClusterCreation(ctx context.Context, cloud *azureClient, resourceGroupName, clusterName string) (*armcontainerservice.ManagedCluster, error) {
	var cluster *armcontainerservice.ManagedCluster
	err := wait.PollInfiniteWithContext(ctx, waitUntilClusterNotCreatingPollInterval, func(ctx context.Context) (bool, error) {
		clusterResp, err := cloud.aksClient.Get(ctx, resourceGroupName, clusterName, nil)
		if err != nil {
			return false, err
		}

		if clusterResp.Properties == nil || clusterResp.Properties.ProvisioningState == nil {
			return false, fmt.Errorf("%q: nil cluster properties/provisioning state when waiting for non-\"Creating\" provisioning state", *cluster.Name)
		}

		if *clusterResp.Properties.ProvisioningState == "Creating" {
			return false, nil
		}

		cluster = &clusterResp.ManagedCluster
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error waiting for cluster %q to be non-\"Creating\" provisioning state: %w", clusterName, err)
	}

	return cluster, nil
}

func waitUntilNodeReady(ctx context.Context, kube *kubeclient, vmssName string) (string, error) {
	var nodeName string
	err := wait.PollImmediateWithContext(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		nodes, err := kube.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			if strings.HasPrefix(node.Name, vmssName) {
				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
						nodeName = node.Name
						return true, nil
					}
				}
			}
		}

		return false, nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to find or wait for node to be ready: %w", err)
	}

	return nodeName, nil
}

func waitUntilPodRunning(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodRunningPollInterval, waitUntilPodRunningPollingTimeout, func(ctx context.Context) (bool, error) {
		pod, err := kube.typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == corev1.PodPhase("Running"), nil
	})
}

func waitUntilPodDeleted(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodDeletedPollInterval, waitUntilPodDeletedPollingTimeout, func(ctx context.Context) (bool, error) {
		err := kube.typed.CoreV1().Pods(defaultNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return err == nil, err
	})
}

type Poller[T any] interface {
	PollUntilDone(ctx context.Context, options *runtime.PollUntilDoneOptions) (T, error)
}

func pollVMSSOperation[T any](ctx context.Context, vmssName string, pollerOpts *runtime.PollUntilDoneOptions, vmssOperation func() (Poller[T], error)) (*T, error) {
	var requestError azure.RequestError
	var err error
	var poller Poller[T]
	for i := 0; i < vmssRetryAttempts; i++ {
		poller, err = vmssOperation()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("unable to complete VMSS operation in allotted time of %s: %w", createVMSSPollingTimeout.String(), err)
			}
			if errors.As(err, &requestError) && requestError.ServiceError != nil {
				log.Printf("VMSS operation failed on attempt %d due to %v, retrying...", i+1, requestError)
				time.Sleep(vmssRetrySleep)
				continue
			}
			return nil, err
		}
		break
	}
	if err != nil {
		return nil, fmt.Errorf("error on VMSS operation, all retries failed %q: %v", vmssName, err)
	}

	var vmssResp T
	for i := 0; i < vmssRetryAttempts; i++ {
		vmssResp, err = poller.PollUntilDone(ctx, pollerOpts)
		if err != nil {
			if errors.As(err, &requestError) && requestError.ServiceError != nil {
				log.Printf("VMSS operation failed on attempt %d due to %v, retrying...", i+1, requestError)
				time.Sleep(vmssRetrySleep)
				continue
			}
			return nil, err
		}
		break
	}
	if err != nil {
		return nil, fmt.Errorf("error on VMSS operation, all retries failed %q: %v", vmssName, err)
	}
	return &vmssResp, nil
}
