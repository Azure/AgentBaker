package components

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestImagesAreFullySpecified(t *testing.T) {
	images, err := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	require.NoError(t, err)
	tags, err := getWindowsContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	require.NoError(t, err)
	image := images[0]
	tag := tags[0]
	require.Equal(t, fmt.Sprintf("mcr.microsoft.com/windows/servercore:%s", tag), image, "Image does not contain the expected tag")
}

func TestWs2025Gen2ServerCore(t *testing.T) {
	serverCoreVersions, err := getWindowsContainerImageTags("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 6)
}

func TestWs2025Gen2Nanoserver(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025-gen2")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2025ServerCore(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2025")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 6)
}

func TestWs2025Nanoserver(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2025")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022ServerCore(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 3)
}

func TestWs2022Nanoserver(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCoreGen2(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/servercore:*", "2022-containerd-gen2")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 3)
}

func TestWs2022NanoserverGen2(t *testing.T) {
	serverCoreVersions, err := GetWindowsContainerImages("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd-gen2")
	require.NoError(t, err)
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWindowsImagesHaveServercoreAndNanoserverSpecified(t *testing.T) {
	// This test ensures that all Windows images have the servercore tag specified.
	// If this test fails, it means that a new Windows image has been added without the servercore tag.

	windowsImages := []*config.Image{
		config.VHDWindows2022Containerd,
		config.VHDWindows2022ContainerdGen2,
		config.VHDWindows2025,
		config.VHDWindows2025Gen2,
		config.VHDWindows2025Gen2TL,
	}

	for _, image := range windowsImages {
		cached := *image
		cached.Name = "abe2etest-westus3-amd64-genV1-windows"
		t.Run(fmt.Sprintf("testing servercore has versions for %s", image.Name), func(t *testing.T) {
			images, err := GetServercoreImagesForVHD(image)
			require.NoError(t, err)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows servercore images found")
			cachedImages, err := GetServercoreImagesForVHD(&cached)
			require.NoError(t, err)
			require.Equal(t, images, cachedImages)
		})
		t.Run(fmt.Sprintf("testing nanoserver has versions for %s", image.Name), func(t *testing.T) {
			images, err := GetNanoserverImagesForVhd(image)
			require.NoError(t, err)
			t.Logf("found servercore version %v", images)
			require.NotEmpty(t, images, "No Windows nanoserver images found")
			cachedImages, err := GetNanoserverImagesForVhd(&cached)
			require.NoError(t, err)
			require.Equal(t, images, cachedImages)
		})
	}
}

func TestWindowsContainerImageTagsReadError(t *testing.T) {
	tags, err := getWindowsContainerImageTagsFromFS(fstest.MapFS{}, "servercore:*", "2025")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.ErrorContains(t, err, "parts/common/components.json")
	require.Nil(t, tags)
}

func TestWindowsContainerImageTagsInvalidManifest(t *testing.T) {
	for _, tt := range []struct {
		name     string
		manifest string
		wantErr  string
	}{
		{"invalid JSON", `{"ContainerImages":[`, "invalid JSON"},
		{"empty images", `{"ContainerImages":[]}`, "no Windows image tags"},
		{"empty latest", `{"ContainerImages":[{"downloadURL":"servercore:*","windowsVersions":[{"latestVersion":""}]}]}`, "latestVersion"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := fstest.MapFS{"parts/common/components.json": {Data: []byte(tt.manifest)}}
			tags, err := getWindowsContainerImageTagsFromFS(files, "servercore:*", "2025")
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, tags)
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
