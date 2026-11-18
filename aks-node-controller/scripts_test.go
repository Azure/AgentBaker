package main

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Azure/agentbaker/parts"
	"github.com/stretchr/testify/require"
)

func TestUnpackImage(t *testing.T) { //nolint:gocognit // Shared setup checks the complete OS-specific file matrix.
	t.Setenv("ENABLE_FIPS", "false")
	t.Setenv("FEATURE_FLAGS", "")
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
			require.NoError(t, unpackScripts(osRelease, root))
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
			containerd, containerdErr := os.ReadFile(filepath.Join(root, "etc/systemd/system/containerd.service.d/exec_start.conf"))
			require.NoError(t, containerdErr)
			require.Contains(t, string(containerd), "LimitNOFILE=1048576\n")
			if strings.Contains(release, "VARIANT_ID=osguard") {
				require.FileExists(t, filepath.Join(root, "opt/azure/image-build/cis.sh"))
				require.NoFileExists(t, filepath.Join(root, "opt/azure/containers/provision_cis.sh"))
			} else {
				require.FileExists(t, filepath.Join(root, "opt/azure/containers/provision_cis.sh"))
				require.NoFileExists(t, filepath.Join(root, "opt/azure/image-build/cis.sh"))
			}
			if strings.Contains(release, "azurecontainerlinux") {
				require.NoFileExists(t, filepath.Join(root, "etc/systemd/system/containerd.service"))
			}
			if strings.Contains(release, "ID=mariner") {
				require.FileExists(t, filepath.Join(root, "opt/azure/containers/npd-update.sh"))
				require.FileExists(t, filepath.Join(root, "opt/azure/containers/post-deprovision-walinuxagent.sh"))
				require.NoFileExists(t, filepath.Join(root, "usr/lib/systemd/system/ig-import-gadgets.service"))
				require.NoFileExists(t, filepath.Join(root, "etc/node-exporter.d/web-config.yml"))
			}
			if strings.Contains(release, "ID=azurelinux\nVERSION_ID=3.0") {
				require.FileExists(t, filepath.Join(root, "etc/systemd/system/aks-networkd-recovery.timer"))
				rules, err := os.ReadFile(filepath.Join(root, "etc/modprobe.d/CIS.conf"))
				require.NoError(t, err)
				require.NotContains(t, string(rules), "algif_aead")
				require.Contains(t, string(rules), "blacklist cramfs")
			}
			if strings.Contains(release, "ID=ubuntu\n") {
				source := "root/ubuntu/etc/pam.d/common-auth"
				if strings.Contains(release, "VERSION_ID=20.04") {
					source = "root/shared/etc/pam.d/common-auth"
				}
				want, err := parts.Root.ReadFile(source)
				require.NoError(t, err)
				got, err := os.ReadFile(filepath.Join(root, "etc/pam.d/common-auth"))
				require.NoError(t, err)
				require.Equal(t, want, got)
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
			require.NoError(t, unpackScripts(osRelease, root))
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

func TestRsyslogConfigUsesOriginalSource(t *testing.T) {
	original, err := parts.Root.ReadFile("root/shared/etc/rsyslog.d/60-CIS.conf")
	require.NoError(t, err)
	for root, rule := range map[string]string{
		"ubuntu":     "*.*;mail.none;news.none            -/var/log/messages",
		"acl":        "*.*;mail.none;news.none            -/var/log/messages",
		"mariner":    "news.none                          -/var/log/messages",
		"azurelinux": "news.none                          -/var/log/messages",
		"osguard":    "news.none                          -/var/log/messages",
	} {
		t.Run(root, func(t *testing.T) {
			files, err := resolveImageFiles(parts.Root, []string{"common", root})
			require.NoError(t, err)
			require.Equal(t, string(original)+"\n"+rule+"\n", string(files["etc/rsyslog.d/60-CIS.conf"].content))
		})
	}
}

func TestAzureLinux3CISContentKeepsUnrelatedRules(t *testing.T) {
	input := "# keep this comment\ninstall dccp /bin/true\n" +
		"# CVE-2026-31431 (Copy Fail): mitigation\ninstall algif_aead /bin/false\nblacklist algif_aead\n" +
		"# DirtyFrag / CopyFail2: mitigation\ninstall\tesp4\t/bin/false # mitigation\nblacklist esp6\nblacklist rxrpc\n" +
		"# rules added after the mitigation block\nblacklist another_module\n" +
		"blacklist esp4_extra\ninstall algif_aead /bin/true\n"
	expected := "# keep this comment\ninstall dccp /bin/true\n" +
		"# rules added after the mitigation block\nblacklist another_module\n" +
		"blacklist esp4_extra\ninstall algif_aead /bin/true\n"
	require.Equal(t, expected, string(azureLinux3CISContent([]byte(input))))
}

func TestUnpackPreservesInstalledFilePolicy(t *testing.T) {
	t.Setenv("ENABLE_FIPS", "false")
	t.Setenv("FEATURE_FLAGS", "")
	for _, platform := range []struct {
		name    string
		release string
		modes   map[string]fs.FileMode
	}{
		{"ubuntu", "ID=ubuntu\nVERSION_ID=22.04\n", map[string]fs.FileMode{
			"opt/azure/containers/security-update.sh":      0o544,
			"opt/azure/containers/cleanup-vhd.sh":          0o644,
			"opt/azure/containers/install_walinuxagent.py": 0o644,
			"opt/azure/containers/provision_configs.sh":    0o744,
			"etc/systemd/system/localdns.service":          0o644,
			"etc/security/faillock.conf":                   0o600,
		}},
		{"osguard", "ID=azurelinux\nVERSION_ID=3.0\nVARIANT_ID=osguard\n", map[string]fs.FileMode{
			"etc/systemd/system/localdns.service":             0o600,
			"etc/systemd/system/aks-log-collector.timer":      0o600,
			"etc/systemd/system/localdns-exporter.socket":     0o644,
			"etc/systemd/system/disk_queue.service":           0o644,
			"opt/azure/containers/cloud-init-status-check.sh": 0o755,
			"opt/scripts/ipv6_nftables.sh":                    0o744,
		}},
	} {
		t.Run(platform.name, func(t *testing.T) {
			root := t.TempDir()
			release := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(release, []byte(platform.release), 0o600))
			require.NoError(t, unpackScripts(release, root))
			for name, mode := range map[string]fs.FileMode{
				"NOTICE.txt":                                     0o444,
				"etc/ssh/sshd_config":                            0o600,
				"etc/systemd/system/kubelet.service":             0o600,
				"opt/azure/containers/bind-mount.sh":             0o544,
				"opt/azure/containers/provision.sh":              0o744,
				"opt/azure/containers/fetch_provision_config.py": 0o744,
				"opt/scripts/cgroup-telemetry-events.sh":         0o644,
			} {
				platform.modes[name] = mode
			}
			for name, mode := range platform.modes {
				info, err := os.Stat(filepath.Join(root, name))
				require.NoError(t, err, name)
				require.Equal(t, mode, info.Mode().Perm(), name)
			}
		})
	}
}

func TestUnpackCombinesFlatcarNotices(t *testing.T) {
	common, err := parts.Root.ReadFile("root/common/NOTICE.txt")
	require.NoError(t, err)
	flatcar, err := parts.Root.ReadFile("root/shared/NOTICE_FLATCAR.txt")
	require.NoError(t, err)
	for _, release := range []string{"ID=flatcar\n", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n"} {
		t.Run(release, func(t *testing.T) {
			root := t.TempDir()
			releasePath := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte(release), 0o600))
			for range 2 {
				require.NoError(t, unpackScripts(releasePath, root))
				actual, err := os.ReadFile(filepath.Join(root, "NOTICE.txt"))
				require.NoError(t, err)
				require.Equal(t, sha256.Sum256([]byte(string(common)+"\n"+string(flatcar))), sha256.Sum256(actual))
				require.NoFileExists(t, filepath.Join(root, "NOTICE_FLATCAR.txt"))
			}
		})
	}
}

