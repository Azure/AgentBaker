package scenario

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Shared shell prelude for every node-side phase: logging, assertion counters, a
// bounded waiter, and the .11 ownership check. Each phase is a separate SSH
// invocation, so this cannot live in a single long-lived script.
//
// The ownership check is load-bearing, not decoration: without it an answer from a
// still-running localdns would make every "the fallback served this" assertion
// vacuous, which is exactly how a broken harness run once scored green.
const localDNSFallbackPrelude = `set -uo pipefail
CLUSTER_IP=169.254.10.11
NODE_IP=169.254.10.10
ENVF=/etc/localdns/environment
ENVBAK=/run/localdns-fallback-e2e-env.bak
COREFILE_BAK=/run/localdns-fallback-e2e-corefile.bak
UPDATED_COREFILE=/opt/azure/containers/localdns/updated.localdns.corefile
FALLBACK_COREFILE=/opt/azure/containers/localdns/fallback/fallback.corefile
KUBELET_DEFAULT_FILE=/etc/default/kubelet
FAIL=0

log()  { echo "fallback-e2e: $*"; }
ok()   { echo "fallback-e2e: PASS: $*"; }
fail() { echo "fallback-e2e: FAIL: $*" >&2; FAIL=1; }
finish() { [ "$FAIL" -eq 0 ] || { echo "fallback-e2e: phase FAILED" >&2; exit 1; }; echo "fallback-e2e: phase passed"; exit 0; }

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

kubelet_cluster_dns() { grep -oE '\-\-cluster-dns=[^" ]+' "$KUBELET_DEFAULT_FILE" 2>/dev/null | head -1 | cut -d= -f2-; }
localdns_substate()   { systemctl show localdns.service -p SubState --value 2>/dev/null; }
owner_of_11() { sudo ss -lunpH 2>/dev/null | grep "${CLUSTER_IP}:53" | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2; }
owner_unit() {
  local p; p=$(owner_of_11); [ -z "$p" ] && { echo "NONE"; return; }
  sudo grep -oE 'localdns[a-z-]*\.service' /proc/"$p"/cgroup 2>/dev/null | head -1 || echo "unknown"
}
assert_fallback_owns_11() {
  local u; u=$(owner_unit)
  [ "$u" = "localdns-fallback.service" ] \
    && ok "$1: ${CLUSTER_IP}:53 owned by localdns-fallback.service" \
    || fail "$1: ${CLUSTER_IP}:53 owner is '${u}', not the fallback"
}

# Resolve a name from inside a pod's network namespace, against a given server.
# Both the netns AND an explicit server are required: 'nsenter -n' changes only the
# network namespace, so a bare dig would read the HOST's /etc/resolv.conf and
# silently measure the wrong resolver.
pod_netns() {
  local sandbox
  sandbox=$(sudo crictl pods --name "$1" --state Ready -q 2>/dev/null | head -1)
  [ -z "$sandbox" ] && return 1
  sudo crictl inspectp "$sandbox" 2>/dev/null | grep -oE '/var/run/netns/[A-Za-z0-9._-]+' | head -1
}
pod_nameserver() {
  local sandbox
  sandbox=$(sudo crictl pods --name "$1" --state Ready -q 2>/dev/null | head -1)
  [ -z "$sandbox" ] && return 1
  sudo grep -oE '^nameserver .*' \
    /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/"$sandbox"/resolv.conf 2>/dev/null \
    | head -1 | awk '{print $2}'
}
pod_resolves_via() {   # <pod> <server>
  local ns; ns=$(pod_netns "$1") || return 1
  sudo nsenter --net="$ns" dig +short +timeout=3 +tries=1 \
    kubernetes.default.svc.cluster.local "@$2" 2>/dev/null | grep -qE '^[0-9]+\.'
}
pod_resolves_external_via() {
  local ns; ns=$(pod_netns "$1") || return 1
  sudo nsenter --net="$ns" dig +short +timeout=3 +tries=1 \
    mcr.microsoft.com "@$2" 2>/dev/null | grep -qE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+'
}
`

