// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/templates"
	"github.com/Azure/go-autorest/autorest/to"
)

// TemplateGenerator represents the object that performs the template generation.
type TemplateGenerator struct{}

// InitializeTemplateGenerator creates a new template generator object
func InitializeTemplateGenerator() *TemplateGenerator {
	t := &TemplateGenerator{}
	return t
}

// GetNodeBootstrappingPayload get node bootstrapping data
func (t *TemplateGenerator) GetNodeBootstrappingPayload(config *datamodel.NodeBootstrappingConfiguration) string {
	var customData string
	if config.AgentPoolProfile.IsWindows() {
		customData = getCustomDataFromJSON(t.getWindowsNodeCustomDataJSONObject(config))
	} else {
		customData = getCustomDataFromJSON(t.getLinuxNodeCustomDataJSONObject(config))
	}
	return base64.StdEncoding.EncodeToString([]byte(customData))
}

// GetLinuxNodeCustomDataJSONObject returns Linux customData JSON object in the form
// { "customData": "<customData string>" }
func (t *TemplateGenerator) getLinuxNodeCustomDataJSONObject(config *datamodel.NodeBootstrappingConfiguration) string {
	// validate and fix input
	validateAndSetLinuxNodeBootstrappingConfiguration(config)
	//get parameters
	parameters := getParameters(config, "baker", "1.0")
	//get variable cloudInit
	variables := getCustomDataVariables(config)
	str, e := t.getSingleLineForTemplate(kubernetesNodeCustomDataYaml,
		config.AgentPoolProfile, t.getBakerFuncMap(config, parameters, variables))

	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("{\"customData\": \"%s\"}", str)
}

// GetWindowsNodeCustomDataJSONObject returns Windows customData JSON object in the form
// { "customData": "<customData string>" }
func (t *TemplateGenerator) getWindowsNodeCustomDataJSONObject(config *datamodel.NodeBootstrappingConfiguration) string {
	// validtae and fix input
	validateAndSetWindowsNodeBootstrappingConfiguration(config)

	cs := config.ContainerService
	profile := config.AgentPoolProfile
	//get parameters
	parameters := getParameters(config, "", "")
	//get variable custom data
	variables := getWindowsCustomDataVariables(config)
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
	return fmt.Sprintf("{\"customData\": \"%s\"}", str)
}

// GetNodeBootstrappingCmd get node bootstrapping cmd
func (t *TemplateGenerator) GetNodeBootstrappingCmd(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return t.getWindowsNodeCSECommand(config)
	}
	return t.getLinuxNodeCSECommand(config)
}

// getLinuxNodeCSECommand returns Linux node custom script extension execution command
func (t *TemplateGenerator) getLinuxNodeCSECommand(config *datamodel.NodeBootstrappingConfiguration) string {
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

// getWindowsNodeCSECommand returns Windows node custom script extension execution command
func (t *TemplateGenerator) getWindowsNodeCSECommand(config *datamodel.NodeBootstrappingConfiguration) string {
	//get parameters
	parameters := getParameters(config, "", "")
	//get variable
	variables := getCSECommandVariables(config)

	//NOTE: that CSE command will be executed by VMSS extension so it doesn't need extra escaping like custom data does
	str, e := t.getSingleLine(
		kubernetesWindowsAgentCSECommandPS1,
		config.AgentPoolProfile,
		t.getBakerFuncMap(config, parameters, variables),
	)

	if e != nil {
		panic(e)
	}
	// NOTE(qinahao): windows cse cmd uses esapced \" to quote Powershell command in [csecmd.p1](https://github.com/Azure/AgentBaker/blob/master/parts/windows/csecmd.ps1)
	// to not break go template parsing. We switch \" back to " otherwise Azure ARM template will escape \ to be \\\"
	str = strings.Replace(str, `\"`, `"`, -1)

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
		return "", fmt.Errorf("yaml file %s does not exist", textFilename)
	}

	// use go templates to process the text filename
	templ := template.New("customdata template").Option("missingkey=zero").Funcs(funcMap)
	if _, err = templ.New(textFilename).Parse(string(b)); err != nil {
		return "", fmt.Errorf("error parsing file %s: %v", textFilename, err)
	}

	var buffer bytes.Buffer
	if err = templ.ExecuteTemplate(&buffer, textFilename, profile); err != nil {
		return "", fmt.Errorf("error executing template for file %s: %v", textFilename, err)
	}
	expandedTemplate := buffer.String()

	return expandedTemplate, nil
}

