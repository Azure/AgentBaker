// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// windowsBootstrapConfigSchemaVersion is the windowsBootstrapConfig version that
// kuberneteswindowssetup.ps1.template accepts. Bump it for any change that the script must know about.
const windowsBootstrapConfigSchemaVersion = 1

// windowsBootstrapConfig carries every value AgentBaker renders into the Windows CSE script when
// EnableWindowsStructuredBootstrapConfig is set. It is sent as gzip-compressed JSON in base64, and the
// script only parses it as data: no value is ever part of PowerShell code.
//
// Each JSON key is the name of the PowerShell variable the script assigns it to, and the Go type sets
// the PowerShell type: string, bool, uint32, or string array. The script and this struct must list the
// same keys. To add a value, add a field here and one assignment in the script's variables block.
type windowsBootstrapConfig struct {
	SchemaVersion int `json:"SchemaVersion"`

	MasterIP             string  `json:"MasterIP"`
	KubeDNSServiceIP     string  `json:"KubeDnsServiceIp"`
	MasterFQDNPrefix     string  `json:"MasterFQDNPrefix"`
	Location             string  `json:"Location"`
	UserAssignedClientID *string `json:"UserAssignedClientID"` // null leaves the variable unset, as before.
	TargetEnvironment    string  `json:"TargetEnvironment"`
	ArmResourceEndpoint  string  `json:"ArmResourceEndpoint"`
	AADClientID          string  `json:"AADClientId"`

	SSHKeys          []string `json:"SSHKeys"`
	CACertificate    string   `json:"CACertificate"`
	AgentCertificate string   `json:"AgentCertificate"`

	KubeBinariesPackageSASURL                string `json:"KubeBinariesPackageSASURL"`
	WindowsKubeBinariesURL                   string `json:"WindowsKubeBinariesURL"`
	KubeBinariesVersion                      string `json:"KubeBinariesVersion"`
	ContainerdURL                            string `json:"ContainerdUrl"`
	ContainerdSdnPluginURL                   string `json:"ContainerdSdnPluginUrl"`
	DockerVersion                            string `json:"DockerVersion"`
	DefaultContainerdWindowsSandboxIsolation string `json:"DefaultContainerdWindowsSandboxIsolation"`
	ContainerdWindowsRuntimeHandlers         string `json:"ContainerdWindowsRuntimeHandlers"`

	WindowsTelemetryGUID       string `json:"WindowsTelemetryGUID"`
	TenantID                   string `json:"TenantId"`
	SubscriptionID             string `json:"SubscriptionId"`
	ResourceGroup              string `json:"ResourceGroup"`
	VMType                     string `json:"VmType"`
	SubnetName                 string `json:"SubnetName"`
	SecurityGroupName          string `json:"SecurityGroupName"`
	VNetName                   string `json:"VNetName"`
	RouteTableName             string `json:"RouteTableName"`
	PrimaryAvailabilitySetName string `json:"PrimaryAvailabilitySetName"`
	PrimaryScaleSetName        string `json:"PrimaryScaleSetName"`

	KubeClusterCIDR        string   `json:"KubeClusterCIDR"`
	KubeServiceCIDR        string   `json:"KubeServiceCIDR"`
	VNetCIDR               string   `json:"VNetCIDR"`
	KubeletNodeLabels      string   `json:"KubeletNodeLabels"`
	KubeletConfigArgs      []string `json:"KubeletConfigArgs"`
	KubeletHealthzEndpoint string   `json:"KubeletHealthzEndpoint"`
	KubeproxyConfigArgs    []string `json:"KubeproxyConfigArgs"`
	KubeproxyFeatureGates  []string `json:"KubeproxyFeatureGates"`

	UseManagedIdentityExtension string `json:"UseManagedIdentityExtension"`
	UseInstanceMetadata         string `json:"UseInstanceMetadata"`
	LoadBalancerSku             string `json:"LoadBalancerSku"`
	ExcludeMasterFromStandardLB string `json:"ExcludeMasterFromStandardLB"`
	PrivateEgressProxyAddress   string `json:"PrivateEgressProxyAddress"`

	NetworkPlugin            string `json:"NetworkPlugin"`
	VNetCNIPluginsURL        string `json:"VNetCNIPluginsURL"`
	IsDualStackEnabled       bool   `json:"IsDualStackEnabled"`
	IsAzureCNIOverlayEnabled bool   `json:"IsAzureCNIOverlayEnabled"`
	CiliumDataplaneEnabled   bool   `json:"CiliumDataplaneEnabled"`
	IsIMDSRestrictionEnabled bool   `json:"IsIMDSRestrictionEnabled"`

	CredentialProviderURL       string `json:"CredentialProviderURL"`
	EnableCsiProxy              bool   `json:"EnableCsiProxy"`
	CsiProxyURL                 string `json:"CsiProxyUrl"`
	EnableHostsConfigAgent      bool   `json:"EnableHostsConfigAgent"`
	CSEScriptsPackageURL        string `json:"CSEScriptsPackageUrl"`
	GpuDriverURL                string `json:"GpuDriverURL"`
	WindowsPauseImageURL        string `json:"WindowsPauseImageURL"`
	AlwaysPullWindowsPauseImage bool   `json:"AlwaysPullWindowsPauseImage"`
	WindowsCalicoPackageURL     string `json:"WindowsCalicoPackageURL"`
	ConfigGPUDriverIfNeeded     bool   `json:"ConfigGPUDriverIfNeeded"`
	WindowsGmsaPackageURL       string `json:"WindowsGmsaPackageUrl"`
	TLSBootstrapToken           string `json:"TLSBootstrapToken"`

	EnableSecureTLSBootstrapping                    bool   `json:"EnableSecureTLSBootstrapping"`
	SecureTLSBootstrappingAADResource               string `json:"SecureTLSBootstrappingAADResource"`
	SecureTLSBootstrappingUserAssignedIdentityID    string `json:"SecureTLSBootstrappingUserAssignedIdentityID"`
	CustomSecureTLSBootstrappingClientDownloadURL   string `json:"CustomSecureTLSBootstrappingClientDownloadURL"`
	SecureTLSBootstrappingValidateKubeconfigTimeout string `json:"SecureTLSBootstrappingValidateKubeconfigTimeout"`
	SecureTLSBootstrappingGetAccessTokenTimeout     string `json:"SecureTLSBootstrappingGetAccessTokenTimeout"`
	SecureTLSBootstrappingGetInstanceDataTimeout    string `json:"SecureTLSBootstrappingGetInstanceDataTimeout"`
	SecureTLSBootstrappingGetNonceTimeout           string `json:"SecureTLSBootstrappingGetNonceTimeout"`
	SecureTLSBootstrappingGetAttestedDataTimeout    string `json:"SecureTLSBootstrappingGetAttestedDataTimeout"`
	SecureTLSBootstrappingGetCredentialTimeout      string `json:"SecureTLSBootstrappingGetCredentialTimeout"`

	IsDisableWindowsOutboundNat             bool   `json:"IsDisableWindowsOutboundNat"`
	FIPSEnabled                             bool   `json:"fipsEnabled"`
	HNSRemediatorIntervalInMinutes          uint32 `json:"HNSRemediatorIntervalInMinutes"`
	LogGeneratorIntervalInMinutes           uint32 `json:"LogGeneratorIntervalInMinutes"`
	IsSkipCleanupNetwork                    bool   `json:"IsSkipCleanupNetwork"`
	PreProvisionOnly                        bool   `json:"PreProvisionOnly"`
	EnableKubeletServingCertificateRotation bool   `json:"EnableKubeletServingCertificateRotation"`
	EnableWindowsCiliumNetworking           bool   `json:"EnableWindowsCiliumNetworking"`
	WindowsCiliumNetworkingConfiguration    string `json:"WindowsCiliumNetworkingConfiguration"`
	BootstrapProfileContainerRegistryServer string `json:"BootstrapProfileContainerRegistryServer"`
	MCRRepositoryBase                       string `json:"MCRRepositoryBase"`
	NetworkIsolatedClusterTestMode          bool   `json:"NetworkIsolatedClusterTestMode"`

	WindowsSSHEnabled                        bool   `json:"WindowsSSHEnabled"`
	IsAKSCustomCloud                         bool   `json:"IsAKSCustomCloud"`
	AKSCustomCloudContainerRegistryDNSSuffix string `json:"AKSCustomCloudContainerRegistryDNSSuffix"`
	AKSCustomCloudEnvironmentJSONBase64      string `json:"AKSCustomCloudEnvironmentJSONBase64"`
	IdentitySystem                           string `json:"IdentitySystem"`
}