// vhdHasLocalDNSFallbackArtifacts reports whether the VHD under test has the
// pod-DNS fallback artifacts baked in. The AgentBaker E2E pipeline runs against
// VHDs built from main, which may not yet include these files until this PR
// merges; mirror the guard used for the hosts-plugin feature so the scenario
// degrades gracefully on older VHDs instead of failing spuriously.
func vhdHasLocalDNSFallbackArtifacts(ctx context.Context, s *Scenario) (bool, error) {
	result, err := execScriptOnVMForScenario(ctx, s,
		"test -f /etc/systemd/system/localdns-fallback.service && "+
			"test -x /opt/azure/containers/localdns/localdns-fallback.sh && "+
			"test -x /opt/azure/containers/localdns/localdns-kubelet-dns.sh && "+
			"test -f /etc/systemd/system/localdns-fallback-probe.timer")
	if err != nil {
		return false, fmt.Errorf("failed to check for localdns fallback artifacts on the VM: %w", err)
	}
	return result.exitCode == "0", nil
}

// podClusterFirstDNSLinux builds a ClusterFirst pod pinned to the scenario's node.
// kubelet writes whatever --cluster-dns currently says into the pod's sandbox
// resolv.conf at creation time and never revisits it, so WHEN a pod is created
// relative to the localdns failure determines which resolver it is stuck with for
// life. That is the whole point of the three cases below.
func podClusterFirstDNSLinux(s *Scenario, suffix string) *corev1.Pod {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if s.Tags.MockAzureChinaCloud {
		image = "mcr.azk8s.cn/cbl-mariner/busybox:2.0"
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-localdns-%s", s.Runtime.VM.KubeName, suffix),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			DNSPolicy: corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{Name: "probe", Image: image, Command: []string{"sleep", "infinity"}},
			},
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
		},
	}
}

// startLocalDNSProbePod creates a ClusterFirst pod and waits for it to run,
// returning its name and a deletion func. Unlike startPodAndCheckItRuns it does
// NOT delete the pod on success: these pods must outlive the outage, because what
// is being tested is the resolver they were born with.
func startLocalDNSProbePod(ctx context.Context, s *Scenario, suffix string) (string, func(), error) {
	kube := s.Runtime.Kube
	pod := podClusterFirstDNSLinux(s, suffix)
	pod.Name = uniqueKubernetesResourceName(pod.Name)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return "", func() {}, fmt.Errorf("set owner reference on %q: %w", pod.Name, err)
	}
	created, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", func() {}, fmt.Errorf("create pod %q: %w", pod.Name, err)
	}
	del := func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		opts := metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))}
		if err := kube.Typed.CoreV1().Pods(created.Namespace).Delete(cleanupCtx, created.Name, opts); err != nil && !apierrors.IsNotFound(err) {
			logging.Logf(ctx, "could not delete pod %s: %v", created.Name, err)
		}
	}
	if _, err := kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name); err != nil {
		del()
		return "", func() {}, fmt.Errorf("wait for pod %q to run: %w", created.Name, err)
	}
	logging.Logf(ctx, "localdns probe pod %q is running", created.Name)
	return created.Name, del, nil
}

