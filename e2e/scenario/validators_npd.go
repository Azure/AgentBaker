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
# Specialized MAI images have the marker but do not use our config package.
if ! command -v dpkg-query >/dev/null || ! dpkg-query -W -f='${db:Status-Status}' node-problem-detector-aks-config 2>/dev/null | grep -qx installed; then
  echo "marker present without AgentBaker config package; skipping"
  exit 0
fi
systemctl is-enabled node-problem-detector.service >/dev/null
for attempt in $(seq 1 30); do
  if systemctl is-active --quiet node-problem-detector.service && curl --noproxy '*' -fsS --max-time 2 http://127.0.0.1:20256/healthz >/dev/null; then
    echo "enabled and healthy: binary $(dpkg-query -W -f='${Version}' node-problem-detector-kubernetes 2>/dev/null)," \
      "config $(dpkg-query -W -f='${Version}' node-problem-detector-aks-config 2>/dev/null)"
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
