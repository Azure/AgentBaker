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
// bounded waiter, and helpers for inspecting the fallback's nat chain. Each phase
// is a separate SSH invocation, so this cannot live in one long-lived script.
//
// The shebang is load-bearing. execScriptOnVMForScenario SCPs any multi-line script
// to the node and executes the file (exec.go: copyScriptToRemoteIfRequired), so
// without it the node runs these with /bin/sh (dash), which rejects 'pipefail' and
// every other bashism below with "Illegal option -o pipefail".
//
// That helper runs the script as azureuser. These phases drive systemd units,
// iptables and /etc/localdns, so the prelude re-execs once as root rather than
// prefixing every line below with sudo -- which is also what keeps the reads
// (the environment file, journalctl) working regardless of their modes.
const localDNSFallbackPrelude = `#!/bin/bash
[ "$(id -u)" -eq 0 ] || exec sudo -E "$0" "$@"
set -uo pipefail
CLUSTER_IP=169.254.10.11
NODE_IP=169.254.10.10
CHAIN=LOCALDNS-FALLBACK
FAULT_DIR=/run/systemd/system/localdns.service.d
FAULT=$FAULT_DIR/zz-e2e-fault.conf
KUBELET_DEFAULT_FILE=/etc/default/kubelet
RESOLV=/run/systemd/resolve/resolv.conf
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

# The fallback installs no listener, so there is no socket to attribute. Its
# entire observable state is the nat chain and the two jumps into it.
chain_exists()  { sudo iptables -w -t nat -S "$CHAIN" >/dev/null 2>&1; }
chain_rules()   { sudo iptables -w -t nat -S "$CHAIN" 2>/dev/null; }
hook_jumps()    { sudo iptables -w -t nat -S "$1" 2>/dev/null | grep -c -- "-j ${CHAIN}"; }
vnet_dns()      { awk -v self="$NODE_IP" '$1 == "nameserver" && $2 != self { print $2; exit }' "$RESOLV" 2>/dev/null; }

diag() {
  echo "fallback-e2e: ---- diagnostics: $* ----"
  echo "  localdns=$(systemctl is-active localdns.service)/$(systemctl is-failed localdns.service) fallback=$(systemctl is-active localdns-fallback.service)"
  echo "  kubelet --cluster-dns=$(kubelet_cluster_dns)"
  echo "  resolv upstream=$(vnet_dns)  [$(grep -c '^nameserver' "$RESOLV" 2>/dev/null) nameserver lines]"
  echo "  --- nat chain ---"; chain_rules | sed 's/^/    /' || echo "    (absent)"
  echo "  PREROUTING jumps=$(hook_jumps PREROUTING)  OUTPUT jumps=$(hook_jumps OUTPUT)"
  echo "  node dig @.11: [$(dig +short +timeout=3 +tries=1 kubernetes.default.svc.cluster.local "@${CLUSTER_IP}" 2>&1 | tr '\n' ' ')]"
  echo "  node dig @.10: [$(dig +short +timeout=3 +tries=1 mcr.microsoft.com "@${NODE_IP}" 2>&1 | tr '\n' ' ')]"
  echo "  --- fallback journal ---"; journalctl -u localdns-fallback.service --since "-5min" --no-pager -o cat 2>/dev/null | tail -12 | sed 's/^/    /'
  echo "  --- localdns journal ---"; journalctl -u localdns.service --since "-5min" --no-pager -o cat 2>/dev/null | tail -8 | sed 's/^/    /'
  echo "fallback-e2e: ---- end diagnostics ----"
}

# Resolve from inside a pod's network namespace. Both the netns AND an explicit
# server are required: 'nsenter -n' changes only the network namespace, so a bare
# dig would read the HOST's /etc/resolv.conf and silently measure the wrong
# resolver.
#
# Exit codes are load-bearing throughout this group:
#
#   0  the query ran and resolved
#   1  the query ran and did not resolve   <- a DNS failure, what we are testing
#   2  we never got as far as querying     <- a broken probe, says nothing about DNS
#
# Collapsing 2 into 1 reports a crictl/nsenter problem as a pod DNS outage, which
# is the exact conclusion this scenario exists to draw. PROBE_ERR carries the
# detail so the failure names what actually went wrong.
PROBE_ERR=""

pod_netns() {
  local sandbox ns
  sandbox=$(sudo crictl pods --name "$1" --state Ready -q 2>/dev/null | head -1)
  if [ -z "$sandbox" ]; then PROBE_ERR="crictl found no Ready sandbox for pod $1"; return 2; fi
  # Assigned, then tested. Ending the function on a pipeline would return head's
  # status, which is 0 even when grep matched nothing, so an empty netns would
  # reach nsenter as '--net=' and fail there instead of here.
  ns=$(sudo crictl inspectp "$sandbox" 2>/dev/null | grep -oE '/var/run/netns/[A-Za-z0-9._-]+' | head -1)
  if [ -z "$ns" ]; then PROBE_ERR="crictl inspectp reported no netns path for pod $1 (sandbox $sandbox)"; return 2; fi
  printf '%s' "$ns"
}

pod_nameserver() {
  local sandbox
  sandbox=$(sudo crictl pods --name "$1" --state Ready -q 2>/dev/null | head -1)
  [ -z "$sandbox" ] && return 1
  sudo grep -oE '^nameserver .*' \
    /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/"$sandbox"/resolv.conf 2>/dev/null \
    | head -1 | awk '{print $2}'
}

# <pod> <server> <fqdn> <answer-pattern>
pod_digs() {
  local ns out
  ns=$(pod_netns "$1") || return 2
  # Prove the namespace is enterable before attributing anything to DNS. Without
  # this, an nsenter that cannot run at all is indistinguishable from a dig that
  # ran and got no answer.
  sudo nsenter --net="$ns" true 2>/dev/null || {
    PROBE_ERR="cannot enter netns $ns for pod $1"; return 2; }
  out=$(sudo nsenter --net="$ns" dig +short +timeout=3 +tries=1 "$3" "@$2" 2>/dev/null)
  printf '%s\n' "$out" | grep -qE "$4" && return 0
  PROBE_ERR="dig $3 @$2 from pod $1 returned: [$(printf '%s' "$out" | tr '\n' ' ')]"
  return 1
}

pod_resolves_cluster_via()  { pod_digs "$1" "$2" kubernetes.default.svc.cluster.local '^[0-9]+\.'; }
pod_resolves_external_via() { pod_digs "$1" "$2" mcr.microsoft.com '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+'; }

# Poll until a lookup succeeds, then report by cause rather than by outcome. A
# probe that never ran is not evidence that DNS is down, and must not be recorded
# as though it were.
# <secs> <desc> <fn> <args...>
expect_resolves() {
  local secs="$1" desc="$2"; shift 2
  local i=0 rc=2
  while [ "$i" -lt "$secs" ]; do
    "$@"; rc=$?
    [ "$rc" -eq 0 ] && { ok "${desc} (after ${i}s)"; return 0; }
    sleep 1; i=$((i+1))
  done
  diag "$desc"
  if [ "$rc" -eq 2 ]; then
    fail "PROBE ERROR: ${desc} -- could not query from inside the pod, so this says nothing about DNS: ${PROBE_ERR}"
  else
    fail "${desc}: ${PROBE_ERR}"
  fi
  return 1
}

# hostNetwork pods share the host netns, so query directly.
host_resolves_external_via() { dig +short +timeout=3 +tries=1 mcr.microsoft.com "@$1" 2>/dev/null | grep -qE '[0-9]+\.'; }
`

