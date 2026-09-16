package scenario

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// vhdHasLocalDNSFallbackArtifacts reports whether the VHD under test has the
// pod-DNS fallback artifacts baked in. The AgentBaker E2E pipeline runs against
// VHDs built from main, which may not yet include these files until this PR
// merges; mirror the guard used for the hosts-plugin feature so the scenario
// degrades gracefully on older VHDs instead of failing spuriously.
func vhdHasLocalDNSFallbackArtifacts(ctx context.Context, s *Scenario) (bool, error) {
	result, err := execScriptOnVMForScenario(ctx, s,
		"test -f /etc/systemd/system/localdns-fallback.service && "+
			"test -x /opt/azure/containers/localdns/localdns-fallback.sh && "+
			"test -f /etc/systemd/system/localdns-fallback-probe.timer")
	if err != nil {
		return false, fmt.Errorf("failed to check for localdns fallback artifacts on the VM: %w", err)
	}
	return result.exitCode == "0", nil
}

// podClusterFirstDNSLinux builds a long-lived ClusterFirst pod pinned to the
// scenario's node. It is the subject of the test, not a helper: kubelet writes
// 169.254.10.11 into its sandbox resolv.conf at creation time and never revisits
// it, so this pod is exactly the "already-running pod that cannot be repointed"
// the fallback exists to protect. It must be created BEFORE localdns is broken.
func podClusterFirstDNSLinux(s *Scenario) *corev1.Pod {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if s.Tags.MockAzureChinaCloud {
		image = "mcr.azk8s.cn/cbl-mariner/busybox:2.0"
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-localdns-clusterfirst", s.Runtime.VM.KubeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			// Explicit for clarity: this is the policy that makes kubelet write
			// --cluster-dns (169.254.10.11) into the pod's resolv.conf.
			DNSPolicy: corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{
					Name:    "probe",
					Image:   image,
					Command: []string{"sleep", "infinity"},
				},
			},
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
		},
	}
}

