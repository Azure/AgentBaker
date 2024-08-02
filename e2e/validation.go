package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validateNodeHealth(ctx context.Context, t *testing.T, kube *Kubeclient, vmssName string) string {
	nodeName := waitUntilNodeReady(ctx, t, kube, vmssName)
	nginxPodName := fmt.Sprintf("%s-nginx", nodeName)
	nginxPodManifest := getNginxPodTemplate(nodeName)
	err := ensurePod(ctx, t, defaultNamespace, kube, nginxPodName, nginxPodManifest)
	require.NoError(t, err, "failed to validate node health, unable to ensure nginx pod on node %q", nodeName)
	return nodeName
}

func validateWasm(ctx context.Context, t *testing.T, kube *Kubeclient, nodeName string) {
	t.Logf("wasm scenario: running wasm validation on %s...", nodeName)
	spinClassName := fmt.Sprintf("wasmtime-%s", wasmHandlerSpin)
	err := createRuntimeClass(ctx, kube, spinClassName, wasmHandlerSpin)
	require.NoError(t, err)
	err = ensureWasmRuntimeClasses(ctx, kube)
	require.NoError(t, err)
	spinPodName := fmt.Sprintf("%s-wasm-spin", nodeName)
	spinPodManifest := getWasmSpinPodTemplate(nodeName)
	err = ensurePod(ctx, t, defaultNamespace, kube, spinPodName, spinPodManifest)
	require.NoError(t, err, "unable to ensure wasm pod on node %q", nodeName)
}

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, privateIP, sshPrivateKey string, opts *scenarioRunOpts) error {
	podName, err := getDebugPodName(ctx, opts.clusterConfig.Kube)
	if err != nil {
		return fmt.Errorf("While running live validator for node %s, unable to get debug pod name: %w", vmssName, err)
	}

	validators := commonLiveVMValidators()
	if opts.scenario.LiveVMValidators != nil {
		validators = append(validators, opts.scenario.LiveVMValidators...)
	}
	/*
		We could figure out the expected versions by parsing the components.json file, but then we'd need logic
		to resolve the right version based on the distro. It seems easier to have the versions of containerd and runc hard coded here because:

		* Having additional code for parsing the files and resolving the version introduces risk of bugs in the test code.
		* Re-using the parser from prod introduces risk that there's a bug in the parser that we don't pick up as it impacts both test and prod code.
		* It's not much effort for a dev to change the hard coded values here and in components.json
	*/
	if opts.scenario.VHD.ExpectedContainerdVersion != "" {
		validators = append(validators, containerdVersionValidator(opts.scenario.VHD.ExpectedContainerdVersion))
	}
	if opts.scenario.VHD.ExpectedRunCVersion != "" {
		validators = append(validators, runcVersionValidator(opts.scenario.VHD.ExpectedRunCVersion))
	}

	for _, validator := range validators {
		desc := validator.Description
		command := validator.Command
		isShellBuiltIn := validator.IsShellBuiltIn
		t.Logf("running live VM validator on %s: %q", vmssName, desc)

		execResult, err := pollExecOnVM(ctx, t, opts.clusterConfig.Kube, privateIP, podName, sshPrivateKey, command, isShellBuiltIn)
		if err != nil {
			return fmt.Errorf("unable to execute validator on node %s command %q: %w", vmssName, command, err)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.exitCode, execResult.stdout.String(), execResult.stderr.String())
			if err != nil {
				execResult.dumpAll(t)
				return fmt.Errorf("failed validator on node %s assertion: %w", vmssName, err)
			}
		}
	}

	return nil
}

func commonLiveVMValidators() []*LiveVMValidator {
	return []*LiveVMValidator{
		{
			Description: "assert /etc/default/kubelet should not contain dynamic config dir flag",
			Command:     "cat /etc/default/kubelet",
			Asserter: func(code, stdout, stderr string) error {
				if code != "0" {
					return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
				}
				if strings.Contains(stdout, "--dynamic-config-dir") {
					return fmt.Errorf("/etc/default/kubelet should not contain kubelet flag '--dynamic-config-dir', but does")
				}
				return nil
			},
		},
		SysctlConfigValidator(
			map[string]string{
				"net.ipv4.tcp_retries2":             "8",
				"net.core.message_burst":            "80",
				"net.core.message_cost":             "40",
				"net.core.somaxconn":                "16384",
				"net.ipv4.tcp_max_syn_backlog":      "16384",
				"net.ipv4.neigh.default.gc_thresh1": "4096",
				"net.ipv4.neigh.default.gc_thresh2": "8192",
				"net.ipv4.neigh.default.gc_thresh3": "16384",
			},
		),
		DirectoryValidator(
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
