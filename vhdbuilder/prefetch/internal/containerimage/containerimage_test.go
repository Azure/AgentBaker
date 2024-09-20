package containerimage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/components"
	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/containerimage"
	"github.com/stretchr/testify/assert"
)

const (
	testDataPath       = "testdata/"
	regenerateTestData = "REGENERATE_CONTAINER_IMAGE_PREFETCH_TESTDATA"
)

var (
	componentsTestDataPath     = filepath.Join(testDataPath, "components.json")
	prefetchScriptTestDataPath = filepath.Join(testDataPath, "prefetch.sh")
)

func TestContianerImage(t *testing.T) {
	if strings.EqualFold(os.Getenv(regenerateTestData), "true") {
		generate(t)
	}

	expectedContent, err := os.ReadFile(prefetchScriptTestDataPath)
	assert.NoError(t, err)

	list, err := components.ParseList(componentsTestDataPath)
	assert.NoError(t, err)

	actualContent, err := containerimage.GeneratePrefetchScript(list)
	assert.NoError(t, err)

	assert.Equal(t, expectedContent, actualContent)
}

func generate(t *testing.T) {
	t.Log("generating container image prefetch.sh testdata...")

	err := os.MkdirAll(testDataPath, os.ModePerm)
	assert.NoError(t, err)

	list, err := components.ParseList(componentsTestDataPath)
	assert.NoError(t, err)

	content, err := containerimage.GeneratePrefetchScript(list)
	assert.NoError(t, err)

	err = os.WriteFile(prefetchScriptTestDataPath, content, os.ModePerm)
	assert.NoError(t, err)
}
