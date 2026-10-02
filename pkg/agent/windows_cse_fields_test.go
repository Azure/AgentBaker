package agent

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

// These tests check how each value is written into the Windows CSE script
// (parts/windows/kuberneteswindowssetup.ps1.template): every input must reach PowerShell with its
// expected value and type, without expanding or running any part of the input.

// windowsCSETestValues are the values that each text field is tested with. Running the "code" value would
// set a canary variable, which parts/windows/kuberneteswindowssetup.fields.tests.ps1 checks for.
var windowsCSETestValues = map[string]string{
	"plain":   "value-1.example.com",
	"quotes":  `a'b"c` + "`" + `d''e""f`,
	"code":    `$(Set-Variable -Name AKSInjectionCanary -Value 1 -Scope Global);${env:PATH}@(1)#<#`,
	"unicode": "i\u2018j\u2019k\u201cl\u201dm\u201en\u2013o\U0001F600",
	"control": "x\r\ny\tz",
	"empty":   "",
}

const windowsCSEFieldsFixture = "testdata/windowscse/fields.tsv"
const windowsCSEPackageURLWithDollar = "https://example.blob.core.windows.net/$web/windows-cse.zip"

// windowsCSEField is one value that AgentBaker writes into the Windows CSE script.
type windowsCSEField struct {
	// action identifies the input, independently of any encoding applied by the template.
	action string
	// prefix is the text right before the value in the rendered script.
	prefix string
	// kind describes the expected runtime type; empty means string.
	kind string
	// set puts value into the NodeBootstrappingConfiguration field that the value comes from. It is nil
	// when the value is never free text: a boolean, a number, base64, or a name that AgentBaker builds.
	set func(c *datamodel.NodeBootstrappingConfiguration, value string)
	// want returns the value that the script must get. nil means the value itself.
	want func(c *datamodel.NodeBootstrappingConfiguration, value string) string
	// setup turns on the template branch that writes a value whose set is nil.
	setup func(c *datamodel.NodeBootstrappingConfiguration)
	// stub is prepended to the rendered line to capture a command argument without running the command.
	// It must store the argument in a global variable named like the parameter.
	stub string
}

func orDefault(fallback string) func(*datamodel.NodeBootstrappingConfiguration, string) string {
	return func(_ *datamodel.NodeBootstrappingConfiguration, value string) string {
		if value == "" {
			return fallback
		}
		return value
	}
}

func withCloudSpec(c *datamodel.NodeBootstrappingConfiguration, update func(*datamodel.KubernetesSpecConfig)) {
	cloudSpec := *c.CloudSpecConfig
	update(&cloudSpec.KubernetesSpecConfig)
	c.CloudSpecConfig = &cloudSpec
}

func customVNetSubnetID(vnet, subnet string) string {
	return fmt.Sprintf("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s", vnet, subnet)
}

