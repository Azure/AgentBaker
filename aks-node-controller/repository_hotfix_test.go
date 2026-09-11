package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadBinaryHotfixGatesRepositoryWork(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	aptDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(aptDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte(fmt.Sprintf(
		"deb [arch=amd64 signed-by=/keys/microsoft.gpg] %s/ubuntu/22.04/prod jammy main\n",
		server.URL)), 0o644))
	osRelease := filepath.Join(dir, "os-release")
	require.NoError(t, os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\n"), 0o644))

	var commands atomic.Int32
	app := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		commands.Add(1)
		return nil
	}}).App
	app.aptSourcesDir = aptDir
	app.osReleasePath = osRelease
	app.goArch = "amd64"
	app.repositoryTempDir = dir

	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })

	Version = "202608.21.0"
	require.NoError(t, app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{}))
	Version = "202608.21.2"
	require.NoError(t, app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": "202608.21.1"},
	}))
	Version = "202608.21.0"
	require.NoError(t, app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": "202607.20.2"},
	}))

	assert.Zero(t, requests.Load(), "repository must not be contacted before version gating passes")
	assert.Zero(t, commands.Load(), "package manager must not run before version gating passes")
}

func TestUbuntuRepositoryFastPathParallelSuccessExtractsBinary(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageBytes := []byte("authenticated-deb-package-bytes")
	packageSHA := sha256Hex(packageBytes)
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	packages := []byte(fmt.Sprintf(
		"Package: aks-node-controller\nVersion: %s\nArchitecture: amd64\nFilename: %s\nSHA256: %s\n\n",
		fullVersion, packageLocation, packageSHA))
	packagesSHA := sha256Hex(packages)
	// The InRelease indexes the Packages file suite-relative; the HTTP path is rooted at
	// the repository. Keeping these distinct is what proves we look each one up correctly.
	packagesSuiteRelative := "main/binary-amd64/Packages"
	packagesLocation := "dists/jammy/" + packagesSuiteRelative
	inRelease := clearSignedRelease(packagesSuiteRelative, packagesSHA, int64(len(packages)))

	packageStarted := make(chan struct{})
	metadataStarted := make(chan struct{})
	var packageOnce, metadataOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ubuntu/22.04/prod/" + packageLocation:
			packageOnce.Do(func() { close(packageStarted) })
			select {
			case <-metadataStarted:
			case <-time.After(2 * time.Second):
				http.Error(w, "metadata did not start concurrently", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(packageBytes)
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			metadataOnce.Do(func() { close(metadataStarted) })
			select {
			case <-packageStarted:
			case <-time.After(2 * time.Second):
				http.Error(w, "package did not start concurrently", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/" + packagesLocation:
			_, _ = w.Write(packages)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package-manager fallback was not expected")
	})
	vhdPath := filepath.Join(dir, "aks-node-controller")
	hotfixPath := filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(vhdPath, []byte("vhd-binary"), 0o755))
	app.vhdBinaryPath = vhdPath
	app.hotfixBinaryPath = hotfixPath
	app.verifyRepositorySignature = func(
		_ context.Context, signedPath, signaturePath string, keyrings []string,
	) error {
		assert.Empty(t, signaturePath)
		assert.Equal(t, []string{"/keys/microsoft.gpg"}, keyrings)
		data, err := os.ReadFile(signedPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "BEGIN PGP SIGNED MESSAGE")
		return nil
	}
	app.extractRepositoryPackage = func(
		_ context.Context, format, packagePath, destination string,
	) error {
		assert.Equal(t, "deb", format)
		data, err := os.ReadFile(packagePath)
		require.NoError(t, err)
		assert.Equal(t, packageBytes, data)
		extracted := filepath.Join(destination, filepath.FromSlash(ancPackageBinaryRelativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(extracted), 0o755))
		return os.WriteFile(extracted, []byte("extracted-anc-binary"), 0o644)
	}

	originalVersion := Version
	Version = "202608.21.0"
	t.Cleanup(func() { Version = originalVersion })
	err := app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": hotfixVersion},
	})
	require.NoError(t, err)

	staged, err := os.ReadFile(hotfixPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("extracted-anc-binary"), staged)
	assert.NotEqual(t, packageBytes, staged, "the .deb bytes must never be staged as the executable")
}

