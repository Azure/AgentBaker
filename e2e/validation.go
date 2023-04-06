package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

var (
	sysctlsAlwaysSet = []string{
		"net.ipv4.tcp_retries2",
		"net.core.message_burst",
		"net.core.message_cost",
		"net.core.somaxconn",
		"net.ipv4.tcp_max_syn_backlog",
		"net.ipv4.neigh.default.gc_thresh1",
		"net.ipv4.neigh.default.gc_thresh2",
		"net.ipv4.neigh.default.gc_thresh3",
	}
)

type vmCommandOutputAsserterFn func(string, string) error

type liveVMValidator struct {
	name            string
	command         string
	outputAsserters []vmCommandOutputAsserterFn
}

func runVMValidationCommands(
	ctx context.Context,
	t *testing.T,
	cloud *azureClient,
	kube *kubeclient,
	subscription, mcResourceGroupName, vmssName, sshPrivateKey string,
	validators []*liveVMValidator) error {
	privateIP, err := getVMPrivateIPAddress(ctx, cloud, subscription, mcResourceGroupName, vmssName)
	if err != nil {
		return fmt.Errorf("unable to get private IP address of VM on VMSS %q: %s", vmssName, err)
	}

	podName, err := getDebugPodName(kube)
	if err != nil {
		return fmt.Errorf("unable to get debug pod name: %s", err)
	}

	for _, validator := range validators {
		name := validator.name
		command := validator.command
		t.Logf("running live VM validator %q, command: %q", name, command)

		execResult, err := execOnVM(ctx, kube, privateIP, podName, sshPrivateKey, command)
		if execResult != nil {
			checkStdErr(execResult.stderr, t)
		}
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %s", command, err)
		}

		if execResult.exitCode != "0" {
			return fmt.Errorf("validator command %q terminated with exit code %s", command, execResult.exitCode)
		}

		if validator.outputAsserters != nil {
			stdoutContent := execResult.stdout.String()
			stderrContent := execResult.stderr.String()
			for _, asserter := range validator.outputAsserters {
				err := asserter(stdoutContent, stderrContent)
				if err != nil {
					return fmt.Errorf("failed validator assertion: %s", err)
				}
			}
		}
	}

	return nil
}

func commonVMValidationCommands() []*liveVMValidator {
	return []*liveVMValidator{
		{
			name:    "assert /etc/default/kubelet should not contain dynamic config dir flag",
			command: "cat /etc/default/kubelet",
			outputAsserters: []vmCommandOutputAsserterFn{
				func(stdout, stderr string) error {
					if strings.Contains(stdout, "--dynamic-config-dir") {
						return fmt.Errorf("/etc/default/kubelet should not contain kubelet flag '--dynamic-config-dir', but does")
					}
					return nil
				},
			},
		},
		{
			name:    "assert sysctls set by customdata",
			command: "sysctl -a",
			outputAsserters: []vmCommandOutputAsserterFn{
				func(stdout, stderr string) error {
					for _, sysctl := range sysctlsAlwaysSet {
						if !strings.Contains(stdout, sysctl) {
							return fmt.Errorf("expected to find sysctl %q set on the live VM, but was not", sysctl)
						}
					}
					return nil
				},
			},
		},
	}
}
