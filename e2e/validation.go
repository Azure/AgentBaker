package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidatePodRunningWithRetry(ctx context.Context, s *Scenario, pod *corev1.Pod, maxRetries int) {
	var err error
	for i := range maxRetries {
		err = validatePodRunning(ctx, s, pod)
		if err != nil {
			time.Sleep(1 * time.Second)
			s.T.Logf("retrying pod %q validation (%d/%d)", pod.Name, i+1, maxRetries)
			continue
		}
		break
	}
	require.NoErrorf(s.T, err, "failed to validate pod running %q", pod.Name)
}

func ValidatePodRunning(ctx context.Context, s *Scenario, pod *corev1.Pod) {
	require.NoErrorf(s.T, validatePodRunning(ctx, s, pod), "failed to validate pod running %q", pod.Name)
}

func ValidateCommonLinux(ctx context.Context, s *Scenario) {
	ValidateTLSBootstrapping(ctx, s)
	ValidateKubeletServingCertificateRotation(ctx, s)
	ValidateSystemdWatchdogForKubernetes132Plus(ctx, s)
	ValidateAKSLogCollector(ctx, s)
	ValidateDiskQueueService(ctx, s)
	ValidateLeakedSecrets(ctx, s)
	ValidateIPTablesCompatibleWithCiliumEBPF(ctx, s)
	ValidateRxBufferDefault(ctx, s)
	ValidateKernelLogs(ctx, s)
	ValidateScriptlessCSECmd(ctx, s)

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

	// Debug diagnostics for Issue 9: run on Flatcar/ACL to confirm the theory
	// that systemd's preset mechanism is broken, preventing ignition-file-extract.service
	// from being enabled. This logs diagnostic info but does not fail the test.
	if s.VHD.Flatcar {
		DebugIgnitionPresetMechanism(ctx, s)
	}

	// localdns is not supported on scriptless, privatekube and VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached.
	if !s.VHD.UnsupportedLocalDns {
		ValidateLocalDNSService(ctx, s, "enabled")
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
	}

	ValidateInspektorGadget(ctx, s)

	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	require.NotContains(s.T, execResult.stdout, "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

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
	if s.HasServicePrincipalData() {
		_ = execScriptOnVMForScenarioValidateExitCode(
			ctx,
			s,
			`sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientId')" && sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientSecret')"`,
			0,
			"AAD client ID and secret should be present in /etc/kubernetes/azure.json")
	}

	// ensure that no unexpected systemd units are in a failed state
	ValidateNoFailedSystemdUnits(ctx, s)
}

func ValidateCommonWindows(ctx context.Context, s *Scenario) {
	ValidateTLSBootstrapping(ctx, s)
	ValidateKubeletServingCertificateRotation(ctx, s)
}

func validatePodRunning(ctx context.Context, s *Scenario, pod *corev1.Pod) error {
	kube := s.Runtime.Cluster.Kube
	truncatePodName(s.T, pod)
	start := time.Now()

	s.T.Logf("creating pod %q", pod.Name)
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, v1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod %q: %v", pod.Name, err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, v1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil {
			s.T.Logf("couldn't not delete pod %s: %v", pod.Name, err)
		}
	}()

	_, err = kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name)
	if err != nil {
		jsonString, jsonError := json.Marshal(pod)
		if jsonError != nil {
			jsonString = []byte(jsonError.Error())
		}
		return fmt.Errorf("failed to wait for pod %q to be in running state. Pod data: %s, Error: %v", pod.Name, jsonString, err)
	}

	timeForReady := time.Since(start)
	toolkit.LogDuration(ctx, timeForReady, time.Minute, fmt.Sprintf("Time for pod %q to get ready was %s", pod.Name, timeForReady))
	s.T.Logf("node health validation: test pod %q is running on node %q", pod.Name, s.Runtime.VM.KubeName)
	return nil
}

// Waits until the specified resource is available on the given node.
// Returns an error if the resource is not available within the specified timeout period.
func waitUntilResourceAvailable(ctx context.Context, s *Scenario, resourceName string) {
	s.T.Helper()
	nodeName := s.Runtime.VM.KubeName
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.T.Fatalf("context cancelled: %v", ctx.Err())
		case <-ticker.C:
			node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			require.NoError(s.T, err, "failed to get node %q", nodeName)

			if isResourceAvailable(node, resourceName) {
				s.T.Logf("resource %q is available", resourceName)
				return
			}
		}
	}
}

