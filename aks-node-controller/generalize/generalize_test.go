package generalize

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

type testExitError struct{ Code int }

func (e *testExitError) Error() string { return "exit status " + strconv.Itoa(e.Code) }

func writeUnderRoot(t *testing.T, root, p, content string) string {
	t.Helper()
	full := filepath.Join(root, p)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	return full
}

func TestGeneralize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("generalize is Linux-only")
	}

	newGeneralizer := func(t *testing.T) (*Generalizer, string) {
		root := t.TempDir()
		writeUnderRoot(t, root, config.OSReleasePath, "ID=ubuntu\n")
		return &Generalizer{KeepAgent: true, rootDir: root}, root
	}

	t.Run("removes per-node state and keeps basePrep and agent state", func(t *testing.T) {
		g, root := newGeneralizer(t)
		machineID := writeUnderRoot(t, root, "/etc/machine-id", "0123456789abcdef\n")
		before, err := os.Stat(machineID)
		require.NoError(t, err)
		for _, p := range nodeStatePaths {
			writeUnderRoot(t, root, p, "x")
		}
		kept := []string{
			writeUnderRoot(t, root, "/opt/azure/containers/base_prep.complete", "x"),
			writeUnderRoot(t, root, "/var/lib/waagent/provisioned", "x"),
			writeUnderRoot(t, root, "/opt/azure/containers/aks-node-controller", "x"),
		}

		require.NoError(t, g.Run(context.Background()))

		after, err := os.Stat(machineID)
		require.NoError(t, err)
		assert.Zero(t, after.Size())
		assert.True(t, os.SameFile(before, after), "machine-id must be truncated in place, not replaced")
		for _, p := range nodeStatePaths {
			assert.NoFileExists(t, filepath.Join(root, p))
		}
		for _, p := range kept {
			assert.FileExists(t, p)
		}
	})

	t.Run("succeeds when per-node files are already absent", func(t *testing.T) {
		g, root := newGeneralizer(t)
		writeUnderRoot(t, root, "/etc/machine-id", "id\n")

		assert.NoError(t, g.Run(context.Background()))
	})

	t.Run("fails when a per-node file cannot be removed", func(t *testing.T) {
		g, root := newGeneralizer(t)
		writeUnderRoot(t, root, "/etc/machine-id", "id\n")
		// A non-empty directory makes os.Remove fail with an error other than not-exist.
		writeUnderRoot(t, root, filepath.Join(config.Linux().ProvisionCompleteFilePath, "child"), "x")

		assert.Error(t, g.Run(context.Background()))
	})

	t.Run("keep-agent skips image-only steps", func(t *testing.T) {
		var cmds []string
		root := t.TempDir()
		g := &Generalizer{KeepAgent: true, rootDir: root, RunCmd: func(cmd *exec.Cmd) error {
			cmds = append(cmds, strings.Join(cmd.Args, " "))
			return nil
		}}
		writeUnderRoot(t, root, config.OSReleasePath, "ID=ubuntu\n")
		writeUnderRoot(t, root, "/etc/machine-id", "id\n")
		artifact := writeUnderRoot(t, root, buildArtifactPaths[0], "key")
		anchor := writeUnderRoot(t, root, "/boot/loader/credentials/nvpcr-anchor.abc.cred", "cred")

		require.NoError(t, g.Run(context.Background()))
		assert.Empty(t, cmds)
		assert.FileExists(t, artifact)
		assert.FileExists(t, anchor)
	})

	t.Run("does not touch boot state on non-ACL", func(t *testing.T) {
		g, root := newGeneralizer(t)
		writeUnderRoot(t, root, "/etc/machine-id", "id\n")
		writeUnderRoot(t, root, "/boot/acl/uki-addons/firstboot.addon.efi", "addon")

		require.NoError(t, g.Run(context.Background()))
		assert.NoFileExists(t, filepath.Join(root, "/boot/flatcar/first_boot"))
	})

	t.Run("fails when os-release is missing", func(t *testing.T) {
		g, root := newGeneralizer(t)
		writeUnderRoot(t, root, "/etc/machine-id", "id\n")
		require.NoError(t, os.Remove(filepath.Join(root, config.OSReleasePath)))

		assert.Error(t, g.Run(context.Background()))
	})
}

func TestGeneralizeACLFirstBoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("generalize is Linux-only")
	}

	for _, tc := range []struct {
		name        string
		osRelease   string
		ukis        []string
		addonExists bool
		noSource    bool
		wantUKI     string
		wantErr     bool
	}{
		{name: "ACL restores addon for UAPI UKI", osRelease: "ID=azurelinux\nVARIANT_ID=\"azurecontainerlinux\"\n",
			ukis: []string{"vmlinuz-6.6.efi"}, wantUKI: "vmlinuz-6.6.efi"},
		{name: "ACL restores addon for legacy UKI", osRelease: "ID=azurecontainerlinux\n",
			ukis: []string{aclLegacyUKIName}, wantUKI: aclLegacyUKIName},
		{name: "ACL picks first UKI by name", osRelease: "ID=azurecontainerlinux\n",
			ukis: []string{"vmlinuz-6.6.efi", aclLegacyUKIName}, wantUKI: aclLegacyUKIName},
		{name: "ACL keeps an existing addon", osRelease: "ID=azurecontainerlinux\n",
			ukis: []string{"vmlinuz-6.6.efi"}, wantUKI: "vmlinuz-6.6.efi", addonExists: true},
		{name: "ACL without addon source sets only the marker", osRelease: "ID=azurecontainerlinux\n",
			noSource: true},
		{name: "ACL fails without UKI", osRelease: "ID=azurecontainerlinux\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeUnderRoot(t, root, "/etc/machine-id", "id\n")
			writeUnderRoot(t, root, config.OSReleasePath, tc.osRelease)
			require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir("/boot/flatcar/first_boot")), 0o755))
			if !tc.noSource {
				writeUnderRoot(t, root, "/boot/acl/uki-addons/firstboot.addon.efi", "signed addon")
			}
			for _, uki := range tc.ukis {
				writeUnderRoot(t, root, filepath.Join("/boot/EFI/Linux", uki), "uki")
			}
			// A directory with a UKI-like name must be ignored.
			require.NoError(t, os.MkdirAll(filepath.Join(root, "/boot/EFI/Linux", "vmlinuz-0.efi"), 0o755))
			if tc.addonExists {
				writeUnderRoot(t, root, filepath.Join("/boot/EFI/Linux", tc.wantUKI+".extra.d", "firstboot.addon.efi"), "existing")
			}

			err := (&Generalizer{KeepAgent: true, rootDir: root}).Run(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.FileExists(t, filepath.Join(root, "/boot/flatcar/first_boot"))
			if tc.noSource {
				return
			}
			content, err := os.ReadFile(filepath.Join(root, "/boot/EFI/Linux", tc.wantUKI+".extra.d", "firstboot.addon.efi"))
			require.NoError(t, err)
			want := "signed addon"
			if tc.addonExists {
				want = "existing"
			}
			assert.Equal(t, want, string(content))
		})
	}
}

// imageModeFixture is a root with the files that image mode reads, and a fake wireserver.
type imageModeFixture struct {
	g    *Generalizer
	root string
	cmds []string
	ws   *fakeWireserver
	// failCmd makes a command fail when its joined args start with this prefix.
	failCmd string
	// deprovisionOutput is the output of the fake waagent.
	deprovisionOutput string
}

func newImageModeFixture(t *testing.T, osRelease string) *imageModeFixture {
	t.Helper()
	f := &imageModeFixture{root: t.TempDir()}
	f.ws = newFakeWireserver(t)
	f.g = &Generalizer{rootDir: f.root, installer: f.ws.installer(), RunCmd: func(cmd *exec.Cmd) error {
		joined := strings.Join(cmd.Args, " ")
		f.cmds = append(f.cmds, joined)
		if f.failCmd != "" && strings.HasPrefix(joined, f.failCmd) {
			return &testExitError{Code: 1}
		}
		if filepath.Base(cmd.Path) == "waagent" {
			_, err := io.WriteString(cmd.Stdout, f.deprovisionOutput)
			return err
		}
		return nil
	}}
	writeUnderRoot(t, f.root, config.OSReleasePath, osRelease)
	writeUnderRoot(t, f.root, "/etc/machine-id", "id\n")
	writeUnderRoot(t, f.root, config.ComponentsFilePath, testWALinuxAgentComponentsJSON)
	writeUnderRoot(t, f.root, "/etc/waagent.conf", "AutoUpdate.Enabled=n\nAutoUpdate.UpdateToLatestVersion=y\n")
	return f
}

