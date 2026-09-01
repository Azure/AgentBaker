package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Test_LocalDNSLifecycle validates the systemd process lifecycle on isolated
// VMs bootstrapped from each LocalDNS-capable Linux VHD used by E2E.
// Run this against a VHD built from the current branch; LocalDNS unit files
// are baked into the VHD and are not replaced by the bootstrap payload.
func Test_LocalDNSLifecycle(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests LocalDNS systemd lifecycle on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
					},
					Validator: validateLocalDNSLifecycle,
				},
			})
		})
	}
}

func validateLocalDNSLifecycle(ctx context.Context, s *Scenario) {
	execScriptOnVMForScenarioValidateExitCode(ctx, s, `
set -eu
unit=/etc/systemd/system/localdns.service
delegate=/etc/systemd/system/localdns.service.d/delegate.conf

grep -qx 'KillMode=control-group' "$unit"
grep -qx 'Delegate=no' "$delegate"
systemctl is-active --quiet localdns.service
systemctl restart localdns.service
systemctl is-active --quiet localdns.service

# Verify the normal stop path completes before the service is started again.
systemctl stop localdns.service
systemctl is-inactive --quiet localdns.service
systemctl start localdns.service
systemctl is-active --quiet localdns.service

for i in 1 2 3 4 5 6 7 8 9 10; do
    main=$(systemctl show -p MainPID --value localdns.service)
    if [ "$main" -gt 0 ]; then
        kill -9 "$main" || true
    fi
    sleep 0.25
done

sleep 2
state=$(systemctl show localdns.service -p ActiveState -p SubState -p Result -p NRestarts -p ControlGroup)
printf '%s\n' "$state"
printf '%s\n' "$state" | grep -q '^ActiveState=active$'
printf '%s\n' "$state" | grep -q '^SubState=running$'
printf '%s\n' "$state" | grep -q '^Result=success$'
! journalctl -u localdns.service --since '2 minutes ago' --no-pager | grep -E 'Failed to kill control group|Start request repeated too quickly|Failed to start localdns.service'
`, 0, "LocalDNS lifecycle validation failed")
}
