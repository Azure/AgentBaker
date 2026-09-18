package scenario

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
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
// Scope note: the slow modes run on shortened unit clocks so the matrix fits the
// VMSS-scoped validation budget. That means this validates the mechanism -- each mode
// reaches 'failed' because the limiter refused a start after the burst -- not the shipped
// wall-clock durations, which are measured separately and recorded on the PR.
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
	// what this mode actually took when last measured on the gate, in seconds. Data rather
	// than a comment so TestLocalDNSFaultMatrixFitsVMSSBudget can size the matrix against
	// TestTimeoutVMSS: the healthy run costs the sum of these, and only a regressing mode
	// costs its deadline.
	measuredSeconds int
}

// localdnsFaultMatrix is the full set of modes.
//
// Every fault is driven by a hook inside the patched localdns.sh rather than by a systemd
// drop-in. Inducing pre-flight and hung-start with ExecStartPre= would be simpler, but
// ExecStartPre short-circuits before localdns.sh runs, so the ExecStart counter would
// never increment and the burst assertion could not be made. Driving them from inside the
// script also keeps each fault on the real code path.
//
// The three slow modes run on shortened clocks (see installLocalDNSFaultHarness): at their
// shipped values they need ~19.6min between them, and the whole validation phase is bounded
// by TestTimeoutVMSS, which it shares with VM creation. The "shipped" column is what those
// modes take with the real clocks, measured by hand and recorded on the PR; the deadline is
// sized for the shortened run with roughly 2x headroom.
//
//	mode           shipped   shortened by
//	readytimeout     318s    wait_for_localdns_ready args 60/60 -> 8/8
//	watchdog         370s    WatchdogSec 60 -> 10
//	hungstart        625s    TimeoutStartSec 90 -> 15, TimeoutStopSec 30 -> 5
var localdnsFaultMatrix = []localdnsFault{
	{name: "preflight", label: "pre-flight abort", deadlineSeconds: 45, measuredSeconds: 11},
	{name: "resolvdrain", label: "resolv.conf never drains", deadlineSeconds: 80, measuredSeconds: 37},
	{name: "postready", label: "dies right after READY", deadlineSeconds: 120, measuredSeconds: 64},
	{name: "nopidfile", label: "PID file never appears", deadlineSeconds: 90, measuredSeconds: 30},
	{name: "readytimeout", label: "ready-check timeout", deadlineSeconds: 120, measuredSeconds: 75},
	{name: "watchdog", label: "watchdog pings cease", deadlineSeconds: 120, measuredSeconds: 90},
	{name: "hungstart", label: "hung start", deadlineSeconds: 180, measuredSeconds: 110},
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
	// Kept separate from localdnsFaultDropIn so the real-clock measurement can run with the
	// shipped timeouts before the matrix speeds them up.
	localdnsFastClockDropIn = "/run/systemd/system/localdns.service.d/99-e2e-fastclock.conf"
)

// localdnsWorstCycleCeilingSeconds bounds measureLocalDNSWorstCycle's poll. Exported as a
// constant so TestLocalDNSFaultMatrixFitsVMSSBudget can size the whole validation against
// TestTimeoutVMSS at build time.
const localdnsWorstCycleCeilingSeconds = 180

// laneResolvedMainBuiltImage reports whether this lane asked for an image built from main,
// rather than one built from the branch under test.
//
// This is the gate for the restart-budget validation, and it deliberately reads the lane's
// own image selection rather than probing the node for the directives. Probing the node
// would key the skip on a value the validation itself asserts, so the assertion could only
// run on images where it was already guaranteed to pass -- and a later retune of the budget
// would silently turn the whole scenario off on every lane instead of failing it.
//
// The gate lane sets SIG_VERSION_TAG_NAME=buildId / SIG_VERSION_TAG_VALUE=<vhd build id>
// from VHD_BUILD_ID (.pipelines/scripts/e2e_run.sh:83-88) and therefore tests this PR's own
// VHDs. Everything else falls back to the default branch=refs/heads/main selector, which
// resolves a main-built image that legitimately still ships the systemd default 10s/5
// budget: localdns.service is baked into the VHD (vhdbuilder/packer/packer_source.sh), not
// delivered through CustomData. .pipelines/e2e.yaml triggers on any PR touching e2e/** and
// runs in exactly that configuration.
//
// A lane pointed at some other branch's image asserts rather than skips: if you asked for a
// specific branch's VHD, a missing budget there is a real failure worth hearing about.
func laneResolvedMainBuiltImage() bool {
	return config.Config.SIGVersionTagName == "branch" &&
		config.Config.SIGVersionTagValue == "refs/heads/main"
}