func TestUbuntuRepositoryPackageChecksumMismatchIsHardFailure(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageBytes := []byte("tampered-package")
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	packages := []byte(fmt.Sprintf(
		"Package: aks-node-controller\nVersion: %s\nArchitecture: amd64\nFilename: %s\nSHA256: %s\n\n",
		fullVersion, packageLocation, strings.Repeat("a", 64)))
	packagesSuiteRelative := "main/binary-amd64/Packages"
	packagesLocation := "dists/jammy/" + packagesSuiteRelative
	inRelease := clearSignedRelease(packagesSuiteRelative, sha256Hex(packages), int64(len(packages)))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ubuntu/22.04/prod/" + packageLocation:
			_, _ = w.Write(packageBytes)
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/" + packagesLocation:
			_, _ = w.Write(packages)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	var packageManagerCalled atomic.Bool
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		packageManagerCalled.Store(true)
		return nil
	})
	app.verifyRepositorySignature = func(context.Context, string, string, []string) error { return nil }
	hotfixPath := filepath.Join(dir, "aks-node-controller-hotfix")
	app.hotfixBinaryPath = hotfixPath
	require.NoError(t, os.WriteFile(hotfixPath, []byte("stale-hotfix"), 0o755))

	originalVersion := Version
	Version = "202608.21.0"
	t.Cleanup(func() { Version = originalVersion })
	err := app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": hotfixVersion},
	})
	require.Error(t, err)
	assert.True(t, isIntegrityError(err))
	assert.False(t, packageManagerCalled.Load(), "integrity failures must not use apt fallback")
	_, statErr := os.Stat(hotfixPath)
	assert.True(t, os.IsNotExist(statErr), "stale hotfix must be removed after integrity failure")
}

func TestUbuntuRepositoryHTTPErrorFallsBackToApt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "transient repository failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dir := t.TempDir()
	var commands []string
	var mu sync.Mutex
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(cmd *exec.Cmd) error {
		mu.Lock()
		commands = append(commands, strings.Join(cmd.Args, " "))
		mu.Unlock()
		return nil
	})

	originalVersion := Version
	Version = "202608.21.0"
	t.Cleanup(func() { Version = originalVersion })
	err := app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": "202608.21.1"},
	})
	require.Error(t, err) // fallback reaches staging, where /usr/bin is absent in the unit test
	assert.False(t, isIntegrityError(err))
	assert.Condition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, command := range commands {
			if strings.Contains(command, "apt-get install") {
				return true
			}
		}
		return false
	}, "an operational direct-path failure must invoke apt fallback")
}

func TestUbuntuRepositoryFallbackDurationIncludesRepositoryAttempt(t *testing.T) {
	const repositoryDelay = 50 * time.Millisecond
	logs := installLogCapturer(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(repositoryDelay)
		http.Error(w, "transient repository failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return nil
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.pkgBinaryPath = filepath.Join(dir, "usr-bin-aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))
	require.NoError(t, os.WriteFile(app.pkgBinaryPath, []byte("package-manager-binary"), 0o755))

	originalVersion := Version
	Version = "202608.21.0"
	t.Cleanup(func() { Version = originalVersion })
	err := app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{
		Hotfixes: map[string]string{"202608.21": "202608.21.1"},
	})
	require.NoError(t, err)

	var durationMs int64 = -1
	for _, record := range logs.getRecords() {
		if record.Message != "downloaded ANC hotfix" {
			continue
		}
		duration, parseErr := strconv.ParseInt(record.Attrs["durationMs"], 10, 64)
		require.NoError(t, parseErr)
		durationMs = duration
	}
	require.GreaterOrEqual(t, durationMs, repositoryDelay.Milliseconds())
	staged, err := os.ReadFile(app.hotfixBinaryPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("package-manager-binary"), staged)
}

func TestParseAptRepositoryFormats(t *testing.T) {
	t.Run("one-line deb", func(t *testing.T) {
		repository, err := parseOneLineAptRepository(
			"deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/microsoft-prod.gpg] https://packages.microsoft.com/ubuntu/22.04/prod jammy main\n",
			"microsoft-prod.list", "amd64")
		require.NoError(t, err)
		assert.Equal(t, "https://packages.microsoft.com/ubuntu/22.04/prod", repository.URI)
		assert.Equal(t, "jammy", repository.Suite)
		assert.Equal(t, "main", repository.Component)
		assert.Equal(t, []string{"/usr/share/keyrings/microsoft-prod.gpg"}, repository.SignedBy)
	})

	t.Run("one-line deb using global Microsoft keyring", func(t *testing.T) {
		repository, err := parseOneLineAptRepository(
			"deb [arch=amd64,arm64] https://packages.microsoft.com/ubuntu/22.04/prod jammy main\n",
			"microsoft-prod.list", "amd64")
		require.NoError(t, err)
		assert.Empty(t, repository.SignedBy)

		keyringsDir := t.TempDir()
		expected := filepath.Join(keyringsDir, "microsoft.gpg")
		require.NoError(t, os.WriteFile(expected, []byte("keyring"), 0o644))
		keyrings, err := microsoftAptTrustedKeyrings(keyringsDir)
		require.NoError(t, err)
		assert.Equal(t, []string{expected}, keyrings)
	})

	t.Run("one-line deb rejects signed-by fingerprint constraints", func(t *testing.T) {
		_, err := parseOneLineAptRepository(
			"deb [arch=amd64 signed-by=/usr/share/keyrings/microsoft-prod.gpg ABCD1234] https://packages.microsoft.com/ubuntu/22.04/prod jammy main\n",
			"microsoft-prod.list", "amd64")
		require.Error(t, err)
		var unsupported *unsupportedRepositoryError
		assert.True(t, errors.As(err, &unsupported))
	})

	t.Run("deb822", func(t *testing.T) {
		repository, err := parseDeb822Repository(`
Types: deb
URIs: https://repodepot.example/microsoft/ubuntu/22.04/prod
Suites: jammy
Components: main
Architectures: amd64 arm64
Signed-By: /usr/share/keyrings/microsoft-prod.gpg /usr/share/keyrings/microsoft-2025.gpg
`, "microsoft-prod.sources", "arm64")
		require.NoError(t, err)
		assert.Equal(t, "https://repodepot.example/microsoft/ubuntu/22.04/prod", repository.URI)
		assert.Equal(t, []string{
			"/usr/share/keyrings/microsoft-prod.gpg",
			"/usr/share/keyrings/microsoft-2025.gpg",
		}, repository.SignedBy)
	})

	t.Run("deb822 rejects embedded signed-by key", func(t *testing.T) {
		_, err := parseDeb822Repository(`
Types: deb
URIs: https://repodepot.example/microsoft/ubuntu/22.04/prod
Suites: jammy
Components: main
Architectures: amd64
Signed-By:
 -----BEGIN PGP PUBLIC KEY BLOCK-----
 fake
 -----END PGP PUBLIC KEY BLOCK-----
`, "microsoft-prod.sources", "amd64")
		require.Error(t, err)
		var unsupported *unsupportedRepositoryError
		assert.True(t, errors.As(err, &unsupported))
	})
}

func TestPrepareGPGVKeyringsDearmorsArmoredKeys(t *testing.T) {
	dir := t.TempDir()
	armoredPath := filepath.Join(dir, "MICROSOFT-RPM-GPG-KEY")
	require.NoError(t, os.WriteFile(armoredPath, []byte(
		"-----BEGIN PGP PUBLIC KEY BLOCK-----\nfake\n-----END PGP PUBLIC KEY BLOCK-----\n"), 0o644))

	app := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			assert.Equal(t, "gpg", filepath.Base(cmd.Path))
			outputIndex := -1
			for i, arg := range cmd.Args {
				if arg == "--output" {
					outputIndex = i + 1
					break
				}
			}
			require.Greater(t, outputIndex, 0)
			return os.WriteFile(cmd.Args[outputIndex], []byte("binary-keyring"), 0o600)
		},
	}).App
	app.repositoryTempDir = dir

	keyrings, cleanup, err := app.prepareGPGVKeyrings(context.Background(), []string{armoredPath})
	require.NoError(t, err)
	require.Len(t, keyrings, 1)
	assert.NotEqual(t, armoredPath, keyrings[0])
	assert.FileExists(t, keyrings[0])
	cleanup()
	assert.NoFileExists(t, keyrings[0])
}

