// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/templates"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/blang/semver"
)

// TranslatedKubeletConfigFlags represents kubelet flags that will be translated into config file (if kubelet config file is enabled)
var TranslatedKubeletConfigFlags map[string]bool = map[string]bool{
	"--address":                           true,
	"--anonymous-auth":                    true,
	"--client-ca-file":                    true,
	"--authentication-token-webhook":      true,
	"--authorization-mode":                true,
	"--pod-manifest-path":                 true,
	"--cluster-dns":                       true,
	"--cgroups-per-qos":                   true,
	"--tls-cert-file":                     true,
	"--tls-private-key-file":              true,
	"--tls-cipher-suites":                 true,
	"--cluster-domain":                    true,
	"--max-pods":                          true,
	"--eviction-hard":                     true,
	"--node-status-update-frequency":      true,
	"--node-status-report-frequency":      true,
	"--image-gc-high-threshold":           true,
	"--image-gc-low-threshold":            true,
	"--event-qps":                         true,
	"--pod-max-pids":                      true,
	"--enforce-node-allocatable":          true,
	"--streaming-connection-idle-timeout": true,
	"--rotate-certificates":               true,
	"--rotate-server-certificates":        true,
	"--read-only-port":                    true,
	"--feature-gates":                     true,
	"--protect-kernel-defaults":           true,
	"--resolv-conf":                       true,
	"--system-reserved":                   true,
	"--kube-reserved":                     true,
	"--cpu-manager-policy":                true,
	"--cpu-cfs-quota":                     true,
	"--cpu-cfs-quota-period":              true,
	"--topology-manager-policy":           true,
	"--allowed-unsafe-sysctls":            true,
	"--fail-swap-on":                      true,
	"--container-log-max-size":            true,
	"--container-log-max-files":           true,
}

var keyvaultSecretPathRe *regexp.Regexp

func init() {
	keyvaultSecretPathRe = regexp.MustCompile(`^(/subscriptions/\S+/resourceGroups/\S+/providers/Microsoft.KeyVault/vaults/\S+)/secrets/([^/\s]+)(/(\S+))?$`)
}

type paramsMap map[string]interface{}

func addValue(m paramsMap, k string, v interface{}) {
	m[k] = paramsMap{
		"value": v,
	}
}

func addKeyvaultReference(m paramsMap, k string, vaultID, secretName, secretVersion string) {
	m[k] = paramsMap{
		"reference": &datamodel.KeyVaultRef{
			KeyVault: datamodel.KeyVaultID{
				ID: vaultID,
			},
			SecretName:    secretName,
			SecretVersion: secretVersion,
		},
	}
}

func addSecret(m paramsMap, k string, v interface{}, encode bool) {
	str, ok := v.(string)
	if !ok {
		addValue(m, k, v)
		return
	}
	parts := keyvaultSecretPathRe.FindStringSubmatch(str)
	if parts == nil || len(parts) != 5 {
		if encode {
			addValue(m, k, base64.StdEncoding.EncodeToString([]byte(str)))
		} else {
			addValue(m, k, str)
		}
		return
	}
	addKeyvaultReference(m, k, parts[1], parts[2], parts[4])
}

func makeAgentExtensionScriptCommands(cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile) string {
	if profile.OSType == datamodel.Windows {
		return makeWindowsExtensionScriptCommands(profile.PreprovisionExtension,
			cs.Properties.ExtensionProfiles)
	}
	return makeExtensionScriptCommands(profile.PreprovisionExtension,
		"", cs.Properties.ExtensionProfiles)
}

func makeExtensionScriptCommands(extension *datamodel.Extension, curlCaCertOpt string, extensionProfiles []*datamodel.ExtensionProfile) string {
	var extensionProfile *datamodel.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	extensionsParameterReference := fmt.Sprintf("parameters('%sParameters')", extensionProfile.Name)
	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFilePath := fmt.Sprintf("/opt/azure/containers/extensions/%s/%s", extensionProfile.Name, extensionProfile.Script)
	return fmt.Sprintf("- sudo /usr/bin/curl --retry 5 --retry-delay 10 --retry-max-time 30 -o %s --create-dirs %s \"%s\" \n- sudo /bin/chmod 744 %s \n- sudo %s ',%s,' > /var/log/%s-output.log",
		scriptFilePath, curlCaCertOpt, scriptURL, scriptFilePath, scriptFilePath, extensionsParameterReference, extensionProfile.Name)
}

