package scenario

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// The shebang is load-bearing: these are uploaded as files and executed, and
// without it the node runs them with /bin/sh (dash), which rejects 'pipefail' and
// every other bashism below with "Illegal option -o pipefail".
const localDNSFallbackPrelude = `#!/bin/bash
set -uo pipefail
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

# True only when this node is configured the way aks-rp configures a localdns node:
# kubelet pinned to .11, so pods are born pinned to it and cannot be repointed. On
# the scriptless/ANC path that is not the case (see the COREDNS_SERVICE_IP assertion
# in the wiring phase), and pod-level assertions there would be testing a premise the
# environment never provided -- so they are skipped loudly rather than failed or,
# worse, quietly passed.
pods_are_pinned_to_11() { [ "$(kubelet_cluster_dns)" = "$CLUSTER_IP" ]; }
skip_unpinned() { log "SKIP ($1): kubelet --cluster-dns is $(kubelet_cluster_dns), not ${CLUSTER_IP}, so no pod on this node was ever pinned to the fallback listener"; }

kubelet_cluster_dns() { grep -oE '\-\-cluster-dns=[^" ]+' "$KUBELET_DEFAULT_FILE" 2>/dev/null | head -1 | cut -d= -f2-; }
localdns_substate()   { systemctl show localdns.service -p SubState --value 2>/dev/null; }
owner_of_11() { sudo ss -lunpH 2>/dev/null | grep "${CLUSTER_IP}:53" | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2; }
owner_unit() {
  local p; p=$(owner_of_11); [ -z "$p" ] && { echo "NONE"; return; }
  sudo grep -oE 'localdns[a-z-]*\.service' /proc/"$p"/cgroup 2>/dev/null | head -1 || echo "unknown"
}
# Dump enough state to diagnose a DNS failure from the CI log alone. A phase is a
# single shot -- there is no second look -- so anything not captured here is lost.
diag() {
  echo "fallback-e2e: ---- diagnostics: $* ----"
  echo "  localdns=$(systemctl is-active localdns.service)/$(systemctl is-failed localdns.service) fallback=$(systemctl is-active localdns-fallback.service)"
  echo "  .11 owner unit=$(owner_unit) pid=$(owner_of_11)"
  echo "  iface: $(ip -br addr show dev localdns 2>&1 | tr -s ' ')"
  echo "  kubelet --cluster-dns=$(kubelet_cluster_dns)"
  echo "  COREDNS_SERVICE_IP=$(grep -m1 '^COREDNS_SERVICE_IP=' "$ENVF" 2>/dev/null)"
  echo "  recorded-original: $(sudo cat /etc/localdns/kubelet-cluster-dns.orig 2>/dev/null || echo NONE)"
  echo "  node dig @.11: [$(dig +short +timeout=3 +tries=1 kubernetes.default.svc.cluster.local "@${CLUSTER_IP}" 2>&1 | tr '\n' ' ')]"
  echo "  dig binary: $(command -v dig || echo MISSING)   crictl: $(command -v crictl || echo MISSING)   nsenter: $(command -v nsenter || echo MISSING)"
  echo "  --- fallback corefile ---"; sudo sed 's/^/    /' "$FALLBACK_COREFILE" 2>/dev/null || echo "    (none)"
  echo "  --- fallback journal ---"; journalctl -u localdns-fallback.service --since "-5min" --no-pager -o cat 2>/dev/null | grep -v "Unknown key" | tail -12 | sed 's/^/    /'
  echo "  --- kubelet-dns journal ---"; journalctl --since "-5min" --no-pager -o cat 2>/dev/null | grep "localdns-kubelet-dns:" | tail -6 | sed 's/^/    /'
  echo "  --- localdns journal ---"; journalctl -u localdns.service --since "-5min" --no-pager -o cat 2>/dev/null | tail -8 | sed 's/^/    /'
  echo "fallback-e2e: ---- end diagnostics ----"
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

// uploadAndRunOnVM writes content to the node in 4KB chunks, then runs it.
//
// Bastion SSH tunnels have an 8KB WebSocket buffer, so any single command larger
// than that dies as "remote command exited without exit status or exit signal" -- a
// transport error that says nothing about the script. Both this scenario's phase
// scripts (~10-15KB) and its staged artifacts (up to 19KB) exceed it, so neither
// could ever have run as a single command. Same chunked-upload approach as
// ValidateLocalDNSExporterMetrics, which documents the limit.
//
// runCmd receives the remote path; return "" from it to upload without executing.
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

// localDNSFallbackArtifacts are the files this PR adds to the VHD, with their
// on-node destination and mode. Kept in one place so staging and the presence
// check cannot drift apart.
var localDNSFallbackArtifacts = []struct {
	repoFile string
	dest     string
	mode     string
}{
	// localdns.sh must be staged alongside localdns.service, not left at the VHD's
	// version. The unit's ExecStopPost calls 'localdns.sh cleanup', a mode older
	// scripts do not have, and ExecStopPost carries no '-' prefix -- so a mismatched
	// pair fails on every stop and leaves the unit 'failed' instead of 'inactive',
	// which silently converts the clean-stop case into the OnFailure= case.
	{"localdns.sh", "/opt/azure/containers/localdns/localdns.sh", "0755"},
	{"localdns.service", "/etc/systemd/system/localdns.service", "0644"},
	{"localdns-fallback.service", "/etc/systemd/system/localdns-fallback.service", "0644"},
	{"localdns-fallback.sh", "/opt/azure/containers/localdns/localdns-fallback.sh", "0755"},
	{"localdns-fallback-probe.service", "/etc/systemd/system/localdns-fallback-probe.service", "0644"},
	{"localdns-fallback-probe.timer", "/etc/systemd/system/localdns-fallback-probe.timer", "0644"},
	{"localdns-fallback-probe.sh", "/opt/azure/containers/localdns/localdns-fallback-probe.sh", "0755"},
	{"localdns-kubelet-dns.sh", "/opt/azure/containers/localdns/localdns-kubelet-dns.sh", "0755"},
}

// stageLocalDNSFallbackArtifacts installs this branch's localdns artifacts onto the
// node.
//
// Without it this scenario can never run before the PR merges. The e2e provisions a
// node with CSE and custom data generated from THIS branch, but the base VHD is a
// published image, and these files are baked into the VHD by packer rather than
// delivered at provision time. So vhdHasLocalDNSFallbackArtifacts is false on every
// pipeline run and the whole scenario skips -- which means it would first execute
// only after merging, which is exactly backwards.
//
// Staging closes that gap: it is the same thing a human does by hand to test a
// VHD-baked change, done deterministically. It is a no-op once the artifacts ship.
func stageLocalDNSFallbackArtifacts(ctx context.Context, s *Scenario) error {
	// One small command per file, gzipped. A single command carrying all eight
	// files inline is ~150KB of base64, and execScriptOnVMForScenario scp's the
	// script under a 10s timeout (exec.go), so a large payload over a tunneled
	// connection fails as "remote command exited without exit status or exit
	// signal" -- a transport error that says nothing about what went wrong.
	// Per-file also means a failure names the file it failed on.
	for _, a := range localDNSFallbackArtifacts {
		src := repoPath(filepath.Join("parts", "linux", "cloud-init", "artifacts", a.repoFile))
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s for staging: %w", src, err)
		}
		staged := "/home/azureuser/stage_" + a.repoFile
		install := func(remote string) string {
			// Small enough to stay well under the tunnel's 8KB command limit.
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

	// localdns.service was replaced, so the loaded unit is stale until reloaded. The
	// probe timer is normally enabled by enableLocalDNS, but its backward-compat guard
	// skipped it at provisioning time because the unit did not exist yet.
	//
	// Deliberately NO restart here: see restartLocalDNSDetached below.
	reload := `set -euo pipefail