// ValidateLocalDNSFallbackRecovery covers the three pod generations on a node
// whose localdns has failed. A pod's /etc/resolv.conf is written once by kubelet at
// sandbox creation and never revisited, so each generation needs a different
// remedy and they must be asserted separately:
//
//	Case 1  pod created BEFORE the failure  -> stuck on .11 forever; the fallback
//	        must keep answering there (localdns-fallback.service)
//	Case 2  pod created DURING the failure  -> kubelet has been repointed, so it is
//	        born with the real CoreDNS ClusterIP and never touches .11 at all
//	Case 3  pod created AFTER recovery      -> back to .11, served by localdns
//
// Two DIFFERENT faults are exercised, because localdns has two cleanup paths with
// different scopes and a single fault only reaches one of them. Measured on a live
// node (Ubuntu 24.04, systemd 255), with #9360's ExecStopPost installed:
//
//	              node DNS + iptables        dummy interface
//	kill -9       restored (ExecStopPost)    SURVIVES
//	script exit   restored                   DELETED (EXIT trap)
//
// ExecStopPost is run by systemd, not by the dying process, so SIGKILL does NOT
// skip it -- node DNS is restored either way. But bash traps cannot catch SIGKILL,
// and the EXIT trap is the only thing that deletes the interface
// (localdns.sh: 'ip link del name localdns'); ExecStopPost deliberately leaves it
// alone (localdns.sh: "It intentionally does not delete the dummy localdns
// interface"). So a kill -9 test can never exercise the rebuild-from-nothing path,
// and a graceful-failure test can never exercise the interface-already-present
// path. Both are covered below; do not collapse them into one.
//
// Also covered: that the fallback corefile is DERIVED from the .11 half of
// localdns's real corefile rather than a lossy hand-written minimum, the minimal
// corefile floor when that derivation is impossible, and the no-op guards that keep
// a spurious trigger from hot-looping on a healthy node.
//
// Fault injection for the teardown path corrupts LOCALDNS_COREFILE_BASE in
// /etc/localdns/environment. Corrupting localdns.corefile does NOT work:
// localdns.sh logs "Regenerating localdns corefile" and rebuilds it from the
// base64 on every start, so it self-heals and the node never breaks.
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) error {
	hasArtifacts, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
	if err != nil {
		return fmt.Errorf("detect localdns fallback artifacts: %w", err)
	}
	if !hasArtifacts {
		logging.Logf(ctx, "WARNING: VHD does not have localdns-fallback artifacts — skipping fallback validation")
		return nil
	}

	phase := func(name, script string) error {
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localDNSFallbackPrelude+script, 0,
			"localdns fallback e2e: "+name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}

	// Always put the node back, even if a phase fails part-way. A node left with a
	// corrupted localdns environment or a repointed kubelet would poison every
	// later scenario sharing it.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		restore := localDNSFallbackPrelude + `
[ -f "$ENVBAK" ] && sudo cp -a "$ENVBAK" "$ENVF"
[ -f "$COREFILE_BAK" ] && sudo cp -a "$COREFILE_BAK" "$UPDATED_COREFILE"
sudo rm -f "$ENVBAK" "$COREFILE_BAK" /run/localdns-fallback/consecutive_fails 2>/dev/null || true
# Drop-in from the crash phase, in case that phase aborted before removing it.
sudo rm -f /etc/systemd/system/localdns.service.d/99-e2e-crashpath.conf 2>/dev/null || true
sudo systemctl daemon-reload 2>/dev/null || true
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl start localdns.service 2>/dev/null || true
sleep 10
# The fallback repoints kubelet; if a phase aborted mid-outage, put it back so the
# node is not left sending new pods somewhere the next scenario does not expect.
if [ "$(kubelet_cluster_dns)" != "$CLUSTER_IP" ]; then
  sudo /opt/azure/containers/localdns/localdns-kubelet-dns.sh restore 2>/dev/null || true
fi
log "post-test state: localdns=$(systemctl is-active localdns.service) cluster-dns=$(kubelet_cluster_dns)"
exit 0
`
		if _, err := execScriptOnVMForScenario(cleanupCtx, s, restore); err != nil {
			logging.Logf(ctx, "localdns fallback e2e: node restore failed: %v", err)
		}
	}()

	// ---- Case 1 subject: created while localdns is healthy -------------------
	pod1, del1, err := startLocalDNSProbePod(ctx, s, "before")
	if err != nil {
		return fmt.Errorf("create pre-failure probe pod: %w", err)
	}
	defer del1()

	// ---- Phase 0: wiring ----------------------------------------------------
	// Runs before anything is broken, deliberately. These are cheap, they need no
	// fault, and each maps to a specific defect found on a live node. If one of
	// them regresses, it must surface as its own message rather than as a
	// confusing downstream failure in a phase that assumed the wiring was sound.
	if err := phase("phase0-wiring", `
# Wiring regressions, each tied to a defect found on a live node.
systemctl show localdns.service -p OnFailure 2>/dev/null | grep -q "OnFailure=localdns-fallback.service" \
  && ok "localdns declares OnFailure=localdns-fallback.service" || fail "localdns missing OnFailure="
systemctl show localdns.service -p ExecStartPost --value 2>/dev/null | grep -q "localdns-kubelet-dns.sh restore" \
  && ok "localdns restores kubelet --cluster-dns via ExecStartPost" || fail "localdns missing the kubelet restore ExecStartPost"
# Conflicts=localdns.service made every fallback start stop localdns, resetting its
# restart counter so terminal 'failed' was never reached.
conflicts=$(systemctl show localdns-fallback.service -p Conflicts --value 2>/dev/null || true)
case "$conflicts" in
  *localdns.service*) fail "fallback must not declare Conflicts=localdns.service (got: $conflicts)" ;;
  *) ok "fallback declares no Conflicts= on localdns.service" ;;
