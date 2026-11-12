package scenario

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomDataWithANCHotfixFlowFixture(t *testing.T) {
	// Mirror the real custom data layout. Baker expands its own file writes at the boothook
	// template's %s before #hotfix-marker, then concatenates serviceStartTemplate. On
	// official/** branches one of those writes is a hotfix pointer committed by
	// hotfix-generate, so the stub includes it: the fixture has to win that race.
	bakerPointerWrite := "cat <<'EOF' | base64 -d | gzip -d >" + ancHotfixPointerPath + "\nQkFLRVI=\nEOF\nchmod 0600 " + ancHotfixPointerPath
	bakerNBCCmdWrite := "cat <<'EOF' | base64 -d | gzip -d >" + ancNBCCmdPath + "\nQkFLRVI=\nEOF\nchmod 0600 " + ancNBCCmdPath
	in := base64.StdEncoding.EncodeToString([]byte(
		"prefix\n" + bakerPointerWrite + "\n" + bakerNBCCmdWrite + "\n#hotfix-marker\n" +
			`logger -t aks-boothook "launching aks-node-controller $(date -Ins)"` + "\nsuffix\n"))
	out, err := CustomDataWithANCHotfixFlowFixture(in, "https://example.test/anc")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(decoded)
	t.Logf("rendered:\n%s", rendered)

	if strings.Contains(rendered, "ENABLE_PROVISIONING_HOTFIX") {
		t.Error("fixture must not enable the provisioning hotfix feature flag")
	}

	// The fixture's own writes use `cat >path`, baker's use `gzip -d >path`, so the two are
	// distinguishable even though they target the same file.
	bakerWrite := strings.Index(rendered, "gzip -d >"+ancHotfixPointerPath)
	fixtureWrite := strings.Index(rendered, "cat >"+ancHotfixPointerPath)
	controllerStart := strings.Index(rendered, `logger -t aks-boothook "launching aks-node-controller`)
	if bakerWrite < 0 || fixtureWrite < 0 || controllerStart < 0 {
		t.Fatalf("expected baker write, fixture write and controller start; got %d, %d, %d", bakerWrite, fixtureWrite, controllerStart)
	}
	if fixtureWrite < bakerWrite {
		t.Error("fixture pointer write must land after baker's, otherwise the pointer hotfix-generate commits on official/** branches clobbers it")
	}
	if fixtureWrite > controllerStart {
		t.Error("fixture must be spliced before the launcher starts")
	}
	if strings.Contains(rendered, hotfixMarker) {
		t.Error("fixture must replace the hotfix marker")
	}

	// the heredoc body must be valid JSON matching the hotfixConfig shape
	start := strings.Index(rendered, "{\"hotfixes\"")
	if start < 0 {
		t.Fatal("pointer JSON not found")
	}
	end := strings.Index(rendered[start:], "\n")
	if end < 0 {
		t.Fatalf("pointer JSON line is not newline-terminated: %q", rendered[start:])
	}
	var cfg struct {
		Hotfixes map[string]string `json:"hotfixes"`
	}
	body := rendered[start : start+end]
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("pointer JSON invalid (%q): %v", body, err)
	}
	if got := cfg.Hotfixes["202608.21"]; got != "202608.21.1" {
		t.Errorf("hotfixes[202608.21] = %q, want 202608.21.1", got)
	}

	bakerNBCCmd := strings.Index(rendered, "gzip -d >"+ancNBCCmdPath)
	fixtureNBCCmd := strings.LastIndex(rendered, "cat >"+ancNBCCmdPath)
	if bakerNBCCmd < 0 || fixtureNBCCmd < 0 {
		t.Fatalf("expected baker and fixture NBC command writes; got %d and %d", bakerNBCCmd, fixtureNBCCmd)
	}
	if fixtureNBCCmd < bakerNBCCmd {
		t.Error("fixture NBC command must overwrite baker's generated NBC command")
	}
	if !strings.Contains(rendered[fixtureNBCCmd:], "echo \"ok\"") {
		t.Error("fixture NBC command must contain the successful no-op command")
	}

	scriptStart := strings.Index(rendered[fixtureNBCCmd:], "\n") + fixtureNBCCmd + 1
	scriptEnd := strings.Index(rendered[scriptStart:], "\nEOF")
	if scriptEnd < 0 {
		t.Fatal("fixture NBC script heredoc is not terminated")
	}
	script := rendered[scriptStart : scriptStart+scriptEnd]
	statusWrite := strings.Index(script, ">/var/log/azure/aks/provision.json")
	completeWrite := strings.Index(script, "touch /opt/azure/containers/provision.complete")
	if statusWrite < 0 || completeWrite <= statusWrite {
		t.Fatal("fixture must write provision.json before creating provision.complete")
	}

	dir := t.TempDir()
	statusDir := filepath.Join(dir, "status")
	completePath := filepath.Join(dir, "provision.complete")
	script = strings.ReplaceAll(script, "/var/log/azure/aks", statusDir)
	script = strings.ReplaceAll(script, "/opt/azure/containers/provision.complete", completePath)
	output, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture NBC script failed: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(filepath.Join(statusDir, "provision.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		ExitCode string
		Error    string
		Output   string
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.ExitCode != "0" || status.Error != "" || status.Output != "anc-hotfix-flow-nbc-executed" {
		t.Fatalf("unexpected provision status: %+v", status)
	}
	if _, err := os.Stat(completePath); err != nil {
		t.Fatal(err)
	}
}

// TestCustomDataWithANCHotfixFlowFixtureWithoutMarker pins the failure mode when custom data does not
// expose the injection point. The fixture must refuse loudly instead of silently producing
// custom data that never seeds the pointer.
func TestCustomDataWithANCHotfixFlowFixtureWithoutMarker(t *testing.T) {
	in := base64.StdEncoding.EncodeToString([]byte(
		"prefix\n" + `logger -t aks-boothook "launching aks-node-controller $(date -Ins)"` + "\nsuffix\n"))
	if _, err := CustomDataWithANCHotfixFlowFixture(in, "https://example.test/anc"); err == nil {
		t.Fatal("expected an error when the hotfix marker is absent")
	} else if !strings.Contains(err.Error(), "hotfix marker") {
		t.Errorf("error should name the missing hotfix marker, got %v", err)
	}
}