// validateLocalDNSRestartBudget asserts that the unit's restart budget terminates each
// supplied failure mode in 'failed', and leaves LocalDNS healthy afterwards.
func validateLocalDNSRestartBudget(ctx context.Context, s *Scenario, faults []localdnsFault) error {
	if len(faults) == 0 {
		return fmt.Errorf("no LocalDNS faults selected: the fault matrix is misconfigured")
	}

	if laneResolvedMainBuiltImage() {
		logging.Logf(ctx, "SKIP: this lane resolved a main-built image (%s=%s), which predates the "+
			"LocalDNS restart budget baked into the VHD; run against the PR's VHD build to exercise it",
			config.Config.SIGVersionTagName, config.Config.SIGVersionTagValue)
		return nil
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
	// running the patched script or a temporary drop-in. This runs even when ctx is already
	// cancelled -- a scenario deadline or a dropped exec is exactly the path where cleanup
	// matters most, and reusing ctx would make this return immediately, leaving the fault
	// file, the patched script and the drop-in behind for whatever runs next. WithoutCancel
	// keeps the exec's credentials and values; the timeout stops teardown hanging forever.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		_, _ = execScriptOnVMForScenario(cleanupCtx, s, localdnsFaultTeardownScript)
	}()

	// Ground truth before anything is sped up: measure one real failure cycle against the
	// shipped timeouts and check it fits under the threshold. The margin assertion above is
	// arithmetic on a model of the unit; this is the machine's own answer. The matrix below
	// runs on shortened clocks and cannot check this.
	if err := measureLocalDNSWorstCycle(ctx, s); err != nil {
		return err
	}

	if err := installLocalDNSFastClocks(ctx, s); err != nil {
		return fmt.Errorf("install shortened fault clocks: %w", err)
	}

	for _, fault := range faults {
		if err := runLocalDNSFault(ctx, s, fault); err != nil {
			return err
		}
	}
	return nil
}

// installLocalDNSFastClocks shortens WatchdogSec, TimeoutStartSec and TimeoutStopSec for the
// matrix.
//
// At their shipped values (60s, 90s and 30s) the watchdog and hung-start modes need ~16
// minutes between them to exhaust the burst, which does not fit the VMSS-scoped validation
// budget that TestTimeoutVMSS shares with VM creation. Shortened, the same code paths still
// run -- a real watchdog timeout, a real start timeout, SIGTERM, restart, limiter -- on a
// faster clock.
//
// TimeoutStopSec is shortened for a non-obvious reason: hung-start spends its stop phase in
// full. The fault holds a foreground 'sleep infinity', and localdns.sh now traps TERM, so
// bash queues the handler until the foreground child returns -- which it never does. Nothing
// answers the SIGTERM and systemd waits out TimeoutStopSec before SIGKILLing the cgroup on
// every cycle. Before the TERM trap existed an untrapped SIGTERM killed bash immediately and
// the stop phase cost ~6s, which is why the by-hand measurement recorded on the PR (487s)
// is lower than what this now costs shipped (~625s). At 30s the shortened cycle is 47s and
// five of them do not fit any sane deadline; at 5s it is 22s, which is what the matrix's
// hungstart deadline is sized against.
//
// StartLimitIntervalSec, StartLimitBurst and RestartSec are never touched: they are what is
// under test. The sizing this shortening stops exercising is covered by the margin
// assertion and by measureLocalDNSWorstCycle, both of which run before this.
func installLocalDNSFastClocks(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, `
set -eu
printf '[Service]\nWatchdogSec=10\nTimeoutStartSec=15\nTimeoutStopSec=5\n' | sudo tee `+localdnsFastClockDropIn+` >/dev/null
sudo systemctl daemon-reload
echo "shortened fault clocks installed"
`, 0, "failed to install the shortened fault clocks")
	return err
}