func (f *imageModeFixture) run() error {
	return f.g.Run(context.Background())
}

func TestGeneralizeImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("generalize is Linux-only")
	}

	t.Run("runs all steps in order", func(t *testing.T) {
		f := newImageModeFixture(t, "ID=ubuntu\n")
		for _, p := range buildArtifactPaths {
			writeUnderRoot(t, f.root, p, "x")
		}
		for _, p := range nodeStatePaths {
			writeUnderRoot(t, f.root, p, "x")
		}
		staleAgent := writeUnderRoot(t, f.root, "/var/lib/waagent/WALinuxAgent-2.9.0/bin", "x")

		require.NoError(t, f.run())

		assert.Equal(t, []string{
			"systemctl daemon-reload",
			"systemctl disable --now containerd",
			"systemctl kill --signal=SIGKILL gcd.service",
			"systemctl disable --now gcd.service",
			"systemctl stop walinuxagent.service waagent.service azure-walinuxagent-logcollector.slice azure-vmextensions.slice",
			"waagent -force -deprovision+user",
			"systemctl stop walinuxagent.service waagent.service",
		}, f.cmds)
		for _, p := range append(append([]string{}, buildArtifactPaths...), nodeStatePaths...) {
			assert.NoFileExists(t, filepath.Join(f.root, p))
		}
		info, err := os.Stat(filepath.Join(f.root, "/etc/machine-id"))
		require.NoError(t, err)
		assert.Zero(t, info.Size())
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

		assert.FileExists(t, filepath.Join(f.root, walinuxagentStateDir, "WALinuxAgent-"+testWALinuxAgentVersion, "HandlerManifest.json"))
		assert.FileExists(t, filepath.Join(f.root, "/opt/walinuxagent/downloads", "WALinuxAgent-"+testWALinuxAgentVersion+".zip"))
		assert.NoFileExists(t, staleAgent)
		conf, err := os.ReadFile(filepath.Join(f.root, "/etc/waagent.conf"))
		require.NoError(t, err)
		assert.Equal(t, "AutoUpdate.Enabled=y\nAutoUpdate.UpdateToLatestVersion=n\n", string(conf))
		notes, err := os.ReadFile(filepath.Join(f.root, "/opt/azure/vhd-install.complete"))
		require.NoError(t, err)
		assert.Equal(t, "  - WALinuxAgent version "+testWALinuxAgentVersion+"\n", string(notes))
	})

	t.Run("OS-provided agent skips the pinned install", func(t *testing.T) {
		for _, osRelease := range []string{
			"ID=flatcar\n",
			"ID=azurecontainerlinux\n",
			"ID=azurelinux\nVARIANT_ID=osguard\n",
			"ID=azurelinux\nVARIANT_ID=AzureContainerLinux\n",
		} {
			f := newImageModeFixture(t, osRelease)
			writeUnderRoot(t, f.root, "/boot/acl/uki-addons/firstboot.addon.efi", "addon")
			writeUnderRoot(t, f.root, filepath.Join("/boot/EFI/Linux", aclLegacyUKIName), "uki")
			require.NoError(t, os.MkdirAll(filepath.Join(f.root, filepath.Dir("/boot/flatcar/first_boot")), 0o755))

			require.NoError(t, f.run(), osRelease)
			assert.Contains(t, f.cmds, "waagent -force -deprovision+user", osRelease)
			assert.Zero(t, f.ws.requests.Load(), osRelease)
			assert.NoDirExists(t, filepath.Join(f.root, walinuxagentStateDir), osRelease)
		}
	})

	t.Run("a failed step stops later steps", func(t *testing.T) {
		f := newImageModeFixture(t, "ID=ubuntu\n")
		f.failCmd = "systemctl disable"
		artifact := writeUnderRoot(t, f.root, buildArtifactPaths[0], "key")

		assert.Error(t, f.run())
		assert.FileExists(t, artifact)
		content, err := os.ReadFile(filepath.Join(f.root, "/etc/machine-id"))
		require.NoError(t, err)
		assert.Equal(t, "id\n", string(content))
	})

	t.Run("fails when waagent deprovision fails", func(t *testing.T) {
		f := newImageModeFixture(t, "ID=ubuntu\n")
		f.failCmd = "waagent"

		assert.Error(t, f.run())
		assert.NoDirExists(t, filepath.Join(f.root, walinuxagentStateDir, "WALinuxAgent-"+testWALinuxAgentVersion))
	})

	t.Run("fails when waagent deprovision logs a failure but exits 0", func(t *testing.T) {
		f := newImageModeFixture(t, "ID=ubuntu\n")
		f.deprovisionOutput = "Failed to run 'deprovision+user': OSError: [Errno 39] Directory not empty: '/var/lib/waagent/events'\n"

		assert.ErrorContains(t, f.run(), "Failed to run 'deprovision")
		assert.NoDirExists(t, filepath.Join(f.root, walinuxagentStateDir, "WALinuxAgent-"+testWALinuxAgentVersion))
	})

	t.Run("fails when the pinned agent cannot be downloaded", func(t *testing.T) {
		f := newImageModeFixture(t, "ID=azurelinux\n")
		f.ws.manifestVersion = "1.0.0"

		assert.Error(t, f.run())
		assert.Empty(t, f.cmds, "a failed download must stop the build before any change")
		assert.NoFileExists(t, filepath.Join(f.root, "/opt/azure/vhd-install.complete"))
	})
}

