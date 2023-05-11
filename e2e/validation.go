package e2e_test

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/validation"
)

func validateNodeHealth(ctx context.Context, kube *kubeclient, vmssName string) (string, error) {
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

func validateWasm(ctx context.Context, nodeName string, kube *kubeclient, executor remoteCommandExecutor) error {
	spinPodName, err := ensureWasmPods(ctx, kube, nodeName)
	if err != nil {
		return fmt.Errorf("failed to valiate wasm, unable to ensure wasm pods on node %q: %w", nodeName, err)
	}

	spinPodIP, err := getPodIP(ctx, kube, defaultNamespace, spinPodName)
	if err != nil {
		return fmt.Errorf("unable to get IP of wasm spin pod %q: %w", spinPodName, err)
	}

	execResult, err := executor.onPod(curlCommand(fmt.Sprintf("http://%s/hello", spinPodIP)))
	if err != nil {
		return fmt.Errorf("unable to execute wasm validation command: %w", err)
	}

	if execResult.exitCode != "0" {
		// retry getting the pod IP + curling the hello endpoint if the original curl reports connection refused or a timeout
		// since the wasm spin pod usually restarts at least once after initial creation, giving it a new IP
		if execResult.exitCode == "7" || execResult.exitCode == "28" {
			spinPodIP, err = getPodIP(ctx, kube, defaultNamespace, spinPodName)
			if err != nil {
				return fmt.Errorf("unable to get IP of wasm spin pod %q: %w", spinPodName, err)
			}

			execResult, err = executor.onPod(curlCommand(fmt.Sprintf("http://%s/hello", spinPodIP)))
			if err != nil {
				return fmt.Errorf("unable to execute wasm validation command on wasm pod %q at %s: %w", spinPodName, spinPodIP, err)
			}

			if execResult.exitCode != "0" {
				execResult.dumpAll()
				return fmt.Errorf("curl wasm endpoint on pod %q at %s terminated with exit code %s", spinPodName, spinPodIP, execResult.exitCode)
			}
		} else {
			execResult.dumpAll()
			return fmt.Errorf("curl wasm endpoint on pod %q at %s terminated with exit code %s", spinPodName, spinPodIP, execResult.exitCode)
		}
	}

	if err := waitUntilPodDeleted(ctx, kube, spinPodName); err != nil {
		return fmt.Errorf("error waiting for wasm pod deletion: %w", err)
	}

	return nil
}

func runLiveVMValidators(ctx context.Context, vmssName string, executor remoteCommandExecutor, opts *scenarioRunOpts) error {
	validators := validation.CommonLiveVMValidators()
	if opts.scenario.LiveVMValidators != nil {
		validators = append(validators, opts.scenario.LiveVMValidators...)
	}

	for _, validator := range validators {
		desc := validator.Description
		command := validator.Command
		log.Printf("running live VM validator: %q", desc)

		execResult, err := executor.onVM(command)
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %w", command, err)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.exitCode, execResult.stdout.String(), execResult.stderr.String())
			if err != nil {
				execResult.dumpAll()
				return fmt.Errorf("failed validator assertion: %w", err)
			}
		}
	}

	return nil
}