esac
# StartLimitIntervalSec= in [Service] is silently dropped, leaving the 10s/5 default.
sli=$(systemctl show localdns-fallback.service -p StartLimitIntervalUSec --value 2>/dev/null || true)
[ "$sli" = "0" ] && ok "fallback StartLimitIntervalUSec=0 (key is in [Unit])" \
  || fail "fallback StartLimitIntervalUSec should be 0; got '$sli'"

finish
`); err != nil {
		return err
	}

	// ---- Phase 1: the crash path (kill -9) ----------------------------------
	// SIGKILL is the most realistic failure there is -- a crash, an OOM kill, a
	// watchdog kill -- and it produces a genuinely different end state from a
	// graceful failure. Asserting the difference is the point.
	if err := phase("phase1-crash-path", fmt.Sprintf(`
POD1=%q
# Restart=no for this phase only. With Restart=on-failure the unit comes back
# within RestartSec and its START path deletes and recreates the interface, so the
# post-kill state would be a race instead of an observation.
sudo mkdir -p /etc/systemd/system/localdns.service.d
printf '[Service]\nRestart=no\n' | sudo tee /etc/systemd/system/localdns.service.d/99-e2e-crashpath.conf >/dev/null
sudo systemctl daemon-reload
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl start localdns.service 2>/dev/null || true
wait_for 90 "localdns healthy before the crash" sh -c 'systemctl is-active --quiet localdns.service' || true
ip link show localdns >/dev/null 2>&1 && ok "pre-crash: dummy interface exists" || fail "pre-crash: no dummy interface"

MARK=$(date -u '+%%Y-%%m-%%d %%H:%%M:%%S'); sleep 1
PID=$(systemctl show -p MainPID --value localdns.service)
log "sending SIGKILL to localdns MainPID=${PID}"
sudo kill -9 "$PID"
wait_for 60 "localdns in terminal failed after SIGKILL" \
  sh -c '[ "$(systemctl is-failed localdns.service)" = "failed" ]' || true

# #9360: ExecStopPost is run by systemd, not the dying process, so SIGKILL does
# not skip it. Node DNS and iptables must be restored even here.
[ "$(grep -m1 nameserver /run/systemd/resolve/resolv.conf 2>/dev/null | awk '{print $2}')" != "$NODE_IP" ] \
  && ok "CRASH: node DNS was restored off ${NODE_IP} (ExecStopPost ran despite SIGKILL)" \
  || fail "CRASH: node DNS still points at ${NODE_IP}; ExecStopPost did not run"
[ "$(sudo iptables -w -t raw -L -n 2>/dev/null | grep -c 'localdns: skip conntrack')" = "0" ] \
  && ok "CRASH: localdns iptables rules were removed" \
  || fail "CRASH: localdns iptables rules survived the crash"

# ...but the EXIT trap cannot run under SIGKILL, and it is the only thing that
# deletes the interface. This asymmetry is exactly why a kill -9 test cannot stand
# in for the teardown test.
ip link show localdns >/dev/null 2>&1 \
  && ok "CRASH: dummy interface SURVIVES a SIGKILL (EXIT trap never ran)" \
  || fail "CRASH: dummy interface was deleted; the crash/teardown asymmetry no longer holds"
ip addr show dev localdns 2>/dev/null | grep -qw "$CLUSTER_IP" \
  && ok "CRASH: ${CLUSTER_IP} is still assigned after the crash" \
  || fail "CRASH: ${CLUSTER_IP} missing after the crash"
journalctl -u localdns.service --since "$MARK" --no-pager -o cat 2>/dev/null \
  | grep -q "Removing localdns dummy interface" \
  && fail "CRASH: the EXIT trap ran under SIGKILL, which should be impossible" \
  || ok "CRASH: no EXIT-trap teardown in the journal, as expected under SIGKILL"

# The fallback's other branch: taking over an interface that is ALREADY present.
# The teardown phases only ever exercise the create-from-nothing branch.
sudo systemctl reset-failed localdns-fallback.service 2>/dev/null || true
sudo systemctl start localdns-fallback.service 2>/dev/null || true
wait_for 30 "fallback active over the surviving interface" \
  sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
journalctl -u localdns-fallback.service --since "$MARK" --no-pager -o cat 2>/dev/null \
  | grep -q "already present on localdns" \
  && ok "CRASH: fallback reused the surviving interface instead of recreating it" \
  || fail "CRASH: fallback did not report reusing an already-present ${CLUSTER_IP}"
