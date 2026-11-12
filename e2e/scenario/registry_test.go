package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisteredScenarioCount(t *testing.T) {
	assert.NotEmpty(t, List(), "at least one scenario must be registered")
}

func TestRegisterDuplicateNameCaseInsensitive(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "DupTest"})

	require.Panics(t, func() {
		Register(&Scenario{Name: "duptest"})
	})
}

func TestRegisterPreservesStableOrder(t *testing.T) {
	defer resetRegistryForTest(t)()

	names := []string{"Charlie", "Alpha", "Bravo"}
	for _, name := range names {
		Register(&Scenario{Name: name})
	}

	got := List()
	require.Len(t, got, len(names))
	for i, name := range names {
		assert.Equal(t, name, got[i].Name, "entry %d: registration order not preserved", i)
	}
}

func TestRegisteredScenariosReturnsCopy(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "First"})
	Register(&Scenario{Name: "Second"})

	got := List()
	got[0] = &Scenario{Name: "Changed"}

	assert.Equal(t, "First", registry[0].Name, "mutating the returned slice changed the registry")
}

func resetRegistryForTest(t *testing.T) func() {
	t.Helper()
	savedRegistry := append([]*Scenario(nil), registry...)
	savedNames := make(map[string]struct{}, len(registryNames))
	for k := range registryNames {
		savedNames[k] = struct{}{}
	}
	registry = nil
	registryNames = map[string]struct{}{}
	return func() {
		registry = savedRegistry
		registryNames = savedNames
	}
}

func TestEffectiveTagsDoNotMutateDefinition(t *testing.T) {
	s := &Scenario{
		Name: "Example",
		Tags: Tags{Name: "stale", OS: "stale", GPU: true},
		Config: Config{
			VHD:        &config.Image{Name: "image", OS: config.OSUbuntu, Arch: "arm64"},
			VHDCaching: true,
		},
	}
	before := s.Tags
	tags := s.EffectiveTags()
	assert.Equal(t, Tags{Name: "Example", ImageName: "image", OS: string(config.OSUbuntu), Arch: "arm64", GPU: true, VHDCaching: true}, tags)
	assert.Equal(t, before, s.Tags)
}
