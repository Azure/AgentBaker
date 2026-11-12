package scenario

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/logging"
)

// ValidateBakedNPD checks the package-owned installation. Image content tests
// require the marker on supported builds; older images remain extension-managed.
func ValidateBakedNPD(ctx context.Context, s *Scenario) error {
	exists, err := fileExist(ctx, s, "/etc/node-problem-detector.d/skip_vhd_npd")
	if err != nil {
		return err
	}
	if !exists {
		logging.Log(ctx, "baked NPD: no skip marker; NPD remains extension-managed")
		return nil
	}
	const script = `set -e
package_version() {
  if command -v dpkg-query >/dev/null; then
    [ "$(dpkg-query -W -f='${db:Status-Status}' "$1" 2>/dev/null)" = installed ] || return 1
    dpkg-query -W -f='${Version}' "$1"
  else
    rpm -q --qf '%{VERSION}-%{RELEASE}' "$1" 2>/dev/null
  fi
}
# Specialized MAI images have the marker but do not use our config package.
if ! config_version="$(package_version node-problem-detector-aks-config)"; then
  echo "marker present without AgentBaker config package; skipping"
  exit 0
fi
systemctl is-enabled node-problem-detector.service >/dev/null
for attempt in $(seq 1 30); do
  if systemctl is-active --quiet node-problem-detector.service && curl --noproxy '*' -fsS --max-time 2 http://127.0.0.1:20256/healthz >/dev/null; then
    echo "enabled and healthy: binary $(package_version node-problem-detector-kubernetes), config ${config_version}"
    exit 0
  fi
  sleep 2
done
journalctl -u node-problem-detector.service --no-pager -n 50
exit 1
`
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "baked NPD must be enabled and healthy")
	if err != nil {
		return fmt.Errorf("validate baked NPD: %w", err)
	}
	logging.Logf(ctx, "baked NPD: %s", strings.TrimSpace(result.stdout))
	return nil
}
