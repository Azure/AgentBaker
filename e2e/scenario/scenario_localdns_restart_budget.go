package scenario

import (
	"context"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
)

// LocalDNS restart-budget validation.
//
// localdns.service carries StartLimitIntervalSec=720 / StartLimitBurst=5 / RestartSec=2 so
// that unrecoverable failures terminate in 'failed' rather than restarting forever. Only the
// terminal state lets an OnFailure= handoff fire and gives NPD a stable state to observe, so
// the directives are load-bearing -- but nothing guarded them: the lifecycle scenario does
// three kill/recovery cycles and would still pass if they were deleted.
//
// Two assertions, deliberately:
//
//  1. The budget is in localdns.service itself, not in a drop-in. 'systemctl show' reads
//     effective values and cannot tell those apart, so the file is grepped directly. The
//     budget living in the VHD unit is what makes the terminal-'failed' guarantee
//     unconditional; delivered any other way it depends on what else reached the node.
//  2. The unit actually reaches terminal 'failed' once the burst is spent inside the window.
//     A directive can be present and still not bound anything.

// laneResolvedMainBuiltImage reports whether this lane resolved a main-built image rather
// than one built from the branch under test.
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

// validateLocalDNSRestartBudget asserts that the unit carries the budget and that the budget
// terminates a failing start in 'failed', then leaves LocalDNS healthy again.
func validateLocalDNSRestartBudget(ctx context.Context, s *Scenario) error {
	if laneResolvedMainBuiltImage() {
		logging.Logf(ctx, "SKIP: this lane resolved a main-built image (%s=%s), which predates the "+
			"LocalDNS restart budget baked into the VHD; run against the PR's VHD build to exercise it",
			config.Config.SIGVersionTagName, config.Config.SIGVersionTagValue)
		return nil
	}

	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsBudgetDirectiveScript, 0,
		"LocalDNS restart budget directives are not as shipped"); err != nil {
		return err
	}

	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsTerminalFailedScript, 0,
		"LocalDNS did not reach terminal 'failed' after exhausting its restart budget")
	return err
}

// localdnsBudgetDirectiveScript asserts both where the budget comes from and what it
// resolves to on the node.
//
// The grep is the half that 'systemctl show' cannot do. Effective values are identical
// whether 720/5 came from localdns.service or from a drop-in written at provisioning time,
// so without it the one regression this scenario exists to prevent is invisible.
const localdnsBudgetDirectiveScript = `
set -uo pipefail

fail() { echo "FAIL: $*"; exit 1; }

UNIT=/etc/systemd/system/localdns.service

grep -qE '^StartLimitIntervalSec=720$' "$UNIT" ||
    fail "StartLimitIntervalSec=720 is not in localdns.service. The budget must ship in the VHD unit: delivered any other way, whether a node gets it depends on what else reached that node."
grep -qE '^StartLimitBurst=5$' "$UNIT" ||
    fail "StartLimitBurst=5 is not in localdns.service (see above)."

check() {
    actual=$(systemctl show localdns.service -p "$1" --value)
    [ "$actual" = "$2" ] || fail "$1 is '$actual', expected '$2'"
}

check StartLimitIntervalUSec "12min"
check StartLimitBurst        "5"
check RestartUSec            "2s"
check Restart                "on-failure"

echo "OK: restart budget ships in localdns.service and resolves as expected"
`

// localdnsTerminalFailedScript spends the burst against a start that always fails and
// asserts the unit lands in terminal 'failed' because the limiter refused a further start.
//
// Measured, and it shapes the assertion: systemd does NOT report Result=start-limit-hit
// here. It reports the underlying cause -- exit-code for this fault -- and the journal
// carries "Start request repeated too quickly." So the terminal state is asserted via
// ActiveState plus the journal refusal line plus the start count, and Result is printed
// as diagnostic only.
//
// ExecStart=/bin/false keeps the cycle at RestartSec (2s), so five starts and the refused
// sixth complete in ~12s -- well inside the 720s window, and cheap enough to run on every
// distro rather than only where a shortened clock was installed.
//
// The drop-in lives in /run so a reboot cannot leave it behind, and is named to sort after
// anything shipped in the unit directory. Teardown runs unconditionally, including on the
// failure paths, because leaving a /bin/false ExecStart on the node would break every
// later validator in the scenario.
const localdnsTerminalFailedScript = `
set -uo pipefail

DROPIN_DIR=/run/systemd/system/localdns.service.d
DROPIN="$DROPIN_DIR/zz-e2e-fault.conf"

teardown() {
    sudo rm -f "$DROPIN"
    sudo systemctl daemon-reload
    sudo systemctl reset-failed localdns.service 2>/dev/null || true
    sudo systemctl restart localdns.service || true
}
trap teardown EXIT

fail() { echo "FAIL: $*"; exit 1; }

sudo mkdir -p "$DROPIN_DIR"
printf '[Service]\nExecStart=\nExecStart=/bin/false\n' | sudo tee "$DROPIN" >/dev/null
sudo systemctl daemon-reload
sudo systemctl reset-failed localdns.service 2>/dev/null || true

fault_start=$(date +%s)

# restart, not start: validateLocalDNSLifecycle runs immediately before this
# (scenario_localdns_hosts.go) and can only exit with the unit active -- its EXIT trap
# retries 'systemctl start' until is-active. daemon-reload does not restart anything, so
# 'start' on an active unit is a no-op and the faulted ExecStart would never run.
# --no-block: the start fails immediately, but do not depend on that to return.
sudo systemctl restart localdns.service --no-block 2>/dev/null || true

state=""
result=""
for _ in $(seq 1 120); do
    state=$(systemctl show localdns.service -p ActiveState --value)
    result=$(systemctl show localdns.service -p Result --value)
    [ "$state" = "failed" ] && break
    sleep 1
done

[ "$state" = "failed" ] ||
    fail "localdns is '$state' after 120s of failing starts, expected 'failed'. The budget is not bounding this failure, so an unrecoverable fault would restart forever and never reach the state NPD and OnFailure= depend on."

nrestarts=$(systemctl show localdns.service -p NRestarts --value)
echo "diagnostic: Result=$result NRestarts=$nrestarts"

# The limiter, not some other failure, is what stopped it. systemd reports the underlying
# cause in Result, so the refusal only shows up in the journal.
#
# Anchored to fault_start rather than a wall-clock window: the lifecycle validator runs
# three kill/recovery cycles on this same unit just before, and a refusal from those would
# otherwise satisfy this grep.
sudo journalctl -u localdns.service --since "@$fault_start" --no-pager |
    grep -q "Start request repeated too quickly" ||
    fail "localdns reached 'failed' but the journal has no 'Start request repeated too quickly'. It failed for some other reason, so this run did not exercise the budget."

burst=$(systemctl show localdns.service -p StartLimitBurst --value)
[ "$nrestarts" -ge "$burst" ] ||
    fail "localdns restarted $nrestarts times against a burst of $burst; the limiter did not bound this failure."

echo "OK: localdns reached terminal 'failed' via the start limiter"
`
