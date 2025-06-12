package e2e

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWs2025Gen2ServerCore(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Gen2Nanoserver(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "2025-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2025ServerCore(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 4)
}

func TestWs2025Nanoserver(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "2025")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2ServerCore(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2Nanoserver(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "23H2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs23H2ServerCoreGen2(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs23H2NanoserverGen2(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "23H2-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCore(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022Nanoserver(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2022ServerCoreGen2(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2022NanoserverGen2(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "2022-containerd-gen2")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}

func TestWs2019ServerCore(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/servercore:*", "2019-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 2)
}

func TestWs2019Nanoserver(t *testing.T) {
	serverCoreVersions := getExpectedContainerImagesWindows("mcr.microsoft.com/windows/nanoserver:*", "2019-containerd")
	t.Logf("found servercore version %v", serverCoreVersions)
	require.Len(t, serverCoreVersions, 1)
}
