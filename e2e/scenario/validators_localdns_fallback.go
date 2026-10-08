package scenario

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
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
// The shebang is load-bearing: these are uploaded as files and executed, and
// without it the node runs them with /bin/sh (dash), which rejects 'pipefail' and
// every other bashism below with "Illegal option -o pipefail".
const localDNSFallbackPrelude = `#!/bin/bash
set -uo pipefail
CLUSTER_IP=169.254.10.11
NODE_IP=169.254.10.10
CHAIN=LOCALDNS-FALLBACK
ENVF=/etc/localdns/environment
ENVBAK=/run/localdns-fallback-e2e-env.bak
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
pod_resolves_cluster_via() {   # <pod> <server>
  local ns; ns=$(pod_netns "$1") || return 1
  sudo nsenter --net="$ns" dig +short +timeout=3 +tries=1 \
    kubernetes.default.svc.cluster.local "@$2" 2>/dev/null | grep -qE '^[0-9]+\.'
}
pod_resolves_external_via() {
  local ns; ns=$(pod_netns "$1") || return 1
  sudo nsenter --net="$ns" dig +short +timeout=3 +tries=1 \
    mcr.microsoft.com "@$2" 2>/dev/null | grep -qE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+'
}
# hostNetwork pods share the host netns, so query directly.
host_resolves_external_via() { dig +short +timeout=3 +tries=1 mcr.microsoft.com "@$1" 2>/dev/null | grep -qE '[0-9]+\.'; }
`

// vhdHasLocalDNSFallbackArtifacts reports whether the VHD under test already has
// the fallback artifacts baked in. The pipeline runs against VHDs built from main,
// which will not include them until this PR merges, so the scenario stages them
// rather than skipping.
func vhdHasLocalDNSFallbackArtifacts(ctx context.Context, s *Scenario) (bool, error) {
	result, err := execScriptOnVMForScenario(ctx, s,
		"test -f /etc/systemd/system/localdns-fallback.service && "+
			"test -x /opt/azure/containers/localdns/localdns-fallback.sh")
	if err != nil {
		return false, fmt.Errorf("failed to check for localdns fallback artifacts on the VM: %w", err)
	}
	return result.exitCode == "0", nil
}

func uploadAndRunOnVM(ctx context.Context, s *Scenario, content, remotePath string, runCmd func(remote string) string, label string) (*podExecResult, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	remoteB64 := remotePath + ".b64"
	const chunkSize = 4096
	for i := 0; i < len(encoded); i += chunkSize {
		end := min(i+chunkSize, len(encoded))
		redirect := ">>"
		if i == 0 {
			redirect = ">"
		}
		// Each chunk appends to the last, so a failed chunk leaves a truncated file:
		// abort rather than continue.
		cmd := fmt.Sprintf("echo -n '%s' %s %s", encoded[i:end], redirect, remoteB64)
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, cmd, 0,
			fmt.Sprintf("%s: upload chunk at offset %d", label, i)); err != nil {
			return nil, err
		}
	}
	decode := fmt.Sprintf("base64 -d %s > %s && chmod +x %s && rm -f %s", remoteB64, remotePath, remotePath, remoteB64)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, decode, 0, label+": decode uploaded file"); err != nil {
		return nil, err
	}
	run := runCmd(remotePath)
	if run == "" {
		return nil, nil
	}
	res, err := execScriptOnVMForScenario(ctx, s, run)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return res, nil
}

// localDNSFallbackArtifacts are the files this PR adds to or changes in the VHD.
// Kept in one place so staging and the presence check cannot drift apart.
//
// localdns.sh and localdns.service must be staged as a PAIR with the fallback.
// The unit's ExecStopPost calls 'localdns.sh cleanup', a mode older scripts do not
// have, and ExecStopPost carries no '-' prefix -- so a mismatched pair fails on
// every stop and leaves the unit 'failed' instead of 'inactive'.
var localDNSFallbackArtifacts = []struct {
	repoFile string
	dest     string
	mode     string
}{
	{"localdns.sh", "/opt/azure/containers/localdns/localdns.sh", "0755"},
	{"localdns.service", "/etc/systemd/system/localdns.service", "0644"},
	{"localdns-fallback.service", "/etc/systemd/system/localdns-fallback.service", "0644"},
	{"localdns-fallback.sh", "/opt/azure/containers/localdns/localdns-fallback.sh", "0755"},
}

