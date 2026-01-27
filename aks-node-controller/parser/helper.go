/*
Portions Copyright (c) Microsoft Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Parser helpers are used to get values of the env variables and pass to the provision scripts execution. For example, default values, values computed by others, etc.
package parser

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/pkg/agent"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	//go:embed templates/kubenet-cni.json.gtpl
	kubenetTemplateContent []byte
	//go:embed  templates/containerd.toml.gtpl
	containerdConfigTemplateText string
	//nolint:gochecknoglobals
	containerdConfigTemplate = template.Must(
		template.New("containerdconfig").Funcs(getFuncMapForContainerdConfigTemplate()).Parse(containerdConfigTemplateText),
	)
	//go:embed  templates/containerd_no_GPU.toml.gtpl
	containerdConfigNoGPUTemplateText string
	//nolint:gochecknoglobals
	containerdConfigNoGPUTemplate = template.Must(
		template.New("nogpucontainerdconfig").Funcs(getFuncMapForContainerdConfigTemplate()).Parse(containerdConfigNoGPUTemplateText),
	)

	//go:embed templates/localdns.toml.gtpl
	localDnsCorefileTemplateText string
	//nolint:gochecknoglobals
	localDnsCorefileTemplate = template.Must(
		template.New("localdnscorefile").Funcs(getFuncMapForLocalDnsCorefileTemplate()).Parse(localDnsCorefileTemplateText),
	)
)

func getFuncMap() template.FuncMap {
	return template.FuncMap{
		"getInitAKSCustomCloudFilepath": getInitAKSCustomCloudFilepath,
		"getIsAksCustomCloud":           getIsAksCustomCloud,
	}
}

func getFuncMapForContainerdConfigTemplate() template.FuncMap {
	return template.FuncMap{
		"derefBool":                        deref[bool],
		"getEnsureNoDupePromiscuousBridge": getEnsureNoDupePromiscuousBridge,
		"isKubernetesVersionGe":            helpers.IsKubernetesVersionGe,
		"getHasDataDir":                    getHasDataDir,
		"getEnableNvidia":                  getEnableNvidia,
	}
}

func getStringFromVMType(enum aksnodeconfigv1.VmType) string {
	switch enum {
	case aksnodeconfigv1.VmType_VM_TYPE_STANDARD:
		return helpers.VMTypeStandard
	case aksnodeconfigv1.VmType_VM_TYPE_VMSS:
		return helpers.VMTypeVmss
	case aksnodeconfigv1.VmType_VM_TYPE_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

//nolint:exhaustive // NetworkPlugin_NETWORK_PLUGIN_NONE and NetworkPlugin_NETWORK_PLUGIN_UNSPECIFIED should both return ""
func getStringFromNetworkPluginType(enum aksnodeconfigv1.NetworkPlugin) string {
	switch enum {
	case aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE:
		return helpers.NetworkPluginAzure
	case aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET:
		return helpers.NetworkPluginKubenet
	default:
		return ""
	}
}

//nolint:exhaustive // NetworkPolicy_NETWORK_POLICY_NONE and NetworkPolicy_NETWORK_POLICY_UNSPECIFIED should both return ""
func getStringFromNetworkPolicyType(enum aksnodeconfigv1.NetworkPolicy) string {
	switch enum {
	case aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_AZURE:
		return helpers.NetworkPolicyAzure
	case aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_CALICO:
		return helpers.NetworkPolicyCalico
	default:
		return ""
	}
}

//nolint:exhaustive // Default and LoadBalancerConfig_UNSPECIFIED should both return ""
func getStringFromLoadBalancerSkuType(enum aksnodeconfigv1.LoadBalancerSku) string {
	switch enum {
	case aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_BASIC:
		return helpers.LoadBalancerBasic
	case aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_STANDARD:
		return helpers.LoadBalancerStandard
	default:
		return ""
	}
}

// deref is a helper function to dereference a pointer of any type to its value.
func deref[T interface{}](p *T) T {
	if p == nil {
		var zeroValue T
		return zeroValue
	}
	return *p
}

func getStringifiedStringArray(arr []string, delimiter string) string {
	if len(arr) == 0 {
		return ""
	}

	return strings.Join(arr, delimiter)
}

// getKubenetTemplate returns the base64 encoded Kubenet template.
func getKubenetTemplate() string {
	return base64.StdEncoding.EncodeToString(kubenetTemplateContent)
}

// getContainerdConfigBase64 returns the base64 encoded containerd config depending on whether the node is with GPU or not.
func getContainerdConfigBase64(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if aksnodeconfig == nil {
		return ""
	}

	containerdConfig, err := containerdConfigFromAKSNodeConfig(aksnodeconfig, false)
	if err != nil {
		return fmt.Sprintf("error getting containerd config from node bootstrap variables: %v", err)
	}

	return base64.StdEncoding.EncodeToString([]byte(containerdConfig))
}

// getNoGPUContainerdConfigBase64 returns the base64 encoded containerd config depending on whether the node is with GPU or not.
func getNoGPUContainerdConfigBase64(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if aksnodeconfig == nil {
		return ""
	}

	containerdConfig, err := containerdConfigFromAKSNodeConfig(aksnodeconfig, true)
	if err != nil {
		return fmt.Sprintf("error getting No GPU containerd config from node bootstrap variables: %v", err)
	}

	return base64.StdEncoding.EncodeToString([]byte(containerdConfig))
}

func containerdConfigFromAKSNodeConfig(aksnodeconfig *aksnodeconfigv1.Configuration, noGPU bool) (string, error) {
	if aksnodeconfig == nil {
		return "", fmt.Errorf("AKSNodeConfig is nil")
	}

	// the containerd config template is different based on whether the node is with GPU or not.
	_template := containerdConfigTemplate
	if noGPU {
		_template = containerdConfigNoGPUTemplate
	}

	var buffer bytes.Buffer
	if err := _template.Execute(&buffer, aksnodeconfig); err != nil {
		return "", fmt.Errorf("error executing containerd config template for AKSNodeConfig: %w", err)
	}

	return buffer.String(), nil
}

func getIsMIGNode(gpuInstanceProfile string) bool {
	return gpuInstanceProfile != ""
}

func getCustomCACertsStatus(customCACerts []string) bool {
	return len(customCACerts) > 0
}

func getEnableSecureTLSBootstrapping(bootstrapConfig *aksnodeconfigv1.BootstrappingConfig) bool {
	// TODO: Change logic to default to true once Secure TLS Bootstrapping is complete
	return bootstrapConfig.GetBootstrappingAuthMethod() == aksnodeconfigv1.BootstrappingAuthMethod_BOOTSTRAPPING_AUTH_METHOD_SECURE_TLS_BOOTSTRAPPING
}

func getEnsureNoDupePromiscuousBridge(nc *aksnodeconfigv1.NetworkConfig) bool {
	return nc.GetNetworkPlugin() == aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET && nc.GetNetworkPolicy() != aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_CALICO
}

func getHasSearchDomain(csd *aksnodeconfigv1.CustomSearchDomainConfig) bool {
	if csd.GetDomainName() != "" && csd.GetRealmUser() != "" && csd.GetRealmPassword() != "" {
		return true
	}
	return false
}

func getCSEHelpersFilepath() string {
	return cseHelpersScriptFilepath
}

func getCSEDistroHelpersFilepath() string {
	return cseHelpersScriptDistroFilepath
}

func getCSEInstallFilepath() string {
	return cseInstallScriptFilepath
}

func getCSEDistroInstallFilepath() string {
	return cseInstallScriptDistroFilepath
}

func getCSEConfigFilepath() string {
	return cseConfigScriptFilepath
}

func getCustomSearchDomainFilepath() string {
	return customSearchDomainsCSEScriptFilepath
}

func getDHCPV6ServiceFilepath() string {
	return dhcpV6ServiceCSEScriptFilepath
}

func getDHCPV6ConfigFilepath() string {
	return dhcpV6ConfigCSEScriptFilepath
}

// getSysctlContent converts aksnodeconfigv1.SysctlConfig to a string with key=value pairs, with default values.
//
//gocyclo:ignore
//nolint:funlen,gocognit,cyclop // This function is long because it has to handle all the sysctl values.
func getSysctlContent(s *aksnodeconfigv1.SysctlConfig) string {
	// This is a partial workaround to this upstream Kubernetes issue:
	// https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731

	if s == nil {
		// If the sysctl config is nil, setting it to non-nil so that it can go through the defaulting logic below to get the default values.
		s = &aksnodeconfigv1.SysctlConfig{}
	}

	m := make(map[string]interface{})
	m["net.ipv4.tcp_retries2"] = defaultNetIpv4TcpRetries2
	m["net.core.message_burst"] = defaultNetCoreMessageBurst
	m["net.core.message_cost"] = defaultNetCoreMessageCost

	// Access the variable directly, instead of using the getter, so that it knows whether it's nil or not.
	// This is based on protobuf3 explicit presence feature.
	// Other directly access variables in this function implies the same idea.
	if s.NetCoreSomaxconn == nil {
		m["net.core.somaxconn"] = defaultNetCoreSomaxconn
	} else {
		// either using getter for NetCoreSomaxconn or direct access is fine because we ensure it's not nil.
		m["net.core.somaxconn"] = s.GetNetCoreSomaxconn()
	}

	if s.NetIpv4TcpMaxSynBacklog == nil {
		m["net.ipv4.tcp_max_syn_backlog"] = defaultNetIpv4TcpMaxSynBacklog
	} else {
		m["net.ipv4.tcp_max_syn_backlog"] = s.GetNetIpv4TcpMaxSynBacklog()
	}

	if s.NetIpv4NeighDefaultGcThresh1 == nil {
		m["net.ipv4.neigh.default.gc_thresh1"] = defaultNetIpv4NeighDefaultGcThresh1
	} else {
		m["net.ipv4.neigh.default.gc_thresh1"] = s.GetNetIpv4NeighDefaultGcThresh1()
	}

	if s.NetIpv4NeighDefaultGcThresh2 == nil {
		m["net.ipv4.neigh.default.gc_thresh2"] = defaultNetIpv4NeighDefaultGcThresh2
	} else {
		m["net.ipv4.neigh.default.gc_thresh2"] = s.GetNetIpv4NeighDefaultGcThresh2()
	}

	if s.NetIpv4NeighDefaultGcThresh3 == nil {
		m["net.ipv4.neigh.default.gc_thresh3"] = defaultNetIpv4NeighDefaultGcThresh3
	} else {
		m["net.ipv4.neigh.default.gc_thresh3"] = s.GetNetIpv4NeighDefaultGcThresh3()
	}

	if s.NetCoreNetdevMaxBacklog != nil {
		m["net.core.netdev_max_backlog"] = s.GetNetCoreNetdevMaxBacklog()
	}

	if s.NetCoreRmemDefault != nil {
		m["net.core.rmem_default"] = s.GetNetCoreRmemDefault()
	}

	if s.NetCoreRmemMax != nil {
		m["net.core.rmem_max"] = s.GetNetCoreRmemMax()
	}

	if s.NetCoreWmemDefault != nil {
		m["net.core.wmem_default"] = s.GetNetCoreWmemDefault()
	}

	if s.NetCoreWmemMax != nil {
		m["net.core.wmem_max"] = s.GetNetCoreWmemMax()
	}

	if s.NetCoreOptmemMax != nil {
		m["net.core.optmem_max"] = s.GetNetCoreOptmemMax()
	}

	if s.NetIpv4TcpMaxTwBuckets != nil {
		m["net.ipv4.tcp_max_tw_buckets"] = s.GetNetIpv4TcpMaxTwBuckets()
	}

	if s.NetIpv4TcpFinTimeout != nil {
		m["net.ipv4.tcp_fin_timeout"] = s.GetNetIpv4TcpFinTimeout()
	}

	if s.NetIpv4TcpKeepaliveTime != nil {
		m["net.ipv4.tcp_keepalive_time"] = s.GetNetIpv4TcpKeepaliveTime()
	}

	if s.NetIpv4TcpKeepaliveProbes != nil {
		m["net.ipv4.tcp_keepalive_probes"] = s.GetNetIpv4TcpKeepaliveProbes()
	}

	if s.NetIpv4TcpkeepaliveIntvl != nil {
		m["net.ipv4.tcp_keepalive_intvl"] = s.GetNetIpv4TcpkeepaliveIntvl()
	}

	if s.NetIpv4TcpTwReuse != nil {
		if s.GetNetIpv4TcpTwReuse() {
			m["net.ipv4.tcp_tw_reuse"] = 1
		} else {
			m["net.ipv4.tcp_tw_reuse"] = 0
		}
	}

	if s.GetNetIpv4IpLocalPortRange() != "" {
		m["net.ipv4.ip_local_port_range"] = s.GetNetIpv4IpLocalPortRange()
		if getPortRangeEndValue(s.GetNetIpv4IpLocalPortRange()) > ipLocalReservedPorts {
			m["net.ipv4.ip_local_reserved_ports"] = ipLocalReservedPorts
		}
	}

	if s.NetNetfilterNfConntrackMax != nil {
		m["net.netfilter.nf_conntrack_max"] = s.GetNetNetfilterNfConntrackMax()
	}

	if s.NetNetfilterNfConntrackBuckets != nil {
		m["net.netfilter.nf_conntrack_buckets"] = s.GetNetNetfilterNfConntrackBuckets()
	}

	if s.FsInotifyMaxUserWatches != nil {
		m["fs.inotify.max_user_watches"] = s.GetFsInotifyMaxUserWatches()
	}

	if s.FsFileMax != nil {
		m["fs.file-max"] = s.GetFsFileMax()
	}

	if s.FsAioMaxNr != nil {
		m["fs.aio-max-nr"] = s.GetFsAioMaxNr()
	}

	if s.FsNrOpen != nil {
		m["fs.nr_open"] = s.GetFsNrOpen()
	}

	if s.KernelThreadsMax != nil {
		m["kernel.threads-max"] = s.GetKernelThreadsMax()
	}

	if s.VmMaxMapCount != nil {
		m["vm.max_map_count"] = s.GetVmMaxMapCount()
	}

	if s.VmSwappiness != nil {
		m["vm.swappiness"] = s.GetVmSwappiness()
	}

	if s.VmVfsCachePressure != nil {
		m["vm.vfs_cache_pressure"] = s.GetVmVfsCachePressure()
	}

	return base64.StdEncoding.EncodeToString([]byte(createSortedKeyValuePairs(m, "\n")))
}

func getShouldConfigContainerdUlimits(u *aksnodeconfigv1.UlimitConfig) bool {
	return u != nil
}

// getUlimitContent converts aksnodeconfigv1.UlimitConfig to a string with key=value pairs.
func getUlimitContent(u *aksnodeconfigv1.UlimitConfig) string {
	if u == nil {
		return ""
	}

	header := "[Service]\n"
	m := make(map[string]string)
	if u.NoFile != nil {
		m["LimitNOFILE"] = u.GetNoFile()
	}

	if u.MaxLockedMemory != nil {
		m["LimitMEMLOCK"] = u.GetMaxLockedMemory()
	}

	return header + createSortedKeyValuePairs(m, " ")
}

// getPortRangeEndValue returns the end value of the port range where the input is in the format of "start end".
func getPortRangeEndValue(portRange string) int {
	if portRange == "" {
		return -1
	}

	arr := strings.Split(portRange, " ")

	// we are expecting only two values, start and end.
	if len(arr) != MinArgs {
		return -1
	}

	var start, end int
	var err error

	// the start value should be a valid port number.
	if start, err = strconv.Atoi(arr[0]); err != nil {
		log.Printf("error converting port range start value to int: %v", err)
		return -1
	}

	// the end value should be a valid port number.
	if end, err = strconv.Atoi(arr[1]); err != nil {
		log.Printf("error converting port range end value to int: %v", err)
		return -1
	}

	if start <= 0 || end <= 0 {
		log.Printf("port range values should be greater than 0: %d", start)
		return -1
	}

	if start >= end {
		log.Printf("port range end value should be greater than the start value: %d >= %d", start, end)
		return -1
	}

	return end
}

// createSortedKeyValuePairs creates a string with key=value pairs, sorted by key, with custom delimiter.
func createSortedKeyValuePairs[T any](m map[string]T, delimiter string) string {
	keys := []string{}
	for key := range m {
		keys = append(keys, key)
	}

	// we are sorting the keys for deterministic output for readability and testing.
	sort.Strings(keys)
	var buf bytes.Buffer
	i := 0
	for _, key := range keys {
		i++
		// set the last delimiter to empty string
		if i == len(keys) {
			delimiter = ""
		}
		buf.WriteString(fmt.Sprintf("%s=%v%s", key, m[key], delimiter))
	}
	return buf.String()
}

func getExcludeMasterFromStandardLB(lb *aksnodeconfigv1.LoadBalancerConfig) bool {
	if lb == nil || lb.ExcludeMasterFromStandardLoadBalancer == nil {
		return true
	}
	return lb.GetExcludeMasterFromStandardLoadBalancer()
}

func getMaxLBRuleCount(lb *aksnodeconfigv1.LoadBalancerConfig) int32 {
	if lb == nil || lb.MaxLoadBalancerRuleCount == nil {
		return int32(maxLBRuleCountDefault)
	}
	return lb.GetMaxLoadBalancerRuleCount()
}

func getGpuImageSha(vmSize string) string {
	return agent.GetAKSGPUImageSHA(vmSize)
}

func getGpuDriverType(vmSize string) string {
	return agent.GetGPUDriverType(vmSize)
}

func getGpuDriverVersion(vmSize string) string {
	return agent.GetGPUDriverVersion(vmSize)
}

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support.
func getIsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case helpers.VMSizeStandardDc2s, helpers.VMSizeStandardDc4s:
		return true
	}
	return false
}

func getShouldConfigureHTTPProxy(httpProxyConfig *aksnodeconfigv1.HttpProxyConfig) bool {
	return httpProxyConfig.GetHttpProxy() != "" || httpProxyConfig.GetHttpsProxy() != ""
}

func getShouldConfigureHTTPProxyCA(httpProxyConfig *aksnodeconfigv1.HttpProxyConfig) bool {
	return httpProxyConfig.GetProxyTrustedCa() != ""
}

func getIsAksCustomCloud(customCloudConfig *aksnodeconfigv1.CustomCloudConfig) bool {
	return strings.EqualFold(customCloudConfig.GetCustomCloudEnvName(), helpers.AksCustomCloudName)
}

/* GetCloudTargetEnv determines and returns whether the region is a sovereign cloud which
have their own data compliance regulations (China/Germany/USGov) or standard.  */
// Azure public cloud.
func getCloudTargetEnv(v *aksnodeconfigv1.Configuration) string {
	loc := strings.ToLower(strings.Join(strings.Fields(v.GetClusterConfig().GetLocation()), ""))
	switch {
	case strings.HasPrefix(loc, "china"):
		return "AzureChinaCloud"
	case loc == "germanynortheast" || loc == "germanycentral":
		return "AzureGermanCloud"
	case strings.HasPrefix(loc, "usgov") || strings.HasPrefix(loc, "usdod"):
		return "AzureUSGovernmentCloud"
	default:
		return helpers.DefaultCloudName
	}
}

