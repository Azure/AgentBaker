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
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/templates"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/blang/semver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logsapi "k8s.io/component-base/config/v1alpha1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// TranslatedKubeletConfigFlags represents kubelet flags that will be translated into config file (if kubelet config file is enabled)
var TranslatedKubeletConfigFlags map[string]bool = map[string]bool{
	"--address":                                      true,
	"--allowed-unsafe-sysctls":                       true,
	"--anonymous-auth":                               true,
	"--authentication-token-webhook":                 true,
	"--authentication-token-webhook-cache-ttl":       true,
	"--authorization-mode":                           true,
	"--authorization-webhook-cache-authorized-ttl":   true,
	"--authorization-webhook-cache-unauthorized-ttl": true,
	"--cgroup-driver":                                true,
	"--cgroup-root":                                  true,
	"--cgroups-per-qos":                              true,
	"--client-ca-file":                               true,
	"--cluster-dns":                                  true,
	"--cluster-domain":                               true,
	"--container-log-max-files":                      true,
	"--container-log-max-size":                       true,
	"--contention-profiling":                         true,
	"--cpu-cfs-quota":                                true,
	"--cpu-cfs-quota-period":                         true,
	"--cpu-manager-policy":                           true,
	"--cpu-manager-policy-options":                   true,
	"--cpu-manager-reconcile-period":                 true,
	"--enable-controller-attach-detach":              true,
	"--enable-debugging-handlers":                    true,
	"--enforce-node-allocatable":                     true,
	"--event-burst":                                  true,
	"--event-qps":                                    true,
	"--eviction-hard":                                true,
	"--eviction-max-pod-grace-period":                true,
	"--eviction-minimum-reclaim":                     true,
	"--eviction-pressure-transition-period":          true,
	"--eviction-soft":                                true,
	"--eviction-soft-grace-period":                   true,
	"--fail-swap-on":                                 true,
	"--feature-gates":                                true,
	"--file-check-frequency":                         true,
	"--hairpin-mode":                                 true,
	"--healthz-bind-address":                         true,
	"--healthz-port":                                 true,
	"--http-check-frequency":                         true,
	"--image-gc-high-threshold":                      true,
	"--image-gc-low-threshold":                       true,
	"--iptables-drop-bit":                            true,
	"--iptables-masquerade-bit":                      true,
	"--kernel-memcg-notification":                    true,
	"--kube-api-burst":                               true,
	"--kube-api-content-type":                        true,
	"--kube-api-qps":                                 true,
	"--kube-reserved":                                true,
	"--kube-reserved-cgroup":                         true,
	"--kubelet-cgroups":                              true,
	"--log-flush-frequency":                          true,
	"--log-json-info-buffer-size":                    true,
	"--log-json-split-stream":                        true,
	"--logging-format":                               true,
	"--make-iptables-util-chains":                    true,
	"--manifest-url":                                 true,
	"--manifest-url-header":                          true,
	"--max-open-files":                               true,
	"--max-pods":                                     true,
	"--memory-manager-policy":                        true,
	"--node-status-max-images":                       true,
	"--node-status-update-frequency":                 true,
	"--node-status-report-frequency":                 true,
	"--oom-score-adj":                                true,
	"--pod-cidr":                                     true,
	"--pod-manifest-path":                            true,
	"--pod-max-pids":                                 true,
	"--pods-per-core":                                true,
	"--port":                                         true,
	"--protect-kernel-defaults":                      true,
	"--provider-id":                                  true,
	"--qos-reserved":                                 true,
	"--read-only-port":                               true,
	"--registry-burst":                               true,
	"--resolv-conf":                                  true,
	"--register-node":                                true,
	"--register-with-taints":                         true,
	"--registry-qps":                                 true,
	"--reserved-cpus":                                true,
	"--reserved-memory":                              true,
	"--rotate-certificates":                          true,
	"--runonce":                                      true,
	"--runtime-request-timeout":                      true,
	"--seccomp-default":                              true,
	"--serialize-image-pulls":                        true,
	"--streaming-connection-idle-timeout":            true,
	"--sync-frequency":                               true,
	"--system-cgroups":                               true,
	"--system-reserved":                              true,
	"--system-reserved-cgroup":                       true,
	"--tls-cert-file":                                true,
	"--tls-cipher-suites":                            true,
	"--tls-min-version":                              true,
	"--tls-private-key-file":                         true,
	"--topology-manager-policy":                      true,
	"--topology-manager-scope":                       true,
	"--volume-plugin-dir":                            true,
	"--volume-stats-agg-period":                      true,
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
			// --node-status-report-frequency is not a valid command line flag
			// and should only be explicitely set in the config file
			if key != "--node-status-report-frequency" {
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
		keys = append(keys, key)
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
	// Translate simple values
	kubeletConfig := &datamodel.AKSKubeletConfiguration{
		APIVersion:                       "kubelet.config.k8s.io/v1beta1",
		Kind:                             "KubeletConfiguration",
		Address:                          kc["--address"],
		AllowedUnsafeSysctls:             strings.Split(kc["--allowed-unsafe-sysctls"], ","),
		CgroupDriver:                     kc["--cgroup-driver"],
		CgroupRoot:                       kc["--cgroup-root"],
		CgroupsPerQOS:                    strToBoolPtr(kc["--cgroups-per-qos"]),
		ClusterDNS:                       strings.Split(kc["--cluster-dns"], ","),
		ClusterDomain:                    kc["--cluster-domain"],
		ContainerLogMaxFiles:             strToInt32Ptr(kc["--container-log-max-files"]),
		ContainerLogMaxSize:              kc["--container-log-max-size"],
		ContentType:                      kc["--kube-api-content-type"],
		CPUCFSQuota:                      strToBoolPtr(kc["--cpu-cfs-quota"]),
		CPUCFSQuotaPeriod:                strToMetaDurationPtr(kc["--cpu-cfs-quota-period"]),
		CPUManagerPolicy:                 kc["--cpu-manager-policy"],
		CPUManagerReconcilePeriod:        strToMetaDuration(kc["--cpu-manager-reconcile-period"]),
		EnableContentionProfiling:        strToBool(kc["--contention-profiling"]),
		EnableControllerAttachDetach:     strToBoolPtr(kc["--enable-controller-attach-detach"]),
		EnableDebuggingHandlers:          strToBoolPtr(kc["--enable-debugging-handlers"]),
		EnableServer:                     strToBoolPtr(kc["--enable-server"]),
		EnforceNodeAllocatable:           strings.Split(kc["--enforce-node-allocatable"], ","),
		EventBurst:                       strToInt32(kc["--event-burst"]),
		EventRecordQPS:                   strToInt32Ptr(kc["--event-qps"]),
		EvictionMaxPodGracePeriod:        strToInt32(kc["--eviction-max-pod-grace-period"]),
		EvictionPressureTransitionPeriod: strToMetaDuration(kc["--eviction-pressure-transition-period"]),
		FailSwapOn:                       strToBoolPtr(kc["--fail-swap-on"]),
		FileCheckFrequency:               strToMetaDuration(kc["--file-check-frequency"]),
		HairpinMode:                      kc["--hairpin-mode"],
		HealthzBindAddress:               kc["--healthz-bind-address"],
		HealthzPort:                      strToInt32Ptr(kc["--healthz-port"]),
		HTTPCheckFrequency:               strToMetaDuration(kc["--http-check-frequency"]),
		ImageGCHighThresholdPercent:      strToInt32Ptr(kc["--image-gc-high-threshold"]),
		ImageGCLowThresholdPercent:       strToInt32Ptr(kc["--image-gc-low-threshold"]),
		IPTablesDropBit:                  strToInt32Ptr(kc["--iptables-drop-bit"]),
		IPTablesMasqueradeBit:            strToInt32Ptr(kc["--iptables-masquerade-bit"]),
		KernelMemcgNotification:          strToBool(kc["--kernel-memcg-notification"]),
		KubeAPIBurst:                     strToInt32(kc["--kube-api-burst"]),
		KubeAPIQPS:                       strToInt32Ptr(kc["--kube-api-qps"]),
		KubeReservedCgroup:               kc["--kube-reserved-cgroup"],
		KubeletCgroups:                   kc["--kubelet-cgroups"],
		MakeIPTablesUtilChains:           strToBoolPtr(kc["--make-iptables-util-chains"]),
		MaxOpenFiles:                     strToInt64(kc["--max-open-files"]),
		MaxPods:                          strToInt32(kc["--max-pods"]),
		MemoryManagerPolicy:              kc["--memory-manager-policy"],
		NodeStatusMaxImages:              strToInt32Ptr(kc["--node-status-max-images"]),
		NodeStatusUpdateFrequency:        strToMetaDuration(kc["--node-status-update-frequency"]),
		NodeStatusReportFrequency:        strToMetaDuration(kc["--node-status-report-frequency"]),
		OOMScoreAdj:                      strToInt32Ptr(kc["--oom-score-adj"]),
		PodCIDR:                          kc["--pod-cidr"],
		PodPidsLimit:                     strToInt64Ptr(kc["--pod-max-pods"]),
		PodsPerCore:                      strToInt32(kc["--pods-per-core"]),
		Port:                             strToInt32(kc["--port"]),
		ProtectKernelDefaults:            strToBool(kc["--protect-kernel-defaults"]),
		ProviderID:                       kc["--provider-id"],
		ReadOnlyPort:                     strToInt32(kc["--read-only-port"]),
		RegisterNode:                     strToBoolPtr(kc["--register-node"]),
		RegisterWithTaints:               strToTaintSlice(kc["--register-with-taints"]),
		RegistryBurst:                    strToInt32(kc["--registry-burst"]),
		RegistryPullQPS:                  strToInt32Ptr(kc["--registry-qps"]),
		ReservedSystemCPUs:               kc["--reserved-cpus"],
		ResolverConfig:                   strToStrPtr(kc["--resolv-conf"]),
		RotateCertificates:               strToBool(kc["--rotate-certificates"]),
		RunOnce:                          strToBool(kc["--run-once"]),
		RuntimeRequestTimeout:            strToMetaDuration(kc["--runtime-request-timeout"]),
		SeccompDefault:                   strToBoolPtr(kc["--seccomp-default"]),
		SerializeImagePulls:              strToBoolPtr(kc["--serialize-image-pulls"]),
		StaticPodPath:                    kc["--pod-manifest-path"],
		StaticPodURL:                     kc["--manifest-url"],
		StreamingConnectionIdleTimeout:   strToMetaDuration(kc["--streaming-connection-idle-timeout"]),
		SyncFrequency:                    strToMetaDuration(kc["--sync-frequency"]),
		SystemCgroups:                    kc["--system-cgroups"],
		SystemReservedCgroup:             kc["--system-reserved-cgroup"],
		TLSCertFile:                      kc["--tls-cert-file"],
		TLSCipherSuites:                  strings.Split(kc["--tls-cipher-suites"], ","),
		TLSMinVersion:                    kc["--tls-min-version"],
		TLSPrivateKeyFile:                kc["--tls-private-key-file"],
		TopologyManagerPolicy:            kc["--topology-manager-policy"],
		TopologyManagerScope:             kc["--topology-manager-scope"],
		VolumePluginDir:                  kc["--volume-plugin-dir"],
		VolumeStatsAggPeriod:             strToMetaDuration(kc["--volume-stats-agg-period"]),
	}

	// Authentication
	kubeletConfig.Authentication = kubeletconfigv1beta1.KubeletAuthentication{
		X509: kubeletconfigv1beta1.KubeletX509Authentication{
			ClientCAFile: kc["--client-ca-file"],
		},
		Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
			Enabled:  strToBoolPtr(kc["--authentication-token-webhook"]),
			CacheTTL: strToMetaDuration(kc["--authentication-token-webhook-cache-ttl"]),
		},
		Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
			Enabled: strToBoolPtr(kc["--anonymous-auth"]),
		},
	}

	// Authorization
	kubeletConfig.Authorization = kubeletconfigv1beta1.KubeletAuthorization{
		Mode: kubeletconfigv1beta1.KubeletAuthorizationMode(kc["--authorization-mode"]),
		Webhook: kubeletconfigv1beta1.KubeletWebhookAuthorization{
			CacheAuthorizedTTL:   strToMetaDuration(kc["--authorization-webhook-cache-authorized-ttl"]),
			CacheUnauthorizedTTL: strToMetaDuration(kc["--authorization-webhook-cache-unauthorized-ttl"]),
		},
	}

	//Logging
	kubeletConfig.Logging = logsapi.LoggingConfiguration{
		Format: kc["--logging-format"],
		Options: logsapi.FormatOptions{
			JSON: logsapi.JSONOptions{
				SplitStream: strToBool(kc["--log-json-split-stream"]),
			},
		},
	}
	if infoBufferSize := kc["--log-json-info-buffer-size"]; infoBufferSize != "" {
		kubeletConfig.Logging.Options.JSON.InfoBufferSize = resource.QuantityValue{
			Quantity: resource.MustParse(infoBufferSize),
		}
	}
	flushFrequency, _ := time.ParseDuration(kc["--log-flush-frequency"])
	kubeletConfig.Logging.FlushFrequency = flushFrequency

	// StaticPodURLHeader
	kubeletConfig.StaticPodURLHeader = strKeySliceValToMap(kc["--manifest-url-header"], ",", ":")
	// CPUManagerPolicyOptions and EvictionMinimumReclaim
	if cpuManagerPolicyOpts := kc["--cpu-manager-policy-options"]; cpuManagerPolicyOpts != "" {
		kubeletConfig.CPUManagerPolicyOptions = strKeyValToMap(cpuManagerPolicyOpts, ",", "=")
	}
	// EvictionMinimumReclaim
	if evictionMinReclaim := kc["--eviction-minimum-reclaim"]; evictionMinReclaim != "" {
		kubeletConfig.EvictionMinimumReclaim = strKeyValToMap(evictionMinReclaim, ",", "=")
	}
	// EvictionHard
	// default: "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%"
	if evictionHard := kc["--eviction-hard"]; evictionHard != "" {
		kubeletConfig.EvictionHard = strKeyValToMap(evictionHard, ",", "<")
	}
	// EvictionSoft
	if evictionSoft := kc["--eviction-soft"]; evictionSoft != "" {
		kubeletConfig.EvictionSoft = strKeyValToMap(evictionSoft, ",", "<")
	}
	// EvictionSoftGracePeriod
	if evictionSoftGracePeriod := kc["--eviction-soft-grace-period"]; evictionSoftGracePeriod != "" {
		kubeletConfig.EvictionSoftGracePeriod = strKeyValToMap(evictionSoftGracePeriod, ",", "=")
	}
	// FeatureGates
	// look like "f1=true,f2=true"
	if featureGates := kc["--feature-gates"]; featureGates != "" {
		kubeletConfig.FeatureGates = strKeyValToMapBool(featureGates, ",", "=")
	}
	// numa node memory reservations
	// looks like "0:memory=1Gi,hugepages-1M=2Gi"
	kubeletConfig.ReservedMemory = strToMemoryReservationSlice(kc["--reserved-memory"])

	// SystemReserved
	// looks like "cpu=100m,memory=1638Mi"
	if systemReserved := kc["--system-reserved"]; systemReserved != "" {
		kubeletConfig.SystemReserved = strKeyValToMap(systemReserved, ",", "=")
	}
	// KubeReserved
	if kubeReserved := kc["--kube-reserved"]; kubeReserved != "" {
		kubeletConfig.KubeReserved = strKeyValToMap(kubeReserved, ",", "=")
	}
	// QosReserved
	// looks like "memory=50%,cpu=30%"
	if qosReserved := kc["--qos-reserved"]; qosReserved != "" {
		kubeletConfig.QOSReserved = strKeyValToMap(qosReserved, ",", "=")
	}

	// Settings from customKubeletConfig, only take if it's set
	if customKc != nil {
		if customKc.CPUManagerPolicy != "" {
			kubeletConfig.CPUManagerPolicy = customKc.CPUManagerPolicy
		}
		if customKc.CPUCfsQuota != nil {
			kubeletConfig.CPUCFSQuota = customKc.CPUCfsQuota
		}
		if customKc.CPUCfsQuotaPeriod != "" {
			kubeletConfig.CPUCFSQuotaPeriod = strToMetaDurationPtr(customKc.CPUCfsQuotaPeriod)
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

func strToStrPtr(str string) *string {
	if str == "" {
		return nil
	}
	return &str
}

func strToMetaDuration(str string) metav1.Duration {
	d, _ := time.ParseDuration(str)
	return metav1.Duration{Duration: d}
}

func strToMetaDurationPtr(str string) *metav1.Duration {
	if str == "" {
		return nil
	}
	d := strToMetaDuration(str)
	return &d
}

func strToTaint(str string) v1.Taint {
	taintParts := strings.Split(str, "=")
	valueAndEffect := strings.Split(taintParts[1], ":")
	return v1.Taint{
		Key:    taintParts[0],
		Value:  valueAndEffect[0],
		Effect: v1.TaintEffect(valueAndEffect[1]),
	}
}

func strToTaintSlice(str string) []v1.Taint {
	if str == "" {
		return nil
	}
	taints := []v1.Taint{}
	rawTaints := strings.Split(str, ",")
	for _, taintStr := range rawTaints {
		taints = append(taints, strToTaint(taintStr))
	}
	return taints
}

func strToMemoryReservationSlice(str string) []kubeletconfigv1beta1.MemoryReservation {
	if str == "" {
		return nil
	}
	reservations := []kubeletconfigv1beta1.MemoryReservation{}
	for _, reservation := range strings.Split(str, ";") {
		reservationParts := strings.Split(reservation, ":")
		nodeIndex := strToInt32(reservationParts[0])
		limits := v1.ResourceList{}
		for _, limit := range strings.Split(reservationParts[1], ",") {
			resourceNameAndQuantity := strings.Split(limit, "=")
			resourceName := v1.ResourceName(resourceNameAndQuantity[0])
			quantity := resource.MustParse(resourceNameAndQuantity[1])
			limits[resourceName] = quantity
		}
		reservations = append(reservations, kubeletconfigv1beta1.MemoryReservation{
			NumaNode: nodeIndex,
			Limits:   limits,
		})
	}
	return reservations
}

func strKeySliceValToMap(str string, strDelim string, pairDelim string) map[string][]string {
	if str == "" {
		return nil
	}
	m := make(map[string][]string)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		key := strings.TrimSpace(pair[0])
		val := []string{strings.TrimSpace(pair[1])}
		m[key] = val
	}
	return m
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