// windowsCSEFields lists every value in the Windows CSE script, in template order.
// TestWindowsCSETemplateInputsHaveTests fails if a template input is missing here.
func windowsCSEFields() []windowsCSEField {
	windowsProfile := func(c *datamodel.NodeBootstrappingConfiguration) *datamodel.WindowsProfile {
		return c.ContainerService.Properties.WindowsProfile
	}
	kubernetesConfig := func(c *datamodel.NodeBootstrappingConfiguration) *datamodel.KubernetesConfig {
		return c.ContainerService.Properties.OrchestratorProfile.KubernetesConfig
	}
	unescapedArg := func(_ *datamodel.NodeBootstrappingConfiguration, value string) string {
		return "--aaa-test=" + strings.ReplaceAll(value, `""`, `"`)
	}
	return []windowsCSEField{
		{action: `GetKubernetesEndpoint`, prefix: `$MasterIP=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.HostedMasterProfile.IPAddress = ""
			c.ContainerService.Properties.HostedMasterProfile.FQDN = v
		}},
		{action: `GetParameter "kubeDNSServiceIP"`, prefix: `$KubeDnsServiceIp=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).DNSServiceIP = v
		}},
		{action: `GetParameter "masterEndpointDNSNamePrefix"`, prefix: `$MasterFQDNPrefix=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.HostedMasterProfile.DNSPrefix = v
			c.ContainerService.Properties.HostedMasterProfile.FQDNSubdomain = ""
		}, want: orDefault("localcluster")},
		{action: `GetVariable "location"`, prefix: `$Location=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Location = v
		}},
		{action: `GetVariable "userAssignedIdentityID"`, prefix: `$UserAssignedClientID=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.UserAssignedIdentityClientID = v
		}},
		{action: `GetTargetEnvironment`, prefix: `$TargetEnvironment=`},
		{action: `GetArmResourceEndpoint`, prefix: `$ArmResourceEndpoint=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{ResourceManagerEndpoint: v}
		}},
		{action: `GetParameter "servicePrincipalClientId"`, prefix: `$AADClientId=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.ServicePrincipalProfile.ClientID = v
		}},
		{action: `GetSshPublicKeysPowerShell`, prefix: `$global:SSHKeys=`, kind: "first", set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: v}}
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			return strings.TrimSpace(v)
		}},
		{action: `GetParameter "caCertificate"`, prefix: `$global:CACertificate=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.CertificateProfile.CaCertificate = v
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			return base64.StdEncoding.EncodeToString([]byte(v))
		}},
		{action: `GetParameter "clientCertificate"`, prefix: `$global:AgentCertificate=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.CertificateProfile.ClientCertificate = v
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			return base64.StdEncoding.EncodeToString([]byte(v))
		}},
		{action: `GetParameter "kubeBinariesSASURL"`, prefix: `$global:KubeBinariesPackageSASURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.K8sComponents.WindowsPackageURL = v
		}},
		// Nothing sets windowsKubeBinariesURL, so the value is always empty.
		{action: `GetParameter "windowsKubeBinariesURL"`, prefix: `$global:WindowsKubeBinariesURL=`},
		{action: `GetParameter "kubeBinariesVersion"`, prefix: `$global:KubeBinariesVersion=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = v
		}},
		{action: `GetParameter "windowsContainerdURL"`, prefix: `$global:ContainerdUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).WindowsContainerdURL = v
		}},
		{action: `GetParameter "windowsSdnPluginURL"`, prefix: `$global:ContainerdSdnPluginUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).WindowsSdnPluginURL = v
		}},
		{action: `GetParameter "windowsDockerVersion"`, prefix: `$global:DockerVersion=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsDockerVersion = v
		}, want: orDefault(datamodel.KubernetesWindowsDockerVersion)},
		{action: `GetParameter "defaultContainerdWindowsSandboxIsolation"`, prefix: `$global:DefaultContainerdWindowsSandboxIsolation=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				windowsProfile(c).ContainerdWindowsRuntimes = &datamodel.ContainerdWindowsRuntimes{DefaultSandboxIsolation: v}
			}, want: orDefault(datamodel.KubernetesDefaultContainerdWindowsSandboxIsolation)},
		{action: `GetParameter "containerdWindowsRuntimeHandlers"`, prefix: `$global:ContainerdWindowsRuntimeHandlers=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				windowsProfile(c).ContainerdWindowsRuntimes = &datamodel.ContainerdWindowsRuntimes{RuntimeHandlers: []datamodel.RuntimeHandlers{{BuildNumber: v}}}
			}},
		{action: `GetParameter "windowsTelemetryGUID"`, prefix: `$global:WindowsTelemetryGUID=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			withCloudSpec(c, func(s *datamodel.KubernetesSpecConfig) { s.WindowsTelemetryGUID = v })
		}},
		{action: `GetVariable "tenantID"`, prefix: `$global:TenantId=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.TenantID = v
		}},
		{action: `GetVariable "subscriptionId"`, prefix: `$global:SubscriptionId=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.SubscriptionID = v
		}},
		{action: `GetVariable "resourceGroup"`, prefix: `$global:ResourceGroup=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ResourceGroupName = v
		}},
		{action: `GetVariable "vmType"`, prefix: `$global:VmType=`},
		{action: `GetVariable "subnetName"`, prefix: `$global:SubnetName=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.AgentPoolProfile.VnetSubnetID = customVNetSubnetID("vnet", v)
		}},
		{action: `GetVariable "nsgName"`, prefix: `$global:SecurityGroupName=`},
		{action: `GetVariable "virtualNetworkName"`, prefix: `$global:VNetName=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.AgentPoolProfile.VnetSubnetID = customVNetSubnetID(v, "subnet")
		}},
		{action: `GetVariable "routeTableName"`, prefix: `$global:RouteTableName=`},
		{action: `GetVariable "primaryAvailabilitySetName"`, prefix: `$global:PrimaryAvailabilitySetName=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.AgentPoolProfile.AvailabilityProfile = datamodel.AvailabilitySet
				c.AgentPoolProfile.Name = v
			}, want: func(c *datamodel.NodeBootstrappingConfiguration, v string) string {
				return v + "-availabilitySet-" + c.ContainerService.Properties.GetClusterID()
			}},
		{action: `GetVariable "primaryScaleSetName"`, prefix: `$global:PrimaryScaleSetName=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.PrimaryScaleSetName = v
		}},
		{action: `GetParameter "kubeClusterCidr"`, prefix: `$global:KubeClusterCIDR=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).ClusterSubnet = v
		}},
		{action: `GetParameter "kubeServiceCidr"`, prefix: `$global:KubeServiceCIDR=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).ServiceCIDR = v
		}},
		{action: `GetParameter "vnetCidr"`, prefix: `$global:VNetCIDR=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.AgentPoolProfile.VnetCidrs = []string{v}
		}},
		{action: `GetAgentKubernetesLabels .`, prefix: `$global:KubeletNodeLabels=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.AgentPoolProfile.CustomNodeLabels["hostile"] = v
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			return "agentpool=winnp,kubernetes.azure.com/agentpool=winnp,hostile=" + v + ",kubernetes.azure.com/mode=user,team=payments"
		}},
		// Arguments are sorted, so --aaa-test is the first item of the array.
		{action: `GetKubeletConfigKeyValsPsh`, prefix: `$global:KubeletConfigArgs=`, kind: "first", set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeletConfig["--aaa-test"] = v
		}, want: unescapedArg},
		{action: `GetKubeletHealthzEndpoint`, prefix: `$global:KubeletHealthzEndpoint=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeletConfig["--healthz-bind-address"] = v
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			if v == "" {
				v = "127.0.0.1"
			}
			return "http://" + net.JoinHostPort(v, "10248") + "/healthz"
		}},
		{action: `GetKubeproxyConfigKeyValsPsh`, prefix: `$global:KubeproxyConfigArgs=`, kind: "first", set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeproxyConfig = map[string]string{"--aaa-test": v}
		}, want: unescapedArg},
		{action: `GetKubeProxyFeatureGatesPsh`, prefix: `$global:KubeproxyFeatureGates=`, kind: "array",
			want: func(c *datamodel.NodeBootstrappingConfiguration, _ string) string {
				return strings.Join(c.ContainerService.Properties.GetKubeProxyFeatureGatesForWindows(), "\n")
			}},
		{action: `GetVariable "useManagedIdentityExtension"`, prefix: `$global:UseManagedIdentityExtension=`},
		{action: `GetVariable "useInstanceMetadata"`, prefix: `$global:UseInstanceMetadata=`},
		{action: `GetVariable "loadBalancerSku"`, prefix: `$global:LoadBalancerSku=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).LoadBalancerSku = v
		}},
		{action: `GetVariable "excludeMasterFromStandardLB"`, prefix: `$global:ExcludeMasterFromStandardLB=`},
		{action: `GetPrivateEgressProxyAddress`, prefix: `$global:PrivateEgressProxyAddress=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{PrivateEgress: &datamodel.PrivateEgress{Enabled: true, ProxyAddress: v}}
		}},
		{action: `GetParameter "networkPlugin"`, prefix: `$global:NetworkPlugin=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).NetworkPlugin = v
		}},
		{action: `GetParameter "vnetCniWindowsPluginsURL"`, prefix: `$global:VNetCNIPluginsURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			kubernetesConfig(c).AzureCNIURLWindows = v
		}, want: func(c *datamodel.NodeBootstrappingConfiguration, v string) string {
			return orDefault(c.CloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL)(c, v)
		}},
		{action: `IsIPv6DualStackFeatureEnabled`, prefix: `$global:IsDualStackEnabled=`, kind: "boolean"},
		{action: `IsAzureCNIOverlayFeatureEnabled`, prefix: `$global:IsAzureCNIOverlayEnabled=`, kind: "boolean"},
		{action: `CiliumDataplaneEnabled`, prefix: `$global:CiliumDataplaneEnabled=`, kind: "boolean"},
		{action: `EnableIMDSRestriction`, prefix: `$global:IsIMDSRestrictionEnabled=`, kind: "boolean"},
		{action: `GetParameter "windowsCredentialProviderURL"`, prefix: `$global:CredentialProviderURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.K8sComponents.WindowsCredentialProviderURL = v
		}},
		{action: `GetVariable "windowsEnableCSIProxy"`, prefix: `$global:EnableCsiProxy=`, kind: "boolean"},
		{action: `GetVariable "windowsCSIProxyURL"`, prefix: `$global:CsiProxyUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).CSIProxyURL = v
		}},
		{action: `EnableHostsConfigAgent`, prefix: `$global:EnableHostsConfigAgent=`, kind: "boolean"},
		{action: `GetVariable "windowsCSEScriptsPackageURL"`, prefix: `$global:CSEScriptsPackageUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).CseScriptsPackageURL = v
		}},
		{action: `GetVariable "windowsGpuDriverURL"`, prefix: `$global:GpuDriverURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).GpuDriverURL = v
		}},
		{action: `GetVariable "windowsPauseImageURL"`, prefix: `$global:WindowsPauseImageURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsPauseImageURL = v
		}},
		{action: `GetVariable "alwaysPullWindowsPauseImage"`, prefix: `$global:AlwaysPullWindowsPauseImage=`, kind: "boolean"},
		{action: `GetVariable "windowsCalicoPackageURL"`, prefix: `$global:WindowsCalicoPackageURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsCalicoPackageURL = v
		}},
		{action: `GetVariable "configGPUDriverIfNeeded"`, prefix: `$global:ConfigGPUDriverIfNeeded=`, kind: "boolean"},
		{action: `GetVariable "windowsGmsaPackageUrl"`, prefix: `$global:WindowsGmsaPackageUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsGmsaPackageUrl = v
		}},
		{action: `GetTLSBootstrapTokenForKubeConfig`, prefix: `$global:TLSBootstrapToken=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeletClientTLSBootstrapToken = to.StringPtr(v)
		}},
		{action: `EnableSecureTLSBootstrapping`, prefix: `$global:EnableSecureTLSBootstrapping=`, kind: "boolean"},
		{action: `GetSecureTLSBootstrappingAADResource`, prefix: `$global:SecureTLSBootstrappingAADResource=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.AADResource = v
			}},
		{action: `GetSecureTLSBootstrappingUserAssignedIdentityID`, prefix: `$global:SecureTLSBootstrappingUserAssignedIdentityID=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.UserAssignedIdentityID = v
			}},
		{action: `GetCustomSecureTLSBootstrappingClientDownloadURL`, prefix: `$global:CustomSecureTLSBootstrappingClientDownloadURL=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.CustomClientDownloadURL = v
			}},
		{action: `GetSecureTLSBootstrappingValidateKubeconfigTimeout`, prefix: `$global:SecureTLSBootstrappingValidateKubeconfigTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.ValidateKubeconfigTimeout = v
			}},
		{action: `GetSecureTLSBootstrappingGetAccessTokenTimeout`, prefix: `$global:SecureTLSBootstrappingGetAccessTokenTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.GetAccessTokenTimeout = v
			}},
		{action: `GetSecureTLSBootstrappingGetInstanceDataTimeout`, prefix: `$global:SecureTLSBootstrappingGetInstanceDataTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.GetInstanceDataTimeout = v
			}},
		{action: `GetSecureTLSBootstrappingGetNonceTimeout`, prefix: `$global:SecureTLSBootstrappingGetNonceTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.GetNonceTimeout = v
			}},
		{action: `GetSecureTLSBootstrappingGetAttestedDataTimeout`, prefix: `$global:SecureTLSBootstrappingGetAttestedDataTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.GetAttestedDataTimeout = v
			}},
		{action: `GetSecureTLSBootstrappingGetCredentialTimeout`, prefix: `$global:SecureTLSBootstrappingGetCredentialTimeout=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.SecureTLSBootstrappingConfig.GetCredentialTimeout = v
			}},
		{action: `GetVariable "isDisableWindowsOutboundNat"`, prefix: `$global:IsDisableWindowsOutboundNat=`, kind: "boolean"},
		// The base64 of the static helper scripts.
		{action: `GetKubernetesWindowsAgentFunctions`, prefix: `$zippedFiles=`},
		{action: `FIPSEnabled`, prefix: `$fipsEnabled=`, kind: "boolean"},
		{action: `GetHnsRemediatorIntervalInMinutes`, prefix: `$global:HNSRemediatorIntervalInMinutes=`, kind: "uint32"},
		{action: `GetLogGeneratorIntervalInMinutes`, prefix: `$global:LogGeneratorIntervalInMinutes=`, kind: "uint32"},
		{action: `GetVariable "isSkipCleanupNetwork"`, prefix: `$global:IsSkipCleanupNetwork=`, kind: "boolean"},
		{action: `GetPreProvisionOnly`, prefix: `$PreProvisionOnly=`, kind: "boolean"},
		{action: `EnableKubeletServingCertificateRotation`, prefix: `$global:EnableKubeletServingCertificateRotation=`, kind: "boolean"},
		{action: `GetVariable "nextGenNetworkingEnabled"`, prefix: `$global:EnableWindowsCiliumNetworking=`, kind: "boolean"},
		{action: `GetVariable "nextGenNetworkingConfig"`, prefix: `$global:WindowsCiliumNetworkingConfiguration=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{NextGenNetworkingConfig: to.StringPtr(v)}
			}},
		{action: `GetBootstrapProfileContainerRegistryServer`, prefix: `$global:BootstrapProfileContainerRegistryServer=`,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{PrivateEgress: &datamodel.PrivateEgress{Enabled: true, ContainerRegistryServer: v}}
			}},
		{action: `GetMCRRepositoryBase`, prefix: `$global:MCRRepositoryBase=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			withCloudSpec(c, func(s *datamodel.KubernetesSpecConfig) { s.MCRKubernetesImageBase = v })
		}, want: orDefault("mcr.microsoft.com")},
		{action: `GetNetworkIsolatedClusterTestMode`, prefix: `$global:NetworkIsolatedClusterTestMode=`, kind: "boolean"},
		{action: `WindowsSSHEnabled`, prefix: `$sshEnabled=`, kind: "boolean"},
		{action: `AKSCustomCloudContainerRegistryDNSSuffix`, prefix: `-CustomCloudContainerRegistryDNSSuffix `,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom", ContainerRegistryDNSSuffix: v}
			},
			stub: `function Install-CredentialProvider { param($KubeDir, $CustomCloudContainerRegistryDNSSuffix) ` +
				`$global:CustomCloudContainerRegistryDNSSuffix = $CustomCloudContainerRegistryDNSSuffix }; `},
		// The base64 of the custom cloud environment JSON.
		{action: `GetBase64EncodedEnvironmentJSON`, prefix: `$envJSON=`, setup: func(c *datamodel.NodeBootstrappingConfiguration) {
			c.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
		}},
		{action: `GetIdentitySystem`, prefix: `-IdentitySystem `,
			stub: `function Set-AzureConfig { param($IdentitySystem) $global:IdentitySystem = $IdentitySystem }; Set-AzureConfig `},
	}
}

