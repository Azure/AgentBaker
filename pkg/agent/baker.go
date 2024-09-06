// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/common"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

// TemplateGenerator represents the object that performs the template generation.
type TemplateGenerator struct{}

// InitializeTemplateGenerator creates a new template generator object.
func InitializeTemplateGenerator() *TemplateGenerator {
	t := &TemplateGenerator{}
	return t
}

// GetNodeBootstrappingPayload get node bootstrapping data.
// This function only can be called after the validation of the input NodeBootstrappingConfiguration.
func (t *TemplateGenerator) getNodeBootstrappingPayload(config *datamodel.NodeBootstrappingConfiguration) string {
	var customData string
	if config.AgentPoolProfile.IsWindows() {
		customData = getCustomDataFromJSON(t.getWindowsNodeCustomDataJSONObject(config))
	} else {
		customData = getCustomDataFromJSON(t.getLinuxNodeCustomDataJSONObject(config))
	}
	return base64.StdEncoding.EncodeToString([]byte(customData))
}

// GetLinuxNodeCustomDataJSONObject returns Linux customData JSON object in the form.
// { "customData": "<customData string>" }.
func (t *TemplateGenerator) getLinuxNodeCustomDataJSONObject(config *datamodel.NodeBootstrappingConfiguration) string {
	// get parameters
	parameters := getParameters(config)
	// get variable cloudInit
	variables := getCustomDataVariables(config)
	str, e := t.getSingleLineForTemplate(kubernetesNodeCustomDataYaml, config.AgentPoolProfile, getBakerFuncMap(config, parameters, variables), true)

	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("{\"customData\": \"%s\"}", str)
}

// GetWindowsNodeCustomDataJSONObject returns Windows customData JSON object in the form.
// { "customData": "<customData string>" }.
func (t *TemplateGenerator) getWindowsNodeCustomDataJSONObject(config *datamodel.NodeBootstrappingConfiguration) string {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	// get parameters
	parameters := getParameters(config)
	// get variable custom data
	variables := getWindowsCustomDataVariables(config)
	str, e := t.getSingleLineForTemplate(kubernetesWindowsAgentCustomDataPS1, profile, getBakerFuncMap(config, parameters, variables), false)

	if e != nil {
		panic(e)
	}

	preprovisionCmd := ""

	if profile.PreprovisionExtension != nil {
		preprovisionCmd = makeAgentExtensionScriptCommands(cs, profile)
	}

	str = strings.ReplaceAll(str, "PREPROVISION_EXTENSION", escapeSingleLine(strings.TrimSpace(preprovisionCmd)))
	return fmt.Sprintf("{\"customData\": \"%s\"}", str)
}

// GetNodeBootstrappingCmd get node bootstrapping cmd.
// This function only can be called after the validation of the input NodeBootstrappingConfiguration.
func (t *TemplateGenerator) getNodeBootstrappingCmd(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return t.getWindowsNodeCSECommand(config)
	}
	return t.getLinuxNodeCSECommand(config)
}

// getLinuxNodeCSECommand returns Linux node custom script extension execution command.
func (t *TemplateGenerator) getLinuxNodeCSECommand(config *datamodel.NodeBootstrappingConfiguration) string {
	// get parameters
	parameters := getParameters(config)
	// get variable
	variables := getCSECommandVariables(config)
	// NOTE: that CSE command will be executed by VM/VMSS extension so it doesn't need extra escaping like custom data does
	str, e := t.getSingleLine(
		kubernetesCSECommandString,
		config.AgentPoolProfile,
		getBakerFuncMap(config, parameters, variables),
		true,
	)

	if e != nil {
		panic(e)
	}
	// NOTE: we break the one-line CSE command into different lines in a file for better management
	// so we need to combine them into one line here
	return strings.ReplaceAll(str, "\n", " ")
}