func getTargetEnvironment(v *aksnodeconfigv1.Configuration) string {
	if getIsAksCustomCloud(v.GetCustomCloudConfig()) {
		return helpers.AksCustomCloudName
	}
	return getCloudTargetEnv(v)
}

func getTargetCloud(v *aksnodeconfigv1.Configuration) string {
	if getIsAksCustomCloud(v.GetCustomCloudConfig()) {
		return helpers.AzureStackCloud
	}
	return getTargetEnvironment(v)
}

func getAzureEnvironmentFilepath(v *aksnodeconfigv1.Configuration) string {
	if getIsAksCustomCloud(v.GetCustomCloudConfig()) {
		return fmt.Sprintf("/etc/kubernetes/%s.json", getTargetEnvironment(v))
	}
	return ""
}

func getLinuxAdminUsername(username string) string {
	if username == "" {
		return helpers.DefaultLinuxUser
	}
	return username
}

func getIsVHD(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func getDisableSSH(v *aksnodeconfigv1.Configuration) bool {
	if v.EnableSsh == nil {
		return false
	}
	return !v.GetEnableSsh()
}

func getServicePrincipalFileContent(authConfig *aksnodeconfigv1.AuthConfig) string {
	if authConfig.GetServicePrincipalSecret() == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(authConfig.GetServicePrincipalSecret()))
}

