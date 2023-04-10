package e2e_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
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

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, sshPrivateKey string, opts *scenarioRunOpts) error {
	privateIP, err := getVMPrivateIPAddress(ctx, opts.cloud, opts.suiteConfig.subscription, *opts.chosenCluster.Properties.NodeResourceGroup, vmssName)
	if err != nil {
		return fmt.Errorf("unable to get private IP address of VM on VMSS %q: %s", vmssName, err)
	}

	podName, err := getDebugPodName(opts.kube)
	if err != nil {
		return fmt.Errorf("unable to get debug pod name: %s", err)
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
		if execResult != nil {
			checkStdErr(execResult.stderr, t)
		}
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %s", command, err)
		}

		if execResult.exitCode != "0" {
			return fmt.Errorf("validator command %q terminated with exit code %s", command, execResult.exitCode)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.stdout.String(), execResult.stderr.String())
			if err != nil {
				return fmt.Errorf("failed validator assertion: %s", err)
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
		{
			Description: "assert sysctls set by customdata",
			Command:     "sysctl -a",
			Asserter: func(stdout, stderr string) error {
				for _, sysctl := range sysctlsAlwaysSet {
					if !strings.Contains(stdout, sysctl) {
						return fmt.Errorf("expected to find sysctl %q set on the live VM, but was not", sysctl)
					}
				}
				return nil
			},
		},
	}
}
