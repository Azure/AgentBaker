package e2e

import (
	"context"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Test_LocalDNSLifecycle validates unexpected-exit DNS cleanup on isolated VMs.
// Run this against a VHD built from the current branch because the LocalDNS
// unit files are baked into the VHD.
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
				Description: "Tests LocalDNS unexpected-exit cleanup on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
					},
					AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
						cfg.LocalDnsProfile.EnableLocalDns = true
					},
					Validator: validateLocalDNSLifecycle,
				},
			})
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
for i in 1 2 3; do
    main=$(sudo systemctl show -p MainPID --value localdns.service)
    test "$main" -gt 0
    sudo kill -9 "$main"

    recovered=false
    for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
        state=$(sudo systemctl show localdns.service -p ActiveState -p SubState --value)
        if [ "$state" = $'active\nrunning' ]; then
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
! sudo journalctl -u localdns.service --since '2 minutes ago' --no-pager | grep -E 'Failed to kill control group|Start request repeated too quickly|Failed to start localdns.service'
`, 0, "LocalDNS lifecycle validation failed")
	return err
}
