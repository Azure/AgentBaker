package scenario

import (
	"fmt"
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
// which first exceeds 8,192 at a script size of 12,243. The limit below is therefore the
// last safe size, measured by sweeping script sizes one byte at a time against a real
// x/crypto/ssh client and server.
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
