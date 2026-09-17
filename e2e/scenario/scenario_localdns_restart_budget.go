package scenario

import (
	"context"
	"fmt"
)

// LocalDNS restart-budget validation.
//
// localdns.service pins StartLimitIntervalSec=720 / StartLimitBurst=5 / RestartSec=2 so
// that unrecoverable failures terminate in 'failed' rather than restarting forever. Only
// the terminal state lets an OnFailure= handoff fire and gives NPD a stable state to
// observe, so the directives are load-bearing -- but nothing guarded them: the lifecycle
// scenario does three kill/recovery cycles and would still pass if they were deleted.
//
// These checks reproduce, in CI, a failure-mode matrix that was measured by hand on a live
// mode=Required nodepool (Ubuntu 24.04.4, systemd 255.4). Each fault drives a real code
// path in localdns.sh or a real systemd timeout rather than replacing the service with a
// stub, and each asserts that the unit reaches 'failed'.
//
// Measured note that shapes the assertions: systemd does NOT report
// Result=start-limit-hit here. It reports the underlying cause -- exit-code for most
// modes, watchdog for the watchdog kill, timeout for the hung start -- and the journal
// shows "Start request repeated too quickly." followed by "Failed with result 'timeout'.".
// So the terminal state is asserted via ActiveState plus the journal refusal line plus the
// ExecStart count, and Result is printed as diagnostic only.

// localdnsFault is one row of the failure-mode matrix.
type localdnsFault struct {
	// token written to the fault file and matched by the injected hooks
	name string
	// human-readable description used in failure messages
	label string
	// seconds to wait for the unit to reach 'failed', from the measured time plus margin
	deadlineSeconds int
}

// localdnsFaultMatrix is the full set of modes, with the measured time to 'failed' under
// 720/5/2 noted against each deadline.
//
// Every fault is driven by a hook inside the patched localdns.sh rather than by a systemd
// drop-in. Inducing pre-flight and hung-start with ExecStartPre= would be simpler, but
// ExecStartPre short-circuits before localdns.sh runs, so the ExecStart counter would
// never increment and the burst assertion could not be made. Driving them from inside the
// script also keeps each fault on the real code path.
var localdnsFaultMatrix = []localdnsFault{
	{name: "preflight", label: "pre-flight abort", deadlineSeconds: 60},           // measured 11s
	{name: "resolvdrain", label: "resolv.conf never drains", deadlineSeconds: 90}, // measured 37s
	{name: "postready", label: "dies right after READY", deadlineSeconds: 150},    // measured 64s
	{name: "nopidfile", label: "PID file never appears", deadlineSeconds: 150},    // measured 64s
	{name: "readytimeout", label: "ready-check timeout", deadlineSeconds: 420},    // measured 318s
	{name: "watchdog", label: "watchdog pings cease", deadlineSeconds: 480},       // measured 370s
	{name: "hungstart", label: "hung start", deadlineSeconds: 600},                // measured 487s
}

// localdnsDiscriminatingFault is the single mode used on distros that do not carry the
// full matrix.
//
// It must be a mode whose restart cycle sits between the two thresholds: above the shipped
// default's interval/burst = 10/5 = 2s, and below this unit's 720/5 = 144s. resolv.conf
// drain (~5.5s cycle) is the cheapest such mode -- measured 55 restarts over 300s without
// ever reaching 'failed' under the default budget, versus 'failed' in 37s under 720/5.
//
// Pre-flight is deliberately NOT used here despite being faster: its ~0.4s cycle is below
// the default threshold too, so it reaches 'failed' with or without these directives and
// would guard nothing.
func localdnsDiscriminatingFault() []localdnsFault {
	for _, f := range localdnsFaultMatrix {
		if f.name == "resolvdrain" {
			return []localdnsFault{f}
		}
	}
	// unreachable unless the matrix above is edited; fail loudly rather than silently
	// running an empty matrix.
	return nil
}

const (
	localdnsFaultScript  = "/run/localdns-e2e.sh"
	localdnsFaultFile    = "/run/localdns-e2e-fault"
	localdnsFaultCounter = "/run/localdns-e2e-starts"
	localdnsFaultDropIn  = "/run/systemd/system/localdns.service.d/99-e2e-fault.conf"
)

// validateLocalDNSRestartBudget asserts that the unit's restart budget terminates each
// supplied failure mode in 'failed', and leaves LocalDNS healthy afterwards.
func validateLocalDNSRestartBudget(ctx context.Context, s *Scenario, faults []localdnsFault) error {
	if len(faults) == 0 {
		return fmt.Errorf("no LocalDNS faults selected: the fault matrix is misconfigured")
	}

	if err := assertLocalDNSBudgetDirectives(ctx, s); err != nil {
		return err
	}
	if err := assertLocalDNSSurvivesProvisioningRestarts(ctx, s); err != nil {
		return err
	}

	if err := installLocalDNSFaultHarness(ctx, s); err != nil {
		return fmt.Errorf("install LocalDNS fault harness: %w", err)
	}
	// Always tear the harness down, including on early return, so the node is not left
	// running the patched script or a temporary drop-in.
	defer func() {
		_, _ = execScriptOnVMForScenario(ctx, s, localdnsFaultTeardownScript)
	}()

	for _, fault := range faults {
		if err := runLocalDNSFault(ctx, s, fault); err != nil {
			return err
		}
	}
	return nil
}