func makeWindowsExtensionScriptCommands(extension *datamodel.Extension, extensionProfiles []*datamodel.ExtensionProfile) string {
	var extensionProfile *datamodel.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFileDir := fmt.Sprintf("$env:SystemDrive:/AzureData/extensions/%s", extensionProfile.Name)
	scriptFilePath := fmt.Sprintf("%s/%s", scriptFileDir, extensionProfile.Script)
	return fmt.Sprintf("New-Item -ItemType Directory -Force -Path \"%s\" ; curl.exe --retry 5 --retry-delay 0 -L \"%s\" -o \"%s\" ; powershell \"%s `\"',parameters('%sParameters'),'`\"\"\n", scriptFileDir, scriptURL, scriptFilePath, scriptFilePath, extensionProfile.Name)
}

func escapeSingleLine(escapedStr string) string {
	// template.JSEscapeString leaves undesirable chars that don't work with pretty print
	escapedStr = strings.Replace(escapedStr, "\\", "\\\\", -1)
	escapedStr = strings.Replace(escapedStr, "\r\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\"", "\\\"", -1)
	return escapedStr
}

// getBase64EncodedGzippedCustomScript will return a base64 of the CSE
func getBase64EncodedGzippedCustomScript(csFilename string, config *datamodel.NodeBootstrappingConfiguration) string {
	b, err := templates.Asset(csFilename)
	if err != nil {
		// this should never happen and this is a bug
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	// translate the parameters
	templ := template.New("ContainerService template").Option("missingkey=error").Funcs(getContainerServiceFuncMap(config))
	_, err = templ.Parse(string(b))
	if err != nil {
		// this should never happen and this is a bug
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	var buffer bytes.Buffer
	templ.Execute(&buffer, config.ContainerService)
	csStr := buffer.String()
	csStr = strings.Replace(csStr, "\r\n", "\n", -1)
	return getBase64EncodedGzippedCustomScriptFromStr(csStr)
}

// getBase64EncodedGzippedCustomScriptFromStr will return a base64-encoded string of the gzip'd source data
func getBase64EncodedGzippedCustomScriptFromStr(str string) string {
	var gzipB bytes.Buffer
	w := gzip.NewWriter(&gzipB)
	w.Write([]byte(str))
	w.Close()
	return base64.StdEncoding.EncodeToString(gzipB.Bytes())
}

func getExtensionURL(rootURL, extensionName, version, fileName, query string) string {
	extensionsDir := "extensions"
	url := rootURL + extensionsDir + "/" + extensionName + "/" + version + "/" + fileName
	if query != "" {
		url += "?" + query
	}
	return url
}

func getSSHPublicKeysPowerShell(linuxProfile *datamodel.LinuxProfile) string {
	str := ""
	if linuxProfile != nil {
		lastItem := len(linuxProfile.SSH.PublicKeys) - 1
		for i, publicKey := range linuxProfile.SSH.PublicKeys {
			str += `"` + strings.TrimSpace(publicKey.KeyData) + `"`
			if i < lastItem {
				str += ", "
			}
		}
	}
	return str
}

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support
func IsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case "Standard_DC2s", "Standard_DC4s":
		return true
	}
	return false
}

// GetCloudTargetEnv determines and returns whether the region is a sovereign cloud which
// have their own data compliance regulations (China/Germany/USGov) or standard
// Azure public cloud
func GetCloudTargetEnv(location string) string {
	loc := strings.ToLower(strings.Join(strings.Fields(location), ""))
	switch {
	case strings.HasPrefix(loc, "china"):
		return "AzureChinaCloud"
	case loc == "germanynortheast" || loc == "germanycentral":
		return "AzureGermanCloud"
	case strings.HasPrefix(loc, "usgov") || strings.HasPrefix(loc, "usdod"):
		return "AzureUSGovernmentCloud"
	default:
		return "AzurePublicCloud"
	}
}

// IsKubernetesVersionGe returns true if actualVersion is greater than or equal to version
func IsKubernetesVersionGe(actualVersion, version string) bool {
	v1, _ := semver.Make(actualVersion)
	v2, _ := semver.Make(version)
	return v1.GE(v2)
}

func getCustomDataFromJSON(jsonStr string) string {
	var customDataObj map[string]string
	err := json.Unmarshal([]byte(jsonStr), &customDataObj)
	if err != nil {
		panic(err)
	}
	return customDataObj["customData"]
}