func sortedWindowsCSETestValueNames() []string {
	names := make([]string, 0, len(windowsCSETestValues))
	for name := range windowsCSETestValues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func renderWindowsCSE(t *testing.T, config *datamodel.NodeBootstrappingConfiguration) (string, string) {
	t.Helper()
	validateAndSetWindowsNodeBootstrappingConfiguration(config)
	templateGenerator := InitializeTemplateGenerator()
	customData, err := base64.StdEncoding.DecodeString(templateGenerator.getWindowsNodeBootstrappingPayload(config))
	require.NoError(t, err)
	return string(customData), templateGenerator.getWindowsNodeCSECommand(config)
}

// renderedWindowsCSEField returns the real rendered statement and its expected runtime value.
func renderedWindowsCSEField(t *testing.T, field windowsCSEField, value string) (string, string) {
	t.Helper()
	config := newWindowsBootstrapTestConfig()
	if field.setup != nil {
		field.setup(config)
	}
	if field.set != nil {
		field.set(config, value)
	}
	customData, _ := renderWindowsCSE(t, config)
	want := value
	if field.set == nil {
		// Read constrained values from their source getter, before any PowerShell encoding.
		funcMap := getBakerFuncMap(config, getParameters(config), getWindowsCustomDataVariables(config))
		source, err := template.New("input").Funcs(funcMap).Parse("{{" + field.action + "}}")
		require.NoError(t, err)
		var raw strings.Builder
		require.NoError(t, source.Execute(&raw, config.AgentPoolProfile))
		want = raw.String()
	}
	if field.want != nil {
		want = field.want(config, value)
	}
	return field.stub + windowsCSEFieldStatement(t, customData, field.prefix), want
}

func windowsCSEFieldStatement(t *testing.T, customData, prefix string) string {
	t.Helper()
	index := strings.Index(customData, prefix)
	require.GreaterOrEqual(t, index, 0, "assignment or argument %s was not rendered", prefix)
	start := strings.LastIndex(customData[:index], "\n") + 1
	end := strings.Index(customData[index:], "\n")
	require.GreaterOrEqual(t, end, 0, "assignment or argument %s has no terminating newline", prefix)
	return strings.TrimSpace(customData[start : index+end])
}

// TestWindowsCSEFieldsFixture covers every input; PowerShell checks actual values and types rather than
// comparing the rendered text with the production encoder.
// Set GENERATE_TEST_DATA=true to rewrite the fixture.
func TestWindowsCSEFieldsFixture(t *testing.T) {
	var lines []string
	add := func(field windowsCSEField, value string) {
		line, expected := renderedWindowsCSEField(t, field, value)
		name := strings.TrimPrefix(strings.SplitN(field.prefix, "=", 2)[0], "$")
		name = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(name, "global:"), "-"))
		kind := field.kind
		if kind == "" {
			kind = "string"
		}
		if kind == "boolean" {
			boolean, err := strconv.ParseBool(expected)
			require.NoError(t, err)
			expected = "False"
			if boolean {
				expected = "True"
			}
		}
		lines = append(lines, strings.Join([]string{name, kind, base64.StdEncoding.EncodeToString([]byte(expected)), line}, "\t"))
	}
	for _, field := range windowsCSEFields() {
		require.True(t, strings.HasPrefix(field.prefix, "$") || field.stub != "", "%s needs a stub so the Pester test can run it", field.action)
		if field.set == nil {
			add(field, "")
			continue
		}
		for _, name := range sortedWindowsCSETestValueNames() {
			add(field, windowsCSETestValues[name])
		}
		if field.action == `GetVariable "windowsCSEScriptsPackageURL"` {
			add(field, windowsCSEPackageURLWithDollar)
		}
	}
	got := strings.Join(lines, "\n") + "\n"
	requireASCII(t, "fixture", got)

	if os.Getenv("GENERATE_TEST_DATA") == "true" {
		require.NoError(t, os.MkdirAll(filepath.Dir(windowsCSEFieldsFixture), 0o755))
		require.NoError(t, os.WriteFile(windowsCSEFieldsFixture, []byte(got), 0o600))
	}
	want, err := os.ReadFile(windowsCSEFieldsFixture)
	require.NoError(t, err, "regenerate %s with: make generate-testdata", windowsCSEFieldsFixture)
	require.Equal(t, string(want), got, "%s is stale; regenerate it with: make generate-testdata", windowsCSEFieldsFixture)
}

