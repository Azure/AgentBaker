package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ValidatePodRunningWithRetry(ctx context.Context, s *Scenario, pod *corev1.Pod, maxRetries int) error {
	i := 1
	err := startPodAndCheckItRuns(ctx, s, pod)

	for i <= maxRetries && err != nil {
		retryBackoff := time.Duration(1 << uint(i))
		s.Logger.Logf("sleeping %d seconds before retrying pod %q", retryBackoff, pod.Name)
		time.Sleep(retryBackoff * time.Second)
		s.Logger.Logf("retrying pod %q validation (%d/%d)", pod.Name, i+1, maxRetries)

		i++
		err = startPodAndCheckItRuns(ctx, s, pod)
	}
	if err != nil {
		return fmt.Errorf("failed to validate pod running %q: %w", pod.Name, err)
	}
	return nil
}

func ValidatePodRunning(ctx context.Context, s *Scenario, pod *corev1.Pod) error {
	if err := startPodAndCheckItRuns(ctx, s, pod); err != nil {
		return fmt.Errorf("failed to validate pod running %q: %w", pod.Name, err)
	}
	return nil
}

func ValidateCommonLinux(ctx context.Context, s *Scenario) error {
	defer toolkit.LogStep(s.Logger, "running common Linux validation")()

	parallelErr := runValidators(ctx, s,
		ValidateTLSBootstrapping,
		ValidateKubeletServingCertificateRotation,
		ValidateSystemdWatchdogForKubernetes132Plus,
		ValidateAKSLogCollector,
		ValidateDiskQueueService,
		ValidateLeakedSecrets,
		ValidateKubeletActiveFlagsEvent,
		ValidateIPTablesCompatibleWithCiliumEBPF,
		ValidateRxBufferDefault,
		ValidateKernelLogs,
		ValidateWaagentLog,
		ValidateScriptlessCSECmd,
		ValidateScriptlessNBCCSECmd,
		ValidateScriptlessPhase3,
		ValidateNodeExporter,
		ValidateCommonSysctlConfig,
		ValidateAKSLogDirectory,
		ValidateKubeletNodeIPIfSupported,
		ValidateInspektorGadget,
		ValidateKubeletDynamicConfigDisabled,
		ValidateWireServerReachable,
		ValidateWireServerBlocked,
		ValidateStaleCachedKubeBinariesRemoved,
		ValidateServicePrincipalData,
	)

	return errors.Join(
		parallelErr,
		runValidator(ctx, s, ValidateMANAIfPresent),
		runValidator(ctx, s, ValidateCommonLocalDNS),
		runValidator(ctx, s, ValidateVulnerableKernelModulesDisabled),
		runValidator(ctx, s, ValidateNoFailedSystemdUnits),
	)
}

func ValidateCommonWindows(ctx context.Context, s *Scenario) error {
	defer toolkit.LogStep(s.Logger, "running common Windows validation")()

	return runValidators(ctx, s,
		ValidateTLSBootstrapping,
		ValidateKubeletServingCertificateRotation,
	)
}

func ValidateMANAIfPresent(ctx context.Context, s *Scenario) error {
	hasMANA, err := hasMANAHardware(ctx, s)
	if err != nil {
		return fmt.Errorf("failed to detect MANA hardware: %w", err)
	}
	if !hasMANA {
		return nil
	}
	return ValidateMANA(ctx, s)
}

func ValidateCommonSysctlConfig(ctx context.Context, s *Scenario) error {
	return ValidateSysctlConfig(ctx, s, map[string]string{
		"net.ipv4.tcp_retries2":             "8",
		"net.core.message_burst":            "80",
		"net.core.message_cost":             "40",
		"net.core.somaxconn":                "16384",
		"net.ipv4.tcp_max_syn_backlog":      "16384",
		"net.ipv4.neigh.default.gc_thresh1": "4096",
		"net.ipv4.neigh.default.gc_thresh2": "8192",
		"net.ipv4.neigh.default.gc_thresh3": "16384",
	})
}

func ValidateAKSLogDirectory(ctx context.Context, s *Scenario) error {
	return ValidateDirectoryContent(ctx, s, "/var/log/azure/aks", []string{
		"cluster-provision.log",
		"cluster-provision-cse-output.log",
		"cloud-init-files.paved",
		"vhd-install.complete",
	})
}

func ValidateKubeletNodeIPIfSupported(ctx context.Context, s *Scenario) error {
	if s.VHD.UnsupportedKubeletNodeIP {
		return nil
	}
	return ValidateKubeletNodeIP(ctx, s)
}

