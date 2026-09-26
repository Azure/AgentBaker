package scenario

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// External DNS and service health can both pass with a broken CoreDNS upstream.
// Use the API's service address as the oracle, independently of rendered config.
func ValidateLocalDNSServiceDiscovery(ctx context.Context, s *Scenario) error {
	svc, err := s.Runtime.Kube.Typed.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get Kubernetes service: %w", err)
	}
	var errs []error
	for _, listener := range []string{"169.254.10.10", "169.254.10.11"} {
		result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("dig +short +time=5 +tries=2 @%s kubernetes.default.svc.cluster.local A", listener), 0, "cluster DNS lookup failed")
		if err == nil {
			err = validateServiceDNSAnswer(result.stdout, svc.Spec.ClusterIP)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("LocalDNS listener %s: %w", listener, err))
		}
	}
	return errors.Join(errs...)
}

func validateServiceDNSAnswer(answer, expected string) error {
	if strings.TrimSpace(answer) != expected || expected == "" {
		return fmt.Errorf("service DNS answer %q, expected %q", strings.TrimSpace(answer), expected)
	}
	return nil
}

// Verify a normal pod's resolver, without @server or a host network namespace.
func validateDNSWorkload(ctx context.Context, s *Scenario, expectedNameserver string) error {
	svc, err := s.Runtime.Kube.Typed.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return err
	}
	script, err := dnsWorkloadScript(expectedNameserver, svc.Spec.ClusterIP)
	if err != nil {
		return err
	}
	pod := podHTTPServerLinux(s)
	pod.Name = ""
	pod.GenerateName = "dns-workload-"
	pod.Spec.DNSPolicy = corev1.DNSClusterFirst
	pod.Spec.Containers[0].Command = []string{"sleep", "3600"}
	pod.Spec.Containers[0].Args = nil
	pods := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace)
	pod, err = pods.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := pods.Delete(cleanupCtx, pod.Name, metav1.DeleteOptions{}); err != nil {
			logging.Logf(ctx, "delete DNS workload %s: %v", pod.Name, err)
		}
	}()
	if _, err = s.Runtime.Kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name); err != nil {
		return err
	}
	// Node/Pod Ready can precede service routing convergence on a fresh node.
	// Keep checking the actual resolver; persistent misconfiguration must time out.
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
		result, err := execOnPod(ctx, s.Runtime.Kube, pod.Namespace, pod.Name, []string{"sh", "-c", script})
		if err != nil {
			return false, err
		}
		logging.Logf(ctx, "DNS workload (exit %s): %s\n%s", result.exitCode, result.stdout, result.stderr)
		return result.exitCode == "0", nil
	})
}

func dnsWorkloadScript(nameserver, serviceIP string) (string, error) {
	for _, ip := range []string{nameserver, serviceIP} {
		if _, err := netip.ParseAddr(ip); err != nil {
			return "", fmt.Errorf("invalid DNS expectation: %w", err)
		}
	}
	return fmt.Sprintf(`set -eu
cat /etc/resolv.conf
actual=$(awk '$1 == "nameserver" {print $2}' /etc/resolv.conf)
test "$actual" = '%s'
if ! answer=$(nslookup kubernetes.default.svc.cluster.local 2>&1); then
  echo "$answer"
  exit 1
fi
echo "$answer"
echo "$answer" | awk -v expected='%s' '$1 == "Name:" {answer=1} answer && (($1 == "Address:" && $2 == expected) || ($1 == "Address" && $3 == expected)) {found=1} END {exit !found}'
`, nameserver, serviceIP), nil
}