func TestExtractRepositoryTarMemberOnlyWritesANCBinary(t *testing.T) {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "etc/not-anc",
		Mode: 0o644,
		Size: int64(len("not-anc")),
	}))
	_, err := writer.Write([]byte("not-anc"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "./" + ancPackageBinaryRelativePath,
		Mode: 0o755,
		Size: int64(len("anc-binary")),
	}))
	_, err = writer.Write([]byte("anc-binary"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "usr/bin/other",
		Mode: 0o755,
		Size: int64(len("other")),
	}))
	_, err = writer.Write([]byte("other"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	dir := t.TempDir()
	found, err := extractRepositoryTarMember(bytes.NewReader(archive.Bytes()), dir)
	require.NoError(t, err)
	assert.True(t, found)
	extracted, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ancPackageBinaryRelativePath)))
	require.NoError(t, err)
	assert.Equal(t, []byte("anc-binary"), extracted)
	assert.NoFileExists(t, filepath.Join(dir, "etc/not-anc"))
	assert.NoFileExists(t, filepath.Join(dir, "usr/bin/other"))
}

func TestExtractRepositoryTarMemberRejectsOversizedANCBinary(t *testing.T) {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "./" + ancPackageBinaryRelativePath,
		Mode: 0o755,
		Size: repositoryBinaryMaxBytes + 1,
	}))

	dir := t.TempDir()
	found, err := extractRepositoryTarMember(bytes.NewReader(archive.Bytes()), dir)
	require.Error(t, err)
	assert.True(t, found)
	assert.True(t, isIntegrityError(err))
	assert.NoFileExists(t, filepath.Join(dir, filepath.FromSlash(ancPackageBinaryRelativePath)))
}

func TestParseMSOSSRepository(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "azurelinux-ms-oss.repo"), []byte(`
[azurelinux-official-ms-oss]
name=Azure Linux Microsoft Open Source
baseurl=https://repodepot.example/azurelinux/$releasever/prod/ms-oss/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
enabled=1
`), 0o644))

	repository, err := parseMSOSSRepository(dir)
	require.NoError(t, err)
	assert.Equal(t, "https://repodepot.example/azurelinux/$releasever/prod/ms-oss/$basearch", repository.BaseURL)
	assert.Equal(t, []string{"/etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY"}, repository.GPGKeys)
	assert.Equal(t, "azurelinux-official-ms-oss", repository.Section)

	app := NewTestApp(t, TestAppConfig{}).App
	app.yumReposDir = dir
	plan, err := app.rpmRepositoryPlan(platformInfo{
		OS: "linux", ID: "azurelinux", VersionID: "3.0", Arch: "amd64",
	}, "202607.20.2")
	require.NoError(t, err)
	assert.Equal(t,
		"https://repodepot.example/azurelinux/3.0/prod/ms-oss/x86_64/Packages/a/aks-node-controller-202607.20.2-1.azl3.x86_64.rpm",
		plan.packageURL)
}