func ValidateCommonLocalDNS(ctx context.Context, s *Scenario) error {
	if s.VHD.UnsupportedLocalDns || config.Config.TestPreProvision || s.VHDCaching {
		return nil
	}
	errs := []error{
		ValidateLocalDNSService(ctx, s, "enabled"),
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10"),
		ValidateLocalDNSExporterMetrics(ctx, s),
	}
	if !s.IsHostsPluginEnabled() {
		return errors.Join(errs...)
	}
	hasArtifacts, err := vhdHasHostsPluginArtifacts(ctx, s)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("failed to detect hosts plugin artifacts on the VHD: %w", err))...)
	}
	if !hasArtifacts {
		s.Logger.Logf("WARNING: VHD does not have aks-localdns-hosts-setup.service — skipping hosts plugin validation")
		return errors.Join(errs...)
	}
	errs = append(errs,
		ValidateLocalDNSHostsFile(ctx, s, s.GetDefaultFQDNsForValidation()),
		ValidateAKSLocalDNSHostsSetupService(ctx, s),
		ValidateLocalDNSHostsPluginBypass(ctx, s),
		ValidateLocalDNSHostsPluginIPv6(ctx, s),
		ValidateLocalDNSHostsPluginColdStart(ctx, s),
	)
	return errors.Join(errs...)
}

func ValidateKubeletDynamicConfigDisabled(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	if err != nil {
		return err
	}
	return assert.NotContains(result.stdout, "--dynamic-config-dir",
		"kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s", result.stdout)
}

func ValidateWireServerReachable(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo curl http://168.63.129.16:32526/vmSettings", 0, "curl to wireserver failed")
	return err
}

func ValidateServicePrincipalData(ctx context.Context, s *Scenario) error {
	if !s.HasServicePrincipalData() {
		return nil
	}
	_, err := execScriptOnVMForScenarioValidateExitCode(
		ctx, s,
		`sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientId')" && sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientSecret')"`,
		0, "AAD client ID and secret should be present in /etc/kubernetes/azure.json")
	return err
}

func startPodAndCheckItRuns(ctx context.Context, s *Scenario, pod *corev1.Pod) error {
	kube := s.Runtime.Kube
	pod = pod.DeepCopy()
	pod.Name = uniqueKubernetesResourceName(pod.Name)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return err
	}
	start := time.Now()

	s.Logger.Logf("creating pod %q", pod.Name)
	created, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod %q: %v", pod.Name, err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))}
		err := kube.Typed.CoreV1().Pods(created.Namespace).Delete(ctx, created.Name, deleteOptions)
		if err != nil && !apierrors.IsNotFound(err) {
			s.Logger.Logf("could not delete pod %s: %v", created.Name, err)
		}
	}()

	_, err = kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name)
	if err != nil {
		jsonString, jsonError := json.Marshal(pod)
		if jsonError != nil {
			jsonString = []byte(jsonError.Error())
		}
		return fmt.Errorf("failed to wait for pod %q to be in running state. Pod data: %s, Error: %v", pod.Name, jsonString, err)
	}

	timeForReady := time.Since(start)
	toolkit.LogDuration(ctx, timeForReady, time.Minute, fmt.Sprintf("Time for pod %q to get ready was %s", pod.Name, timeForReady))
	s.Logger.Logf("node health validation: test pod %q is running on node %q", pod.Name, s.Runtime.VM.KubeName)
	return nil
}

// Waits until the specified resource is available on the given node.
// Returns an error if the resource is not available within the specified timeout period.
func waitUntilResourceAvailable(ctx context.Context, s *Scenario, resourceName string) error {
	nodeName := s.Runtime.VM.KubeName
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for resource %q on node %q: %w", resourceName, nodeName, ctx.Err())
		case <-ticker.C:
			node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get node %q: %w", nodeName, err)
			}

			if isResourceAvailable(node, resourceName) {
				s.Logger.Logf("resource %q is available", resourceName)
				return nil
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

func dllLoadedWindows(ctx context.Context, s *Scenario, dllName string) (bool, error) {
	steps := []string{
		"$ErrorActionPreference = \"Continue\"",
		fmt.Sprintf("tasklist /m %s", dllName),
	}
	execResult, err := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
	if err != nil {
		return false, fmt.Errorf("failed to list tasks loading DLL %q: %w", dllName, err)
	}
	dllLoaded := strings.Contains(execResult.stdout, dllName)

	s.Logger.Logf("stdout: %s\nstderr: %s", execResult.stdout, execResult.stderr)
	return dllLoaded, nil
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
			`-A INPUT -p udp --dport 68 -j ACCEPT`,
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
		`^-A CILIUM_\S+ `,
		`-A FORWARD ! -s (?:\d{1,3}\.){3}\d{1,3}/32 -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP`,
	}

	return tablePatterns, globalPatterns
}