func TestWindowsCSEScriptIsASCII(t *testing.T) {
	// Windows PowerShell 5.1 reads the script with the ANSI code page, so the bytes of a non-ASCII
	// character could be read as a quote. Values with non-ASCII characters must be base64-encoded.
	config := newWindowsBootstrapTestConfig()
	for _, field := range windowsCSEFields() {
		if field.set != nil {
			field.set(config, windowsCSETestValues["unicode"])
		}
	}
	config.ContainerService.Properties.CertificateProfile.ClientPrivateKey = windowsCSETestValues["unicode"]
	config.ContainerService.Properties.ServicePrincipalProfile.Secret = windowsCSETestValues["unicode"]
	customData, cse := renderWindowsCSE(t, config)
	requireASCII(t, "CustomData", customData)
	requireASCII(t, "CSE command", cse)
}

func requireASCII(t *testing.T, name, s string) {
	t.Helper()
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			t.Fatalf("%s has a non-ASCII byte 0x%x on line %d", name, s[i], 1+strings.Count(s[:i], "\n"))
		}
	}
}

func parseWindowsTemplateActions(t *testing.T, text string) []*parse.PipeNode {
	t.Helper()
	funcMap := getBakerFuncMap(newWindowsBootstrapTestConfig(), paramsMap{}, paramsMap{})
	tmpl, err := template.New("windows").Funcs(funcMap).Parse(text)
	require.NoError(t, err)

	var actions []*parse.PipeNode
	var walk func(parse.Node)
	walk = func(node parse.Node) {
		switch n := node.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, child := range n.Nodes {
				walk(child)
			}
		case *parse.IfNode:
			// An inline conditional assignment is an input too, even though it has no {{value}} action.
			lineStart := strings.LastIndex(text[:n.Pos], "\n") + 1
			prefix := strings.TrimSpace(text[lineStart:n.Pos])
			if strings.HasPrefix(prefix, "$") && strings.Contains(prefix, "=") {
				actions = append(actions, n.Pipe)
			}
			walk(n.List)
			walk(n.ElseList)
		case *parse.ActionNode:
			actions = append(actions, n.Pipe)
		case *parse.TextNode, *parse.CommentNode:
		default:
			t.Fatalf("unexpected template node %q", node.String())
		}
	}
	walk(tmpl.Root)
	return actions
}

