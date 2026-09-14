package scenario

import (
	"context"
	"fmt"
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

// ValidateLocalDNSFallbackRecovery exercises the pod-DNS fallback end to end on
// a LocalDNS-enabled node, covering both triggers from the behavior matrix and
// the recovery handoff. It is intentionally state-driven (poll for observed
// conditions with hard time caps) rather than count/sleep-driven, so it is
// robust against systemd restart timing.
//
//	Case 1 (OnFailure): drive localdns to 'failed' and assert the fallback binds
//	                    .11 and pod DNS keeps resolving.
//	Recovery:           restart localdns and assert it reclaims .11 with no
//	                    sustained SERVFAIL gap (ExecStartPre handoff).
//	Case 3/4 (probe):   with the fallback and localdns both stopped, assert the
//	                    probe timer brings the fallback up within the debounce
//	                    window so .11 recovers without an OnFailure event.
func ValidateLocalDNSFallbackRecovery(ctx context.Context, s *Scenario) error {
	hasArtifacts, err := vhdHasLocalDNSFallbackArtifacts(ctx, s)
	if err != nil {
		return fmt.Errorf("detect localdns fallback artifacts: %w", err)
	}
	if !hasArtifacts {
		s.Logger.Logf("WARNING: VHD does not have localdns-fallback artifacts — skipping fallback validation")
		return nil
	}

	// All logic runs on the node. The script restores state via an EXIT trap so a
	// mid-test abort does not poison later scenarios sharing the node.
	script := `set -uo pipefail
CLUSTER_IP=169.254.10.11
PROBE_NAME=health-check.localdns.local
FAIL=0

log() { echo "fallback-e2e: $*"; }
fail() { echo "fallback-e2e: FAIL: $*" >&2; FAIL=1; }

cleanup() {
  log "cleanup: restoring localdns to healthy state"
  sudo systemctl stop localdns-fallback.service 2>/dev/null || true
  sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
  sudo systemctl reset-failed localdns.service 2>/dev/null || true
  sudo systemctl start localdns.service 2>/dev/null || true
}
trap cleanup EXIT

# dig helper: exit 0 on ANY DNS response (incl. NXDOMAIN) => a listener is alive.
answers_11() {
  dig +short +timeout=1 +tries=1 "$PROBE_NAME" "@${CLUSTER_IP}" >/dev/null 2>&1
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

# ---- Preconditions ---------------------------------------------------------
log "asserting systemd wiring"
props=$(systemctl show localdns.service -p OnFailure -p StartLimitBurst 2>/dev/null || true)
echo "$props"
echo "$props" | grep -q "OnFailure=localdns-fallback.service" || fail "localdns missing OnFailure=localdns-fallback.service"

log "baseline: .11 answers (served by localdns)"
wait_for 30 ".11 baseline answers" answers_11 || true

# ---- Case 1: crash storm -> failed -> OnFailure fallback -------------------
log "driving localdns to 'failed' by killing the supervisor until it latches"
deadline=$(( $(date +%s) + 120 ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  state=$(systemctl is-failed localdns.service 2>/dev/null || true)
  [ "$state" = "failed" ] && break
  pid=$(systemctl show -p MainPID --value localdns.service 2>/dev/null || echo 0)
  if [ "${pid:-0}" -gt 0 ]; then sudo kill -9 "$pid" 2>/dev/null || true; fi
  sleep 0.5
done
[ "$(systemctl is-failed localdns.service 2>/dev/null || true)" = "failed" ] \
  || fail "localdns did not reach 'failed' within the window"

log "asserting OnFailure started the fallback"
wait_for 20 "fallback active" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
ip addr show | grep -qw "$CLUSTER_IP" || fail ".11 not bound after fallback start"

log "asserting pod DNS survives via the fallback while localdns is failed"
wait_for 20 ".11 answers via fallback" answers_11 || true

# ---- Recovery + handoff: no sustained gap ---------------------------------
log "recovering localdns; probing .11 continuously across the handoff"
( end=$(( $(date +%s) + 25 ))
  while [ "$(date +%s)" -lt "$end" ]; do
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
wait_for 30 ".11 served by localdns after recovery" answers_11 || true

# ---- Case 3/4: probe-driven trigger (no OnFailure event) -------------------
# Stop the fallback AND localdns so .11 goes dark with localdns NOT in 'failed'.
# The probe timer must bring the fallback up within its debounce window.
log "probe path: stopping localdns cleanly and the fallback so .11 goes dark"
sudo systemctl stop localdns-fallback.service 2>/dev/null || true
sudo rm -f /run/localdns-fallback/consecutive_fails 2>/dev/null || true
sudo systemctl stop localdns.service 2>/dev/null || true
# localdns is now 'inactive' (a clean stop), NOT 'failed', so OnFailure will NOT fire.
[ "$(systemctl is-failed localdns.service 2>/dev/null || true)" != "failed" ] \
  || log "note: localdns shows failed; probe path still valid"

log "asserting the probe timer is active so it can trigger the fallback"
systemctl is-active --quiet localdns-fallback-probe.timer || fail "probe timer not active"

log "waiting for the probe debounce to start the fallback (~15s + margin)"
wait_for 45 "fallback started by probe" sh -c 'systemctl is-active --quiet localdns-fallback.service' || true
wait_for 20 ".11 answers via probe-started fallback" answers_11 || true

if [ "$FAIL" -ne 0 ]; then
  echo "fallback-e2e: one or more assertions FAILED" >&2
  exit 1
fi
echo "fallback-e2e: all assertions passed"
exit 0
`

	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
		"localdns pod-DNS fallback should recover .11 via both OnFailure and probe triggers"); err != nil {
		return fmt.Errorf("validate localdns fallback recovery: %w", err)
	}
	return nil
}
