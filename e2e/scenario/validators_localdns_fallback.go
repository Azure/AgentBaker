package scenario

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func validateLocalDNSFailureAndFallback(ctx context.Context, s *Scenario) (err error) {
	if err := validateLocalDNSRestartBudgetDirectives(ctx, s); err != nil {
		return err
	}
	for _, path := range []string{
		"/etc/systemd/system/localdns-fallback.service",
		"/opt/azure/containers/localdns/localdns-fallback.sh",
	} {
		if err := ValidateFileExists(ctx, s, path); err != nil {
			return fmt.Errorf("LocalDNS fallback artifact missing from VHD: %w", err)
		}
	}
	// Baseline probes also pull the image before the DNS disruption.
	if err := validateLocalDNSLookupPods(ctx, s); err != nil {
		return fmt.Errorf("LocalDNS baseline: %w", err)
	}

	// The normal path restores once below. Any failure after fault injection also
	// restores, including a failed guard that installed unexpected NAT rules.
	defer func() {
		if err == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		output, diagErr := runLocalDNSNodeCommand(cleanupCtx, s,
			"sudo journalctl -u localdns.service -u localdns-fallback.service -n 40 --no-pager")
		logging.Logf(ctx, "LocalDNS failure journal:\n%s\njournal error: %v", output, diagErr)
		// Force cleanup only after failure, so it cannot mask a broken handoff.
		for _, command := range []string{
			"sudo systemctl stop localdns-fallback.service",
			"sudo /opt/azure/containers/localdns/localdns-fallback.sh clear",
			"sudo systemctl reset-failed localdns-fallback.service",
		} {
			if _, cleanupErr := runLocalDNSNodeCommand(cleanupCtx, s, command); cleanupErr != nil {
				err = errors.Join(err, cleanupErr)
			}
		}
		if cleanupErr := restoreLocalDNSAfterFault(cleanupCtx, s); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("restore LocalDNS: %w", cleanupErr))
		}
	}()

	if err := driveLocalDNSToTerminalFailed(ctx, s); err != nil {
		return err
	}
	if err := waitForLocalDNSState(ctx, s, "localdns-fallback.service", "active", time.Minute); err != nil {
		return err
	}
	resolvers, err := runLocalDNSNodeCommand(ctx, s, "cat /run/systemd/resolve/resolv.conf")
	if err != nil {
		return err
	}
	upstreamRestored := false
	for _, line := range strings.Split(resolvers, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" && fields[1] != "169.254.10.10" {
			upstreamRestored = true
		}
	}
	if !upstreamRestored {
		return fmt.Errorf("DHCP resolvers were not restored after LocalDNS exit:\n%s", resolvers)
	}
	if err := checkLocalDNSNAT(ctx, s, true); err != nil {
		return err
	}
	if err := validateLocalDNSLookupPods(ctx, s); err != nil {
		return fmt.Errorf("LocalDNS fallback: %w", err)
	}

	if err := restoreLocalDNSAfterFault(ctx, s); err != nil {
		return err
	}
	if err := validateLocalDNSLookupPods(ctx, s); err != nil {
		return fmt.Errorf("LocalDNS recovery: %w", err)
	}

	// A blocking start must be skipped by ExecCondition while LocalDNS is healthy.
	if _, err := runLocalDNSNodeCommand(ctx, s, "sudo systemctl start localdns-fallback.service"); err != nil {
		return err
	}
	state, err := runLocalDNSNodeCommand(ctx, s, "systemctl show localdns-fallback.service -p ActiveState --value")
	if err != nil {
		return err
	}
	if strings.TrimSpace(state) != "inactive" {
		return fmt.Errorf("fallback start was not skipped while LocalDNS was healthy: %s", state)
	}
	return checkLocalDNSNAT(ctx, s, false)
}

func runLocalDNSNodeCommand(ctx context.Context, s *Scenario, command string) (string, error) {
	// Single-line commands avoid the SSH helper's SCP upload path.
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"bash -euo pipefail -c '"+strings.ReplaceAll(command, "'", "'\"'\"'")+"'", 0, "LocalDNS node check failed")
	if err != nil {
		return "", err
	}
	return result.stdout, nil
}

func waitForLocalDNSState(ctx context.Context, s *Scenario, unit, want string, timeout time.Duration) error {
	var state string
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		output, err := runLocalDNSNodeCommand(ctx, s, "systemctl show "+unit+" -p ActiveState --value")
		if err != nil {
			return false, err
		}
		state = strings.TrimSpace(output)
		return state == want, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for %s=%s (last state %q): %w", unit, want, state, err)
	}
	return nil
}