// Checks if the specified resource is available on the node.
func isResourceAvailable(node *corev1.Node, resourceName string) bool {
	for rn, quantity := range node.Status.Allocatable {
		if rn == corev1.ResourceName(resourceName) && quantity.Cmp(resource.MustParse("1")) >= 0 {
			return true
		}
	}
	return false
}

func dllLoadedWindows(ctx context.Context, s *Scenario, dllName string) bool {
	s.T.Helper()

	steps := []string{
		"$ErrorActionPreference = \"Continue\"",
		fmt.Sprintf("tasklist /m %s", dllName),
	}
	execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
	dllLoaded := strings.Contains(execResult.stdout, dllName)

	s.T.Logf("stdout: %s\nstderr: %s", execResult.stdout, execResult.stderr)
	return dllLoaded
}

// getIPTablesRulesCompatibleWithEBPFHostRouting returns the expected iptables patterns that are accounted for when EBPF host routing is enabled.
// If tests are failing due to unexpected iptables rules, it is because an iptables rule has been found, that was not accounted for in the implementation
// of the eBPF host routing feature in Cilium CNI. In eBPF host routing mode, iptables rules in the host network namespace are bypassed for pod
// traffic. So, any functionality that is built using iptables needs an equivalent non-iptables implementation that works in Cilium's eBPF host routing
// mode. For guidance on how this may be done, please contact acndp@microsoft.com (Azure Container Networking Dataplane team). Once the feature
// is supported in eBPF host routing mode, or is blocked from being enabled alongside eBPF host routing mode, you can update this list.
func getIPTablesRulesCompatibleWithEBPFHostRouting() (map[string][]string, []string) {
	tablePatterns := map[string][]string{
		"filter": {
			`-A FORWARD -d 168.63.129.16/32 -p tcp -m tcp --dport 32526 -j DROP`,
			`-A FORWARD -d 168.63.129.16/32 -p tcp -m tcp --dport 80 -j DROP`,
		},
		"mangle": {
			`-A FORWARD -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 80 -j DROP`,
			`-A FORWARD -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 32526 -j DROP`,
		},
		"nat": {
			`-A POSTROUTING -j SWIFT`,
			`-A SWIFT -s`,
			`-A POSTROUTING -j SWIFT-POSTROUTING`,
			`-A SWIFT-POSTROUTING -s`,
		},
		"raw": {
			`^-A (PREROUTING|OUTPUT) -d 169\.254\.10\.(10|11)\/32 -p (tcp|udp) -m comment --comment "localdns: skip conntrack" -m (tcp|udp) --dport 53 -j NOTRACK$`,
		},
		"security": {
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 53 -j ACCEPT`,
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m owner --uid-owner 0 -j ACCEPT`,
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m conntrack --ctstate INVALID,NEW -j DROP`,
		},
	}

	globalPatterns := []string{
		`^-N .*`,
		`^-P .*`,
		`^-A (KUBE-SERVICES|KUBE-EXTERNAL-SERVICES|KUBE-NODEPORTS|KUBE-POSTROUTING|KUBE-MARK-MASQ|KUBE-FORWARD|KUBE-PROXY-FIREWALL|KUBE-PROXY-CANARY|KUBE-FIREWALL|KUBE-MARK-DROP) .*`,
		`^-A (KUBE-SEP|KUBE-SVC)`,
		`^-A .* -j (KUBE-SEP|KUBE-SVC|KUBE-SERVICES|KUBE-EXTERNAL-SERVICES|KUBE-NODEPORTS|KUBE-POSTROUTING|KUBE-MARK-MASQ|KUBE-FORWARD|KUBE-PROXY-FIREWALL|KUBE-PROXY-CANARY|KUBE-FIREWALL|KUBE-MARK-DROP)`,
		`^-A IP-MASQ-AGENT`,
		`^-A .* -j IP-MASQ-AGENT`,
		`^.*--comment.*cilium:`,
		`^.*--comment.*cilium-feeder:`,
		`-A FORWARD ! -s (?:\d{1,3}\.){3}\d{1,3}/32 -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP`,
	}

	return tablePatterns, globalPatterns
}