// measureLocalDNSWorstCycle times one real restart cycle of the slowest failure mode and
// asserts it fits under the budget's threshold.
//
// This is the design claim checked against reality rather than against the formula. Waiting
// for the full burst would cost ~8 minutes; two consecutive ExecStarts give the cycle
// length, which is the quantity the budget is sized against, in about 100 seconds.
func measureLocalDNSWorstCycle(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsWorstCycleScript, 0,
		"LocalDNS worst restart cycle does not fit under the start-limit threshold")
	return err
}

var localdnsWorstCycleScript = `
set -eu

usec() { systemd-analyze timespan "$1" 2>/dev/null | sed -n '2s/.*: *//p'; }
interval_us=$(usec "$(systemctl show localdns.service -p StartLimitIntervalUSec --value)")
burst_n=$(systemctl show localdns.service -p StartLimitBurst --value)
threshold_s=$(( interval_us / burst_n / 1000000 ))

echo "=== measuring the real worst restart cycle (hung start, shipped clocks) ==="
echo "threshold is ${threshold_s}s"

sudo systemctl stop localdns.service 2>/dev/null || true
sudo systemctl reset-failed localdns.service 2>/dev/null || true
echo hungstart | sudo tee ` + localdnsFaultFile + ` >/dev/null
sudo rm -f ` + localdnsFaultCounter + `
sudo systemctl start --no-block localdns.service || true

# Two ExecStarts is all that is needed. Worst case here is TimeoutStartSec 90 +
# TimeoutStopSec 30 + RestartSec 2 = ~122s. The ceiling is deliberately close to that
# rather than generous: this poll shares TestTimeoutVMSS with VM creation and the whole
# fault matrix, and if the scenario runs out of budget the VMSS timeout pre-empts the
# per-fault deadlines and you lose the specific diagnostic they exist to produce. On the
# success path it breaks early anyway. Keep it in step with localdnsWorstCycleCeilingSeconds.
observed=0
for i in $(seq 1 ` + fmt.Sprint(localdnsWorstCycleCeilingSeconds) + `); do
    observed=$(sudo grep -c EXECSTART ` + localdnsFaultCounter + ` 2>/dev/null || echo 0)
    [ "$observed" -ge 2 ] && break
    sleep 1
done

# Abort the run: the remaining starts would only repeat the same cycle.
sudo rm -f ` + localdnsFaultFile + `
sudo systemctl stop localdns.service 2>/dev/null || true
sudo systemctl reset-failed localdns.service 2>/dev/null || true

if [ "$observed" -lt 2 ]; then
    echo "FAIL: only observed $observed ExecStart(s) in 240s; cannot measure a restart cycle"
    sudo systemctl status localdns.service --no-pager -l || true
    exit 1
fi

t1=$(sudo sed -n '1p' ` + localdnsFaultCounter + ` | awk '{print $1}')
t2=$(sudo sed -n '2p' ` + localdnsFaultCounter + ` | awk '{print $1}')
cycle_s=$(( t2 - t1 ))
echo "measured worst restart cycle: ${cycle_s}s (threshold ${threshold_s}s)"

if [ "$cycle_s" -ge "$threshold_s" ]; then
    echo "FAIL: the slowest failure cycle (${cycle_s}s) is not under the threshold (${threshold_s}s)."
    echo "      A failure on this cycle never accumulates StartLimitBurst starts inside the"
    echo "      window, so it would restart forever instead of terminating in 'failed'."
    exit 1
fi

# Leave the service healthy for the matrix that follows.
sudo systemctl start localdns.service
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    sudo systemctl is-active --quiet localdns.service && break
    sleep 1
done
sudo systemctl is-active --quiet localdns.service || {
    echo "FAIL: localdns did not recover after the worst-cycle measurement"
    exit 1
}
`

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

# Restart= is not part of this PR, but the whole matrix depends on it: with Restart=no the
# unit goes terminal after a single start and every fault would report "reached 'failed'
# without the start limiter refusing a start". An earlier e2e validation leaving a
# Restart=no drop-in behind caused exactly that, so assert it here where the message is
# unambiguous rather than letting each fault fail confusingly.
check Restart "on-failure"

