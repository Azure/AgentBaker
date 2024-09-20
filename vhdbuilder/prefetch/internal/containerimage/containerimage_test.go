package containerimage_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/components"
	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/containerimage"
	"github.com/stretchr/testify/assert"
)

const (
	componentsTestDataPath     = "testdata/components.json"
	prefetchScriptTestDataPath = "testdata/prefetch.sh"
)

func TestContianerImage(t *testing.T) {
	if strings.EqualFold(os.Getenv("PREFETCH_REGENERATE_FIXTURES"), "true") {
		generate(t)
	}

	expectedContent, err := os.ReadFile(prefetchScriptTestDataPath)
	assert.NoError(t, err)

	raw, err := os.ReadFile(componentsTestDataPath)
	assert.NoError(t, err)

	var list components.List
	err = json.Unmarshal(raw, &list)
	assert.NoError(t, err)

	actualContent, err := containerimage.GeneratePrefetchScript(&list)
	assert.NoError(t, err)

	assert.Equal(t, expectedContent, actualContent)
}

func generate(t *testing.T) {
	raw, err := os.ReadFile(componentsTestDataPath)
	assert.NoError(t, err)

	var list components.List
	err = json.Unmarshal(raw, &list)
	assert.NoError(t, err)

	content, err := containerimage.GeneratePrefetchScript(&list)
	assert.NoError(t, err)

	err = os.WriteFile(prefetchScriptTestDataPath, content, os.ModePerm)
	assert.NoError(t, err)
}
