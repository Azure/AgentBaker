package scenario

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/e2e/logging"
)

// preferHostGAPluginFeature mirrors the EnabledFeatures key consumed by the VHD-baked
// aks-waagent-prefer-hostgaplugin.py hook.
const preferHostGAPluginFeature = "PREFER_HOSTGAPLUGIN"

// waagentAllowedDestinations are the only endpoints WALinuxAgent may connect to when it uses the
// HostGAPlugin channel: WireServer/HostGAPlugin and IMDS.
var waagentAllowedDestinations = map[string]bool{
	"168.63.129.16":   true,
	"169.254.169.254": true,
}

var waagentEgressLineRe = regexp.MustCompile(`^\s*(\d+)\s+DST=([0-9.]+)\s*$`)

// waagentEgressCaptureScript restarts the guest agent with an iptables LOG rule that records every
// outbound TCP connection attempt made from the agent's own cgroup (daemon + extension handler,
// which is where goal state, manifest, and extension package downloads happen). A restart makes the
// agent re-run protocol detection and goal state initialization, which is exactly where the direct
// storage download is attempted first by default. It prints "<count> DST=<ip>" per destination.
const waagentEgressCaptureScript = `set -uo pipefail
UNIT=walinuxagent.service
systemctl cat "$UNIT" >/dev/null 2>&1 || UNIT=waagent.service
CG=$(systemctl show -p ControlGroup --value "$UNIT")
CG=${CG#/}
PREFIX="E2E-WAAGENT-SYN: "
LOG="/var/log/waagent.log"
if ! sudo iptables -I OUTPUT 1 -p tcp --syn -m cgroup --path "$CG" -j LOG --log-prefix "$PREFIX"; then
  echo "failed to install iptables cgroup LOG rule for $CG" >&2
  exit 2
fi
cleanup() { sudo iptables -D OUTPUT -p tcp --syn -m cgroup --path "$CG" -j LOG --log-prefix "$PREFIX" || true; }
trap cleanup EXIT
OFFSET=$(sudo stat -c %s "$LOG")
START=$(date +%s)
sudo systemctl restart "$UNIT"
READY=0
for _ in $(seq 1 72); do
  if sudo tail -c +$((OFFSET + 1)) "$LOG" | grep -q "Goal state initialization completed"; then READY=1; break; fi
  sleep 5
done
if [ "$READY" != 1 ]; then
  echo "guest agent did not finish goal state initialization within 6 minutes after restart" >&2
  sudo tail -c +$((OFFSET + 1)) "$LOG" >&2
  exit 3
fi
# let the agent run a few goal state / status reporting iterations
sleep 20
echo "=== egress ==="
sudo journalctl -k -o cat --since "@$START" | grep -F "$PREFIX" | grep -o 'DST=[0-9.]*' | sort | uniq -c || true
echo "=== waagent.log since restart ==="
sudo tail -c +$((OFFSET + 1)) "$LOG" | grep -E "Fetch failed|HTTP Failed|Default channel changed" || true
`

type waagentEgress struct {
	destinations map[string]int
	logExcerpt   string
}

func (e waagentEgress) disallowed() []string {
	var out []string
	for ip, count := range e.destinations {
		if !waagentAllowedDestinations[ip] {
			out = append(out, fmt.Sprintf("%s (%d SYN)", ip, count))
		}
	}
	return out
}

func captureWaagentEgressAcrossRestart(ctx context.Context, s *Scenario) (waagentEgress, error) {
	res, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, waagentEgressCaptureScript, 0, "capture guest agent egress across restart")
	if err != nil {
		return waagentEgress{}, err
	}
	logging.Logf(ctx, "guest agent egress capture:\n%s", res.stdout)
	return parseWaagentEgress(res.stdout), nil
}

func parseWaagentEgress(stdout string) waagentEgress {
	egress := waagentEgress{destinations: map[string]int{}}
	section := ""
	var excerpt []string
	for _, line := range strings.Split(stdout, "\n") {
		switch strings.TrimSpace(line) {
		case "=== egress ===":
			section = "egress"
			continue
		case "=== waagent.log since restart ===":
			section = "log"
			continue
		}
		switch section {
		case "egress":
			if m := waagentEgressLineRe.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				egress.destinations[m[2]] += n
			}
		case "log":
			if strings.TrimSpace(line) != "" {
				excerpt = append(excerpt, line)
			}
		}
	}
	egress.logExcerpt = strings.Join(excerpt, "\n")
	return egress
}

// ValidateWaagentUsesHostGAPluginOnly proves that, with PREFER_HOSTGAPLUGIN enabled, the guest agent
// never tries to reach storage (umsa*/md-*.blob...) directly:
//  1. the VHD hook patched the agent,
//  2. the boot-time agent log has no direct-download failure and never had to fall back,
//  3. across an agent restart, every TCP connection attempt from the agent cgroup goes to
//     WireServer/HostGAPlugin (168.63.129.16) or IMDS only.
func ValidateWaagentUsesHostGAPluginOnly(ctx context.Context, s *Scenario) error {
	var errs []error

	hook, err := execScriptOnVMForScenario(ctx, s, `sudo journalctl -o cat -u walinuxagent.service -u waagent.service | grep -F "aks-waagent-prefer-hostgaplugin: " || true`)
	if err != nil {
		errs = append(errs, fmt.Errorf("read prefer-hostgaplugin hook output: %w", err))
	} else if !strings.Contains(hook.stdout, "aks-waagent-prefer-hostgaplugin: patched") {
		errs = append(errs, fmt.Errorf("expected the prefer-hostgaplugin hook to patch the guest agent, hook output:\n%s", hook.stdout))
	}

	errs = append(errs,
		ValidateFileExcludesContent(ctx, s, "/var/log/waagent.log", "Default channel changed to HostGAPlugin channel"),
		ValidateFileExcludesContent(ctx, s, "/var/log/waagent.log", "[HTTP Failed] GET https://"),
	)

	egress, err := captureWaagentEgressAcrossRestart(ctx, s)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	if len(egress.destinations) == 0 {
		errs = append(errs, errors.New("expected to observe guest agent connections to 168.63.129.16 across the restart, saw none; the egress capture is not working"))
	}
	if bad := egress.disallowed(); len(bad) > 0 {
		errs = append(errs, fmt.Errorf("guest agent attempted direct connections outside WireServer/IMDS: %s\nwaagent.log:\n%s", strings.Join(bad, ", "), egress.logExcerpt))
	}
	if strings.Contains(egress.logExcerpt, "Default channel changed") || strings.Contains(egress.logExcerpt, "HTTP Failed") {
		errs = append(errs, fmt.Errorf("guest agent fell back from a direct download after restart:\n%s", egress.logExcerpt))
	}
	return errors.Join(errs...)
}

// ValidateWaagentAttemptsDirectStorage is the negative control for ValidateWaagentUsesHostGAPluginOnly:
// without PREFER_HOSTGAPLUGIN, the same capture must observe the agent's blocked direct storage
// connection attempt, proving the capture can detect it.
func ValidateWaagentAttemptsDirectStorage(ctx context.Context, s *Scenario) error {
	egress, err := captureWaagentEgressAcrossRestart(ctx, s)
	if err != nil {
		return err
	}
	if len(egress.disallowed()) == 0 {
		return fmt.Errorf("expected the guest agent to attempt a direct (non-WireServer) connection without %s, observed destinations: %v\nwaagent.log:\n%s",
			preferHostGAPluginFeature, egress.destinations, egress.logExcerpt)
	}
	return nil
}