// GetOrderedKubeletConfigFlagString returns an ordered string of key/val pairs
// copied from AKS-Engine and filter out flags that already translated to config file
func GetOrderedKubeletConfigFlagString(k map[string]string, cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile, kubeletConfigFileToggleEnabled bool) string {
	// NOTE(mainred): kubeConfigFile now relies on CustomKubeletConfig, while custom configuration is not compatible
	// with CustomKubeletConfig. When custom configuration is set we want to override every configuration with the
	// customized one.
	kubeletCustomConfigurations := getKubeletCustomConfiguration(cs.Properties)
	if kubeletCustomConfigurations != nil {
		return getOrderedKubeletConfigFlagWithCustomConfigurationString(kubeletCustomConfigurations, k)
	}

	if k == nil {
		return ""
	}
	// Always force remove of dynamic-config-dir.
	kubeletConfigFileEnabled := IsKubeletConfigFileEnabled(cs, profile, kubeletConfigFileToggleEnabled)
	keys := []string{}
	for key := range k {
		if !kubeletConfigFileEnabled || !TranslatedKubeletConfigFlags[key] {
			if !datamodel.CommandLineOmittedKubeletConfigFlags[key] {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("%s=%s ", key, k[key]))
	}
	return buf.String()
}

func getOrderedKubeletConfigFlagWithCustomConfigurationString(customConfig, defaultConfig map[string]string) string {
	config := customConfig

	for k, v := range defaultConfig {
		// add key-value only when the flag does not exist in custom config
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}

	keys := []string{}
	for key := range config {
		if !datamodel.CommandLineOmittedKubeletConfigFlags[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("%s=%s ", key, config[key]))
	}
	return buf.String()
}

func getKubeletCustomConfiguration(properties *datamodel.Properties) map[string]string {
	if properties.CustomConfiguration == nil || properties.CustomConfiguration.KubernetesConfigurations == nil {
		return nil
	}
	kubeletConfigurations, ok := properties.CustomConfiguration.KubernetesConfigurations["kubelet"]
	if !ok {
		return nil
	}
	if kubeletConfigurations.Config == nil {
		return nil
	}
	// empty config is treated as nil
	if len(kubeletConfigurations.Config) == 0 {
		return nil
	}
	return kubeletConfigurations.Config
}

// IsKubeletConfigFileEnabled get if dynamic kubelet is supported in AKS and toggle is on
func IsKubeletConfigFileEnabled(cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile, kubeletConfigFileToggleEnabled bool) bool {
	// TODO(bowa) remove toggle when backfill
	// If customKubeletConfig or customLinuxOSConfig is used (API20201101 and later), use kubelet config file
	return profile.CustomKubeletConfig != nil || profile.CustomLinuxOSConfig != nil ||
		(kubeletConfigFileToggleEnabled && cs.Properties.OrchestratorProfile.IsKubernetes() &&
			IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.14.0"))
}

// IsKubeletClientTLSBootstrappingEnabled get if kubelet client TLS bootstrapping is enabled
func IsKubeletClientTLSBootstrappingEnabled(tlsBootstrapToken *string) bool {
	return tlsBootstrapToken != nil
}

// GetTLSBootstrapTokenForKubeConfig returns the TLS bootstrap token for kubeconfig usage.
// It returns empty string if TLS bootstrap token is not enabled.
//
// ref: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#kubelet-configuration
func GetTLSBootstrapTokenForKubeConfig(tlsBootstrapToken *string) string {
	if tlsBootstrapToken == nil {
		// not set
		return ""
	}

	return *tlsBootstrapToken
}

// GetKubeletConfigFileContent converts kubelet flags we set to a file, and return the json content
func GetKubeletConfigFileContent(kc map[string]string, customKc *datamodel.CustomKubeletConfig) string {
	if kc == nil {
		return ""
	}
	// translate simple values
	kubeletConfig := &datamodel.AKSKubeletConfiguration{
		APIVersion:    "kubelet.config.k8s.io/v1beta1",
		Kind:          "KubeletConfiguration",
		Address:       kc["--address"],
		StaticPodPath: kc["--pod-manifest-path"],
		Authorization: datamodel.KubeletAuthorization{
			Mode: datamodel.KubeletAuthorizationMode(kc["--authorization-mode"]),
		},
		ClusterDNS:                     strings.Split(kc["--cluster-dns"], ","),
		CgroupsPerQOS:                  strToBoolPtr(kc["--cgroups-per-qos"]),
		TLSCertFile:                    kc["--tls-cert-file"],
		TLSPrivateKeyFile:              kc["--tls-private-key-file"],
		TLSCipherSuites:                strings.Split(kc["--tls-cipher-suites"], ","),
		ClusterDomain:                  kc["--cluster-domain"],
		MaxPods:                        strToInt32(kc["--max-pods"]),
		NodeStatusUpdateFrequency:      datamodel.Duration(kc["--node-status-update-frequency"]),
		NodeStatusReportFrequency:      datamodel.Duration(kc["--node-status-report-frequency"]),
		ImageGCHighThresholdPercent:    strToInt32Ptr(kc["--image-gc-high-threshold"]),
		ImageGCLowThresholdPercent:     strToInt32Ptr(kc["--image-gc-low-threshold"]),
		EventRecordQPS:                 strToInt32Ptr(kc["--event-qps"]),
		PodPidsLimit:                   strToInt64Ptr(kc["--pod-max-pids"]),
		EnforceNodeAllocatable:         strings.Split(kc["--enforce-node-allocatable"], ","),
		StreamingConnectionIdleTimeout: datamodel.Duration(kc["--streaming-connection-idle-timeout"]),
		RotateCertificates:             strToBool(kc["--rotate-certificates"]),
		ReadOnlyPort:                   strToInt32(kc["--read-only-port"]),
		ProtectKernelDefaults:          strToBool(kc["--protect-kernel-defaults"]),
		ResolverConfig:                 kc["--resolv-conf"],
		ContainerLogMaxSize:            kc["--container-log-max-size"],
	}

	// Authentication
	kubeletConfig.Authentication = datamodel.KubeletAuthentication{}
	if ca := kc["--client-ca-file"]; ca != "" {
		kubeletConfig.Authentication.X509 = datamodel.KubeletX509Authentication{
			ClientCAFile: ca,
		}
	}
	if aw := kc["--authentication-token-webhook"]; aw != "" {
		kubeletConfig.Authentication.Webhook = datamodel.KubeletWebhookAuthentication{
			Enabled: strToBool(aw),
		}
	}
	if aa := kc["--anonymous-auth"]; aa != "" {
		kubeletConfig.Authentication.Anonymous = datamodel.KubeletAnonymousAuthentication{
			Enabled: strToBool(aa),
		}
	}

	// EvictionHard
	// default: "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%"
	if eh, ok := kc["--eviction-hard"]; ok && eh != "" {
		kubeletConfig.EvictionHard = strKeyValToMap(eh, ",", "<")
	}

	// feature gates
	// look like "f1=true,f2=true"
	kubeletConfig.FeatureGates = strKeyValToMapBool(kc["--feature-gates"], ",", "=")

	// system reserve and kube reserve
	// looks like "cpu=100m,memory=1638Mi"
	kubeletConfig.SystemReserved = strKeyValToMap(kc["--system-reserved"], ",", "=")
	kubeletConfig.KubeReserved = strKeyValToMap(kc["--kube-reserved"], ",", "=")

	// Settings from customKubeletConfig, only take if it's set
	if customKc != nil {
		if customKc.CPUManagerPolicy != "" {
			kubeletConfig.CPUManagerPolicy = customKc.CPUManagerPolicy
		}
		if customKc.CPUCfsQuota != nil {
			kubeletConfig.CPUCFSQuota = customKc.CPUCfsQuota
		}
		if customKc.CPUCfsQuotaPeriod != "" {
			kubeletConfig.CPUCFSQuotaPeriod = datamodel.Duration(customKc.CPUCfsQuotaPeriod)
			// enable CustomCPUCFSQuotaPeriod feature gate is required for this configuration
			kubeletConfig.FeatureGates["CustomCPUCFSQuotaPeriod"] = true
		}
		if customKc.TopologyManagerPolicy != "" {
			kubeletConfig.TopologyManagerPolicy = customKc.TopologyManagerPolicy
			// enable TopologyManager feature gate is required for this configuration
			kubeletConfig.FeatureGates["TopologyManager"] = true
		}
		if customKc.ImageGcHighThreshold != nil {
			kubeletConfig.ImageGCHighThresholdPercent = customKc.ImageGcHighThreshold
		}
		if customKc.ImageGcLowThreshold != nil {
			kubeletConfig.ImageGCLowThresholdPercent = customKc.ImageGcLowThreshold
		}
		if customKc.AllowedUnsafeSysctls != nil {
			kubeletConfig.AllowedUnsafeSysctls = *customKc.AllowedUnsafeSysctls
		}
		if customKc.FailSwapOn != nil {
			kubeletConfig.FailSwapOn = customKc.FailSwapOn
		}
		if customKc.ContainerLogMaxSizeMB != nil {
			kubeletConfig.ContainerLogMaxSize = fmt.Sprintf("%dM", *customKc.ContainerLogMaxSizeMB)
		}
		if customKc.ContainerLogMaxFiles != nil {
			kubeletConfig.ContainerLogMaxFiles = customKc.ContainerLogMaxFiles
		}
		if customKc.PodMaxPids != nil {
			kubeletConfig.PodPidsLimit = to.Int64Ptr(int64(*customKc.PodMaxPids))
		}
	}

	configStringByte, _ := json.MarshalIndent(kubeletConfig, "", "    ")
	return string(configStringByte)
}

func strToBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

func strToBoolPtr(str string) *bool {
	if str == "" {
		return nil
	}
	b := strToBool(str)
	return &b
}

func strToInt32(str string) int32 {
	i, _ := strconv.ParseInt(str, 10, 32)
	return int32(i)
}

func strToInt32Ptr(str string) *int32 {
	if str == "" {
		return nil
	}
	i := strToInt32(str)
	return &i
}

func strToInt64(str string) int64 {
	i, _ := strconv.ParseInt(str, 10, 64)
	return i
}

func strToInt64Ptr(str string) *int64 {
	if str == "" {
		return nil
	}
	i := strToInt64(str)
	return &i
}

func strKeyValToMap(str string, strDelim string, pairDelim string) map[string]string {
	m := make(map[string]string)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		if len(pair) == 2 {
			key := strings.TrimSpace(pair[0])
			val := strings.TrimSpace(pair[1])
			m[key] = val
		}
	}
	return m
}

func strKeyValToMapBool(str string, strDelim string, pairDelim string) map[string]bool {
	m := make(map[string]bool)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		if len(pair) == 2 {
			key := strings.TrimSpace(pair[0])
			val := strings.TrimSpace(pair[1])
			m[key] = strToBool(val)
		}
	}
	return m
}

