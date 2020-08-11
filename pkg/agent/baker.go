// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/templates"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/i18n"
	"github.com/Azure/go-autorest/autorest/to"
)

// TemplateGenerator represents the object that performs the template generation.
type TemplateGenerator struct {
	Translator *i18n.Translator
}

// InitializeTemplateGenerator creates a new template generator object
func InitializeTemplateGenerator() *TemplateGenerator {
	t := &TemplateGenerator{
		Translator: &i18n.Translator{},
	}

	return t
}

// GetNodeBootstrappingPayload get node bootstrapping data
func (t *TemplateGenerator) GetNodeBootstrappingPayload(config *NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return getCustomDataFromJSON(t.getWindowsNodeCustomDataJSONObject(config))
	}
	return getCustomDataFromJSON(t.getLinuxNodeCustomDataJSONObject(config))
}

// GetLinuxNodeCustomDataJSONObject returns Linux customData JSON object in the form
// { "customData": "[base64(concat(<customData string>))]" }
func (t *TemplateGenerator) getLinuxNodeCustomDataJSONObject(config *NodeBootstrappingConfiguration) string {
	//get parameters
	parameters := getParameters(config, "baker", "1.0")
	//get variable cloudInit
	variables := getCustomDataVariables(config)
	str, e := t.getSingleLineForTemplate(kubernetesNodeCustomDataYaml,
		config.AgentPoolProfile, t.getBakerFuncMap(config, parameters, variables))

	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("{\"customData\": \"[base64(concat('%s'))]\"}", str)
}

// GetWindowsNodeCustomDataJSONObject returns Windows customData JSON object in the form
// { "customData": "[base64(concat(<customData string>))]" }
func (t *TemplateGenerator) getWindowsNodeCustomDataJSONObject(config *NodeBootstrappingConfiguration) string {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	//get parameters
	parameters := getParameters(config, "", "")
	//get variable cloudInit
	variables := getCustomDataVariables(config)
	str, e := t.getSingleLineForTemplate(kubernetesWindowsAgentCustomDataPS1,
		profile, t.getBakerFuncMap(config, parameters, variables))

	if e != nil {
		panic(e)
	}

	preprovisionCmd := ""

	if profile.PreprovisionExtension != nil {
		preprovisionCmd = makeAgentExtensionScriptCommands(cs, profile)
	}

	str = strings.Replace(str, "PREPROVISION_EXTENSION", escapeSingleLine(strings.TrimSpace(preprovisionCmd)), -1)

	return fmt.Sprintf("{\"customData\": \"[base64(concat('%s'))]\"}", str)
}

// GetNodeBootstrappingCmd get node bootstrapping cmd
func (t *TemplateGenerator) GetNodeBootstrappingCmd(config *NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return t.getWindowsNodeCustomDataJSONObject(config)
	}
	return t.getLinuxNodeCSECommand(config)
}

// getLinuxNodeCSECommand returns Linux node custom script extension execution command
func (t *TemplateGenerator) getLinuxNodeCSECommand(config *NodeBootstrappingConfiguration) string {
	//get parameters
	parameters := getParameters(config, "", "")
	//get variable
	variables := getCSECommandVariables(config)
	//NOTE: that CSE command will be executed by VM/VMSS extension so it doesn't need extra escaping like custom data does
	str, e := t.getSingleLine(
		kubernetesCSECommandString,
		config.AgentPoolProfile,
		t.getBakerFuncMap(config, parameters, variables),
	)

	if e != nil {
		panic(e)
	}
	// NOTE: we break the one-line CSE command into different lines in a file for better management
	// so we need to combine them into one line here
	return strings.Replace(str, "\n", " ", -1)
}

// getSingleLineForTemplate returns the file as a single line for embedding in an arm template
func (t *TemplateGenerator) getSingleLineForTemplate(textFilename string, profile interface{},
	funcMap template.FuncMap) (string, error) {
	expandedTemplate, err := t.getSingleLine(textFilename, profile, funcMap)
	if err != nil {
		return "", err
	}

	textStr := escapeSingleLine(expandedTemplate)

	return textStr, nil
}