# The threshold is StartLimitIntervalSec/StartLimitBurst = 720/5 = 144s, which only clears
# the slowest restart cycle at TimeoutStartSec=90s. That is pinned in the unit rather than
# inherited, because the manager default is a systemd build-time constant that differs by
# image -- Ubuntu builds it at 90s, Azure Linux at 45s -- so an inherited value made the
# margin, and this assertion, distro-dependent. Assert the pin so dropping it fails here.
check TimeoutStartUSec "1min 30s"

# The other half of the same problem, and the one that actually bit. Azure Linux ships a
# global service.d drop-in setting TimeoutStopFailureMode=abort, which makes a stop timeout
# send SIGABRT and then wait a SECOND TimeoutStopSec before SIGKILL. The stop side of the
# cycle is doubled, and the arithmetic below -- which models it as one TimeoutStopSec -- has
# no way to see that. Measured on AzureLinuxV3: 153s against a 144s threshold. The unit pins
# 'terminate' to take the distro's drop-in out of the cycle; assert the pin, because if it is
# dropped the margin check below still reports a comfortable 122s while the machine is at 153s.
check TimeoutStopFailureMode "terminate"

# The design rule itself, as arithmetic on the live values rather than on the numbers this
# comment happens to quote.
#
#   threshold   = StartLimitIntervalSec / StartLimitBurst
#   worst cycle = TimeoutStartSec + TimeoutStopSec + RestartSec
#
# A failure repeating more slowly than the threshold never accumulates StartLimitBurst
# starts inside the window, so it restarts forever instead of reaching 'failed' -- exactly
# the behaviour this PR exists to prevent. Checking every input this way means any change
# to the interval, the burst, either timeout, or RestartSec that closes the margin fails
# here, instead of silently shipping a budget that cannot catch the slow modes.
usec() { systemd-analyze timespan "$1" 2>/dev/null | sed -n '2s/.*: *//p'; }

interval_us=$(usec "$(systemctl show localdns.service -p StartLimitIntervalUSec --value)")
tstart_us=$(usec  "$(systemctl show localdns.service -p TimeoutStartUSec        --value)")
tstop_us=$(usec   "$(systemctl show localdns.service -p TimeoutStopUSec         --value)")
restart_us=$(usec "$(systemctl show localdns.service -p RestartUSec             --value)")
burst_n=$(systemctl show localdns.service -p StartLimitBurst --value)

if [ -z "$interval_us" ] || [ -z "$tstart_us" ] || [ -z "$tstop_us" ] || [ -z "$restart_us" ] \
   || [ -z "$burst_n" ] || [ "$burst_n" -le 0 ] 2>/dev/null; then
    echo "FAIL: could not read the restart-budget inputs needed for the margin check"
    echo "      interval=[$interval_us] burst=[$burst_n] tstart=[$tstart_us] tstop=[$tstop_us] restart=[$restart_us]"
    fail=1
else
    threshold_us=$(( interval_us / burst_n ))
    worst_us=$(( tstart_us + tstop_us + restart_us ))
    echo "margin: threshold $((threshold_us/1000000))s vs worst cycle $((worst_us/1000000))s"
    if [ "$threshold_us" -le "$worst_us" ]; then
        echo "FAIL: threshold $((threshold_us/1000000))s does not clear the worst restart cycle $((worst_us/1000000))s"
        echo "      (TimeoutStartSec $((tstart_us/1000000))s + TimeoutStopSec $((tstop_us/1000000))s + RestartSec $((restart_us/1000000))s)"
        echo "      A failure on that cycle would restart forever rather than terminate in 'failed'."
        fail=1
    fi
fi

if [ "$fail" -ne 0 ]; then
    echo
    echo "localdns.service is baked into the VHD (vhdbuilder/packer/packer_source.sh), not"
    echo "delivered through CustomData, so these directives only exist on an image built"
    echo "from a branch that carries them."
    echo
    echo "This validation is gated on the lane, not on the node: laneResolvedMainBuiltImage()"
    echo "skips it when the lane asked for a main-built image. Reaching this message therefore"
    echo "means the lane asked for THIS branch's VHD and the image it got does not have the"
    echo "budget -- either it was removed, a drop-in overrode it, or it is inherited on this"
    echo "distro when the unit should be pinning it."
    echo
    echo "TimeoutStartUSec and TimeoutStopFailureMode are both pinned by the unit precisely"
    echo "because their defaults are systemd build-time constants that differ by image:"
    echo "Ubuntu builds the start timeout at 90s and Azure Linux at 45s, and Azure Linux also"
    echo "ships a global TimeoutStopFailureMode=abort drop-in that doubles the stop side."