// ValidateWireServerBlocked checks that unprivileged pods cannot reach WireServer.
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
func ValidateWireServerBlocked(ctx context.Context, s *Scenario) error {
	defer toolkit.LogStep(s.Logger, "validating wireserver is blocked from unprivileged pods")()

	nonHostPod, err := s.Runtime.Kube.GetPodNetworkDebugPodForNode(ctx, s.Runtime.VM.KubeName)
	if err != nil {
		return fmt.Errorf("failed to get non host debug pod for wireserver validation: %w", err)
	}

	type wireServerCheck struct {
		cmd  string
		desc string
	}

	checks := []wireServerCheck{
		{
			cmd:  "curl http://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4 --max-time 8",
			desc: "wireserver port 80 goalstate",
		},
		{
			cmd:  "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4 --max-time 8",
			desc: "wireserver port 32526 vmSettings",
		},
	}

	allowedExitCodes := map[string]bool{"28": true, "7": true}

	var errs []error
	for _, check := range checks {
		var execResult *podExecResult
		// Per-attempt cap (15s) prevents a single SPDY/exec hang from consuming the entire
		// poll budget. Derived from the poll's inner ctx so it honors both the per-attempt
		// cap and the overall poll deadline, whichever fires first.
		pollErr := wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
			attemptCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			r, execErr := execOnUnprivilegedPod(attemptCtx, s.Runtime.Kube, nonHostPod.Namespace, nonHostPod.Name, check.cmd)
			if execErr != nil {
				if errors.Is(execErr, context.DeadlineExceeded) {
					s.Logger.Logf("wireserver check %q: exec attempt timed out after 15s (retrying): %v", check.desc, execErr)
				} else {
					s.Logger.Logf("wireserver check %q: exec error (retrying): %v", check.desc, execErr)
				}
				return false, nil
			}
			execResult = r
			return true, nil
		})
		if pollErr != nil {
			// Without a curl result there is nothing to assert on for this check, but the
			// remaining checks are independent so keep going.
			errs = append(errs, fmt.Errorf("wireserver check %q: exec failed after retries: %w", check.desc, pollErr))
			continue
		}

		if allowedExitCodes[execResult.exitCode] {
			continue
		}

		// Diagnostics are only collected on failure, so the happy path stays cheap.
		errs = append(errs, fmt.Errorf("wireserver check %q: unexpected curl exit code %q (want 28 timeout or 7 refused)\n"+
			"stdout=%q, stderr=%q\n"+
			"FORWARD chain:\n%s\n"+
			"KUBE-FORWARD chain:\n%s\n"+
			"iptables-save filter:\n%s\n"+
			"conntrack:\n%s",
			check.desc, execResult.exitCode, execResult.stdout, execResult.stderr,
			collectVMDiagnostic(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers"),
			collectVMDiagnostic(ctx, s, "sudo iptables -t filter -L KUBE-FORWARD -v -n --line-numbers 2>/dev/null || echo 'chain not found'"),
			collectVMDiagnostic(ctx, s, "sudo iptables-save -t filter 2>/dev/null | head -80"),
			collectVMDiagnostic(ctx, s, "sudo conntrack -L -d 168.63.129.16 2>/dev/null || echo 'conntrack not available'")))
	}

	return errors.Join(errs...)
}

// collectVMDiagnostic runs a diagnostic command on the VM and returns its combined output.
// It is only used to enrich failure messages, so a collection failure is rendered inline
// rather than returned - it must never mask the failure being diagnosed.
func collectVMDiagnostic(ctx context.Context, s *Scenario, cmd string) string {
	result, err := execScriptOnVMForScenario(ctx, s, cmd)
	if err != nil {
		return fmt.Sprintf("<failed to collect %q: %v>", cmd, err)
	}
	return result.String()
}

// vhdHasHostsPluginArtifacts checks if the VHD has aks-localdns-hosts-setup.service installed
// by running a file existence check on the VM. Returns false if the service file is absent,
// meaning the VHD predates the hosts plugin feature and validators should be skipped.
func vhdHasHostsPluginArtifacts(ctx context.Context, s *Scenario) (bool, error) {
	result, err := execScriptOnVMForScenario(ctx, s, "test -f /etc/systemd/system/aks-localdns-hosts-setup.service")
	if err != nil {
		return false, fmt.Errorf("failed to check for aks-localdns-hosts-setup.service on the VM: %w", err)
	}
	return result.exitCode == "0", nil
}
