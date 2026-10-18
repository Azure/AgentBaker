package scenario

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"
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
		minor   uint64
	}{
		{"Ubuntu2204_Dalec_CredentialProvider_Kubernetes133", config.VHDUbuntu2204Gen2Containerd, ClusterKubenet, 33},
		{"Windows2022_Dalec_CredentialProvider", config.VHDWindows2022ContainerdGen2, ClusterAzureNetwork, 33},
		{"Windows2022Gen1_Dalec_CredentialProvider", config.VHDWindows2022Containerd, ClusterAzureNetwork, 33},
		{"Windows2022_Legacy_CredentialProvider_Kubernetes132", config.VHDWindows2022ContainerdGen2, ClusterAzureNetwork, 32},
		{"Windows2022Gen1_Legacy_CredentialProvider_Kubernetes132", config.VHDWindows2022Containerd, ClusterAzureNetwork, 32},
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
			require.False(t, s.SkipDefaultValidation, "node readiness and workload validation must run")
			require.NotNil(t, s.VMConfigMutator)
			vmss := &armcompute.VirtualMachineScaleSet{Tags: map[string]*string{}}
			s.VMConfigMutator(vmss)
			require.Empty(t, vmss.Tags, "the natural gate must not set ShouldEnforceKubePMCInstall")

			if tc.name == "Ubuntu2204_Dalec_CredentialProvider_Kubernetes133" {
				nbc := credentialProviderTestNBC()
				require.NoError(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nbc))
				version := nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
				require.True(t, strings.HasPrefix(version, "1.33."))
				require.Contains(t, components.GetExpectedPackageVersions("kubernetes-binaries", "default", "current"), "v"+version)
				require.Equal(t, "/var/lib/kubelet/credential-provider-config.yaml", nbc.KubeletConfig["--image-credential-provider-config"])
				require.Equal(t, "/var/lib/kubelet/credential-provider", nbc.KubeletConfig["--image-credential-provider-bin-dir"])
				require.Equal(t, "published-cse", nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
			} else {
				require.NotNil(t, s.SkipIf)
				nbc := credentialProviderTestNBC()
				require.NoError(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nbc))
				expected := credentialProviderTestNBC()
				require.NoError(t, configureWindowsCredentialProvider(expected,
					components.GetExpectedPackageVersions("kubernetes-binaries", "windows", "default"),
					components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), tc.minor))
				require.Equal(t, expected, nbc)
				require.True(t, strings.HasPrefix(nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, fmt.Sprintf("1.%d.", tc.minor)))
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				require.ErrorIs(t, s.BootstrapConfigMutatorWithError(ctx, nil, credentialProviderTestNBC()), context.Canceled)
			}
		})
	}
	for _, s := range List() {
		require.NotEqual(t, "Ubuntu2204_PMC_CredentialProvider_Kubernetes133", s.Name, "the old dedicated PMC name must not remain registered")
	}
}

