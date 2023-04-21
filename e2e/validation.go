package e2e_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
)

func validateNodeHealth(ctx context.Context, t *testing.T, kube *kubeclient, vmssName string) (string, error) {
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
		return "", fmt.Errorf("error waiting for pod deletion: %w", err)
	}

	return nodeName, nil
}

func validateWasm(ctx context.Context, kube *kubeclient, nodeName, privateKey, vmPrivateIP string) error {
	spinPodName, err := ensureWasmPods(ctx, kube, nodeName)
	if err != nil {
		return fmt.Errorf("failed to valiate wasm, unable to ensure wasm pods on node %q: %w", nodeName, err)
	}

	spinPodIP, err := getPodIP(ctx, kube, defaultNamespace, spinPodName)
	if err != nil {
		return fmt.Errorf("unable to get pod IP of wasm spin pod %q: %w", spinPodName, err)
	}

	debugPodName, err := getDebugPodName(kube)
	if err != nil {
		return fmt.Errorf("unable to get debug pod name to validate wasm: %w", err)
	}

	curlCmd := fmt.Sprintf("curl http://%s/hello", spinPodIP)

	execResult, err := pollExecOnPod(ctx, kube, defaultNamespace, debugPodName, curlCmd)
	if err != nil {
		return fmt.Errorf("unable to execute wasm validation command: %w", err)
	}

	if execResult.exitCode != "0" {
		execResult.dumpAll()
		return fmt.Errorf("wasm validation curl command %q terminated with exit code %s", curlCmd, execResult.exitCode)
	}

	if err := waitUntilPodDeleted(ctx, kube, spinPodName); err != nil {
		return fmt.Errorf("error waiting for wasm pod deletion: %w", err)
	}

	return nil
}

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, sshPrivateKey, vmPrivateIP string, opts *scenarioRunOpts) error {
	podName, err := getDebugPodName(opts.kube)
	if err != nil {
		return fmt.Errorf("unable to get debug pod name: %w", err)
	}

	validators := commonLiveVMValidators()
	if opts.scenario.LiveVMValidators != nil {
		validators = append(validators, opts.scenario.LiveVMValidators...)
	}

	for _, validator := range validators {
		desc := validator.Description
		command := validator.Command
		log.Printf("running live VM validator: %q", desc)

		execResult, err := pollExecOnVM(ctx, opts.kube, vmPrivateIP, podName, sshPrivateKey, command)
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %w", command, err)
		}

		if execResult.exitCode != "0" {
			execResult.dumpAll()
			return fmt.Errorf("validator command %q terminated with exit code %s", command, execResult.exitCode)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.stdout.String(), execResult.stderr.String())
			if err != nil {
				execResult.dumpAll()
				return fmt.Errorf("failed validator assertion: %w", err)
			}
		}
	}

	return nil
}

func commonLiveVMValidators() []*scenario.LiveVMValidator {
	return []*scenario.LiveVMValidator{
		{
			Description: "assert /etc/default/kubelet should not contain dynamic config dir flag",
			Command:     "cat /etc/default/kubelet",
			Asserter: func(stdout, stderr string) error {
				if strings.Contains(stdout, "--dynamic-config-dir") {
					return fmt.Errorf("/etc/default/kubelet should not contain kubelet flag '--dynamic-config-dir', but does")
				}
				return nil
			},
		},
		scenario.SysctlConfigValidator(
			map[string]int{
				"net.ipv4.tcp_retries2":             8,
				"net.core.message_burst":            80,
				"net.core.message_cost":             40,
				"net.core.somaxconn":                16384,
				"net.ipv4.tcp_max_syn_backlog":      16384,
				"net.ipv4.neigh.default.gc_thresh1": 4096,
				"net.ipv4.neigh.default.gc_thresh2": 8192,
				"net.ipv4.neigh.default.gc_thresh3": 16384,
			},
		),
		scenario.DirectoryValidator(
			"/var/log/azure/aks",
			[]string{
				"cluster-provision.log",
				"cluster-provision-cse-output.log",
				"cloud-init-files.paved",
				"vhd-install.complete",
				"cloud-config.txt",
			},
		),
	}
}