func TestUnpackLogrotateSelection(t *testing.T) {
	for _, bundledTimer := range []bool{false, true} {
		root := t.TempDir()
		release := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID=22.04\n"), 0o600))
		if !bundledTimer {
			timer := filepath.Join(root, "usr/lib/systemd/system/logrotate.timer")
			require.NoError(t, os.MkdirAll(filepath.Dir(timer), 0o755))
			require.NoError(t, os.WriteFile(timer, nil, 0o644))
		}
		require.NoError(t, unpackScripts(release, root))
		require.NoError(t, unpackScripts(release, root))
		timer := filepath.Join(root, "etc/systemd/system/logrotate.timer")
		override := filepath.Join(root, "etc/systemd/system/logrotate.timer.d/override.conf")
		if bundledTimer {
			require.FileExists(t, timer)
			require.NoFileExists(t, override)
			require.NoDirExists(t, filepath.Dir(override))
		} else {
			require.NoFileExists(t, timer)
			require.FileExists(t, override)
		}
	}
}

func TestUnpackLogrotatePreservesOtherDropIns(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID=22.04\n"), 0o600))
	custom := filepath.Join(root, "etc/systemd/system/logrotate.timer.d/custom.conf")
	require.NoError(t, os.MkdirAll(filepath.Dir(custom), 0o755))
	require.NoError(t, os.WriteFile(custom, []byte("[Timer]\nAccuracySec=1s\n"), 0o600))
	require.NoError(t, unpackScripts(release, root))
	require.FileExists(t, custom)
	require.NoFileExists(t, filepath.Join(filepath.Dir(custom), "override.conf"))
	require.NoError(t, unpackScripts(release, root))
	require.FileExists(t, custom)
}