sudo systemctl daemon-reload
sudo systemctl enable --now localdns-fallback-probe.timer
echo "staged: daemon reloaded, probe-timer=$(systemctl is-active localdns-fallback-probe.timer)"`
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
	detach := `sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo setsid nohup systemctl restart localdns.service >/dev/null 2>&1 &
exit 0`
	if _, err := execScriptOnVMForScenario(ctx, s, detach); err != nil {
		return fmt.Errorf("request detached localdns restart: %w", err)
	}

	const attempts = 20
	var lastErr error
	for i := 0; i < attempts; i++ {
		time.Sleep(5 * time.Second)
		res, err := execScriptOnVMForScenario(ctx, s, "systemctl is-active localdns.service")
		if err != nil {
			// Expected while DNS is down mid-restart; keep polling.
			lastErr = err
			continue
		}
		if strings.TrimSpace(res.stdout) == "active" {
			logging.Logf(ctx, "localdns is active again after staging (attempt %d)", i+1)
			return nil
		}
		lastErr = fmt.Errorf("localdns is %q", strings.TrimSpace(res.stdout))
	}
	return fmt.Errorf("localdns did not become active after staging: %w", lastErr)
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

// alternateKubeDNSService is a second Service in front of the same CoreDNS pods,
// used to prove COREDNS_SERVICE_IP is honoured verbatim rather than falling back to
// the hardcoded 10.0.0.10 default. The ClusterIP is deliberately left unset so the
// API server allocates one from whatever service CIDR this cluster uses --
// hardcoding an address would guess wrong on a custom-CIDR cluster, which is
// precisely the case this assertion exists to cover.
func alternateKubeDNSService(s *Scenario) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			// Literal prefix first: Service names are RFC 1035 labels and must start
			// with a letter, but the scenario's node name starts with the date, so
			// "<node>-kube-dns-alt" is rejected. (Pod names are RFC 1123 subdomains
			// and do allow a leading digit, which is why only this one needs it.)
			Name:      fmt.Sprintf("kube-dns-alt-%s", s.Runtime.VM.KubeName),
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
		// Do not skip: install this branch's artifacts and test them. See
		// stageLocalDNSFallbackArtifacts for why skipping would mean this scenario
		// only ever runs after the code it covers has already merged.
		logging.Logf(ctx, "VHD predates the localdns-fallback artifacts; staging them from the working tree")
		if err := stageLocalDNSFallbackArtifacts(ctx, s); err != nil {
			return err
		}
		staged, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
		if err != nil {
			return fmt.Errorf("re-check localdns fallback artifacts after staging: %w", err)
		}
		if !staged {
			return fmt.Errorf("localdns fallback artifacts still absent after staging")
		}
	}

	// Phase scripts are 10-15KB, well past the bastion tunnel's 8KB command limit,
	// so they are uploaded in chunks and then executed rather than sent inline.
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
		if _, err := uploadAndRunOnVM(cleanupCtx, s, restore, "/home/azureuser/localdns_fallback_restore.sh",
			func(r string) string { return "sudo " + r }, "localdns fallback e2e: restore"); err != nil {
			logging.Logf(ctx, "localdns fallback e2e: node restore failed: %v", err)
		}
	}()

	// Alternate kube-dns Service: a real, kube-proxy-programmed ClusterIP that is
	// provably not the cluster default, for the custom-ClusterIP assertions below.
	kube := s.Runtime.Kube
	svc := alternateKubeDNSService(s)
	createdSvc, err := kube.Typed.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create alternate kube-dns service: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := kube.Typed.CoreV1().Services(createdSvc.Namespace).Delete(cleanupCtx, createdSvc.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			logging.Logf(ctx, "could not delete alternate kube-dns service %s: %v", createdSvc.Name, err)
		}
	}()
	kubeDNS, err := kube.Typed.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get kube-dns service: %w", err)
	}
	kubeDNSClusterIP := kubeDNS.Spec.ClusterIP
	logging.Logf(ctx, "cluster kube-dns ClusterIP is %s", kubeDNSClusterIP)

	altClusterIP := createdSvc.Spec.ClusterIP
	if altClusterIP == "" {
		return fmt.Errorf("alternate kube-dns service %q was not allocated a ClusterIP", createdSvc.Name)
	}
	logging.Logf(ctx, "alternate kube-dns Service %q allocated ClusterIP %s", createdSvc.Name, altClusterIP)

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
	if err := phase("phase0-wiring", fmt.Sprintf(`
