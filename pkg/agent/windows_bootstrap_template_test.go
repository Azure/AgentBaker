package agent

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

const (
	windowsBootstrapVariablesBegin = "# ====== BEGIN AKS BOOTSTRAP VARIABLES ======"
	windowsBootstrapVariablesEnd   = "# ====== END AKS BOOTSTRAP VARIABLES ======"
	windowsBootstrapTestdataDir    = "testdata/windowsbootstrap"
)

// windowsBootstrapLegacyNodes lists, in order, every template action and condition in the variables
// block. An action renders a value directly into the Windows CSE script, and a condition chooses what
// is rendered. The list is frozen: do not add entries, because each value that is pasted into
// PowerShell code needs its own escaping review.
var windowsBootstrapLegacyNodes = []string{
	`action GetKubernetesEndpoint`,
	`action GetParameter "kubeDNSServiceIP"`,
	`action GetParameter "masterEndpointDNSNamePrefix"`,
	`action GetVariable "location"`,
	`if UserAssignedIDEnabled`,
	`action GetVariable "userAssignedIdentityID"`,
	`action GetTargetEnvironment`,
	`action GetArmResourceEndpoint`,
	`action GetParameter "servicePrincipalClientId"`,
	`action GetSshPublicKeysPowerShell`,
	`action GetParameter "caCertificate"`,
	`action GetParameter "clientCertificate"`,
	`action GetParameter "kubeBinariesSASURL"`,
	`action GetParameter "windowsKubeBinariesURL"`,
	`action GetParameter "kubeBinariesVersion"`,
	`action GetParameter "windowsContainerdURL"`,
	`action GetParameter "windowsSdnPluginURL"`,
	`action GetParameter "windowsDockerVersion"`,
	`action GetParameter "defaultContainerdWindowsSandboxIsolation"`,
	`action GetParameter "containerdWindowsRuntimeHandlers"`,
	`action GetParameter "windowsTelemetryGUID"`,
	`action GetVariable "tenantID"`,
	`action GetVariable "subscriptionId"`,
	`action GetVariable "resourceGroup"`,
	`action GetVariable "vmType"`,
	`action GetVariable "subnetName"`,
	`action GetVariable "nsgName"`,
	`action GetVariable "virtualNetworkName"`,
	`action GetVariable "routeTableName"`,
	`action GetVariable "primaryAvailabilitySetName"`,
	`action GetVariable "primaryScaleSetName"`,
	`action GetParameter "kubeClusterCidr"`,
	`action GetParameter "kubeServiceCidr"`,
	`action GetParameter "vnetCidr"`,
	`action GetAgentKubernetesLabels .`,
	`action GetKubeletConfigKeyValsPsh`,
	`action GetKubeletHealthzEndpoint`,
	`action GetKubeproxyConfigKeyValsPsh`,
	`action GetKubeProxyFeatureGatesPsh`,
	`action GetVariable "useManagedIdentityExtension"`,
	`action GetVariable "useInstanceMetadata"`,
	`action GetVariable "loadBalancerSku"`,
	`action GetVariable "excludeMasterFromStandardLB"`,
	`action GetPrivateEgressProxyAddress`,
	`action GetParameter "networkPlugin"`,
	`action GetParameter "vnetCniWindowsPluginsURL"`,
	`if IsIPv6DualStackFeatureEnabled`,
	`if IsAzureCNIOverlayFeatureEnabled`,
	`if CiliumDataplaneEnabled`,
	`if EnableIMDSRestriction`,
	`action GetParameter "windowsCredentialProviderURL"`,
	`action GetVariable "windowsEnableCSIProxy"`,
	`action GetVariable "windowsCSIProxyURL"`,
	`action EnableHostsConfigAgent`,
	`action GetVariable "windowsCSEScriptsPackageURL"`,
	`action GetVariable "windowsGpuDriverURL"`,
	`action GetVariable "windowsPauseImageURL"`,
	`action GetVariable "alwaysPullWindowsPauseImage"`,
	`action GetVariable "windowsCalicoPackageURL"`,
	`action GetVariable "configGPUDriverIfNeeded"`,
	`action GetVariable "windowsGmsaPackageUrl"`,
	`action GetTLSBootstrapTokenForKubeConfig`,
	`action EnableSecureTLSBootstrapping`,
	`action GetSecureTLSBootstrappingAADResource`,
	`action GetSecureTLSBootstrappingUserAssignedIdentityID`,
	`action GetCustomSecureTLSBootstrappingClientDownloadURL`,
	`action GetSecureTLSBootstrappingValidateKubeconfigTimeout`,
	`action GetSecureTLSBootstrappingGetAccessTokenTimeout`,
	`action GetSecureTLSBootstrappingGetInstanceDataTimeout`,
	`action GetSecureTLSBootstrappingGetNonceTimeout`,
	`action GetSecureTLSBootstrappingGetAttestedDataTimeout`,
	`action GetSecureTLSBootstrappingGetCredentialTimeout`,
	`action GetVariable "isDisableWindowsOutboundNat"`,
	`action FIPSEnabled`,
	`action GetHnsRemediatorIntervalInMinutes`,
	`action GetLogGeneratorIntervalInMinutes`,
	`action GetVariable "isSkipCleanupNetwork"`,
	`action GetPreProvisionOnly`,
	`action EnableKubeletServingCertificateRotation`,
	`action GetVariable "nextGenNetworkingEnabled"`,
	`action GetVariable "nextGenNetworkingConfig"`,
	`action GetBootstrapProfileContainerRegistryServer`,
	`action GetMCRRepositoryBase`,
	`action GetNetworkIsolatedClusterTestMode`,
	`action WindowsSSHEnabled`,
	`action IsAKSCustomCloud`,
	`if IsAKSCustomCloud`,
	`action AKSCustomCloudContainerRegistryDNSSuffix`,
	`action GetBase64EncodedEnvironmentJSON`,
	`action GetIdentitySystem`,
}

