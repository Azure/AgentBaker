package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestFlagsRepeatedParseDoesNotInheritPreviousRun(t *testing.T) {
	original := *Config
	defer func() { *Config = original }()

	trueDefault := DefaultConfiguration().DefaultLocation

	*Config = *DefaultConfiguration()
	cmd1 := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	require.NoError(t, cmd1.Run(t.Context(), []string{"e2e-test-config", "--location", "custom-location-xyz"}), "first parse failed")
	assert.Equal(t, "custom-location-xyz", Config.DefaultLocation, "first parse did not set DefaultLocation")

	cmd2 := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	require.NoError(t, cmd2.Run(t.Context(), []string{"e2e-test-config"}), "second parse failed")
	assert.Equal(t, trueDefault, Config.DefaultLocation, "second parse leaked the first run's value")
}

func TestFlagsEnvironmentSourceStillWorks(t *testing.T) {
	original := *Config
	defer func() { *Config = original }()

	t.Setenv("E2E_PARALLEL", "7")
	*Config = *DefaultConfiguration()
	cmd := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	require.NoError(t, cmd.Run(t.Context(), []string{"e2e-test-config"}), "parse failed")
	assert.Equal(t, 7, Config.Parallel, "E2E_PARALLEL was not applied")
}

func TestFlagsConfigureLinuxAndWindowsGalleriesIndependently(t *testing.T) {
	original := *Config
	defer func() { *Config = original }()

	*Config = *DefaultConfiguration()
	cmd := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	err := cmd.Run(t.Context(), []string{
		"e2e-test-config",
		"--linux-gallery-name", "linux-gallery",
		"--windows-gallery-name", "windows-gallery",
	})
	require.NoError(t, err, "parse failed")
	assert.Equal(t, "linux-gallery", Config.GalleryLinux.Name)
	assert.Equal(t, "windows-gallery", Config.GalleryWindows.Name)
}