func windowsTemplateInputs(t *testing.T, text string) []string {
	t.Helper()
	var inputs []string
	var collect func(*parse.PipeNode)
	collect = func(pipe *parse.PipeNode) {
		inputs = append(inputs, pipe.Cmds[0].String())
		for _, cmd := range pipe.Cmds {
			for _, arg := range cmd.Args {
				if nested, ok := arg.(*parse.PipeNode); ok {
					collect(nested)
				}
			}
		}
	}
	for _, action := range parseWindowsTemplateActions(t, text) {
		collect(action)
	}
	return inputs
}

// Every rendered input needs a test row, regardless of how the template encodes it.
func TestWindowsCSETemplateInputsHaveTests(t *testing.T) {
	text, err := parts.Templates.ReadFile(kubernetesWindowsAgentCustomDataPS1)
	require.NoError(t, err)
	var listed []string
	for _, field := range windowsCSEFields() {
		listed = append(listed, field.action)
	}
	for name, source := range map[string]string{
		"current template": string(text),
		"without encoder":  strings.ReplaceAll(string(text), " | PowerShellLiteral", ""),
	} {
		t.Run(name, func(t *testing.T) {
			require.ElementsMatch(t, listed, windowsTemplateInputs(t, source),
				"every input in %s must have a row in windowsCSEFields", kubernetesWindowsAgentCustomDataPS1)
		})
	}
}