func TestCredentialProviderWindowsEligibility(t *testing.T) {
	savedConfig := *config.Config
	t.Cleanup(func() { *config.Config = savedConfig })

	for _, minor := range []uint64{32, 33} {
		for _, vhd := range []*config.Image{config.VHDWindows2022ContainerdGen2, config.VHDWindows2022Containerd} {
			t.Run(fmt.Sprintf("1.%d/%s", minor, vhd.Name), func(t *testing.T) {
				// Construct before configuring the lane: registration must not latch eligibility.
				image := *vhd
				s := newWindowsCredentialProviderScenario("CredentialProvider", &image, minor)
				require.Empty(t, s.SkipReason)
				require.Nil(t, s.Runtime)
				for _, tc := range []struct {
					name         string
					currentBuild string
					tagName      string
					tagValue     string
					version      string
					metadataFile string
					wantSkip     string
				}{
					{name: "standalone main", currentBuild: "123", tagName: "branch", tagValue: "refs/heads/main", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "older build", currentBuild: "123", tagName: "buildId", tagValue: "122", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "local explicit build", tagName: "buildId", tagValue: "123", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "empty build IDs", tagName: "buildId", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "empty selector", currentBuild: "123", tagName: "buildId", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "wrong tag", currentBuild: "123", tagName: "branch", tagValue: "123", wantSkip: "require buildId=BUILD_BUILDID"},
					{name: "pinned image overrides current build", currentBuild: "123", tagName: "buildId", tagValue: "123", version: "202601.01.0", wantSkip: "pinned versions and VHD metadata overrides"},
					{name: "metadata overrides current build", currentBuild: "123", tagName: "buildId", tagValue: "123", metadataFile: "images.json", wantSkip: "pinned versions and VHD metadata overrides"},
					{name: "current build", currentBuild: "123", tagName: "buildId", tagValue: "123"},
				} {
					t.Run(tc.name, func(t *testing.T) {
						t.Setenv("BUILD_BUILDID", tc.currentBuild)
						// Neither raw VHD_BUILD_ID nor the runner's BUILD_ID proves eligibility.
						t.Setenv("VHD_BUILD_ID", "123")
						config.Config.BuildID = "123"
						config.Config.SIGVersionTagName = tc.tagName
						config.Config.SIGVersionTagValue = tc.tagValue
						config.Config.VHDMetadataFile = tc.metadataFile
						image.Version = tc.version

						reason := s.SkipIf(t.Context())
						if tc.wantSkip != "" {
							require.Contains(t, reason, tc.wantSkip)
							outcome := Run(t.Context(), s.Name, s.Name, s)
							require.NoError(t, outcome.Error)
							require.Equal(t, reason, outcome.SkipReason)
						} else {
							require.Empty(t, reason)
							for _, runErr := range []error{nil, errors.New("credential-provider validation failed")} {
								ran := false
								outcome := runExecution(t.Context(), s.Name, s.Name, s,
									func(ctx context.Context, _ string, prepared *Scenario) error {
										ran = true
										require.NotSame(t, s, prepared)
										require.Equal(t, &image, prepared.VHD)
										nbc := credentialProviderTestNBC()
										require.NoError(t, prepared.BootstrapConfigMutatorWithError(ctx, nil, nbc))
										require.Equal(t, "published-cse", nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
										return runErr
									})
								require.True(t, ran)
								require.Empty(t, outcome.SkipReason)
								require.ErrorIs(t, outcome.Error, runErr)
							}
						}
						require.Empty(t, s.SkipReason)
						require.Nil(t, s.Runtime)
					})
				}
			})
		}
	}
}

func TestCredentialProviderVersion(t *testing.T) {
	for _, minor := range []uint64{32, 33} {
		prefix := fmt.Sprintf("1.%d", minor)
		otherMinor := uint64(32)
		if minor == 32 {
			otherMinor = 33
		}
		for _, tc := range []struct {
			name     string
			versions []string
			want     string
		}{
			{"numeric patch order", []string{"v" + prefix + ".9", "v" + prefix + ".13", "v" + prefix + ".12"}, prefix + ".13"},
			{"legacy versions without v", []string{"1.31.8", prefix + ".3", "1.34.1"}, prefix + ".3"},
			{"absent", nil, ""},
			{"other minors cannot replace gate", []string{"v1.31.11", "v1.34.11", "v" + prefix + "0.1", fmt.Sprintf("v2.%d.1", minor)}, ""},
			{"opposite side of gate cannot replace minor", []string{fmt.Sprintf("v1.%d.12", otherMinor)}, ""},
			{"invalid incomplete and prerelease", []string{"invalid", "v" + prefix, "v" + prefix + ".0-rc.1", "v" + prefix + ".01", "vv" + prefix + ".2"}, ""},
			{"higher prerelease excluded", []string{prefix + ".9", "v" + prefix + ".14-rc.1", prefix + ".13-beta.1"}, prefix + ".9"},
		} {
			t.Run(prefix+"/"+tc.name, func(t *testing.T) {
				got, err := credentialProviderVersion(tc.versions, "test/windows/default", minor)
				if tc.want == "" {
					require.ErrorContains(t, err, "cached "+prefix+" patch")
					require.ErrorContains(t, err, "components.json test/windows/default")
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			})
		}
	}
}

func TestCredentialProviderScenarioMetadataSetup(t *testing.T) {
	t.Run("Ubuntu Dalec selects cached patch", func(t *testing.T) {
		nbc := credentialProviderTestNBC()
		require.NoError(t, configureUbuntuDalecCredentialProvider(nbc, []string{"v1.33.9", "v1.33.12"}))
		require.Equal(t, "1.33.12", nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
		require.Empty(t, nbc.K8sComponents.WindowsPackageURL)
		require.Empty(t, nbc.K8sComponents.WindowsCredentialProviderURL)
	})
	t.Run("Ubuntu Dalec missing minor fails", func(t *testing.T) {
		require.ErrorContains(t, configureUbuntuDalecCredentialProvider(credentialProviderTestNBC(), []string{"v1.34.1"}), "kubernetes-binaries/default/current")
	})
	for _, minor := range []uint64{32, 33} {
		t.Run(fmt.Sprintf("Windows 1.%d", minor), func(t *testing.T) {
			t.Run("Windows independently selects kubelet and legacy provider", func(t *testing.T) {
				nbc := credentialProviderTestNBC()
				kubeletVersion := fmt.Sprintf("1.%d.12", minor)
				providerVersion := fmt.Sprintf("1.%d.3", minor)
				require.NoError(t, configureWindowsCredentialProvider(nbc, []string{"v" + kubeletVersion}, []string{providerVersion}, minor))
				require.Equal(t, kubeletVersion, nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				require.Equal(t, fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", kubeletVersion, kubeletVersion), nbc.K8sComponents.WindowsPackageURL)
				legacyURL, err := windowsLegacyCredentialProviderURL(providerVersion)
				require.NoError(t, err)
				require.Equal(t, legacyURL, nbc.K8sComponents.WindowsCredentialProviderURL)
				require.Contains(t, nbc.K8sComponents.WindowsCredentialProviderURL, providerVersion)
				require.NotContains(t, nbc.K8sComponents.WindowsCredentialProviderURL, kubeletVersion)
				require.Equal(t, `C:\k\credential-provider-config.yaml`, nbc.KubeletConfig["--image-credential-provider-config"])
				require.Equal(t, `C:\var\lib\kubelet\credential-provider`, nbc.KubeletConfig["--image-credential-provider-bin-dir"])
				require.Equal(t, "published-cse", nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
			})
			t.Run("Windows missing kubelet never falls back to Linux metadata", func(t *testing.T) {
				nbc := credentialProviderTestNBC()
				require.ErrorContains(t, configureWindowsCredentialProvider(nbc, []string{"v1.34.1"}, []string{fmt.Sprintf("1.%d.3", minor)}, minor), "kubernetes-binaries/windows/default")
				require.Equal(t, credentialProviderTestNBC(), nbc)
			})
			t.Run("Windows missing legacy provider never infers kubelet release", func(t *testing.T) {
				nbc := credentialProviderTestNBC()
				require.ErrorContains(t, configureWindowsCredentialProvider(nbc, []string{fmt.Sprintf("v1.%d.12", minor)}, []string{"1.31.8"}, minor), "windows credential provider/windows/default")
				require.Equal(t, credentialProviderTestNBC(), nbc)
			})
		})
	}
}

func TestCredentialProviderWindowsPreservesCSEURL(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, minor := range []uint64{32, 33} {
		t.Run(fmt.Sprintf("1.%d", minor), func(t *testing.T) {
			for _, url := range []string{"", "https://packages.aks.azure.com/aks/windows/cse/", "https://example.invalid/custom-cse.zip"} {
				t.Run(url, func(t *testing.T) {
					nbc := credentialProviderTestNBC()
					nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = url
					require.NoError(t, windowsCredentialProviderMutator(t.Context(), nil, nbc, minor))
					require.Equal(t, url, nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL)
				})
			}

			nbc := credentialProviderTestNBC()
			require.NoError(t, windowsCredentialProviderMutator(t.Context(), nil, nbc, minor))

			version := nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
			require.Contains(t, components.GetExpectedPackageVersions("kubernetes-binaries", "windows", "default"), "v"+version)
			require.Equal(t, fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", version, version), nbc.K8sComponents.WindowsPackageURL)
			legacyVersion, err := credentialProviderVersion(components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), "windows credential provider/windows/default", minor)
			require.NoError(t, err)
			legacyURL, err := windowsLegacyCredentialProviderURL(legacyVersion)
			require.NoError(t, err)
			require.Equal(t, legacyURL, nbc.K8sComponents.WindowsCredentialProviderURL)
		})
	}
}

func TestCredentialProviderWindowsCancellation(t *testing.T) {
	for _, minor := range []uint64{32, 33} {
		nbc := credentialProviderTestNBC()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, windowsCredentialProviderMutator(ctx, nil, nbc, minor), context.Canceled)
		require.Equal(t, credentialProviderTestNBC(), nbc)
	}
}

func TestCredentialProviderWindowsLegacyURLMetadata(t *testing.T) {
	metadata := []byte(`{"Packages":[{"name":"windows credential provider","downloadURIs":{"windows":{"default":{"downloadURL":"https://metadata.example.invalid/releases/${version}/provider-${version}.tar.gz"}}}}]}`)
	got, err := resolveWindowsLegacyCredentialProviderURL(metadata, "1.32.8")
	require.NoError(t, err)
	require.Equal(t, "https://metadata.example.invalid/releases/1.32.8/provider-1.32.8.tar.gz", got)

	for _, tc := range []struct {
		name string
		data string
	}{
		{"invalid JSON", `{`},
		{"missing package", `{"Packages":[]}`},
		{"missing windows", `{"Packages":[{"name":"windows credential provider","downloadURIs":{"default":{}}}]}`},
		{"missing URL", `{"Packages":[{"name":"windows credential provider","downloadURIs":{"windows":{"default":{}}}}]}`},
		{"unversioned URL", strings.ReplaceAll(string(metadata), "${version}", "fixed")},
		{"unresolved variable", strings.ReplaceAll(string(metadata), "provider-", "${unknown}-")},
		{"invalid URL", strings.ReplaceAll(string(metadata), "https://metadata.example.invalid", "://broken")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveWindowsLegacyCredentialProviderURL([]byte(tc.data), "1.32.8")
			require.ErrorContains(t, err, "components.json windows credential provider/windows/default/downloadURL")
		})
	}
}

func TestCredentialProviderWindowsLegacyLog(t *testing.T) {
	version, err := credentialProviderVersion(components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), "windows credential provider/windows/default", 32)
	require.NoError(t, err)
	legacyURL, err := windowsLegacyCredentialProviderURL(version)
	require.NoError(t, err)
	filename := path.Base(legacyURL)
	for _, tc := range []struct {
		name      string
		log       string
		wantError bool
	}{
		{"cache copy", fmt.Sprintf(`Using cached version of %s - Copying file from C:\akse-cache\credential-provider\%s to C:\tmp\credentialprovider.tar.gz`, filename, filename), false},
		{"completed download", fmt.Sprintf(`Downloaded file %s to C:\tmp\credentialprovider.tar.gz in 12 ms`, legacyURL), false},
		{"remapped download", fmt.Sprintf(`Downloaded file https://packages.aks.azure.us/binaries/%s to C:\tmp\credentialprovider.tar.gz in 12 ms`, filename), false},
		{"generic pre-selection", fmt.Sprintf(`Download credential provider binary from %s to C:\var\lib\kubelet\credential-provider`, legacyURL), true},
		{"attempted download", fmt.Sprintf(`Updated URL %s -> %s to download %s to C:\tmp\credentialprovider.tar.gz`, legacyURL, legacyURL, filename), true},
		{"different provider version", `Using cached version of other-provider.tar.gz - Copying file from C:\cache\other-provider.tar.gz to C:\tmp\credentialprovider.tar.gz`, true},
		{"different destination", fmt.Sprintf(`Downloaded file %s to C:\tmp\unrelated.tar.gz in 12 ms`, legacyURL), true},
		{"unrelated lines", fmt.Sprintf("Using cached version of %s - Copying file from C:\\cache\\%s\nunrelated to C:\\tmp\\credentialprovider.tar.gz", filename, filename), true},
		{"missing log", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWindowsLegacyCredentialProviderLog(tc.log, legacyURL)
			if tc.wantError {
				require.ErrorContains(t, err, "expected legacy credential-provider")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