func removeFeatureGateString(featureGates string, key string) string {
	fgMap := strKeyValToMapBool(featureGates, ",", "=")
	delete(fgMap, key)
	keys := make([]string, 0, len(fgMap))
	for k := range fgMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%t", k, fgMap[k]))
	}
	return strings.Join(pairs, ",")
}

func addFeatureGateString(featureGates string, key string, value bool) string {
	fgMap := strKeyValToMapBool(featureGates, ",", "=")
	fgMap[key] = value
	keys := make([]string, 0, len(fgMap))
	for k := range fgMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%t", k, fgMap[k]))
	}
	return strings.Join(pairs, ",")
}

// ParseCSEMessage parses the raw CSE output
func ParseCSEMessage(message string) (*datamodel.CSEStatus, *datamodel.CSEStatusParsingError) {
	start := strings.Index(message, "[stdout]") + len("[stdout]")
	end := strings.Index(message, "[stderr]")
	if end > start {
		return parseLinuxCSEMessage(message, start, end)
	} else if strings.Contains(message, "Command execution finished") {
		return parseWindowsCSEMessage(message)
	}
	return nil, datamodel.NewError(datamodel.InvalidCSEMessage, message)
}

func parseLinuxCSEMessage(message string, start int, end int) (*datamodel.CSEStatus, *datamodel.CSEStatusParsingError) {
	// Linux CSE message example: Enable succeeded: \n[stdout]\n{ \"ExitCode\": \"0\", \"Output\": \"Tue Dec 28" } }\n\n[stderr]\nBootup is not yet finished. Please try again later.
	var cseStatus datamodel.CSEStatus
	rawInstanceViewInfo := message[start:end]
	err := json.Unmarshal([]byte(rawInstanceViewInfo), &cseStatus)
	if err != nil {
		return nil, datamodel.NewError(datamodel.CSEMessageUnmarshalError, message)
	}
	if cseStatus.ExitCode == "" {
		return nil, datamodel.NewError(datamodel.CSEMessageExitCodeEmptyError, message)
	}
	return &cseStatus, nil
}

func parseWindowsCSEMessage(message string) (*datamodel.CSEStatus, *datamodel.CSEStatusParsingError) {
	// Windows CSE message example: Command execution finished, but failed because it returned a non-zero exit code of: '1'.
	var cseStatus datamodel.CSEStatus
	re := regexp.MustCompile(`a non-zero exit code of: '(\d+)'`)
	match := re.FindStringSubmatch(message)
	if match != nil {
		cseStatus.ExitCode = match[1]
	} else {
		cseStatus.ExitCode = "0"
	}
	cseStatus.Output = message
	return &cseStatus, nil
}