// getTemplateFuncMap returns the general purpose template func map from getContainerServiceFuncMap
func (t *TemplateGenerator) getBakerFuncMap(config *datamodel.NodeBootstrappingConfiguration, params paramsMap, variables paramsMap) template.FuncMap {
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

// normalizeResourceGroupNameForLabel normalizes resource group name to be used as a label,
// similar to what the ARM template used to do.
//
// When ARM template was used, the following is used:
//   variables('labelResourceGroup')
// which is defined as:
//   [if(or(or(endsWith(variables('truncatedResourceGroup'), '-'), endsWith(variables('truncatedResourceGroup'), '_')), endsWith(variables('truncatedResourceGroup'), '.')), concat(take(variables('truncatedResourceGroup'), 62), 'z'), variables('truncatedResourceGroup'))]
// the "truncatedResourceGroup" is defined as:
//   [take(replace(replace(resourceGroup().name, '(', '-'), ')', '-'), 63)]
// This function does the same processing.
func normalizeResourceGroupNameForLabel(resourceGroupName string) string {
	truncated := resourceGroupName
	truncated = strings.ReplaceAll(truncated, "(", "-")
	truncated = strings.ReplaceAll(truncated, ")", "-")
	const maxLen = 63
	if len(truncated) > maxLen {
		truncated = truncated[0:maxLen]
	}

	if strings.HasSuffix(truncated, "-") ||
		strings.HasSuffix(truncated, "_") ||
		strings.HasSuffix(truncated, ".") {

		if len(truncated) > 62 {
			return truncated[0:len(truncated)-1] + "z"
		} else {
			return truncated + "z"
		}
	}
	return truncated
}

func validateAndSetLinuxNodeBootstrappingConfiguration(config *datamodel.NodeBootstrappingConfiguration) {
	// If using kubelet config file, disable DynamicKubeletConfig feature gate and remove dynamic-config-dir
	// we should only allow users to configure from API (20201101 and later)
	profile := config.AgentPoolProfile
	if config.KubeletConfig != nil {
		kubeletFlags := config.KubeletConfig
		delete(kubeletFlags, "--dynamic-config-dir")
		delete(kubeletFlags, "--non-masquerade-cidr")
		if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" && profile.KubernetesConfig.ContainerRuntime == "containerd" {
			for _, flag := range dockerShimFlags {
				delete(kubeletFlags, flag)
			}
		}
		if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.24.0") {
			kubeletFlags["--feature-gates"] = removeFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig")
		} else if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.11.0") {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig", false)
		}

		// ContainerInsights depends on GPU accelerator Usage metrics from Kubelet cAdvisor endpoint but deprecation of this feature moved to beta which breaks the ContainerInsights customers with K8s version 1.20 or higher
		// Until Container Insights move to new API adding this feature gate to get the GPU metrics continue to work
		// Reference - https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1867-disable-accelerator-usage-metrics
		if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.20.0") &&
			!IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.25.0") {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DisableAcceleratorUsageMetrics", false)
		}
	}
}

func validateAndSetWindowsNodeBootstrappingConfiguration(config *datamodel.NodeBootstrappingConfiguration) {
	if IsKubeletClientTLSBootstrappingEnabled(config.KubeletClientTLSBootstrapToken) {
		// backfill proper flags for Windows agent node TLS bootstrapping
		if config.KubeletConfig == nil {
			config.KubeletConfig = make(map[string]string)
		}

		config.KubeletConfig["--bootstrap-kubeconfig"] = "c:\\k\\bootstrap-config"
		config.KubeletConfig["--cert-dir"] = "c:\\k\\pki"
	}
	if config.KubeletConfig != nil {
		kubeletFlags := config.KubeletConfig
		delete(kubeletFlags, "--dynamic-config-dir")
		if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.24.0") {
			kubeletFlags["--feature-gates"] = removeFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig")
		} else if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.11.0") {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig", false)
		}
	}
}

