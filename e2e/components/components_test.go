package components

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
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

func TestGetE2EContainerImage(t *testing.T) {
	t.Run("returns image for known name", func(t *testing.T) {
		image := GetE2EContainerImage("nvidia-k8s-device-plugin")
		require.NotEmpty(t, image, "expected non-empty image for nvidia-k8s-device-plugin")
		require.Contains(t, image, "mcr.microsoft.com/oss/v2/nvidia/k8s-device-plugin:")
		require.NotContains(t, image, "*", "wildcard should be replaced with version")
	})

	t.Run("returns empty for unknown name", func(t *testing.T) {
		image := GetE2EContainerImage("nonexistent-image")
		require.Empty(t, image, "expected empty image for unknown name")
	})
}

// baseSemver extracts the major.minor.patch portion from a version string,
// stripping any leading 'v' and trailing distro/packaging suffixes.
// e.g. "v0.18.2-1" -> "0.18.2", "0.18.2-ubuntu22.04u1" -> "0.18.2"
func baseSemver(version string) string {
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	m := re.FindStringSubmatch(version)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// TestNvidiaDevicePluginVersionConsistency ensures the E2E container image version
// (MCR tag) stays aligned with the managed deb package version on the base semver.
// These are different artifacts (container image vs deb package) with different suffixes,
// but the base version (e.g. 0.18.2) must match to ensure we test the same release.
func TestNvidiaDevicePluginVersionConsistency(t *testing.T) {
	e2eImage := GetE2EContainerImage("nvidia-k8s-device-plugin")
	require.NotEmpty(t, e2eImage, "E2E container image not found in components.json")

	// Extract version tag from the image URL (after the last ':')
	parts := regexp.MustCompile(`:`).Split(e2eImage, -1)
	require.Len(t, parts, 2, "expected image:tag format, got %s", e2eImage)
	e2eBase := baseSemver(parts[1])
	require.NotEmpty(t, e2eBase, "could not extract semver from E2E image tag %q", parts[1])

	// Check against each distro/release that has the managed package
	distroReleases := []struct {
		distro  string
		release string
	}{
		{"ubuntu", "r2204"},
		{"ubuntu", "r2404"},
		{"azurelinux", "v3.0"},
	}

	for _, dr := range distroReleases {
		t.Run(fmt.Sprintf("%s/%s", dr.distro, dr.release), func(t *testing.T) {
			versions := GetExpectedPackageVersions("nvidia-device-plugin", dr.distro, dr.release)
			require.NotEmpty(t, versions, "no managed nvidia-device-plugin version found for %s/%s", dr.distro, dr.release)

			managedBase := baseSemver(versions[0])
			require.NotEmpty(t, managedBase, "could not extract semver from managed version %q", versions[0])
			require.Equal(t, e2eBase, managedBase,
				"E2E container image base version (%s from %s) does not match managed package version (%s from %s) for %s/%s",
				e2eBase, parts[1], managedBase, versions[0], dr.distro, dr.release)
		})
	}
}
