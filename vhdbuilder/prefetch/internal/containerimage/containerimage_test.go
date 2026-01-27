package containerimage_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/components"
	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/containerimage"
	"github.com/stretchr/testify/assert"
)

const (
	testDataPath       = "testdata/"
	artifactsRelPath   = "parts/linux/cloud-init/artifacts"
	regenerateTestData = "REGENERATE_CONTAINER_IMAGE_PREFETCH_TESTDATA"
)

var (
	prefetchScriptTestDataPath = filepath.Join(testDataPath, "prefetch.sh")
)

func TestContainerImage(t *testing.T) {
	componentsPath := resolveComponentsPath(t)

	if strings.EqualFold(os.Getenv(regenerateTestData), "true") {
		generate(t, componentsPath)
	}

	expectedContent, err := os.ReadFile(prefetchScriptTestDataPath)
	assert.NoError(t, err)

	list, err := components.ParseList(componentsPath)
	assert.NoError(t, err)

	actualContent, err := containerimage.GeneratePrefetchScript(list)
	assert.NoError(t, err)

	assert.Equal(t, expectedContent, actualContent)
}

func generate(t *testing.T, componentsPath string) {
	t.Log("generating container image prefetch.sh testdata...")

	err := os.MkdirAll(testDataPath, os.ModePerm)
	assert.NoError(t, err)

	list, err := components.ParseList(componentsPath)
	assert.NoError(t, err)

	content, err := containerimage.GeneratePrefetchScript(list)
	assert.NoError(t, err)

	err = os.WriteFile(prefetchScriptTestDataPath, content, os.ModePerm)
	assert.NoError(t, err)
}

func resolveComponentsPath(t *testing.T) string {
	// this is a hack until we can get rid of storing static testdata altogether
	repoBasePath, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	assert.NoError(t, err, "unable to determine repo root with git rev-parse")
	basePath := strings.ReplaceAll(string(repoBasePath), "\n", "")
	return filepath.Join(filepath.Join(basePath, artifactsRelPath), "components.json")
}