func TestRepositoryArchitectureAndReleaseMappings(t *testing.T) {
	debAMD64, err := debArchitecture("amd64")
	require.NoError(t, err)
	assert.Equal(t, "amd64", debAMD64)
	debARM64, err := debArchitecture("arm64")
	require.NoError(t, err)
	assert.Equal(t, "arm64", debARM64)

	rpmAMD64, err := rpmArchitecture("amd64")
	require.NoError(t, err)
	assert.Equal(t, "x86_64", rpmAMD64)
	rpmARM64, err := rpmArchitecture("arm64")
	require.NoError(t, err)
	assert.Equal(t, "aarch64", rpmARM64)

	azlSuffix, err := rpmReleaseSuffix(platformInfo{ID: "azurelinux", VersionID: "3.0"})
	require.NoError(t, err)
	assert.Equal(t, "azl3", azlSuffix)
	marinerSuffix, err := rpmReleaseSuffix(platformInfo{ID: "mariner", VersionID: "2.0"})
	require.NoError(t, err)
	assert.Equal(t, "cm2", marinerSuffix)
	_, err = rpmReleaseSuffix(platformInfo{ID: "azurelinux", VersionID: "2.0"})
	assert.Error(t, err)
}

func TestRPMMetadataParsing(t *testing.T) {
	const (
		primaryLocation = "repodata/abc-primary.xml.gz"
		packageLocation = "Packages/a/aks-node-controller-202607.20.2-1.azl3.x86_64.rpm"
	)
	dir := t.TempDir()
	repomdPath := filepath.Join(dir, "repomd.xml")
	primaryPath := filepath.Join(dir, "primary.xml")
	require.NoError(t, os.WriteFile(repomdPath, []byte(fmt.Sprintf(`
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="primary">
    <checksum type="sha256">%s</checksum>
    <open-checksum type="sha256">%s</open-checksum>
    <location href="%s"/>
    <size>123</size>
    <open-size>456</open-size>
  </data>
</repomd>`, strings.Repeat("b", 64), strings.Repeat("c", 64), primaryLocation)), 0o644))
	require.NoError(t, os.WriteFile(primaryPath, []byte(fmt.Sprintf(`
<metadata xmlns="http://linux.duke.edu/metadata/common" packages="1">
  <package type="rpm">
    <name>aks-node-controller</name>
    <arch>x86_64</arch>
    <version epoch="0" ver="202607.20.2" rel="1.azl3"/>
    <checksum type="sha256" pkgid="YES">%s</checksum>
    <location href="%s"/>
  </package>
</metadata>`, strings.Repeat("d", 64), packageLocation)), 0o644))

	reference, err := parsePrimaryReference(repomdPath)
	require.NoError(t, err)
	assert.Equal(t, primaryLocation, reference.location)
	assert.Equal(t, strings.Repeat("b", 64), reference.checksum)
	assert.Equal(t, strings.Repeat("c", 64), reference.openChecksum)
	assert.Equal(t, int64(123), reference.size)
	assert.Equal(t, int64(456), reference.openSize)

	sum, err := parseRPMPrimaryMetadata(
		primaryPath, "202607.20.2", "1.azl3", "x86_64", packageLocation)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("d", 64), sum)

	_, err = parseRPMPrimaryMetadata(
		primaryPath, "202607.20.2", "1.azl3", "x86_64", "Packages/a/wrong.rpm")
	require.Error(t, err)
	assert.True(t, isIntegrityError(err))
}