// ValidateLocalDNSFallbackRecovery exercises the pod-DNS fallback end to end on
// a LocalDNS-enabled node, from the point of view of a pod that was already
// running when localdns broke.
//
//	Wiring:        assert the three lifecycle fixes are actually in effect
//	               (OnFailure present; no Conflicts= on the fallback;
//	               StartLimitIntervalSec honoured rather than silently dropped).
//	Anti-flap:     while localdns is between restart attempts, the fallback must
//	               NOT seize .11 — otherwise it is torn down ~RestartSec later
//	               and pods see a flapping resolver instead of a stable one.
//	Case 1:        drive localdns to terminal 'failed' and assert the already-
//	               running ClusterFirst pod still resolves through .11.
//	Teardown:      delete the dummy interface entirely (full localdns teardown)
//	               and assert the fallback recreates it and pod DNS returns.
//	Recovery:      restart localdns and assert it reclaims .11 with no sustained
//	               gap (the ExecStartPre handoff).
//	Case 3/4:      with localdns cleanly stopped (never 'failed'), assert the
//	               probe timer brings the fallback up within the debounce window.
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) error {
	hasArtifacts, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
	if err != nil {
		return fmt.Errorf("detect localdns fallback artifacts: %w", err)
	}
	if !hasArtifacts {
		s.Logger.Logf("WARNING: VHD does not have localdns-fallback artifacts — skipping fallback validation")
		return nil
	}

	// Create the ClusterFirst pod and keep it running for the whole test. We do
	// not reuse startPodAndCheckItRuns because that deletes the pod as soon as it
	// is Running, and the entire point here is that the pod outlives the outage.
	kube := s.Runtime.Kube
	pod := podClusterFirstDNSLinux(s)
	pod.Name = uniqueKubernetesResourceName(pod.Name)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return fmt.Errorf("set owner reference on localdns probe pod: %w", err)
	}
	s.Logger.Logf("creating long-lived ClusterFirst pod %q for localdns fallback validation", pod.Name)
	created, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create localdns probe pod %q: %w", pod.Name, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))}
		if err := kube.Typed.CoreV1().Pods(created.Namespace).Delete(cleanupCtx, created.Name, deleteOptions); err != nil && !apierrors.IsNotFound(err) {
			s.Logger.Logf("could not delete localdns probe pod %s: %v", created.Name, err)
		}
	}()
	if _, err := kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name); err != nil {
		return fmt.Errorf("wait for localdns probe pod %q to run: %w", created.Name, err)
	}

	// All remaining logic runs on the node so that DNS assertions do not depend on
	// the API server path (which itself goes through DNS). The script restores
	// state via an EXIT trap so a mid-test abort does not poison later scenarios
	// sharing the node.
	//
	// Time discipline: a node whose localdns is down goes NotReady, and AKS node
	// auto-repair reboots it once that persists past ~5 minutes. Every wait below
	// is hard-capped so the whole script finishes well inside that window.
	script := fmt.Sprintf(`set -uo pipefail
POD_NAME=%q
CLUSTER_IP=169.254.10.11
PROBE_NAME=health-check.localdns.local
FAIL=0

log()  { echo "fallback-e2e: $*"; }
fail() { echo "fallback-e2e: FAIL: $*" >&2; FAIL=1; }

cleanup() {
  log "cleanup: restoring localdns to healthy state"
  sudo systemctl stop localdns-fallback.service 2>/dev/null || true
  sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
  sudo systemctl reset-failed localdns.service 2>/dev/null || true
  sudo systemctl start localdns.service 2>/dev/null || true
}
trap cleanup EXIT

# Bounded wait for a shell predicate. Args: <seconds> <description> <cmd...>
wait_for() {
  local secs="$1" desc="$2"; shift 2
  local i=0
  while [ "$i" -lt "$secs" ]; do
    if "$@"; then log "condition met: ${desc} (after ${i}s)"; return 0; fi
    sleep 1; i=$((i+1))
  done
  fail "timed out waiting for: ${desc} (${secs}s)"
  return 1
}

# ---- Locate the pod's network namespace -----------------------------------
# Queries are issued from inside the running pod's netns so this is real
# pod-originated traffic against an unmodified /etc/resolv.conf, not a node-side
# stand-in for it. The netns path is read from the sandbox rather than a pid so
# we do not have to parse crictl's nested JSON.
SANDBOX=$(sudo crictl pods --name "$POD_NAME" --state Ready -q 2>/dev/null | head -1)
if [ -z "$SANDBOX" ]; then
  echo "fallback-e2e: FAIL: could not find a Ready sandbox for pod $POD_NAME" >&2
  exit 1
fi
NETNS=$(sudo crictl inspectp "$SANDBOX" 2>/dev/null | grep -oE '/var/run/netns/[A-Za-z0-9._-]+' | head -1)
if [ -z "$NETNS" ] || [ ! -e "$NETNS" ]; then
  echo "fallback-e2e: FAIL: could not resolve netns path for sandbox $SANDBOX" >&2
  exit 1
fi
log "pod $POD_NAME sandbox=$SANDBOX netns=$NETNS"

POD_RESOLV=$(sudo cat /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/"$SANDBOX"/resolv.conf 2>/dev/null || true)
echo "$POD_RESOLV" | grep -q "nameserver ${CLUSTER_IP}" \
  || fail "pod resolv.conf does not point at ${CLUSTER_IP} (got: $(echo "$POD_RESOLV" | tr '\n' ' '))"

# Resolve a cluster name AS THE POD. Exit 0 only on a real answer.
pod_resolves() {
  sudo nsenter --net="$NETNS" dig +short +timeout=2 +tries=1 \
    kubernetes.default.svc.cluster.local "@${CLUSTER_IP}" 2>/dev/null | grep -qE '^[0-9]+\.'
}
# Exit 0 for ANY DNS response (incl. NXDOMAIN) => some listener is alive on .11.
answers_11() {
  dig +short +timeout=1 +tries=1 "$PROBE_NAME" "@${CLUSTER_IP}" >/dev/null 2>&1
}
localdns_substate() { systemctl show localdns.service -p SubState --value 2>/dev/null; }

# ---- Wiring assertions (regression guards for the lifecycle fixes) ---------
log "asserting systemd wiring"
systemctl show localdns.service -p OnFailure 2>/dev/null | grep -q "OnFailure=localdns-fallback.service" \
  || fail "localdns missing OnFailure=localdns-fallback.service"

# Regression: Conflicts=localdns.service made every fallback start stop localdns,
# which reset localdns's restart counter, so the StartLimit budget was never
# spent and terminal 'failed' was never reached.
conflicts=$(systemctl show localdns-fallback.service -p Conflicts --value 2>/dev/null || true)
case "$conflicts" in
  *localdns.service*) fail "localdns-fallback.service must not declare Conflicts=localdns.service (got: $conflicts)" ;;
  *) log "fallback has no Conflicts= on localdns.service" ;;
esac

# Regression: StartLimitIntervalSec= in [Service] is silently dropped by systemd
# ("Unknown key name ... in section 'Service', ignoring"), leaving the default
# 10s/5 budget instead of the intended unbounded one. With the key in [Unit],
# 'systemctl show' reports StartLimitIntervalUSec=0 (rate limiting disabled);
# with it dropped it reports the 10s default, so this distinguishes the two.
sli=$(systemctl show localdns-fallback.service -p StartLimitIntervalUSec --value 2>/dev/null || true)
[ "$sli" = "0" ] \
  || fail "localdns-fallback StartLimitIntervalUSec should be '0' (key must live in [Unit]); got '$sli'"

# ---- Baseline -------------------------------------------------------------
log "baseline: pod resolves through .11 (served by localdns)"
wait_for 60 "baseline pod resolution via .11" pod_resolves || true

# ---- Anti-flap: no seizure of .11 mid restart-cycle ------------------------
# systemd fires OnFailure= on EVERY failed start, not just the terminal one.
# Sample localdns's SubState and the fallback together: the fallback must never
# be the owner of .11 while localdns is in auto-restart.
log "driving localdns to 'failed' by killing the supervisor until it latches"
flap_violations=0
deadline=$(( $(date +%%s) + 120 ))
while [ "$(date +%%s)" -lt "$deadline" ]; do
  [ "$(systemctl is-failed localdns.service 2>/dev/null || true)" = "failed" ] && break
  sub=$(localdns_substate)
  if [ "$sub" = "auto-restart" ] || [ "$sub" = "auto-restart-queued" ]; then
    if systemctl is-active --quiet localdns-fallback.service; then
      flap_violations=$((flap_violations+1))
    fi
  fi
  pid=$(systemctl show -p MainPID --value localdns.service 2>/dev/null || echo 0)
  if [ "${pid:-0}" -gt 0 ]; then sudo kill -9 "$pid" 2>/dev/null || true; fi
  sleep 0.5
done
[ "$(systemctl is-failed localdns.service 2>/dev/null || true)" = "failed" ] \
  || fail "localdns did not reach terminal 'failed' within the window (Conflicts= regression resets the restart counter)"
[ "$flap_violations" -eq 0 ] \
  || fail "fallback was active ${flap_violations} time(s) while localdns was mid restart-cycle (flapping)"

# ---- Case 1: terminal failed -> fallback serves the EXISTING pod -----------
log "asserting OnFailure started the fallback once localdns settled"
wait_for 30 "fallback active" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
ip addr show | grep -qw "$CLUSTER_IP" || fail ".11 not bound after fallback start"

log "asserting the already-running pod still resolves through .11 while localdns is failed"
wait_for 30 "pod resolution via fallback" pod_resolves \
  || fail "existing ClusterFirst pod could not resolve through ${CLUSTER_IP} while localdns was failed"

# ---- Full teardown: dummy interface deleted entirely -----------------------
# localdns's own teardown path (cleanup_localdns_configs) deletes the dummy
# interface, taking .11 off the node completely. The fallback must be able to
# recreate it, not just bind an address that happens to still be there.
log "simulating full localdns teardown: deleting the dummy interface"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo ip link del localdns 2>/dev/null || true
ip link show localdns >/dev/null 2>&1 && fail "dummy interface still present after teardown"
sudo systemctl start localdns-fallback.service 2>/dev/null || true
wait_for 30 "fallback recreated the dummy interface" sh -c 'ip link show localdns >/dev/null 2>&1' || true
ip addr show | grep -qw "$CLUSTER_IP" || fail ".11 not re-assigned after teardown recovery"
wait_for 30 "pod resolution after full teardown" pod_resolves \
  || fail "existing pod could not resolve through ${CLUSTER_IP} after a full localdns teardown"

# ---- Recovery + handoff: no sustained gap ---------------------------------
log "recovering localdns; probing .11 continuously across the handoff"
( end=$(( $(date +%%s) + 25 ))
  while [ "$(date +%%s)" -lt "$end" ]; do
    if answers_11; then echo OK; else echo SERVFAIL; fi
    sleep 0.2
  done ) > /tmp/fallback_handoff.log 2>&1 &
probe_pid=$!
sudo systemctl reset-failed localdns.service || true
sudo systemctl start localdns.service || true
wait_for 60 "localdns active again" sh -c 'systemctl is-active --quiet localdns.service' || true
wait "$probe_pid" 2>/dev/null || true

# Allow brief blips; fail only on a sustained run (>=5 consecutive ~1s) SERVFAILs.
if awk 'BEGIN{r=0;m=0}/SERVFAIL/{r++;if(r>m)m=r;next}{r=0}END{exit (m>=5)?1:0}' /tmp/fallback_handoff.log; then
  log "handoff: no sustained .11 gap"
else
  fail "sustained .11 SERVFAIL run during recovery handoff"
  echo "---- handoff log ----"; cat /tmp/fallback_handoff.log || true
fi
wait_for 30 "pod resolution restored by localdns" pod_resolves || true
systemctl is-active --quiet localdns-fallback.service \
  && fail "fallback should have stood down once localdns reclaimed .11"

# ---- Case 3/4: probe-driven trigger (no OnFailure event) -------------------
# Stop the fallback AND localdns so .11 goes dark with localdns NOT in 'failed'.
# The probe timer must bring the fallback up within its debounce window.
log "probe path: stopping localdns cleanly and the fallback so .11 goes dark"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
sudo systemctl stop localdns.service 2>/dev/null || true

log "asserting the probe timer is active so it can trigger the fallback"
systemctl is-active --quiet localdns-fallback-probe.timer || fail "probe timer not active"

log "waiting for the probe debounce to start the fallback (~15s + margin)"
wait_for 45 "fallback started by probe" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
wait_for 30 "pod resolution via probe-started fallback" pod_resolves || true

if [ "$FAIL" -ne 0 ]; then
  echo "fallback-e2e: one or more assertions FAILED" >&2
  exit 1
fi
echo "fallback-e2e: all assertions passed"
exit 0
`, created.Name)

	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
		"localdns pod-DNS fallback should keep an already-running ClusterFirst pod resolving on .11"); err != nil {
		return fmt.Errorf("validate localdns fallback recovery: %w", err)
	}
	return nil
}
