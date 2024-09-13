package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/config"
	"github.com/stretchr/testify/require"
)

func validateNodeHealth(ctx context.Context, t *testing.T, kube *Kubeclient, vmssName string) string {
	nodeName := waitUntilNodeReady(ctx, t, kube, vmssName)
	testPodName := fmt.Sprintf("test-pod-%s", nodeName)
	testPodManifest := getHTTPServerTemplate(testPodName, nodeName)
	err := ensurePod(ctx, t, defaultNamespace, kube, testPodName, testPodManifest)
	require.NoError(t, err, "failed to validate node health, unable to ensure test pod on node %q", nodeName)
	return nodeName
}

func validateWasm(ctx context.Context, t *testing.T, kube *Kubeclient, nodeName string) {
	t.Logf("wasm scenario: running wasm validation on %s...", nodeName)
	spinClassName := fmt.Sprintf("wasmtime-%s", wasmHandlerSpin)
	err := createRuntimeClass(ctx, kube, spinClassName, wasmHandlerSpin)
	require.NoError(t, err)
	err = ensureWasmRuntimeClasses(ctx, kube)
	require.NoError(t, err)
	spinPodName := fmt.Sprintf("wasm-spin-%s", nodeName)
	spinPodManifest := getWasmSpinPodTemplate(spinPodName, nodeName)
	err = ensurePod(ctx, t, defaultNamespace, kube, spinPodName, spinPodManifest)
	require.NoError(t, err, "unable to ensure wasm pod on node %q", nodeName)
}

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, privateIP, sshPrivateKey string, opts *scenarioRunOpts) error {
	hostPodName, err := getHostNetworkDebugPodName(ctx, opts.clusterConfig.Kube)
	if err != nil {
		return fmt.Errorf("while running live validator for node %s, unable to get debug pod name: %w", vmssName, err)
	}

	nonHostPodName, err := getPodNetworkDebugPodNameForVMSS(ctx, opts.clusterConfig.Kube, vmssName)
	if err != nil {
		return fmt.Errorf("while running live validator for node %s, unable to get non host debug pod name: %w", vmssName, err)
	}

	validators := commonLiveVMValidators(opts)
	if opts.scenario.LiveVMValidators != nil {
		validators = append(validators, opts.scenario.LiveVMValidators...)
	}

	for _, validator := range validators {
		t.Logf("running live VM validator on %s: %q", vmssName, validator.Description)

		var execResult *podExecResult
		var err error
		// Non Host Validators - meaning we want to execute checks through a pod which is NOT connected to host's network
		if validator.IsPodNetwork {
			execResult, err = execOnUnprivilegedPod(ctx, opts.clusterConfig.Kube, "default", nonHostPodName, validator.Command)
		} else {
			execResult, err = execOnVM(ctx, opts.clusterConfig.Kube, privateIP, hostPodName, sshPrivateKey, validator.Command, validator.IsShellBuiltIn)
		}
		if err != nil {
			return fmt.Errorf("unable to execute validator on node %s command %q: %w", vmssName, validator.Command, err)
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

func commonLiveVMValidators(opts *scenarioRunOpts) []*LiveVMValidator {
	validators := []*LiveVMValidator{
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
		// this check will run from host's network - we expect it to succeed
		{
			Description: "check that curl to wireserver succeeds from host's network",
			Command:     "curl http://168.63.129.16:32526/vmSettings",
			Asserter: func(code, stdout, stderr string) error {
				if code != "0" {
					return fmt.Errorf("validator command terminated with exit code %q but expected code 0 (succeeded)", code)
				}
				return nil
			},
		},
		// CURL goes to port 443 by default for HTTPS
		{
			Description: "check that curl to wireserver fails",
			Command:     "curl https://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4",
			Asserter: func(code, stdout, stderr string) error {
				if code != "28" {
					return fmt.Errorf("validator command terminated with exit code %q but expected code 28 (CURL timeout)", code)
				}
				return nil
			},
			IsPodNetwork: true,
		},
		{
			Description: "check that curl to wireserver port 32526 fails",
			Command:     "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4",
			Asserter: func(code, stdout, stderr string) error {
				if code != "28" {
					return fmt.Errorf("validator command terminated with exit code %q but expected code 28 (CURL timeout)", code)
				}
				return nil
			},
			IsPodNetwork: true,
		},
	}
	validators = append(validators, leakedSecretsValidators(opts)...)

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if opts.scenario.VHD.Version != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg.Version {
		validators = append(validators, kubeletNodeIPValidator())
	}

	return validators
}

func leakedSecretsValidators(opts *scenarioRunOpts) []*LiveVMValidator {
	logPath := "/var/log/azure/cluster-provision.log"
	clientPrivateKey := opts.nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey
	spSecret := opts.nbc.ContainerService.Properties.ServicePrincipalProfile.Secret
	bootstrapToken := *opts.nbc.KubeletClientTLSBootstrapToken

	b64Encoded := func(val string) string {
		return base64.StdEncoding.EncodeToString([]byte(val))
	}
	return []*LiveVMValidator{
		// Base64 encoded in baker.go (GetKubeletClientKey)
		FileExcludesContentsValidator(logPath, b64Encoded(clientPrivateKey), "client private key"),
		// Base64 encoded in baker.go (GetServicePrincipalSecret)
		FileExcludesContentsValidator(logPath, b64Encoded(spSecret), "service principal secret"),
		// Bootstrap token is already encoded so we don't need to
		// encode it again here.
		FileExcludesContentsValidator(logPath, bootstrapToken, "bootstrap token"),
	}
}
