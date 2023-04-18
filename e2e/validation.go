package e2e_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
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
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %s", command, err)
		}

		if execResult.exitCode != "0" {
			execResult.dumpAll()
			return fmt.Errorf("validator command %q terminated with exit code %s", command, execResult.exitCode)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.stdout.String(), execResult.stderr.String())
			if err != nil {
				execResult.dumpAll()
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
