package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/stretchr/testify/require"
)

func TestUnpackImage(t *testing.T) {
	for _, release := range []string{
		"ID=ubuntu\nVERSION_ID=22.04\n",
		"ID=ubuntu\nVERSION_ID=20.04\n",
		"ID=azurelinux\nVERSION_ID=3.0\n",
		"ID=azurelinux\nVERSION_ID=4.0\n",
		"ID=mariner\nVERSION_ID=2.0\n",
		"ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n",
		"ID=azurelinux\nVARIANT_ID=osguard\n",
		"ID=flatcar\n",
	} {
		t.Run(release, func(t *testing.T) {
			root := t.TempDir()
			osRelease := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(osRelease, []byte(release), 0o600))
			require.NoError(t, unpackScripts(osRelease, root, nil))
			for _, name := range []string{
				"etc/systemd/system/kubelet.service",
				"etc/systemd/system/localdns.service",
				"opt/azure/containers/provision.sh",
				"opt/azure/containers/provision_source_distro.sh",
				"opt/azure/image-build/install-dependencies.sh",
			} {
				require.FileExists(t, filepath.Join(root, name))
			}
			require.NoDirExists(t, filepath.Join(root, "home/packer"))
			if strings.Contains(release, "azurecontainerlinux") {
				require.NoFileExists(t, filepath.Join(root, "etc/systemd/system/containerd.service"))
				require.FileExists(t, filepath.Join(root, "NOTICE_FLATCAR.txt"))
			}
			if strings.Contains(release, "ID=mariner") {
				require.FileExists(t, filepath.Join(root, "opt/azure/containers/npd-update.sh"))
				require.FileExists(t, filepath.Join(root, "opt/azure/containers/post-deprovision-walinuxagent.sh"))
				require.NoFileExists(t, filepath.Join(root, "usr/lib/systemd/system/ig-import-gadgets.service"))
				require.NoFileExists(t, filepath.Join(root, "etc/node-exporter.d/web-config.yml"))
			}
			if strings.Contains(release, "ID=azurelinux\nVERSION_ID=3.0") {
				rules, err := os.ReadFile(filepath.Join(root, "etc/modprobe.d/CIS.conf"))
				require.NoError(t, err)
				require.NotContains(t, string(rules), "algif_aead")
				require.Contains(t, string(rules), "blacklist cramfs")
			}
			if strings.Contains(release, "ID=azurelinux\nVERSION_ID=4.0") {
				rules, err := os.ReadFile(filepath.Join(root, "etc/modprobe.d/CIS.conf"))
				require.NoError(t, err)
				require.Contains(t, string(rules), "blacklist algif_aead")
				require.NoFileExists(t, filepath.Join(root, "etc/systemd/system/aks-networkd-recovery.timer"))
			}
			if strings.Contains(release, "ID=flatcar") {
				require.NoFileExists(t, filepath.Join(root, "etc/tmpfiles.d/aks-mglru.conf"))
				require.NoFileExists(t, filepath.Join(root, "etc/systemd/system/localdns-exporter.socket"))
			}
			logging, err := os.ReadFile(filepath.Join(root, "etc/rsyslog.d/60-CIS.conf"))
			require.NoError(t, err)
			require.Contains(t, string(logging), "-/var/log/messages")
			path := filepath.Join(root, "etc/systemd/system/kubelet.service")
			expected, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, []byte("old image"), 0o644))
			require.NoError(t, os.Chmod(path, 0o600))
			require.NoError(t, unpackScripts(osRelease, root, nil))
			actual, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
			require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/nvidia.list"))
			require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/doca-net.list"))
			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
		})
	}
}

func TestUnpackRemembersImageVariants(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID=24.04\n"), 0o600))
	require.NoError(t, unpackScripts(release, root, []string{"gb"}))
	path := filepath.Join(root, "etc/containerd/config.toml")
	expected, err := parts.Root.ReadFile("root/overlays/gb/etc/containerd/config.toml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	require.NoError(t, unpackScripts(release, root, nil))
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/nvidia.list"))
	require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/doca-net.list"))
	require.FileExists(t, filepath.Join(root, "opt/azure/image-build/gb/nvidia.list"))
	require.FileExists(t, filepath.Join(root, "opt/azure/containers/gb-mai-bom.json"))
}

func TestUnpackFailsExplicitly(t *testing.T) {
	release := filepath.Join(t.TempDir(), "os-release")
	root := t.TempDir()
	require.Error(t, unpackScripts(release, root, nil))
	require.NoError(t, os.WriteFile(release, []byte("ID=unsupported\n"), 0o600))
	require.ErrorContains(t, unpackScripts(release, root, nil), "unsupported OS")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\n"), 0o600))
	require.ErrorContains(t, unpackScripts(release, "relative", nil), "absolute")
	require.ErrorContains(t, unpackScripts(release, root, []string{"../outside"}), "invalid image overlay")
	require.ErrorContains(t, unpackScripts(release, root, []string{"missing"}), "unpack overlays/missing")
	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), nil, 0o600))
	require.Error(t, unpackScripts(release, filepath.Join(root, "blocked"), nil))
}