// getWindowsBootstrapConfig builds the config from the same template functions that render the legacy
// variables block, so both blocks carry the same text. Only the conversion to bool, uint32 and lists
// happens here instead of in PowerShell.
//
//nolint:funlen // One assignment per value keeps the mapping to the script easy to review.
func getWindowsBootstrapConfig(config *datamodel.NodeBootstrappingConfiguration, funcMap template.FuncMap) (*windowsBootstrapConfig, error) {
	v := &windowsTemplateValues{funcMap: funcMap}
	profile := config.AgentPoolProfile
	properties := config.ContainerService.Properties

	c := &windowsBootstrapConfig{
		SchemaVersion:    windowsBootstrapConfigSchemaVersion,
		MasterIP:         v.text("GetKubernetesEndpoint"),
		KubeDNSServiceIP: v.parameter("kubeDNSServiceIP"),
		MasterFQDNPrefix: v.parameter("masterEndpointDNSNamePrefix"),
		Location:         v.variable("location"),
	}
	if v.boolean("UserAssignedIDEnabled") {
		userAssignedClientID := v.variable("userAssignedIdentityID")
		c.UserAssignedClientID = &userAssignedClientID
	}
	c.TargetEnvironment = v.text("GetTargetEnvironment")
	c.ArmResourceEndpoint = v.text("GetArmResourceEndpoint")
	c.AADClientID = v.parameter("servicePrincipalClientId")

	c.SSHKeys = []string{}
	if properties.LinuxProfile != nil {
		for _, publicKey := range properties.LinuxProfile.SSH.PublicKeys {
			c.SSHKeys = append(c.SSHKeys, strings.TrimSpace(publicKey.KeyData))
		}
	}
	c.CACertificate = v.parameter("caCertificate")
	c.AgentCertificate = v.parameter("clientCertificate")

	c.KubeBinariesPackageSASURL = v.parameter("kubeBinariesSASURL")
	c.WindowsKubeBinariesURL = v.parameter("windowsKubeBinariesURL")
	c.KubeBinariesVersion = v.parameter("kubeBinariesVersion")
	c.ContainerdURL = v.parameter("windowsContainerdURL")
	c.ContainerdSdnPluginURL = v.parameter("windowsSdnPluginURL")
	c.DockerVersion = v.parameter("windowsDockerVersion")
	c.DefaultContainerdWindowsSandboxIsolation = v.parameter("defaultContainerdWindowsSandboxIsolation")
	c.ContainerdWindowsRuntimeHandlers = v.parameter("containerdWindowsRuntimeHandlers")

	c.WindowsTelemetryGUID = v.parameter("windowsTelemetryGUID")
	c.TenantID = v.variable("tenantID")
	c.SubscriptionID = v.variable("subscriptionId")
	c.ResourceGroup = v.variable("resourceGroup")
	c.VMType = v.variable("vmType")
	c.SubnetName = v.variable("subnetName")
	c.SecurityGroupName = v.variable("nsgName")
	c.VNetName = v.variable("virtualNetworkName")
	c.RouteTableName = v.variable("routeTableName")
	c.PrimaryAvailabilitySetName = v.variable("primaryAvailabilitySetName")
	c.PrimaryScaleSetName = v.variable("primaryScaleSetName")

	c.KubeClusterCIDR = v.parameter("kubeClusterCidr")
	c.KubeServiceCIDR = v.parameter("kubeServiceCidr")
	c.VNetCIDR = v.parameter("vnetCidr")
	c.KubeletNodeLabels = v.text("GetAgentKubernetesLabels", profile)
	c.KubeletConfigArgs = powershellDoubleQuotedValues(config.GetOrderedKubeletConfigArgsForWindows(profile.CustomKubeletConfig))
	c.KubeletHealthzEndpoint = v.text("GetKubeletHealthzEndpoint")
	c.KubeproxyConfigArgs = powershellDoubleQuotedValues(config.GetOrderedKubeproxyConfigArgsForWindows())
	c.KubeproxyFeatureGates = powershellDoubleQuotedValues(properties.GetKubeProxyFeatureGatesForWindows())

	c.UseManagedIdentityExtension = v.variable("useManagedIdentityExtension")
	c.UseInstanceMetadata = v.variable("useInstanceMetadata")
	c.LoadBalancerSku = v.variable("loadBalancerSku")
	c.ExcludeMasterFromStandardLB = v.variable("excludeMasterFromStandardLB")
	c.PrivateEgressProxyAddress = v.text("GetPrivateEgressProxyAddress")

	c.NetworkPlugin = v.parameter("networkPlugin")
	c.VNetCNIPluginsURL = v.parameter("vnetCniWindowsPluginsURL")
	c.IsDualStackEnabled = v.boolean("IsIPv6DualStackFeatureEnabled")
	c.IsAzureCNIOverlayEnabled = v.boolean("IsAzureCNIOverlayFeatureEnabled")
	c.CiliumDataplaneEnabled = v.boolean("CiliumDataplaneEnabled")
	c.IsIMDSRestrictionEnabled = v.boolean("EnableIMDSRestriction")

	c.CredentialProviderURL = v.parameter("windowsCredentialProviderURL")
	c.EnableCsiProxy = v.boolean("GetVariable", "windowsEnableCSIProxy")
	c.CsiProxyURL = v.variable("windowsCSIProxyURL")
	c.EnableHostsConfigAgent = v.boolean("EnableHostsConfigAgent")
	c.CSEScriptsPackageURL = v.variable("windowsCSEScriptsPackageURL")
	c.GpuDriverURL = v.variable("windowsGpuDriverURL")
	c.WindowsPauseImageURL = v.variable("windowsPauseImageURL")
	c.AlwaysPullWindowsPauseImage = v.boolean("GetVariable", "alwaysPullWindowsPauseImage")
	c.WindowsCalicoPackageURL = v.variable("windowsCalicoPackageURL")
	c.ConfigGPUDriverIfNeeded = v.boolean("GetVariable", "configGPUDriverIfNeeded")
	c.WindowsGmsaPackageURL = v.variable("windowsGmsaPackageUrl")
	c.TLSBootstrapToken = v.text("GetTLSBootstrapTokenForKubeConfig")

	c.EnableSecureTLSBootstrapping = v.boolean("EnableSecureTLSBootstrapping")
	c.SecureTLSBootstrappingAADResource = v.text("GetSecureTLSBootstrappingAADResource")
	c.SecureTLSBootstrappingUserAssignedIdentityID = v.text("GetSecureTLSBootstrappingUserAssignedIdentityID")
	c.CustomSecureTLSBootstrappingClientDownloadURL = v.text("GetCustomSecureTLSBootstrappingClientDownloadURL")
	c.SecureTLSBootstrappingValidateKubeconfigTimeout = v.text("GetSecureTLSBootstrappingValidateKubeconfigTimeout")
	c.SecureTLSBootstrappingGetAccessTokenTimeout = v.text("GetSecureTLSBootstrappingGetAccessTokenTimeout")
	c.SecureTLSBootstrappingGetInstanceDataTimeout = v.text("GetSecureTLSBootstrappingGetInstanceDataTimeout")
	c.SecureTLSBootstrappingGetNonceTimeout = v.text("GetSecureTLSBootstrappingGetNonceTimeout")
	c.SecureTLSBootstrappingGetAttestedDataTimeout = v.text("GetSecureTLSBootstrappingGetAttestedDataTimeout")
	c.SecureTLSBootstrappingGetCredentialTimeout = v.text("GetSecureTLSBootstrappingGetCredentialTimeout")

	c.IsDisableWindowsOutboundNat = v.boolean("GetVariable", "isDisableWindowsOutboundNat")
	c.FIPSEnabled = v.boolean("FIPSEnabled")
	c.HNSRemediatorIntervalInMinutes = v.uint32("GetHnsRemediatorIntervalInMinutes")
	c.LogGeneratorIntervalInMinutes = v.uint32("GetLogGeneratorIntervalInMinutes")
	c.IsSkipCleanupNetwork = v.boolean("GetVariable", "isSkipCleanupNetwork")
	c.PreProvisionOnly = v.boolean("GetPreProvisionOnly")
	c.EnableKubeletServingCertificateRotation = v.boolean("EnableKubeletServingCertificateRotation")
	c.EnableWindowsCiliumNetworking = v.boolean("GetVariable", "nextGenNetworkingEnabled")
	c.WindowsCiliumNetworkingConfiguration = v.variable("nextGenNetworkingConfig")
	c.BootstrapProfileContainerRegistryServer = v.text("GetBootstrapProfileContainerRegistryServer")
	c.MCRRepositoryBase = v.text("GetMCRRepositoryBase")
	c.NetworkIsolatedClusterTestMode = v.boolean("GetNetworkIsolatedClusterTestMode")

	c.WindowsSSHEnabled = v.boolean("WindowsSSHEnabled")
	c.IsAKSCustomCloud = v.boolean("IsAKSCustomCloud")
	if c.IsAKSCustomCloud {
		c.AKSCustomCloudContainerRegistryDNSSuffix = v.text("AKSCustomCloudContainerRegistryDNSSuffix")
		c.AKSCustomCloudEnvironmentJSONBase64 = v.text("GetBase64EncodedEnvironmentJSON")
	}
	c.IdentitySystem = v.text("GetIdentitySystem")

	if v.err != nil {
		return nil, v.err
	}
	return c, nil
}