// assertLocalDNSBudgetDirectives checks the effective directives rather than the file, so
// a drop-in that quietly overrides them is caught too.
func assertLocalDNSBudgetDirectives(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsDirectiveAssertScript, 0,
		"LocalDNS restart-budget directives are not as expected")
	return err
}

const localdnsDirectiveAssertScript = `
set -eu

fail=0
check() {
    actual=$(systemctl show localdns.service -p "$1" --value)
    if [ "$actual" != "$2" ]; then
        echo "FAIL: expected $1=$2, got $1=$actual"
        fail=1
    fi
}

# The budget under test.
check StartLimitIntervalUSec "12min"
check StartLimitBurst "5"
check RestartUSec "2s"

# The threshold is StartLimitIntervalSec/StartLimitBurst = 720/5 = 144s, which only clears
# the slowest restart cycle while TimeoutStartSec stays at the inherited 90s. It is not
# pinned in the unit, so a change to DefaultTimeoutStartSec would move the slowest cycle
# and silently invalidate the margin. Assert it so that change fails here instead.
check TimeoutStartUSec "1min 30s"

exit "$fail"
`

// assertLocalDNSSurvivesProvisioningRestarts models what CSE does at provisioning time.
//
// enableLocalDNS() runs 'systemctl reset-failed localdns' before each start attempt
// because manual restarts draw on the same StartLimitBurst as automatic ones: without the
// reset, one CSE start plus Restart=on-failure drains all five slots in ~11s and every
// later attempt is refused for the rest of the 720s window. This asserts a CSE-shaped loop
// keeps the unit startable.
func assertLocalDNSSurvivesProvisioningRestarts(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsProvisioningRestartScript, 0,
		"LocalDNS could not survive a CSE-style provisioning restart loop")
	return err
}

const localdnsProvisioningRestartScript = `
set -eu

for i in 1 2 3 4 5 6; do
    sudo systemctl reset-failed localdns.service 2>/dev/null || true
    if ! sudo timeout 60 systemctl restart localdns.service; then
        echo "FAIL: provisioning-style restart $i was refused"
        sudo systemctl status localdns.service --no-pager -l || true
        exit 1
    fi
done

state=$(sudo systemctl show localdns.service -p ActiveState --value)
if [ "$state" != active ]; then
    echo "FAIL: localdns is $state after six reset-failed+restart cycles"
    exit 1
fi
echo "provisioning-style restart loop OK"
`

// installLocalDNSFaultHarness builds a patched copy of the shipped localdns.sh with the
// fault hooks inserted, and points a transient drop-in at it.
//
// The drop-in deliberately overrides only ExecStart: StartLimitIntervalSec,
// StartLimitBurst, RestartSec and TimeoutStartSec continue to come from the shipped unit,
// because those are the thing under test.
func installLocalDNSFaultHarness(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsFaultHarnessInstallScript, 0,
		"failed to install the LocalDNS fault harness")
	return err
}