func getKubeletFlags(kubeletConfig *aksnodeconfigv1.KubeletConfig) string {
	return createSortedKeyValuePairs(kubeletConfig.GetKubeletFlags(), " ")
}

func marshalToJSON(v any) ([]byte, error) {
	// Originally we can set the Multiline here and it will marshal to a JSON we can use.
	// However the protojson team intentionally randomly add extra whitespace after the key in the key-value.
	// E.g., Sometimes it is "key": "value" and sometimes it is "key":  "value".
	// They did it intentionally to make the output format not reliable,
	// because they think it is not a good idea to rely on the JSON output format.
	// ref: https://github.com/protocolbuffers/protobuf-go/commit/582ab3de426ef0758666e018b422dd20390f7f26
	marshaler := &protojson.MarshalOptions{
		Indent: "",
	}

	// Therefore, we have to implement our own re-formatting to make the output reliable as we expected.
	switch v := v.(type) {
	case *aksnodeconfigv1.KubeletConfigFileConfig:
		data, err := marshaler.Marshal(v)
		if err != nil {
			log.Printf("error marshalling: %v", err)
			return nil, err
		}

		var rawMessage json.RawMessage = data
		jsonByte, err := json.MarshalIndent(rawMessage, "", "  ")
		if err != nil {
			log.Printf("error marshalling kubelet config file content: %v", err)
			return nil, err
		}
		return jsonByte, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}

// getKubeletConfigFileContent converts kubelet flags we set to a file, and return the json content.
func getKubeletConfigFileContent(kubeletConfig *aksnodeconfigv1.KubeletConfig) string {
	if kubeletConfig == nil {
		return ""
	}
	kubeletConfigFileConfig := kubeletConfig.GetKubeletConfigFileConfig()
	kubeletConfigFileConfigByte, err := marshalToJSON(kubeletConfigFileConfig)
	if err != nil {
		log.Printf("error marshalling kubelet config file content: %v", err)
		return ""
	}
	return string(kubeletConfigFileConfigByte)
}

func getKubeletConfigFileContentBase64(kubeletConfig *aksnodeconfigv1.KubeletConfig) string {
	return base64.StdEncoding.EncodeToString([]byte(getKubeletConfigFileContent(kubeletConfig)))
}

func getEnableSwapConfig(v *aksnodeconfigv1.CustomLinuxOsConfig) bool {
	return v.GetEnableSwapConfig() && v.GetSwapFileSize() > 0
}

func getShouldConfigTransparentHugePage(v *aksnodeconfigv1.CustomLinuxOsConfig) bool {
	return v.GetTransparentDefrag() != "" || v.GetTransparentHugepageSupport() != ""
}

func getProxyVariables(proxyConfig *aksnodeconfigv1.HttpProxyConfig) string {
	// only use https proxy, if user doesn't specify httpsProxy we autofill it with value from httpProxy.
	proxyVars := ""
	if proxyConfig.GetHttpProxy() != "" {
		// from https://curl.se/docs/manual.html, curl uses http_proxy but uppercase for others?
		proxyVars = fmt.Sprintf("export http_proxy=\"%s\";", proxyConfig.GetHttpProxy())
	}
	if proxyConfig.GetHttpsProxy() != "" {
		proxyVars = fmt.Sprintf("export HTTPS_PROXY=\"%s\"; %s", proxyConfig.GetHttpsProxy(), proxyVars)
	}
	if proxyConfig.GetNoProxyEntries() != nil {
		proxyVars = fmt.Sprintf("export NO_PROXY=\"%s\"; %s", strings.Join(proxyConfig.GetNoProxyEntries(), ","), proxyVars)
	}
	return proxyVars
}

func getHasDataDir(kubeletConfig *aksnodeconfigv1.KubeletConfig) bool {
	return kubeletConfig.GetContainerDataDir() != ""
}

func getHasKubeletDiskType(kubeletConfig *aksnodeconfigv1.KubeletConfig) bool {
	return kubeletConfig.GetKubeletDiskType() == aksnodeconfigv1.KubeletDisk_KUBELET_DISK_TEMP_DISK
}

func getInitAKSCustomCloudFilepath() string {
	return initAKSCustomCloudFilepath
}

func getGPUNeedsFabricManager(vmSize string) bool {
	return agent.GPUNeedsFabricManager(vmSize)
}

func getEnableNvidia(config *aksnodeconfigv1.Configuration) bool {
	if config.GpuConfig != nil && config.GpuConfig.EnableNvidia != nil {
		return *config.GpuConfig.EnableNvidia
	}
	return false
}

func removeNewlines(str string) string {
	sanitizedStr := strings.ReplaceAll(str, "\n", "")
	sanitizedStr = strings.ReplaceAll(sanitizedStr, "\r", "")
	return sanitizedStr
}

// ---------------------- Start of localdns related helper code ----------------------//

// FuncMap used for generating localdns corefile.
// getLocalDnsNodeListenerIp, getLocalDnsClusterListenerIp, getAzureDnsIp and getCoreDnsServiceIp have their place holder in
// localdns.toml.gtpl template and will be replaced by the values returned from these functions.
func getFuncMapForLocalDnsCorefileTemplate() template.FuncMap {
	return template.FuncMap{
		"hasSuffix":                    strings.HasSuffix,
		"getLocalDnsNodeListenerIp":    getLocalDnsNodeListenerIp,
		"getLocalDnsClusterListenerIp": getLocalDnsClusterListenerIp,
		"getAzureDnsIp":                getAzureDnsIp,
		"getCoreDnsServiceIp":          getCoreDnsServiceIp,
	}
}

// getLocalDnsCorefileBase64 returns the base64 encoded LocalDns corefile.
// base64 encoded corefile returned from this function will decoded and written
// to /opt/azure/containers/localdns/localdns.corefile in cse_config.sh
// and then used by localdns systemd unit to start localdns systemd unit.
func getLocalDnsCorefileBase64(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if aksnodeconfig == nil {
		return ""
	}
	// If LocalDnsProfile is nil or EnableLocalDns is false, return empty string.
	// This means localdns is not enabled for the agent pool.
	// In this case we don't need to generate localdns corefile.
	if aksnodeconfig.GetLocalDnsProfile() == nil {
		return ""
	}
	if !aksnodeconfig.GetLocalDnsProfile().GetEnableLocalDns() {
		return ""
	}

	localDnsConfig, err := generateLocalDnsCorefileFromAKSNodeConfig(aksnodeconfig)
	if err != nil {
		return fmt.Sprintf("error getting localdns corfile from aks node config: %v", err)
	}
	return base64.StdEncoding.EncodeToString([]byte(localDnsConfig))
}

// Corefile is created using localdns.toml.gtpl template and aksnodeconfig values.
func generateLocalDnsCorefileFromAKSNodeConfig(aksnodeconfig *aksnodeconfigv1.Configuration) (string, error) {
	var corefileBuffer bytes.Buffer
	if err := localDnsCorefileTemplate.Execute(&corefileBuffer, aksnodeconfig); err != nil {
		return "", fmt.Errorf("failed to execute localdns corefile template: %w", err)
	}
	return corefileBuffer.String(), nil
}

// getLocalDnsClusterListenerIp returns APIPA-IP address that will be used in localdns systemd unit.
func getLocalDnsClusterListenerIp() string {
	return localDnsClusterListenerIp
}

// getLocalDnsNodeListenerIp returns APIPA-IP address that will be used in localdns systemd unit.
func getLocalDnsNodeListenerIp() string {
	return localDnsNodeListenerIp
}

// getAzureDnsIp returns 168.63.129.16 address.
func getAzureDnsIp() string {
	return azureDnsIp
}

func getCoreDnsServiceIp(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if aksnodeconfig != nil && aksnodeconfig.GetClusterConfig() != nil && aksnodeconfig.GetClusterConfig().GetClusterNetworkConfig() != nil {
		if coreDnsServiceIP := aksnodeconfig.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp(); coreDnsServiceIP != "" {
			return coreDnsServiceIP
		}
	}
	return defaultCoreDnsServiceIp
}

// shouldEnableLocalDns returns true if aksnodeconfig, LocalDnsProfile is not nil and
// EnableLocalDns for the Agentpool is true. EnableLocalDns boolean value is sent by AKS RP.
// This will tell if localdns should be enabled for the agent pool or not.
// If this function returns true only then we generate localdns corefile.
func shouldEnableLocalDns(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	return fmt.Sprintf("%v", aksnodeconfig != nil && aksnodeconfig.GetLocalDnsProfile() != nil && aksnodeconfig.GetLocalDnsProfile().GetEnableLocalDns())
}

// getLocalDnsCpuLimitInPercentage returns CPU limit in percentage unit that will be used in localdns systemd unit.
func getLocalDnsCpuLimitInPercentage(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if shouldEnableLocalDns(aksnodeconfig) == "true" && aksnodeconfig.GetLocalDnsProfile().GetCpuLimitInMilliCores() != 0 {
		// Convert milli-cores to percentage and return as formatted string.
		return fmt.Sprintf("%.1f%%", float64(aksnodeconfig.GetLocalDnsProfile().GetCpuLimitInMilliCores())/10.0)
	}
	return defaultLocalDnsCpuLimitInPercentage
}

// getLocalDnsMemoryLimitInMb returns memory limit in MB that will be used in localdns systemd unit.
func getLocalDnsMemoryLimitInMb(aksnodeconfig *aksnodeconfigv1.Configuration) string {
	if shouldEnableLocalDns(aksnodeconfig) == "true" && aksnodeconfig.GetLocalDnsProfile().GetMemoryLimitInMb() != 0 {
		// Return memory limit as a string with "M" suffix.
		return fmt.Sprintf("%dM", aksnodeconfig.GetLocalDnsProfile().GetMemoryLimitInMb())
	}
	return defaultLocalDnsMemoryLimitInMb
}

// ---------------------- End of localdns related helper code ----------------------//
