package e2e

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func ValidatePodRunning(ctx context.Context, s *Scenario) {
	testPod := func() *corev1.Pod {
		if s.VHD.OS == config.OSWindows {
			return podHTTPServerWindows(s)
		}
		return podHTTPServerLinux(s)
	}()
	ensurePod(ctx, s, testPod)
	s.T.Logf("node health validation: test pod %q is running on node %q", testPod.Name, s.Runtime.KubeNodeName)
}

func ValidateWASM(ctx context.Context, s *Scenario, nodeName string) {
	s.T.Logf("wasm scenario: running wasm validation on %s...", nodeName)
	spinClassName := fmt.Sprintf("wasmtime-%s", wasmHandlerSpin)
	err := createRuntimeClass(ctx, s.Runtime.Cluster.Kube, spinClassName, wasmHandlerSpin)
	require.NoError(s.T, err)
	err = ensureWasmRuntimeClasses(ctx, s.Runtime.Cluster.Kube)
	require.NoError(s.T, err)
	spinPodManifest := podWASMSpin(s)
	ensurePod(ctx, s, spinPodManifest)
	require.NoError(s.T, err, "unable to ensure wasm pod on node %q", nodeName)
}

func ValidateCommonLinux(ctx context.Context, s *Scenario) {
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, []string{"sudo cat /etc/default/kubelet"}, 0, "could not read kubelet config")
	stdout := execResult.stdout.String()
	require.NotContains(s.T, stdout, "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

	// the instructions belows expects the SSH key to be uploaded to the user pool VM.
	// which happens as a side-effect of execCommandOnVMForScenario, it's ugly but works.
	// maybe we should use a single ssh key per cluster, but need to be careful with parallel test runs.
	logSSHInstructions(s)

	ValidateSysctlConfig(ctx, s, map[string]string{
		"net.ipv4.tcp_retries2":             "8",
		"net.core.message_burst":            "80",
		"net.core.message_cost":             "40",
		"net.core.somaxconn":                "16384",
		"net.ipv4.tcp_max_syn_backlog":      "16384",
		"net.ipv4.neigh.default.gc_thresh1": "4096",
		"net.ipv4.neigh.default.gc_thresh2": "8192",
		"net.ipv4.neigh.default.gc_thresh3": "16384",
	})

	ValidateDirectoryContent(ctx, s, "/var/log/azure/aks", []string{
		"cluster-provision.log",
		"cluster-provision-cse-output.log",
		"cloud-init-files.paved",
		"vhd-install.complete",
		//"cloud-config.txt", // file with UserData
	})

	execResult = execScriptOnVMForScenarioValidateExitCode(ctx, s, []string{"sudo curl http://168.63.129.16:32526/vmSettings"}, 0, "curl to wireserver failed")

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl https://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4")
	require.Equal(s.T, "28", execResult.exitCode, "curl to wireserver should fail")

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4")
	require.Equal(s.T, "28", execResult.exitCode, "curl to wireserver port 32526 shouldn't succeed")

	ValidateLeakedSecrets(ctx, s)

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if s.VHD.Version != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg.Version {
		ValidateKubeletNodeIP(ctx, s)
	}
}

func ValidateLeakedSecrets(ctx context.Context, s *Scenario) {
	var secrets map[string]string
	b64Encoded := func(val string) string {
		return base64.StdEncoding.EncodeToString([]byte(val))
	}
	if s.Runtime.NBC != nil {
		secrets = map[string]string{
			"client private key":       b64Encoded(s.Runtime.NBC.ContainerService.Properties.CertificateProfile.ClientPrivateKey),
			"service principal secret": b64Encoded(s.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile.Secret),
			"bootstrap token":          *s.Runtime.NBC.KubeletClientTLSBootstrapToken,
		}
	} else {
		token := s.Runtime.AKSNodeConfig.BootstrappingConfig.TlsBootstrappingToken
		strToken := ""
		if token != nil {
			strToken = *token
		}
		secrets = map[string]string{
			"client private key":       b64Encoded(s.Runtime.AKSNodeConfig.KubeletConfig.KubeletClientKey),
			"service principal secret": b64Encoded(s.Runtime.AKSNodeConfig.AuthConfig.ServicePrincipalSecret),
			"bootstrap token":          strToken,
		}
	}

	for _, logFile := range []string{"/var/log/azure/cluster-provision.log", "/var/log/azure/aks-node-controller.log"} {
		for _, secretValue := range secrets {
			if secretValue != "" {
				ValidateFileExcludesContent(ctx, s, logFile, secretValue)
			}
		}
	}
}