assert_fallback_owns_11 "CRASH"
wait_for 30 "pod resolution over the surviving interface" pod_resolves_via "$POD1" "$CLUSTER_IP" \
  && ok "CRASH: pre-existing pod resolves through the fallback after a SIGKILL" \
  || fail "CRASH: pre-existing pod cannot resolve after a SIGKILL"

# Restore for the teardown phases that follow.
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/localdns.service.d/99-e2e-crashpath.conf
sudo systemctl daemon-reload
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl start localdns.service 2>/dev/null || true
wait_for 90 "localdns healthy again" sh -c 'systemctl is-active --quiet localdns.service' || true
wait_for 60 "kubelet --cluster-dns restored after the crash phase" \
  sh -c '[ "$(grep -oE "\-\-cluster-dns=[^\" ]+" /etc/default/kubelet | head -1 | cut -d= -f2-)" = "169.254.10.11" ]' \
  && ok "CRASH: kubelet --cluster-dns was restored on recovery" \
  || fail "CRASH: kubelet --cluster-dns is $(kubelet_cluster_dns) after recovery"
finish
`, pod1)); err != nil {
		return err
	}

	// ---- Phase 2: baseline, induce the teardown failure ----------------------
	if err := phase("phase2-induce-failure", fmt.Sprintf(`
POD1=%q
sudo cp -a "$ENVF" "$ENVBAK"
sudo cp -a "$UPDATED_COREFILE" "$COREFILE_BAK"

[ "$(kubelet_cluster_dns)" = "$CLUSTER_IP" ] \
  && ok "baseline: kubelet --cluster-dns is ${CLUSTER_IP}" \
  || fail "baseline: kubelet --cluster-dns is $(kubelet_cluster_dns), expected ${CLUSTER_IP}"
[ "$(pod_nameserver "$POD1")" = "$CLUSTER_IP" ] \
  && ok "baseline: pre-failure pod resolv.conf points at ${CLUSTER_IP}" \
  || fail "baseline: pre-failure pod nameserver is $(pod_nameserver "$POD1")"
wait_for 60 "baseline pod resolution" pod_resolves_via "$POD1" "$CLUSTER_IP" || true

# Corrupt the base64 corefile SOURCE so localdns fails unrecoverably.
bad=$(printf 'THIS IS NOT A VALID COREFILE {{{\n' | base64 -w0)
tmp=$(mktemp)
while IFS= read -r l; do
  case "$l" in LOCALDNS_COREFILE_BASE=*) echo "LOCALDNS_COREFILE_BASE=${bad}" ;; *) echo "$l" ;; esac
done < "$ENVF" > "$tmp"
sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
log "injected fault; restarting localdns"
sudo systemctl restart --no-block localdns.service

flap=0
START=$(date +%%s)
while [ $(( $(date +%%s) - START )) -lt 240 ]; do
  [ "$(systemctl is-failed localdns.service)" = "failed" ] && break
  sub=$(localdns_substate)
  if [ "$sub" = "auto-restart" ] || [ "$sub" = "auto-restart-queued" ]; then
    systemctl is-active --quiet localdns-fallback.service && flap=$((flap+1))
  fi
  sleep 1
done
if [ "$(systemctl is-failed localdns.service)" != "failed" ]; then
  echo "fallback-e2e: FAIL: localdns never reached terminal 'failed'" >&2; exit 1
fi
ok "localdns reached terminal 'failed' in $(( $(date +%%s) - START ))s"
[ "$flap" -eq 0 ] && ok "fallback never active while localdns was mid restart-cycle" \
  || fail "fallback was active ${flap} time(s) during auto-restart (flapping)"

J=$(journalctl -u localdns.service --since "-6min" --no-pager -o cat 2>/dev/null)
echo "$J" | grep -q "Successfully removed localdns dummy interface" \
  && ok "TEARDOWN: localdns's EXIT trap deleted the dummy interface (.11 off the node entirely)" \
  || fail "TEARDOWN: interface was not deleted; this is the path kill -9 cannot reach, so it must be exercised here"
journalctl -u localdns.service --since "-6min" --no-pager -o short 2>/dev/null | grep -q "Triggering OnFailure= dependencies" \
  && ok "systemd triggered OnFailure=" || fail "OnFailure= never triggered"

wait_for 45 "fallback active" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
journalctl -u localdns-fallback.service --since "-6min" --no-pager -o cat 2>/dev/null \
  | grep -q "dummy interface 'localdns' absent; creating it" \
  && ok "fallback rebuilt the dummy interface from nothing" \
  || fail "fallback did not report recreating an absent interface"
