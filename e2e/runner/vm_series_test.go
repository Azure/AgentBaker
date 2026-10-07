package runner

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/stretchr/testify/require"
)

func TestPartitionVMSeries(t *testing.T) {
	candidates := []*scenario.Scenario{
		{Name: "x64 Linux", Tags: scenario.Tags{VMSeriesCoverageTest: true}, Config: scenario.Config{VHD: &config.Image{OS: config.OSUbuntu, Arch: "amd64"}}},
		{Name: "ARM Linux", Tags: scenario.Tags{KernelCoverageTest: true}, Config: scenario.Config{VHD: &config.Image{OS: config.OSAzureLinux, Arch: "arm64"}}},
		{Name: "x64 Windows", Tags: scenario.Tags{VMSeriesCoverageTest: true}, Config: scenario.Config{VHD: &config.Image{OS: config.OSWindows, Arch: "amd64"}}},
		{Name: "Gen1", Tags: scenario.Tags{VMSeriesCoverageTest: true}, Config: scenario.Config{VHD: &config.Image{OS: config.OSUbuntu, Arch: "amd64", UnsupportedGen2: true}}},
		{Name: "unrelated", Config: scenario.Config{VHD: &config.Image{OS: config.OSUbuntu, Arch: "amd64"}}},
	}
	for _, tt := range []struct {
		arch, os, want string
	}{
		{"amd64", "linux", "x64 Linux"},
		{"arm64", "linux", "ARM Linux"},
		{"amd64", "windows", "x64 Windows"},
		{"arm64", "windows", ""},
	} {
		t.Run(tt.arch+"/"+tt.os, func(t *testing.T) {
			got, excluded, err := partitionVMSeries(candidates, tt.arch, tt.os)
			if tt.want == "" {
				require.ErrorContains(t, err, "Windows ARM64 is unsupported")
				require.Empty(t, got)
			} else {
				require.NoError(t, err)
				require.Len(t, got, 1)
				require.Equal(t, tt.want, got[0].Name)
			}
			require.Len(t, excluded, len(candidates)-len(got))
		})
	}
	_, _, err := partitionVMSeries(candidates[4:], "amd64", "linux")
	require.ErrorContains(t, err, "no compatible VM-series scenarios")
}

func TestVMSeriesOptionsFailClosed(t *testing.T) {
	for _, tt := range []struct {
		name     string
		edit     func(*config.Configuration)
		parallel int
		want     string
	}{
		{"valid", func(*config.Configuration) {}, 1, ""},
		{"parallel", func(*config.Configuration) {}, 2, "--parallel=1"},
		{"keep", func(c *config.Configuration) { c.KeepVMSS = true }, 1, "--keep-vmss"},
		{"OS", func(c *config.Configuration) { c.VMSeriesOS = "all" }, 1, "linux or windows"},
		{"pool", func(c *config.Configuration) { c.SystemPoolVMSKU = " " }, 1, "system-pool"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := config.DefaultConfiguration()
			tt.edit(c)
			err := validateVMSeriesOptions(c, tt.parallel)
			if tt.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.want)
			}
		})
	}
}