func TestResolveRPMPackageMetadata(t *testing.T) {
	const packageLocation = "Packages/a/aks-node-controller-202607.20.2-1.azl3.x86_64.rpm"
	packageSHA := strings.Repeat("e", 64)
	primaryXML := []byte(fmt.Sprintf(`
<metadata xmlns="http://linux.duke.edu/metadata/common" packages="1">
  <package type="rpm">
    <name>aks-node-controller</name>
    <arch>x86_64</arch>
    <version epoch="0" ver="202607.20.2" rel="1.azl3"/>
    <checksum type="sha256" pkgid="YES">%s</checksum>
    <location href="%s"/>
  </package>
</metadata>`, packageSHA, packageLocation))
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	_, err := gzipWriter.Write(primaryXML)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())
	primaryBytes := compressed.Bytes()
	primaryLocation := "repodata/test-primary.xml.gz"
	repomd := []byte(fmt.Sprintf(`
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="primary">
    <checksum type="sha256">%s</checksum>
    <open-checksum type="sha256">%s</open-checksum>
    <location href="%s"/>
    <size>%d</size>
    <open-size>%d</open-size>
  </data>
</repomd>`,
		sha256Hex(primaryBytes), sha256Hex(primaryXML), primaryLocation,
		len(primaryBytes), len(primaryXML)))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/azurelinux/3.0/prod/ms-oss/x86_64/repodata/repomd.xml":
			_, _ = w.Write(repomd)
		case "/azurelinux/3.0/prod/ms-oss/x86_64/repodata/repomd.xml.asc":
			_, _ = w.Write([]byte("detached-signature"))
		case "/azurelinux/3.0/prod/ms-oss/x86_64/" + primaryLocation:
			_, _ = w.Write(primaryBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	origin, err := validateRepositoryURL(
		server.URL + "/azurelinux/3.0/prod/ms-oss/x86_64")
	require.NoError(t, err)
	origin = asRepositoryBase(origin)
	app := NewTestApp(t, TestAppConfig{}).App
	app.repositoryTempDir = t.TempDir()
	app.verifyRepositorySignature = func(
		_ context.Context, signedPath, signaturePath string, keyrings []string,
	) error {
		assert.Equal(t, []string{"/keys/ms-rpm.gpg"}, keyrings)
		assert.NotEmpty(t, signedPath)
		assert.NotEmpty(t, signaturePath)
		return nil
	}

	metadata, err := app.resolveRPMPackageMetadata(
		context.Background(),
		origin,
		[]string{"/keys/ms-rpm.gpg"},
		"202607.20.2",
		"1.azl3",
		"x86_64",
		packageLocation,
	)
	require.NoError(t, err)
	assert.Equal(t, packageSHA, metadata.sha256)
}

func configuredUbuntuRepositoryApp(
	t *testing.T,
	dir, serverURL string,
	runFunc func(*exec.Cmd) error,
) *App {
	t.Helper()
	aptDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(aptDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte(fmt.Sprintf(
		"deb [arch=amd64 signed-by=/keys/microsoft.gpg] %s/ubuntu/22.04/prod jammy main\n",
		serverURL)), 0o644))
	osRelease := filepath.Join(dir, "os-release")
	require.NoError(t, os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\n"), 0o644))
	app := NewTestApp(t, TestAppConfig{RunFunc: runFunc}).App
	app.aptSourcesDir = aptDir
	app.osReleasePath = osRelease
	app.goArch = "amd64"
	app.repositoryTempDir = dir
	return app
}