// getSingleLine returns the file as a single line
func (t *TemplateGenerator) getSingleLine(textFilename string, profile interface{},
	funcMap template.FuncMap) (string, error) {
	b, err := templates.Asset(textFilename)
	if err != nil {
		return "", t.Translator.Errorf("yaml file %s does not exist", textFilename)
	}

	// use go templates to process the text filename
	templ := template.New("customdata template").Option("missingkey=zero").Funcs(funcMap)
	if _, err = templ.New(textFilename).Parse(string(b)); err != nil {
		return "", t.Translator.Errorf("error parsing file %s: %v", textFilename, err)
	}

	var buffer bytes.Buffer
	if err = templ.ExecuteTemplate(&buffer, textFilename, profile); err != nil {
		return "", t.Translator.Errorf("error executing template for file %s: %v", textFilename, err)
	}
	expandedTemplate := buffer.String()

	return expandedTemplate, nil
}

// getTemplateFuncMap returns the general purpose template func map from getContainerServiceFuncMap
func (t *TemplateGenerator) getBakerFuncMap(config *NodeBootstrappingConfiguration, params paramsMap, variables paramsMap) template.FuncMap {
	funcMap := getContainerServiceFuncMap(config)

	funcMap["GetParameter"] = func(s string) interface{} {
		if v, ok := params[s].(paramsMap); ok && v != nil {
			if v["value"] == nil {
				// return empty string so we don't get <no value> from go template
				return ""
			}
			return v["value"]
		}
		return ""
	}

	//TODO: GetParameterPropertyLower
	funcMap["GetParameterProperty"] = func(s, p string) interface{} {
		if v, ok := params[s].(paramsMap); ok && v != nil {
			if v["value"].(paramsMap)[p] == nil {
				// return empty string so we don't get <no value> from go template
				return ""
			}
			return v["value"].(paramsMap)[p]
		}
		return ""
	}

	funcMap["GetVariable"] = func(s string) interface{} {
		if variables[s] == nil {
			// return empty string so we don't get <no value> from go template
			return ""
		}
		return variables[s]
	}

	funcMap["GetVariableProperty"] = func(v, p string) interface{} {
		if v, ok := variables[v].(paramsMap); ok && v != nil {
			if v[p] == nil {
				// return empty string so we don't get <no value> from go template
				return ""
			}
			return v[p]
		}
		return ""
	}

	return funcMap
}

