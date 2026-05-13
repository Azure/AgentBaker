package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
	ValidateWaagentLog(ctx, s)
	ValidateScriptlessCSECmd(ctx, s)
	ValidateScriptlessNBCCSECmd(ctx, s)
	ValidateNodeExporter(ctx, s)

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

	// localdns validation is skipped for VHDs with UnsupportedLocalDns=true:
	// FIPS VHDs, older pinned VHDs (privatekube, network-isolated-k8s-not-cached), and AzureLinux OSGuard.
	// See e2e/config/vhd.go for the full list.
	if !s.VHD.UnsupportedLocalDns && !config.Config.TestPreProvision && !s.VHDCaching {
		ValidateLocalDNSService(ctx, s, "enabled")
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
		ValidateLocalDNSExporterMetrics(ctx, s)

		// Validate hosts plugin validators only if hosts plugin is explicitly enabled
		if s.IsHostsPluginEnabled() {
			// Guard: skip hosts plugin validation if the VHD doesn't have the required artifacts.
			// The Agentbaker E2E pipeline uses VHDs from main, which may not yet include
			// aks-localdns-hosts-setup artifacts until the PR merges. This mirrors the pattern
			// used by PR #7917 for the localdns-exporter feature.
			if !vhdHasHostsPluginArtifacts(ctx, s) {
				s.T.Logf("WARNING: VHD does not have aks-localdns-hosts-setup.service — skipping hosts plugin validation")
			} else {
				// Validate hosts file contains resolved IPs for critical FQDNs (IPs resolved dynamically).
				// CSE sets up the hosts file and enables the aks-localdns-hosts-setup timer, but population
				// is performed asynchronously by the timer/service rather than synchronously during provisioning.
				ValidateLocalDNSHostsFile(ctx, s, s.GetDefaultFQDNsForValidation())
				// Validate aks-localdns-hosts-setup service ran successfully and timer is active
				ValidateAKSLocalDNSHostsSetupService(ctx, s)
				// No restart needed: select_localdns_corefile() uses feature flag to select WITH_HOSTS corefile,
				// and CoreDNS's reload 5s hot-reloads the hosts file when it gets populated.
				// Validate hosts plugin serves responses with IPs matching /etc/localdns/hosts
				ValidateLocalDNSHostsPluginBypass(ctx, s)
				// Validate IPv6 entries in hosts file are served correctly by CoreDNS (skips if no IPv6 present)
				ValidateLocalDNSHostsPluginIPv6(ctx, s)
				// Validate localdns cold start with empty hosts file: restart → fallthrough → populate → reload
				ValidateLocalDNSHostsPluginColdStart(ctx, s)
			}
		}
	}

	ValidateInspektorGadget(ctx, s)

	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	require.NotContains(s.T, execResult.stdout, "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

	_ = execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo curl http://168.63.129.16:32526/vmSettings", 0, "curl to wireserver failed")

	validateWireServerBlocked(ctx, s)
	ValidateVulnerableKernelModulesDisabled(ctx, s)

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
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod %q: %v", pod.Name, err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
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

// validateWireServerBlocked checks that unprivileged pods cannot reach WireServer.
// Wireserver must never be reachable from pods — any successful connection is a
// security issue, not a transient condition to retry through.
//
// We accept two curl exit codes as evidence of a working block:
//
//	28 = operation timeout   (FORWARD DROP — packets silently dropped)
//	 7 = couldn't connect    (FORWARD REJECT — RST / ICMP unreachable)
//
// Any other exit code is suspicious and fails the test with full diagnostics:
//
//	  0  = wireserver reachable (security regression)
//	127  = curl missing from debug image (test would otherwise silently bypass)
//	2/3  = invalid curl args
//	  6  = DNS resolution issue (wireserver IP is literal — should not happen)
//
// We do retry transient kube-apiserver exec hiccups, but never on the curl
// result itself — a single observation of an unexpected exit code is enough
// to fail loudly.
func validateWireServerBlocked(ctx context.Context, s *Scenario) {
	defer toolkit.LogStep(s.T, "validating wireserver is blocked from unprivileged pods")()

	nonHostPod, err := s.Runtime.Cluster.Kube.GetPodNetworkDebugPodForNode(ctx, s.Runtime.VM.KubeName)
	require.NoError(s.T, err, "failed to get non host debug pod for wireserver validation")

	type wireServerCheck struct {
		cmd  string
		desc string
	}

	checks := []wireServerCheck{
		{
			cmd:  "curl http://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4",
			desc: "wireserver port 80 goalstate",
		},
		{
			cmd:  "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4",
			desc: "wireserver port 32526 vmSettings",
		},
	}

	allowedExitCodes := map[string]bool{"28": true, "7": true}

	for _, check := range checks {
		var execResult *podExecResult
		pollErr := wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			r, execErr := execOnUnprivilegedPod(ctx, s.Runtime.Cluster.Kube, nonHostPod.Namespace, nonHostPod.Name, check.cmd)
			if execErr != nil {
				s.T.Logf("wireserver check %q: exec error (retrying): %v", check.desc, execErr)
				return false, nil
			}
			execResult = r
			return true, nil
		})
		require.NoErrorf(s.T, pollErr, "wireserver check %q: exec failed after retries", check.desc)

		if allowedExitCodes[execResult.exitCode] {
			continue
		}

		iptablesFwd := execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers").String()
		iptablesKubeFwd := execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L KUBE-FORWARD -v -n --line-numbers 2>/dev/null || echo 'chain not found'").String()
		iptablesSave := execScriptOnVMForScenario(ctx, s, "sudo iptables-save -t filter 2>/dev/null | head -80").String()
		conntrack := execScriptOnVMForScenario(ctx, s, "sudo conntrack -L -d 168.63.129.16 2>/dev/null || echo 'conntrack not available'").String()
		s.T.Fatalf("wireserver check %q: unexpected curl exit code %q (want 28 timeout or 7 refused)\n"+
			"stdout=%q, stderr=%q\n"+
			"FORWARD chain:\n%s\n"+
			"KUBE-FORWARD chain:\n%s\n"+
			"iptables-save filter:\n%s\n"+
			"conntrack:\n%s",
			check.desc, execResult.exitCode, execResult.stdout, execResult.stderr,
			iptablesFwd, iptablesKubeFwd, iptablesSave, conntrack)
	}
}

// vhdHasHostsPluginArtifacts checks if the VHD has aks-localdns-hosts-setup.service installed
// by running a file existence check on the VM. Returns false if the service file is absent,
// meaning the VHD predates the hosts plugin feature and validators should be skipped.
func vhdHasHostsPluginArtifacts(ctx context.Context, s *Scenario) bool {
	result := execScriptOnVMForScenario(ctx, s, "test -f /etc/systemd/system/aks-localdns-hosts-setup.service")
	return result.exitCode == "0"
}