// clearSignedRelease builds a minimal InRelease. Per the Debian repository format,
// suiteRelativePath must be relative to the suite directory holding the InRelease (e.g.
// "main/binary-amd64/Packages"), matching what a real Ubuntu/PMC InRelease publishes --
// not the repository-root-relative path ("dists/<suite>/...") used to fetch the file.
func clearSignedRelease(suiteRelativePath, sum string, size int64) []byte {
	return []byte(fmt.Sprintf(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Microsoft
SHA256:
 %s %d %s
-----BEGIN PGP SIGNATURE-----
fake-signature
-----END PGP SIGNATURE-----
`, sum, size, suiteRelativePath))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// A fast package failure must cancel the in-flight metadata branch instead of waiting it
// out, and the induced cancellation must not be reported as an integrity failure -- that
// classification would disarm the staged hotfix and skip the package-manager fallback.
func TestRepositoryFastPathCancelsPeerBranchOnFailure(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"

	metadataCtxDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".deb") {
			http.NotFound(w, r) // fails fast, cancelling the metadata branch
			return
		}
		// Stand in for a slow InRelease fetch: block until cancelled, or give up well
		// before the 30s request timeout so a regression fails loudly instead of hanging.
		select {
		case <-r.Context().Done():
			close(metadataCtxDone)
		case <-time.After(10 * time.Second):
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package manager should not run in this test")
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))

	start := time.Now()
	err := app.tryRepositoryDownload(context.Background(), hotfixVersion)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), packageLocation[strings.LastIndex(packageLocation, "/")+1:],
		"the package failure should be reported, not the cancelled peer")
	assert.False(t, isIntegrityError(err),
		"a cancelled peer must not be reported as an integrity failure: that would disarm "+
			"the staged hotfix and skip the package-manager fallback")

	// Wait for the peer's context to be cancelled rather than sampling it: a
	// non-blocking check would pass on a build that never cancels, because the handler's
	// own 10s timeout eventually closes the channel anyway. The 2s bound is far above the
	// milliseconds a correct run needs and far below the 10s a non-cancelling one takes.
	select {
	case <-metadataCtxDone:
	case <-time.After(2 * time.Second):
		t.Fatal("metadata branch was not cancelled when the package download failed")
	}
	// Secondary guard that the call did not simply wait the peer out. Deliberately loose:
	// a regression blocks for the handler's full 10s while a correct run returns in
	// milliseconds, so anything under 10s separates them, and a tighter bound only makes
	// the test sensitive to CPU contention when the full suite runs in parallel.
	assert.Less(t, elapsed, 8*time.Second,
		"failure should return promptly rather than waiting out the peer branch")
}

func TestRepositoryFastPathPrefersIntegrityErrorFromEitherBranch(t *testing.T) {
	packageRequested := make(chan struct{})
	var closePackageRequested sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".deb") {
			closePackageRequested.Do(func() { close(packageRequested) })
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	origin, err := validateRepositoryURL(server.URL)
	require.NoError(t, err)
	app := NewTestApp(t, TestAppConfig{}).App
	app.repositoryTempDir = t.TempDir()
	_, _, err = app.fetchPackageAndMetadata(context.Background(), repositoryDownloadPlan{
		packageURL:    server.URL + "/aks-node-controller.deb",
		trustedOrigin: origin,
		resolveMetadata: func(context.Context) (repositoryPackageMetadata, error) {
			<-packageRequested
			return repositoryPackageMetadata{}, newIntegrityError("authenticated metadata checksum mismatch")
		},
	})
	require.Error(t, err)
	assert.True(t, isIntegrityError(err), "integrity errors must outrank operational package failures")
	assert.Contains(t, err.Error(), "resolve authenticated repository metadata")
	assert.Contains(t, err.Error(), "authenticated metadata checksum mismatch")
}

func TestPreferredRPMExtractionErrorPreservesBothCommandFailures(t *testing.T) {
	err := preferredRPMExtractionError(nil, errors.New("bad rpm payload"), errors.New("cpio read failed"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rpm2cpio: bad rpm payload")
	assert.Contains(t, err.Error(), "cpio: cpio read failed")
}

// Mariner 2.0 has no ms-oss repository -- that path 404s on packages.microsoft.com. Its
// Microsoft-published packages live in [mariner-microsoft] at .../prod/Microsoft/$basearch
// (see mariner-package-update.sh, which lists mariner-microsoft.repo). Discovery keyed only
// on "ms-oss" therefore excluded every Mariner node from the repository fast path.
func TestParseMicrosoftRepositoryMariner(t *testing.T) {
	dir := t.TempDir()
	// Sibling repos that must not be selected, mirroring a real Mariner node.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mariner-official-base.repo"), []byte(`
[mariner-official-base]
name=CBL-Mariner Official Base
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
enabled=1
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mariner-microsoft.repo"), []byte(`
[mariner-microsoft]
name=CBL-Mariner Microsoft
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/Microsoft/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
enabled=1
`), 0o644))

	repository, err := parseMSOSSRepository(dir)
	require.NoError(t, err)
	assert.Equal(t, "mariner-microsoft", repository.Section)
	assert.Equal(t,
		"https://packages.microsoft.com/cbl-mariner/$releasever/prod/Microsoft/$basearch",
		repository.BaseURL)

	app := NewTestApp(t, TestAppConfig{}).App
	app.yumReposDir = dir
	plan, err := app.rpmRepositoryPlan(platformInfo{
		OS: "linux", ID: "mariner", VersionID: "2.0", Arch: "amd64",
	}, "202607.20.2")
	require.NoError(t, err)
	assert.Equal(t,
		"https://packages.microsoft.com/cbl-mariner/2.0/prod/Microsoft/x86_64/"+
			"Packages/a/aks-node-controller-202607.20.2-1.cm2.x86_64.rpm",
		plan.packageURL)
	assert.Equal(t, "rpm", plan.format)
}

// A disabled Microsoft repo must be skipped rather than selected.
func TestParseMicrosoftRepositorySkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mariner-microsoft.repo"), []byte(`
[mariner-microsoft]
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/Microsoft/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
enabled=0
`), 0o644))

	_, err := parseMSOSSRepository(dir)
	require.Error(t, err)
	assert.False(t, isIntegrityError(err), "an absent repository is unsupported, not tampering")
}

// gzipBytes compresses data the way a repository publishes Packages.gz.
func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

// clearSignedReleaseEntries builds an InRelease listing several checksum entries, as a real
// InRelease does (both Packages and Packages.gz).
func clearSignedReleaseEntries(entries ...[3]string) []byte {
	var lines strings.Builder
	for _, e := range entries {
		// sum, size, suite-relative path
		fmt.Fprintf(&lines, " %s %s %s\n", e[0], e[1], e[2])
	}
	return []byte(fmt.Sprintf(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Microsoft
SHA256:
%s-----BEGIN PGP SIGNATURE-----
fake-signature
-----END PGP SIGNATURE-----
`, lines.String()))
}