func TestUnpackPreservesPreparedSSHConfig(t *testing.T) {
	source := fstest.MapFS{
		"root/common/etc/ssh/sshd_config":             {Data: []byte("#PubkeyAuthentication yes\n")},
		"root/common/opt/azure/containers/kubelet.sh": {Data: []byte("updated script\n")},
	}
	for _, prepared := range []bool{false, true} {
		root := t.TempDir()
		sshConfig := filepath.Join(root, "etc/ssh/sshd_config")
		require.NoError(t, os.MkdirAll(filepath.Dir(sshConfig), 0o755))
		require.NoError(t, os.WriteFile(sshConfig, []byte("PubkeyAuthentication no\n"), 0o600))
		if prepared {
			marker := filepath.Join(root, "opt/azure/containers/base_prep.complete")
			require.NoError(t, os.MkdirAll(filepath.Dir(marker), 0o755))
			require.NoError(t, os.WriteFile(marker, nil, 0o644))
		}
		require.NoError(t, unpackRoot(source, []string{"common"}, root))
		content, err := os.ReadFile(sshConfig)
		require.NoError(t, err)
		if prepared {
			require.Equal(t, "PubkeyAuthentication no\n", string(content))
		} else {
			require.Equal(t, "#PubkeyAuthentication yes\n", string(content))
		}
		info, err := os.Stat(sshConfig)
		require.NoError(t, err)
		require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
		content, err = os.ReadFile(filepath.Join(root, "opt/azure/containers/kubelet.sh"))
		require.NoError(t, err)
		require.Equal(t, "updated script\n", string(content))
	}
}

func TestImageReferencesAndPrecedence(t *testing.T) {
	source := fstest.MapFS{
		"root/common/etc/crictl.yaml":                   {Data: []byte("common")},
		"root/ubuntu/etc/crictl.yaml.link":              {Data: []byte("shared/config\n")},
		"root/ubuntu/opt/bin/ci-syslog-watcher.sh.link": {Data: []byte("shared/helper")},
		"root/shared/config":                            {Data: []byte("ubuntu")},
		"root/shared/helper":                            {Data: []byte("#!/bin/sh\nexit 0\n")},
		"root/overlays/example/etc/crictl.yaml":         {Data: []byte("variant")},
	}
	root := t.TempDir()
	require.NoError(t, unpackRoot(source, []string{"common", "ubuntu"}, root))
	data, err := os.ReadFile(filepath.Join(root, "etc/crictl.yaml"))
	require.NoError(t, err)
	require.Equal(t, "ubuntu", string(data))
	info, err := os.Stat(filepath.Join(root, "opt/bin/ci-syslog-watcher.sh"))
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
	require.NoFileExists(t, filepath.Join(root, "opt/bin/ci-syslog-watcher.sh.link"))
	require.NoDirExists(t, filepath.Join(root, "shared"))

	require.NoError(t, unpackRoot(source, []string{"common", "ubuntu", "overlays/example"}, root))
	data, err = os.ReadFile(filepath.Join(root, "etc/crictl.yaml"))
	require.NoError(t, err)
	require.Equal(t, "variant", string(data))
}

func TestUnpackPreservesExistingSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "usr/lib/issue")
	link := filepath.Join(root, "etc/issue")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("old banner\n"), 0o640))
	require.NoError(t, os.Symlink("../usr/lib/issue", link))
	source := fstest.MapFS{"root/common/etc/issue": {Data: []byte("updated banner\n")}}

	require.NoError(t, unpackRoot(source, []string{"common"}, root))
	gotLink, err := os.Readlink(link)
	require.NoError(t, err)
	require.Equal(t, "../usr/lib/issue", gotLink)
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "updated banner\n", string(content))
	info, err := os.Stat(target)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o640), info.Mode().Perm())
}

func TestUnpackRejectsSymlinkOutsideDestination(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "issue")
	require.NoError(t, os.WriteFile(outside, []byte("outside\n"), 0o600))
	link := filepath.Join(root, "etc/issue")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(outside, link))
	source := fstest.MapFS{"root/common/etc/issue": {Data: []byte("updated banner\n")}}

	require.ErrorContains(t, unpackRoot(source, []string{"common"}, root), "escapes destination")
	content, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "outside\n", string(content))
}

func TestInvalidImagePlansWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files fstest.MapFS
		roots []string
		want  string
	}{
		{"missing target", fstest.MapFS{"root/ubuntu/z.link": {Data: []byte("shared/missing")}}, []string{"ubuntu"}, "resolve shared/missing"},
		{"missing permissions", fstest.MapFS{"root/ubuntu/unknown": {Data: []byte("data")}}, []string{"ubuntu"}, "missing permissions"},
		{"missing common notice", fstest.MapFS{"root/ubuntu/NOTICE_FLATCAR.txt": {Data: []byte("license")}}, []string{"ubuntu"}, "require the common NOTICE.txt"},
		{"traversal", fstest.MapFS{"root/ubuntu/z.link": {Data: []byte("shared/../secret")}}, []string{"ubuntu"}, "invalid shared reference"},
		{"outside shared", fstest.MapFS{"root/ubuntu/z.link": {Data: []byte("common/a")}}, []string{"ubuntu"}, "invalid shared reference"},
		{"reference chain", fstest.MapFS{
			"root/ubuntu/z.link":    {Data: []byte("shared/next.link")},
			"root/shared/next.link": {Data: []byte("shared/data")},
		}, []string{"ubuntu"}, "invalid shared reference"},
		{"directory reference", fstest.MapFS{
			"root/ubuntu/z.link":         {Data: []byte("shared/directory")},
			"root/shared/directory/file": {Data: []byte("data")},
		}, []string{"ubuntu"}, "not a regular file"},
		{"duplicate destination", fstest.MapFS{
			"root/ubuntu/z":      {Data: []byte("local")},
			"root/ubuntu/z.link": {Data: []byte("shared/data")},
			"root/shared/data":   {Data: []byte("shared")},
		}, []string{"ubuntu"}, "conflicts between root/ubuntu/z and root/ubuntu/z.link"},
		{"parent conflict", fstest.MapFS{
			"root/common/etc":        {Data: []byte("file")},
			"root/ubuntu/etc/config": {Data: []byte("nested")},
		}, []string{"ubuntu"}, "file/directory conflict"},
		{"overlapping variants", fstest.MapFS{
			"root/ubuntu/z":          {Data: []byte("OS")},
			"root/overlays/first/z":  {Data: []byte("first")},
			"root/overlays/second/z": {Data: []byte("second")},
		}, []string{"ubuntu", "overlays/first", "overlays/second"}, "conflicts between root/overlays/first/z and root/overlays/second/z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.files["root/common/a"] = &fstest.MapFile{Data: []byte("must not be written")}
			root := t.TempDir()
			err := unpackRoot(tc.files, append([]string{"common"}, tc.roots...), root)
			require.ErrorContains(t, err, tc.want)
			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestParseImageFileModes(t *testing.T) {
	modes, err := parseImageFileModes("# modes\n\n0544 opt/helper.sh 0755 # OSGuard\n0600 etc/config\n")
	require.NoError(t, err)
	require.Equal(t, imageFilePermissions{standard: 0o544, osguard: 0o755}, modes["opt/helper.sh"])
	require.Equal(t, imageFilePermissions{standard: 0o600, osguard: 0o600}, modes["etc/config"])
	for _, tc := range []struct {
		name, input, want string
	}{
		{"empty", "# comment\n", "no file permissions"},
		{"fields", "0644\n", ":1: expected"},
		{"absolute", "0644 /etc/config\n", "invalid destination"},
		{"traversal", "0644 etc/../config\n", "invalid destination"},
		{"non-octal", "0688 etc/config\n", "invalid octal"},
		{"special bits", "4755 opt/helper.sh\n", "invalid octal"},
		{"override", "0644 etc/config bad\n", "invalid octal"},
		{"duplicate", "0644 etc/config\n0600 etc/config\n", ":2: duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseImageFileModes(tc.input)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestUnpackRemembersImageVariants(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID=24.04\n"), 0o600))
	selection := filepath.Join(root, imageOverlaysPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(selection), 0o755))
	require.NoError(t, os.WriteFile(selection, []byte("gb\n"), 0o644))
	require.NoError(t, unpackScripts(release, root))
	path := filepath.Join(root, "etc/containerd/config.toml")
	expected, err := parts.Root.ReadFile("root/overlays/gb/etc/containerd/config.toml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	t.Setenv("FEATURE_FLAGS", "None")
	require.NoError(t, unpackScripts(release, root))
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/nvidia.list"))
	require.NoFileExists(t, filepath.Join(root, "etc/apt/sources.list.d/doca-net.list"))
	require.FileExists(t, filepath.Join(root, "opt/azure/image-build/gb/nvidia.list"))
	require.FileExists(t, filepath.Join(root, "opt/azure/containers/gb-mai-bom.json"))
}

func TestInferImageOverlaysBuildSelection(t *testing.T) {
	for _, tc := range []struct {
		name, id, version, variant, arch, fips, features string
		want                                             []string
	}{
		{"Ubuntu ARM64", "ubuntu", "24.04", "", "arm64", "false", "", []string{"nvidia-grub"}},
		{"Ubuntu AMD64", "ubuntu", "24.04", "", "amd64", "false", "", nil},
		{"Ubuntu FIPS", "ubuntu", "22.04", "", "amd64", "TRUE", "", []string{"ubuntu2204-fips"}},
		{"Ubuntu 20 FIPS", "ubuntu", "20.04", "", "amd64", "true", "", nil},
		{"Azure Linux ARM64", "azurelinux", "3.0", "", "arm64", "false", "", []string{"nvidia-grub"}},
		{"Azure Linux FIPS", "azurelinux", "3.0", "", "arm64", "true", "", nil},
		{"Azure Linux Kata", "azurelinux", "3.0", "kata", "arm64", "false", "", nil},
		{"Azure Linux Kata ID", "azurelinuxkata", "3.0", "", "arm64", "false", "", nil},
		{"Azure Linux 4", "azurelinux", "4.0", "", "arm64", "false", "", nil},
		{"ACL", "azurelinux", "3.0", "azurecontainerlinux", "arm64", "false", "", nil},
		{"GB", "ubuntu", "24.04", "", "arm64", "false", "NVIDIA_GB", []string{"nvidia-grub", "gb"}},
		{"OSGuard", "azurelinux", "3.0", "osguard", "arm64", "false", "", nil},
		{"Flatcar", "flatcar", "", "", "arm64", "false", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENABLE_FIPS", tc.fips)
			t.Setenv("FEATURE_FLAGS", tc.features)
			root := t.TempDir()
			timer := filepath.Join(root, "usr/lib/systemd/system/logrotate.timer")
			require.NoError(t, os.MkdirAll(filepath.Dir(timer), 0o755))
			require.NoError(t, os.WriteFile(timer, nil, 0o644))
			release := map[string]string{"ID": tc.id, "VERSION_ID": tc.version, "VARIANT_ID": tc.variant}
			osRoot, err := imageRoot(release)
			require.NoError(t, err)
			actual, err := inferImageOverlays(osRoot, release, root, tc.arch)
			require.NoError(t, err)
			require.Equal(t, tc.want, actual)
		})
	}
}

