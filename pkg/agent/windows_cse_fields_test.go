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
// (parts/windows/kuberneteswindowssetup.ps1.template): every value must reach PowerShell as a plain string
// that PowerShell does not expand or run.

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

// windowsCSEField is one value that AgentBaker writes into the Windows CSE script.
type windowsCSEField struct {
	// action is the template pipeline that produces the value, without "| PowerShellLiteral".
	action string
	// prefix is the text right before the value in the rendered script.
	prefix string
	// set puts value into the NodeBootstrappingConfiguration field that the value comes from. It is nil
	// when the value is never free text: a boolean, a number, base64, or a name that AgentBaker builds.
	set func(c *datamodel.NodeBootstrappingConfiguration, value string)
	// want returns the value that the script must get. nil means the value itself.
	want func(c *datamodel.NodeBootstrappingConfiguration, value string) string
	// encode returns how the value is written. nil means powerShellLiteral.
	encode func(value string) string
	// setup turns on the template branch that writes a value whose set is nil.
	setup func(c *datamodel.NodeBootstrappingConfiguration)
	// stub defines the command that a value is an argument of, so the Pester test can run the line. The
	// stub must store the argument in a global variable named like the parameter.
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
// TestWindowsCSETemplateWritesEveryValueAsLiteral fails if a template value is missing here.
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
		{action: `GetSshPublicKeysPowerShell`, prefix: `$global:SSHKeys=@( `, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: v}}
		}, want: func(_ *datamodel.NodeBootstrappingConfiguration, v string) string {
			return strings.TrimSpace(v)
		}, encode: encodePowerShellBase64Literal},
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
		{action: `GetKubeletConfigKeyValsPsh`, prefix: `$global:KubeletConfigArgs=@( `, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
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
		{action: `GetKubeproxyConfigKeyValsPsh`, prefix: `$global:KubeproxyConfigArgs=@( `, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeproxyConfig = map[string]string{"--aaa-test": v}
		}, want: unescapedArg},
		{action: `GetKubeProxyFeatureGatesPsh`, prefix: `$global:KubeproxyFeatureGates=@( `},
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
		{action: `GetParameter "windowsCredentialProviderURL"`, prefix: `$global:CredentialProviderURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.K8sComponents.WindowsCredentialProviderURL = v
		}},
		{action: `GetVariable "windowsEnableCSIProxy"`, prefix: `$global:EnableCsiProxy=[System.Convert]::ToBoolean(`},
		{action: `GetVariable "windowsCSIProxyURL"`, prefix: `$global:CsiProxyUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).CSIProxyURL = v
		}},
		{action: `EnableHostsConfigAgent`, prefix: `$global:EnableHostsConfigAgent=[System.Convert]::ToBoolean(`},
		{action: `GetVariable "windowsCSEScriptsPackageURL"`, prefix: `$global:CSEScriptsPackageUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).CseScriptsPackageURL = v
		}},
		{action: `GetVariable "windowsGpuDriverURL"`, prefix: `$global:GpuDriverURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).GpuDriverURL = v
		}},
		{action: `GetVariable "windowsPauseImageURL"`, prefix: `$global:WindowsPauseImageURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsPauseImageURL = v
		}},
		{action: `GetVariable "alwaysPullWindowsPauseImage"`, prefix: `$global:AlwaysPullWindowsPauseImage=[System.Convert]::ToBoolean(`},
		{action: `GetVariable "windowsCalicoPackageURL"`, prefix: `$global:WindowsCalicoPackageURL=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsCalicoPackageURL = v
		}},
		{action: `GetVariable "configGPUDriverIfNeeded"`, prefix: `$global:ConfigGPUDriverIfNeeded=[System.Convert]::ToBoolean(`},
		{action: `GetVariable "windowsGmsaPackageUrl"`, prefix: `$global:WindowsGmsaPackageUrl=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			windowsProfile(c).WindowsGmsaPackageUrl = v
		}},
		{action: `GetTLSBootstrapTokenForKubeConfig`, prefix: `$global:TLSBootstrapToken=`, set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
			c.KubeletClientTLSBootstrapToken = to.StringPtr(v)
		}},
		{action: `EnableSecureTLSBootstrapping`, prefix: `$global:EnableSecureTLSBootstrapping=[System.Convert]::ToBoolean(`},
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
		{action: `GetVariable "isDisableWindowsOutboundNat"`, prefix: `$global:IsDisableWindowsOutboundNat=[System.Convert]::ToBoolean(`},
		// The base64 of the static helper scripts.
		{action: `GetKubernetesWindowsAgentFunctions`, prefix: `$zippedFiles=`},
		{action: `FIPSEnabled`, prefix: `$fipsEnabled=[System.Convert]::ToBoolean(`},
		{action: `GetHnsRemediatorIntervalInMinutes`, prefix: `$global:HNSRemediatorIntervalInMinutes=[System.Convert]::ToUInt32(`},
		{action: `GetLogGeneratorIntervalInMinutes`, prefix: `$global:LogGeneratorIntervalInMinutes=[System.Convert]::ToUInt32(`},
		{action: `GetVariable "isSkipCleanupNetwork"`, prefix: `$global:IsSkipCleanupNetwork=[System.Convert]::ToBoolean(`},
		{action: `GetPreProvisionOnly`, prefix: `$PreProvisionOnly=[System.Convert]::ToBoolean(`},
		{action: `EnableKubeletServingCertificateRotation`, prefix: `$global:EnableKubeletServingCertificateRotation=[System.Convert]::ToBoolean(`},
		{action: `GetVariable "nextGenNetworkingEnabled"`, prefix: `$global:EnableWindowsCiliumNetworking=[System.Convert]::ToBoolean(`},
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
		{action: `GetNetworkIsolatedClusterTestMode`, prefix: `$global:NetworkIsolatedClusterTestMode=[System.Convert]::ToBoolean(`},
		{action: `WindowsSSHEnabled`, prefix: `$sshEnabled=[System.Convert]::ToBoolean(`},
		{action: `AKSCustomCloudContainerRegistryDNSSuffix`, prefix: `-CustomCloudContainerRegistryDNSSuffix `,
			set: func(c *datamodel.NodeBootstrappingConfiguration, v string) {
				c.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom", ContainerRegistryDNSSuffix: v}
			},
			stub: `function Install-CredentialProvider { param($KubeDir, $CustomCloudContainerRegistryDNSSuffix) ` +
				`$global:CustomCloudContainerRegistryDNSSuffix = $CustomCloudContainerRegistryDNSSuffix }`},
		// The base64 of the custom cloud environment JSON.
		{action: `GetBase64EncodedEnvironmentJSON`, prefix: `$envJSON=`, setup: func(c *datamodel.NodeBootstrappingConfiguration) {
			c.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
		}},
		{action: `GetIdentitySystem`, prefix: `-IdentitySystem `},
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

// renderedWindowsCSEField renders the script with value in field and returns the text that must be in it.
func renderedWindowsCSEField(t *testing.T, field windowsCSEField, value string) (string, string) {
	t.Helper()
	config := newWindowsBootstrapTestConfig()
	field.set(config, value)
	customData, _ := renderWindowsCSE(t, config)
	want := value
	if field.want != nil {
		want = field.want(config, value)
	}
	encode := powerShellLiteral
	if field.encode != nil {
		encode = field.encode
	}
	return customData, field.prefix + encode(want)
}

func TestWindowsCSEFieldsAreWrittenAsLiterals(t *testing.T) {
	for _, field := range windowsCSEFields() {
		t.Run(field.action, func(t *testing.T) {
			if field.set == nil {
				// The value is a boolean, a number, base64, or a name that AgentBaker builds: a plain literal.
				config := newWindowsBootstrapTestConfig()
				if field.setup != nil {
					field.setup(config)
				}
				customData, _ := renderWindowsCSE(t, config)
				require.Regexp(t, regexp.QuoteMeta(field.prefix)+`'[A-Za-z0-9+/=._:-]*'`, customData)
				return
			}
			for _, name := range sortedWindowsCSETestValueNames() {
				customData, want := renderedWindowsCSEField(t, field, windowsCSETestValues[name])
				require.Contains(t, customData, want, "value %q", name)
			}
		})
	}
}

// TestWindowsCSEFieldsFixture writes the rendered line of each text field, which
// parts/windows/kuberneteswindowssetup.fields.tests.ps1 runs in PowerShell to check the value it gets.
// Set GENERATE_TEST_DATA=true to rewrite the fixture.
func TestWindowsCSEFieldsFixture(t *testing.T) {
	var lines []string
	add := func(field windowsCSEField, value string) {
		customData, want := renderedWindowsCSEField(t, field, value)
		index := strings.Index(customData, want)
		require.GreaterOrEqual(t, index, 0)
		start := strings.LastIndex(customData[:index], "\n") + 1
		end := index + strings.Index(customData[index:], "\n")
		name := strings.TrimPrefix(strings.SplitN(field.prefix, "=", 2)[0], "$")
		name = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(name, "global:"), "-"))
		kind := "string"
		if strings.HasSuffix(field.prefix, "@( ") {
			kind = "first"
		}
		expected := value
		if field.want != nil {
			expected = field.want(newWindowsBootstrapTestConfig(), value)
		}
		line := strings.TrimSpace(customData[start:end])
		if field.stub != "" {
			line = field.stub + "; " + line
		}
		lines = append(lines, strings.Join([]string{name, kind, base64.StdEncoding.EncodeToString([]byte(expected)), line}, "\t"))
	}
	for _, field := range windowsCSEFields() {
		if field.set == nil {
			continue
		}
		require.True(t, strings.HasPrefix(field.prefix, "$") || field.stub != "", "%s needs a stub so the Pester test can run it", field.action)
		if field.action == `GetVariable "tenantID"` {
			for _, name := range sortedWindowsCSETestValueNames() {
				add(field, windowsCSETestValues[name])
			}
			continue
		}
		add(field, windowsCSETestValues["code"])
		add(field, windowsCSETestValues["unicode"])
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

// windowsTemplateAction is a {{ }} action in a Windows template.
type windowsTemplateAction struct {
	pipe  *parse.PipeNode
	start int // offset of "{{"
	end   int // offset after "}}"
}

func parseWindowsTemplateActions(t *testing.T, templatePath string) (string, []windowsTemplateAction) {
	t.Helper()
	b, err := parts.Templates.ReadFile(templatePath)
	require.NoError(t, err)
	text := string(b)
	funcMap := getBakerFuncMap(newWindowsBootstrapTestConfig(), paramsMap{}, paramsMap{})
	tmpl, err := template.New(templatePath).Funcs(funcMap).Parse(text)
	require.NoError(t, err)

	var actions []windowsTemplateAction
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
			walk(n.List)
			walk(n.ElseList)
		case *parse.ActionNode:
			start := strings.LastIndex(text[:n.Pos], "{{")
			end := int(n.Pos) + strings.Index(text[n.Pos:], "}}") + len("}}")
			actions = append(actions, windowsTemplateAction{pipe: n.Pipe, start: start, end: end})
		case *parse.TextNode, *parse.CommentNode:
		default:
			t.Fatalf("unexpected template node %q in %s", node.String(), templatePath)
		}
	}
	walk(tmpl.Root)
	return text, actions
}

// windowsTemplateCommentsAndHereStrings returns the spans of block comments and here-strings, where a
// PowerShell literal would not be read as a literal.
func windowsTemplateCommentsAndHereStrings(text string) [][2]int {
	var spans [][2]int
	for _, delimiters := range [][2]string{{"<#", "#>"}, {"@\"\n", "\n\"@"}, {"@'\n", "\n'@"}} {
		for offset := 0; ; {
			start := strings.Index(text[offset:], delimiters[0])
			if start < 0 {
				break
			}
			start += offset
			end := strings.Index(text[start:], delimiters[1])
			if end < 0 {
				end = len(text) - start
			}
			spans = append(spans, [2]int{start, start + end})
			offset = start + end
		}
	}
	return spans
}

func TestWindowsCSETemplateWritesEveryValueAsLiteral(t *testing.T) {
	text, actions := parseWindowsTemplateActions(t, kubernetesWindowsAgentCustomDataPS1)
	// These functions return PowerShell arrays of literals and have their own tests.
	listFunctions := map[string]bool{
		"GetSshPublicKeysPowerShell":   true,
		"GetKubeletConfigKeyValsPsh":   true,
		"GetKubeproxyConfigKeyValsPsh": true,
		"GetKubeProxyFeatureGatesPsh":  true,
	}
	nonCode := windowsTemplateCommentsAndHereStrings(text)

	var found []string
	for _, action := range actions {
		line := 1 + strings.Count(text[:action.start], "\n")
		lineStart := strings.LastIndex(text[:action.start], "\n") + 1
		cmds := action.pipe.Cmds
		last := cmds[len(cmds)-1].String()
		switch {
		case last == "PowerShellLiteral" && len(cmds) > 1:
			names := make([]string, 0, len(cmds)-1)
			for _, cmd := range cmds[:len(cmds)-1] {
				names = append(names, cmd.String())
			}
			found = append(found, strings.Join(names, " | "))
		case len(cmds) == 1 && listFunctions[last]:
			found = append(found, last)
		default:
			t.Errorf("line %d: {{%s}} must end with | PowerShellLiteral", line, action.pipe)
			continue
		}
		if strings.ContainsAny(text[action.start-1:action.start], `"'`) || strings.ContainsAny(text[action.end:action.end+1], `"'`) {
			t.Errorf("line %d: {{%s}} must not be inside quotes", line, action.pipe)
		}
		if strings.Contains(text[lineStart:action.start], "#") {
			t.Errorf("line %d: {{%s}} must not be in a comment", line, action.pipe)
		}
		for _, span := range nonCode {
			if action.start >= span[0] && action.start < span[1] {
				t.Errorf("line %d: {{%s}} must not be in a block comment or here-string", line, action.pipe)
			}
		}
	}

	var listed []string
	for _, field := range windowsCSEFields() {
		listed = append(listed, field.action)
	}
	require.ElementsMatch(t, listed, found, "every value in %s must have a row in windowsCSEFields", kubernetesWindowsAgentCustomDataPS1)
}

// windowsCSEParseCheck parses a rendered script with PowerShell and prints, in source order, each token
// that contains a marker and each base64 marker with the two method calls around it.
const windowsCSEParseCheck = `param([string] $Path)
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Path, [ref] $tokens, [ref] $errors)
foreach ($parseError in $errors) { "error` + "`" + `t" + $parseError.Message }
foreach ($token in $tokens) {
    if ($token.Text -match 'AKSVALUE') { "token` + "`" + `t{0}` + "`" + `t{1}" -f $token.Kind, $token.Text }
}
$isBase64Marker = { param($node) $node -is [System.Management.Automation.Language.StringConstantExpressionAst] -and $node.Value -match '^QUtTVkFM' }
foreach ($node in $ast.FindAll($isBase64Marker, $true)) {
    $value = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($node.Value)).Trim()
    "base64` + "`" + `t{0}` + "`" + `t{1}` + "`" + `t{2}" -f $value, $node.Parent.Member, $node.Parent.Parent.Member
}
`

// TestWindowsCSEValuesParseAsLiterals renders the script with a marker in place of each value and parses
// it with PowerShell. Each marker must be a single-quoted string constant, so no value is inside a
// double-quoted string, a comment, or a here-string. Each base64 marker must still be decoded by
// GetString, so the base64 form also works where a value is a command argument.
// The test needs pwsh and is skipped without it.
func TestWindowsCSEValuesParseAsLiterals(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is not installed")
	}
	_, actions := parseWindowsTemplateActions(t, kubernetesWindowsAgentCustomDataPS1)
	b, err := parts.Templates.ReadFile(kubernetesWindowsAgentCustomDataPS1)
	require.NoError(t, err)
	directory := t.TempDir()
	checkPath := filepath.Join(directory, "check.ps1")
	require.NoError(t, os.WriteFile(checkPath, []byte(windowsCSEParseCheck), 0o600))

	for _, mode := range []string{"literal", "base64"} {
		t.Run(mode, func(t *testing.T) {
			config := newWindowsBootstrapTestConfig()
			// Take every template branch that writes a value.
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
			var markers []string
			marker := func() string {
				value := fmt.Sprintf("AKSVALUE%03dX", len(markers))
				markers = append(markers, value)
				if mode == "base64" {
					return powerShellLiteral(value + "\t")
				}
				return powerShellLiteral(value)
			}
			funcMap := getBakerFuncMap(config, getParameters(config), getWindowsCustomDataVariables(config))
			funcMap["PowerShellLiteral"] = func(interface{}) string { return marker() }
			for _, name := range []string{"GetSshPublicKeysPowerShell", "GetKubeletConfigKeyValsPsh", "GetKubeproxyConfigKeyValsPsh", "GetKubeProxyFeatureGatesPsh"} {
				funcMap[name] = func() string { return marker() }
			}
			tmpl, err := template.New("script").Option("missingkey=zero").Funcs(funcMap).Parse(string(b))
			require.NoError(t, err)
			var script strings.Builder
			require.NoError(t, tmpl.Execute(&script, config.AgentPoolProfile))
			require.Len(t, markers, len(actions), "every value in the template must be rendered")

			scriptPath := filepath.Join(directory, mode+".ps1")
			require.NoError(t, os.WriteFile(scriptPath, []byte(script.String()), 0o600))
			output, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", checkPath, scriptPath).CombinedOutput()
			require.NoError(t, err, string(output))

			var want []string
			for _, value := range markers {
				if mode == "base64" {
					want = append(want, "base64\t"+value+"\tFromBase64String\tGetString")
				} else {
					want = append(want, "token\tStringLiteral\t'"+value+"'")
				}
			}
			got := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
			require.Equal(t, want, got, "each value must parse as a string literal in code, not inside a string, comment, or here-string")
		})
	}
}

func TestWindowsCSECommandRendersOnlyBase64Values(t *testing.T) {
	_, actions := parseWindowsTemplateActions(t, kubernetesWindowsAgentCSECommandPS1)
	var pipes []string
	for _, action := range actions {
		pipes = append(pipes, action.pipe.String())
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

func TestWindowsKubeletArgumentsKeepTheirValues(t *testing.T) {
	// RP relies on "" becoming " for these arguments, as it did inside PowerShell double-quoted strings.
	config := newWindowsBootstrapTestConfig()
	customData, _ := renderWindowsCSE(t, config)
	require.Contains(t, customData, `'--enforce-node-allocatable=""'`)
	require.Contains(t, customData, `'--resolv-conf=""'`)
	require.Contains(t, customData, `'--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256'`)
	require.Contains(t, customData, `$global:KubeproxyFeatureGates=@( 'WinDSR=true', 'WinOverlay=false' )`)
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