func TestRemoveBuildArtifactsNvPCR(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("generalize is Linux-only")
	}
	root := t.TempDir()
	g := &Generalizer{rootDir: root}
	anchors := []string{
		writeUnderRoot(t, root, "/var/lib/systemd/nvpcr/nvpcr-anchor.cred", "x"),
		writeUnderRoot(t, root, "/run/systemd/nvpcr/nvpcr-anchor.cred", "x"),
		writeUnderRoot(t, root, "/boot/loader/credentials/nvpcr-anchor.abc.cred", "x"),
		writeUnderRoot(t, root, "/efi/loader/credentials/NVPCR-ANCHOR.def.cred", "x"),
	}
	target := writeUnderRoot(t, root, "/boot/target.cred", "x")
	link := filepath.Join(root, "/boot/loader/credentials/nvpcr-anchor.link.cred")
	require.NoError(t, os.Symlink(target, link))
	kept := []string{
		writeUnderRoot(t, root, "/boot/loader/credentials/other.cred", "x"),
		writeUnderRoot(t, root, "/boot/loader/credentials/sub/nvpcr-anchor.nested.cred", "x"),
		writeUnderRoot(t, root, "/var/lib/systemd/tpm2-srk-public-key.pem", "x"),
		target,
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "/boot/loader/credentials/nvpcr-anchor.dir.cred"), 0o755))

	require.NoError(t, g.removeBuildArtifacts(context.Background(), nil))
	for _, p := range append(anchors, link) {
		assert.NoFileExists(t, p)
	}
	for _, p := range kept {
		assert.FileExists(t, p)
	}
	assert.DirExists(t, filepath.Join(root, "/boot/loader/credentials/nvpcr-anchor.dir.cred"))
	// A repeated run, with /boot/efi/loader/credentials missing, succeeds.
	require.NoError(t, g.removeBuildArtifacts(context.Background(), nil))

	// A failed credential-dir read propagates.
	require.NoError(t, os.Chmod(filepath.Join(root, "/boot/loader/credentials"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "/boot/loader/credentials"), 0o755) })
	if os.Geteuid() != 0 {
		require.Error(t, g.removeBuildArtifacts(context.Background(), nil))
	}
}