func TestUnpackInfersLegacyImageVariants(t *testing.T) {
	t.Setenv("ENABLE_FIPS", "")
	require.NoError(t, os.Unsetenv("ENABLE_FIPS"))
	t.Setenv("FEATURE_FLAGS", "")
	require.NoError(t, os.Unsetenv("FEATURE_FLAGS"))
	for _, tc := range []struct {
		name    string
		version string
		markers []string
		overlay string
		file    string
	}{
		{"FIPS", "22.04", []string{"etc/default/grub.d/99-fips.cfg"}, "ubuntu2204-fips", "etc/ssh/sshd_config"},
		{"GB", "24.04", []string{"opt/azure/containers/gb-mai-bom.json"}, "gb", "etc/containerd/config.toml"},
		{"bundled logrotate", "22.04", []string{"opt/bin/logrotate.sh", "etc/systemd/system/logrotate.timer"}, "logrotate", "opt/bin/logrotate.sh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range tc.markers {
				target := filepath.Join(root, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
				require.NoError(t, os.WriteFile(target, nil, 0o644))
			}
			release := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID="+tc.version+"\n"), 0o600))
			require.NoError(t, unpackScripts(release, root))
			expected, err := parts.Root.ReadFile("root/overlays/" + tc.overlay + "/" + tc.file)
			require.NoError(t, err)
			actual, err := os.ReadFile(filepath.Join(root, tc.file))
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

func TestUnpackRetainsEmptySelectionAndItsMode(t *testing.T) {
	t.Setenv("ENABLE_FIPS", "false")
	t.Setenv("FEATURE_FLAGS", "")
	root := t.TempDir()
	timer := filepath.Join(root, "usr/lib/systemd/system/logrotate.timer")
	require.NoError(t, os.MkdirAll(filepath.Dir(timer), 0o755))
	require.NoError(t, os.WriteFile(timer, nil, 0o644))
	release := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\nVERSION_ID=22.04\n"), 0o600))
	require.NoError(t, unpackScripts(release, root))
	selection := filepath.Join(root, imageOverlaysPath)
	require.NoError(t, os.Chmod(selection, 0o600))
	sshConfig := filepath.Join(root, "etc/ssh/sshd_config")
	expected, err := os.ReadFile(sshConfig)
	require.NoError(t, err)
	t.Setenv("ENABLE_FIPS", "true")
	t.Setenv("FEATURE_FLAGS", "NVIDIA_GB")
	require.NoError(t, unpackScripts(release, root))
	actual, err := os.ReadFile(sshConfig)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NoFileExists(t, filepath.Join(root, "opt/azure/containers/gb-mai-bom.json"))
	info, err := os.Stat(selection)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
}

func TestInferLegacyAzureLinuxARM64Variant(t *testing.T) {
	t.Setenv("ENABLE_FIPS", "")
	require.NoError(t, os.Unsetenv("ENABLE_FIPS"))
	t.Setenv("FEATURE_FLAGS", "")
	for _, hasGrub := range []bool{false, true} {
		root := t.TempDir()
		names := []string{"usr/lib/systemd/system/logrotate.timer"}
		if hasGrub {
			names = append(names, "etc/grub.d/10_azure_nvidia")
		}
		for _, name := range names {
			target := filepath.Join(root, name)
			require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
			require.NoError(t, os.WriteFile(target, nil, 0o644))
		}
		actual, err := inferImageOverlays("azurelinux", map[string]string{"ID": "azurelinux", "VERSION_ID": "3.0"}, root, "arm64")
		require.NoError(t, err)
		if hasGrub {
			require.Equal(t, []string{"nvidia-grub"}, actual)
		} else {
			require.Empty(t, actual)
		}
	}
}

func TestUnpackPreservesFinalImageBuildResults(t *testing.T) {
	source := fstest.MapFS{
		"root/common/etc/issue":                                             {Data: []byte("embedded banner\n")},
		"root/common/etc/issue.net":                                         {Data: []byte("embedded banner\n")},
		"root/common/opt/azure/containers/install_walinuxagent.py":          {Data: []byte("installer\n")},
		"root/common/opt/azure/containers/post-deprovision-walinuxagent.sh": {Data: []byte("deprovision\n")},
		"root/common/opt/azure/containers/provision.sh":                     {Data: []byte("new provisioning script\n")},
	}
	for _, built := range []bool{false, true} {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "etc"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "usr/lib"), 0o755))
		for _, name := range []string{"usr/lib/issue", "etc/issue.net"} {
			require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("package banner\n"), 0o640))
		}
		require.NoError(t, os.Symlink("../usr/lib/issue", filepath.Join(root, "etc/issue")))
		if built {
			marker := filepath.Join(root, "opt/azure/vhd-install.complete")
			require.NoError(t, os.MkdirAll(filepath.Dir(marker), 0o755))
			require.NoError(t, os.WriteFile(marker, nil, 0o644))
		}
		require.NoError(t, unpackRoot(source, []string{"common"}, root))
		expected := "embedded banner\n"
		if built {
			expected = "package banner\n"
		}
		for _, name := range []string{"etc/issue", "etc/issue.net"} {
			content, err := os.ReadFile(filepath.Join(root, name))
			require.NoError(t, err)
			require.Equal(t, expected, string(content))
			info, err := os.Stat(filepath.Join(root, name))
			require.NoError(t, err)
			require.Equal(t, fs.FileMode(0o640), info.Mode().Perm())
		}
		target, err := os.Readlink(filepath.Join(root, "etc/issue"))
		require.NoError(t, err)
		require.Equal(t, "../usr/lib/issue", target)
		for _, name := range []string{"install_walinuxagent.py", "post-deprovision-walinuxagent.sh"} {
			file := filepath.Join(root, "opt/azure/containers", name)
			if built {
				require.NoFileExists(t, file)
			} else {
				require.FileExists(t, file)
			}
		}
		require.FileExists(t, filepath.Join(root, "opt/azure/containers/provision.sh"))
	}
}