// stageLocalDNSFallbackArtifacts installs this branch's localdns artifacts onto
// the node.
//
// Without it this scenario can never run before the PR merges: the files are baked
// into the VHD by packer rather than delivered at provision time, so
// vhdHasLocalDNSFallbackArtifacts is false on every pipeline run and the scenario
// would first execute only after merging, which is exactly backwards. It is a
// no-op once the artifacts ship.
func stageLocalDNSFallbackArtifacts(ctx context.Context, s *Scenario) error {
	// One small command per file. A single command carrying every file inline is
	// ~150KB of base64, and execScriptOnVMForScenario scp's the script under a 10s
	// timeout (exec.go), so a large payload over a tunneled connection fails as
	// "remote command exited without exit status or exit signal" -- a transport
	// error that says nothing about what went wrong. Per-file also means a failure
	// names the file it failed on.
	for _, a := range localDNSFallbackArtifacts {
		src := repoPath(filepath.Join("parts", "linux", "cloud-init", "artifacts", a.repoFile))
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s for staging: %w", src, err)
		}
		staged := "/home/azureuser/stage_" + a.repoFile
		install := func(remote string) string {
			return fmt.Sprintf("sudo mkdir -p $(dirname %s) && sudo cp %s %s && sudo chmod %s %s && test -s %s && echo staged %s",
				a.dest, remote, a.dest, a.mode, a.dest, a.dest, a.dest)
		}
		res, err := uploadAndRunOnVM(ctx, s, string(content), staged, install, "stage "+a.repoFile)
		if err != nil {
			return fmt.Errorf("stage %s: %w", a.repoFile, err)
		}
		if res.exitCode != "0" {
			return fmt.Errorf("stage %s: exit %s: %s", a.repoFile, res.exitCode, res.stderr)
		}
	}

	// localdns.service was replaced, so the loaded unit is stale until reloaded.
	// Deliberately NO restart here: see restartLocalDNSDetached below.
	reload := `set -euo pipefail
sudo systemctl daemon-reload
echo "staged: daemon reloaded"`
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, reload, 0,
		"reload systemd after staging localdns fallback artifacts"); err != nil {
		return fmt.Errorf("reload after staging: %w", err)
	}
	return restartLocalDNSDetached(ctx, s)
}