// getContainerServiceFuncMap returns all functions used in template generation
// These funcs are a thin wrapper for template generation operations,
// all business logic is implemented in the underlying func
func getContainerServiceFuncMap(config *datamodel.NodeBootstrappingConfiguration) template.FuncMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	return template.FuncMap{
		"Disable1804SystemdResolved": func() bool {
			return config.Disable1804SystemdResolved
		},
		// This was DisableUnattendedUpgrade when we had UU enabled by default in image.
		// Now we don't, so we have to deliberately enable it.
		// Someone smarter than me can fix the API.
		"EnableUnattendedUpgrade": func() bool {
			return !config.DisableUnattendedUpgrades
		},
		"IsIPMasqAgentEnabled": func() bool {
			return cs.Properties.IsIPMasqAgentEnabled()
		},
		"IsKubernetesVersionGe": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"GetAgentKubernetesLabels": func(profile *datamodel.AgentPoolProfile) string {
			return profile.GetKubernetesLabels(normalizeResourceGroupNameForLabel(config.ResourceGroupName),
				false, config.EnableNvidia, config.FIPSEnabled, config.OSSKU)
		},
		"GetAgentKubernetesLabelsDeprecated": func(profile *datamodel.AgentPoolProfile) string {
			return profile.GetKubernetesLabels(normalizeResourceGroupNameForLabel(config.ResourceGroupName),
				true, config.EnableNvidia, config.FIPSEnabled, config.OSSKU)
		},
		"GetGPUInstanceProfile": func() string {
			return config.GPUInstanceProfile
		},
		"IsMIGEnabledNode": func() bool {
			return config.GPUInstanceProfile != ""
		},
		"GetKubeletConfigFileContent": func() string {
			return GetKubeletConfigFileContent(config.KubeletConfig, profile.CustomKubeletConfig)
		},
		"IsKubeletConfigFileEnabled": func() bool {
			return IsKubeletConfigFileEnabled(cs, profile, config.EnableKubeletConfigFile)
		},
		"IsKubeletClientTLSBootstrappingEnabled": func() bool {
			return IsKubeletClientTLSBootstrappingEnabled(config.KubeletClientTLSBootstrapToken)
		},
		"GetTLSBootstrapTokenForKubeConfig": func() string {
			return GetTLSBootstrapTokenForKubeConfig(config.KubeletClientTLSBootstrapToken)
		},
		"GetKubeletConfigKeyVals": func() string {
			return GetOrderedKubeletConfigFlagString(config.KubeletConfig, cs, profile, config.EnableKubeletConfigFile)
		},
		"GetKrustletFlags": func() string {
			maxPods := config.KubeletConfig["--max-pods"]
			if maxPods != "" {
				return fmt.Sprintf("--max-pods=\"%s\"", maxPods)
			}
			return ""
		},
		"GetKubeletConfigKeyValsPsh": func() string {
			return config.GetOrderedKubeletConfigStringForPowershell(profile.CustomKubeletConfig)
		},
		"GetKubeproxyConfigKeyValsPsh": func() string {
			return config.GetOrderedKubeproxyConfigStringForPowershell()
		},
		"Is2204VHD": func() bool {
			return profile.Is2204VHDDistro()
		},
		"GetKubeProxyFeatureGatesPsh": func() string {
			return cs.Properties.GetKubeProxyFeatureGatesWindowsArguments()
		},
		"ShouldConfigCustomSysctl": func() bool {
			return profile.CustomLinuxOSConfig != nil && profile.CustomLinuxOSConfig.Sysctls != nil
		},
		"GetCustomSysctlConfigByName": func(fn string) interface{} {
			if profile.CustomLinuxOSConfig != nil && profile.CustomLinuxOSConfig.Sysctls != nil {
				v := reflect.ValueOf(*profile.CustomLinuxOSConfig.Sysctls)
				return v.FieldByName(fn).Interface()
			}
			return nil
		},
		"ShouldConfigTransparentHugePage": func() bool {
			return profile.CustomLinuxOSConfig != nil && (profile.CustomLinuxOSConfig.TransparentHugePageEnabled != "" || profile.CustomLinuxOSConfig.TransparentHugePageDefrag != "")
		},
		"GetTransparentHugePageEnabled": func() string {
			if profile.CustomLinuxOSConfig == nil {
				return ""
			}
			return profile.CustomLinuxOSConfig.TransparentHugePageEnabled
		},
		"GetTransparentHugePageDefrag": func() string {
			if profile.CustomLinuxOSConfig == nil {
				return ""
			}
			return profile.CustomLinuxOSConfig.TransparentHugePageDefrag
		},
		"ShouldConfigSwapFile": func() bool {
			// only configure swap file when FailSwapOn is false and SwapFileSizeMB is valid
			return profile.CustomKubeletConfig != nil && profile.CustomKubeletConfig.FailSwapOn != nil && !*profile.CustomKubeletConfig.FailSwapOn &&
				profile.CustomLinuxOSConfig != nil && profile.CustomLinuxOSConfig.SwapFileSizeMB != nil && *profile.CustomLinuxOSConfig.SwapFileSizeMB > 0
		},
		"GetSwapFileSizeMB": func() int32 {
			return *profile.CustomLinuxOSConfig.SwapFileSizeMB
		},
		"IsKubernetes": func() bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes()
		},
		"GetKubernetesEndpoint": func() string {
			if cs.Properties.HostedMasterProfile == nil {
				return ""
			}
			if cs.Properties.HostedMasterProfile.IPAddress != "" {
				return cs.Properties.HostedMasterProfile.IPAddress
			}
			return cs.Properties.HostedMasterProfile.FQDN
		},
		"IsAzureCNI": func() bool {
			return cs.Properties.OrchestratorProfile.IsAzureCNI()
		},
		"IsNoneCNI": func() bool {
			return cs.Properties.OrchestratorProfile.IsNoneCNI()
		},
		"IsMariner": func() bool {
			// TODO(ace): do we care about both? 2nd one should be more general and catch custom VHD for mariner
			return profile.Distro.IsCBLMarinerDistro() || isMariner(config.OSSKU)
		},
		"IsKata": func() bool {
			return profile.Distro.IsKataDistro()
		},
		"EnableHostsConfigAgent": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster != nil &&
				to.Bool(cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent)
		},
		"UseManagedIdentity": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity
		},
		"GetSshPublicKeysPowerShell": func() string {
			return getSSHPublicKeysPowerShell(cs.Properties.LinuxProfile)
		},
		"GetKubernetesAgentPreprovisionYaml": func(profile *datamodel.AgentPoolProfile) string {
			str := ""
			if profile.PreprovisionExtension != nil {
				str += "\n"
				str += makeAgentExtensionScriptCommands(cs, profile)
			}
			return str
		},
		"GetKubernetesWindowsAgentFunctions": func() string {
			// Collect all the parts into a zip
			var parts = []string{
				kubernetesWindowsCSEHelperPS1,
				kubernetesWindowsSendLogsPS1,
			}

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
		"IsNSeriesSKU": func() bool {
			return config.EnableNvidia
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
		"HasCalicoNetworkPolicy": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy == NetworkPolicyCalico
		},
		"HasAntreaNetworkPolicy": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy == NetworkPolicyAntrea
		},
		"HasFlannelNetworkPlugin": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin == NetworkPluginFlannel
		},
		"HasKubeletClientKey": func() bool {
			return cs.Properties.CertificateProfile != nil && cs.Properties.CertificateProfile.ClientPrivateKey != ""
		},
		"GetKubeletClientKey": func() string {
			if cs.Properties.CertificateProfile != nil && cs.Properties.CertificateProfile.ClientPrivateKey != "" {
				padded := fmt.Sprintf("%s\n%s", cs.Properties.CertificateProfile.ClientPrivateKey, "#EOF")
				encoded := base64.StdEncoding.EncodeToString([]byte(padded))
				return encoded
			}
			return ""
		},
		"HasServicePrincipalSecret": func() bool {
			return cs.Properties.ServicePrincipalProfile != nil && cs.Properties.ServicePrincipalProfile.Secret != ""
		},
		"GetServicePrincipalSecret": func() string {
			if cs.Properties.ServicePrincipalProfile != nil && cs.Properties.ServicePrincipalProfile.Secret != "" {
				padded := fmt.Sprintf("%s\n%s", cs.Properties.ServicePrincipalProfile.Secret, "#EOF")
				encoded := base64.StdEncoding.EncodeToString([]byte(padded))
				return encoded
			}
			return ""
		},
		"WindowsSSHEnabled": func() bool {
			return cs.Properties.WindowsProfile.GetSSHEnabled()
		},
		"IsIPv6DualStackFeatureEnabled": func() bool {
			return cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6DualStack")
		},
		"GetBase64EncodedEnvironmentJSON": func() string {
			customEnvironmentJSON, _ := cs.Properties.GetCustomEnvironmentJSON(false)
			return base64.StdEncoding.EncodeToString([]byte(customEnvironmentJSON))
		},
		"GetIdentitySystem": func() string {
			return datamodel.AzureADIdentitySystem
		},
		"GetPodInfraContainerSpec": func() string {
			return config.K8sComponents.PodInfraContainerImageURL
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
		"UseRuncShimV2": func() bool {
			return config.EnableRuncShimV2
		},
		"IsDockerContainerRuntime": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
				return profile.KubernetesConfig.ContainerRuntime == datamodel.Docker
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime == datamodel.Docker
		},
		"RequiresDocker": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
				return profile.KubernetesConfig.RequiresDocker()
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.RequiresDocker()
		},
		"HasDataDir": func() bool {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntimeConfig != nil && profile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != "" {
				return true
			}
			if profile.KubeletDiskType == datamodel.TempDisk {
				return true
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig != nil && cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != ""
		},
		"GetDataDir": func() string {
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntimeConfig != nil && profile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != "" {
				return profile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey]
			}
			if profile.KubeletDiskType == datamodel.TempDisk {
				return datamodel.TempDiskContainerDataDir
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey]
		},
		"HasKubeletDiskType": func() bool {
			return profile != nil && profile.KubeletDiskType != "" && profile.KubeletDiskType != datamodel.OSDisk
		},
		"GetKubeletDiskType": func() string {
			if profile != nil && profile.KubeletDiskType != "" && profile.KubeletDiskType != datamodel.OSDisk {
				return string(profile.KubeletDiskType)
			}
			return ""
		},
		"IsKrustlet": func() bool {
			return strings.EqualFold(string(profile.WorkloadRuntime), string(datamodel.WasmWasi))
		},
		"GetBase64CertificateAuthorityData": func() string {
			if cs != nil && cs.Properties != nil && cs.Properties.CertificateProfile != nil && cs.Properties.CertificateProfile.CaCertificate != "" {
				data := cs.Properties.CertificateProfile.CaCertificate
				return base64.StdEncoding.EncodeToString([]byte(data))
			}
			return ""
		},
		"TeleportEnabled": func() bool {
			return config.EnableACRTeleportPlugin
		},
		"HasDCSeriesSKU": func() bool {
			return cs.Properties.HasDCSeriesSKU()
		},
		"GetHyperkubeImageReference": func() string {
			return config.K8sComponents.HyperkubeImageURL
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
		"GetCSEHelpersScriptFilepath": func() string {
			return cseHelpersScriptFilepath
		},
		"GetCSEHelpersScriptDistroFilepath": func() string {
			return cseHelpersScriptDistroFilepath
		},
		"GetCSEInstallScriptFilepath": func() string {
			return cseInstallScriptFilepath
		},
		"GetCSEInstallScriptDistroFilepath": func() string {
			return cseInstallScriptDistroFilepath
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
		"OpenBraces": func() string {
			return "{{"
		},
		"CloseBraces": func() string {
			return "}}"
		},
		"BoolPtrToInt": func(p *bool) int {
			if p == nil {
				return 0
			}
			if v := *p; v {
				return 1
			}
			return 0
		},
		"UserAssignedIDEnabled": func() bool {
			// TODO(qinhao): we need to move this to NodeBootstrappingConfiguration as cs.Properties
			//               is to be moved away from NodeBootstrappingConfiguration
			return cs.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedIDEnabled()
		},
		// HTTP proxy related funcs
		"ShouldConfigureHTTPProxy": func() bool {
			return config.HTTPProxyConfig != nil && (config.HTTPProxyConfig.HTTPProxy != nil || config.HTTPProxyConfig.HTTPSProxy != nil)
		},
		"HasHTTPProxy": func() bool {
			return config.HTTPProxyConfig != nil && config.HTTPProxyConfig.HTTPProxy != nil
		},
		"HasHTTPSProxy": func() bool {
			return config.HTTPProxyConfig != nil && config.HTTPProxyConfig.HTTPSProxy != nil
		},
		"HasNoProxy": func() bool {
			return config.HTTPProxyConfig != nil && config.HTTPProxyConfig.NoProxy != nil
		},
		"GetHTTPProxy": func() string {
			if config.HTTPProxyConfig != nil && config.HTTPProxyConfig.HTTPProxy != nil {
				return *config.HTTPProxyConfig.HTTPProxy
			}
			return ""
		},
		"GetHTTPSProxy": func() string {
			if config.HTTPProxyConfig != nil && config.HTTPProxyConfig.HTTPSProxy != nil {
				return *config.HTTPProxyConfig.HTTPSProxy
			}
			return ""
		},
		"GetNoProxy": func() string {
			if config.HTTPProxyConfig != nil && config.HTTPProxyConfig.NoProxy != nil {
				return strings.Join(*config.HTTPProxyConfig.NoProxy, ",")
			}
			return ""
		},
		"ShouldConfigureHTTPProxyCA": func() bool {
			return config.HTTPProxyConfig != nil && config.HTTPProxyConfig.TrustedCA != nil
		},
		"GetHTTPProxyCA": func() string {
			if config.HTTPProxyConfig != nil && config.HTTPProxyConfig.TrustedCA != nil {
				dec, _ := base64.StdEncoding.DecodeString(*config.HTTPProxyConfig.TrustedCA)
				return datamodel.IndentString(string(dec), 4)
			}
			return ""
		},
		"FIPSEnabled": func() bool {
			return config.FIPSEnabled
		},
		"GetMessageOfTheDay": func() string {
			return profile.MessageOfTheDay
		},
		"HasMessageOfTheDay": func() bool {
			return profile.MessageOfTheDay != ""
		},
		"GetOutboundCommand": func() string {
			return getOutBoundCmd(config, config.CloudSpecConfig)
		},
		"GPUDriverVersion": func() string {
			return getGPUDriverVersion(profile.VMSize)
		},
		"GetHnsRemediatorIntervalInMinutes": func() uint32 {
			if cs.Properties.WindowsProfile != nil {
				return cs.Properties.WindowsProfile.GetHnsRemediatorIntervalInMinutes()
			}
			return 0
		},
		"ShouldConfigureCustomCATrust": func() bool {
			return areCustomCATrustCertsPopulated(*config)
		},
		"GetCustomCATrustConfigCerts": func() []string {
			if areCustomCATrustCertsPopulated(*config) {
				return config.CustomCATrustConfig.CustomCATrustCerts
			}
			return []string{}
		},
	}
}

