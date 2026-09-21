package scenario

import (
	"fmt"
	"os"
	"path/filepath"
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
// The constant below is the last safe script size, measured by sweeping sizes one byte at a
// time against a real x/crypto/ssh client and server. SSH pads each binary packet up to a
// multiple of the cipher block size, so the wire write plateaus and then steps -- there is
// no script size that lands exactly on the 8,192 cap:
//
//	12236..12242 -> 8180
//	12243..12248 -> 8196   <- first over 8,192
//
// Do not derive this constant arithmetically from the script size. An earlier version of
// this comment did exactly that and was wrong twice: off by 11 bytes at the plateau, and
// implying a 1-byte granularity the transport does not have.
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
// localdnsScriptsUnderTest is the single inventory of scripts this package sends to a node,
// keyed by declaration name. Both tests below read it, so registering a script and
// size-checking it are the same edit -- there is no way to do one without the other.
//
// Two lists would not be equivalent: a script added to one and missed in the other passes
// both tests while going unchecked, which is the exact failure this is here to prevent.
func localdnsScriptsUnderTest() map[string][]string {
	faultRunScripts := make([]string, 0, len(localdnsFaultMatrix))
	for _, fault := range localdnsFaultMatrix {
		faultRunScripts = append(faultRunScripts, localdnsFaultRunScript(fault))
	}
	return map[string][]string{
		"localdnsLifecycleScript":           {localdnsLifecycleScript("true"), localdnsLifecycleScript("false")},
		"localdnsDirectiveAssertScript":     {localdnsDirectiveAssertScript},
		"localdnsWorstCycleScript":          {localdnsWorstCycleScript},
		"localdnsProvisioningRestartScript": {localdnsProvisioningRestartScript},
		"localdnsFaultHarnessInstallScript": {localdnsFaultHarnessInstallScript},
		"localdnsFaultTeardownScript":       {localdnsFaultTeardownScript},
		"localdnsFaultRunScript":            faultRunScripts,
	}
}

func TestLocalDNSScriptsFitBastionLimit(t *testing.T) {
	scripts := map[string]string{}
	for declaration, rendered := range localdnsScriptsUnderTest() {
		for i, script := range rendered {
			name := declaration
			if len(rendered) > 1 {
				name = fmt.Sprintf("%s[%d]", declaration, i)
			}
			scripts[name] = script
		}
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
	registered := localdnsScriptsUnderTest()
	declaration := regexp.MustCompile(`(?m)^(?:var|const|func) (localdns\w*Script)\b`)
	// Glob rather than a literal file list: go test runs in the package directory, so this
	// covers new scenario files on arrival instead of only when someone remembers to add
	// them here. Known gap it cannot close -- the regex keys on the localdns*Script naming
	// convention, so a script sent through execScriptOnVMForScenario* under another name is
	// still unguarded (validate_localdns_exporter_metrics.go does this today).
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, match := range declaration.FindAllStringSubmatch(string(src), -1) {
			if _, ok := registered[match[1]]; !ok {
				t.Errorf("%s is declared in %s but is not in localdnsScriptsUnderTest, so it is "+
					"never size-checked and can outgrow the Bastion limit unnoticed. Add it "+
					"there -- that one edit both registers and size-checks it.",
					match[1], file)
			}
		}
	}
}