// restartLocalDNSDetached restarts localdns without holding the SSH session open
// across the restart, then waits for it to come back in separate calls.
//
// Restarting localdns inline is what a straightforward implementation does, and it
// fails: localdns owns the node resolver, so the restart tears down DNS underneath
// the connection running it and the channel closes with "remote command exited
// without exit status or exit signal" -- the command's real result is lost, and the
// caller sees a transport error rather than anything about localdns. Detaching the
// restart keeps the disruptive part off the connection, and the readiness poll is
// retried because those probes can themselves land in the DNS-down window.
func restartLocalDNSDetached(ctx context.Context, s *Scenario) error {
	kick := `set -uo pipefail
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemd-run --unit=localdns-e2e-restart --collect /bin/systemctl restart localdns.service >/dev/null 2>&1 || true
echo kicked`
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, kick, 0, "kick localdns restart"); err != nil {
		return fmt.Errorf("kick localdns restart: %w", err)
	}
	var lastErr error
	for i := 0; i < 12; i++ {
		time.Sleep(10 * time.Second)
		res, err := execScriptOnVMForScenario(ctx, s, "systemctl is-active --quiet localdns.service && echo up || echo down")
		if err != nil {
			lastErr = err
			continue
		}
		if res.exitCode == "0" && res.stdout != "" && res.stdout[0] == 'u' {
			logging.Logf(ctx, "localdns is active again after staging")
			return nil
		}
	}
	return fmt.Errorf("localdns did not come back after staging: %w", lastErr)
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
// Fault injection corrupts LOCALDNS_COREFILE_BASE in /etc/localdns/environment.
// Corrupting localdns.corefile does NOT work: localdns.sh rebuilds it from the
// base64 on every start, so it self-heals and the node never breaks.
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) error {
	hasArtifacts, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
	if err != nil {
		return fmt.Errorf("detect localdns fallback artifacts: %w", err)
	}
	if !hasArtifacts {
		logging.Logf(ctx, "VHD lacks the localdns fallback artifacts; staging this branch's copies")
		if err := stageLocalDNSFallbackArtifacts(ctx, s); err != nil {
			return fmt.Errorf("stage localdns fallback artifacts: %w", err)
		}
		staged, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
		if err != nil {
			return fmt.Errorf("re-check fallback artifacts after staging: %w", err)
		}
		if !staged {
			return fmt.Errorf("localdns fallback artifacts still absent after staging")
		}
	}

	phase := func(name, script string) error {
		remote := "/home/azureuser/localdns_fallback_" + name + ".sh"
		res, err := uploadAndRunOnVM(ctx, s, localDNSFallbackPrelude+script, remote,
			func(r string) string { return "sudo " + r }, "localdns fallback e2e: "+name)
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
	// scenario sharing it.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		restore := localDNSFallbackPrelude + `
[ -f "$ENVBAK" ] && sudo cp -a "$ENVBAK" "$ENVF"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo /opt/azure/containers/localdns/localdns-fallback.sh clear 2>/dev/null || true
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl start localdns.service 2>/dev/null || true
sleep 10
log "post-test state: localdns=$(systemctl is-active localdns.service) chain=$(chain_exists && echo present || echo absent)"
exit 0
`
		if _, err := uploadAndRunOnVM(cleanupCtx, s, restore, "/home/azureuser/localdns_fallback_restore.sh",
			func(r string) string { return "sudo " + r }, "localdns fallback e2e: restore"); err != nil {
			logging.Logf(ctx, "localdns fallback e2e: node restore failed: %v", err)
		}
	}()

	// ---- Phase 0: wiring -----------------------------------------------------
	if err := phase("phase0-wiring", `
systemctl show localdns.service -p OnFailure 2>/dev/null | grep -q "OnFailure=localdns-fallback.service" \
  && ok "localdns declares OnFailure=localdns-fallback.service" || fail "localdns missing OnFailure="

# Conflicts= would be resolved by STOPPING localdns, and a stop resets its restart
# counter -- so it would never spend the 720/5 budget and never reach terminal
# 'failed', defeating both the handoff and the state NPD observes.
conflicts=$(systemctl show localdns-fallback.service -p Conflicts --value 2>/dev/null || true)
case "$conflicts" in
  *localdns.service*) fail "fallback declares Conflicts=localdns.service" ;;
  *) ok "fallback declares no Conflicts= on localdns.service" ;;
esac

# OnFailure= can fire more than once while localdns works through its restart
# budget. Those extra starts are gated no-ops, and without an unbounded budget they
# would rate-limit the one start that matters.
sli=$(systemctl show localdns-fallback.service -p StartLimitIntervalUSec --value 2>/dev/null || true)
case "$sli" in
  0|0s|infinity) ok "fallback StartLimitIntervalUSec=${sli}" ;;
  *) fail "fallback StartLimitIntervalUSec should be 0; got '$sli'" ;;
esac

# A oneshot that stays 'active' after exit: there is no daemon, so RemainAfterExit
# is what makes 'systemctl stop' meaningful and lets ExecStop tear the rules down.
[ "$(systemctl show localdns-fallback.service -p Type --value)" = "oneshot" ] \
  && ok "fallback is Type=oneshot" || fail "fallback Type is $(systemctl show localdns-fallback.service -p Type --value)"
[ "$(systemctl show localdns-fallback.service -p RemainAfterExit --value)" = "yes" ] \
  && ok "fallback is RemainAfterExit=yes" || fail "fallback RemainAfterExit is not yes"
systemctl show localdns-fallback.service -p ExecStop --value 2>/dev/null | grep -q "localdns-fallback.sh clear" \
  && ok "fallback tears rules down via ExecStop" || fail "fallback has no 'clear' ExecStop"

# No listener means nothing to collide with on recovery -- but it also means the
# script must never bind anything.
grep -qE "coredns|-conf |bind " /opt/azure/containers/localdns/localdns-fallback.sh \
  && fail "fallback script still references a CoreDNS listener" \
  || ok "fallback script runs no CoreDNS and binds nothing"

command -v dig >/dev/null 2>&1 && ok "dig is present for the assertions below" || fail "dig missing"

