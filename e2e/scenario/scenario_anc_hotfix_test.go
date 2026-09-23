package scenario

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderANCHotfixFlowFixture(t *testing.T) {
	in := base64.StdEncoding.EncodeToString([]byte("prefix\n#hotfix-marker\nsuffix\n"))
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
	if strings.Contains(rendered, "#hotfix-marker") {
		t.Error("marker was not substituted")
	}

	// the heredoc body must be valid JSON matching the hotfixConfig shape
	start := strings.Index(rendered, "{\"hotfixes\"")
	if start < 0 {
		t.Fatal("pointer JSON not found")
	}
	end := strings.Index(rendered[start:], "\n")
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

	// The launcher is baked into the VHD by packer, so the fixture must overwrite it with the
	// working-tree copy or the scenario silently validates whatever the E2E VHD shipped.
	if !strings.Contains(rendered, ">"+ancLauncherPath) {
		t.Errorf("fixture does not overwrite %s", ancLauncherPath)
	}
	if !strings.Contains(rendered, "chmod 0755 "+ancLauncherPath) {
		t.Errorf("launcher override must be installed executable")
	}
	if strings.Index(rendered, ancLauncherPath) > strings.Index(rendered, ancHotfixPointerPath) {
		t.Error("launcher override must be written before the hotfix pointer")
	}
}

func TestANCLauncherOverrideRoundTrips(t *testing.T) {
	cmd, err := ancLauncherOverrideCmd()
	if err != nil {
		t.Fatal(err)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(repoRoot, ancLauncherRepoPath))
	if err != nil {
		t.Fatal(err)
	}

	// Pull the heredoc payload back out and decode it the same way the node will, so an
	// encoding change cannot ship a payload the node would fail to reconstruct.
	lines := strings.Split(cmd, "\n")
	var payload string
	for i, line := range lines {
		if strings.HasSuffix(line, ">"+ancLauncherPath) && i+1 < len(lines) {
			payload = lines[i+1]
			break
		}
	}
	if payload == "" {
		t.Fatal("launcher payload not found in rendered override")
	}
	if strings.Contains(payload, "EOF") {
		t.Fatal("payload collides with the heredoc delimiter")
	}

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("payload is not valid base64: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("payload is not valid gzip: %v", err)
	}
	defer zr.Close()
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("decoded launcher differs from %s", ancLauncherRepoPath)
	}
}
