package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
)

func TestArtifactHelpersUseTheGivenArtifactName(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	const artifactName = "TestScenario/vhd-provision"

	if got, want := artifactDir(artifactName), filepath.Join(loggingDir, artifactName); got != want {
		t.Errorf("artifactDir: got %q, want %q", got, want)
	}

	if err := writeToFile(artifactName, "single.log", "single"); err != nil {
		t.Fatalf("writeToFile: %v", err)
	}
	if err := dumpFileMapToDir(artifactName, map[string]string{"/var/log/nested/mapped.log": "mapped"}); err != nil {
		t.Fatalf("dumpFileMapToDir: %v", err)
	}

	for name, want := range map[string]string{"single.log": "single", "mapped.log": "mapped"} {
		got, err := os.ReadFile(filepath.Join(artifactDir(artifactName), name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestGenerateVMSSNameLinuxUsesTheGivenArtifactName(t *testing.T) {
	name := generateVMSSNameLinux("TestScenario_Ubuntu2204/vhd_caching")

	if len(name) > 57 {
		t.Errorf("name %q is longer than the 57 character VMSS limit", name)
	}
	if strings.ContainsAny(name, "_/") || strings.Contains(name, "Test") {
		t.Errorf("name %q still contains characters that are invalid for a VMSS name", name)
	}
	if name != strings.ToLower(name) {
		t.Errorf("name %q is not lowercase", name)
	}
	if !strings.Contains(name, "scenarioubuntu2204") {
		t.Errorf("name %q does not carry the test name", name)
	}
}