// Encoding changes must not alter the input inventory, but new inputs and conditional branches must.
func TestWindowsTemplateInputs(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"plain", `$x="{{GetVariable "tenantID"}}"`, []string{`GetVariable "tenantID"`}},
		{"literal", `$x={{GetVariable "tenantID" | PowerShellLiteral}}`, []string{`GetVariable "tenantID"`}},
		{"other encoding", `$x={{GetVariable "tenantID" | printf "%q"}}`, []string{`GetVariable "tenantID"`}},
		{"list", `$x=@( {{GetKubeletConfigKeyValsPsh}} )`, []string{`GetKubeletConfigKeyValsPsh`}},
		{"new input", `{{GetVariable "tenantID"}} {{GetParameter "newInput"}}`,
			[]string{`GetVariable "tenantID"`, `GetParameter "newInput"`}},
		{"nested input", `{{GetVariable "tenantID" | printf "%s%s" (GetParameter "newInput")}}`,
			[]string{`GetVariable "tenantID"`, `GetParameter "newInput"`}},
		{"branches", `{{if UserAssignedIDEnabled}}{{GetVariable "userAssignedIdentityID"}}{{else}}{{GetVariable "tenantID"}}{{end}}`,
			[]string{`GetVariable "userAssignedIdentityID"`, `GetVariable "tenantID"`}},
		{"boolean assignment", `$global:IsDualStackEnabled={{if IsIPv6DualStackFeatureEnabled}}$true{{else}}$false{{end}}`,
			[]string{`IsIPv6DualStackFeatureEnabled`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, windowsTemplateInputs(t, tt.text))
		})
	}
}

// Check real script syntax and input placement, without prescribing a string-literal representation.
const windowsCSEParseCheck = `param([string] $Path)
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Path, [ref] $tokens, [ref] $errors)
if ($errors.Count -gt 0) {
    $errors | ForEach-Object { $_.Message }
    exit 1
}
foreach ($node in $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.AssignmentStatementAst] }, $true)) {
    $node.Left.Extent.Text
}
foreach ($node in $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.CommandParameterAst] }, $true)) {
    '-' + $node.ParameterName
}
`

// Parse real inputs in the complete script so comments and here-strings cannot hide tested assignments.
func TestWindowsCSEScriptParses(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is not installed")
	}
	directory := t.TempDir()
	checkPath := filepath.Join(directory, "check.ps1")
	require.NoError(t, os.WriteFile(checkPath, []byte(windowsCSEParseCheck), 0o600))

	for _, name := range sortedWindowsCSETestValueNames() {
		t.Run(name, func(t *testing.T) {
			config := newWindowsBootstrapTestConfig()
			for _, field := range windowsCSEFields() {
				if field.set != nil {
					field.set(config, windowsCSETestValues[name])
				}
			}
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
			script, _ := renderWindowsCSE(t, config)
			scriptPath := filepath.Join(directory, name+".ps1")
			require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))
			output, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", checkPath, scriptPath).CombinedOutput()
			require.NoError(t, err, string(output))
			got := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
			for _, field := range windowsCSEFields() {
				target := strings.TrimSpace(strings.SplitN(field.prefix, "=", 2)[0])
				require.Contains(t, got, target, "%s must be an assignment or command argument, not commented-out text", field.action)
			}
		})
	}
}