// getContainerServiceFuncMap returns all functions used in template generation
// These funcs are a thin wrapper for template generation operations,
// all business logic is implemented in the underlying func
func getContainerServiceFuncMap(config *NodeBootstrappingConfiguration) template.FuncMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	if profile.IsWindows() {
		profile = nil
	}
	return template.FuncMap{
		"IsMultiMasterCluster": func() bool {
			return cs.Properties.MasterProfile != nil && cs.Properties.MasterProfile.HasMultipleNodes()
		},
		"IsMasterVirtualMachineScaleSets": func() bool {
			return cs.Properties.MasterProfile != nil && cs.Properties.MasterProfile.IsVirtualMachineScaleSets()
		},
		"IsHostedMaster": func() bool {
			return cs.Properties.IsHostedMasterProfile()
		},
		"IsIPMasqAgentEnabled": func() bool {
			return cs.Properties.IsIPMasqAgentEnabled()
		},
		"IsKubernetesVersionGe": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"IsKubernetesVersionLt": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && !IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"GetAgentKubernetesLabels": func(profile *datamodel.AgentPoolProfile, rg string) string {
			return profile.GetKubernetesLabels(rg, false)
		},
		"GetAgentKubernetesLabelsDeprecated": func(profile *datamodel.AgentPoolProfile, rg string) string {
			return profile.GetKubernetesLabels(rg, true)
		},
		"GetDynamicKubeletConfigFileContent": func() string {
			if profile.KubernetesConfig == nil {
				return ""
			}
			return getDynamicKubeletConfigFileContent(profile.KubernetesConfig.KubeletConfig)
		},
		"IsDynamicKubeletEnabled": func() bool {
			return IsDynamicKubeletEnabled(cs, config.EnableDynamicKubelet)
		},
		"GetKubeletConfigKeyVals": func(kc *api.KubernetesConfig) string {
			if kc == nil {
				return ""
			}
			return GetOrderedKubeletConfigFlagString(kc, cs, config.EnableDynamicKubelet)
		},
		"GetKubeletConfigKeyValsPsh": func(kc *api.KubernetesConfig) string {
			if kc == nil {
				return ""
			}
			return kc.GetOrderedKubeletConfigStringForPowershell()
		},
		"HasPrivateRegistry": func() bool {
			if cs.Properties.OrchestratorProfile.DcosConfig != nil {
				return cs.Properties.OrchestratorProfile.DcosConfig.HasPrivateRegistry()
			}
			return false
		},
		"IsSwarmMode": func() bool {
			return cs.Properties.OrchestratorProfile.IsSwarmMode()
		},
		"IsKubernetes": func() bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes()
		},
		"GetKubernetesEndpoint": func() string {
			if cs.Properties.HostedMasterProfile == nil {
				return ""
			}
			return cs.Properties.HostedMasterProfile.FQDN
		},
		"IsAzureCNI": func() bool {
			return cs.Properties.OrchestratorProfile.IsAzureCNI()
		},
		"HasCosmosEtcd": func() bool {
			return cs.Properties.MasterProfile != nil && cs.Properties.MasterProfile.HasCosmosEtcd()
		},
		"GetCosmosEndPointUri": func() string {
			if cs.Properties.MasterProfile != nil {
				return cs.Properties.MasterProfile.GetCosmosEndPointURI()
			}
			return ""
		},
		"IsPrivateCluster": func() bool {
			return cs.Properties.OrchestratorProfile.IsPrivateCluster()
		},
		"EnableHostsConfigAgent": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster != nil &&
				to.Bool(cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent)
		},
		"ProvisionJumpbox": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateJumpboxProvision()
		},
		"UseManagedIdentity": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity
		},
		"GetVNETSubnetDependencies": func() string {
			return getVNETSubnetDependencies(cs.Properties)
		},
		"GetLBRules": func(name string, ports []int) string {
			return getLBRules(name, ports)
		},
		"GetProbes": func(ports []int) string {
			return getProbes(ports)
		},
		"GetSecurityRules": func(ports []int) string {
			return getSecurityRules(ports)
		},
		"GetUniqueNameSuffix": func() string {
			return cs.Properties.GetClusterID()
		},
		"GetVNETAddressPrefixes": func() string {
			return getVNETAddressPrefixes(cs.Properties)
		},
		"GetVNETSubnets": func(addNSG bool) string {
			return getVNETSubnets(cs.Properties, addNSG)
		},
		"GetDataDisks": func(profile *datamodel.AgentPoolProfile) string {
			return getDataDisks(profile)
		},
		"HasBootstrap": func() bool {
			return cs.Properties.OrchestratorProfile.DcosConfig != nil && cs.Properties.OrchestratorProfile.DcosConfig.HasBootstrap()
		},
		"GetDefaultVNETCIDR": func() string {
			return DefaultVNETCIDR
		},
		"GetDefaultVNETCIDRIPv6": func() string {
			return DefaultVNETCIDRIPv6
		},
		"GetSshPublicKeysPowerShell": func() string {
			return getSSHPublicKeysPowerShell(cs.Properties.LinuxProfile)
		},
		"GetWindowsMasterSubnetARMParam": func() string {
			return getWindowsMasterSubnetARMParam(cs.Properties.MasterProfile)
		},
		"GetKubernetesAgentPreprovisionYaml": func(profile *datamodel.AgentPoolProfile) string {
			str := ""
			if profile.PreprovisionExtension != nil {
				str += "\n"
				str += makeAgentExtensionScriptCommands(cs, profile)
			}
			return str
		},
		"GetLocation": func() string {
			return cs.Location
		},
		"GetKubernetesWindowsAgentFunctions": func() string {
			// Collect all the parts into a zip
			var parts = []string{
				kubernetesWindowsAgentFunctionsPS1,
				kubernetesWindowsConfigFunctionsPS1,
				kubernetesWindowsKubeletFunctionsPS1,
				kubernetesWindowsCniFunctionsPS1,
				kubernetesWindowsAzureCniFunctionsPS1,
				kubernetesWindowsOpenSSHFunctionPS1}

			// Create a buffer, new zip
			buf := new(bytes.Buffer)
			zw := zip.NewWriter(buf)

			for _, part := range parts {
				f, err := zw.Create(part)
				if err != nil {
					panic(err)
				}
				partContents, err := templates.Asset(part)
				if err != nil {
					panic(err)
				}
				_, err = f.Write(partContents)
				if err != nil {
					panic(err)
				}
			}
			err := zw.Close()
			if err != nil {
				panic(err)
			}
			return base64.StdEncoding.EncodeToString(buf.Bytes())
		},
		"AnyAgentIsLinux": func() bool {
			return cs.Properties.AnyAgentIsLinux()
		},
		"IsNSeriesSKU": func(profile *datamodel.AgentPoolProfile) bool {
			return IsNvidiaEnabledSKU(profile.VMSize)
		},
		"HasAvailabilityZones": func(profile *datamodel.AgentPoolProfile) bool {
			return profile.HasAvailabilityZones()
		},
		"HasLinuxSecrets": func() bool {
			return cs.Properties.LinuxProfile.HasSecrets()
		},
		"HasCustomSearchDomain": func() bool {
			return cs.Properties.LinuxProfile != nil && cs.Properties.LinuxProfile.HasSearchDomain()
		},
		"GetSearchDomainName": func() string {
			if cs.Properties.LinuxProfile != nil && cs.Properties.LinuxProfile.HasSearchDomain() {
				return cs.Properties.LinuxProfile.CustomSearchDomain.Name
			}
			return ""
		},
		"GetSearchDomainRealmUser": func() string {
			if cs.Properties.LinuxProfile != nil && cs.Properties.LinuxProfile.HasSearchDomain() {
				return cs.Properties.LinuxProfile.CustomSearchDomain.RealmUser
			}
			return ""
		},
		"GetSearchDomainRealmPassword": func() string {
			if cs.Properties.LinuxProfile != nil && cs.Properties.LinuxProfile.HasSearchDomain() {
				return cs.Properties.LinuxProfile.CustomSearchDomain.RealmPassword
			}
			return ""
		},
		"HasCiliumNetworkPlugin": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin == NetworkPluginCilium
		},
		"HasCiliumNetworkPolicy": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy == NetworkPolicyCilium
		},
		"HasAntreaNetworkPolicy": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy == NetworkPolicyAntrea
		},
		"HasFlannelNetworkPlugin": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin == NetworkPluginFlannel
		},
		"HasCustomNodesDNS": func() bool {
			return cs.Properties.LinuxProfile != nil && cs.Properties.LinuxProfile.HasCustomNodesDNS()
		},
		"HasWindowsSecrets": func() bool {
			return cs.Properties.WindowsProfile.HasSecrets()
		},
		"HasWindowsCustomImage": func() bool {
			return cs.Properties.WindowsProfile.HasCustomImage()
		},
		"WindowsSSHEnabled": func() bool {
			return cs.Properties.WindowsProfile.GetSSHEnabled()
		},
		"GetConfigurationScriptRootURL": func() string {
			linuxProfile := cs.Properties.LinuxProfile
			if linuxProfile == nil || linuxProfile.ScriptRootURL == "" {
				return DefaultConfigurationScriptRootURL
			}
			return linuxProfile.ScriptRootURL
		},
		"GetMasterOSImageOffer": func() string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageOffer)
		},
		"GetMasterOSImagePublisher": func() string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImagePublisher)
		},
		"GetMasterOSImageSKU": func() string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageSku)
		},
		"GetMasterOSImageVersion": func() string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageVersion)
		},
		"GetAgentOSImageOffer": func(profile *datamodel.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageOffer)
		},
		"GetAgentOSImagePublisher": func(profile *datamodel.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImagePublisher)
		},
		"GetAgentOSImageSKU": func(profile *datamodel.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageSku)
		},
		"GetAgentOSImageVersion": func(profile *datamodel.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageVersion)
		},
		"UseCloudControllerManager": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager != nil && *cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager
		},
		"AdminGroupID": func() bool {
			return cs.Properties.AADProfile != nil && cs.Properties.AADProfile.AdminGroupID != ""
		},
		"EnableDataEncryptionAtRest": func() bool {
			return to.Bool(cs.Properties.OrchestratorProfile.KubernetesConfig.EnableDataEncryptionAtRest)
		},
		"EnableEncryptionWithExternalKms": func() bool {
			return to.Bool(cs.Properties.OrchestratorProfile.KubernetesConfig.EnableEncryptionWithExternalKms)
		},
		"EnableAggregatedAPIs": func() bool {
			if cs.Properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs {
				return true
			} else if IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.9.0") {
				return true
			}
			return false
		},
		"EnablePodSecurityPolicy": func() bool {
			return to.Bool(cs.Properties.OrchestratorProfile.KubernetesConfig.EnablePodSecurityPolicy)
		},
		"IsCustomVNET": func() bool {
			return cs.Properties.AreAgentProfilesCustomVNET()
		},
		"IsIPv6DualStackFeatureEnabled": func() bool {
			return cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6DualStack")
		},
		"GetBase64EncodedEnvironmentJSON": func() string {
			customEnvironmentJSON, _ := cs.Properties.GetCustomEnvironmentJSON(false)
			return base64.StdEncoding.EncodeToString([]byte(customEnvironmentJSON))
		},
		"GetIdentitySystem": func() string {
			return api.AzureADIdentitySystem
		},
		"GetPodInfraContainerSpec": func() string {
			return cs.Properties.OrchestratorProfile.GetPodInfraContainerSpec()
		},
		"IsKubenet": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin == NetworkPluginKubenet
		},
		"NeedsContainerd": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
				return profile.KubernetesConfig.NeedsContainerd()
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NeedsContainerd()
		},
		"IsDockerContainerRuntime": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
				return profile.KubernetesConfig.ContainerRuntime == api.Docker
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime == api.Docker
		},
		"RequiresDocker": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
				return profile.KubernetesConfig.RequiresDocker()
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.RequiresDocker()
		},
		"HasDataDir": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig != nil && cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig[common.ContainerDataDirKey] != ""
		},
		"GetDataDir": func() string {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig[common.ContainerDataDirKey]
		},
		"HasNSeriesSKU": func() bool {
			return cs.Properties.HasNSeriesSKU()
		},
		"HasDCSeriesSKU": func() bool {
			return cs.Properties.HasDCSeriesSKU()
		},
		"HasCoreOS": func() bool {
			return cs.Properties.HasCoreOS()
		},
		"GetComponentImageReference": func(name string) string {
			k := cs.Properties.OrchestratorProfile.KubernetesConfig
			switch name {
			case "kube-apiserver":
				if k.CustomKubeAPIServerImage != "" {
					return k.CustomKubeAPIServerImage
				}
			case "kube-controller-manager":
				if k.CustomKubeControllerManagerImage != "" {
					return k.CustomKubeControllerManagerImage
				}
			case "kube-scheduler":
				if k.CustomKubeSchedulerImage != "" {
					return k.CustomKubeSchedulerImage
				}
			}
			kubernetesImageBase := k.KubernetesImageBase
			k8sComponents := api.K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
			return kubernetesImageBase + k8sComponents[name]
		},
		"GetHyperkubeImageReference": func() string {
			hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
			k8sComponents := api.K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
			hyperkubeImage := hyperkubeImageBase + k8sComponents["hyperkube"]
			if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
				hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
			}
			return hyperkubeImage
		},
		"GetTargetEnvironment": func() string {
			if cs.IsAKSCustomCloud() {
				return cs.Properties.CustomCloudEnv.Name
			}
			return GetCloudTargetEnv(cs.Location)
		},
		"IsAKSCustomCloud": func() bool {
			return cs.IsAKSCustomCloud()
		},
		"GetInitAKSCustomCloudFilepath": func() string {
			return initAKSCustomCloudFilepath
		},
		"AKSCustomCloudRepoDepotEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.RepoDepotEndpoint
		},
		"AKSCustomCloudManagementPortalURL": func() string {
			return cs.Properties.CustomCloudEnv.ManagementPortalURL
		},
		"AKSCustomCloudPublishSettingsURL": func() string {
			return cs.Properties.CustomCloudEnv.PublishSettingsURL
		},
		"AKSCustomCloudServiceManagementEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.ServiceManagementEndpoint
		},
		"AKSCustomCloudResourceManagerEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.ResourceManagerEndpoint
		},
		"AKSCustomCloudActiveDirectoryEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.ActiveDirectoryEndpoint
		},
		"AKSCustomCloudGalleryEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.GalleryEndpoint
		},
		"AKSCustomCloudKeyVaultEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.KeyVaultEndpoint
		},
		"AKSCustomCloudGraphEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.GraphEndpoint
		},
		"AKSCustomCloudServiceBusEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.ServiceBusEndpoint
		},
		"AKSCustomCloudBatchManagementEndpoint": func() string {
			return cs.Properties.CustomCloudEnv.BatchManagementEndpoint
		},
		"AKSCustomCloudStorageEndpointSuffix": func() string {
			return cs.Properties.CustomCloudEnv.StorageEndpointSuffix
		},
		"AKSCustomCloudSqlDatabaseDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.SQLDatabaseDNSSuffix
		},
		"AKSCustomCloudTrafficManagerDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.TrafficManagerDNSSuffix
		},
		"AKSCustomCloudKeyVaultDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.KeyVaultDNSSuffix
		},
		"AKSCustomCloudServiceBusEndpointSuffix": func() string {
			return cs.Properties.CustomCloudEnv.ServiceBusEndpointSuffix
		},
		"AKSCustomCloudServiceManagementVMDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.ServiceManagementVMDNSSuffix
		},
		"AKSCustomCloudResourceManagerVMDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.ResourceManagerVMDNSSuffix
		},
		"AKSCustomCloudContainerRegistryDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.ContainerRegistryDNSSuffix
		},
		"AKSCustomCloudCosmosDBDNSSuffix": func() string {
			return cs.Properties.CustomCloudEnv.CosmosDBDNSSuffix
		},
		"AKSCustomCloudTokenAudience": func() string {
			return cs.Properties.CustomCloudEnv.TokenAudience
		},
		"AKSCustomCloudResourceIdentifiersGraph": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.Graph
		},
		"AKSCustomCloudResourceIdentifiersKeyVault": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.KeyVault
		},
		"AKSCustomCloudResourceIdentifiersDatalake": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.Datalake
		},
		"AKSCustomCloudResourceIdentifiersBatch": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.Batch
		},
		"AKSCustomCloudResourceIdentifiersOperationalInsights": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.OperationalInsights
		},
		"AKSCustomCloudResourceIdentifiersStorage": func() string {
			return cs.Properties.CustomCloudEnv.ResourceIdentifiers.Storage
		},
		"GetCustomCloudConfigCSEScriptFilepath": func() string {
			return customCloudConfigCSEScriptFilepath
		},
		"GetCSEHelpersScriptFilepath": func() string {
			return cseHelpersScriptFilepath
		},
		"GetCSEInstallScriptFilepath": func() string {
			return cseInstallScriptFilepath
		},
		"GetCSEConfigScriptFilepath": func() string {
			return cseConfigScriptFilepath
		},
		"GetCustomSearchDomainsCSEScriptFilepath": func() string {
			return customSearchDomainsCSEScriptFilepath
		},
		"GetDHCPv6ServiceCSEScriptFilepath": func() string {
			return dhcpV6ServiceCSEScriptFilepath
		},
		"GetDHCPv6ConfigCSEScriptFilepath": func() string {
			return dhcpV6ConfigCSEScriptFilepath
		},
		"HasPrivateAzureRegistryServer": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateAzureRegistryServer != ""
		},
		"GetPrivateAzureRegistryServer": func() string {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateAzureRegistryServer
		},
		"HasTelemetryEnabled": func() bool {
			return cs.Properties.FeatureFlags != nil && cs.Properties.FeatureFlags.EnableTelemetry
		},
		"GetApplicationInsightsTelemetryKey": func() string {
			return cs.Properties.TelemetryProfile.ApplicationInsightsKey
		},
		"OpenBraces": func() string {
			return "{{"
		},
		"CloseBraces": func() string {
			return "}}"
		},
	}
}
