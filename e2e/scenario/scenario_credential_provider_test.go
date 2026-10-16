package scenario

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
)

func credentialProviderTestNBC() *datamodel.NodeBootstrappingConfiguration {
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{},
				WindowsProfile:      &datamodel.WindowsProfile{CseScriptsPackageURL: "published-cse"},
			},
		},
		K8sComponents: &datamodel.K8sComponents{},
		KubeletConfig: map[string]string{},
	}
}

func TestCredentialProviderScenariosRegisteredOnce(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vhd     *config.Image
		cluster func(context.Context, ClusterRequest) (*Cluster, error)
	}{
		{"Ubuntu2204_PMC_CredentialProvider_Kubernetes133", config.VHDUbuntu2204Gen2Containerd, ClusterKubenet},
		{"Windows2022_Dalec_CredentialProvider", config.VHDWindows2022ContainerdGen2, ClusterAzureNetwork},
		{"Windows2022Gen1_Dalec_CredentialProvider", config.VHDWindows2022Containerd, ClusterAzureNetwork},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var matches []*Scenario
			for _, s := range List() {
				if s.Name == tc.name {
					matches = append(matches, s)
				}
			}
			require.Len(t, matches, 1)
			s := matches[0]
			require.Equal(t, tc.vhd, s.VHD)
			require.Equal(t, reflect.ValueOf(tc.cluster).Pointer(), reflect.ValueOf(s.Cluster).Pointer())
			require.Nil(t, s.Runtime, "listing must not prepare a scenario")
			require.Empty(t, s.SkipReason)
			require.Nil(t, s.BootstrapConfigMutator)
			require.NotNil(t, s.BootstrapConfigMutatorWithError)
			require.NotNil(t, s.Validator)
			require.NotNil(t, s.VMConfigMutator)
			vmss := &armcompute.VirtualMachineScaleSet{Tags: map[string]*string{}}
			s.VMConfigMutator(vmss)
			require.Empty(t, vmss.Tags, "the natural gate must not set ShouldEnforceKubePMCInstall")

			if tc.name == "Ubuntu2204_PMC_CredentialProvider_Kubernetes133" {
				nbc := credentialProviderTestNBC()
				require.NoError(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nbc))
				version := nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
				require.Contains(t, components.GetExpectedPackageVersions("kubernetes-binaries", "default", "current"), "v"+version)
				require.Equal(t, "/var/lib/kubelet/credential-provider-config.yaml", nbc.KubeletConfig["--image-credential-provider-config"])
				require.Equal(t, "/var/lib/kubelet/credential-provider", nbc.KubeletConfig["--image-credential-provider-bin-dir"])
				require.Equal(t, "published-cse", nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
			} else {
				nbc := credentialProviderTestNBC()
				require.NoError(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nbc))
				expected := credentialProviderTestNBC()
				require.NoError(t, configureWindowsDalecCredentialProvider(expected,
					components.GetExpectedPackageVersions("kubernetes-binaries", "windows", "default"),
					components.GetExpectedPackageVersions("windows credential provider", "windows", "default")))
				require.Equal(t, expected, nbc)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				require.ErrorIs(t, s.BootstrapConfigMutatorWithError(ctx, nil, credentialProviderTestNBC()), context.Canceled)
			}
		})
	}
}