assert_fallback_owns_11 "phase1"
wait_for 45 "kubelet repointed to the CoreDNS ClusterIP" \
  sh -c '[ "$(grep -oE "\-\-cluster-dns=[^\" ]+" /etc/default/kubelet | head -1 | cut -d= -f2-)" != "169.254.10.11" ]' || true
finish
`, pod1)); err != nil {
		return err
	}

	// ---- Case 2 subject: created while localdns is failed --------------------
	pod2, del2, err := startLocalDNSProbePod(ctx, s, "during")
	if err != nil {
		return fmt.Errorf("create during-failure probe pod: %w", err)
	}
	defer del2()

	// ---- Phase 3: assert cases 1 and 2, and the derived corefile -------------
	if err := phase("phase3-cases-1-and-2", fmt.Sprintf(`
POD1=%q
POD2=%q
COREDNS_IP=$(kubelet_cluster_dns)
log "kubelet --cluster-dns is now ${COREDNS_IP}"

# --- Case 1: the pod that predates the failure is still on .11, served by us ---
[ "$(pod_nameserver "$POD1")" = "$CLUSTER_IP" ] \
  && ok "CASE 1: pre-failure pod is still pinned to ${CLUSTER_IP}" \
  || fail "CASE 1: pre-failure pod nameserver changed to $(pod_nameserver "$POD1")"
assert_fallback_owns_11 "CASE 1"
wait_for 30 "CASE 1 cluster lookup" pod_resolves_via "$POD1" "$CLUSTER_IP" \
  && ok "CASE 1: pre-failure pod resolves a cluster name through the fallback" \
  || fail "CASE 1: pre-failure pod cannot resolve through ${CLUSTER_IP}"
pod_resolves_external_via "$POD1" "$CLUSTER_IP" \
  && ok "CASE 1: pre-failure pod resolves an external name through the fallback" \
  || fail "CASE 1: external lookup failed through ${CLUSTER_IP}"

# --- Case 2: the pod born during the outage bypasses .11 entirely ---
[ "$COREDNS_IP" != "$CLUSTER_IP" ] \
  && ok "CASE 2: kubelet --cluster-dns was repointed away from ${CLUSTER_IP}" \
  || fail "CASE 2: kubelet --cluster-dns is still ${CLUSTER_IP}"
[ "$(pod_nameserver "$POD2")" = "$COREDNS_IP" ] \
  && ok "CASE 2: during-failure pod was born with nameserver ${COREDNS_IP}" \
  || fail "CASE 2: during-failure pod nameserver is $(pod_nameserver "$POD2"), expected ${COREDNS_IP}"
[ "$(pod_nameserver "$POD2")" != "$CLUSTER_IP" ] \
  && ok "CASE 2: during-failure pod does not depend on the fallback at all" \
  || fail "CASE 2: during-failure pod is pinned to ${CLUSTER_IP}"
wait_for 30 "CASE 2 cluster lookup" pod_resolves_via "$POD2" "$COREDNS_IP" \
  && ok "CASE 2: during-failure pod resolves a cluster name directly against CoreDNS" \
  || fail "CASE 2: during-failure pod cannot resolve against ${COREDNS_IP}"
pod_resolves_external_via "$POD2" "$COREDNS_IP" \
  && ok "CASE 2: during-failure pod resolves an external name" \
  || fail "CASE 2: external lookup failed against ${COREDNS_IP}"
sudo test -s /etc/localdns/kubelet-cluster-dns.orig \
  && ok "CASE 2: original --cluster-dns was recorded for restore" \
  || fail "CASE 2: no recorded original --cluster-dns"

# --- Minimal-corefile floor: the fault corrupted the corefile source, so the
# derivation must have rejected it rather than shipping garbage to coredns. ---
journalctl -u localdns-fallback.service --since "-8min" --no-pager -o cat 2>/dev/null \
  | grep -q "falling back to the minimal corefile" \
  && ok "FLOOR: derivation rejected the corrupted corefile and used the minimal one" \
  || log "note: no rejection logged (localdns may have failed before regenerating the corefile)"
sudo grep -q "bind ${CLUSTER_IP}" "$FALLBACK_COREFILE" \
  && ok "FLOOR: fallback corefile binds ${CLUSTER_IP}" || fail "FLOOR: fallback corefile does not bind ${CLUSTER_IP}"
