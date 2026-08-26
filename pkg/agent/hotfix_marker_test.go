package agent

import (
	"strings"
	"testing"
)

// TestScriptlessBoothookTemplatesHaveHotfixMarker guards the e2e binary-injection contract.
// The e2e CustomDataWithNBCCmdHack replaces #hotfix-marker with a curl download of the freshly
// compiled aks-node-controller binary, and the on-node launcher (aks-node-controller-launcher.sh)
// unconditionally prefers that binary when present. Both scriptless boothook templates must carry
// the marker so e2e runs the compiled binary rather than the stale VHD-baked one — otherwise the
// scriptless provision-config vs nbc-cmd parity check compares a baked (stale) ANC against the
// PR's baker for any PR that changes containerd rendering. In production #hotfix-marker is just a
// bash comment (no-op).
func TestScriptlessBoothookTemplatesHaveHotfixMarker(t *testing.T) {
	for name, tmpl := range map[string]string{
		"boothookTemplate":    boothookTemplate,    // EnableScriptlessCSECmd (mode 1)
		"cseBootHookTemplate": cseBootHookTemplate, // EnableScriptlessNBCCSECmd (phase 2.5)
	} {
		if !strings.Contains(tmpl, "#hotfix-marker") {
			t.Errorf("%s is missing #hotfix-marker; e2e cannot inject the compiled aks-node-controller binary", name)
		}
	}
}