// localDNSFallbackArtifacts are the files this scenario needs on the node. They are
// baked into the VHD by vhdbuilder/packer/packer_source.sh rather than delivered at
// provision time, so whether they are present is a property of the image.
var localDNSFallbackArtifacts = []string{
	"/etc/systemd/system/localdns-fallback.service",
	"/opt/azure/containers/localdns/localdns-fallback.sh",
}

func localDNSProbePod(s *Scenario, suffix string, policy corev1.DNSPolicy, hostNetwork bool) *corev1.Pod {
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
			DNSPolicy:   policy,
			HostNetwork: hostNetwork,
			Containers: []corev1.Container{
				{Name: "probe", Image: image, Command: []string{"sleep", "infinity"}},
			},
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
		},
	}
}

func startLocalDNSProbePod(ctx context.Context, s *Scenario, suffix string, policy corev1.DNSPolicy, hostNetwork bool) (string, func(), error) {
	kube := s.Runtime.Kube
	pod := localDNSProbePod(s, suffix, policy, hostNetwork)
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
	logging.Logf(ctx, "localdns probe pod %q (%s, hostNetwork=%v) is running", created.Name, policy, hostNetwork)
	return created.Name, del, nil
}

// ValidateLocalDNSFallbackRecovery exercises the kernel-NAT fallback end to end on
// a node whose localdns has reached terminal 'failed'.
//
// The fallback runs no CoreDNS and holds no socket. It installs one nat chain that
// redirects both listeners, so every assertion here is about iptables state and
// about what pods can actually resolve -- never about who owns a port.
//
// Both listeners matter, and this is the point the .11-only design missed. A pod's
// /etc/resolv.conf is written once at sandbox creation and never revisited, and
// which address it gets depends on its dnsPolicy:
//
//	ClusterFirst                     -> 169.254.10.11
//	Default, hostNetwork+ClusterFirst -> 169.254.10.10
//
// The second group includes CoreDNS itself, konnectivity-agent, the CSI node
// drivers, kube-proxy and azure-cns. Leaving .10 dark black-holes all of them, and
// also breaks external resolution for .11 clients whose query lands on a CoreDNS
// replica running on this same node -- that replica resolves through its own .10.
//
// Fault injection overrides ExecStart with /bin/false, the same fault
// scenario_localdns_restart_budget.go uses, so the unit spends its burst and
// lands in terminal 'failed' in ~12s. Note what does NOT work: corrupting
// localdns.corefile, because localdns.sh rebuilds it from the base64 in
// /etc/localdns/environment on every start, so the node self-heals and never
// breaks.
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) (err error) {
	// The artifacts ship in the VHD (vhdbuilder/packer/packer_source.sh and, for
	// OSGuard images, azlosguard.yml), not through CustomData, so their absence is a
	// packaging failure and this scenario says so rather than working around it.
	//
	// This replaces staging the branch's copies onto the node. Staging let the
	// scenario run before the artifacts shipped, but once they do it is a mask: it
	// repaired the exact gap the scenario exists to detect and then reported a pass --
	// which is why the OSGuard mode split was invisible here. Modes are asserted by
	// checkLocaldnsScriptsAndConfigs in vhdbuilder/packer/test/linux-vhd-content-test.sh,
	// against the image itself.
	//
	// Deliberately NOT gated on whether the lane resolved a main-built image. Until a
	// main VHD carrying these is published, the standalone e2e lane goes red; that is
	// bounded by one VHD release and self-clearing. A gate would trade that temporary
	// red for a permanent one: "this image is older than the feature" and "this image
	// was built without the feature" look identical from here, so forgiving the first
	// forgives the second, and a packaging regression that actually shipped to main
	// would be skipped instead of reported.
	for _, f := range localDNSFallbackArtifacts {
		if err := ValidateFileExists(ctx, s, f); err != nil {
			return fmt.Errorf("the localdns fallback artifacts are not baked into this VHD: %w", err)
		}
	}

	phase := func(name, script string) error {
		res, err := execScriptOnVMForScenario(ctx, s, localDNSFallbackPrelude+script)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		logging.Logf(ctx, "localdns fallback e2e %s output:\n%s", name, res.stdout)
		if res.exitCode != "0" {
			return fmt.Errorf("%s failed (exit %s)\nstdout: %s\nstderr: %s", name, res.exitCode, res.stdout, res.stderr)
		}
		return nil
	}

	// Always put the node back, even if a phase fails part-way. A node left with a
	// corrupted localdns environment or a live nat chain would poison every later
	// scenario sharing it -- so a restore that does not land is a failure of this
	// scenario, not a log line. It only overwrites err when the test itself passed:
	// the original failure is the more useful one to report.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		restore := localDNSFallbackPrelude + `
rm -f "$FAULT"
systemctl daemon-reload
systemctl stop localdns-fallback.service 2>/dev/null || true
/opt/azure/containers/localdns/localdns-fallback.sh clear 2>/dev/null || true
systemctl reset-failed localdns.service 2>/dev/null || true
systemctl start localdns.service 2>/dev/null || true

# Wait for the state the next scenario needs, rather than sleeping a fixed 10s and
# reporting whatever happened to be true at that moment.
wait_for 120 "localdns healthy again after the test" \
  sh -c 'systemctl is-active --quiet localdns.service' || { diag "restore: localdns not healthy"; }
wait_for 30 "fallback nat chain gone after the test" \
  sh -c '! iptables -w -t nat -S LOCALDNS-FALLBACK >/dev/null 2>&1' || { diag "restore: chain survived"; }
systemctl reset-failed localdns-fallback.service 2>/dev/null || true
finish
`
		res, cerr := execScriptOnVMForScenario(cleanupCtx, s, restore)
		if res != nil {
			logging.Logf(ctx, "localdns fallback e2e restore output:\n%s", res.stdout)
		}
		switch {
		case cerr != nil:
			cerr = fmt.Errorf("localdns fallback e2e: node restore failed: %w", cerr)
		case res.exitCode != "0":
			cerr = fmt.Errorf("localdns fallback e2e: node restore failed (exit %s): the node is left "+
				"without a healthy localdns or with fallback rules still installed\nstdout: %s\nstderr: %s",
				res.exitCode, res.stdout, res.stderr)
		default:
			return
		}
		if err == nil {
			err = cerr
		} else {
			logging.Logf(ctx, "%v (reporting the earlier test failure instead)", cerr)
		}
	}()

	// ---- Probe pods, created BEFORE the fault --------------------------------
	// Their resolv.conf is written now and never revisited; that is the whole
	// reason the fallback has to exist.
	podCF, delCF, err := startLocalDNSProbePod(ctx, s, "clusterfirst", corev1.DNSClusterFirst, false)
	if err != nil {
		return fmt.Errorf("create ClusterFirst probe pod: %w", err)
	}
	defer delCF()

	podDefault, delDefault, err := startLocalDNSProbePod(ctx, s, "default", corev1.DNSDefault, false)
	if err != nil {
		return fmt.Errorf("create Default probe pod: %w", err)
	}
	defer delDefault()

	// ---- Phase 1: baseline, then induce an unrecoverable failure -------------
	if err := phase("phase1-induce-failure", fmt.Sprintf(`
POD_CF=%q
POD_DEF=%q

[ "$(pod_nameserver "$POD_CF")" = "$CLUSTER_IP" ] \
  && ok "baseline: ClusterFirst pod is pinned to ${CLUSTER_IP}" \
  || fail "baseline: ClusterFirst pod nameserver is $(pod_nameserver "$POD_CF")"
[ "$(pod_nameserver "$POD_DEF")" = "$NODE_IP" ] \
  && ok "baseline: Default pod is pinned to ${NODE_IP}" \
  || fail "baseline: Default pod nameserver is $(pod_nameserver "$POD_DEF")"

# Same fault the restart-budget validation uses (scenario_localdns_restart_budget.go):
# override ExecStart so every start fails immediately. Five starts at RestartSec=2
# and the refused sixth reach terminal 'failed' in ~12s, against ~69s for a
# malformed Corefile, which has to run the real startup path to its timeout on
# every attempt.
#
# It drives the fallback's gate identically. 'restart' stops the healthy localdns
# first, so its ExecStopPost runs the real 'localdns.sh cleanup': the .10 drop-in
# comes off, networkctl reload puts the DHCP upstream back, and the raw-table
# NOTRACK rules go. Those are exactly the three things apply needs -- terminal
# 'failed', a non-.10 upstream, and conntrack restored for the listeners.
#
# The shorter cycle also tightens the race the fallback's resolv.conf poll exists
# for: OnFailure= fires as soon as ExecStopPost returns, while networkctl reload
# is still asynchronous. Reaching 'failed' in 12s rather than 69s gives that poll
# less slack, not more.
#
# /run, so a reboot cannot leave the fault behind; zz- so it sorts after anything
# shipped in the unit directory.
mkdir -p "$FAULT_DIR"
printf '[Service]\nExecStart=\nExecStart=/bin/false\n' > "$FAULT"
systemctl daemon-reload
log "injected fault; restarting localdns"
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl restart --no-block localdns.service

wait_for 120 "localdns reaches terminal 'failed'" sh -c 'systemctl is-failed --quiet localdns.service' \
  || { diag "no terminal failed"; fail "localdns never reached terminal 'failed'"; finish; }
ok "localdns reached terminal 'failed'"

# localdns's ExecStopPost removes the resolv.conf drop-in and reloads networkd,
# which is what puts a usable upstream back for the .10 redirect. The reload is
# asynchronous, so apply polls for it rather than reading once.
wait_for 30 "DHCP resolvers restored after localdns exit" sh -c '[ -n "$(awk '"'"'$1 == "nameserver" && $2 != "169.254.10.10" { print $2; exit }'"'"' /run/systemd/resolve/resolv.conf 2>/dev/null)" ]' \
  && ok "an upstream other than ${NODE_IP} is back in ${RESOLV}: $(vnet_dns)" \
  || { diag "no upstream"; fail "resolv.conf never regained a usable upstream"; }

wait_for 60 "fallback active" sh -c 'systemctl is-active --quiet localdns-fallback.service' \
  || { diag "fallback not active"; fail "OnFailure= did not bring the fallback up"; finish; }
ok "OnFailure= started the fallback"
finish
`, podCF, podDefault)); err != nil {
		return err
	}

	// ---- Phase 2: the rules themselves ---------------------------------------
	if err := phase("phase2-nat-rules", `
chain_exists && ok "nat chain ${CHAIN} exists" || { diag "no chain"; fail "nat chain ${CHAIN} missing"; finish; }

R=$(chain_rules)
log "chain contents:"; echo "$R" | sed 's/^/    /'
echo "$R" | grep -qE -- "-d ${NODE_IP}/32 -p udp .*--dport 53 .*-j DNAT" \
  && ok "node listener redirected over udp" || fail "no udp DNAT for ${NODE_IP}"
echo "$R" | grep -qE -- "-d ${NODE_IP}/32 -p tcp .*--dport 53 .*-j DNAT" \
  && ok "node listener redirected over tcp" || fail "no tcp DNAT for ${NODE_IP}"
echo "$R" | grep -q -- "-d ${NODE_IP}/32 .* --to-destination $(vnet_dns):53" \
  && ok "node listener points at the VNet DNS $(vnet_dns)" || fail "node listener does not point at $(vnet_dns)"

echo "$R" | grep -q -- "-d ${CLUSTER_IP}/32" \
  && ok "cluster listener is redirected" || fail "no rule for ${CLUSTER_IP}"

# The ClusterIP is deliberately never a target: the nat table is traversed once per
# connection and DNAT is terminal, so rewriting .11 to the ClusterIP would consume
# the traversal KUBE-SERVICES needed to translate it into a pod IP. Jump into
# kube-proxy's own service chain, or DNAT straight at the backends on Cilium.
if echo "$R" | grep -qE -- "-d ${CLUSTER_IP}/32 -p udp .*--dport 53 .*-j KUBE-SVC-"; then
  ok "cluster listener jumps into kube-proxy's kube-dns chain"
elif echo "$R" | grep -qE -- "-d ${CLUSTER_IP}/32 -p udp .*--dport 53 .*-j DNAT"; then
  ok "cluster listener DNATs across cilium backends"
else
  diag "cluster listener rule shape"
  fail "cluster listener has neither a KUBE-SVC jump nor backend DNATs"
fi

[ "$(hook_jumps PREROUTING)" -ge 1 ] && ok "chain is hooked into PREROUTING" || fail "no PREROUTING jump"
[ "$(hook_jumps OUTPUT)" -ge 1 ] && ok "chain is hooked into OUTPUT" || fail "no OUTPUT jump"

# DNAT needs conntrack, and localdns's raw-table NOTRACK rules for the listeners run
# first -- raw is priority -300, nat is -100 -- so an untracked packet skips the nat
# table entirely. If those rules survived into the outage the whole chain above is
# inert for pod traffic while still looking perfectly correct in 'iptables -S'.
nt=$(sudo iptables -w -t raw -S 2>/dev/null | grep -cE -- "-d 169\.254\.10\.(10|11)/32 .*-j NOTRACK")
[ "$nt" -eq 0 ] \
  && ok "localdns NOTRACK rules are gone, so the redirect can take effect" \
  || { diag "NOTRACK survived"; fail "${nt} NOTRACK rule(s) for the listeners survived; the nat redirect is inert for pod traffic"; }
finish
`); err != nil {
		return err
	}

	// ---- A pod created DURING the outage -------------------------------------
	podDuring, delDuring, err := startLocalDNSProbePod(ctx, s, "during", corev1.DNSClusterFirst, false)
	if err != nil {
		return fmt.Errorf("create during-failure probe pod: %w", err)
	}
	defer delDuring()

	// ---- Phase 3: what pods can actually resolve -----------------------------
	if err := phase("phase3-pod-resolution", fmt.Sprintf(`
POD_CF=%q
POD_DEF=%q
POD_DURING=%q

# .11 clients: both cluster and external names. External matters because the query
# goes .11 -> a CoreDNS replica -> that replica's own .10; if .10 were dark this
# would fail even though .11 is covered.
[ "$(pod_nameserver "$POD_CF")" = "$CLUSTER_IP" ] \
  && ok "pre-failure ClusterFirst pod is still pinned to ${CLUSTER_IP}" \
  || fail "pre-failure pod nameserver changed to $(pod_nameserver "$POD_CF")"
expect_resolves 45 "pre-failure pod resolves a cluster name through the fallback" \
  pod_resolves_cluster_via "$POD_CF" "$CLUSTER_IP"
expect_resolves 5 "pre-failure pod resolves an external name through the fallback" \
  pod_resolves_external_via "$POD_CF" "$CLUSTER_IP"

# .10 clients -- the group the .11-only design black-holed. CoreDNS itself,
# konnectivity-agent and the CSI node drivers all live here.
[ "$(pod_nameserver "$POD_DEF")" = "$NODE_IP" ] \
  && ok "Default-policy pod is pinned to ${NODE_IP}" \
  || fail "Default-policy pod nameserver is $(pod_nameserver "$POD_DEF")"
expect_resolves 45 "Default-policy pod resolves an external name through the fallback" \
  pod_resolves_external_via "$POD_DEF" "$NODE_IP"

# The node itself, and every hostNetwork pod, uses .10 the same way.
host_resolves_external_via "$NODE_IP" \
  && ok "node resolves an external name through ${NODE_IP}" \
  || { diag "node external"; fail "node cannot resolve through ${NODE_IP}"; }

# A pod born during the outage gets the same treatment as one that predates it.
[ "$(pod_nameserver "$POD_DURING")" = "$CLUSTER_IP" ] \
  && ok "during-failure pod was born with nameserver ${CLUSTER_IP}" \
  || fail "during-failure pod nameserver is $(pod_nameserver "$POD_DURING")"
expect_resolves 45 "during-failure pod resolves a cluster name through the fallback" \
  pod_resolves_cluster_via "$POD_DURING" "$CLUSTER_IP"
finish
`, podCF, podDefault, podDuring)); err != nil {
		return err
	}

	// ---- Phase 4: recovery ---------------------------------------------------
	if err := phase("phase4-recovery", `
rm -f "$FAULT"
systemctl daemon-reload
log "fault removed; restarting localdns"
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemd-run --unit=localdns-e2e-recover --collect /bin/systemctl restart localdns.service >/dev/null 2>&1 || true

wait_for 180 "localdns active again" sh -c 'systemctl is-active --quiet localdns.service' \
  || { diag "no recovery"; fail "localdns did not come back"; finish; }
ok "localdns is serving again"

# localdns.service's ExecStartPre stops the fallback, whose ExecStop flushes the
# chain. If that handoff did not happen, traffic to .10/.11 would keep being
# rewritten past the listeners localdns just bound.
wait_for 30 "nat chain removed" sh -c '! sudo iptables -w -t nat -S LOCALDNS-FALLBACK >/dev/null 2>&1' \
  || { diag "chain survived"; fail "nat chain survived localdns recovery"; finish; }
ok "RECOVERY: the fallback stood down and its nat chain is gone"
[ "$(hook_jumps PREROUTING)" -eq 0 ] && [ "$(hook_jumps OUTPUT)" -eq 0 ] \
  && ok "RECOVERY: no jumps left in PREROUTING or OUTPUT" || fail "RECOVERY: jumps survived"

# A clean stand-down must not be recorded as a failure. Nothing clears that state --
# localdns's ExecStartPre runs 'stop', which is not 'reset-failed' -- so a unit left
# 'failed' here stays failed on a healthy node until reboot.
systemctl is-failed --quiet localdns-fallback.service \
  && { diag "fallback failed after recovery"; fail "RECOVERY: fallback unit is 'failed' after a clean handoff"; } \
  || ok "RECOVERY: fallback unit is not 'failed' (state=$(systemctl is-active localdns-fallback.service))"

wait_for 60 "localdns answers on .11 again" sh -c 'dig +short +timeout=3 +tries=1 kubernetes.default.svc.cluster.local @169.254.10.11 | grep -qE "^[0-9]+\."' \
  && ok "RECOVERY: localdns answers on ${CLUSTER_IP}" || fail "RECOVERY: ${CLUSTER_IP} silent after recovery"
finish
`); err != nil {
		return err
	}

	// ---- Phase 5: the safety gate --------------------------------------------
	return phase("phase5-guards", `
# localdns is healthy now, so apply must refuse. Redirecting here would silently
# take traffic away from a working listener.
out=$(sudo /opt/azure/containers/localdns/localdns-fallback.sh apply 2>&1); rc=$?
[ "$rc" -ne 0 ] && ok "GUARD: apply refuses while localdns is active (rc=${rc})" \
  || fail "GUARD: apply succeeded while localdns was active"
case "$out" in
  *"not failed; not redirecting"*) ok "GUARD: refusal names the reason" ;;
  *) fail "GUARD: unexpected refusal message: ${out}" ;;
