package components

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/blakesmith/ar"
	"github.com/cavaliergopher/rpm"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestImagesAreFullySpecified(t *testing.T) {
	images := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	tags := getWindowsContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	image := images[0]
	tag := tags[0]
	require.Equal(t, fmt.Sprintf("mcr.microsoft.com/windows/servercore:%s", tag), image, "Image does not contain the expected tag")
}

func TestWs2025Gen2ServerCore(t *testing.T) {
	serverCoreVersions := getWindowsContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Gen2Nanoserver(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2025ServerCore(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Nanoserver(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2ServerCore(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2Nanoserver(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs23H2ServerCoreGen2(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2NanoserverGen2(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCore(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022Nanoserver(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCoreGen2(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022NanoserverGen2(t *testing.T) {
	serverCoreVersions := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWindowsImagesHaveServercoreAndNanoserverSpecified(t *testing.T) {
	// This test ensures that all Windows images have the servercore tag specified.
	// If this test fails, it means that a new Windows image has been added without the servercore tag.

	windowsImages := []*config.Image{
		config.VHDWindows2022Containerd,
		config.VHDWindows2022ContainerdGen2,
		config.VHDWindows23H2,
		config.VHDWindows23H2Gen2,
		config.VHDWindows2025,
		config.VHDWindows2025Gen2,
	}

	for _, image := range windowsImages {
		t.Run(fmt.Sprintf("testing servercore has versions for %s", image.Name), func(t *testing.T) {
			images := GetServercoreImagesForVHD(image)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows servercore images found")
		})
		t.Run(fmt.Sprintf("testing nanoserver has versions for %s", image.Name), func(t *testing.T) {
			images := GetNanoserverImagesForVhd(image)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows nanoserver images found")
		})
	}
}

func TestDCGMExporterCompatibility(t *testing.T) {
	type testCase struct {
		name        string
		os          string
		osVersion   string
		downloadURL string
		parseDeps   func(t *testing.T, path string) (coreVersion, propVersion string)
	}

	testCases := []testCase{
		{
			name:        "Ubuntu2404",
			os:          "ubuntu",
			osVersion:   "r2404",
			downloadURL: "https://packages.microsoft.com/repos/microsoft-ubuntu-noble-prod/pool/main/d/dcgm-exporter/dcgm-exporter_%s_amd64.deb",
			parseDeps:   parseDebDeps,
		},
		{
			name:        "AzureLinux3",
			os:          "azurelinux",
			osVersion:   "v3.0",
			downloadURL: "https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/Packages/d/dcgm-exporter-%s.x86_64.rpm",
			parseDeps:   parseRPMDeps,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get expected versions from components.json
			dcgmExporterVersions := GetExpectedPackageVersions("dcgm-exporter", tc.os, tc.osVersion)
			require.Len(t, dcgmExporterVersions, 1, "Expected exactly one dcgm-exporter version")
			dcgmExporterVersion := dcgmExporterVersions[0]

			coreVersions := GetExpectedPackageVersions("datacenter-gpu-manager-4-core", tc.os, tc.osVersion)
			require.Len(t, coreVersions, 1, "Expected exactly one datacenter-gpu-manager-4-core version")
			expectedCoreVersion := coreVersions[0]

			propVersions := GetExpectedPackageVersions("datacenter-gpu-manager-4-proprietary", tc.os, tc.osVersion)
			require.Len(t, propVersions, 1, "Expected exactly one datacenter-gpu-manager-4-proprietary version")
			expectedPropVersion := propVersions[0]

			t.Logf("Expected versions from components.json:")
			t.Logf("  dcgm-exporter: %s", dcgmExporterVersion)
			t.Logf("  datacenter-gpu-manager-4-core: %s", expectedCoreVersion)
			t.Logf("  datacenter-gpu-manager-4-proprietary: %s", expectedPropVersion)

			// Download the dcgm-exporter package
			url := fmt.Sprintf(tc.downloadURL, dcgmExporterVersion)
			t.Logf("Downloading dcgm-exporter package from %s", url)

			tmpFile, err := os.CreateTemp("", "dcgm-exporter-*")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to download dcgm-exporter package from %s", url)

			_, err = io.Copy(tmpFile, resp.Body)
			require.NoError(t, err)
			require.NoError(t, tmpFile.Close())

			// Parse dependencies from the package
			actualCoreVersion, actualPropVersion := tc.parseDeps(t, tmpFile.Name())

			t.Logf("Actual versions from dcgm-exporter package:")
			t.Logf("  datacenter-gpu-manager-4-core: %s", actualCoreVersion)
			t.Logf("  datacenter-gpu-manager-4-proprietary: %s", actualPropVersion)

			// Verify versions match
			require.Equalf(t, expectedCoreVersion, actualCoreVersion,
				"datacenter-gpu-manager-4-core version mismatch: components.json has %s but dcgm-exporter requires %s",
				expectedCoreVersion, actualCoreVersion)

			require.Equalf(t, expectedPropVersion, actualPropVersion,
				"datacenter-gpu-manager-4-proprietary version mismatch: components.json has %s but dcgm-exporter requires %s",
				expectedPropVersion, actualPropVersion)

			t.Logf("✅ Version compatibility verified: dcgm-exporter %s is compatible with DCGM packages %s",
				dcgmExporterVersion, expectedCoreVersion)
		})
	}
}

// parseDebDeps extracts datacenter-gpu-manager-4-core and datacenter-gpu-manager-4-proprietary
// versions from a .deb package's control file.
func parseDebDeps(t *testing.T, path string) (string, string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	reader := ar.NewReader(f)
	for {
		header, err := reader.Next()
		require.NoError(t, err, "control file not found in .deb package")

		if !strings.HasPrefix(header.Name, "control.tar") {
			continue
		}

		var tarReader *tar.Reader
		if strings.HasSuffix(header.Name, ".gz") {
			gz, err := gzip.NewReader(reader)
			require.NoError(t, err)
			defer gz.Close()
			tarReader = tar.NewReader(gz)
		} else if strings.HasSuffix(header.Name, ".zst") {
			zr, err := zstd.NewReader(reader)
			require.NoError(t, err)
			defer zr.Close()
			tarReader = tar.NewReader(zr)
		} else {
			tarReader = tar.NewReader(reader)
		}

		for {
			th, err := tarReader.Next()
			require.NoError(t, err, "control file not found in control.tar")

			if th.Name == "./control" || th.Name == "control" {
				data, err := io.ReadAll(tarReader)
				require.NoError(t, err)

				// Parse Depends line
				scanner := bufio.NewScanner(strings.NewReader(string(data)))
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "Depends:") {
						coreRegex := regexp.MustCompile(`datacenter-gpu-manager-4-core \(= ([^)]+)\)`)
						propRegex := regexp.MustCompile(`datacenter-gpu-manager-4-proprietary \(= ([^)]+)\)`)

						coreMatches := coreRegex.FindStringSubmatch(line)
						require.Len(t, coreMatches, 2, "Failed to extract datacenter-gpu-manager-4-core version from Depends")

						propMatches := propRegex.FindStringSubmatch(line)
						require.Len(t, propMatches, 2, "Failed to extract datacenter-gpu-manager-4-proprietary version from Depends")

						return coreMatches[1], propMatches[1]
					}
				}
				require.Fail(t, "Depends line not found in control file")
			}
		}
	}
}

// parseRPMDeps extracts datacenter-gpu-manager-4-core and datacenter-gpu-manager-4-proprietary
// versions from an .rpm package's Requires metadata.
func parseRPMDeps(t *testing.T, path string) (string, string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	pkg, err := rpm.Read(f)
	require.NoError(t, err)

	var coreVersion, propVersion string

	for _, req := range pkg.Requires() {
		name := req.Name()
		if name == "datacenter-gpu-manager-4-core" {
			coreVersion = formatRPMVersion(req)
		}
		if name == "datacenter-gpu-manager-4-proprietary" {
			propVersion = formatRPMVersion(req)
		}
	}

	require.NotEmpty(t, coreVersion, "datacenter-gpu-manager-4-core dependency not found in RPM Requires")
	require.NotEmpty(t, propVersion, "datacenter-gpu-manager-4-proprietary dependency not found in RPM Requires")

	return coreVersion, propVersion
}

// formatRPMVersion formats an RPM dependency's version as "epoch:version-release",
// matching the version format used in components.json.
func formatRPMVersion(dep rpm.Dependency) string {
	epoch := dep.Epoch()
	version := dep.Version()
	release := dep.Release()
	if epoch > 0 {
		return fmt.Sprintf("%d:%s-%s", epoch, version, release)
	}
	if release != "" {
		return fmt.Sprintf("%s-%s", version, release)
	}
	return version
}

type versionCheck struct {
	input    string
	expected string
}

func TestRemoveLeadingV(t *testing.T) {
	tests := []versionCheck{
		{input: "v1.30.0", expected: "1.30.0"},
		{input: "v1.32.6", expected: "1.32.6"},
		{input: "1.30.0", expected: "1.30.0"},
		{input: "", expected: ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("testing removing leading v of \"%s\" gives \"%s\"", test.input, test.expected), func(t *testing.T) {
			require.Equal(t, test.expected, RemoveLeadingV(test.input))
		})
	}
}
