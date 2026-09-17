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

// alternateKubeDNSService is a second Service in front of the same CoreDNS pods,
// used to prove COREDNS_SERVICE_IP is honoured verbatim rather than falling back
// to the hardcoded 10.0.0.10 default. The ClusterIP is deliberately left unset so
// the API server allocates one from whatever service CIDR this cluster uses --
// hardcoding an address here would guess wrong on a custom-CIDR cluster, which is
// precisely the case this assertion exists to cover.
func alternateKubeDNSService(s *Scenario) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kube-dns-alt", s.Runtime.VM.KubeName),
			Namespace: "kube-system",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"k8s-app": "kube-dns"},
			Ports: []corev1.ServicePort{
				{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP},
				{Name: "dns-tcp", Port: 53, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

// ValidateLocalDNSFallbackRecovery exercises the pod-DNS fallback end to end on a
// LocalDNS-enabled node, from the point of view of a pod that was already running
// when localdns broke.
//
//	Wiring:      the three lifecycle fixes are in effect (OnFailure present; no
//	             Conflicts= on the fallback; StartLimitIntervalSec honoured).
//	Anti-flap:   the fallback must not seize .11 while localdns is in auto-restart,
//	             or it is torn down ~RestartSec later and pods see a flapping
//	             resolver instead of a stable one.
//	Teardown:    localdns's own cleanup deletes the dummy interface, taking .11 off
//	             the node entirely; the fallback must rebuild it from nothing.
//	Case 1:      with localdns in terminal 'failed', the already-running
//	             ClusterFirst pod still resolves cluster and external names.
//	Custom IP:   COREDNS_SERVICE_IP is honoured verbatim, proven positively (an
//	             alternate kube-dns ClusterIP resolves) and negatively (a bogus
//	             upstream must fail, so a silent fallback to the hardcoded default
//	             cannot pass).
//	Recovery:    localdns reclaims .11 and the fallback stands down.
//	Case 3/4:    with localdns cleanly stopped (never 'failed'), the probe timer
//	             brings the fallback up within the debounce window.
//
// Fault injection corrupts LOCALDNS_COREFILE_BASE in /etc/localdns/environment.
// Two alternatives do NOT work and should not be reintroduced:
//   - Corrupting localdns.corefile: localdns.sh logs "Regenerating localdns
//     corefile" and rebuilds it from the base64 on every start, so it self-heals.
//   - kill -9 on the supervisor: SIGKILL means localdns's trap never runs, so
//     cleanup_localdns_configs never deletes the dummy interface and the teardown
//     case is silently skipped. Only ExecStopPost runs, and that path deliberately
//     leaves the interface in place (localdns.sh:741-748).
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) error {
	hasArtifacts, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
	if err != nil {
		return fmt.Errorf("detect localdns fallback artifacts: %w", err)
	}
	if !hasArtifacts {
		s.Logger.Logf("WARNING: VHD does not have localdns-fallback artifacts — skipping fallback validation")
		return nil
	}

	kube := s.Runtime.Kube

	// Alternate kube-dns Service, so we have a real, kube-proxy-programmed
	// ClusterIP that is provably not the default one.
	svc := alternateKubeDNSService(s)
	createdSvc, err := kube.Typed.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create alternate kube-dns service: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := kube.Typed.CoreV1().Services(createdSvc.Namespace).Delete(cleanupCtx, createdSvc.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			s.Logger.Logf("could not delete alternate kube-dns service %s: %v", createdSvc.Name, err)
		}
	}()
	altClusterIP := createdSvc.Spec.ClusterIP
	if altClusterIP == "" {
		return fmt.Errorf("alternate kube-dns service %q was not allocated a ClusterIP", createdSvc.Name)
	}
	s.Logger.Logf("alternate kube-dns Service %q allocated ClusterIP %s", createdSvc.Name, altClusterIP)

	// Create the ClusterFirst pod and keep it running for the whole test. We do
	// not reuse startPodAndCheckItRuns because that deletes the pod as soon as it
	// is Running, and the entire point here is that the pod outlives the outage.
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
	// auto-repair reboots it once that persists past ~5 minutes. Fault-to-terminal-
	// 'failed' measures ~60s, and every wait below is hard-capped so the whole
	// script finishes well inside that window.
	script := fmt.Sprintf(`set -uo pipefail
POD_NAME=%q
ALT_CLUSTER_IP=%q
CLUSTER_IP=169.254.10.11
PROBE_NAME=health-check.localdns.local
BOGUS_UPSTREAM=240.0.0.1
ENVF=/etc/localdns/environment
ENVBAK=/run/localdns-fallback-e2e-env.bak
FAIL=0

log()  { echo "fallback-e2e: $*"; }
ok()   { echo "fallback-e2e: PASS: $*"; }
fail() { echo "fallback-e2e: FAIL: $*" >&2; FAIL=1; }

cleanup() {
  log "cleanup: restoring localdns to healthy state"
  [ -f "$ENVBAK" ] && sudo cp -a "$ENVBAK" "$ENVF" 2>/dev/null || true
  sudo rm -f "$ENVBAK" 2>/dev/null || true
  sudo systemctl stop localdns-fallback.service 2>/dev/null || true
  sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
  sudo systemctl reset-failed localdns.service 2>/dev/null || true
  sudo systemctl start localdns.service 2>/dev/null || true
}
trap cleanup EXIT
sudo cp -a "$ENVF" "$ENVBAK"

# Read-then-write. 'sudo tee' on the same file you are reading truncates it first,
# so always stage through a temp file.
set_upstream() {
  local ip="$1" tmp; tmp=$(mktemp)
  grep -v '^COREDNS_SERVICE_IP=' "$ENVF" > "$tmp"
  echo "COREDNS_SERVICE_IP=${ip}" >> "$tmp"
  sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
}
corrupt_corefile_base() {
  local tmp bad; tmp=$(mktemp)
  bad=$(printf 'THIS IS NOT A VALID COREFILE {{{\n' | base64 -w0)
  while IFS= read -r l; do
    case "$l" in LOCALDNS_COREFILE_BASE=*) echo "LOCALDNS_COREFILE_BASE=${bad}" ;; *) echo "$l" ;; esac
  done < "$ENVF" > "$tmp"
  sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
}

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
# pod-originated traffic against an unmodified /etc/resolv.conf. The netns path is
# read from the sandbox rather than a pid because grepping crictl's JSON for "pid"
# matches the wrong field and silently yields 1 (the host netns).
SANDBOX=$(sudo crictl pods --name "$POD_NAME" --state Ready -q 2>/dev/null | head -1)
if [ -z "$SANDBOX" ]; then echo "fallback-e2e: FAIL: no Ready sandbox for $POD_NAME" >&2; exit 1; fi
NETNS=$(sudo crictl inspectp "$SANDBOX" 2>/dev/null | grep -oE '/var/run/netns/[A-Za-z0-9._-]+' | head -1)
if [ -z "$NETNS" ] || [ ! -e "$NETNS" ]; then echo "fallback-e2e: FAIL: no netns for $SANDBOX" >&2; exit 1; fi
log "pod $POD_NAME sandbox=$SANDBOX netns=$NETNS alt_clusterip=$ALT_CLUSTER_IP"

sudo cat /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/"$SANDBOX"/resolv.conf 2>/dev/null \
  | grep -q "nameserver ${CLUSTER_IP}" \
  && ok "pod resolv.conf points at ${CLUSTER_IP}" \
  || fail "pod resolv.conf does not point at ${CLUSTER_IP}"

pod_resolves()     { sudo nsenter --net="$NETNS" dig +short +timeout=3 +tries=1 kubernetes.default.svc.cluster.local "@${CLUSTER_IP}" 2>/dev/null | grep -qE '^[0-9]+\.'; }
pod_resolves_ext() { sudo nsenter --net="$NETNS" dig +short +timeout=3 +tries=1 mcr.microsoft.com "@${CLUSTER_IP}" 2>/dev/null | grep -qE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+'; }
answers_11()       { dig +short +timeout=1 +tries=1 "$PROBE_NAME" "@${CLUSTER_IP}" >/dev/null 2>&1; }
localdns_substate() { systemctl show localdns.service -p SubState --value 2>/dev/null; }

# INTEGRITY: which unit owns .11:53. Without this, an answer could be coming from a
# still-running localdns and every fallback assertion below would be vacuous.
owner_of_11() { sudo ss -lunpH 2>/dev/null | grep '169.254.10.11:53' | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2; }
owner_unit() {
  local p; p=$(owner_of_11); [ -z "$p" ] && { echo "NONE"; return; }
  sudo grep -oE 'localdns[a-z-]*\.service' /proc/"$p"/cgroup 2>/dev/null | head -1 || echo "unknown"
}
assert_fallback_owns_11() {
  local u; u=$(owner_unit)
  [ "$u" = "localdns-fallback.service" ] \
    && ok "$1: .11:53 owned by localdns-fallback.service (pid $(owner_of_11))" \
    || fail "$1: .11:53 owner is '${u}', not the fallback — answers would be untrustworthy"
}

# ---- Wiring assertions (regression guards for the lifecycle fixes) ---------
systemctl show localdns.service -p OnFailure 2>/dev/null | grep -q "OnFailure=localdns-fallback.service" \
  && ok "localdns declares OnFailure=localdns-fallback.service" \
  || fail "localdns missing OnFailure=localdns-fallback.service"

# Regression: Conflicts=localdns.service made every fallback start stop localdns,
# which reset localdns's restart counter, so the StartLimit budget was never spent
# and terminal 'failed' was never reached.
conflicts=$(systemctl show localdns-fallback.service -p Conflicts --value 2>/dev/null || true)
case "$conflicts" in
  *localdns.service*) fail "localdns-fallback.service must not declare Conflicts=localdns.service (got: $conflicts)" ;;
  *) ok "fallback declares no Conflicts= on localdns.service" ;;
esac

# Regression: StartLimitIntervalSec= in [Service] is silently dropped by systemd
# ("Unknown key name ... in section 'Service', ignoring"), leaving the default
# 10s/5 budget. With the key in [Unit], systemctl reports 0 (rate limiting off);
# with it dropped it reports the 10s default, so this distinguishes the two.
sli=$(systemctl show localdns-fallback.service -p StartLimitIntervalUSec --value 2>/dev/null || true)
[ "$sli" = "0" ] && ok "fallback StartLimitIntervalUSec=0 (key is in [Unit])" \
  || fail "fallback StartLimitIntervalUSec should be '0'; got '$sli' (key likely still in [Service])"

# ---- Baseline -------------------------------------------------------------
set_upstream 10.0.0.10
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl restart localdns.service 2>/dev/null || true
wait_for 90 "baseline pod resolution via .11" pod_resolves || true

# ---- Drive to terminal 'failed', watching for flapping --------------------
MARK=$(date -u '+%%Y-%%m-%%d %%H:%%M:%%S'); sleep 1
corrupt_corefile_base
log "restarting localdns to pick up the bad config"
sudo systemctl restart --no-block localdns.service
flap_violations=0
START=$(date +%%s)
while [ $(( $(date +%%s) - START )) -lt 220 ]; do
  [ "$(systemctl is-failed localdns.service 2>/dev/null || true)" = "failed" ] && break
  sub=$(localdns_substate)
  if [ "$sub" = "auto-restart" ] || [ "$sub" = "auto-restart-queued" ]; then
    systemctl is-active --quiet localdns-fallback.service && flap_violations=$((flap_violations+1))
  fi
  sleep 1
done
if [ "$(systemctl is-failed localdns.service 2>/dev/null || true)" != "failed" ]; then
  # Hard abort: without this precondition every assertion below is meaningless.
  echo "fallback-e2e: FAIL: localdns never reached terminal 'failed' (Conflicts= regression resets the restart counter)" >&2
  exit 1
fi
ok "localdns reached terminal 'failed' in $(( $(date +%%s) - START ))s"
[ "$flap_violations" -eq 0 ] \
  && ok "fallback never active while localdns was mid restart-cycle" \
  || fail "fallback was active ${flap_violations} time(s) during auto-restart (flapping)"

# ---- Teardown + trigger sequence ------------------------------------------
J=$(journalctl -u localdns.service --since "$MARK" --no-pager -o short 2>/dev/null)
echo "$J" | grep -q "Triggering OnFailure= dependencies" \
  && ok "systemd logged 'Triggering OnFailure= dependencies'" || fail "OnFailure= never triggered"
echo "$J" | grep -q "Successfully removed localdns dummy interface" \
  && ok "localdns cleanup deleted the dummy interface (full teardown: .11 off the node)" \
  || fail "cleanup did not delete the dummy interface — the teardown case was not exercised"
echo "$J" | grep -q "Successfully cleanup localdns related configurations" \
  && ok "localdns cleanup reported completion" || fail "cleanup did not report completion"

wait_for 30 "fallback active" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
F=$(journalctl -u localdns-fallback.service --since "$MARK" --no-pager -o short 2>/dev/null | grep -v "Unknown key")
echo "$F" | grep -q "dummy interface 'localdns' absent; creating it" \
  && ok "fallback found .11 gone and rebuilt the interface from nothing" \
  || fail "fallback did not report recreating an absent interface"
echo "$F" | grep -q "not taking over ${CLUSTER_IP} yet" \
  && ok "fallback declined at least one mid-restart OnFailure= invocation" \
  || log "note: no declined invocation observed (localdns may have failed on its first attempt)"
assert_fallback_owns_11 "terminal-failed"

# ---- Case 1: the already-running pod keeps resolving ----------------------
wait_for 30 "pod resolution via fallback (default ClusterIP)" pod_resolves \
  && ok "existing ClusterFirst pod resolved a cluster name through the fallback" \
  || fail "existing pod could not resolve a cluster name while localdns was failed"
pod_resolves_ext && ok "existing pod resolved an external name through the fallback" \
  || fail "existing pod could not resolve an external name"

# ---- Custom ClusterIP: positive ------------------------------------------
# localdns stays in terminal 'failed'; only COREDNS_SERVICE_IP changes.
sudo conntrack -D -p udp --dport 53 >/dev/null 2>&1 || true
set_upstream "$ALT_CLUSTER_IP"
sudo systemctl restart localdns-fallback.service; sleep 5
assert_fallback_owns_11 "custom-clusterip"
COREFILE=/opt/azure/containers/localdns/fallback/fallback.corefile
sudo grep -q "forward . ${ALT_CLUSTER_IP}" "$COREFILE" \
  && ok "fallback corefile forwards to the custom ClusterIP ${ALT_CLUSTER_IP}" \
  || fail "corefile does not forward to ${ALT_CLUSTER_IP}"
sudo grep -q "forward . 10.0.0.10" "$COREFILE" \
  && fail "corefile fell back to the hardcoded default 10.0.0.10" \
  || ok "corefile does not reference the hardcoded default"
wait_for 30 "pod resolution via custom ClusterIP" pod_resolves \
  && ok "existing pod resolved a cluster name via the custom ClusterIP" \
  || fail "pod could not resolve via ${ALT_CLUSTER_IP}"
pod_resolves_ext && ok "existing pod resolved an external name via the custom ClusterIP" \
  || fail "external lookup failed via ${ALT_CLUSTER_IP}"
if command -v conntrack >/dev/null 2>&1; then
  sudo conntrack -L -p udp 2>/dev/null | grep "dst=${ALT_CLUSTER_IP}" | head -3 || true
  sudo conntrack -L -p udp 2>/dev/null | grep -q "dst=${ALT_CLUSTER_IP}" \
    && ok "conntrack confirms queries were sent to ${ALT_CLUSTER_IP} and DNAT'd by kube-proxy" \
    || log "note: no conntrack entry to ${ALT_CLUSTER_IP} captured (entry may have aged out)"
else
  log "note: conntrack not installed on this image; skipping the packet-path assertion"
fi

# ---- Custom ClusterIP: negative control ----------------------------------
# Without this, a silent fallback to the hardcoded 10.0.0.10 would still pass the
# positive test on a default-CIDR cluster.
set_upstream "$BOGUS_UPSTREAM"
sudo systemctl restart localdns-fallback.service; sleep 5
assert_fallback_owns_11 "bogus-upstream"
if pod_resolves; then
  fail "pod still resolved with upstream ${BOGUS_UPSTREAM} — COREDNS_SERVICE_IP is being ignored"
else
  ok "pod correctly failed to resolve with a bogus upstream (the value is honoured)"
fi

# ---- Back to the custom ClusterIP: prove the negative was not a fluke -----
set_upstream "$ALT_CLUSTER_IP"
sudo systemctl restart localdns-fallback.service; sleep 5
wait_for 30 "pod resolution restored on the custom ClusterIP" pod_resolves \
  && ok "pod resolves again on the custom ClusterIP" \
  || fail "pod did not recover on ${ALT_CLUSTER_IP}"

# ---- Recovery + handoff: no sustained gap ---------------------------------
sudo cp -a "$ENVBAK" "$ENVF"
set_upstream "$ALT_CLUSTER_IP"
log "recovering localdns; probing .11 continuously across the handoff"
( end=$(( $(date +%%s) + 25 ))
  while [ "$(date +%%s)" -lt "$end" ]; do
    if answers_11; then echo OK; else echo SERVFAIL; fi
    sleep 0.2
  done ) > /tmp/fallback_handoff.log 2>&1 &
probe_pid=$!
sudo systemctl reset-failed localdns.service || true
sudo systemctl start --no-block localdns.service || true
wait_for 90 "localdns active again" sh -c 'systemctl is-active --quiet localdns.service' || true
wait "$probe_pid" 2>/dev/null || true

# Allow brief blips; fail only on a sustained run (>=5 consecutive ~1s) SERVFAILs.
if awk 'BEGIN{r=0;m=0}/SERVFAIL/{r++;if(r>m)m=r;next}{r=0}END{exit (m>=5)?1:0}' /tmp/fallback_handoff.log; then
  ok "handoff: no sustained .11 gap"
else
  fail "sustained .11 SERVFAIL run during recovery handoff"
  echo "---- handoff log ----"; cat /tmp/fallback_handoff.log || true
fi
sleep 3
[ "$(owner_unit)" = "localdns.service" ] \
  && ok "localdns reclaimed .11 and the fallback stood down" \
  || fail "after recovery .11 is owned by '$(owner_unit)', expected localdns.service"
wait_for 30 "pod resolution restored by localdns" pod_resolves || true

# ---- Case 3/4: probe-driven trigger (no OnFailure event) -------------------
# Stop the fallback AND localdns so .11 goes dark with localdns NOT in 'failed'.
# A clean stop is not a failure, so OnFailure= never fires and the probe timer is
# the only thing that can bring the fallback back.
log "probe path: stopping localdns cleanly so .11 goes dark without a failure"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
sudo systemctl stop localdns.service 2>/dev/null || true
[ "$(systemctl is-failed localdns.service 2>/dev/null || true)" != "failed" ] \
  && ok "localdns is stopped but NOT 'failed' (so OnFailure= cannot fire)" \
  || log "note: localdns shows failed; the probe path is still exercised below"
systemctl is-active --quiet localdns-fallback-probe.timer \
  && ok "probe timer is active" || fail "probe timer not active"
wait_for 45 "fallback started by the probe" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
assert_fallback_owns_11 "probe-triggered"
wait_for 30 "pod resolution via probe-started fallback" pod_resolves \
  && ok "existing pod resolves after a clean stop, via the probe-started fallback" \
  || fail "pod could not resolve after a clean localdns stop"

if [ "$FAIL" -ne 0 ]; then
  echo "fallback-e2e: one or more assertions FAILED" >&2
  exit 1
fi
echo "fallback-e2e: all assertions passed"
exit 0
`, created.Name, altClusterIP)

	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
		"localdns pod-DNS fallback should keep an already-running ClusterFirst pod resolving on .11, including on a custom CoreDNS ClusterIP"); err != nil {
		return fmt.Errorf("validate localdns fallback recovery: %w", err)
	}
	return nil
}
