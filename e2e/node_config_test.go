package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNBCToAKSNodeConfigV1PreservesContainerdConfig(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.33.5", "amd64")
	nbc.ContainerdVersion = "2.3.2-ubuntu24.04u1"
	nbc.ContainerdPackageURL = "https://packages.aks.azure.com/containerd/test-containerd.deb"
	nbc.CloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase = "https://packages.aks.azure.com/containerd/"

	cfg := nbcToAKSNodeConfigV1(nbc)

	require.NotNil(t, cfg.ContainerdConfig)
	require.Equal(t, nbc.CloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase, cfg.ContainerdConfig.ContainerdDownloadUrlBase)
	require.Equal(t, nbc.ContainerdVersion, cfg.ContainerdConfig.ContainerdVersion)
	require.Equal(t, nbc.ContainerdPackageURL, cfg.ContainerdConfig.ContainerdPackageUrl)
}