esac
chain_exists && fail "GUARD: refused apply still created the chain" \
  || ok "GUARD: refused apply left no chain behind"

# The same refusal, but through systemd rather than by running the script directly.
# This is what OnFailure= actually does on every start while localdns works through
# its restart budget, and it is the only assertion here that can tell whether
# ExecCondition= is doing anything: without it systemd runs apply, apply exits 1,
# and the unit lands in 'failed' on a node whose localdns is perfectly healthy.
sudo systemctl reset-failed localdns-fallback.service 2>/dev/null || true
# No sleep afterwards: the unit is Type=oneshot, so 'systemctl start' blocks until it
# has finished (or been skipped by ExecCondition). Its state is settled on return.
sudo systemctl start localdns-fallback.service >/dev/null 2>&1 || true
if systemctl is-failed --quiet localdns-fallback.service; then
  diag "gated start left the unit failed"
  fail "GUARD: starting the unit with localdns healthy left it 'failed'; ExecCondition did not skip it"
else
  ok "GUARD: starting the unit with localdns healthy is skipped, not failed (state=$(systemctl is-active localdns-fallback.service))"
fi
chain_exists && fail "GUARD: the skipped start still installed a chain" \
  || ok "GUARD: the skipped start installed nothing"
sudo systemctl stop localdns-fallback.service >/dev/null 2>&1 || true
sudo systemctl reset-failed localdns-fallback.service 2>/dev/null || true

# clear is idempotent: recovery already ran it via ExecStop.
sudo /opt/azure/containers/localdns/localdns-fallback.sh clear >/dev/null 2>&1 \
  && ok "GUARD: clear is idempotent" || fail "GUARD: clear failed when there was nothing to clear"
finish
`)
}
