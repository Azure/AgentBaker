// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"text/template"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/i18n"
)

type ARMTemplate struct {
	Schema         string      `json:"$schema,omitempty"`
	ContentVersion string      `json:"contentVersion,omitempty"`
	Parameters     interface{} `json:"parameters,omitempty"`
	Variables      interface{} `json:"variables,omitempty"`
	Resources      interface{} `json:"resources,omitempty"`
	Outputs        interface{} `json:"outputs,omitempty"`
}

// TemplateGenerator represents the object that performs the template generation.
type TemplateGenerator struct {
	Translator     *i18n.Translator
	cloudInitFiles map[string]interface{}
	parameters     paramsMap
}

// InitializeTemplateGenerator creates a new template generator object
func InitializeTemplateGenerator(ctx Context, cs *api.ContainerService) (*TemplateGenerator, error) {
	t := &TemplateGenerator{
		Translator: ctx.Translator,
	}

	if t.Translator == nil {
		t.Translator = &i18n.Translator{}
	}

	if err := t.verifyFiles(); err != nil {
		return nil, err
	}

	t.cloudInitFiles = map[string]interface{}{
		"provisionScript":           getBase64EncodedGzippedCustomScript(kubernetesCSEMainScript, cs),
		"provisionSource":           getBase64EncodedGzippedCustomScript(kubernetesCSEHelpersScript, cs),
		"provisionInstalls":         getBase64EncodedGzippedCustomScript(kubernetesCSEInstall, cs),
		"provisionConfigs":          getBase64EncodedGzippedCustomScript(kubernetesCSEConfig, cs),
		"customSearchDomainsScript": getBase64EncodedGzippedCustomScript(kubernetesCustomSearchDomainsScript, cs),
		"dhcpv6SystemdService":      getBase64EncodedGzippedCustomScript(dhcpv6SystemdService, cs),
		"dhcpv6ConfigurationScript": getBase64EncodedGzippedCustomScript(dhcpv6ConfigurationScript, cs),
		"kubeletSystemdService":     getBase64EncodedGzippedCustomScript(kubeletSystemdService, cs),
		"systemdBPFMount":           getBase64EncodedGzippedCustomScript(systemdBPFMount, cs),
	}
	t.parameters = getParameters(cs, "baker", "1.0")
	return t, nil
}

func (t *TemplateGenerator) verifyFiles() error {
	allFiles := commonTemplateFiles
	allFiles = append(allFiles, swarmTemplateFiles...)
	for _, file := range allFiles {
		if _, err := Asset(file); err != nil {
			return t.Translator.Errorf("template file %s does not exist", file)
		}
	}
	return nil
}

// GetKubernetesLinuxNodeCustomDataJSONObject returns Linux customData JSON object in the form
// { "customData": "[base64(concat(<customData string>))]" }
func (t *TemplateGenerator) GetKubernetesLinuxNodeCustomDataJSONObject(cs *api.ContainerService, profile *api.AgentPoolProfile) string {
	str, e := t.getSingleLineForTemplate(kubernetesNodeCustomDataYaml, cs, profile)

	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("{\"customData\": \"[base64(concat('%s'))]\"}", str)
}