// NV series GPUs target graphics workloads vs NC which targets compute
// they typically use GRID, not CUDA drivers, and will fail to install CUDA drivers.
// NVv1 seems to run with CUDA, NVv5 requires GRID.
// NVv3 is untested on AKS, NVv4 is AMD so n/a, and NVv2 no longer seems to exist (?)
func getGPUDriverVersion(size string) string {
	if useGridDrivers(size) {
		return datamodel.Nvidia510GridDriverVersion
	}
	if isStandardNCv1(size) {
		return datamodel.Nvidia470CudaDriverVersion
	}
	return datamodel.Nvidia510CudaDriverVersion
}

func isStandardNCv1(size string) bool {
	tmp := strings.ToLower(size)
	return strings.HasPrefix(tmp, "standard_nc") && !strings.Contains(tmp, "_v")
}

func useGridDrivers(size string) bool {
	return datamodel.GridGPUSizes[strings.ToLower(size)]
}

func areCustomCATrustCertsPopulated(config datamodel.NodeBootstrappingConfiguration) bool {
	return config.CustomCATrustConfig != nil && len(config.CustomCATrustConfig.CustomCATrustCerts) > 0
}

func isMariner(osSku string) bool {
	return osSku == datamodel.OSSKUCBLMariner || osSku == datamodel.OSSKUMariner
}
