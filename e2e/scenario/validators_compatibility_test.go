package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestUbuntu2004CVMProxyFailureException(t *testing.T) {
	previousLogDir := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = previousLogDir })
	key, _ := newTestSigningCertificate(t)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	for _, tt := range []struct {
		name    string
		vhd     *config.Image
		cvm     bool
		units   []string
		wantErr string
	}{
		{"2004 CVM proxy only", config.VHDUbuntu2004CVMGen2Containerd, true, []string{"azurefile-proxy.service"}, ""},
		{"2004 CVM other failure", config.VHDUbuntu2004CVMGen2Containerd, true, []string{"azurefile-proxy.service", "kubelet.service"}, "kubelet.service"},
		{"non-CVM scenario", config.VHDUbuntu2004CVMGen2Containerd, false, []string{"azurefile-proxy.service"}, "azurefile-proxy.service"},
		{"non-CVM image", config.VHDUbuntu2004FIPSGen2Containerd, true, []string{"azurefile-proxy.service"}, "azurefile-proxy.service"},
		{"2404 CVM", config.VHDUbuntu2404CVMGen2Containerd, true, []string{"azurefile-proxy.service"}, "azurefile-proxy.service"},
		{"different unit", config.VHDUbuntu2004CVMGen2Containerd, true, []string{"azurefile-proxy-other.service"}, "azurefile-proxy-other.service"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var units []systemdUnit
			var legacyOutput strings.Builder
			for _, unit := range tt.units {
				units = append(units, systemdUnit{Name: unit})
				fmt.Fprintf(&legacyOutput, "%s loaded failed failed Test service\n", unit)
			}
			data, err := json.Marshal(units)
			require.NoError(t, err)
			var script string
			address, _, done := startCVMCommandSSHServer(t, signer, func(_ *ssh.ServerConn, channel ssh.Channel, command string) {
				switch {
				case strings.HasPrefix(command, "umask"):
					uploaded, _ := io.ReadAll(channel)
					script = string(uploaded)
				case strings.Contains(script, "systemctl list-units") && strings.Contains(script, "--output json"):
					_, _ = channel.Write(data)
				case strings.Contains(script, "systemctl list-units"):
					_, _ = channel.Write([]byte(legacyOutput.String()))
				default:
					_, _ = channel.Write([]byte("GLIBC_2.32 not found\nGLIBC_2.34 not found\n"))
				}
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				_ = channel.Close()
			})
			client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
				User: "azureuser", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
			})
			require.NoError(t, err)
			sshClient := newSSHClient(client)
			t.Cleanup(func() { _ = sshClient.Close(); <-done })
			vhd := *tt.vhd
			vhd.SkipOldVHDValidations = false
			s := &Scenario{
				artifactName: tt.name,
				Tags:         Tags{CVM: tt.cvm},
				Config:       Config{VHD: &vhd},
				Runtime:      &ScenarioRuntime{VM: &ScenarioVM{SSHClient: sshClient}},
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err = ValidateNoFailedSystemdUnits(ctx, s)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			if tt.wantErr == "kubelet.service" {
				require.NotContains(t, err.Error(), "unexpectedly entered a failed state: azurefile-proxy.service")
			}
			for _, unit := range tt.units {
				log, err := os.ReadFile(filepath.Join(config.Config.E2ELoggingDir, tt.name, unit+".log"))
				require.NoError(t, err)
				require.Contains(t, string(log), "GLIBC_2.34 not found")
			}
		})
	}
}

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