func TestWindowsCSECommandRendersOnlyBase64Values(t *testing.T) {
	text, err := parts.Templates.ReadFile(kubernetesWindowsAgentCSECommandPS1)
	require.NoError(t, err)
	var pipes []string
	for _, action := range parseWindowsTemplateActions(t, string(text)) {
		pipes = append(pipes, action.String())
	}
	require.Equal(t, []string{`GetParameter "clientPrivateKey"`, `GetParameter "encodedServicePrincipalClientSecret"`}, pipes)

	config := newWindowsBootstrapTestConfig()
	config.ContainerService.Properties.CertificateProfile.ClientPrivateKey = windowsCSETestValues["code"] + windowsCSETestValues["quotes"]
	config.ContainerService.Properties.ServicePrincipalProfile.Secret = windowsCSETestValues["code"] + windowsCSETestValues["quotes"]
	_, cse := renderWindowsCSE(t, config)
	for _, parameter := range []string{"AgentKey", "AADClientSecret"} {
		match := regexp.MustCompile(`-` + parameter + ` ''([^']*)''`).FindStringSubmatch(cse)
		require.NotNil(t, match, "-%s not found in the CSE command", parameter)
		require.Regexp(t, `^[A-Za-z0-9+/]*={0,2}$`, match[1], "-%s must be base64", parameter)
	}
}

func TestPowerShellLiteral(t *testing.T) {
	tests := []struct {
		value, want string
	}{
		{"", `''`},
		{"eastus", `'eastus'`},
		{`it's`, `'it''s'`},
		{`$(Get-Date) "x" ` + "`n", `'$(Get-Date) "x" ` + "`n'"},
		{"c:\\k\\azure.json", `'c:\k\azure.json'`},
		{"line1\nline2", "(" + encodePowerShellBase64Literal("line1\nline2") + ")"},
		{"tab\t", "(" + encodePowerShellBase64Literal("tab\t") + ")"},
		{"it\u2019s", "(" + encodePowerShellBase64Literal("it\u2019s") + ")"},
		{"a\u2013b", "(" + encodePowerShellBase64Literal("a\u2013b") + ")"},
		{"caf\u00e9", "(" + encodePowerShellBase64Literal("caf\u00e9") + ")"},
		{"\x7f", "(" + encodePowerShellBase64Literal("\x7f") + ")"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, powerShellLiteral(tt.value), "value %q", tt.value)
	}
	require.Equal(t, `'a', 'b''c'`, powerShellLiteralList([]string{"a", "b'c"}))
	require.Empty(t, powerShellLiteralList(nil))
}

func TestUnescapePowerShellDoubleQuotes(t *testing.T) {
	require.Equal(t,
		[]string{`--resolv-conf=""`, `--a="b"`, "--c=$(d)`e", `--f=""`, ""},
		unescapePowerShellDoubleQuotes([]string{`--resolv-conf=""""`, `--a=""b""`, "--c=$(d)`e", `--f="""`, ""}))
}

// RP relies on "" becoming "; check the resulting arguments rather than the script's quote style.
func TestWindowsKubeletArgumentsKeepTheirValues(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is not installed")
	}
	config := newWindowsBootstrapTestConfig()
	customData, _ := renderWindowsCSE(t, config)
	script := windowsCSEFieldStatement(t, customData, "$global:KubeletConfigArgs=") + "\n" +
		windowsCSEFieldStatement(t, customData, "$global:KubeproxyFeatureGates=") + "\n" +
		`$global:KubeletConfigArgs; $global:KubeproxyFeatureGates`
	path := filepath.Join(t.TempDir(), "arguments.ps1")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o600))
	output, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", path).CombinedOutput()
	require.NoError(t, err, string(output))
	args := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
	require.Contains(t, args, `--enforce-node-allocatable=""`)
	require.Contains(t, args, `--resolv-conf=""`)
	require.Contains(t, args, `--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`)
	require.Contains(t, args, "WinDSR=true")
	require.Contains(t, args, "WinOverlay=false")
}

// Azure Blob URLs can name the $web container; PowerShell must not expand it as a variable.
func TestWindowsCSEPreservesPackageURL(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is not installed")
	}
	config := newWindowsBootstrapTestConfig()
	config.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = windowsCSEPackageURLWithDollar
	customData, _ := renderWindowsCSE(t, config)
	script := "$web = $null\n" +
		windowsCSEFieldStatement(t, customData, "$global:CSEScriptsPackageUrl=") + "\n" +
		"$global:CSEScriptsPackageUrl"
	path := filepath.Join(t.TempDir(), "package-url.ps1")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o600))
	output, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", path).CombinedOutput()
	require.NoError(t, err, string(output))
	require.Equal(t, windowsCSEPackageURLWithDollar, strings.TrimSpace(string(output)))
}