// getWindowsNodeCSECommand returns Windows node custom script extension execution command.
func (t *TemplateGenerator) getWindowsNodeCSECommand(config *datamodel.NodeBootstrappingConfiguration) string {
	// get parameters
	parameters := getParameters(config)
	// get variable
	variables := getCSECommandVariables(config)

	// NOTE: that CSE command will be executed by VMSS extension so it doesn't need extra escaping like custom data does
	str, e := t.getSingleLine(
		kubernetesWindowsAgentCSECommandPS1,
		config.AgentPoolProfile,
		getBakerFuncMap(config, parameters, variables),
		false,
	)

	if e != nil {
		panic(e)
	}
	/* NOTE(qinahao): windows cse cmd uses esapced \" to quote Powershell command in
	[csecmd.p1](https://github.com/Azure/AgentBaker/blob/master/parts/windows/csecmd.ps1). */
	// to not break go template parsing. We switch \" back to " otherwise Azure ARM template will escape \ to be \\\"
	str = strings.ReplaceAll(str, `\"`, `"`)

	// NOTE: we break the one-line CSE command into different lines in a file for better management
	// so we need to combine them into one line here
	return strings.ReplaceAll(str, "\n", " ")
}

// getSingleLineForTemplate returns the file as a single line for embedding in an arm template.
func (t *TemplateGenerator) getSingleLineForTemplate(textFilename string, profile interface{}, funcMap template.FuncMap, isLinux bool) (string, error) {
	expandedTemplate, err := t.getSingleLine(textFilename, profile, funcMap, isLinux)
	if err != nil {
		return "", err
	}

	textStr := escapeSingleLine(expandedTemplate)

	return textStr, nil
}

// getSingleLine returns the file as a single line.
func (t *TemplateGenerator) getSingleLine(textFilename string, profile interface{}, funcMap template.FuncMap, isLinux bool) (string, error) {
	b, err := parts.Templates.ReadFile(textFilename)
	if err != nil {
		return "", fmt.Errorf("yaml file %s does not exist", textFilename)
	}
	if isLinux {
		b = removeComments(b)
	}

	// use go templates to process the text filename
	templ := template.New("customdata template").Option("missingkey=zero").Funcs(funcMap)
	if _, err = templ.New(textFilename).Parse(string(b)); err != nil {
		return "", fmt.Errorf("error parsing file %s: %w", textFilename, err)
	}

	var buffer bytes.Buffer
	if err = templ.ExecuteTemplate(&buffer, textFilename, profile); err != nil {
		return "", fmt.Errorf("error executing template for file %s: %w", textFilename, err)
	}
	expandedTemplate := buffer.String()

	return expandedTemplate, nil
}

// getTemplateFuncMap returns the general purpose template func map from getContainerServiceFuncMap.
func getBakerFuncMap(config *datamodel.NodeBootstrappingConfiguration, params paramsMap, variables paramsMap) template.FuncMap {
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

	// TODO: GetParameterPropertyLower
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

/* normalizeResourceGroupNameForLabel normalizes resource group name to be used as a label,
similar to what the ARM template used to do.

When ARM template was used, the following is used:

variables('labelResourceGroup')

which is defined as:

[if(or(or(endsWith(variables('truncatedResourceGroup'), '-'),
endsWith(variables('truncatedResourceGroup'), '_')),
endsWith(variables('truncatedResourceGroup'), '.')),
concat(take(variables('truncatedResourceGroup'), 62), 'z'), variables('truncatedResourceGroup'))]

the "truncatedResourceGroup" is defined as:

[take(replace(replace(resourceGroup().name, '(', '-'), ')', '-'), 63)]*/

// This function does the same processing.
func normalizeResourceGroupNameForLabel(resourceGroupName string) string {
	truncated := resourceGroupName
	truncated = strings.ReplaceAll(truncated, "(", "-")
	truncated = strings.ReplaceAll(truncated, ")", "-")
	const maxKubernetesLabelLength = 63
	if len(truncated) > maxKubernetesLabelLength {
		truncated = truncated[0:maxKubernetesLabelLength]
	}

	if strings.HasSuffix(truncated, "-") ||
		strings.HasSuffix(truncated, "_") ||
		strings.HasSuffix(truncated, ".") {
		if len(truncated) > maxKubernetesLabelLength-1 {
			return truncated[0:len(truncated)-1] + "z"
		}
		return truncated + "z"
	}
	return truncated
}

func validateAndSetLinuxNodeBootstrappingConfiguration(config *datamodel.NodeBootstrappingConfiguration) {
	if config.KubeletConfig == nil {
		return
	}
	profile := config.AgentPoolProfile
	kubeletFlags := config.KubeletConfig

	// If using kubelet config file, disable DynamicKubeletConfig feature gate and remove dynamic-config-dir
	// we should only allow users to configure from API (20201101 and later)
	delete(kubeletFlags, "--dynamic-config-dir")
	delete(kubeletFlags, "--non-masquerade-cidr")

	if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime == "containerd" {
		dockerShimFlags := []string{
			"--cni-bin-dir",
			"--cni-cache-dir",
			"--cni-conf-dir",
			"--docker-endpoint",
			"--image-pull-progress-deadline",
			"--network-plugin",
			"--network-plugin-mtu",
		}
		for _, flag := range dockerShimFlags {
			delete(kubeletFlags, flag)
		}
	}

	if IsKubeletServingCertificateRotationEnabled(config) {
		// ensure the required feature gate is set
		kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "RotateKubeletServerCertificate", true)
		// backfill deletion of --tls-cert-file and --tls-private-key-file, which are incompatible with --rotate-server-certificates
		// these are set as defaults on the RP-side for Linux
		delete(kubeletFlags, "--tls-cert-file")
		delete(kubeletFlags, "--tls-private-key-file")
	}

	if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.24.0") {
		kubeletFlags["--feature-gates"] = removeFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig")
	} else if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.11.0") {
		kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig", false)
	}

	/* ContainerInsights depends on GPU accelerator Usage metrics from Kubelet cAdvisor endpoint but
	deprecation of this feature moved to beta which breaks the ContainerInsights customers with K8s
		version 1.20 or higher */
	/* Until Container Insights move to new API adding this feature gate to get the GPU metrics
	continue to work */
	/* Reference -
	https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1867-disable-accelerator-usage-metrics */
	if IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.20.0") &&
		!IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, "1.25.0") {
		kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DisableAcceleratorUsageMetrics", false)
	}
}

