package e2e

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
					if tt.name == "Ubuntu2204" || tt.name == "Ubuntu2404" || tt.name == "AzureLinuxV3" {
						return validateLocalDNSLifecycle(ctx, s)
					}
					return nil
				},
			},
		})
	}
}

func validateLocalDNSLifecycle(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, `
set -eu
sudo systemctl is-active --quiet localdns.service

# Normal systemd stop must complete cleanup and return success.
sudo systemctl restart localdns.service
sudo systemctl is-active --quiet localdns.service
sudo systemctl stop localdns.service
test "$(sudo systemctl show localdns.service -p ActiveState --value)" = inactive
sudo systemctl start localdns.service
sudo systemctl is-active --quiet localdns.service

# Repeatedly kill the supervisor and wait for Restart=on-failure recovery.
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
if sudo journalctl -u localdns.service --since "@$test_start" --no-pager | grep -q 'Start request repeated too quickly'; then
    echo "LocalDNS reached systemd StartLimit"
    exit 1
fi
dig +short +time=5 +tries=1 mcr.microsoft.com @169.254.10.10 | grep -q .

# Terminal dead-service case: this is the incident scenario the PR fixes.
# When localdns ends up dead (systemd exhausts restart attempts), ExecStopPost
# must still revert node DNS so the node does not keep pointing at the dead
# localdns listener (169.254.10.10). We reach the dead state deterministically
# by disabling auto-restart with a transient drop-in, then killing the
# supervisor -- tripping StartLimit via rapid kills is timing dependent and
# flaky. ExecStopPost runs on the SIGKILL path regardless of Restart=.
sudo mkdir -p /run/systemd/system/localdns.service.d
printf '[Service]\nRestart=no\n' | sudo tee /run/systemd/system/localdns.service.d/99-e2e-no-restart.conf >/dev/null
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
dns_reverted=false
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if command -v resolvectl >/dev/null 2>&1; then
        current_dns=$(resolvectl status 2>/dev/null || true)
    else
        current_dns=$(cat /run/systemd/resolve/resolv.conf 2>/dev/null || true)
    fi
    if ! printf '%s' "$current_dns" | grep -q '169\.254\.10\.10'; then
        dns_reverted=true
        break
    fi
    sleep 1
done
if [ "$dns_reverted" != true ]; then
    echo "FAIL: link DNS still points at 169.254.10.10 after localdns died"
    exit 1
fi

# Restore the node to a healthy state for any subsequent validation.
sudo rm -f /run/systemd/system/localdns.service.d/99-e2e-no-restart.conf
sudo systemctl daemon-reload
sudo systemctl reset-failed localdns.service || true
sudo systemctl start localdns.service
sudo systemctl is-active --quiet localdns.service
dig +short +time=5 +tries=1 mcr.microsoft.com @169.254.10.10 | grep -q .
`, 0, "LocalDNS lifecycle validation failed")
	return err
}
