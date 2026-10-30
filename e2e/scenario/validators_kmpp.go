package scenario

import (
	"context"
	"fmt"

	"github.com/Azure/agentbaker/e2e/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kmppUnitFile  = "/usr/lib/systemd/system/dbus-com.microsoft.kmpp.service"
	kmppNodeLabel = "kubernetes.azure.com/kmpp"
)

// ValidateKMPP checks KMPP on images that bake it: running and reachable through
// /var/run/kmpp/dbus when EnableKMPP is set, dormant otherwise.
func ValidateKMPP(ctx context.Context, s *Scenario) error {
	baked, err := fileExist(ctx, s, kmppUnitFile)
	if err != nil {
		return err
	}
	if !baked {
		logging.Log(ctx, "KMPP is not baked into this image; skipping")
		return nil
	}
	if s.Runtime.NBC != nil && s.Runtime.NBC.EnableKMPP {
		return validateKMPPEnabled(ctx, s)
	}
	return validateKMPPDormant(ctx, s)
}

func validateKMPPEnabled(ctx context.Context, s *Scenario) error {
	const script = `set -ex
sudo systemctl is-active dbus-com.microsoft.kmpp.service run-kmpp-dbus.mount
sudo systemctl is-enabled dbus-com.microsoft.kmpp.service run-kmpp-dbus.mount
sudo test -S /var/run/kmpp/dbus/system_bus_socket
sudo test -d /var/opt/msft/ap/data/kmpp/certs
sudo busctl --address=unix:path=/var/run/kmpp/dbus/system_bus_socket call com.microsoft.kmpp / com.microsoft.kmpp GetVersion
# Process mode, even on a vTPM node: the package default unit, kmpp not in tss.
! grep -q DeviceAllow /usr/lib/systemd/system/dbus-com.microsoft.kmpp.service
! id -nG kmpp | tr ' ' '\n' | grep -qx tss
# KMPP must not become the system OpenSSL default provider.
! grep -qi kmpp /etc/pki/tls/openssl.cnf
`
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "KMPP must be running and reachable through /var/run/kmpp/dbus"); err != nil {
		return err
	}
	return ValidateNodeHasLabel(ctx, s, kmppNodeLabel, "enabled")
}

func validateKMPPDormant(ctx context.Context, s *Scenario) error {
	const script = `set -ex
[ "$(systemctl is-enabled dbus-com.microsoft.kmpp.service || true)" = masked ]
[ "$(systemctl is-enabled dbus-com.microsoft.kmppctrl.service || true)" = masked ]
! systemctl is-active dbus-com.microsoft.kmpp.service
! mountpoint -q /run/kmpp/dbus
`
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "KMPP must stay masked and unexposed when EnableKMPP is false"); err != nil {
		return err
	}
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %q: %w", s.Runtime.VM.KubeName, err)
	}
	if v, ok := node.Labels[kmppNodeLabel]; ok {
		return fmt.Errorf("node %q has label %s=%s but EnableKMPP is false", s.Runtime.VM.KubeName, kmppNodeLabel, v)
	}
	return nil
}