// GetKubernetesWindowsNodeCustomDataJSONObject returns Windows customData JSON object in the form
// { "customData": "[base64(concat(<customData string>))]" }
func (t *TemplateGenerator) GetKubernetesWindowsNodeCustomDataJSONObject(cs *api.ContainerService, profile *api.AgentPoolProfile) string {
	str, e := t.getSingleLineForTemplate(kubernetesWindowsAgentCustomDataPS1, cs, profile)

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

// getTemplateFuncMap returns the general purpose template func map from getContainerServiceFuncMap
func (t *TemplateGenerator) getTemplateFuncMap(cs *api.ContainerService) template.FuncMap {
	return getContainerServiceFuncMap(cs)
}

// getContainerServiceFuncMap returns all functions used in template generation
// These funcs are a thin wrapper for template generation operations,
// all business logic is implemented in the underlying func
func (t *TemplateGenerator) getContainerServiceFuncMap(cs *api.ContainerService) template.FuncMap {
	return template.FuncMap{
		"IsAzureStackCloud": func() bool {
			return cs.Properties.IsAzureStackCloud()
		},
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
		"IsDCOS19": func() bool {
			return cs.Properties.OrchestratorProfile != nil && cs.Properties.OrchestratorProfile.IsDCOS19()
		},
		"IsKubernetesVersionGe": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"IsKubernetesVersionLt": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && !IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"GetAgentKubernetesLabels": func(profile *api.AgentPoolProfile, rg string) string {
			return profile.GetKubernetesLabels(rg, false)
		},
		"GetAgentKubernetesLabelsDeprecated": func(profile *api.AgentPoolProfile, rg string) string {
			return profile.GetKubernetesLabels(rg, true)
		},
		"GetKubeletConfigKeyVals": func(kc *api.KubernetesConfig) string {
			if kc == nil {
				return ""
			}
			return kc.GetOrderedKubeletConfigString()
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
		"GetDataDisks": func(profile *api.AgentPoolProfile) string {
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
		"getSwarmVersions": func() string {
			return getSwarmVersions(api.SwarmVersion, api.SwarmDockerComposeVersion)
		},
		"GetSwarmModeVersions": func() string {
			return getSwarmVersions(api.DockerCEVersion, api.DockerCEDockerComposeVersion)
		},
		"WriteLinkedTemplatesForExtensions": func() string {
			return getLinkedTemplatesForExtensions(cs.Properties)
		},
		"GetSshPublicKeysPowerShell": func() string {
			return getSSHPublicKeysPowerShell(cs.Properties.LinuxProfile)
		},
		"GetWindowsMasterSubnetARMParam": func() string {
			return getWindowsMasterSubnetARMParam(cs.Properties.MasterProfile)
		},
		"GetKubernetesMasterPreprovisionYaml": func() string {
			str := ""
			if cs.Properties.MasterProfile.PreprovisionExtension != nil {
				str += "\n"
				str += makeMasterExtensionScriptCommands(cs)
			}
			return str
		},
		"GetKubernetesAgentPreprovisionYaml": func(profile *api.AgentPoolProfile) string {
			str := ""
			if profile.PreprovisionExtension != nil {
				str += "\n"
				str += makeAgentExtensionScriptCommands(cs, profile)
			}
			return str
		},
		"GetMasterSwarmCustomData": func() string {
			files := []string{swarmProvision}
			str := buildYamlFileWithWriteFiles(files, cs)
			if cs.Properties.MasterProfile.PreprovisionExtension != nil {
				extensionStr := makeMasterExtensionScriptCommands(cs)
				str += "'runcmd:\n" + extensionStr + "\n\n'"
			}
			str = escapeSingleLine(str)
			return fmt.Sprintf("\"customData\": \"[base64(concat('%s'))]\",", str)
		},
		"GetAgentSwarmCustomData": func(profile *api.AgentPoolProfile) string {
			files := []string{swarmProvision}
			str := buildYamlFileWithWriteFiles(files, cs)
			str = escapeSingleLine(str)
			return fmt.Sprintf("\"customData\": \"[base64(concat('%s',variables('%sRunCmdFile'),variables('%sRunCmd')))]\",", str, profile.Name, profile.Name)
		},
		"GetSwarmAgentPreprovisionExtensionCommands": func(profile *api.AgentPoolProfile) string {
			str := ""
			if profile.PreprovisionExtension != nil {
				makeAgentExtensionScriptCommands(cs, profile)
			}
			str = escapeSingleLine(str)
			return str
		},
		"GetLocation": func() string {
			return cs.Location
		},
		"GetWinAgentSwarmCustomData": func() string {
			str := getBase64EncodedGzippedCustomScript(swarmWindowsProvision, cs)
			return fmt.Sprintf("\"customData\": \"%s\"", str)
		},
		"GetWinAgentSwarmModeCustomData": func() string {
			str := getBase64EncodedGzippedCustomScript(swarmModeWindowsProvision, cs)
			return fmt.Sprintf("\"customData\": \"%s\"", str)
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
				partContents, err := Asset(part)
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
		"GetMasterSwarmModeCustomData": func() string {
			files := []string{swarmModeProvision}
			str := buildYamlFileWithWriteFiles(files, cs)
			if cs.Properties.MasterProfile.PreprovisionExtension != nil {
				extensionStr := makeMasterExtensionScriptCommands(cs)
				str += "runcmd:\n" + extensionStr + "\n\n"
			}
			str = escapeSingleLine(str)
			return fmt.Sprintf("\"customData\": \"[base64(concat('%s'))]\",", str)
		},
		"GetAgentSwarmModeCustomData": func(profile *api.AgentPoolProfile) string {
			files := []string{swarmModeProvision}
			str := buildYamlFileWithWriteFiles(files, cs)
			str = escapeSingleLine(str)
			return fmt.Sprintf("\"customData\": \"[base64(concat('%s',variables('%sRunCmdFile'),variables('%sRunCmd')))]\",", str, profile.Name, profile.Name)
		},
		"WrapAsVariable": func(s string) string {
			return fmt.Sprintf("',variables('%s'),'", s)
		},
		"CloudInitData": func(s string) string {
			return t.cloudInitFiles[s]
		},
		"GetParameter": func(s string) string {
			return t.parameters[s]
		},
		"AnyAgentUsesAvailabilitySets": func() bool {
			return cs.Properties.AnyAgentUsesAvailabilitySets()
		},
		"AnyAgentIsLinux": func() bool {
			return cs.Properties.AnyAgentIsLinux()
		},
		"IsNSeriesSKU": func(profile *api.AgentPoolProfile) bool {
			return IsNvidiaEnabledSKU(profile.VMSize)
		},
		"HasAvailabilityZones": func(profile *api.AgentPoolProfile) bool {
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
			return cs.Properties.WindowsProfile.SSHEnabled
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
		"GetAgentOSImageOffer": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageOffer)
		},
		"GetAgentOSImagePublisher": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImagePublisher)
		},
		"GetAgentOSImageSKU": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := cs.GetCloudSpecConfig()
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageSku)
		},
		"GetAgentOSImageVersion": func(profile *api.AgentPoolProfile) string {
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
			if cs.Properties.IsAzureStackCloud() {
				return cs.Properties.CustomCloudProfile.IdentitySystem
			}

			return api.AzureADIdentitySystem
		},
		"GetPodInfraContainerSpec": func() string {
			return cs.Properties.OrchestratorProfile.GetPodInfraContainerSpec()
		},
		"IsKubenet": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin == NetworkPluginKubenet
		},
		"NeedsContainerd": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NeedsContainerd()
		},
		"IsKataContainerRuntime": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime == api.KataContainers
		},
		"IsDockerContainerRuntime": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime == api.Docker
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
		"RequiresDocker": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.RequiresDocker()
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
			if cs.Properties.IsAzureStackCloud() {
				kubernetesImageBase = cs.GetCloudSpecConfig().KubernetesSpecConfig.KubernetesImageBase
			}
			k8sComponents := api.K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
			return kubernetesImageBase + k8sComponents[name]
		},
		"GetHyperkubeImageReference": func() string {
			hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
			k8sComponents := api.K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
			hyperkubeImage := hyperkubeImageBase + k8sComponents["hyperkube"]
			if cs.Properties.IsAzureStackCloud() {
				hyperkubeImage = hyperkubeImage + AzureStackSuffix
			}
			if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
				hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
			}
			return hyperkubeImage
		},
		"GetTargetEnvironment": func() string {
			return GetCloudTargetEnv(cs.Location)
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

func (t *TemplateGenerator) GenerateBootstrappingPayload(containerService *api.ContainerService, generatorCode string, acsengineVersion string) (templateRaw string, parametersRaw string, err error) {

	return getAgentCustomDataStr(cs, cs.Properties.AgentPoolProfiles[0])
}

func (t *TemplateGenerator) getParameterDescMap(containerService *api.ContainerService) (interface{}, error) {
	var templ *template.Template
	var paramsDescMap map[string]interface{}
	properties := containerService.Properties
	// save the current orchestrator version and restore it after deploying.
	// this allows us to deploy agents on the most recent patch without updating the orchestrator version in the object
	orchVersion := properties.OrchestratorProfile.OrchestratorVersion
	defer func() {
		properties.OrchestratorProfile.OrchestratorVersion = orchVersion
	}()

	templ = template.New("acs template").Funcs(t.getTemplateFuncMap(containerService))

	files, baseFile := kubernetesParamFiles, armParameters

	for _, file := range files {
		bytes, e := Asset(file)
		if e != nil {
			err := t.Translator.Errorf("Error reading file %s, Error: %s", file, e.Error())
			return nil, err
		}
		if _, err := templ.New(file).Parse(string(bytes)); err != nil {
			return nil, err
		}
	}

	var b bytes.Buffer
	if err := templ.ExecuteTemplate(&b, baseFile, properties); err != nil {
		return nil, err
	}

	err := json.Unmarshal(b.Bytes(), &paramsDescMap)

	if err != nil {
		return nil, err
	}

	return paramsDescMap["parameters"], nil
}