type windowsTemplateNode struct {
	kind   string
	pipe   string
	offset int
}

// windowsTemplateNodes parses a Windows template the same way AgentBaker does and returns its text
// and every template node that can emit or select text.
func windowsTemplateNodes(t *testing.T, templatePath string) (string, []windowsTemplateNode) {
	t.Helper()
	b, err := parts.Templates.ReadFile(templatePath)
	require.NoError(t, err)
	funcMap := getBakerFuncMap(newWindowsBootstrapTestConfig(), paramsMap{}, paramsMap{})
	tmpl, err := template.New(templatePath).Funcs(funcMap).Parse(string(b))
	require.NoError(t, err)

	var nodes []windowsTemplateNode
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
		case *parse.ActionNode:
			nodes = append(nodes, windowsTemplateNode{kind: "action", pipe: n.Pipe.String(), offset: int(n.Pos)})
		case *parse.IfNode:
			nodes = append(nodes, windowsTemplateNode{kind: "if", pipe: n.Pipe.String(), offset: int(n.Pos)})
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			nodes = append(nodes, windowsTemplateNode{kind: "range", pipe: n.Pipe.String(), offset: int(n.Pos)})
		case *parse.WithNode:
			nodes = append(nodes, windowsTemplateNode{kind: "with", pipe: n.Pipe.String(), offset: int(n.Pos)})
		case *parse.TemplateNode:
			nodes = append(nodes, windowsTemplateNode{kind: "template", pipe: n.Name, offset: int(n.Pos)})
		}
	}
	walk(tmpl.Root)
	return string(b), nodes
}

