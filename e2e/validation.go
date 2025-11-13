package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidatePodRunning(ctx context.Context, s *Scenario, pod *corev1.Pod) {
	kube := s.Runtime.Cluster.Kube
	truncatePodName(s.T, pod)
	start := time.Now()

	s.T.Logf("creating pod %q", pod.Name)
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, v1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create pod %q", pod.Name)
	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, v1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil {
			s.T.Logf("couldn't not delete pod %s: %v", pod.Name, err)
		}
	})

	_, err = kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name)
	if err != nil {
		jsonString, jsonError := json.Marshal(pod)
		if jsonError != nil {
			jsonString = []byte(jsonError.Error())
		}
		require.NoErrorf(s.T, err, "failed to wait for pod %q to be in running state. Pod data: %s", pod.Name, jsonString)
	}

	timeForReady := time.Since(start)
	toolkit.LogDuration(ctx, timeForReady, time.Minute, fmt.Sprintf("Time for pod %q to get ready was %s", pod.Name, timeForReady))
	s.T.Logf("node health validation: test pod %q is running on node %q", pod.Name, s.Runtime.VM.KubeName)
}

func ValidateCommonLinux(ctx context.Context, s *Scenario) {
	ValidateTLSBootstrapping(ctx, s)
	ValidateKubeletServingCertificateRotation(ctx, s)
	ValidateSystemdWatchdogForKubernetes132Plus(ctx, s)
	ValidateSystemdUnitIsNotFailed(ctx, s, "aks-log-collector")
	ValidateLeakedSecrets(ctx, s)
	ValidateIPTablesCompatibleWithCiliumEBPF(ctx, s)

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
	})

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if !s.VHD.UnsupportedKubeletNodeIP {
		ValidateKubeletNodeIP(ctx, s)
	}

	// localdns is not supported on scriptless, privatekube and VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached.
	if s.Tags.Scriptless != true && s.VHD != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg && s.VHD != config.VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached {
		ValidateLocalDNSService(ctx, s, "enabled")
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
	}

	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	require.NotContains(s.T, execResult.stdout.String(), "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

	_ = execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo curl http://168.63.129.16:32526/vmSettings", 0, "curl to wireserver failed")

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl https://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4")
	if !assert.Equal(s.T, "28", execResult.exitCode, "curl to wireserver expected to fail, but it didn't") {
		// Log debug information. This validator seems to be flaky, hard to catch
		s.T.Logf("host IPTABLES: %s", execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers").String())
		s.T.FailNow()
	}

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4")
	if !assert.Equal(s.T, "28", execResult.exitCode, "curl to wireserver port 32526 expected to fail, but it didn't") {
		// Log debug information. This validator seems to be flaky, hard to catch
		s.T.Logf("host IPTABLES: %s", execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers").String())
		s.T.FailNow()
	}

	// base NBC templates define a mock service principal profile that we can still use to test
	// the correct bootstrapping logic: https://github.com/Azure/AgentBaker/blob/master/e2e/node_config.go#L438-L441
	if hasServicePrincipalData(s) {
		_ = execScriptOnVMForScenarioValidateExitCode(
			ctx,
			s,
			`sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientId')" && sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientSecret')"`,
			0,
			"AAD client ID and secret should be present in /etc/kubernetes/azure.json")
	}
}