func TestCredentialProvider133Version(t *testing.T) {
	for _, tc := range []struct {
		name     string
		versions []string
		want     string
	}{
		{"numeric patch order", []string{"v1.33.9", "v1.33.13", "v1.33.12"}, "1.33.13"},
		{"legacy versions without v", []string{"1.32.8", "1.33.3", "1.34.1"}, "1.33.3"},
		{"absent", nil, ""},
		{"other minors cannot replace gate", []string{"v1.32.11", "v1.34.11", "v1.330.1"}, ""},
		{"invalid incomplete and prerelease", []string{"invalid", "v1.33", "v1.33.0-rc.1"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := credentialProvider133Version(tc.versions, "test/windows/default")
			if tc.want == "" {
				require.ErrorContains(t, err, "cached Kubernetes 1.33 patch")
				require.ErrorContains(t, err, "components.json test/windows/default")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCredentialProviderScenarioMetadataSetup(t *testing.T) {
	t.Run("Ubuntu selects cached patch", func(t *testing.T) {
		nbc := credentialProviderTestNBC()
		require.NoError(t, configureUbuntuPMCCredentialProvider(nbc, []string{"v1.33.9", "v1.33.12"}))
		require.Equal(t, "1.33.12", nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
		require.Empty(t, nbc.K8sComponents.WindowsPackageURL)
		require.Empty(t, nbc.K8sComponents.WindowsCredentialProviderURL)
	})
	t.Run("Ubuntu missing minor fails", func(t *testing.T) {
		require.ErrorContains(t, configureUbuntuPMCCredentialProvider(credentialProviderTestNBC(), []string{"v1.34.1"}), "kubernetes-binaries/default/current")
	})
	t.Run("Windows independently selects kubelet and legacy provider", func(t *testing.T) {
		nbc := credentialProviderTestNBC()
		require.NoError(t, configureWindowsDalecCredentialProvider(nbc, []string{"v1.33.12"}, []string{"1.33.3"}))
		require.Equal(t, "1.33.12", nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
		require.Equal(t, "https://packages.aks.azure.com/kubernetes/v1.33.12/windowszip/v1.33.12-1int.zip", nbc.K8sComponents.WindowsPackageURL)
		require.Equal(t, "https://packages.aks.azure.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz", nbc.K8sComponents.WindowsCredentialProviderURL)
		require.Equal(t, `C:\k\credential-provider-config.yaml`, nbc.KubeletConfig["--image-credential-provider-config"])
		require.Equal(t, `C:\var\lib\kubelet\credential-provider`, nbc.KubeletConfig["--image-credential-provider-bin-dir"])
		require.Equal(t, "published-cse", nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
	})
	t.Run("Windows missing kubelet never falls back to Linux metadata", func(t *testing.T) {
		require.ErrorContains(t, configureWindowsDalecCredentialProvider(credentialProviderTestNBC(), nil, []string{"1.33.3"}), "kubernetes-binaries/windows/default")
	})
	t.Run("Windows missing legacy provider never infers kubelet release", func(t *testing.T) {
		nbc := credentialProviderTestNBC()
		require.ErrorContains(t, configureWindowsDalecCredentialProvider(nbc, []string{"v1.33.12"}, []string{"1.32.8"}), "windows credential provider/windows/default")
		require.Empty(t, nbc.K8sComponents.WindowsCredentialProviderURL)
	})
}

func TestCredentialProviderWindowsPreservesCSEURL(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, url := range []string{"", "https://packages.aks.azure.com/aks/windows/cse/", "https://example.invalid/custom-cse.zip"} {
		t.Run(url, func(t *testing.T) {
			nbc := credentialProviderTestNBC()
			nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = url
			require.NoError(t, windowsDalecCredentialProviderMutator(t.Context(), nil, nbc))
			require.Equal(t, url, nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
		})
	}

	nbc := credentialProviderTestNBC()
	require.NoError(t, windowsDalecCredentialProviderMutator(t.Context(), nil, nbc))

	version := nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
	require.Contains(t, components.GetExpectedPackageVersions("kubernetes-binaries", "windows", "default"), "v"+version)
	require.Equal(t, fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", version, version), nbc.K8sComponents.WindowsPackageURL)
	legacyVersion, err := credentialProvider133Version(components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), "windows credential provider/windows/default")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", legacyVersion, legacyVersion), nbc.K8sComponents.WindowsCredentialProviderURL)
}

func TestCredentialProviderWindowsCancellation(t *testing.T) {
	nbc := credentialProviderTestNBC()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, windowsDalecCredentialProviderMutator(ctx, nil, nbc), context.Canceled)
	require.Equal(t, credentialProviderTestNBC(), nbc)
}