func TestWindowsCSETemplateRendersValuesOnlyInVariablesBlock(t *testing.T) {
	text, nodes := windowsTemplateNodes(t, kubernetesWindowsAgentCustomDataPS1)
	require.Equal(t, 1, strings.Count(text, windowsBootstrapVariablesBegin), "expected exactly one variables block begin marker")
	require.Equal(t, 1, strings.Count(text, windowsBootstrapVariablesEnd), "expected exactly one variables block end marker")
	begin := strings.Index(text, windowsBootstrapVariablesBegin)
	end := strings.Index(text, windowsBootstrapVariablesEnd)
	require.Less(t, begin, end)

	var blockNodes []string
	for _, n := range nodes {
		line := 1 + strings.Count(text[:n.offset], "\n")
		if n.offset > begin && n.offset < end {
			switch n.kind {
			case "action", "if":
				blockNodes = append(blockNodes, n.kind+" "+n.pipe)
			default:
				t.Errorf("line %d: {{%s %s}} is not allowed in the variables block", line, n.kind, n.pipe)
			}
			continue
		}
		// $zippedFiles is the base64 of static AgentBaker scripts, so it never carries a rendered value.
		if n.kind == "action" && n.pipe == "GetKubernetesWindowsAgentFunctions" {
			continue
		}
		t.Errorf("line %d: {{%s %s}} is outside the AKS bootstrap variables block. "+
			"Add the value to the variables block and use the PowerShell variable instead", line, n.kind, n.pipe)
	}
	require.Equal(t, windowsBootstrapLegacyNodes, blockNodes,
		"values or conditions rendered directly into the Windows CSE script changed; new values must not be pasted into PowerShell code")
}

func TestWindowsCSECommandTemplateRendersOnlyBase64Secrets(t *testing.T) {
	_, nodes := windowsTemplateNodes(t, kubernetesWindowsAgentCSECommandPS1)
	got := make([]string, 0, len(nodes))
	for _, n := range nodes {
		got = append(got, n.kind+" "+n.pipe)
	}
	require.Equal(t, []string{
		`action GetParameter "clientPrivateKey"`,
		`action GetParameter "encodedServicePrincipalClientSecret"`,
		`if GetPreProvisionOnly`,
	}, got)

	config := newWindowsBootstrapTestConfig()
	config.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "key'\"$(Get-Date);`n\u2019"
	config.ContainerService.Properties.ServicePrincipalProfile.Secret = "secret'\"$(Get-Date);`n\u201d"
	_, cse := renderWindowsBootstrap(t, config)
	for _, parameter := range []string{"AgentKey", "AADClientSecret"} {
		match := regexp.MustCompile(`-` + parameter + ` ''([^']*)''`).FindStringSubmatch(cse)
		require.NotNil(t, match, "-%s not found in CSE command", parameter)
		require.Regexp(t, `^[A-Za-z0-9+/]*={0,2}$`, match[1], "-%s must be base64", parameter)
	}
}

func TestWindowsBootstrapRenderedScriptIsASCII(t *testing.T) {
	// Windows PowerShell 5.1 reads a script file without a byte order mark using the ANSI code page,
	// so any non-ASCII byte in the rendered script can be decoded as a different character.
	for _, fixture := range windowsBootstrapTestFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			customData, cse := renderWindowsBootstrap(t, fixture.newConfig())
			requireASCII(t, "CustomData", customData)
			requireASCII(t, "CSE command", cse)
		})
	}
}

func TestWindowsBootstrapVariablesGolden(t *testing.T) {
	for _, fixture := range windowsBootstrapTestFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			customData, _ := renderWindowsBootstrap(t, fixture.newConfig())
			requireGoldenFile(t, fixture.name+".legacy.ps1", extractWindowsBootstrapVariables(t, customData))
		})
	}
}

func requireASCII(t *testing.T, name, s string) {
	t.Helper()
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			line := 1 + strings.Count(s[:i], "\n")
			t.Fatalf("%s has a non-ASCII byte 0x%x on line %d", name, s[i], line)
		}
	}
}

