package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
	packagesLocation := "dists/jammy/main/binary-amd64/Packages"
	inRelease := clearSignedRelease(packagesLocation, packagesSHA, int64(len(packages)))

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
	packagesLocation := "dists/jammy/main/binary-amd64/Packages"
	inRelease := clearSignedRelease(packagesLocation, sha256Hex(packages), int64(len(packages)))

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

func clearSignedRelease(path, sum string, size int64) []byte {
	return []byte(fmt.Sprintf(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

Origin: Microsoft
SHA256:
 %s %d %s
-----BEGIN PGP SIGNATURE-----
fake-signature
-----END PGP SIGNATURE-----
`, sum, size, path))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
