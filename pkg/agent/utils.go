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

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/blang/semver"
)

/*
	TranslatedKubeletConfigFlags represents kubelet flags that will be translated into config file

(if kubelet config file is enabled).
*/
//nolint:gochecknoglobals
var TranslatedKubeletConfigFlags = map[string]bool{
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
	"--serialize-image-pulls":             true,
}

type paramsMap map[string]interface{}

const numInPair = 2

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

//nolint:unparam,nolintlint
func addSecret(m paramsMap, k string, v interface{}, encode bool) {
	str, ok := v.(string)
	if !ok {
		addValue(m, k, v)
		return
	}
	keyvaultSecretPathRe := regexp.MustCompile(`^(/subscriptions/\S+/resourceGroups/\S+/providers/Microsoft.KeyVault/vaults/\S+)/secrets/([^/\s]+)(/(\S+))?$`) //nolint:lll
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
	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script,
		extensionProfile.URLQuery)
	scriptFilePath := fmt.Sprintf("/opt/azure/containers/extensions/%s/%s", extensionProfile.Name, extensionProfile.Script)
	return fmt.Sprintf("- sudo /usr/bin/curl --retry 5 --retry-delay 10 --retry-max-time 30 -o %s --create-dirs %s \"%s\" \n- sudo /bin/"+
		"chmod 744 %s \n- sudo %s ',%s,' > /var/log/%s-output.log", scriptFilePath, curlCaCertOpt, scriptURL, scriptFilePath, scriptFilePath,
		extensionsParameterReference, extensionProfile.Name)
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

	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script,
		extensionProfile.URLQuery)
	scriptFileDir := fmt.Sprintf("$env:SystemDrive:/AzureData/extensions/%s", extensionProfile.Name)
	scriptFilePath := fmt.Sprintf("%s/%s", scriptFileDir, extensionProfile.Script)
	return fmt.Sprintf("New-Item -ItemType Directory -Force -Path \"%s\" ; curl.exe --retry 5 --retry-delay 0 -L \"%s\" -o \"%s\" ; powershell \"%s `\"',parameters('%sParameters'),'`\"\"\n", scriptFileDir, scriptURL, scriptFilePath, scriptFilePath, extensionProfile.Name) //nolint:lll
}

func escapeSingleLine(escapedStr string) string {
	// template.JSEscapeString leaves undesirable chars that don't work with pretty print.
	escapedStr = strings.ReplaceAll(escapedStr, "\\", "\\\\")
	escapedStr = strings.ReplaceAll(escapedStr, "\r\n", "\\n")
	escapedStr = strings.ReplaceAll(escapedStr, "\n", "\\n")
	escapedStr = strings.ReplaceAll(escapedStr, "\"", "\\\"")
	return escapedStr
}