// requireGoldenFile compares got with testdata/windowsbootstrap/<name>.
// Set GENERATE_TEST_DATA=true to rewrite the file.
func requireGoldenFile(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(windowsBootstrapTestdataDir, name)
	if os.Getenv("GENERATE_TEST_DATA") == "true" {
		require.NoError(t, os.MkdirAll(windowsBootstrapTestdataDir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing %s; regenerate it with: make generate-testdata", path)
	require.Equal(t, string(want), got, "%s is stale; regenerate it with: make generate-testdata", path)
}

func renderWindowsBootstrap(t *testing.T, config *datamodel.NodeBootstrappingConfiguration) (string, string) {
	t.Helper()
	validateAndSetWindowsNodeBootstrappingConfiguration(config)
	templateGenerator := InitializeTemplateGenerator()
	payload := templateGenerator.getWindowsNodeBootstrappingPayload(config)
	customData, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	return string(customData), templateGenerator.getWindowsNodeCSECommand(config)
}

// extractWindowsBootstrapVariables returns the rendered variables block, markers included.
func extractWindowsBootstrapVariables(t *testing.T, customData string) string {
	t.Helper()
	begin := strings.Index(customData, windowsBootstrapVariablesBegin)
	end := strings.Index(customData, windowsBootstrapVariablesEnd)
	require.True(t, begin >= 0 && end > begin, "variables block markers not found in rendered CustomData")
	return customData[begin:end+len(windowsBootstrapVariablesEnd)] + "\n"
}

type windowsBootstrapTestFixture struct {
	name      string
	newConfig func() *datamodel.NodeBootstrappingConfiguration
}

// windowsBootstrapTestFixtures covers the branches of the Windows CSE variables block
// with values shaped like the ones RP sends. The values only need to be valid PowerShell
// in the legacy block, not real credentials.
func windowsBootstrapTestFixtures() []windowsBootstrapTestFixture {
	return []windowsBootstrapTestFixture{
		{name: "default", newConfig: newWindowsBootstrapTestConfig},
		{name: "customcloud", newConfig: func() *datamodel.NodeBootstrappingConfiguration {
			config := newWindowsBootstrapTestConfig()
			properties := config.ContainerService.Properties
			config.ContainerService.Location = "customcloudregion"
			properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name:                       "akscustom",
				McrURL:                     "mcr.microsoft.custom.example",
				ResourceManagerEndpoint:    "https://management.custom.example/",
				ActiveDirectoryEndpoint:    "https://login.custom.example/",
				ContainerRegistryDNSSuffix: ".azurecr.custom.example",
				ResourceManagerVMDNSSuffix: "cloudapp.custom.example",
			}
			properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = false
			properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = ""
			properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{
				ClientID: "22222222-3333-4444-5555-666666666666",
				Secret:   "sp-secret-value",
			}
			properties.FeatureFlags.EnableIPv6DualStack = true
			properties.WindowsProfile.SSHEnabled = to.BoolPtr(false)
			config.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{}
			return config
		}},
		{name: "pisbake", newConfig: func() *datamodel.NodeBootstrappingConfiguration {
			config := newWindowsBootstrapTestConfig()
			properties := config.ContainerService.Properties
			hnsInterval, logInterval := uint32(1), uint32(5)
			config.PreProvisionOnly = true
			config.FIPSEnabled = true
			config.EnableIMDSRestriction = true
			config.ConfigGPUDriverIfNeeded = true
			config.AgentPoolProfile.Distro = datamodel.AKSWindows2019Containerd
			config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
				DisableOutboundNat:       to.BoolPtr(true),
				NextGenNetworkingEnabled: to.BoolPtr(true),
			}
			config.KubeproxyConfig = map[string]string{"--v": "3"}
			properties.LinuxProfile = nil
			properties.SecurityProfile = &datamodel.SecurityProfile{PrivateEgress: &datamodel.PrivateEgress{
				Enabled:                 true,
				ContainerRegistryServer: "privateacr.azurecr.io/aks-managed-repository",
				ProxyAddress:            "http://10.1.0.5:3128",
			}}
			properties.WindowsProfile.HnsRemediatorIntervalInMinutes = &hnsInterval
			properties.WindowsProfile.LogGeneratorIntervalInMinutes = &logInterval
			properties.WindowsProfile.GpuDriverURL = "https://packages.aks.azure.com/windows/gpu/driver.exe"
			properties.WindowsProfile.WindowsCalicoPackageURL = "https://packages.aks.azure.com/calico-node/v3.24.0/binaries/calico-windows-v3.24.0.zip"
			return config
		}},
	}
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