sudo grep -q "$NODE_IP" "$FALLBACK_COREFILE" \
  && fail "fallback corefile references the node listener ${NODE_IP}" \
  || ok "fallback corefile never references the node listener ${NODE_IP}"

# --- Derived corefile: restore the healthy corefile (localdns stays failed, its
# env source is still corrupt) and restart the fallback so derivation can run. ---
sudo cp -a "$COREFILE_BAK" "$UPDATED_COREFILE"
sudo systemctl restart localdns-fallback.service; sleep 5
assert_fallback_owns_11 "derived"
if journalctl -u localdns-fallback.service --since "-2min" --no-pager -o cat 2>/dev/null | grep -q "derived fallback corefile"; then
  ok "DERIVED: fallback derived its corefile from localdns's own .11 server blocks"
  sudo grep -q "$NODE_IP" "$FALLBACK_COREFILE" \
    && fail "DERIVED: corefile still binds the node listener ${NODE_IP}" \
    || ok "DERIVED: dual-bind health-check block was rewritten to ${CLUSTER_IP} only"
  # Behaviour a hand-written minimal corefile would silently have dropped.
  sudo grep -q "serve_stale" "$FALLBACK_COREFILE" \
    && ok "DERIVED: inherited serve_stale (minimal corefile has none)" \
    || log "note: source corefile has no serve_stale configured"
  sudo grep -q "force_tcp" "$FALLBACK_COREFILE" \
    && ok "DERIVED: inherited the cluster.local force_tcp block" \
    || log "note: source corefile has no force_tcp block"
  wait_for 30 "CASE 1 lookup on the derived corefile" pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "DERIVED: pre-failure pod still resolves on the derived corefile" \
    || fail "DERIVED: pre-failure pod cannot resolve on the derived corefile"

  # Hosts plugin. This is the single largest behavioural difference between the
  # derived corefile and the minimal one, and it only matters on clusters that
  # opted into extra DNS hardening -- i.e. exactly the ones that can least afford
  # to lose it mid-outage. Measured on a live node: with the minimal corefile a
  # name that exists ONLY in /etc/localdns/hosts returns empty; with the derived
  # corefile it still resolves.
  src_hosts=$(sudo grep -c 'hosts /etc/localdns/hosts' "$UPDATED_COREFILE" 2>/dev/null || echo 0)
  drv_hosts=$(sudo grep -c 'hosts /etc/localdns/hosts' "$FALLBACK_COREFILE" 2>/dev/null || echo 0)
  if [ "$src_hosts" -gt 0 ]; then
    [ "$drv_hosts" -gt 0 ] \
      && ok "DERIVED: inherited the hosts plugin block from localdns's corefile" \
      || fail "DERIVED: source corefile has a hosts block but the derived one does not"
    # Functional proof, not just structural: a canary that exists nowhere but the
    # local hosts file must still resolve through the fallback.
    CANARY="localdns-fallback-e2e-canary.invalid"
    if sudo test -f /etc/localdns/hosts; then
      printf '10.99.99.99 %%s\n' "$CANARY" | sudo tee -a /etc/localdns/hosts >/dev/null
      sleep 7   # hosts plugin is configured with reload 5s
      got=$(sudo nsenter --net="$(pod_netns "$POD1")" dig +short +timeout=3 +tries=1 "$CANARY" "@${CLUSTER_IP}" 2>/dev/null | head -1)
      [ "$got" = "10.99.99.99" ] \
        && ok "DERIVED: hosts-plugin entries still resolve through the fallback (canary=${got})" \
        || fail "DERIVED: hosts-plugin canary returned '${got}', expected 10.99.99.99"
      sudo sed -i "/${CANARY}/d" /etc/localdns/hosts
    fi
  else
    [ "$drv_hosts" -eq 0 ] \
      && ok "DERIVED: no hosts block in the source, and none invented in the derived corefile" \
      || fail "DERIVED: derived corefile has a hosts block the source does not"
    log "note: hosts plugin not enabled on this cluster; canary check skipped"
  fi
else
  log "note: derivation did not run (corefile still unusable); minimal floor already asserted"
fi
finish
`, pod1, pod2)); err != nil {
		return err
	}

	// ---- Phase 4: recover ----------------------------------------------------
	if err := phase("phase4-recover", `