// When the InRelease publishes Packages.gz, the fast path must fetch the compressed index
// rather than the plain one: for jammy/main/binary-amd64 that is ~720 KB instead of ~4.2 MB,
// which is what apt itself fetches. Downloading the plain index made the fast path heavier
// on the wire than the package-manager path it exists to beat.
func TestUbuntuFastPathPrefersCompressedPackagesIndex(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageBytes := []byte("authenticated-deb-package-bytes")
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	packages := []byte(fmt.Sprintf(
		"Package: aks-node-controller\nVersion: %s\nArchitecture: amd64\nFilename: %s\nSHA256: %s\n\n",
		fullVersion, packageLocation, sha256Hex(packageBytes)))
	packagesGz := gzipBytes(t, packages)

	suiteRelative := "main/binary-amd64/Packages"
	inRelease := clearSignedReleaseEntries(
		[3]string{sha256Hex(packages), fmt.Sprint(len(packages)), suiteRelative},
		[3]string{sha256Hex(packagesGz), fmt.Sprint(len(packagesGz)), suiteRelative + ".gz"},
	)

	var plainFetched, gzFetched atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ubuntu/22.04/prod/" + packageLocation:
			_, _ = w.Write(packageBytes)
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/dists/jammy/" + suiteRelative + ".gz":
			gzFetched.Store(true)
			_, _ = w.Write(packagesGz)
		case "/ubuntu/22.04/prod/dists/jammy/" + suiteRelative:
			plainFetched.Store(true)
			_, _ = w.Write(packages)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package-manager fallback was not expected")
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))
	app.verifyRepositorySignature = func(context.Context, string, string, []string) error { return nil }
	app.extractRepositoryPackage = func(
		_ context.Context, _, _, destination string,
	) error {
		extracted := filepath.Join(destination, filepath.FromSlash(ancPackageBinaryRelativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(extracted), 0o755))
		return os.WriteFile(extracted, []byte("extracted-anc-binary"), 0o644)
	}

	require.NoError(t, app.tryRepositoryDownload(context.Background(), hotfixVersion))

	assert.True(t, gzFetched.Load(), "the compressed Packages index should have been fetched")
	assert.False(t, plainFetched.Load(),
		"the plain Packages index must not be fetched when Packages.gz is published")
	assert.FileExists(t, app.hotfixBinaryPath)
}

// Repositories that publish only the plain index must still work.
func TestUbuntuFastPathFallsBackToPlainPackagesIndex(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageBytes := []byte("authenticated-deb-package-bytes")
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	packages := []byte(fmt.Sprintf(
		"Package: aks-node-controller\nVersion: %s\nArchitecture: amd64\nFilename: %s\nSHA256: %s\n\n",
		fullVersion, packageLocation, sha256Hex(packageBytes)))

	suiteRelative := "main/binary-amd64/Packages"
	inRelease := clearSignedRelease(suiteRelative, sha256Hex(packages), int64(len(packages)))

	var plainFetched atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ubuntu/22.04/prod/" + packageLocation:
			_, _ = w.Write(packageBytes)
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/dists/jammy/" + suiteRelative:
			plainFetched.Store(true)
			_, _ = w.Write(packages)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package-manager fallback was not expected")
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))
	app.verifyRepositorySignature = func(context.Context, string, string, []string) error { return nil }
	app.extractRepositoryPackage = func(
		_ context.Context, _, _, destination string,
	) error {
		extracted := filepath.Join(destination, filepath.FromSlash(ancPackageBinaryRelativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(extracted), 0o755))
		return os.WriteFile(extracted, []byte("extracted-anc-binary"), 0o644)
	}

	require.NoError(t, app.tryRepositoryDownload(context.Background(), hotfixVersion))
	assert.True(t, plainFetched.Load(), "plain index should be used when no .gz is published")
}

// A tampered compressed index must be rejected before it is decompressed.
func TestUbuntuFastPathRejectsTamperedCompressedIndex(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packages := []byte("Package: aks-node-controller\nVersion: " + fullVersion + "\n\n")
	packagesGz := gzipBytes(t, packages)
	suiteRelative := "main/binary-amd64/Packages"
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	// Advertise a checksum that the served bytes will not match.
	inRelease := clearSignedReleaseEntries(
		[3]string{strings.Repeat("b", 64), fmt.Sprint(len(packagesGz)), suiteRelative + ".gz"},
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// The package must be served successfully: if it 404s, that failure wins the race
		// and cancels the metadata branch, and the integrity error under test is correctly
		// discarded as induced noise.
		case "/ubuntu/22.04/prod/" + packageLocation:
			_, _ = w.Write([]byte("authenticated-deb-package-bytes"))
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/dists/jammy/" + suiteRelative + ".gz":
			_, _ = w.Write(packagesGz)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package-manager fallback was not expected")
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))
	app.verifyRepositorySignature = func(context.Context, string, string, []string) error { return nil }

	err := app.tryRepositoryDownload(context.Background(), hotfixVersion)
	require.Error(t, err)
	assert.True(t, isIntegrityError(err), "a checksum mismatch on the index is an integrity failure")
	assert.NoFileExists(t, app.hotfixBinaryPath, "no binary should be staged from a tampered index")
}

