package e2e

import (
	"context"
	"fmt"
)

// ValidateBakedNPD checks the package-owned installation. Image content tests
// require the marker on supported builds; older images remain extension-managed.
func ValidateBakedNPD(ctx context.Context, s *Scenario) error {
	exists, err := fileExist(ctx, s, "/etc/node-problem-detector.d/skip_vhd_npd")
	if err != nil || !exists {
		return err
	}
	const script = `set -e
# Specialized MAI images have the marker but do not use our config package.
if ! command -v dpkg-query >/dev/null || ! dpkg-query -W -f='${db:Status-Status}' node-problem-detector-aks-config 2>/dev/null | grep -qx installed; then
  exit 0
fi
systemctl is-enabled node-problem-detector.service
for attempt in $(seq 1 30); do
  if systemctl is-active --quiet node-problem-detector.service && curl --noproxy '*' -fsS --max-time 2 http://127.0.0.1:20256/healthz; then
    exit 0
  fi
  sleep 2
done
journalctl -u node-problem-detector.service --no-pager -n 50
exit 1
`
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "baked NPD must be enabled and healthy"); err != nil {
		return fmt.Errorf("validate baked NPD: %w", err)
	}
	return nil
}