sudo cp -a "$ENVBAK" "$ENVF"
sudo systemctl reset-failed localdns.service || true
sudo systemctl start --no-block localdns.service || true
wait_for 120 "localdns active again" sh -c 'systemctl is-active --quiet localdns.service' || true
sleep 5
[ "$(owner_unit)" = "localdns.service" ] \
  && ok "RECOVERY: localdns reclaimed ${CLUSTER_IP} and the fallback stood down" \
  || fail "RECOVERY: ${CLUSTER_IP} owner is '$(owner_unit)', expected localdns.service"
wait_for 60 "kubelet --cluster-dns restored" \
  sh -c '[ "$(grep -oE "\-\-cluster-dns=[^\" ]+" /etc/default/kubelet | head -1 | cut -d= -f2-)" = "169.254.10.11" ]' \
  && ok "RECOVERY: kubelet --cluster-dns is back to ${CLUSTER_IP}" \
  || fail "RECOVERY: kubelet --cluster-dns is $(kubelet_cluster_dns)"
sudo test -e /etc/localdns/kubelet-cluster-dns.orig \
  && fail "RECOVERY: the recorded-original state file was not cleared" \
  || ok "RECOVERY: recorded-original state file was cleared"
finish
`); err != nil {
		return err
	}

	// ---- Case 3 subject: created after recovery ------------------------------
	pod3, del3, err := startLocalDNSProbePod(ctx, s, "after")
	if err != nil {
		return fmt.Errorf("create post-recovery probe pod: %w", err)
	}
	defer del3()

	// ---- Phase 5: case 3 and the anti-hot-loop guards ------------------------
	return phase("phase5-case-3-and-guards", fmt.Sprintf(`
POD1=%q
POD3=%q

# --- Case 3: pods born after recovery are back on the localdns listener ---
[ "$(pod_nameserver "$POD3")" = "$CLUSTER_IP" ] \
  && ok "CASE 3: post-recovery pod was born with nameserver ${CLUSTER_IP}" \
  || fail "CASE 3: post-recovery pod nameserver is $(pod_nameserver "$POD3"), expected ${CLUSTER_IP}"
wait_for 30 "CASE 3 cluster lookup" pod_resolves_via "$POD3" "$CLUSTER_IP" \
  && ok "CASE 3: post-recovery pod resolves through localdns" \
  || fail "CASE 3: post-recovery pod cannot resolve"
pod_resolves_via "$POD1" "$CLUSTER_IP" \
  && ok "CASE 1: pre-failure pod still resolves after recovery" \
  || fail "CASE 1: pre-failure pod broken after recovery"

# --- Guard: a spurious start must not fight a healthy localdns for .11. Without
# this the unit's unbounded restart budget turns a bind conflict into a hot loop. ---
before=$(owner_of_11)
sudo systemctl reset-failed localdns-fallback.service 2>/dev/null || true
sudo systemctl start localdns-fallback.service 2>/dev/null || true
sleep 4
[ "$(systemctl is-active localdns-fallback.service)" != "active" ] \
  && ok "GUARD: fallback declined to start while localdns owns ${CLUSTER_IP}" \
  || fail "GUARD: fallback started while localdns owns ${CLUSTER_IP}"
[ "$(systemctl show localdns-fallback.service -p NRestarts --value)" = "0" ] \
  && ok "GUARD: declined start did not consume a restart (no hot loop)" \
  || fail "GUARD: fallback restarted $(systemctl show localdns-fallback.service -p NRestarts --value) time(s)"
[ "$(owner_of_11)" = "$before" ] \
  && ok "GUARD: localdns kept ${CLUSTER_IP} undisturbed" \
  || fail "GUARD: ${CLUSTER_IP} owner changed during the declined start"

# --- Guard: the probe must fail OPEN when dig is absent. dig is not installed by
# the VHD build on every image, and reading "cannot measure" as ".11 is dark" would
# start the fallback against a healthy localdns on every tick. ---
out=$(sudo env PATH=/nonexistent /bin/bash /opt/azure/containers/localdns/localdns-fallback-probe.sh 2>&1); rc=$?
[ "$rc" -eq 0 ] && echo "$out" | grep -q "dig is not available" \
  && ok "GUARD: probe no-ops when dig is unavailable (rc=0)" \
  || fail "GUARD: probe without dig returned rc=${rc}: ${out}"
[ "$(systemctl is-active localdns-fallback.service)" != "active" ] \
  && ok "GUARD: probe without dig did not start the fallback" \
  || fail "GUARD: probe without dig started the fallback"
finish
`, pod1, pod3))
}