const localdnsFaultHarnessInstallScript = `
set -eu

SRC=/opt/azure/containers/localdns/localdns.sh
DST=` + localdnsFaultScript + `
FAULT=` + localdnsFaultFile + `
COUNTER=` + localdnsFaultCounter + `
DROPIN=` + localdnsFaultDropIn + `
WORK=$(mktemp -d)

sudo test -f "$SRC"
sudo cp "$SRC" "$WORK/script"

# insert_hook <anchor-line> <before|after> <block-file>
#
# Matches the anchor as a whole line and requires exactly one occurrence. If localdns.sh
# moves, this fails loudly rather than silently inserting nothing and leaving a test that
# always passes.
insert_hook() {
    anchor=$1; pos=$2; blockfile=$3
    count=$(grep -c -x -F "$anchor" "$WORK/script" || true)
    if [ "$count" != "1" ]; then
        echo "ANCHOR-FAIL: '$anchor' matched $count lines in localdns.sh, expected exactly 1"
        echo "localdns.sh has changed; refresh the e2e fault anchors."
        exit 1
    fi
    awk -v anchor="$anchor" -v pos="$pos" -v bf="$blockfile" '
        BEGIN { while ((getline line < bf) > 0) block = block line "\n" }
        $0 == anchor {
            if (pos == "before") printf "%s", block
            print
            if (pos == "after") printf "%s", block
            next
        }
        { print }
    ' "$WORK/script" > "$WORK/next"
    mv "$WORK/next" "$WORK/script"
}

# (1) Authoritative ExecStart counter, plus the pre-flight fault.
#
# The counter is written by the script itself on every invocation. Counting starts by
# grepping the journal for "Starting ..." is unreliable here and produced wrong numbers
# during the manual investigation, so it is deliberately not used.
cat > "$WORK/b1" <<'HOOK'
# --- e2e fault harness ---
LDNS_FAULT="$(cat /run/localdns-e2e-fault 2>/dev/null || echo none)"
echo "$(date +%s) EXECSTART fault=${LDNS_FAULT}" >> /run/localdns-e2e-starts
if [ "${LDNS_FAULT}" = "preflight" ]; then
    echo "E2EFAULT preflight: pre-flight check failed"
    exit 1
fi
# --- end e2e fault harness ---
HOOK
insert_hook 'regenerate_localdns_corefile || exit $ERR_LOCALDNS_COREFILE_NOTFOUND' before "$WORK/b1"

# (2) resolv.conf never drains: the real 5s wait times out.
cat > "$WORK/b2" <<'HOOK'
    if [ "$(cat /run/localdns-e2e-fault 2>/dev/null)" = "resolvdrain" ]; then
        echo "E2EFAULT resolvdrain: resolv.conf never drains"; sleep 5; return 1
    fi
HOOK
insert_hook 'wait_for_localdns_removed_from_resolv_conf() {' after "$WORK/b2"

# (3) Watchdog pings cease: the real WatchdogSec=60 fires.
cat > "$WORK/b3" <<'HOOK'
    if [ "$(cat /run/localdns-e2e-fault 2>/dev/null)" = "watchdog" ]; then
        echo "E2EFAULT watchdog: ceasing watchdog pings"
        while true; do sleep 5; done
    fi
HOOK
insert_hook 'start_localdns_watchdog() {' after "$WORK/b3"

# (4) PID file never appears: repoint the wait loop at a path CoreDNS will not create so
# the real START_LOCALDNS_TIMEOUT=10 loop times out. COREDNS_COMMAND has already been built
# from the real path, so CoreDNS itself still behaves normally.
cat > "$WORK/b4" <<'HOOK'
if [ "${LDNS_FAULT}" = "nopidfile" ]; then
    echo "E2EFAULT nopidfile: pid file will never appear"
    LOCALDNS_PID_FILE=/run/localdns-e2e-never-appears.pid
fi
HOOK
insert_hook 'start_localdns || exit $ERR_LOCALDNS_FAIL' before "$WORK/b4"

# (5) Readiness never succeeds: kill CoreDNS so the real wait_for_localdns_ready 60 60
# polls and times out on its own.
cat > "$WORK/b5" <<'HOOK'
if [ "${LDNS_FAULT}" = "readytimeout" ]; then
    echo "E2EFAULT readytimeout: killing coredns so readiness never succeeds"
    kill -9 "${COREDNS_PID}" 2>/dev/null || true
fi
HOOK
insert_hook 'start_localdns || exit $ERR_LOCALDNS_FAIL' after "$WORK/b5"

# (6) Hang before READY so the real TimeoutStartSec fires.
cat > "$WORK/b6" <<'HOOK'
if [ "${LDNS_FAULT}" = "hungstart" ]; then
    echo "E2EFAULT hungstart: hanging before READY"
    sleep infinity
fi
HOOK
insert_hook '   systemd-notify --ready' before "$WORK/b6"

# (7) Die immediately after READY, before the watchdog loop starts.
cat > "$WORK/b7" <<'HOOK'
if [ "${LDNS_FAULT}" = "postready" ]; then
    echo "E2EFAULT postready: exiting immediately after READY"
    exit 1
fi
HOOK
insert_hook 'start_localdns_watchdog' before "$WORK/b7"

bash -n "$WORK/script" || { echo "FAIL: patched localdns.sh is not valid bash"; exit 1; }

sudo install -m 0755 "$WORK/script" "$DST"
sudo rm -f "$FAULT" "$COUNTER"
rm -rf "$WORK"

# Point the unit at the patched copy. Invoking through bash avoids any noexec concern on
# /run. ExecStart= clears the shipped value before setting the replacement.
sudo mkdir -p "$(dirname "$DROPIN")"
printf '[Service]\nExecStart=\nExecStart=/bin/bash %s\n' "$DST" | sudo tee "$DROPIN" >/dev/null
sudo systemctl daemon-reload
echo "fault harness installed"
`

