package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestNBCToAKSNodeConfigV1PreservesContainerdConfig(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.33.5", "amd64")
	nbc.ContainerdVersion = "2.3.2-ubuntu24.04u1"
	nbc.ContainerdPackageURL = "https://packages.aks.azure.com/containerd/test-containerd.deb"
	nbc.CloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase = "https://packages.aks.azure.com/containerd/"

	cfg, err := nbcToAKSNodeConfigV1(nbc)

	require.NoError(t, err)
	require.NotNil(t, cfg.ContainerdConfig)
	require.Equal(t, nbc.CloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase, cfg.ContainerdConfig.ContainerdDownloadUrlBase)
	require.Equal(t, nbc.ContainerdVersion, cfg.ContainerdConfig.ContainerdVersion)
	require.Equal(t, nbc.ContainerdPackageURL, cfg.ContainerdConfig.ContainerdPackageUrl)
}

func TestSetExpectedContainerdVersionForE2EDefaultsAzureLinuxV3(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.33.5", "amd64")

	setExpectedContainerdVersionForE2E(t, nbc, config.VHDAzureLinuxV3Gen2)

	expectedVersions := components.GetExpectedPackageVersions("containerd", "azurelinux", "v3.0")
	require.NotEmpty(t, expectedVersions)
	require.Equal(t, expectedVersions[0], nbc.ContainerdVersion)
}

func TestSetExpectedContainerdVersionForE2EDefaultsUbuntu2404(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.33.5", "amd64")

	setExpectedContainerdVersionForE2E(t, nbc, config.VHDUbuntu2404Gen2Containerd)

	expectedVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
	require.NotEmpty(t, expectedVersions)
	require.Equal(t, expectedVersions[0], nbc.ContainerdVersion)
}

func TestSetExpectedContainerdVersionForE2EPreservesExplicitVersion(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.33.5", "amd64")
	nbc.ContainerdVersion = "2.1.4-custom"

	setExpectedContainerdVersionForE2E(t, nbc, config.VHDUbuntu2404Gen2Containerd)

	require.Equal(t, "2.1.4-custom", nbc.ContainerdVersion)
}
