package scenario

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

// bastionMaxScriptBytes is the largest script this package can send to a node.
//
// Scripts are SCP'd to the VM over the Bastion tunnel, and tunnelSession.Write
// (bastionssh.go) forwards whatever the SSH transport hands it as a single websocket
// message with no chunking. Azure Bastion caps an inbound message at 8,192 bytes and closes
// the tunnel with StatusMessageTooBig when one exceeds it — taking the whole scenario down
// mid-run, with no indication that script length was the cause.
//
// go-scp emits the file body as a 4,096-byte first chunk then the remainder in one write,
// so the largest wire write is:
//
//	max_write = script_size - 4096 + 45     (45 = SSH framing + MAC)
//
// The constant below is the last safe size, and it comes from measurement, not from that
// formula: sweeping script sizes one byte at a time against a real x/crypto/ssh client and
// server, 12,242 is safe and 12,243 produced an 8,196-byte write. The formula predicts
// 8,192 at that size, which would not exceed the cap -- it is an approximation that ignores
// SSH block padding, so writes step rather than increment and arithmetic on it lands a byte
// off. Trust the sweep. Do not "correct" the constant upward from the formula.
//
// Gate build 181818040 lost three LocalDNS lanes to exactly this: the lifecycle script had
// been sitting 380 bytes under the cliff and grew 512 bytes, producing an 8,324-byte write.
//
// The right long-term fix is chunking in tunnelSession.Write, which is shared e2e
// infrastructure and out of scope here. Until that lands, this test is the guard.
const bastionMaxScriptBytes = 12242

// TestLocalDNSScriptsFitBastionLimit fails the build when any script this package sends
// would kill the Bastion tunnel.
//
// It covers every script, not just the one that broke, because the failure mode gives no
// hint about its cause: the scenario dies with a websocket close frame and loses its node
// logs, so whoever hits it next starts from "the node became unreachable" rather than
// "my script got too long". Cheaper to fail here.
//
// If a script does outgrow the limit, prefer moving its comments into Go — they cost the
// same on the wire as code and buy nothing at runtime. validateLocalDNSLifecycle's doc
// comment is the worked example.
func TestLocalDNSScriptsFitBastionLimit(t *testing.T) {
	scripts := map[string]string{
		"lifecycle(true)":                   localdnsLifecycleScript("true"),
		"lifecycle(false)":                  localdnsLifecycleScript("false"),
		"localdnsDirectiveAssertScript":     localdnsDirectiveAssertScript,
		"localdnsWorstCycleScript":          localdnsWorstCycleScript,
		"localdnsProvisioningRestartScript": localdnsProvisioningRestartScript,
		"localdnsFaultHarnessInstallScript": localdnsFaultHarnessInstallScript,
		"localdnsFaultTeardownScript":       localdnsFaultTeardownScript,
	}
	for _, fault := range localdnsFaultMatrix {
		scripts["localdnsFaultRunScript/"+fault.name] = localdnsFaultRunScript(fault)
	}

	for name, script := range scripts {
		if len(script) > bastionMaxScriptBytes {
			t.Errorf(
				"%s is %d bytes, over the %d-byte Bastion tunnel limit by %d.\n"+
					"This will not fail as a size error: the tunnel closes with "+
					"StatusMessageTooBig, the scenario reports a dead SSH session, and node log "+
					"collection fails too. Move the script's comments into Go rather than "+
					"deleting them — they cost the same on the wire and do nothing at runtime.",
				name, len(script), bastionMaxScriptBytes, len(script)-bastionMaxScriptBytes,
			)
		}
	}

	if t.Failed() || testing.Verbose() {
		for name, script := range scripts {
			fmt.Printf("  %-46s %6d bytes (%d headroom)\n",
				name, len(script), bastionMaxScriptBytes-len(script))
		}
	}
}

// TestLocalDNSScriptInventoryIsRegistered fails the build when a script is declared but not
// size-checked.
//
// TestLocalDNSScriptsFitBastionLimit covers every script someone remembered to add to its
// map, which is not the same thing. An eighth script lands silently, and the failure it then
// hits is the one with no diagnostic attached — a dead tunnel and no node logs. So derive
// the inventory from the source rather than restating it.
func TestLocalDNSScriptInventoryIsRegistered(t *testing.T) {
	// Keyed by declaration name, not by the map keys in the size test — those are free to be
	// whatever reads best there ("lifecycle(true)", "localdnsFaultRunScript/preflight").
	registered := map[string]bool{
		"localdnsLifecycleScript":           true,
		"localdnsDirectiveAssertScript":     true,
		"localdnsWorstCycleScript":          true,
		"localdnsProvisioningRestartScript": true,
		"localdnsFaultHarnessInstallScript": true,
		"localdnsFaultTeardownScript":       true,
		"localdnsFaultRunScript":            true,
	}
	declaration := regexp.MustCompile(`(?m)^(?:var|const|func) (localdns\w*Script)\b`)
	for _, file := range []string{"scenario_localdns_hosts.go", "scenario_localdns_restart_budget.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, match := range declaration.FindAllStringSubmatch(string(src), -1) {
			if !registered[match[1]] {
				t.Errorf("%s is declared in %s but is not size-checked by "+
					"TestLocalDNSScriptsFitBastionLimit. Add it to that test's map and to the "+
					"registered set here, or it can outgrow the Bastion limit unnoticed.",
					match[1], file)
			}
		}
	}
}