// The fast path must request an exact, closed set of URLs: the InRelease, one Packages
// index, and the single .deb whose path is derived from name+version+arch+codename. This
// asserts the whole request log, not just that the expected ones appear -- so any extra
// fetch (a directory listing, a second architecture, a dbgsym or source package, a
// Release/Release.gpg probe) fails the test rather than passing unnoticed.
func TestUbuntuFastPathRequestsExactlyTheExpectedURLs(t *testing.T) {
	const (
		hotfixVersion = "202608.21.1"
		fullVersion   = hotfixVersion + "-ubuntu22.04u1"
	)
	packageBytes := []byte("authenticated-deb-package-bytes")
	packageLocation := "pool/main/a/aks-node-controller/aks-node-controller_" +
		fullVersion + "_amd64.deb"
	packages := []byte(fmt.Sprintf(
		"Package: aks-node-controller\nVersion: %s\nArchitecture: amd64\nFilename: %s\nSHA256: %s\n\n",
		fullVersion, packageLocation, sha256Hex(packageBytes)))
	packagesGz := gzipBytes(t, packages)
	suiteRelative := "main/binary-amd64/Packages"
	inRelease := clearSignedReleaseEntries(
		[3]string{sha256Hex(packages), fmt.Sprint(len(packages)), suiteRelative},
		[3]string{sha256Hex(packagesGz), fmt.Sprint(len(packagesGz)), suiteRelative + ".gz"},
	)

	var mu sync.Mutex
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/ubuntu/22.04/prod/" + packageLocation:
			_, _ = w.Write(packageBytes)
		case "/ubuntu/22.04/prod/dists/jammy/InRelease":
			_, _ = w.Write(inRelease)
		case "/ubuntu/22.04/prod/dists/jammy/" + suiteRelative + ".gz":
			_, _ = w.Write(packagesGz)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	app := configuredUbuntuRepositoryApp(t, dir, server.URL, func(*exec.Cmd) error {
		return fmt.Errorf("package-manager fallback was not expected")
	})
	app.vhdBinaryPath = filepath.Join(dir, "aks-node-controller")
	app.hotfixBinaryPath = filepath.Join(dir, "aks-node-controller-hotfix")
	require.NoError(t, os.WriteFile(app.vhdBinaryPath, []byte("vhd-binary"), 0o755))
	app.verifyRepositorySignature = func(context.Context, string, string, []string) error { return nil }
	app.extractRepositoryPackage = func(_ context.Context, _, _, destination string) error {
		extracted := filepath.Join(destination, filepath.FromSlash(ancPackageBinaryRelativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(extracted), 0o755))
		return os.WriteFile(extracted, []byte("extracted-anc-binary"), 0o644)
	}

	require.NoError(t, app.tryRepositoryDownload(context.Background(), hotfixVersion))

	mu.Lock()
	got := append([]string(nil), requested...)
	mu.Unlock()
	sort.Strings(got)

	assert.Equal(t, []string{
		"GET /ubuntu/22.04/prod/dists/jammy/InRelease",
		"GET /ubuntu/22.04/prod/dists/jammy/main/binary-amd64/Packages.gz",
		"GET /ubuntu/22.04/prod/" + packageLocation,
	}, got, "the fast path must fetch exactly these three URLs and nothing else")
}

// Azure Linux and Mariner can report a three-part VERSION_ID carrying a build date, while
// PMC publishes repositories under major.minor only: azurelinux/3.0/prod/... is HTTP 200,
// azurelinux/3.0.20260304/prod/... is 404. Substituting the raw value silently costs every
// such node the fast path.
func TestRPMReleaseVersion(t *testing.T) {
	tests := []struct {
		versionID string
		want      string
	}{
		{"3.0", "3.0"},
		{"2.0", "2.0"},
		// The case that motivated this: a dated VERSION_ID must reduce to the repo path.
		{"3.0.20260304", "3.0"},
		{"2.0.20240808", "2.0"},
		// Must preserve minor rather than forcing ".0": a future 3.1 has to resolve to the
		// 3.1 repository, not silently to 3.0's.
		{"3.1", "3.1"},
		{"3.1.20260101", "3.1"},
		// Degenerate inputs pass through; rpmReleaseSuffix rejects unsupported majors.
		{"3", "3"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.versionID, func(t *testing.T) {
			assert.Equal(t, tc.want, rpmReleaseVersion(tc.versionID))
		})
	}
}

// The plan must build a major.minor repository URL even when the node reports a dated
// VERSION_ID.
func TestRPMRepositoryPlanUsesMajorMinorRepoPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "azurelinux-ms-oss.repo"), []byte(`
[azurelinux-official-ms-oss]
baseurl=https://packages.microsoft.com/azurelinux/$releasever/prod/ms-oss/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
enabled=1
`), 0o644))

	app := NewTestApp(t, TestAppConfig{}).App
	app.yumReposDir = dir
	plan, err := app.rpmRepositoryPlan(platformInfo{
		OS: "linux", ID: osReleaseIDAzureLinux, VersionID: "3.0.20260304", Arch: archAMD64,
	}, "202607.20.2")
	require.NoError(t, err)
	assert.Equal(t,
		"https://packages.microsoft.com/azurelinux/3.0/prod/ms-oss/x86_64/"+
			"Packages/a/aks-node-controller-202607.20.2-1.azl3.x86_64.rpm",
		plan.packageURL)
}