fi

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
    # timeout 30 mirrors the production path exactly (cse_config_localdns.sh). A longer
    # bound here would let a restart that takes 30-60s pass this test while still failing
    # node provisioning, which is the regression this is meant to catch.
    if ! sudo timeout 30 systemctl restart localdns.service; then
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
# 'sudo cat > file' rather than 'sudo cp': cp creates the copy root-owned and, under
# root's umask, mode 0750. This script runs unprivileged (hence sudo everywhere), so the
# greps and awks below could not read a root-only copy -- grep would fail with permission
# denied, print nothing to stdout, and the anchor count would come back empty rather than
# a number. Redirecting makes the copy owned by the calling user.
sudo cat "$SRC" > "$WORK/script"

# insert_hook <anchor-line> <before|after> <block-file>
#
# Matches the anchor as a whole line and requires exactly one occurrence. If localdns.sh
# moves, this fails loudly rather than silently inserting nothing and leaving a test that
# always passes.
insert_hook() {
    anchor=$1; pos=$2; blockfile=$3
    count=$(grep -c -x -F "$anchor" "$WORK/script" 2>/dev/null || true)
    # Distinguish "could not read the file" from "did not match": an unreadable file makes
    # grep print nothing, leaving count empty, which otherwise surfaces as a confusing
    # "matched  lines" and looks like a missing anchor.
    case "$count" in
        ''|*[!0-9]*)
            echo "ANCHOR-FAIL: could not count '$anchor' in $WORK/script (got [$count])"
            ls -la "$WORK/script" || true
            exit 1
            ;;
    esac
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

# (3b) Shorten the readiness wait for the readytimeout mode only. The real polling loop
# still runs and still times out on its own; it just does not need a full 60s per cycle to
# prove the point.
cat > "$WORK/b3b" <<'HOOK'
    if [ "$(cat /run/localdns-e2e-fault 2>/dev/null)" = "readytimeout" ]; then
        maxattempts=8; timeout_duration=8
    fi
HOOK
insert_hook '    local timeout_duration=$2' after "$WORK/b3b"

# (4) PID file never appears: repoint the wait loop at a path CoreDNS will not create so
# the real START_LOCALDNS_TIMEOUT=10 loop times out. COREDNS_COMMAND has already been built
# from the real path, so CoreDNS itself still behaves normally.
cat > "$WORK/b4" <<'HOOK'
if [ "${LDNS_FAULT}" = "nopidfile" ]; then
    echo "E2EFAULT nopidfile: pid file will never appear"
    LOCALDNS_PID_FILE=/run/localdns-e2e-never-appears.pid
    # the real wait loop still runs, on a shorter clock
    START_LOCALDNS_TIMEOUT=3
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
# ExecStart only. The clock shortening lives in a separate drop-in installed later, so the
# real-clock worst-cycle measurement can run against the shipped timeouts first.
printf '[Service]\nExecStart=\nExecStart=/bin/bash %s\n' "$DST" | sudo tee "$DROPIN" >/dev/null
sudo systemctl daemon-reload
echo "fault harness installed"
`

// localdnsFaultTeardownScript removes the harness and restores a healthy LocalDNS. It is
// best-effort by design: it runs from a defer, including on the failure path.
const localdnsFaultTeardownScript = `
set -u
sudo rm -f ` + localdnsFaultFile + ` ` + localdnsFaultCounter + ` ` + localdnsFaultDropIn + ` ` + localdnsFastClockDropIn + ` ` + localdnsFaultScript + `
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
    echo "      ExecStarts observed: $starts (expected the burst of $burst)."
    echo "--- unit properties ---"
    sudo systemctl show localdns.service -p Restart -p RestartUSec -p StartLimitIntervalUSec \
        -p StartLimitBurst -p NRestarts -p Result -p ExecMainStatus --no-pager || true
    echo "--- journal for this fault ---"
    sudo journalctl -u localdns.service --since "$since" --no-pager | tail -40 || true
    echo "--- drop-ins in effect ---"
    ls -la /run/systemd/system/localdns.service.d/ 2>/dev/null || true
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
