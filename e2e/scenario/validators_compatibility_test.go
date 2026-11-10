package scenario

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestParseFailedSystemdUnits(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    []systemdUnit
		wantErr bool
	}{
		{
			name:   "legacy systemd",
			output: "  example.service loaded failed failed Example service\n",
			want:   []systemdUnit{{Name: "example.service"}},
		},
		{
			name: "multiple units including missing initrd unit",
			output: "initrd-core-tmpfiles-setup.service not-found failed failed initrd-core-tmpfiles-setup.service\n" +
				"run-cloud\\x2dinit-tmp-tmp123456.mount loaded failed failed Temporary mount\n",
			want: []systemdUnit{
				{Name: "initrd-core-tmpfiles-setup.service"},
				{Name: `run-cloud\x2dinit-tmp-tmp123456.mount`},
			},
		},
		{name: "no failed units", output: ""},
		{name: "blank lines", output: " \n\t\n"},
		{name: "unexpected header", output: "UNIT LOAD ACTIVE SUB DESCRIPTION\n", wantErr: true},
		{name: "truncated entry", output: "example.service loaded failed\n", wantErr: true},
		{name: "unexpected active state", output: "example.service loaded active running Example\n", wantErr: true},
	}
	for _, distro := range datamodel.AvailableUbuntu2004Distros {
		t.Run(string(distro), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					units, err := parseFailedSystemdUnits(tt.output, distro)
					if tt.wantErr {
						require.Error(t, err)
						require.Nil(t, units)
					} else {
						require.NoError(t, err)
						require.Equal(t, tt.want, units)
					}
				})
			}
		})
	}
}

func TestParseFailedSystemdUnitsJSON(t *testing.T) {
	tests := []struct {
		name    string
		distro  datamodel.Distro
		output  string
		want    []systemdUnit
		wantErr bool
	}{
		{
			name:   "Ubuntu 22.04",
			distro: datamodel.AKSUbuntuContainerd2204Gen2,
			output: `[{"unit":"example.service","load":"loaded","active":"failed","sub":"failed"}]`,
			want:   []systemdUnit{{Name: "example.service"}},
		},
		{
			name:   "Ubuntu 24.04 CVM",
			distro: datamodel.AKSUbuntuContainerd2404CVMGen2,
			output: `[{"unit":"run-cloud\\x2dinit-tmp-tmp123456.mount"}]`,
			want:   []systemdUnit{{Name: `run-cloud\x2dinit-tmp-tmp123456.mount`}},
		},
		{
			name:   "Ubuntu 26.04 CVM with no failed units",
			distro: datamodel.AKSUbuntuMinimalContainerd2604CVMGen2,
			output: `[]`,
			want:   []systemdUnit{},
		},
		{
			name:   "Azure Linux",
			distro: datamodel.AKSAzureLinuxV3Gen2,
			output: `[{"unit":"example.service"}]`,
			want:   []systemdUnit{{Name: "example.service"}},
		},
		{
			name:    "malformed JSON",
			distro:  datamodel.AKSUbuntuContainerd2204Gen2,
			output:  `[{"unit":`,
			wantErr: true,
		},
		{
			name:    "newer image rejects legacy output",
			distro:  datamodel.AKSUbuntuContainerd2404CVMGen2,
			output:  "example.service loaded failed failed Example service\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, err := parseFailedSystemdUnits(tt.output, tt.distro)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, units)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, units)
			}
		})
	}
}

func TestKernelPanicPattern(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"ramoops module load", "systemd[1]: Starting Load Kernel Module ramoops...", false},
		{"ramoops initialization", "ramoops: using 0x100000 at 0x40000000", false},
		{"kernel oops", "kernel: Oops: 0000 [#1] SMP", true},
		{"kernel oops mentioning ramoops", "kernel: Oops: fatal exception in ramoops", true},
		{"kernel panic", "kernel: Kernel panic - not syncing: Fatal exception", true},
		{"kernel bug", "kernel: BUG: unable to handle page fault", true},
		{"call trace", "kernel: Call Trace:", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("grep", "-iE", kernelPanicPattern)
			cmd.Stdin = strings.NewReader(tt.message + "\n")
			output, err := cmd.CombinedOutput()
			if tt.want {
				require.NoError(t, err, string(output))
				require.Contains(t, string(output), tt.message)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr, string(output))
				require.Equal(t, 1, exitErr.ExitCode(), string(output))
				require.Empty(t, output)
			}
		})
	}
}