// getEncodedWindowsBootstrapConfig returns the Windows bootstrap config as gzip-compressed JSON in base64.
func getEncodedWindowsBootstrapConfig(config *datamodel.NodeBootstrappingConfiguration, funcMap template.FuncMap) (string, error) {
	bootstrapConfig, err := getWindowsBootstrapConfig(config, funcMap)
	if err != nil {
		return "", err
	}
	return encodeWindowsBootstrapConfig(bootstrapConfig)
}

// encodeWindowsBootstrapConfig returns the config as gzip-compressed JSON in base64. The base64 alphabet
// has no quote, so the script can hold it in a single-quoted PowerShell string.
func encodeWindowsBootstrapConfig(c *windowsBootstrapConfig) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(c); err != nil {
		return "", fmt.Errorf("failed to serialize the Windows bootstrap config: %w", err)
	}
	return getBase64EncodedGzippedCustomScriptFromStr(buf.String()), nil
}

// powershellDoubleQuotedValues undoes the one PowerShell escape that producers rely on for Windows
// kubelet and kube-proxy arguments. These arguments were always rendered inside PowerShell double-quoted
// strings, and RP sends four double quotes (for example --resolv-conf="""") to get "" on the node.
// No other PowerShell rule ($ expansion, backtick escapes) is applied: the values stay data.
func powershellDoubleQuotedValues(items []string) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, strings.ReplaceAll(item, `""`, `"`))
	}
	return values
}

