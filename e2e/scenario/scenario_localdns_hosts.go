package scenario

import (
	"context"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func init() {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "Ubuntu2604Minimal", vhd: config.VHDUbuntu2604MinimalGen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		tt := tt
		cluster := ClusterKubenet
		if tt.name == "Ubuntu2604Minimal" {
			cluster = ClusterLatestKubernetesVersionKubenet
		}
		Register(&Scenario{
			Name:        "LocalDNSHostsPlugin/" + tt.name,
			Description: "Tests that localdns hosts plugin works correctly on " + tt.name,
			Config: Config{
				Cluster: cluster,
				VHD:     tt.vhd,
				BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
					nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
					nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				},
				AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
					config.LocalDnsProfile.EnableHostsPlugin = true
					config.LocalDnsProfile.EnableLocalDns = true
				},
				VMConfigMutator: tt.vmConfigMutator,
				Validator: func(ctx context.Context, s *Scenario) error {
					// Validate the full LocalDNS service lifecycle (including the
					// unexpected-exit DNS teardown this PR fixes) on the target
					// distros. The hosts-plugin functionality itself is covered by
					// the scenario's default provisioning validation.
					if tt.name != "Ubuntu2204" && tt.name != "Ubuntu2404" && tt.name != "AzureLinuxV3" {
						return nil
					}
					// Cheapest check first. It is one 'systemctl show' and it is what
					// guards the pinned directives; running it after the lifecycle
					// validation meant a broken lifecycle step hid whether the unit was
					// pinned at all. See assertLocalDNSBudgetDirectivesEarly.
					if err := assertLocalDNSBudgetDirectivesEarly(ctx, s); err != nil {
						return err
					}
					if err := validateLocalDNSLifecycle(ctx, s); err != nil {
						return err
					}
					// Then assert the restart budget actually bounds failures.
					//
					// The full failure-mode matrix runs on Ubuntu2404 only: it is
					// systemd 255, where daemon-reload does not clear the start
					// limiter and where the provisioning regression was found. On
					// shortened clocks a healthy run of all seven modes costs ~7min,
					// and the sizing against TestTimeoutVMSS is asserted at build
					// time by TestLocalDNSFaultMatrixFitsVMSSBudget. The other
					// distros run the single discriminating mode instead (~40s) --
					// enough to catch the directives being dropped on those images.
					faults := localdnsDiscriminatingFault()
					if tt.name == "Ubuntu2404" {
						faults = localdnsFaultMatrix
					}
					return validateLocalDNSRestartBudget(ctx, s, faults)
				},
			},
		})
	}
}