// localdnsFaultTeardownScript removes the harness and restores a healthy LocalDNS. It is
// best-effort by design: it runs from a defer, including on the failure path.
const localdnsFaultTeardownScript = `
set -u
sudo rm -f ` + localdnsFaultFile + ` ` + localdnsFaultCounter + ` ` + localdnsFaultDropIn + ` ` + localdnsFaultScript + `
sudo systemctl daemon-reload || true
sudo systemctl reset-failed localdns.service || true
sudo systemctl start localdns.service || true
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if sudo systemctl is-active --quiet localdns.service; then
        break
    fi
    sleep 1
done
sudo systemctl is-active --quiet localdns.service || echo "WARNING: localdns is not active after fault teardown"
`

// runLocalDNSFault arms one fault, waits for the unit to give up, and asserts the terminal
// state before restoring the service for the next mode.
func runLocalDNSFault(ctx context.Context, s *Scenario, fault localdnsFault) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsFaultRunScript(fault), 0,
		fmt.Sprintf("LocalDNS restart budget did not bound the %q failure mode", fault.label))
	if err != nil {
		return fmt.Errorf("fault %s (%s): %w", fault.name, fault.label, err)
	}
	return nil
}

// localdnsFaultRunScript renders the per-fault script.
func localdnsFaultRunScript(fault localdnsFault) string {
	return `
set -eu

FAULT=` + fault.name + `
LABEL="` + fault.label + `"
DEADLINE=` + fmt.Sprint(fault.deadlineSeconds) + `
COUNTER=` + localdnsFaultCounter + `

echo "=== fault: $FAULT ($LABEL), deadline ${DEADLINE}s ==="

sudo systemctl stop localdns.service 2>/dev/null || true
sudo systemctl daemon-reload
# Start each mode from a fresh budget so the previous mode's spent slots cannot make this
# one terminate early and pass for the wrong reason.
sudo systemctl reset-failed localdns.service 2>/dev/null || true

burst=$(systemctl show localdns.service -p StartLimitBurst --value)

echo "$FAULT" | sudo tee ` + localdnsFaultFile + ` >/dev/null
sudo rm -f "$COUNTER"
since=$(date '+%Y-%m-%d %H:%M:%S')

# --no-block: a hung start would otherwise hold this call for TimeoutStartSec.
sudo systemctl start --no-block localdns.service || true

elapsed=0
state=""
while [ "$elapsed" -lt "$DEADLINE" ]; do
    state=$(sudo systemctl show localdns.service -p ActiveState --value)
    if [ "$state" = failed ]; then
        break
    fi
    sleep 1
    elapsed=$((elapsed + 1))
done

starts=$(sudo grep -c EXECSTART "$COUNTER" 2>/dev/null || echo 0)
result=$(sudo systemctl show localdns.service -p Result --value)
substate=$(sudo systemctl show localdns.service -p SubState --value)
echo "fault=$FAULT elapsed=${elapsed}s state=$state sub=$substate result=$result execstarts=$starts burst=$burst"

if [ "$state" != failed ]; then
    echo "FAIL: $LABEL did not terminate in 'failed' within ${DEADLINE}s (state=$state)."
    echo "      The restart budget is not bounding this failure mode; it would restart forever."
    sudo journalctl -u localdns.service --since "$since" --no-pager | tail -40 || true
    exit 1
fi

# The refusal line is the authoritative signal that the limiter -- not some unrelated
# failure -- is what ended the restart loop. Result= is NOT checked: systemd reports the
# underlying cause here (exit-code / watchdog / timeout), never start-limit-hit.
if ! sudo journalctl -u localdns.service --since "$since" --no-pager | grep -q 'Start request repeated too quickly'; then
    echo "FAIL: $LABEL reached 'failed' without the start limiter refusing a start."
    echo "      Something other than the restart budget produced the terminal state."
    exit 1
fi

# Exactly the burst should have run: more means the budget is not being enforced, fewer
# means the unit gave up for an unrelated reason.
if [ "$starts" -lt "$burst" ]; then
    echo "FAIL: $LABEL ran $starts ExecStarts, expected at least the burst of $burst"
    exit 1
fi

echo "OK: $LABEL terminated in 'failed' after $starts ExecStarts in ${elapsed}s"

# Restore for the next mode: drop the fault, clear the budget, and confirm the service and
# node DNS actually come back.
sudo rm -f ` + localdnsFaultFile + `
sudo systemctl reset-failed localdns.service || true
sudo systemctl start localdns.service
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if sudo systemctl is-active --quiet localdns.service; then
        break
    fi
    sleep 1
done
if ! sudo systemctl is-active --quiet localdns.service; then
    echo "FAIL: localdns did not recover after fault $FAULT"
    sudo systemctl status localdns.service --no-pager -l || true
    exit 1
fi
if ! dig +short +time=5 +tries=1 mcr.microsoft.com @169.254.10.10 | grep -q .; then
    echo "FAIL: node listener is not answering after recovery from fault $FAULT"
    exit 1
fi
`
}