func TestUnpackInstallsKernelModuleConfiguration(t *testing.T) {
	t.Setenv("ENABLE_FIPS", "false")
	t.Setenv("FEATURE_FLAGS", "NVIDIA_GB")
	for _, release := range []string{
		"ID=ubuntu\nVERSION_ID=24.04\n",
		"ID=azurelinux\nVERSION_ID=3.0\n",
	} {
		t.Run(release, func(t *testing.T) {
			root := t.TempDir()
			osRelease := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(osRelease, []byte(release), 0o600))
			require.NoError(t, unpackScripts(osRelease, root))
			for _, name := range []string{
				"etc/modprobe.d/CIS.conf",
				"etc/modprobe.d/nvidia.conf",
				"etc/modules-load.d/nvidia-peermem.conf",
			} {
				require.FileExists(t, filepath.Join(root, name))
			}
		})
	}
}

func TestUnpackFailsExplicitly(t *testing.T) {
	release := filepath.Join(t.TempDir(), "os-release")
	root := t.TempDir()
	require.Error(t, unpackScripts(release, root))
	require.NoError(t, os.WriteFile(release, []byte("ID=unsupported\n"), 0o600))
	require.ErrorContains(t, unpackScripts(release, root), "unsupported OS")
	require.NoError(t, os.WriteFile(release, []byte("ID=ubuntu\n"), 0o600))
	require.ErrorContains(t, unpackScripts(release, "relative"), "absolute")
	selection := filepath.Join(root, imageOverlaysPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(selection), 0o755))
	require.NoError(t, os.WriteFile(selection, []byte("../outside\n"), 0o644))
	require.ErrorContains(t, unpackScripts(release, root), "invalid image overlay")
	require.NoError(t, os.WriteFile(selection, []byte("missing\n"), 0o644))
	require.ErrorContains(t, unpackScripts(release, root), "unpack overlays/missing")
	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), nil, 0o600))
	require.Error(t, unpackScripts(release, filepath.Join(root, "blocked")))
}