// validateLocalDNSLifecycle exercises localdns.service end to end on the node: a normal
// stop/start, three supervisor kills recovered by Restart=on-failure, and the terminal
// dead-service case that #9360's ExecStopPost hook exists to handle.
//
// # Why the shell below is sparsely commented
//
// The script is SCP'd to the node, and the e2e's Bastion tunnel (bastionssh.go) forwards
// each SSH packet as a single websocket message with no chunking, against an 8,192-byte
// service cap. go-scp emits the body as a 4,096-byte chunk then the remainder, so
// max_write = script_size - 4096 + 45, and any script over 12,242 bytes kills the tunnel
// mid-run with StatusMessageTooBig. Build 181818040 lost three lanes to exactly that, after
// this script grew 512 bytes past a margin of 380. TestLocalDNSScriptsFitBastionLimit now
// guards every script this package sends.
//
// Comments cost the same as code on that wire and buy nothing at runtime, so the reasoning
// lives here instead. Keep the shell terse; put the "why" in this comment.
//
// # Lane gating (EXPECT_EXECSTOPPOST)
//
// Set from the lane's own image selection rather than probed from the node. Asking
// 'systemctl show -p ExecStopPost' whether to test ExecStopPost means the assertion only
// runs where it is already guaranteed to pass, and deleting the hook would silently turn
// the block off everywhere instead of failing it. See laneResolvedMainBuiltImage.
//
// # The EXIT trap
//
// Installed before any service mutation so 'set -e' cannot leave the node carrying the
// temporary Restart=no override or a failed unit.
//
// The override is removed unconditionally rather than behind '[ -f "$NORESTART" ]'. That
// test runs unprivileged, but sudo creates the drop-in inside a directory that root's umask
// makes 0750 root:root, so the guard could not stat the file, returned false, and skipped
// its own removal -- leaving Restart=no in effect for everything that ran afterwards on the
// node. 'rm -f' is already a no-op when the file is absent, so the guard bought nothing.
// The post-removal check uses 'sudo test -f' for the same reason.
//
// The restore retries rather than firing a single start. The kill block above can leave an
// orphaned CoreDNS holding 169.254.10.10:53 for a moment (the "Failed to kill control
// group" warning this test tolerates), and an immediate start then fails to bind. That is
// the exact transient RestartSec=2 exists to wait out in production, so wait for it here
// rather than failing the scenario on a cleanup race.
//
// # Kill/recovery cycles
//
// reset-failed runs before every kill. The budget (StartLimitBurst=5 within
// StartLimitIntervalSec=720) belongs to the unit and is shared by every actor that starts
// it -- CSE at provisioning, the validations above, and systemd's own Restart=on-failure.
// Without the reset, too few slots remain and the third kill's restart is refused with
// "Start request repeated too quickly", failing this loop for the wrong reason. What is
// under test here is Restart=on-failure, not the rate limiter.
//
// Recovery requires a genuinely new MainPID: immediately after kill -9 systemd may still
// report the killed invocation as active/running until it processes SIGCHLD, so checking
// active/running alone can observe the old process and falsely declare recovery.
//
// NRestarts is asserted >=1, not 3, because reset-failed zeroes it too -- it therefore
// reports the final cycle only. The three cycles are already proven individually by the
// new-MainPID requirement; this is a last check that the final kill was recovered by
// Restart=on-failure and not by something else.
//
// The StartLimit journal check is scoped with --since to these cycles: deliberately
// exhausting the budget is the expected outcome of validateLocalDNSRestartBudget, which
// runs after this returns. The cgroup teardown warning is surfaced but not failed on --
// fixing it is out of scope for this PR.
//
// # Terminal dead-service case
//
// The dead state is reached with a transient Restart=no drop-in and one kill, rather than
// by tripping StartLimit with rapid kills, which is timing-dependent and flaky.
// ExecStopPost runs on the SIGKILL path regardless of Restart=.
//
// Termination is awaited on ActiveState, not SubState: SubState passes through transitional
// values such as stop-post while ExecStopPost is still running the cleanup under test, so
// asserting on drop-in removal then would race it. ActiveState only becomes failed/inactive
// after ExecStopPost completes.
//
// The drop-in absence check uses 'sudo ls' because it concludes "absent" from a failed
// glob. If that directory were ever created root-only -- as this test's own drop-in
// directory is -- an unprivileged ls would fail, the check would read that as success, and
// the regression under test would pass silently. It is 0755 today; the assertion should not
// depend on that.
//
// The resolver check polls because networkctl reload propagates to systemd-resolved
// asynchronously, and it rejects empty or unreadable snapshots rather than treating them as
// "restored" -- a failed read would otherwise mask the regression. Removing the address is
// not sufficient on its own, so a working resolver is verified with getent afterwards.
func validateLocalDNSLifecycle(ctx context.Context, s *Scenario) error {
	expectExecStopPost := "true"
	if laneResolvedMainBuiltImage() {
		expectExecStopPost = "false"
	}
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		localdnsLifecycleScript(expectExecStopPost), 0, "LocalDNS lifecycle validation failed")
	return err
}

