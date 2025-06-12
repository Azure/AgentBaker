package e2e

import (
	"fmt"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImagesAreFullySpecified(t *testing.T) {
	images := getContainerImages("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	tags := getContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	image := images[0]
	tag := tags[0]
	require.Equal(t, fmt.Sprintf("mcr.microsoft.com/windows/servercore:%s", tag), image, "Image does not contain the expected tag")
}

func TestWs2025Gen2ServerCore(t *testing.T) {
	serverCoreVersions := getContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Gen2Nanoserver(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2025ServerCore(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Nanoserver(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2ServerCore(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2Nanoserver(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs23H2ServerCoreGen2(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2NanoserverGen2(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCore(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022Nanoserver(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCoreGen2(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022NanoserverGen2(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2019ServerCore(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/servercore:*", "2019-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2019Nanoserver(t *testing.T) {
	serverCoreVersions := getContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2019-containerd")
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
		config.VHDWindows2019Containerd,
	}

	for _, image := range windowsImages {
		t.Run(fmt.Sprintf("testing servercore has versions for %s", image.Name), func(t *testing.T) {
			images := getServercoreImagesForVHD(image)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows servercore images found")
		})
		t.Run(fmt.Sprintf("testing nanoserver has versions for %s", image.Name), func(t *testing.T) {
			images := getNanoserverImagesForVhd(image)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows nanoserver images found")
		})
	}
}