KUBE_DNS_IP=%q
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

# The upstream this whole feature forwards to must actually be the cluster's kube-dns
# ClusterIP. Asserted by name so a failure reads as the aks-rp gap it is, rather than
# surfacing later as an unexplained DNS timeout on .11.
coredns_ip=$(grep -m1 '^COREDNS_SERVICE_IP=' "$ENVF" 2>/dev/null | cut -d= -f2-)
if [ -z "$coredns_ip" ]; then
  fail "COREDNS_SERVICE_IP is empty in ${ENVF}. aks-rp does not populate ClusterNetworkConfig.core_dns_service_ip on the scriptless/ANC path, so the generated corefile falls back to the hardcoded 10.0.0.10. On this cluster kube-dns is ${KUBE_DNS_IP}, so 169.254.10.11 cannot resolve cluster names at all. Needs an aks-rp fix; see the PR description."
elif [ "$coredns_ip" != "$KUBE_DNS_IP" ]; then
  fail "COREDNS_SERVICE_IP is ${coredns_ip} but this cluster's kube-dns ClusterIP is ${KUBE_DNS_IP}"
else
  ok "COREDNS_SERVICE_IP is populated and matches the cluster's kube-dns ClusterIP (${coredns_ip})"
fi
# Same check against what localdns is actually running, which is what the fallback derives from.
fwd=$(sudo grep -oE 'forward \. [0-9.]+' "$UPDATED_COREFILE" 2>/dev/null | awk '{print $3}' | grep -v '^168\.63\.129\.16$' | sort -u | tr '\n' ' ')
case " $fwd " in
  *" $KUBE_DNS_IP "*) ok "localdns's corefile forwards cluster DNS to ${KUBE_DNS_IP}" ;;
  *) fail "localdns's corefile forwards cluster DNS to [${fwd}], not this cluster's kube-dns ClusterIP ${KUBE_DNS_IP} -- pods using 169.254.10.11 cannot resolve cluster names" ;;
