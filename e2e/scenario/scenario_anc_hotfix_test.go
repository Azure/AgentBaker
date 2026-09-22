package scenario

import (
	"encoding/base64"
	"encoding/json"
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
}