# While localdns serves, its drop-in pins the node resolver to .10 alone
# (DNS=169.254.10.10, UseDNS=false), so there is deliberately NO other upstream
# here. The one the fallback forwards .10 at only reappears once localdns exits
# and its ExecStopPost restores the DHCP resolvers -- which is why apply reads
# this file rather than being handed an address, and why it has to wait for the
# asynchronous networkctl reload to land.
[ -z "$(vnet_dns)" ] \
  && ok "with localdns serving, ${RESOLV} holds only ${NODE_IP} as expected" \
  || log "note: ${RESOLV} already lists an upstream ($(vnet_dns)) while localdns serves"
finish
`); err != nil {
		return err
	}

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
sudo cp -a "$ENVF" "$ENVBAK"

[ "$(pod_nameserver "$POD_CF")" = "$CLUSTER_IP" ] \
  && ok "baseline: ClusterFirst pod is pinned to ${CLUSTER_IP}" \
  || fail "baseline: ClusterFirst pod nameserver is $(pod_nameserver "$POD_CF")"
[ "$(pod_nameserver "$POD_DEF")" = "$NODE_IP" ] \
  && ok "baseline: Default pod is pinned to ${NODE_IP}" \
  || fail "baseline: Default pod nameserver is $(pod_nameserver "$POD_DEF")"

bad=$(printf 'THIS IS NOT A VALID COREFILE {{{\n' | base64 -w0)
tmp=$(mktemp)
while IFS= read -r l; do
  case "$l" in LOCALDNS_COREFILE_BASE=*) echo "LOCALDNS_COREFILE_BASE=${bad}" ;; *) echo "$l" ;; esac
done < "$ENVF" > "$tmp"
sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
log "injected fault; restarting localdns"
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl restart --no-block localdns.service

wait_for 240 "localdns reaches terminal 'failed'" sh -c 'systemctl is-failed --quiet localdns.service' \
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
wait_for 45 "ClusterFirst pod cluster lookup" pod_resolves_cluster_via "$POD_CF" "$CLUSTER_IP" \
  && ok "pre-failure pod resolves a cluster name through the fallback" \
  || { diag "cf cluster"; fail "pre-failure pod cannot resolve a cluster name"; }
pod_resolves_external_via "$POD_CF" "$CLUSTER_IP" \
  && ok "pre-failure pod resolves an external name through the fallback" \
  || { diag "cf external"; fail "pre-failure pod cannot resolve an external name"; }

# .10 clients -- the group the .11-only design black-holed. CoreDNS itself,
# konnectivity-agent and the CSI node drivers all live here.
[ "$(pod_nameserver "$POD_DEF")" = "$NODE_IP" ] \
  && ok "Default-policy pod is pinned to ${NODE_IP}" \
  || fail "Default-policy pod nameserver is $(pod_nameserver "$POD_DEF")"
wait_for 45 "Default pod external lookup" pod_resolves_external_via "$POD_DEF" "$NODE_IP" \
  && ok "Default-policy pod resolves an external name through the fallback" \
  || { diag "default external"; fail "Default-policy pod cannot resolve externally"; }

# The node itself, and every hostNetwork pod, uses .10 the same way.
host_resolves_external_via "$NODE_IP" \
  && ok "node resolves an external name through ${NODE_IP}" \
  || { diag "node external"; fail "node cannot resolve through ${NODE_IP}"; }

# A pod born during the outage gets the same treatment as one that predates it.
[ "$(pod_nameserver "$POD_DURING")" = "$CLUSTER_IP" ] \
  && ok "during-failure pod was born with nameserver ${CLUSTER_IP}" \
  || fail "during-failure pod nameserver is $(pod_nameserver "$POD_DURING")"
wait_for 45 "during-failure pod cluster lookup" pod_resolves_cluster_via "$POD_DURING" "$CLUSTER_IP" \
  && ok "during-failure pod resolves a cluster name through the fallback" \
  || { diag "during cluster"; fail "during-failure pod cannot resolve"; }
finish
`, podCF, podDefault, podDuring)); err != nil {
		return err
	}

	// ---- Phase 4: recovery ---------------------------------------------------
	if err := phase("phase4-recovery", `
sudo cp -a "$ENVBAK" "$ENVF"
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

# clear is idempotent: recovery already ran it via ExecStop.
sudo /opt/azure/containers/localdns/localdns-fallback.sh clear >/dev/null 2>&1 \
  && ok "GUARD: clear is idempotent" || fail "GUARD: clear failed when there was nothing to clear"
finish
`)
}
