package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func (s *Scenario) validateNodeHealth(ctx context.Context) string {
	nodeName := waitUntilNodeReady(ctx, s.T, s.Runtime.Cluster.Kube, s.Runtime.VMSSName)
	testPodName := fmt.Sprintf("test-pod-%s", nodeName)
	testPodManifest := ""
	if s.VHD.Windows() {
		testPodManifest = getHTTPServerTemplateWindows(testPodName, nodeName)
	} else {
		testPodManifest = getHTTPServerTemplate(testPodName, nodeName, s.Tags.Airgap)
	}
	err := ensurePod(ctx, s.T, defaultNamespace, s.Runtime.Cluster.Kube, testPodName, testPodManifest)
	require.NoError(s.T, err, "failed to validate node health, unable to ensure test pod on node %q", nodeName)
	s.T.Logf("node health validation: test pod %q is running on node %q", testPodName, nodeName)
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

func runLiveVMValidators(ctx context.Context, t *testing.T, vmssName, privateIP, sshPrivateKey string, scenario *Scenario) error {
	// TODO: test something
	if scenario.VHD.Windows() {
		return nil
	}
	hostPodName, err := getHostNetworkDebugPodName(ctx, scenario.Runtime.Cluster.Kube, t)
	if err != nil {
		return fmt.Errorf("while running live validator for node %s, unable to get debug pod name: %w", vmssName, err)
	}

	nonHostPodName, err := getPodNetworkDebugPodNameForVMSS(ctx, scenario.Runtime.Cluster.Kube, vmssName, t)
	if err != nil {
		return fmt.Errorf("while running live validator for node %s, unable to get non host debug pod name: %w", vmssName, err)
	}

	validators := commonLiveVMValidators(scenario)
	if scenario.LiveVMValidators != nil {
		validators = append(validators, scenario.LiveVMValidators...)
	}

	for _, validator := range validators {
		t.Logf("running live VM validator on %s: %q", vmssName, validator.Description)

		var execResult *podExecResult
		var err error
		// Non Host Validators - meaning we want to execute checks through a pod which is NOT connected to host's network
		if validator.IsPodNetwork {
			execResult, err = execOnUnprivilegedPod(ctx, scenario.Runtime.Cluster.Kube, "default", nonHostPodName, validator.Command)
		} else {
			execResult, err = execOnVM(ctx, scenario.Runtime.Cluster.Kube, privateIP, hostPodName, sshPrivateKey, validator.Command, validator.IsShellBuiltIn)
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

func commonLiveVMValidators(scenario *Scenario) []*LiveVMValidator {
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
				//"cloud-config.txt", // file with UserData
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
	validators = append(validators, leakedSecretsValidators(scenario)...)

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if scenario.VHD.Version != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg.Version {
		validators = append(validators, kubeletNodeIPValidator())
	}

	return validators
}

func leakedSecretsValidators(scenario *Scenario) []*LiveVMValidator {
	var secrets map[string]string
	b64Encoded := func(val string) string {
		return base64.StdEncoding.EncodeToString([]byte(val))
	}
	if scenario.Runtime.NBC != nil {
		secrets = map[string]string{
			"client private key":       b64Encoded(scenario.Runtime.NBC.ContainerService.Properties.CertificateProfile.ClientPrivateKey),
			"service principal secret": b64Encoded(scenario.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile.Secret),
			"bootstrap token":          *scenario.Runtime.NBC.KubeletClientTLSBootstrapToken,
		}
	} else {
		token := scenario.Runtime.AKSNodeConfig.BootstrappingConfig.TlsBootstrappingToken
		strToken := ""
		if token != nil {
			strToken = *token
		}
		secrets = map[string]string{
			"client private key":       b64Encoded(scenario.Runtime.AKSNodeConfig.KubeletConfig.KubeletClientKey),
			"service principal secret": b64Encoded(scenario.Runtime.AKSNodeConfig.AuthConfig.ServicePrincipalSecret),
			"bootstrap token":          strToken,
		}
	}

	validators := make([]*LiveVMValidator, 0)
	for _, logFile := range []string{"/var/log/azure/cluster-provision.log", "/var/log/azure/aks-node-controller.log"} {
		for secretName, secretValue := range secrets {
			validators = append(validators, FileExcludesContentsValidator(logFile, secretValue, secretName))
		}
	}
	return validators
}
