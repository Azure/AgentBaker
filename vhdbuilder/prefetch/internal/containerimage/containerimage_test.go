package containerimage

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	componentsFixturePath     = "fixtures/components.json"
	prefetchScriptFixturePath = "fixtures/prefetch.sh"
)

func TestContianerImage(t *testing.T) {
	if strings.EqualFold(os.Getenv("PREFETCH_REGENERATE_FIXTURES"), "true") {
		generate(t)
	}

	expectedContent, err := os.ReadFile(prefetchScriptFixturePath)
	assert.NoError(t, err)

	raw, err := os.ReadFile(componentsFixturePath)
	assert.NoError(t, err)

	var components ComponentList
	err = json.Unmarshal(raw, &components)
	assert.NoError(t, err)

	actualContent, err := Generate(&components)
	assert.NoError(t, err)

	assert.Equal(t, expectedContent, actualContent)
}

func generate(t *testing.T) {
	raw, err := os.ReadFile(componentsFixturePath)
	assert.NoError(t, err)

	var components ComponentList
	err = json.Unmarshal(raw, &components)
	assert.NoError(t, err)

	content, err := Generate(&components)
	assert.NoError(t, err)

	err = os.WriteFile(prefetchScriptFixturePath, content, os.ModePerm)
	assert.NoError(t, err)
}
