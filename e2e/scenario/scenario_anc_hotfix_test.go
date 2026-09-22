package scenario

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

// TestANCHotfixPipelineTagsResolve pins the targeted pipeline's scenario filter to the
// registry. TAGS_TO_RUN repeats the scenario names as literal strings in a file the Go
// build never sees, so renaming a scenario without editing the pipeline leaves it
// selecting nothing - the runner then exits non-zero with "no scenarios matched the
// configured filters", but only once the pipeline actually runs and allocates an agent.
//
// The pipeline ships in a separate PR. While it is absent there is nothing to pin.
func TestANCHotfixPipelineTagsResolve(t *testing.T) {
	const pipelinePath = "../../.pipelines/e2e-anc-hotfix.yaml"

	raw, err := os.ReadFile(pipelinePath)
	if errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s is not present on this branch yet", pipelinePath)
	}
	if err != nil {
		t.Fatal(err)
	}

	var pipeline struct {
		Variables struct {
			TagsToRun string `yaml:"TAGS_TO_RUN"`
		} `yaml:"variables"`
	}
	if err := yaml.Unmarshal(raw, &pipeline); err != nil {
		t.Fatalf("parse %s: %v", pipelinePath, err)
	}
	if pipeline.Variables.TagsToRun == "" {
		t.Fatalf("%s does not set TAGS_TO_RUN; the pipeline would run the entire suite", pipelinePath)
	}

	registered := make(map[string]struct{}, len(List()))
	for _, s := range List() {
		registered[strings.ToLower(s.Name)] = struct{}{}
	}

	var nameFilters int
	for _, pair := range strings.Split(pipeline.Variables.TagsToRun, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok {
			t.Errorf("%s has malformed filter %q", pipelinePath, pair)
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(key), "Name") {
			continue
		}
		nameFilters++

		name := strings.TrimSpace(value)
		if _, found := registered[strings.ToLower(name)]; !found {
			t.Errorf("%s selects scenario %q, which is not registered - rename the scenario and the pipeline filter together", pipelinePath, name)
		}
	}

	if nameFilters == 0 {
		t.Errorf("%s sets TAGS_TO_RUN=%q with no Name= filter", pipelinePath, pipeline.Variables.TagsToRun)
	}
}