func validateAndSetWindowsNodeBootstrappingConfiguration(config *datamodel.NodeBootstrappingConfiguration) {
	if IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken) {
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

		if IsKubeletServingCertificateRotationEnabled(config) {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "RotateKubeletServerCertificate", true)
			// RP doesn't currently set these flags for windows, though we filter them out anyways just to be safe
			delete(kubeletFlags, "--tls-cert-file")
			delete(kubeletFlags, "--tls-private-key-file")
		}
	}
}

// getContainerServiceFuncMap returns all functions used in template generation.
/* These funcs are a thin wrapper for template generation operations,
all business logic is implemented in the underlying func. */
//nolint:gocognit, funlen, cyclop, gocyclo
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
			return getAgentKubernetesLabels(profile, config)
		},
		"GetAgentKubernetesLabelsDeprecated": func(profile *datamodel.AgentPoolProfile) string {
			return profile.GetKubernetesLabels()
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
		"GetKubeletConfigFileContentBase64": func() string {
			return base64.StdEncoding.EncodeToString([]byte(GetKubeletConfigFileContent(config.KubeletConfig, profile.CustomKubeletConfig)))
		},
		"IsKubeletConfigFileEnabled": func() bool {
			return IsKubeletConfigFileEnabled(cs, profile, config.EnableKubeletConfigFile)
		},
		"EnableTLSBootstrapping": func() bool {
			// this will be true when we get a hard-coded TLS bootstrap token in the NodeBootstrappingConfiguration to use for performing TLS bootstrapping.
			return IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken)
		},
		"EnableSecureTLSBootstrapping": func() bool {
			// this will be true when we can perform TLS bootstrapping without the use of a hard-coded bootstrap token.
			return config.EnableSecureTLSBootstrapping
		},
		"GetCustomSecureTLSBootstrapAADServerAppID": func() string {
			return config.CustomSecureTLSBootstrapAADServerAppID
		},
		"GetTLSBootstrapTokenForKubeConfig": func() string {
			return GetTLSBootstrapTokenForKubeConfig(config.KubeletClientTLSBootstrapToken)
		},
		"EnableKubeletServingCertificateRotation": func() bool {
			return IsKubeletServingCertificateRotationEnabled(config)
		},
		"GetKubeletConfigKeyVals": func() string {
			return GetOrderedKubeletConfigFlagString(config.KubeletConfig, cs, profile, config.EnableKubeletConfigFile)
		},
		"GetKubeletConfigKeyValsPsh": func() string {
			return config.GetOrderedKubeletConfigStringForPowershell(profile.CustomKubeletConfig)
		},
		"GetKubeproxyConfigKeyValsPsh": func() string {
			return config.GetOrderedKubeproxyConfigStringForPowershell()
		},
		"IsCgroupV2": func() bool {
			return profile.Is2204VHDDistro() || profile.IsAzureLinuxCgroupV2VHDDistro() || profile.Is2404VHDDistro()
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
			return profile.CustomLinuxOSConfig != nil && (profile.CustomLinuxOSConfig.TransparentHugePageEnabled != "" ||
				profile.CustomLinuxOSConfig.TransparentHugePageDefrag != "")
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
			if profile.CustomLinuxOSConfig != nil && profile.CustomLinuxOSConfig.SwapFileSizeMB != nil {
				return *profile.CustomLinuxOSConfig.SwapFileSizeMB
			}
			return 0
		},
		"ShouldConfigContainerdUlimits": func() bool {
			return profile.GetCustomLinuxOSConfig().GetUlimitConfig() != nil
		},
		"GetContainerdUlimitString": func() string {
			ulimitConfig := profile.GetCustomLinuxOSConfig().GetUlimitConfig()
			if ulimitConfig == nil {
				return ""
			}
			var sb strings.Builder
			sb.WriteString("[Service]\n")
			if ulimitConfig.MaxLockedMemory != "" {
				sb.WriteString(fmt.Sprintf("LimitMEMLOCK=%s\n", ulimitConfig.MaxLockedMemory))
			}
			if ulimitConfig.NoFile != "" {
				sb.WriteString(fmt.Sprintf("LimitNOFILE=%s\n", ulimitConfig.NoFile))
			}
			return sb.String()
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
			return profile.Distro.IsAzureLinuxDistro() || isMariner(config.OSSKU)
		},
		"IsKata": func() bool {
			return profile.Distro.IsKataDistro()
		},
		"IsCustomImage": func() bool {
			return profile.Distro == datamodel.CustomizedImage || profile.Distro == datamodel.CustomizedImageKata
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
			neededParts := []string{
				kubernetesWindowsCSEHelperPS1,
				kubernetesWindowsSendLogsPS1,
			}

			// Create a buffer, new zip
			buf := new(bytes.Buffer)
			zw := zip.NewWriter(buf)

			for _, part := range neededParts {
				f, err := zw.Create(part)
				if err != nil {
					panic(err)
				}
				partContents, err := parts.Templates.ReadFile(part)
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
				encoded := base64.StdEncoding.EncodeToString([]byte(cs.Properties.CertificateProfile.ClientPrivateKey))
				return encoded
			}
			return ""
		},
		"GetKubeletClientCert": func() string {
			if cs.Properties.CertificateProfile != nil && cs.Properties.CertificateProfile.ClientCertificate != "" {
				encoded := base64.StdEncoding.EncodeToString([]byte(cs.Properties.CertificateProfile.ClientCertificate))
				return encoded
			}
			return ""
		},
		"HasServicePrincipalSecret": func() bool {
			return cs.Properties.ServicePrincipalProfile != nil && cs.Properties.ServicePrincipalProfile.Secret != ""
		},
		"GetServicePrincipalSecret": func() string {
			if cs.Properties.ServicePrincipalProfile != nil && cs.Properties.ServicePrincipalProfile.Secret != "" {
				encoded := base64.StdEncoding.EncodeToString([]byte(cs.Properties.ServicePrincipalProfile.Secret))
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
		"IsAzureCNIOverlayFeatureEnabled": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.IsUsingNetworkPluginMode("overlay")
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
			if profile != nil && profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntimeConfig != nil &&
				profile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != "" {
				return true
			}
			if profile.KubeletDiskType == datamodel.TempDisk {
				return true
			}
			return cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig != nil &&
				cs.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != ""
		},
		"GetDataDir": func() string {
			if profile != nil && profile.KubernetesConfig != nil &&
				profile.KubernetesConfig.ContainerRuntimeConfig != nil &&
				profile.KubernetesConfig.ContainerRuntimeConfig[datamodel.ContainerDataDirKey] != "" {
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
		"GetKubenetTemplate": func() string {
			return base64.StdEncoding.EncodeToString([]byte(kubenetCniTemplate))
		},
		"GetContainerdConfigContent": func() string {
			output, err := containerdConfigFromTemplate(config, profile, containerdConfigTemplateString)
			if err != nil {
				panic(err)
			}
			return output
		},
		"GetContainerdConfigNoGPUContent": func() string {
			output, err := containerdConfigFromTemplate(config, profile, containerdConfigNoGpuTemplateString)
			if err != nil {
				panic(err)
			}
			return output
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
		"GetLinuxPrivatePackageURL": func() string {
			return config.K8sComponents.LinuxPrivatePackageURL
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
				return *config.HTTPProxyConfig.TrustedCA
			}
			return ""
		},
		"FIPSEnabled": func() bool {
			return config.FIPSEnabled
		},
		"GetMessageOfTheDay": func() string {
			return profile.MessageOfTheDay
		},
		"GetProxyVariables": func() string {
			return getProxyVariables(config)
		},
		"GetOutboundCommand": func() string {
			return getOutBoundCmd(config, config.CloudSpecConfig)
		},
		"BlockOutboundNetwork": func() bool {
			if config.OutboundType == datamodel.OutboundTypeBlock || config.OutboundType == datamodel.OutboundTypeNone {
				return true
			}
			return false
		},
		"GPUNeedsFabricManager": func() bool {
			return common.GPUNeedsFabricManager(profile.VMSize)
		},
		"GPUDriverVersion": func() string {
			return common.GetGPUDriverVersion(profile.VMSize)
		},
		"GPUImageSHA": func() string {
			return common.GetAKSGPUImageSHA(profile.VMSize)
		},
		"GetHnsRemediatorIntervalInMinutes": func() uint32 {
			// Only need to enable HNSRemediator for Windows 2019
			if cs.Properties.WindowsProfile != nil && profile.Distro == datamodel.AKSWindows2019Containerd {
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
		"GetLogGeneratorIntervalInMinutes": func() uint32 {
			if cs.Properties.WindowsProfile != nil {
				return cs.Properties.WindowsProfile.GetLogGeneratorIntervalInMinutes()
			}
			return 0
		},
		"ShouldDisableSSH": func() bool {
			return config.SSHStatus == datamodel.SSHOff
		},
		"GetSysctlContent": func() (string, error) {
			templateFuncMap := make(template.FuncMap)
			templateFuncMap["getPortRangeEndValue"] = getPortRangeEndValue
			sysctlTemplate, err := template.New("sysctl").Funcs(templateFuncMap).Parse(sysctlTemplateString)
			if err != nil {
				return "", fmt.Errorf("failed to parse sysctl template: %w", err)
			}

			var b bytes.Buffer
			if err = sysctlTemplate.Execute(&b, profile); err != nil {
				return "", fmt.Errorf("failed to execute sysctl template: %w", err)
			}
			return base64.StdEncoding.EncodeToString(b.Bytes()), nil
		},
		"ShouldEnableCustomData": func() bool {
			return !config.DisableCustomData
		},
		"GetPrivateEgressProxyAddress": func() string {
			return config.ContainerService.Properties.SecurityProfile.GetProxyAddress()
		},
		"GetBootstrapProfileContainerRegistryServer": func() string {
			return config.ContainerService.Properties.SecurityProfile.GetPrivateEgressContainerRegistryServer()
		},
		"IsArtifactStreamingEnabled": func() bool {
			return config.EnableArtifactStreaming
		},
		"EnableIMDSRestriction": func() bool {
			return config.EnableIMDSRestriction
		},
		"InsertIMDSRestrictionRuleToMangleTable": func() bool {
			return config.InsertIMDSRestrictionRuleToMangleTable
		},
	}
}

func getPortRangeEndValue(portRange string) int {
	arr := strings.Split(portRange, " ")
	num, err := strconv.Atoi(arr[1])
	if err != nil {
		return -1
	}
	return num
}

func areCustomCATrustCertsPopulated(config datamodel.NodeBootstrappingConfiguration) bool {
	return config.CustomCATrustConfig != nil && len(config.CustomCATrustConfig.CustomCATrustCerts) > 0
}

func isMariner(osSku string) bool {
	return osSku == datamodel.OSSKUCBLMariner || osSku == datamodel.OSSKUMariner || osSku == datamodel.OSSKUAzureLinux
}

const sysctlTemplateString = `# This is a partial workaround to this upstream Kubernetes issue:
# https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
net.ipv4.tcp_retries2=8
net.core.message_burst=80
net.core.message_cost=40
{{- if .CustomLinuxOSConfig}}
{{- if .CustomLinuxOSConfig.Sysctls}}
{{- if .CustomLinuxOSConfig.Sysctls.NetCoreSomaxconn}}
net.core.somaxconn={{.CustomLinuxOSConfig.Sysctls.NetCoreSomaxconn}}
{{- else}}
net.core.somaxconn=16384
{{- end}}
{{- if .CustomLinuxOSConfig.Sysctls.NetIpv4TcpMaxSynBacklog}}
net.ipv4.tcp_max_syn_backlog={{.CustomLinuxOSConfig.Sysctls.NetIpv4TcpMaxSynBacklog}}
{{- else}}
net.ipv4.tcp_max_syn_backlog=16384
{{- end}}
{{- if .CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh1}}
net.ipv4.neigh.default.gc_thresh1={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh1}}
{{- else}}
net.ipv4.neigh.default.gc_thresh1=4096
{{- end}}
{{- if .CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh2}}
net.ipv4.neigh.default.gc_thresh2={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh2}}
{{- else}}
net.ipv4.neigh.default.gc_thresh2=8192
{{- end}}
{{- if .CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh3}}
net.ipv4.neigh.default.gc_thresh3={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh3}}
{{- else}}
net.ipv4.neigh.default.gc_thresh3=16384
{{- end}}
{{- else}}
net.core.somaxconn=16384
net.ipv4.tcp_max_syn_backlog=16384
net.ipv4.neigh.default.gc_thresh1=4096
net.ipv4.neigh.default.gc_thresh2=8192
net.ipv4.neigh.default.gc_thresh3=16384
{{- end}}
{{- else}}
net.core.somaxconn=16384
net.ipv4.tcp_max_syn_backlog=16384
net.ipv4.neigh.default.gc_thresh1=4096
net.ipv4.neigh.default.gc_thresh2=8192
net.ipv4.neigh.default.gc_thresh3=16384
{{- end}}
{{- if .CustomLinuxOSConfig}}
{{- if .CustomLinuxOSConfig.Sysctls}}
# The following are sysctl configs passed from API
{{- $s:=.CustomLinuxOSConfig.Sysctls}}
{{- if $s.NetCoreNetdevMaxBacklog}}
net.core.netdev_max_backlog={{$s.NetCoreNetdevMaxBacklog}}
{{- end}}
{{- if $s.NetCoreRmemDefault}}
net.core.rmem_default={{$s.NetCoreRmemDefault}}
{{- end}}
{{- if $s.NetCoreRmemMax}}
net.core.rmem_max={{$s.NetCoreRmemMax}}
{{- end}}
{{- if $s.NetCoreWmemDefault}}
net.core.wmem_default={{$s.NetCoreWmemDefault}}
{{- end}}
{{- if $s.NetCoreWmemMax}}
net.core.wmem_max={{$s.NetCoreWmemMax}}
{{- end}}
{{- if $s.NetCoreOptmemMax}}
net.core.optmem_max={{$s.NetCoreOptmemMax}}
{{- end}}
{{- if $s.NetIpv4TcpMaxTwBuckets}}
net.ipv4.tcp_max_tw_buckets={{$s.NetIpv4TcpMaxTwBuckets}}
{{- end}}
{{- if $s.NetIpv4TcpFinTimeout}}
net.ipv4.tcp_fin_timeout={{$s.NetIpv4TcpFinTimeout}}
{{- end}}
{{- if $s.NetIpv4TcpKeepaliveTime}}
net.ipv4.tcp_keepalive_time={{$s.NetIpv4TcpKeepaliveTime}}
{{- end}}
{{- if $s.NetIpv4TcpKeepaliveProbes}}
net.ipv4.tcp_keepalive_probes={{$s.NetIpv4TcpKeepaliveProbes}}
{{- end}}
{{- if $s.NetIpv4TcpkeepaliveIntvl}}
net.ipv4.tcp_keepalive_intvl={{$s.NetIpv4TcpkeepaliveIntvl}}
{{- end}}
{{- if $s.NetIpv4TcpTwReuse}}
net.ipv4.tcp_tw_reuse={{if $s.NetIpv4TcpTwReuse}}1{{else}}0{{end}}
{{- end}}
{{- if $s.NetIpv4IpLocalPortRange}}
net.ipv4.ip_local_port_range={{$s.NetIpv4IpLocalPortRange}}
{{$rangeEnd := getPortRangeEndValue $s.NetIpv4IpLocalPortRange}}
{{ if ge $rangeEnd 65330}}
net.ipv4.ip_local_reserved_ports=65330
{{- end}}
{{- end}}
{{- if $s.NetNetfilterNfConntrackMax}}
net.netfilter.nf_conntrack_max={{$s.NetNetfilterNfConntrackMax}}
{{- end}}
{{- if $s.NetNetfilterNfConntrackBuckets}}
net.netfilter.nf_conntrack_buckets={{$s.NetNetfilterNfConntrackBuckets}}
{{- end}}
{{- if $s.FsInotifyMaxUserWatches}}
fs.inotify.max_user_watches={{$s.FsInotifyMaxUserWatches}}
{{- end}}
{{- if $s.FsFileMax}}
fs.file-max={{$s.FsFileMax}}
{{- end}}
{{- if $s.FsAioMaxNr}}
fs.aio-max-nr={{$s.FsAioMaxNr}}
{{- end}}
{{- if $s.FsNrOpen}}
fs.nr_open={{$s.FsNrOpen}}
{{- end}}
{{- if $s.KernelThreadsMax}}
kernel.threads-max={{$s.KernelThreadsMax}}
{{- end}}
{{- if $s.VMMaxMapCount}}
vm.max_map_count={{$s.VMMaxMapCount}}
{{- end}}
{{- if $s.VMSwappiness}}
vm.swappiness={{$s.VMSwappiness}}
{{- end}}
{{- if $s.VMVfsCachePressure}}
vm.vfs_cache_pressure={{$s.VMVfsCachePressure}}
{{- end}}
{{- end}}
{{- end}}
`

const kubenetCniTemplate = `
{
    "cniVersion": "0.3.1",
    "name": "kubenet",
    "plugins": [{
    "type": "bridge",
    "bridge": "cbr0",
    "mtu": 1500,
    "addIf": "eth0",
    "isGateway": true,
    "ipMasq": false,
    "promiscMode": true,
    "hairpinMode": false,
    "ipam": {
        "type": "host-local",
        "ranges": [{{range $i, $range := .PodCIDRRanges}}{{if $i}}, {{end}}[{"subnet": "{{$range}}"}]{{end}}],
        "routes": [{{range $i, $route := .Routes}}{{if $i}}, {{end}}{"dst": "{{$route}}"}{{end}}]
    }
    },
    {
    "type": "portmap",
    "capabilities": {"portMappings": true},
    "externalSetMarkChain": "KUBE-MARK-MASQ"
    }]
}
`

const containerdConfigTemplateString = `version = 2
oom_score = 0{{if HasDataDir }}
root = "{{GetDataDir}}"{{- end}}
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "{{GetPodInfraContainerSpec}}"
  [plugins."io.containerd.grpc.v1.cri".containerd]
    {{- if TeleportEnabled }}
    snapshotter = "teleportd"
    disable_snapshot_annotations = false
    {{- else}}
      {{- if IsKata }}
      disable_snapshot_annotations = false
      {{- end}}
    {{- end}}
    {{- if IsArtifactStreamingEnabled }}
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    {{- end}}
    {{- if IsNSeriesSKU }}
    default_runtime_name = "nvidia-container-runtime"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime.options]
      BinaryName = "/usr/bin/nvidia-container-runtime"
      {{- if IsCgroupV2 }}
      SystemdCgroup = true
      {{- end}}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/nvidia-container-runtime"
    {{- else}}
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      {{- if IsCgroupV2 }}
      SystemdCgroup = true
      {{- end}}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
    {{- end}}
    {{- if IsKrustlet }}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-3-0]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-3-0]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-5-1]
      runtime_type = "io.containerd.spin-v0-5-1.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-5-1]
      runtime_type = "io.containerd.slight-v0-5-1.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-8-0]
      runtime_type = "io.containerd.spin-v0-8-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-8-0]
      runtime_type = "io.containerd.slight-v0-8-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.wws-v0-8-0]
      runtime_type = "io.containerd.wws-v0-8-0.v1"
    {{- end}}
  {{- if and (IsKubenet) (not HasCalicoNetworkPolicy) }}
  [plugins."io.containerd.grpc.v1.cri".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
  {{- end}}
  {{- if IsKubernetesVersionGe "1.22.0"}}
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  {{- end}}
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
{{- if TeleportEnabled }}
[proxy_plugins]
  [proxy_plugins.teleportd]
    type = "snapshot"
    address = "/run/teleportd/snapshotter.sock"
{{- end}}
{{- if IsArtifactStreamingEnabled }}
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
{{- end}}
{{- if IsKata }}
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli]
  runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli.options]
  NoPivotRoot = false
  NoNewKeyring = false
  ShimCgroup = ""
  IoUid = 0
  IoGid = 0
  BinaryName = "/usr/bin/kata-runtime"
  Root = ""
  CriuPath = ""
  SystemdCgroup = false
[proxy_plugins]
  [proxy_plugins.tardev]
    type = "snapshot"
    address = "/run/containerd/tardev-snapshotter.sock"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc]
  snapshotter = "tardev"
  runtime_type = "io.containerd.kata-cc.v2"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/confidential-containers/share/defaults/kata-containers/configuration-clh-snp.toml"
{{- end}}
`

// this pains me, but to make it respect mutability of vmss tags,
// we cannot use go templates at runtime.
// CSE needs to be able to generate the full config, with all params,
// with the tags pulled from wireserver. this is a hack to avoid
// moving all the go templates to CSE -- we allow two options,
// duplicate them in CSE base64-encoded, and pick the right one.
// they're identical except for GPU runtime class.
const containerdConfigNoGpuTemplateString = `version = 2
oom_score = 0{{if HasDataDir }}
root = "{{GetDataDir}}"{{- end}}
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "{{GetPodInfraContainerSpec}}"
  [plugins."io.containerd.grpc.v1.cri".containerd]
    {{- if TeleportEnabled }}
    snapshotter = "teleportd"
    disable_snapshot_annotations = false
    {{- else}}
      {{- if IsKata }}
      disable_snapshot_annotations = false
      {{- end}}
    {{- end}}
    {{- if IsArtifactStreamingEnabled }}
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    {{- end}}
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      {{- if IsCgroupV2 }}
      SystemdCgroup = true
      {{- end}}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
    {{- if IsKrustlet }}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-3-0]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-3-0]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-5-1]
      runtime_type = "io.containerd.spin-v0-5-1.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-5-1]
      runtime_type = "io.containerd.slight-v0-5-1.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-8-0]
      runtime_type = "io.containerd.spin-v0-8-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-8-0]
      runtime_type = "io.containerd.slight-v0-8-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.wws-v0-8-0]
      runtime_type = "io.containerd.wws-v0-8-0.v1"
    {{- end}}
  {{- if and (IsKubenet) (not HasCalicoNetworkPolicy) }}
  [plugins."io.containerd.grpc.v1.cri".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
  {{- end}}
  {{- if IsKubernetesVersionGe "1.22.0"}}
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  {{- end}}
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
{{- if TeleportEnabled }}
[proxy_plugins]
  [proxy_plugins.teleportd]
    type = "snapshot"
    address = "/run/teleportd/snapshotter.sock"
{{- end}}
{{- if IsArtifactStreamingEnabled }}
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
{{- end}}
{{- if IsKata }}
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli]
  runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli.options]
  NoPivotRoot = false
  NoNewKeyring = false
  ShimCgroup = ""
  IoUid = 0
  IoGid = 0
  BinaryName = "/usr/bin/kata-runtime"
  Root = ""
  CriuPath = ""
  SystemdCgroup = false
[proxy_plugins]
  [proxy_plugins.tardev]
    type = "snapshot"
    address = "/run/containerd/tardev-snapshotter.sock"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc]
  snapshotter = "tardev"
  runtime_type = "io.containerd.kata-cc.v2"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/confidential-containers/share/defaults/kata-containers/configuration-clh-snp.toml"
{{- end}}
`

func containerdConfigFromTemplate(
	config *datamodel.NodeBootstrappingConfiguration,
	profile *datamodel.AgentPoolProfile,
	tmpl string,
) (string, error) {
	parameters := getParameters(config)
	variables := getCustomDataVariables(config)
	bakerFuncMap := getBakerFuncMap(config, parameters, variables)
	containerdConfigTemplate := template.Must(template.New("kubenet").Funcs(bakerFuncMap).Parse(tmpl))
	var b bytes.Buffer
	if err := containerdConfigTemplate.Execute(&b, profile); err != nil {
		return "", fmt.Errorf("failed to execute sysctl template: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}