func newWindowsBootstrapTestConfig() *datamodel.NodeBootstrappingConfiguration {
	const kubernetesVersion = "1.33.2"
	const kubeletIdentity = "11111111-2222-3333-4444-555555555555"
	profile := &datamodel.AgentPoolProfile{
		Name:                "winnp",
		VMSize:              "Standard_D4s_v3",
		OSType:              datamodel.Windows,
		Distro:              datamodel.AKSWindows2022ContainerdGen2,
		AvailabilityProfile: datamodel.VirtualMachineScaleSets,
		StorageProfile:      "ManagedDisks",
		CustomNodeLabels: map[string]string{
			"kubernetes.azure.com/mode": "user",
			"team":                      "payments",
		},
		KubernetesConfig:     &datamodel.KubernetesConfig{ContainerRuntime: "containerd"},
		VnetCidrs:            []string{"10.224.0.0/12"},
		NotRebootWindowsNode: to.BoolPtr(true),
		CustomKubeletConfig:  &datamodel.CustomKubeletConfig{ImageGcHighThreshold: to.Int32Ptr(90)},
	}
	linuxProfile := &datamodel.LinuxProfile{AdminUsername: "azureuser"}
	linuxProfile.SSH.PublicKeys = []datamodel.PublicKey{
		{KeyData: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7 first@example"},
		{KeyData: " ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB second@example "},
	}
	return &datamodel.NodeBootstrappingConfiguration{
		TenantID:                     "72f988bf-86f1-41af-91ab-2d7cd011db47",
		SubscriptionID:               "00000000-0000-0000-0000-000000000001",
		ResourceGroupName:            "MC_rg_cluster_eastus",
		UserAssignedIdentityClientID: kubeletIdentity,
		PrimaryScaleSetName:          "akswinnp",
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN:      "cluster-dns-abc123.hcp.eastus.azmk8s.io",
					DNSPrefix: "cluster-dns",
				},
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate:     "-----BEGIN CERTIFICATE-----\nMIIBfakeCA\n-----END CERTIFICATE-----\n",
					ClientCertificate: "-----BEGIN CERTIFICATE-----\nMIIBfakeClient\n-----END CERTIFICATE-----\n",
					ClientPrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEfakeKey\n-----END RSA PRIVATE KEY-----\n",
				},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: kubernetesVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						AzureCNIURLWindows:     "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-windows-amd64-v1.6.21.zip",
						ClusterSubnet:          "10.244.0.0/16",
						DNSServiceIP:           "10.0.0.10",
						ServiceCIDR:            "10.0.0.0/16",
						LoadBalancerSku:        "Standard",
						NetworkPlugin:          "azure",
						NetworkPluginMode:      "overlay",
						UseInstanceMetadata:    to.BoolPtr(true),
						UseManagedIdentity:     true,
						UserAssignedID:         kubeletIdentity,
						WindowsContainerdURL:   "https://packages.aks.azure.com/containerd/windows/",
						ContainerRuntimeConfig: map[string]string{},
					},
				},
				AgentPoolProfiles:       []*datamodel.AgentPoolProfile{profile},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{ClientID: "msi", Secret: "msi"},
				FeatureFlags:            &datamodel.FeatureFlags{EnableWinDSR: true},
				WindowsProfile: &datamodel.WindowsProfile{
					AlwaysPullWindowsPauseImage: to.BoolPtr(false),
					CSIProxyURL:                 "https://packages.aks.azure.com/csi-proxy/v1.1.2-hotfix.20230807/binaries/csi-proxy-v1.1.2-hotfix.20230807.tar.gz",
					EnableCSIProxy:              to.BoolPtr(true),
					SSHEnabled:                  to.BoolPtr(true),
					WindowsPauseImageURL:        "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
					CseScriptsPackageURL:        "https://packages.aks.azure.com/aks/windows/cse/",
					WindowsGmsaPackageUrl:       "https://packages.aks.azure.com/windows/gmsa/windows-gmsa-ccgakvplugin-v1.1.9.zip",
				},
				LinuxProfile: linuxProfile,
			},
		},
		CloudSpecConfig: datamodel.AzurePublicCloudSpecForTest,
		K8sComponents: &datamodel.K8sComponents{
			WindowsPackageURL:            "https://packages.aks.azure.com/kubernetes/v1.33.2/windowszip/v1.33.2-1int.zip",
			WindowsCredentialProviderURL: "https://packages.aks.azure.com/cloud-provider-azure/v1.33.2/binaries/azure-acr-credential-provider-windows-amd64-v1.33.2.tar.gz",
		},
		AgentPoolProfile: profile,
		KubeletConfig: map[string]string{
			"--azure-container-registry-config": "c:\\k\\azure.json",
			"--bootstrap-kubeconfig":            "c:\\k\\bootstrap-config",
			"--cert-dir":                        "c:\\k\\pki",
			"--cgroups-per-qos":                 "false",
			"--client-ca-file":                  "c:\\k\\ca.crt",
			"--cloud-provider":                  "external",
			"--cluster-dns":                     "10.0.0.10",
			"--cluster-domain":                  "cluster.local",
			// RP sends four double quotes, so the legacy script's double-quoted string yields "" at runtime.
			"--enforce-node-allocatable":     `""""`,
			"--eviction-hard":                `""""`,
			"--feature-gates":                "RotateKubeletServerCertificate=true",
			"--hairpin-mode":                 "promiscuous-bridge",
			"--kube-reserved":                "cpu=100m,memory=3891Mi",
			"--kubeconfig":                   "c:\\k\\config",
			"--max-pods":                     "30",
			"--node-status-update-frequency": "10s",
			"--resolv-conf":                  `""""`,
			"--rotate-certificates":          "true",
			"--rotate-server-certificates":   "true",
			"--tls-cipher-suites":            "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		},
		KubeletClientTLSBootstrapToken: to.StringPtr("07401b.f395accd246ae52d"),
		SecureTLSBootstrappingConfig: &datamodel.SecureTLSBootstrappingConfig{
			Enabled:                   true,
			AADResource:               "6dae42f8-4368-4678-94ff-3960e28e3630",
			ValidateKubeconfigTimeout: "30s",
			GetCredentialTimeout:      "1m",
		},
	}
}