// windowsTemplateValues calls template functions by name and records the first error.
type windowsTemplateValues struct {
	funcMap template.FuncMap
	err     error
}

// text returns what the template action {{name args...}} would print.
func (v *windowsTemplateValues) text(name string, args ...any) string {
	if v.err != nil {
		return ""
	}
	fn := reflect.ValueOf(v.funcMap[name])
	if fn.Kind() != reflect.Func {
		v.err = fmt.Errorf("template function %q not found", name)
		return ""
	}
	in := make([]reflect.Value, 0, len(args))
	for _, arg := range args {
		in = append(in, reflect.ValueOf(arg))
	}
	out := fn.Call(in)
	if len(out) == 0 {
		v.err = fmt.Errorf("template function %q returns no value", name)
		return ""
	}
	if len(out) > 1 && !out[1].IsNil() {
		err, ok := out[1].Interface().(error)
		if !ok {
			err = errors.New("unexpected second return value")
		}
		v.err = fmt.Errorf("template function %q failed: %w", name, err)
		return ""
	}
	return fmt.Sprint(out[0].Interface())
}

func (v *windowsTemplateValues) parameter(key string) string {
	return v.text("GetParameter", key)
}

func (v *windowsTemplateValues) variable(key string) string {
	return v.text("GetVariable", key)
}

// boolean parses the printed value the way the legacy script did with [System.Convert]::ToBoolean.
func (v *windowsTemplateValues) boolean(name string, args ...any) bool {
	s := v.text(name, args...)
	if v.err != nil {
		return false
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		v.err = fmt.Errorf("template function %q returned %q, not a boolean: %w", name, s, err)
	}
	return b
}

// uint32 parses the printed value the way the legacy script did with [System.Convert]::ToUInt32.
func (v *windowsTemplateValues) uint32(name string) uint32 {
	s := v.text(name)
	if v.err != nil {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		v.err = fmt.Errorf("template function %q returned %q, not a uint32: %w", name, s, err)
	}
	return uint32(n)
}