func restoreLocalDNSAfterFault(ctx context.Context, s *Scenario) error {
	if _, err := runLocalDNSNodeCommand(ctx, s, fmt.Sprintf(
		"sudo rm -f %s; sudo systemctl daemon-reload; sudo systemctl reset-failed localdns.service; sudo systemctl restart localdns.service --no-block",
		localdnsFaultDropIn)); err != nil {
		return err
	}
	if err := waitForLocalDNSState(ctx, s, "localdns.service", "active", 2*time.Minute); err != nil {
		return err
	}
	if err := waitForLocalDNSState(ctx, s, "localdns-fallback.service", "inactive", 30*time.Second); err != nil {
		return err
	}
	return checkLocalDNSNAT(ctx, s, false)
}

func checkLocalDNSNAT(ctx context.Context, s *Scenario, active bool) error {
	nat, err := runLocalDNSNodeCommand(ctx, s, "sudo iptables -w 5 -t nat -S")
	if err != nil {
		return err
	}
	var raw string
	if active {
		raw, err = runLocalDNSNodeCommand(ctx, s, "sudo iptables -w 5 -t raw -S")
		if err != nil {
			return err
		}
	}
	return validateLocalDNSNATRules(nat, raw, active)
}

// localDNSFallbackHooks are the chain and the two jumps that must exist while the
// fallback is active.
//
// Matched as patterns, not exact lines, because the jumps now carry
// -m comment --comment "AKS managed: added by AgentBaker ..." between the hook and
// the -j. Both forms have to pass: a lane resolving a main-built image gets a
// localdns-fallback.sh that predates the comment, while the VHD gate builds from
// the branch and gets one that has it.
var localDNSFallbackHooks = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^-N LOCALDNS-FALLBACK$`),
	regexp.MustCompile(`(?m)^-A PREROUTING .*-j LOCALDNS-FALLBACK$`),
	regexp.MustCompile(`(?m)^-A OUTPUT .*-j LOCALDNS-FALLBACK$`),
}

func validateLocalDNSNATRules(nat, raw string, active bool) error {
	if !active {
		if strings.Contains(nat, "LOCALDNS-FALLBACK") {
			return fmt.Errorf("fallback NAT rules or hooks survived recovery:\n%s", nat)
		}
		return nil
	}
	for _, want := range localDNSFallbackHooks {
		if !want.MatchString(nat) {
			return fmt.Errorf("no fallback rule matching %q:\n%s", want, nat)
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, " -j NOTRACK") &&
			(strings.Contains(line, " -d 169.254.10.10/32 ") ||
				strings.Contains(line, " -d 169.254.10.11/32 ")) {
			return fmt.Errorf("LocalDNS NOTRACK rule survived failure: %s", line)
		}
	}
	return nil
}

func validateLocalDNSLookupPods(ctx context.Context, s *Scenario) error {
	// One name per pod, so a failure names itself and logPodDebugInfo can fetch the
	// container's output -- GetLogs needs an explicit container on a multi-container
	// pod, and swallows the 400 it returns otherwise.
	for _, probe := range []struct{ suffix, server, name string }{
		{"node-ext", "169.254.10.10", "mcr.microsoft.com"},
		{"cluster-svc", localDNSClusterListenerIP, "kubernetes.default.svc.cluster.local"},
		{"cluster-ext", localDNSClusterListenerIP, "mcr.microsoft.com"},
	} {
		if err := runLocalDNSLookupPod(ctx, s, localDNSLookupPod(s, probe.suffix, probe.server, probe.name)); err != nil {
			return err
		}
	}
	// Pod traffic exercises PREROUTING; retain the host's OUTPUT-path check too.
	for _, probe := range []struct{ server, name string }{
		{"169.254.10.10", "mcr.microsoft.com"},
		{localDNSClusterListenerIP, "kubernetes.default.svc.cluster.local"},
	} {
		if err := digFromNode(ctx, s, probe.server, probe.name); err != nil {
			return err
		}
	}
	return nil
}

// digFromNode resolves one name from the node itself, retrying for as long as the
// pod probes get. The retry is here rather than in the command because the node
// only has to run a bare dig: a shell loop would need its own errexit guard (dig
// exits non-zero on timeout, which would abort the loop) and its own way to carry
// the last answer back out past the exit code.
func digFromNode(ctx context.Context, s *Scenario, server, name string) error {
	var last string
	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, true,
		func(ctx context.Context) (bool, error) {
			// Not the exit-code-validating helper: a timed-out attempt is a retry,
			// not a failure.
			result, err := execScriptOnVMForScenario(ctx, s,
				fmt.Sprintf("dig +short +timeout=3 +tries=1 %s @%s", name, server))
			if err != nil {
				return false, err // SSH itself broke, not a DNS answer
			}
			last = result.stdout
			return dnsAnswerHasAddress(result.stdout), nil
		})
	if err != nil {
		return fmt.Errorf("node could not resolve %s via %s (last answer %q): %w", name, server, last, err)
	}
	return nil
}

// dnsAnswerHasAddress reports whether "dig +short" returned an address rather than
// nothing, a CNAME on its own, or an error line.
func dnsAnswerHasAddress(answer string) bool {
	for _, line := range strings.Split(answer, "\n") {
		if ip := net.ParseIP(strings.TrimSpace(line)); ip != nil && ip.To4() != nil {
			return true
		}
	}
	return false
}

func localDNSLookupPod(s *Scenario, suffix, server, name string) *corev1.Pod {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if s.Tags.MockAzureChinaCloud {
		image = "mcr.azk8s.cn/cbl-mariner/busybox:2.0"
	}
	// Retried, not one-shot. Resolving an external name through the cluster listener
	// is the longest path here -- pod -> .11 -> KUBE-SVC -> a CoreDNS replica -> its
	// own .10 -> VNet DNS -- and a single timeout anywhere along it would otherwise
	// fail the scenario. The shell this replaced polled for the same reason.
	//
	// The last attempt runs with output attached so nslookup's own message reaches
	// the container log, which is what TerminationMessageFallbackToLogsOnError and
	// logPodDebugInfo surface on failure. Without it a failure says only "exit 1".
	script := fmt.Sprintf(
		"for i in $(seq 1 10); do nslookup %s %s >/dev/null 2>&1 && exit 0; sleep 3; done; "+
			"echo \"final attempt, %s via %s:\"; nslookup %s %s",
		name, server, name, server, name, server)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueKubernetesResourceName(s.Runtime.VM.KubeName + "-localdns-" + suffix),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: to.Ptr(int64(120)),
			DNSPolicy:             corev1.DNSDefault,
			Containers: []corev1.Container{{
				Name: "lookup", Image: image, ImagePullPolicy: corev1.PullIfNotPresent,
				// Explicit server: a new Default pod during fallback otherwise inherits
				// VNet DNS and bypasses the dead LocalDNS listener.
				Command:                  []string{"sh", "-c", script},
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
			}},
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
		},
	}
}

func runLocalDNSLookupPod(ctx context.Context, s *Scenario, pod *corev1.Pod) (err error) {
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return err
	}
	pods := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace)
	created, err := pods.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create DNS pod %s: %w", pod.Name, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if cleanupErr := pods.Delete(cleanupCtx, created.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))}); cleanupErr != nil && !apierrors.IsNotFound(cleanupErr) {
			err = errors.Join(err, fmt.Errorf("delete DNS pod %s: %w", created.Name, cleanupErr))
		}
	}()
	if err := waitForLocalDNSLookupPod(ctx, s.Runtime.Kube, created); err != nil {
		return err
	}
	logging.Logf(ctx, "DNS pod %s succeeded", created.Name)
	return nil
}

func waitForLocalDNSLookupPod(ctx context.Context, kube *Kubeclient, pod *corev1.Pod) error {
	last := pod
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		current, err := kube.Typed.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		last = current
		switch current.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("DNS pod %s failed: %s %s; %s",
				pod.Name, current.Status.Reason, current.Status.Message, formatPodDiagnostics(current))
		default:
			return false, nil
		}
	})
	if err != nil {
		debugCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		logPodDebugInfo(debugCtx, kube, last)
		for _, container := range last.Status.ContainerStatuses {
			if terminated := container.State.Terminated; terminated != nil && terminated.ExitCode != 0 {
				logging.Logf(debugCtx, "DNS container %s exited %d: %s\n%s",
					container.Name, terminated.ExitCode, terminated.Reason, terminated.Message)
			}
		}
		return fmt.Errorf("waiting for DNS pod %s to succeed (last phase %s): %w", pod.Name, last.Status.Phase, err)
	}
	return nil
}