// localdnsLifecycleScript renders the script validateLocalDNSLifecycle runs on the node.
// Split out from the caller so TestLocalDNSScriptsFitBastionLimit can measure the assembled
// result -- see that test and this file's doc comment for why the size matters.
func localdnsLifecycleScript(expectExecStopPost string) string {
	return `
set -eu

EXPECT_EXECSTOPPOST=` + expectExecStopPost + `

NORESTART=/run/systemd/system/localdns.service.d/99-e2e-no-restart.conf

restore_localdns_test_state() {
    test_status=$?
    trap - EXIT
    set +e
    cleanup_status=0
    # Unconditional: an unprivileged [ -f ] cannot stat inside the 0750 root-owned dir.
    sudo rm -f "$NORESTART" || { echo "ERROR: failed to remove $NORESTART"; cleanup_status=1; }
    sudo systemctl daemon-reload || { echo "ERROR: systemd daemon-reload failed during test cleanup"; cleanup_status=1; }
    if sudo test -f "$NORESTART"; then
        echo "ERROR: $NORESTART still present after cleanup"
        cleanup_status=1
    fi
    # Retry: an orphaned CoreDNS can still hold 169.254.10.10:53 for a moment.
    for cleanup_attempt in 1 2 3 4 5 6; do
        sudo systemctl is-active --quiet localdns.service && break
        sudo systemctl reset-failed localdns.service || true
        sudo systemctl start localdns.service && break
        sleep 3
    done
    if ! sudo systemctl is-active --quiet localdns.service; then
        echo "ERROR: localdns.service is not active after test cleanup"
        sudo systemctl status localdns.service --no-pager -l || true
        cleanup_status=1
    fi
    if [ "$test_status" -eq 0 ] && [ "$cleanup_status" -ne 0 ]; then
        test_status=$cleanup_status
    fi
    exit "$test_status"
}
trap restore_localdns_test_state EXIT

sudo systemctl is-active --quiet localdns.service
control_group=$(sudo systemctl show localdns.service -p ControlGroup --value)
test "$control_group" = "/localdns.slice/localdns.service" || {
    echo "FAIL: expected LocalDNS ControlGroup=/localdns.slice/localdns.service, got $control_group"
    exit 1
}

# A normal stop must complete cleanup and return success.
sudo systemctl restart localdns.service
sudo systemctl is-active --quiet localdns.service
sudo systemctl stop localdns.service
test "$(sudo systemctl show localdns.service -p ActiveState --value)" = inactive
sudo systemctl start localdns.service
sudo systemctl is-active --quiet localdns.service

# Kill the supervisor three times; each must recover with a new MainPID.
test_start=$(date +%s)

for i in 1 2 3; do
    sudo systemctl reset-failed localdns.service || true
    killed=$(sudo systemctl show -p MainPID --value localdns.service)
    test "$killed" -gt 0
    sudo kill -9 "$killed"

    recovered=false
    for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
        state=$(sudo systemctl show localdns.service -p ActiveState -p SubState --value)
        main=$(sudo systemctl show -p MainPID --value localdns.service)
        if [ "$state" = $'active\nrunning' ] && [ "$main" -gt 0 ] && [ "$main" != "$killed" ]; then
            recovered=true
            break
        fi
        sleep 1
    done
    test "$recovered" = true
done

# reset-failed zeroes NRestarts too, so this reports the final cycle only.
restarts_after=$(sudo systemctl show localdns.service -p NRestarts --value)
test "$restarts_after" -ge 1 || {
    echo "FAIL: expected >=1 systemd restart after the final kill, got $restarts_after"
    exit 1
}

state=$(sudo systemctl show localdns.service -p ActiveState -p SubState -p Result -p ControlGroup)
printf '%s\n' "$state"
printf '%s\n' "$state" | grep -q '^ActiveState=active$'
printf '%s\n' "$state" | grep -q '^SubState=running$'
printf '%s\n' "$state" | grep -q '^Result=success$'
# Diagnostic only; not failed on.
if sudo journalctl -u localdns.service --since "@$test_start" --no-pager | grep -q 'Failed to kill control group'; then
    echo "WARNING: LocalDNS cgroup teardown warning observed"
fi
# Scoped by --since: budget exhaustion belongs to the restart-budget validation, not here.
if sudo journalctl -u localdns.service --since "@$test_start" --no-pager | grep -q 'Start request repeated too quickly'; then
    echo "LocalDNS reached systemd StartLimit during the kill/recovery cycles"
    exit 1
fi
dig +short +time=5 +tries=1 mcr.microsoft.com @169.254.10.10 | grep -q .

if [ "$EXPECT_EXECSTOPPOST" = true ]; then
# Assert the hook; do not use its presence as permission to look.
if ! sudo systemctl show localdns.service -p ExecStopPost --value | grep -q 'localdns.sh cleanup'; then
    echo "FAIL: ExecStopPost=localdns.sh cleanup is missing from localdns.service"
    sudo systemctl show localdns.service -p ExecStopPost || true
    exit 1
fi
# Terminal dead-service case: Restart=no drop-in, then one kill.
sudo mkdir -p "$(dirname "$NORESTART")"
printf '[Service]\nRestart=no\n' | sudo tee "$NORESTART" >/dev/null
sudo systemctl daemon-reload

dead_main=$(sudo systemctl show -p MainPID --value localdns.service)
test "$dead_main" -gt 0
sudo kill -9 "$dead_main"

# ActiveState, not SubState: SubState passes through stop-post mid-cleanup.
dead=false
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
    active_state=$(sudo systemctl show localdns.service -p ActiveState --value)
    if [ "$active_state" = failed ] || [ "$active_state" = inactive ]; then
        dead=true
        break
    fi
    sleep 1
done
test "$dead" = true

# sudo ls: "absent" concluded from a failed glob must not come from a permission error.
if sudo ls /run/systemd/network/*.d/70-localdns.conf >/dev/null 2>&1; then
    echo "FAIL: 70-localdns.conf still present after localdns died"
    exit 1
fi

# Poll: networkctl reload reaches systemd-resolved asynchronously. Empty reads are retried.
dns_reverted=false
resolver_state_readable=false
localdns_listener_present=false
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if command -v resolvectl >/dev/null 2>&1; then
        current_dns=$(resolvectl status 2>/dev/null) || current_dns=""
    else
        current_dns=$(cat /run/systemd/resolve/resolv.conf 2>/dev/null) || current_dns=""
    fi
    if [ -z "$current_dns" ]; then
        resolver_state_readable=false
    else
        resolver_state_readable=true
        if printf '%s' "$current_dns" | grep -q '169\.254\.10\.10'; then
            localdns_listener_present=true
        else
            localdns_listener_present=false
            dns_reverted=true
            break
        fi
    fi
    sleep 1
done
if [ "$dns_reverted" != true ]; then
    if [ "$resolver_state_readable" != true ]; then
        echo "FAIL: resolver state was empty or unreadable after localdns died"
    elif [ "$localdns_listener_present" = true ]; then
        echo "FAIL: link DNS still points at 169.254.10.10 after localdns died"
    else
        echo "FAIL: node DNS was not restored after localdns died"
    fi
    exit 1
fi

# Address removal alone is not sufficient; the node must actually resolve.
if ! getent hosts mcr.microsoft.com >/dev/null 2>&1; then
    echo "FAIL: node cannot resolve DNS after localdns died"
    exit 1
fi
else
    echo "SKIP: this lane resolved a main-built image, which predates the ExecStopPost cleanup hook"
fi
`
}
