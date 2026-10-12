package scenario

import (
	"context"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// One LocalDNS scenario with the hosts plugin OFF.
//
// The LocalDNS/<distro> scenarios all run with EnableHostsPlugin=true, which is the
// common pool shape but not the only one, and it changes which corefile variant
// localdns.sh regenerates on every start (WITH_HOSTS vs BASE). This keeps one node on
// the BASE variant so that path is exercised at all.
//
// It is also where the real startup failure lives. The LocalDNS/<distro> scenarios
// drive localdns to terminal 'failed' with ExecStart=/bin/false, which is fast and
// deterministic but never runs localdns.sh -- so nothing there covers the script's own
// failing-startup path: regenerate the corefile, hand it to CoreDNS, have CoreDNS
// reject it, time out waiting for a pidfile that never appears, and clean up. This
// scenario induces exactly that, once, with Restart=no so it costs one cycle rather
// than five.
func init() {
	Register(&Scenario{
		Name: "LocalDNSNoHostsPlugin/Ubuntu2204",
		Description: "Tests localdns with the hosts plugin disabled on Ubuntu2204, including a real " +
			"malformed-corefile startup failure and the cleanup it runs on the way out",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				nbc.KubeletConfig["--cluster-dns"] = localDNSClusterListenerIP
			},
			AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
				config.LocalDnsProfile.EnableLocalDns = true
				config.KubeletConfig.KubeletFlags["--cluster-dns"] = localDNSClusterListenerIP
				// See the note in scenario_localdns_hosts.go: the flag alone is not
				// parity with the CSE path, and the env parity validator fails on it.
				config.KubeletConfig.KubeletConfigFileConfig.ClusterDns = []string{localDNSClusterListenerIP}
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return validateLocalDNSCorefileStartupFailure(ctx, s)
			},
		},
	})
}

// validateLocalDNSCorefileStartupFailure drives localdns through one real startup
// failure of its own script and asserts the cleanup that failure is supposed to run.
//
// Why the fault is LOCALDNS_COREFILE_BASE and not localdns.corefile: regenerate_localdns_corefile
// rewrites the corefile from that base64 on every start, so corrupting the corefile
// self-heals and the node never breaks. The base64 is the source of truth.
//
// Why Restart=no: the point here is one honest trip through the script, not the
// limiter. The budget is asserted on the LocalDNS/<distro> nodes, which reach the same
// terminal state in ~12s with /bin/false rather than ~69s of pidfile timeouts.
func validateLocalDNSCorefileStartupFailure(ctx context.Context, s *Scenario) error {
	// No lane gate. What this asserts -- that localdns.sh runs its startup path, times
	// out waiting for a pidfile CoreDNS never writes, and that ExecStopPost restores
	// node DNS on the way out -- all ships on main already (#9360), so skipping it on
	// main-built images would suppress an assertion that passes.
	res, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, localdnsCorefileFaultScript, 0,
		"LocalDNS did not fail and clean up correctly on a malformed corefile")
	// Logged on success too. execScriptOnVMForScenarioValidateExitCode only prints on
	// failure, so a passing run was indistinguishable from a validator that returned
	// nil early -- confirming it had run meant reading the node's journal afterwards.
	if res != nil {
		logging.Logf(ctx, "localdns malformed-corefile validation output:\n%s", res.stdout)
	}
	return err
}

const localdnsCorefileFaultScript = `
set -uo pipefail

ENVF=/etc/localdns/environment
ENVBAK=/run/localdns-e2e-env.bak
NORESTART=/run/systemd/system/localdns.service.d/zz-e2e-no-restart.conf

fail() { echo "FAIL: $*"; exit 1; }

# Unconditional, and before any mutation: sudo creates the drop-in directory 0750
# root:root, so an unprivileged guard cannot stat what it is trying to remove.
teardown() {
    status=$?
    trap - EXIT
    set +e
    sudo rm -f "$NORESTART"
    [ -f "$ENVBAK" ] && sudo cp -a "$ENVBAK" "$ENVF"
    sudo systemctl daemon-reload
    sudo systemctl reset-failed localdns.service 2>/dev/null || true
    for _ in 1 2 3 4 5 6; do
        sudo systemctl is-active --quiet localdns.service && break
        sudo systemctl reset-failed localdns.service 2>/dev/null || true
        sudo systemctl start localdns.service && break
        sleep 3
    done
    if ! sudo systemctl is-active --quiet localdns.service; then
        echo "ERROR: localdns is not active after teardown"
        sudo systemctl status localdns.service --no-pager -l || true
        [ "$status" -eq 0 ] && status=1
    fi
    exit "$status"
}
trap teardown EXIT

sudo systemctl is-active --quiet localdns.service || fail "localdns is not healthy before the fault"
sudo cp -a "$ENVF" "$ENVBAK"

sudo mkdir -p "$(dirname "$NORESTART")"
printf '[Service]\nRestart=no\n' | sudo tee "$NORESTART" >/dev/null

bad=$(printf 'THIS IS NOT A VALID COREFILE {{{\n' | base64 -w0)
tmp=$(mktemp)
while IFS= read -r l; do
  case "$l" in LOCALDNS_COREFILE_BASE=*) echo "LOCALDNS_COREFILE_BASE=${bad}" ;; *) echo "$l" ;; esac
done < "$ENVF" > "$tmp"
sudo cp "$tmp" "$ENVF"; rm -f "$tmp"
sudo systemctl daemon-reload

fault_start=$(date +%s)
sudo systemctl restart --no-block localdns.service 2>/dev/null || true

# ActiveState, not SubState: SubState passes through stop-post while ExecStopPost is
# still running the cleanup under test, so asserting on it would race the thing we
# are about to check.
state=""
for _ in $(seq 1 90); do
    state=$(systemctl show localdns.service -p ActiveState --value)
    { [ "$state" = failed ] || [ "$state" = inactive ]; } && break
    sleep 1
done
{ [ "$state" = failed ] || [ "$state" = inactive ]; } ||
    fail "localdns is '$state' after 90s with a malformed corefile, expected failed/inactive"
echo "OK: a malformed corefile terminated localdns (ActiveState=$state)"

# The script got far enough to run its own startup path, rather than dying earlier.
# This is the coverage /bin/false cannot give: CoreDNS was handed the corefile and
# rejected it, so no pidfile ever appeared.
sudo journalctl -u localdns.service --since "@$fault_start" --no-pager |
    grep -F "Timed out waiting for CoreDNS to create PID file" >/dev/null ||
    fail "the journal has no pidfile timeout; localdns did not reach its CoreDNS startup path, so this did not exercise the script"
echo "OK: localdns ran its startup path and timed out waiting for a pidfile"

# sudo ls: concluding "absent" from a failed glob must not come from a permission error.
if sudo ls /run/systemd/network/*.d/70-localdns.conf >/dev/null 2>&1; then
    fail "70-localdns.conf survived a failed startup; ExecStopPost did not restore node DNS"
fi
echo "OK: ExecStopPost removed the resolv.conf drop-in"

if ! getent hosts mcr.microsoft.com >/dev/null 2>&1; then
    fail "node cannot resolve after a failed localdns startup"
fi
echo "OK: node DNS still works"
`
