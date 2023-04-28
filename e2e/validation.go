package e2e_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
)

func validateWasm(ctx context.Context, kube *kubeclient, nodeName, privateKey string) error {
	spinPodName, err := ensureWasmPods(ctx, kube, nodeName)
	if err != nil {
		return fmt.Errorf("failed to valiate wasm, unable to ensure wasm pods on node %q: %w", nodeName, err)
	}

	spinPodIP, err := getPodIP(ctx, kube, defaultNamespace, spinPodName)
	if err != nil {
		return fmt.Errorf("unable to get IP of wasm spin pod %q: %w", spinPodName, err)
	}

	debugPodName, err := getDebugPodName(kube)
	if err != nil {
		return fmt.Errorf("unable to get debug pod name to validate wasm: %w", err)
	}

	execResult, err := pollExecOnPod(ctx, kube, defaultNamespace, debugPodName, getWasmCurlCommand(fmt.Sprintf("http://%s/hello", spinPodIP)))
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

			execResult, err = pollExecOnPod(ctx, kube, defaultNamespace, debugPodName, getWasmCurlCommand(fmt.Sprintf("http://%s/hello", spinPodIP)))
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

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, privateIP, sshPrivateKey string, opts *scenarioRunOpts) error {
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

		execResult, err := pollExecOnVM(ctx, opts.kube, privateIP, podName, sshPrivateKey, command)
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