esac

# Distro invariant. localdns-fallback-probe.sh calls dig unconditionally, but nothing
# in the VHD build installs it -- localdns itself health-checks with curl against
# :8181. On an image without dig the probe fails open and never fires, which silently
# removes the ONLY trigger for the clean-stop case below. This scenario runs on
# Ubuntu and AzureLinux, and dig is far likelier to be missing on the latter, so make
# that a CI failure rather than a quiet loss of coverage.
if systemctl is-enabled --quiet localdns-fallback-probe.timer 2>/dev/null \
   || systemctl is-active --quiet localdns-fallback-probe.timer 2>/dev/null; then
  command -v dig >/dev/null 2>&1 \
    && ok "dig is present, so the fallback probe can actually evaluate ${CLUSTER_IP}" \
    || fail "the probe timer is enabled but dig is not installed on this image; the probe fails open and the clean-stop path can never trigger"
else
  log "note: probe timer neither enabled nor active; skipping the dig invariant"
fi

finish
`, kubeDNSClusterIP)); err != nil {
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
if pods_are_pinned_to_11; then
  wait_for 30 "pod resolution over the surviving interface" pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "CRASH: pre-existing pod resolves through the fallback after a SIGKILL" \
    || { diag "pod cannot resolve through the fallback"; fail "CRASH: pre-existing pod cannot resolve after a SIGKILL"; }
  log "pod netns=$(pod_netns "$POD1") nameserver=$(pod_nameserver "$POD1")"
else
  skip_unpinned "CRASH pod resolution"
fi

# Restore for the teardown phases that follow.
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/localdns.service.d/99-e2e-crashpath.conf
sudo systemctl daemon-reload
sudo systemctl reset-failed localdns.service 2>/dev/null || true
sudo systemctl start localdns.service 2>/dev/null || true
wait_for 90 "localdns healthy again" sh -c 'systemctl is-active --quiet localdns.service' || true
if pods_are_pinned_to_11; then
  wait_for 60 "kubelet --cluster-dns restored after the crash phase" \
    sh -c '[ "$(grep -oE "\-\-cluster-dns=[^\" ]+" /etc/default/kubelet | head -1 | cut -d= -f2-)" = "169.254.10.11" ]' \
    && ok "CRASH: kubelet --cluster-dns was restored on recovery" \
    || { diag "kubelet --cluster-dns not restored"; fail "CRASH: kubelet --cluster-dns is $(kubelet_cluster_dns) after recovery"; }
else
  skip_unpinned "CRASH kubelet repoint round trip"
fi
finish
`, pod1)); err != nil {
		return err
	}

	// ---- Phase 2: baseline, induce the teardown failure ----------------------
	if err := phase("phase2-induce-failure", fmt.Sprintf(`
POD1=%q
sudo cp -a "$ENVF" "$ENVBAK"
sudo cp -a "$UPDATED_COREFILE" "$COREFILE_BAK"

if pods_are_pinned_to_11; then
  ok "baseline: kubelet --cluster-dns is ${CLUSTER_IP}"
else
  skip_unpinned "baseline kubelet --cluster-dns"
fi
if pods_are_pinned_to_11; then
  [ "$(pod_nameserver "$POD1")" = "$CLUSTER_IP" ] \
    && ok "baseline: pre-failure pod resolv.conf points at ${CLUSTER_IP}" \
    || fail "baseline: pre-failure pod nameserver is $(pod_nameserver "$POD1")"
  wait_for 60 "baseline pod resolution" pod_resolves_via "$POD1" "$CLUSTER_IP" || true
else
  skip_unpinned "baseline pod resolution"
fi

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
ALT_CLUSTER_IP=%q
COREDNS_IP=$(kubelet_cluster_dns)
log "kubelet --cluster-dns is now ${COREDNS_IP}"

if pods_are_pinned_to_11; then
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
else
  skip_unpinned "CASE 1 and CASE 2 pod generations"
fi

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
if pods_are_pinned_to_11; then
    wait_for 30 "CASE 1 lookup on the derived corefile" pod_resolves_via "$POD1" "$CLUSTER_IP" \
      && ok "DERIVED: pre-failure pod still resolves on the derived corefile" \
      || fail "DERIVED: pre-failure pod cannot resolve on the derived corefile"
else
  skip_unpinned "derived-corefile pod lookup"
fi

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

# --- Custom CoreDNS ClusterIP: positive, then a negative control -------------
# COREDNS_SERVICE_IP must be honoured verbatim. The positive case alone proves
# nothing on a default-CIDR cluster: a silent fall back to the hardcoded 10.0.0.10
# would pass it. The negative control is what makes the pair meaningful.
sudo conntrack -D -p udp --dport 53 >/dev/null 2>&1 || true
set_upstream() {
  local ip="$1" tmp; tmp=$(mktemp)
  grep -v '^COREDNS_SERVICE_IP=' "$ENVF" > "$tmp"
  echo "COREDNS_SERVICE_IP=${ip}" >> "$tmp"
  sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
}
set_upstream "$ALT_CLUSTER_IP"
sudo systemctl restart localdns-fallback.service; sleep 5
assert_fallback_owns_11 "custom-clusterip"
sudo grep -q "forward . ${ALT_CLUSTER_IP}" "$FALLBACK_COREFILE" \
  && ok "CUSTOM: fallback forwards to the alternate ClusterIP ${ALT_CLUSTER_IP}" \
  || fail "CUSTOM: corefile does not forward to ${ALT_CLUSTER_IP}"
if pods_are_pinned_to_11; then
  wait_for 30 "pod resolution via the alternate ClusterIP" pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "CUSTOM: existing pod resolves through the alternate ClusterIP" \
    || fail "CUSTOM: pod could not resolve via ${ALT_CLUSTER_IP}"
else
  skip_unpinned "custom ClusterIP pod lookup"
fi

if command -v conntrack >/dev/null 2>&1; then
  sudo conntrack -L -p udp 2>/dev/null | grep "dst=${ALT_CLUSTER_IP}" | head -2 || true
  sudo conntrack -L -p udp 2>/dev/null | grep -q "dst=${ALT_CLUSTER_IP}" \
    && ok "CUSTOM: conntrack confirms queries went to ${ALT_CLUSTER_IP} and were DNAT'd by kube-proxy" \
    || log "note: no conntrack entry to ${ALT_CLUSTER_IP} captured (may have aged out)"
else
  log "note: conntrack not installed on this image; packet-path assertion skipped"
fi

if pods_are_pinned_to_11; then
  # Negative control: an unroutable upstream must break resolution. If it does not,
  # COREDNS_SERVICE_IP is being ignored and the positive case above was meaningless.
  set_upstream "240.0.0.1"
  sudo systemctl restart localdns-fallback.service; sleep 5
  assert_fallback_owns_11 "bogus-upstream"
  if pod_resolves_via "$POD1" "$CLUSTER_IP"; then
    fail "CUSTOM: pod still resolved with upstream 240.0.0.1 — COREDNS_SERVICE_IP is being ignored"
  else
    ok "CUSTOM: pod correctly failed to resolve with a bogus upstream (the value is honoured)"
  fi
else
  skip_unpinned "custom ClusterIP negative control"
fi

# Back to the alternate ClusterIP, so the negative result above is shown to be causal.
set_upstream "$ALT_CLUSTER_IP"
sudo systemctl restart localdns-fallback.service; sleep 5
if pods_are_pinned_to_11; then
  wait_for 30 "pod resolution restored on the alternate ClusterIP" pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "CUSTOM: pod resolves again on the alternate ClusterIP (the negative was causal)" \
    || fail "CUSTOM: pod did not recover on ${ALT_CLUSTER_IP}"
else
  skip_unpinned "custom ClusterIP recovery"
fi

finish
`, pod1, pod2, altClusterIP)); err != nil {
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
if pods_are_pinned_to_11; then
  [ "$(pod_nameserver "$POD3")" = "$CLUSTER_IP" ] \
    && ok "CASE 3: post-recovery pod was born with nameserver ${CLUSTER_IP}" \
    || fail "CASE 3: post-recovery pod nameserver is $(pod_nameserver "$POD3"), expected ${CLUSTER_IP}"
  wait_for 30 "CASE 3 cluster lookup" pod_resolves_via "$POD3" "$CLUSTER_IP" \
    && ok "CASE 3: post-recovery pod resolves through localdns" \
    || fail "CASE 3: post-recovery pod cannot resolve"
  pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "CASE 1: pre-failure pod still resolves after recovery" \
    || fail "CASE 1: pre-failure pod broken after recovery"
else
  skip_unpinned "CASE 3 pod generation"
fi

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

# --- Clean stop: the one row of the behaviour matrix OnFailure= cannot cover ---
# 'systemctl stop' is not a failure, so OnFailure= never fires and the unit lands in
# 'inactive', not 'failed'. The probe timer is then the ONLY thing that can restore
# .11, and its trap path has also deleted the dummy interface. This path had never
# been exercised on any node before this assertion existed.
log "probe path: stopping localdns cleanly so .11 goes dark without a failure"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
sudo systemctl stop localdns.service 2>/dev/null || true
sleep 2
[ "$(systemctl is-failed localdns.service 2>/dev/null || true)" != "failed" ] \
  && ok "PROBE: localdns is stopped but NOT 'failed', so OnFailure= cannot fire" \
  || fail "PROBE: localdns reports 'failed' after a clean stop; this is not the clean-stop path"
systemctl is-active --quiet localdns-fallback-probe.timer \
  && ok "PROBE: the probe timer is active" || fail "PROBE: probe timer is not active"
wait_for 60 "fallback started by the probe (debounce ~15s)" \
  sh -c 'systemctl is-active --quiet localdns-fallback.service' \
  && ok "PROBE: the probe started the fallback with no OnFailure= event" \
  || fail "PROBE: the probe never started the fallback"
assert_fallback_owns_11 "PROBE"
if pods_are_pinned_to_11; then
  wait_for 30 "pod resolution via the probe-started fallback" pod_resolves_via "$POD1" "$CLUSTER_IP" \
    && ok "PROBE: pre-existing pod resolves again after a clean localdns stop" \
    || fail "PROBE: pod could not resolve after a clean localdns stop"
else
  skip_unpinned "probe-path pod lookup"
fi


finish
`, pod1, pod3))
}
