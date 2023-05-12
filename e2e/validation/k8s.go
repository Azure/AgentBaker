package validation

import (
	"context"
	"fmt"

	"github.com/Azure/agentbakere2e/clients"
	"github.com/Azure/agentbakere2e/exec"
	"github.com/Azure/agentbakere2e/util"
)

func CommonK8sValidators() []*K8sValidator {
	return []*K8sValidator{
		NodeHealthValidator(),
	}
}

func NodeHealthValidator() *K8sValidator {
	return &K8sValidator{
		Description: "wait for node registration/readiness and run an nginx pod",
		ValidatorFn: func(ctx context.Context, kube *clients.KubeClient, executor *exec.RemoteCommandExecutor, validatorConfig K8sValidationConfig) error {
			if err := util.WaitUntilNodeReady(ctx, kube, validatorConfig.NodeName); err != nil {
				return fmt.Errorf("error waiting for node ready: %w", err)
			}

			nginxPodName, err := ensureTestNginxPod(ctx, kube, validatorConfig.Namespace, validatorConfig.NodeName)
			if err != nil {
				return fmt.Errorf("error waiting for nginx pod to be ready: %w", err)
			}

			if err = util.WaitUntilPodDeleted(ctx, kube, validatorConfig.Namespace, nginxPodName); err != nil {
				return fmt.Errorf("error waiting for nginx pod to be deleted: %w", err)
			}

			return nil
		},
	}
}

func WASMValidator() *K8sValidator {
	return &K8sValidator{
		Description: "deploy wasm pods and ensure apps are reachable from within pod network",
		ValidatorFn: func(ctx context.Context, kube *clients.KubeClient, executor *exec.RemoteCommandExecutor, validatorConfig K8sValidationConfig) error {
			spinPodName, err := ensureWasmPods(ctx, kube, validatorConfig.Namespace, validatorConfig.NodeName)
			if err != nil {
				return fmt.Errorf("failed to valiate wasm, unable to ensure wasm pods on node %q: %w", validatorConfig.NodeName, err)
			}

			spinPodIP, err := util.GetPodIP(ctx, kube, validatorConfig.Namespace, spinPodName)
			if err != nil {
				return fmt.Errorf("unable to get IP of wasm spin pod %q: %w", spinPodName, err)
			}

			curlCmd := []string{exec.CurlCommand(fmt.Sprintf("http://%s/hello", spinPodIP))}
			execResult, err := executor.OnPod(curlCmd)
			if err != nil {
				return fmt.Errorf("unable to execute wasm validation command: %w", err)
			}

			if execResult.ExitCode != "0" {
				// retry getting the pod IP + curling the hello endpoint if the original curl reports connection refused or a timeout
				// since the wasm spin pod usually restarts at least once after initial creation, giving it a new IP
				if execResult.ExitCode == "7" || execResult.ExitCode == "28" {
					spinPodIP, err = util.GetPodIP(ctx, kube, validatorConfig.Namespace, spinPodName)
					if err != nil {
						return fmt.Errorf("unable to get IP of wasm spin pod %q: %w", spinPodName, err)
					}

					curlCmd = []string{exec.CurlCommand(fmt.Sprintf("http://%s/hello", spinPodIP))}
					execResult, err = executor.OnPod(curlCmd)
					if err != nil {
						return fmt.Errorf("unable to execute wasm validation command on wasm pod %q at %s: %w", spinPodName, spinPodIP, err)
					}

					if execResult.ExitCode != "0" {
						execResult.DumpAll()
						return fmt.Errorf("curl wasm endpoint on pod %q at %s terminated with exit code %s", spinPodName, spinPodIP, execResult.ExitCode)
					}
				} else {
					execResult.DumpAll()
					return fmt.Errorf("curl wasm endpoint on pod %q at %s terminated with exit code %s", spinPodName, spinPodIP, execResult.ExitCode)
				}
			}

			if err := util.WaitUntilPodDeleted(ctx, kube, validatorConfig.Namespace, spinPodName); err != nil {
				return fmt.Errorf("error waiting for wasm pod deletion: %w", err)
			}

			return nil
		},
	}
}

func ensureTestNginxPod(ctx context.Context, kube *clients.KubeClient, namespace, nodeName string) (string, error) {
	nginxPodName := fmt.Sprintf("%s-nginx", nodeName)
	nginxPodManifest := util.GetNginxPodTemplate(nodeName)
	if err := util.EnsurePod(ctx, kube, namespace, nginxPodName, nginxPodManifest); err != nil {
		return "", fmt.Errorf("failed to ensure test nginx pod %q: %w", nginxPodName, err)
	}
	return nginxPodName, nil
}

func ensureWasmPods(ctx context.Context, kube *clients.KubeClient, namespace, nodeName string) (string, error) {
	spinPodName := fmt.Sprintf("%s-wasm-spin", nodeName)
	spinPodManifest := util.GetWasmSpinPodTemplate(nodeName)
	if err := util.EnsurePod(ctx, kube, namespace, spinPodName, spinPodManifest); err != nil {
		return "", fmt.Errorf("failed to ensure wasm spin pod %q: %w", spinPodName, err)
	}
	return spinPodName, nil
}
