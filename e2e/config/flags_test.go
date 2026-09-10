package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestFlagsConfigureSpecializedVMSKUs(t *testing.T) {
	for _, setting := range []struct {
		name, flag, env, defaultSKU, envSKU, argSKU string
		value                                       func(*Configuration) string
	}{
		{
			name: "MANA", flag: "--mana-vm-sku", env: "MANA_VM_SKU",
			defaultSKU: "Standard_D2ds_v6", envSKU: "Standard_D4ds_v6", argSKU: "Standard_D8ds_v6",
			value: func(c *Configuration) string { return c.MANAVMSKU },
		},
		{
			name: "Gen1 fallback", flag: "--gen1-vm-sku", env: "DEFAULT_GEN1_VM_SKU",
			defaultSKU: "Standard_D2ds_v5", envSKU: "Standard_D4ds_v5", argSKU: "Standard_D8ds_v5",
			value: func(c *Configuration) string { return c.DefaultGen1VMSKU },
		},
	} {
		t.Run(setting.name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				env  string
				args []string
				want string
			}{
				{name: "default", want: setting.defaultSKU},
				{name: "environment", env: setting.envSKU, want: setting.envSKU},
				{name: "argument", args: []string{setting.flag, setting.argSKU}, want: setting.argSKU},
				{name: "argument overrides environment", env: setting.envSKU, args: []string{setting.flag, setting.argSKU}, want: setting.argSKU},
			} {
				t.Run(tc.name, func(t *testing.T) {
					original := *Config
					t.Cleanup(func() { *Config = original })
					t.Setenv(setting.env, tc.env)
					if tc.env == "" {
						require.NoError(t, os.Unsetenv(setting.env))
					}
					t.Setenv("DEFAULT_VM_SKU", "Standard_E4s_v7")
					*Config = *DefaultConfiguration()
					cmd := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
					require.NoError(t, cmd.Run(t.Context(), append([]string{"e2e-test-config"}, tc.args...)))
					assert.Equal(t, tc.want, setting.value(Config))
					assert.Equal(t, "Standard_E4s_v7", Config.DefaultVMSKU)
				})
			}
		})
	}
}

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