// getBase64EncodedGzippedCustomScript will return a base64 of the CSE.
func getBase64EncodedGzippedCustomScript(csFilename string, config *datamodel.NodeBootstrappingConfiguration) string {
	b, err := parts.Templates.ReadFile(csFilename)
	if err != nil {
		// this should never happen and this is a bug.
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	// translate the parameters.
	b = removeComments(b)
	templ := template.New("ContainerService template").Option("missingkey=error").Funcs(getContainerServiceFuncMap(config))
	_, err = templ.Parse(string(b))
	if err != nil {
		// this should never happen and this is a bug.
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	var buffer bytes.Buffer
	err = templ.Execute(&buffer, config.ContainerService)
	if err != nil {
		// this should never happen and this is a bug.
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	csStr := buffer.String()
	csStr = strings.ReplaceAll(csStr, "\r\n", "\n")
	return getBase64EncodedGzippedCustomScriptFromStr(csStr)
}

// This is "best-effort" - removes MOST of the comments with obvious formats, to lower the space required by CustomData component.
func removeComments(b []byte) []byte {
	var contentWithoutComments []string
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		lineNoWhitespace := strings.TrimSpace(line)
		if lineStartsWithComment(lineNoWhitespace) {
			// ignore entire line that is a comment
			continue
		}
		line = trimTrailingComment(line)
		contentWithoutComments = append(contentWithoutComments, line)
	}
	return []byte(strings.Join(contentWithoutComments, "\n"))
}

func lineStartsWithComment(trimmedToCheck string) bool {
	return strings.HasPrefix(trimmedToCheck, "# ") || strings.HasPrefix(trimmedToCheck, "##")
}

func trimTrailingComment(line string) string {
	lastHashIndex := strings.LastIndex(line, "#")
	if lastHashIndex > 0 && isCommentAtTheEndOfLine(lastHashIndex, line) && !lineLogsToOutput(line) {
		// remove only the comment part from line
		line = line[:lastHashIndex]
	}
	return line
}

func lineLogsToOutput(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "echo")
}

// Trying to avoid using a regex. There are certain patterns we ignore just to be on the safe side. This is enough to get rid of most of the obvious comments.
func isCommentAtTheEndOfLine(lastHashIndex int, trimmedToCheck string) bool {
	getSlice := func(start, end int, str string) string {
		if end > len(str) || start > end {
			return ""
		}
		return str[start:end]
	}
	// These are two of patterns that are present amongst Agent Baker files that we need to specifically check for. Non-exhaustive
	tailingCommentSegmentLen := 2
	return getSlice(lastHashIndex-1, lastHashIndex+1, trimmedToCheck) != "<#" && getSlice(lastHashIndex, lastHashIndex+tailingCommentSegmentLen, trimmedToCheck) == "# "
}

// getBase64EncodedGzippedCustomScriptFromStr will return a base64-encoded string of the gzip'd source data.
func getBase64EncodedGzippedCustomScriptFromStr(str string) string {
	var gzipB bytes.Buffer
	w := gzip.NewWriter(&gzipB)
	_, err := w.Write([]byte(str))
	if err != nil {
		// this should never happen and this is a bug.
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
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

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support.
func IsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case "Standard_DC2s", "Standard_DC4s":
		return true
	}
	return false
}

/* GetCloudTargetEnv determines and returns whether the region is a sovereign cloud which
have their own data compliance regulations (China/Germany/USGov) or standard.  */
// Azure public cloud.
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

// IsKubernetesVersionGe returns true if actualVersion is greater than or equal to version.
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

// GetOrderedKubeletConfigFlagString returns an ordered string of key/val pairs.
// copied from AKS-Engine and filter out flags that already translated to config file.
func GetOrderedKubeletConfigFlagString(k map[string]string, cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile,
	kubeletConfigFileToggleEnabled bool) string {
	/* NOTE(mainred): kubeConfigFile now relies on CustomKubeletConfig, while custom configuration is not
	compatible with CustomKubeletConfig. When custom configuration is set we want to override every
	configuration with the customized one. */
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
	ommitedKubletConfigFlags := datamodel.GetCommandLineOmittedKubeletConfigFlags()
	for key := range k {
		if !kubeletConfigFileEnabled || !TranslatedKubeletConfigFlags[key] {
			if !ommitedKubletConfigFlags[key] {
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
		// add key-value only when the flag does not exist in custom config.
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}

	keys := []string{}
	ommitedKubletConfigFlags := datamodel.GetCommandLineOmittedKubeletConfigFlags()
	for key := range config {
		if !ommitedKubletConfigFlags[key] {
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
	// empty config is treated as nil.
	if len(kubeletConfigurations.Config) == 0 {
		return nil
	}
	return kubeletConfigurations.Config
}

// IsKubeletConfigFileEnabled get if dynamic kubelet is supported in AKS and toggle is on.
func IsKubeletConfigFileEnabled(cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile, kubeletConfigFileToggleEnabled bool) bool {
	// TODO(bowa) remove toggle when backfill.
	// If customKubeletConfig or customLinuxOSConfig is used (API20201101 and later), use kubelet config file.
	return profile.CustomKubeletConfig != nil || profile.CustomLinuxOSConfig != nil ||
		(kubeletConfigFileToggleEnabled && cs.Properties.OrchestratorProfile.IsKubernetes() &&
			IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.14.0"))
}

// IsTLSBootstrappingEnabledWithHardCodedToken returns true if the specified TLS bootstrap token is non-nil, meaning
// we will use it to perform TLS bootstrapping.
func IsTLSBootstrappingEnabledWithHardCodedToken(tlsBootstrapToken *string) bool {
	return tlsBootstrapToken != nil
}

// GetTLSBootstrapTokenForKubeConfig returns the TLS bootstrap token for kubeconfig usage.
// It returns empty string if TLS bootstrap token is not enabled.
// ref: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#kubelet-configuration
func GetTLSBootstrapTokenForKubeConfig(tlsBootstrapToken *string) string {
	if tlsBootstrapToken == nil {
		// not set
		return ""
	}

	return *tlsBootstrapToken
}

func IsKubeletServingCertificateRotationEnabled(config *datamodel.NodeBootstrappingConfiguration) bool {
	if config == nil || config.KubeletConfig == nil {
		return false
	}
	return config.KubeletConfig["--rotate-server-certificates"] == "true"
}

func getAgentKubernetesLabels(profile *datamodel.AgentPoolProfile, config *datamodel.NodeBootstrappingConfiguration) string {
	var labels string
	if profile != nil {
		labels = profile.GetKubernetesLabels()
	}
	kubeletServingSignerLabel := getKubeletServingCALabel(config)

	if labels == "" {
		return kubeletServingSignerLabel
	}
	if kubeletServingSignerLabel == "" {
		return labels
	}
	return fmt.Sprintf("%s,%s", labels, kubeletServingSignerLabel)
}

// getKubeletServingCALabel determines the value of the special kubelet serving CA label,
// based on the specified NodeBootstrappingConfiguration. This label is used to denote, out-of-band from RP-set
// CustomNodeLabels, whether or not the given kubelet is started with the --rotate-server-certificates flag.
// When the flag is set, this label will in the form of "kubernetes.azure.com/kubelet-serving-ca=cluster",
// indicating the CA that signed the kubelet's serving certificate is the cluster CA.
// Otherwise, this will return an empty string, and no extra labels will be added to the node.
// TODO(cameissner): revisit whether to add a negative label for the disabled case,
// e.g. "kubernetes.azure.com/kubelet-serving-ca=self", before this feature is rolled out.
func getKubeletServingCALabel(config *datamodel.NodeBootstrappingConfiguration) string {
	if IsKubeletServingCertificateRotationEnabled(config) {
		return "kubernetes.azure.com/kubelet-serving-ca=cluster"
	}
	return ""
}

func getAKSKubeletConfiguration(kc map[string]string) *datamodel.AKSKubeletConfiguration {
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
		ServerTLSBootstrap:             strToBool(kc["--rotate-server-certificates"]),
		ReadOnlyPort:                   strToInt32(kc["--read-only-port"]),
		ProtectKernelDefaults:          strToBool(kc["--protect-kernel-defaults"]),
		ResolverConfig:                 kc["--resolv-conf"],
		ContainerLogMaxSize:            kc["--container-log-max-size"],
	}

	// Serialize Image Pulls will only be set for k8s >= 1.31, currently RP doesnt pass this flag
	// It will starting with k8s 1.31
	if value, exists := kc["--serialize-image-pulls"]; exists {
		kubeletConfig.SerializeImagePulls = strToBoolPtr(value)
	}

	return kubeletConfig
}

//nolint:gocognit
func setCustomKubeletConfig(customKc *datamodel.CustomKubeletConfig,
	kubeletConfig *datamodel.AKSKubeletConfiguration) {
	if customKc != nil { //nolint:nestif
		if customKc.CPUManagerPolicy != "" {
			kubeletConfig.CPUManagerPolicy = customKc.CPUManagerPolicy
		}
		if customKc.CPUCfsQuota != nil {
			kubeletConfig.CPUCFSQuota = customKc.CPUCfsQuota
		}
		if customKc.CPUCfsQuotaPeriod != "" {
			kubeletConfig.CPUCFSQuotaPeriod = datamodel.Duration(customKc.CPUCfsQuotaPeriod)
			// enable CustomCPUCFSQuotaPeriod feature gate is required for this configuration.
			kubeletConfig.FeatureGates["CustomCPUCFSQuotaPeriod"] = true
		}
		if customKc.TopologyManagerPolicy != "" {
			kubeletConfig.TopologyManagerPolicy = customKc.TopologyManagerPolicy
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
		if customKc.SeccompDefault != nil {
			kubeletConfig.SeccompDefault = customKc.SeccompDefault
		}
	}
}

// GetKubeletConfigFileContent converts kubelet flags we set to a file, and return the json content.
func GetKubeletConfigFileContent(kc map[string]string, customKc *datamodel.CustomKubeletConfig) string {
	if kc == nil {
		return ""
	}
	// translate simple values.
	kubeletConfig := getAKSKubeletConfiguration(kc)

	// Authentication.
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

	// EvictionHard.
	// default: "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%".
	if eh, ok := kc["--eviction-hard"]; ok && eh != "" {
		kubeletConfig.EvictionHard = strKeyValToMap(eh, ",", "<")
	}

	// feature gates.
	// look like "f1=true,f2=true".
	kubeletConfig.FeatureGates = strKeyValToMapBool(kc["--feature-gates"], ",", "=")

	// system reserve and kube reserve.
	// looks like "cpu=100m,memory=1638Mi".
	kubeletConfig.SystemReserved = strKeyValToMap(kc["--system-reserved"], ",", "=")
	kubeletConfig.KubeReserved = strKeyValToMap(kc["--kube-reserved"], ",", "=")

	// Settings from customKubeletConfig, only take if it's set.
	setCustomKubeletConfig(customKc, kubeletConfig)

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
		if len(pair) == numInPair {
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
		if len(pair) == numInPair {
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
