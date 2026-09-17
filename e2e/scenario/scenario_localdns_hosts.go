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
					if err := validateLocalDNSLifecycle(ctx, s); err != nil {
						return err
					}
					// Then assert the restart budget actually bounds failures.
					//
					// The full failure-mode matrix runs on Ubuntu2404 only: it is
					// systemd 255, where daemon-reload does not clear the start
					// limiter and where the provisioning regression was found. It
					// costs ~23min, so the other distros run the single
					// discriminating mode instead (~40s) -- enough to catch the
					// directives being dropped on those images.
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

func validateLocalDNSLifecycle(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, `
set -eu

NORESTART=/run/systemd/system/localdns.service.d/99-e2e-no-restart.conf

# Install cleanup before any service mutation so set -e cannot leave the node
# with the temporary Restart=no override or a failed LocalDNS unit.
restore_localdns_test_state() {
    test_status=$?
    trap - EXIT
    set +e
    cleanup_status=0
    if [ -f "$NORESTART" ]; then
        sudo rm -f "$NORESTART" || { echo "ERROR: failed to remove $NORESTART"; cleanup_status=1; }
        sudo systemctl daemon-reload || { echo "ERROR: systemd daemon-reload failed during test cleanup"; cleanup_status=1; }
    fi
    # The restart loop can hit systemd's start limit without creating NORESTART.
    # Clear any failed state before trying to start LocalDNS; this is best-effort
    # so a reset failure does not prevent the rest of cleanup.
    sudo systemctl reset-failed localdns.service || true
    if ! sudo systemctl is-active --quiet localdns.service; then
        sudo systemctl start localdns.service || { echo "ERROR: failed to restart localdns.service during test cleanup"; cleanup_status=1; }
    fi
    if ! sudo systemctl is-active --quiet localdns.service; then
        echo "ERROR: localdns.service is not active after test cleanup"
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

# Normal systemd stop must complete cleanup and return success.
sudo systemctl restart localdns.service
sudo systemctl is-active --quiet localdns.service
sudo systemctl stop localdns.service
test "$(sudo systemctl show localdns.service -p ActiveState --value)" = inactive
sudo systemctl start localdns.service
sudo systemctl is-active --quiet localdns.service

# Repeatedly kill the supervisor and wait for Restart=on-failure recovery.
# This verifies that systemd can restart LocalDNS; startup cleanup may restore
# DNS on this path, so it does not by itself validate the ExecStopPost fix.
# The terminal dead-service block below validates that fix directly.
# Require a genuinely new MainPID after each kill: immediately after kill -9,
# systemd may still report the killed invocation as active/running until it
# processes SIGCHLD, so checking active/running alone can observe the old
# process and falsely declare recovery. Save the killed PID and require the
# new MainPID to be nonzero and different from it.
test_start=$(date +%s)

for i in 1 2 3; do
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

restarts_after=$(sudo systemctl show localdns.service -p NRestarts --value)
# The loop above performs three kill/restart cycles. The manual start before
# the loop resets NRestarts to zero, so assert the absolute restart count.
test "$restarts_after" -ge 3 || {
    echo "FAIL: expected >=3 systemd restarts, got $restarts_after"
    exit 1
}

state=$(sudo systemctl show localdns.service -p ActiveState -p SubState -p Result -p ControlGroup)
printf '%s\n' "$state"
printf '%s\n' "$state" | grep -q '^ActiveState=active$'
printf '%s\n' "$state" | grep -q '^SubState=running$'
printf '%s\n' "$state" | grep -q '^Result=success$'
# The cgroup teardown warning is diagnostic only: fixing it is out of scope for
# this PR (which is about restoring node DNS after an unexpected exit), so we
# surface it but do not fail on it.
if sudo journalctl -u localdns.service --since "@$test_start" --no-pager | grep -q 'Failed to kill control group'; then
    echo "WARNING: LocalDNS cgroup teardown warning observed"
fi
# Three kills well inside the window must not exhaust the budget -- if they do, recovery
# from ordinary crashes is broken. Note this is scoped to the kill/recovery cycles above
# via --since: deliberately exhausting the budget is the expected outcome in the
# restart-budget validation, which runs separately after this function returns.
if sudo journalctl -u localdns.service --since "@$test_start" --no-pager | grep -q 'Start request repeated too quickly'; then
    echo "LocalDNS reached systemd StartLimit during the kill/recovery cycles"
    exit 1
fi
dig +short +time=5 +tries=1 mcr.microsoft.com @169.254.10.10 | grep -q .

if sudo systemctl show localdns.service -p ExecStopPost --value | grep -q 'localdns.sh cleanup'; then
# Terminal dead-service case: this is the incident scenario the PR fixes.
# When localdns ends up dead (systemd exhausts restart attempts), ExecStopPost
# must still revert node DNS so the node does not keep pointing at the dead
# localdns listener (169.254.10.10). We reach the dead state deterministically
# by disabling auto-restart with a transient drop-in, then killing the
# supervisor -- tripping StartLimit via rapid kills is timing dependent and
# flaky. ExecStopPost runs on the SIGKILL path regardless of Restart=.
sudo mkdir -p "$(dirname "$NORESTART")"
printf '[Service]\nRestart=no\n' | sudo tee "$NORESTART" >/dev/null
sudo systemctl daemon-reload

dead_main=$(sudo systemctl show -p MainPID --value localdns.service)
test "$dead_main" -gt 0
sudo kill -9 "$dead_main"

# Wait for the service to reach a terminal ActiveState (failed or inactive).
# A non-running SubState is not sufficient: SubState passes through transitional
# values such as stop-post while ExecStopPost is still running the cleanup under
# test, so asserting on drop-in removal then could race the cleanup. ActiveState
# only becomes failed/inactive after ExecStopPost has completed.
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

# The localdns network drop-in must have been removed by ExecStopPost. This is
# the authoritative signal that DNS was reverted: the drop-in is what points the
# link's DNS at the localdns listener.
if ls /run/systemd/network/*.d/70-localdns.conf >/dev/null 2>&1; then
    echo "FAIL: 70-localdns.conf still present after localdns died"
    exit 1
fi

# The live link DNS must no longer include the localdns node listener. This is
# eventually consistent: networkctl reload propagates to systemd-resolved
# asynchronously, so poll (like wait_for_localdns_removed_from_resolv_conf does)
# until the listener IP is gone rather than checking once. Prefer resolvectl
# (the per-link view the drop-in configures); fall back to the resolved stub.
# Only accept a successful, non-empty resolver snapshot: an errored or empty
# read must not be treated as "restored", or a failed read would mask the very
# regression under test. Retry those instead.
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

# Removing the LocalDNS address is not sufficient: verify the node has a
# working resolver after cleanup.
if ! getent hosts mcr.microsoft.com >/dev/null 2>&1; then
    echo "FAIL: node cannot resolve DNS after localdns died"
    exit 1
fi
else
    echo "SKIP: VHD predates the ExecStopPost cleanup hook"
fi

# The EXIT trap removes the temporary override and restores LocalDNS even if
# an assertion above exits the validation early.
`, 0, "LocalDNS lifecycle validation failed")
	return err
}
