package parser

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/node-bootstrapper/utils"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

var (
	//go:embed templates/cse_cmd.sh.gtpl
	bootstrapTrigger         string
	bootstrapTriggerTemplate = template.Must(template.New("triggerBootstrapScript").Funcs(getFuncMap()).Parse(bootstrapTrigger)) //nolint:gochecknoglobals

)

func executeBootstrapTemplate(inputContract *nbcontractv1.Configuration) (string, error) {
	var buffer bytes.Buffer
	if err := bootstrapTriggerTemplate.Execute(&buffer, inputContract); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// this function will eventually take a pointer to the bootstrap contract struct.
// it will then template out the variables into the final bootstrap trigger script.
func Parse(inputJSON []byte) (utils.SensitiveString, error) {
	// Parse the JSON into a nbcontractv1.Configuration struct
	var nbc nbcontractv1.Configuration
	err := json.Unmarshal(inputJSON, &nbc)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal the json to nbcontractv1: %w", err)
	}

	if nbc.Version != "v0" {
		return "", fmt.Errorf("unsupported version: %s", nbc.Version)
	}

	triggerBootstrapScript, err := executeBootstrapTemplate(&nbc)
	if err != nil {
		return "", fmt.Errorf("failed to execute the template: %w", err)
	}

	// Convert to one-liner
	return utils.SensitiveString(strings.ReplaceAll(triggerBootstrapScript, "\n", " ")), nil
}
func getCSEEnv(config *nbcontractv1.Configuration) map[string]string {
	env := make(map[string]string)

	env["PROVISION_OUTPUT"] = "/var/log/azure/cluster-provision.log;"
	env["MOBY_VERSION"] = ""
	env["CLOUDPROVIDER_BACKOFF"] = "true"
	env["CLOUDPROVIDER_BACKOFF_MODE"] = "v2"
	env["CLOUDPROVIDER_BACKOFF_RETRIES"] = "6"
	env["CLOUDPROVIDER_BACKOFF_EXPONENT"] = "0"
	env["CLOUDPROVIDER_BACKOFF_DURATION"] = "5"
	env["CLOUDPROVIDER_BACKOFF_JITTER"] = "0"
	env["CLOUDPROVIDER_RATELIMIT"] = "true"
	env["CLOUDPROVIDER_RATELIMIT_QPS"] = "10"
	env["CLOUDPROVIDER_RATELIMIT_QPS_WRITE"] = "10"
	env["CLOUDPROVIDER_RATELIMIT_BUCKET"] = "100"
	env["CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE"] = "100"
	env["CONTAINER_RUNTIME"] = " containerd"
	env["CLI_TOOL"] = "ctr"
	env["NETWORK_MODE"] = "transparent"
	env["NEEDS_CONTAINERD"] = "true"
	env["NEEDS_DOCKER_LOGIN"] = "false"

	env["ADMINUSER"] = getLinuxAdminUsername(config.GetLinuxAdminUsername())
	env["TENANT_ID"] = config.AuthConfig.GetTenantId()
	env["KUBERNETES_VERSION"] = config.GetKubernetesVersion()
	env["KUBE_BINARY_URL"] = config.KubeBinaryConfig.GetKubeBinaryUrl()
	env["CUSTOM_KUBE_BINARY_URL"] = config.KubeBinaryConfig.GetCustomKubeBinaryUrl()
	env["PRIVATE_KUBE_BINARY_URL"] = config.KubeBinaryConfig.GetPrivateKubeBinaryUrl()
	env["KUBEPROXY_URL"] = config.GetKubeProxyUrl()
	env["APISERVER_PUBLIC_KEY"] = config.ApiServerConfig.GetApiServerPublicKey()
	env["SUBSCRIPTION_ID"] = config.AuthConfig.GetSubscriptionId()
	env["RESOURCE_GROUP"] = config.ClusterConfig.GetResourceGroup()
	env["LOCATION"] = config.ClusterConfig.GetLocation()
	env["VM_TYPE"] = getStringFromVMType(config.ClusterConfig.GetVmType())
	env["SUBNET"] = config.ClusterConfig.GetClusterNetworkConfig().GetSubnet()
	env["NETWORK_SECURITY_GROUP"] = config.ClusterConfig.GetClusterNetworkConfig().GetSecurityGroupName()
	env["VIRTUAL_NETWORK"] = config.ClusterConfig.GetClusterNetworkConfig().GetVnetName()
	env["VIRTUAL_NETWORK_RESOURCE_GROUP"] = config.ClusterConfig.GetClusterNetworkConfig().GetVnetResourceGroup()
	env["ROUTE_TABLE"] = config.ClusterConfig.GetClusterNetworkConfig().GetRouteTable()
	env["PRIMARY_AVAILABILITY_SET"] = config.ClusterConfig.GetPrimaryAvailabilitySet()
	env["PRIMARY_SCALE_SET"] = config.ClusterConfig.GetPrimaryScaleSet()
	env["SERVICE_PRINCIPAL_CLIENT_ID"] = config.AuthConfig.GetServicePrincipalId()
	env["NETWORK_PLUGIN"] = getStringFromNetworkPluginType(config.GetNetworkConfig().GetNetworkPlugin())
	env["NETWORK_POLICY"] = getStringFromNetworkPolicyType(config.GetNetworkConfig().GetNetworkPolicy())
	env["VNET_CNI_PLUGINS_URL"] = config.GetNetworkConfig().GetVnetCniPluginsUrl()
	env["LOAD_BALANCER_DISABLE_OUTBOUND_SNAT"] = fmt.Sprintf("%v", config.ClusterConfig.GetLoadBalancerConfig().GetDisableOutboundSnat())
	env["USE_MANAGED_IDENTITY_EXTENSION"] = fmt.Sprintf("%v", config.AuthConfig.GetUseManagedIdentityExtension())
	env["USE_INSTANCE_METADATA"] = fmt.Sprintf("%v", config.ClusterConfig.GetUseInstanceMetadata())
	env["LOAD_BALANCER_SKU"] = getStringFromLoadBalancerSkuType(config.ClusterConfig.GetLoadBalancerConfig().GetLoadBalancerSku())
	env["EXCLUDE_MASTER_FROM_STANDARD_LB"] = fmt.Sprintf("%v", getExcludeMasterFromStandardLB(config.ClusterConfig.GetLoadBalancerConfig()))
	env["MAXIMUM_LOADBALANCER_RULE_COUNT"] = fmt.Sprintf("%v", getMaxLBRuleCount(config.ClusterConfig.GetLoadBalancerConfig()))
	env["CONTAINERD_DOWNLOAD_URL_BASE"] = config.ContainerdConfig.GetContainerdDownloadUrlBase()
	env["USER_ASSIGNED_IDENTITY_ID"] = config.AuthConfig.GetAssignedIdentityId()
	env["API_SERVER_NAME"] = config.ApiServerConfig.GetApiServerName()
	env["IS_VHD"] = fmt.Sprintf("%v", getIsVHD(config.IsVhd))
	env["GPU_NODE"] = fmt.Sprintf("%v", getEnableNvidia(config))
	env["SGX_NODE"] = fmt.Sprintf("%v", getIsSgxEnabledSKU(config.VmSize))
	env["MIG_NODE"] = fmt.Sprintf("%v", getIsMIGNode(config.GpuConfig.GetGpuInstanceProfile()))
	env["CONFIG_GPU_DRIVER_IF_NEEDED"] = fmt.Sprintf("%v", config.GpuConfig.GetConfigGpuDriver())
	env["ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED"] = fmt.Sprintf("%v", config.GpuConfig.GetGpuDevicePlugin())
	env["TELEPORTD_PLUGIN_DOWNLOAD_URL"] = config.TeleportConfig.GetTeleportdPluginDownloadUrl()
	env["CREDENTIAL_PROVIDER_DOWNLOAD_URL"] = config.KubeBinaryConfig.GetLinuxCredentialProviderUrl()
	env["CONTAINERD_VERSION"] = config.ContainerdConfig.GetContainerdVersion()
	env["CONTAINERD_PACKAGE_URL"] = config.ContainerdConfig.GetContainerdPackageUrl()
	env["RUNC_VERSION"] = config.RuncConfig.GetRuncVersion()
	env["RUNC_PACKAGE_URL"] = config.RuncConfig.GetRuncPackageUrl()
	env["ENABLE_HOSTS_CONFIG_AGENT"] = fmt.Sprintf("%v", config.GetEnableHostsConfigAgent())
	env["DISABLE_SSH"] = fmt.Sprintf("%v", getDisableSSH(config))
	env["TELEPORT_ENABLED"] = fmt.Sprintf("%v", config.TeleportConfig.GetStatus())
	env["SHOULD_CONFIGURE_HTTP_PROXY"] = fmt.Sprintf("%v", getShouldConfigureHTTPProxy(config.HttpProxyConfig))
	env["SHOULD_CONFIGURE_HTTP_PROXY_CA"] = fmt.Sprintf("%v", getShouldConfigureHTTPProxyCA(config.HttpProxyConfig))
	env["HTTP_PROXY_TRUSTED_CA"] = config.HttpProxyConfig.GetProxyTrustedCa()
	env["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"] = fmt.Sprintf("%v", getCustomCACertsStatus(config.GetCustomCaCerts()))
	env["CUSTOM_CA_TRUST_COUNT"] = fmt.Sprintf("%v", len(config.GetCustomCaCerts()))
	for i, cert := range config.CustomCaCerts {
		env[fmt.Sprintf("CUSTOM_CA_CERT_%d", i)] = cert
	}
	env["IS_KRUSTLET"] = fmt.Sprintf("%v", getIsKrustlet(config.GetWorkloadRuntime()))
	env["GPU_NEEDS_FABRIC_MANAGER"] = fmt.Sprintf("%v", getGPUNeedsFabricManager(config.VmSize))
	env["IPV6_DUAL_STACK_ENABLED"] = fmt.Sprintf("%v", config.GetIpv6DualStackEnabled())
	env["OUTBOUND_COMMAND"] = config.GetOutboundCommand()
	env["ENABLE_UNATTENDED_UPGRADES"] = fmt.Sprintf("%v", config.GetEnableUnattendedUpgrade())
	env["ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE"] = fmt.Sprintf("%v", getEnsureNoDupePromiscuousBridge(config.GetNetworkConfig()))
	env["SHOULD_CONFIG_SWAP_FILE"] = fmt.Sprintf("%v", getEnableSwapConfig(config.CustomLinuxOsConfig))
	env["SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE"] = fmt.Sprintf("%v", getShouldConfigTransparentHugePage(config.CustomLinuxOsConfig))
	env["SHOULD_CONFIG_CONTAINERD_ULIMITS"] = fmt.Sprintf("%v", getShouldConfigContainerdUlimits(config.CustomLinuxOsConfig.GetUlimitConfig()))
	env["CONTAINERD_ULIMITS"] = getUlimitContent(config.CustomLinuxOsConfig.GetUlimitConfig())
	env["TARGET_CLOUD"] = getTargetCloud(config)
	env["TARGET_ENVIRONMENT"] = getTargetEnvironment(config)
	env["CUSTOM_ENV_JSON"] = config.CustomCloudConfig.GetCustomEnvJsonContent()
	env["IS_CUSTOM_CLOUD"] = fmt.Sprintf("%v", getIsAksCustomCloud(config.CustomCloudConfig))
	env["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"] = config.CustomCloudConfig.GetContainerRegistryDnsSuffix()
	env["CSE_HELPERS_FILEPATH"] = getCSEHelpersFilepath()
	env["CSE_DISTRO_HELPERS_FILEPATH"] = getCSEDistroHelpersFilepath()
	env["CSE_INSTALL_FILEPATH"] = getCSEInstallFilepath()
	env["CSE_DISTRO_INSTALL_FILEPATH"] = getCSEDistroInstallFilepath()
	env["CSE_CONFIG_FILEPATH"] = getCSEConfigFilepath()
	env["AZURE_PRIVATE_REGISTRY_SERVER"] = config.GetAzurePrivateRegistryServer()
	env["HAS_CUSTOM_SEARCH_DOMAIN"] = fmt.Sprintf("%v", getHasSearchDomain(config.GetCustomSearchDomainConfig()))
	env["CUSTOM_SEARCH_DOMAIN_FILEPATH"] = getCustomSearchDomainFilepath()
	env["HTTP_PROXY_URLS"] = config.HttpProxyConfig.GetHttpProxy()
	env["HTTPS_PROXY_URLS"] = config.HttpProxyConfig.GetHttpsProxy()
	env["NO_PROXY_URLS"] = getStringifiedStringArray(config.HttpProxyConfig.GetNoProxyEntries(), ",")
	env["PROXY_VARS"] = getProxyVariables(config.HttpProxyConfig)
	env["ENABLE_TLS_BOOTSTRAPPING"] = fmt.Sprintf("%v", getEnableTLSBootstrap(config.TlsBootstrappingConfig))
	env["ENABLE_SECURE_TLS_BOOTSTRAPPING"] = fmt.Sprintf("%v", getEnableSecureTLSBootstrap(config.TlsBootstrappingConfig))
	env["CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID"] = getCustomSecureTLSBootstrapAADServerAppID(config.TlsBootstrappingConfig)
	env["DHCPV6_SERVICE_FILEPATH"] = getDHCPV6ServiceFilepath()
	env["DHCPV6_CONFIG_FILEPATH"] = getDHCPV6ConfigFilepath()
	env["THP_ENABLED"] = config.CustomLinuxOsConfig.GetTransparentHugepageSupport()
	env["THP_DEFRAG"] = config.CustomLinuxOsConfig.GetTransparentDefrag()
	env["SERVICE_PRINCIPAL_FILE_CONTENT"] = getServicePrincipalFileContent(config.AuthConfig)
	env["KUBELET_CLIENT_CONTENT"] = config.KubeletConfig.GetKubeletClientKey()
	env["KUBELET_CLIENT_CONTENT"] = config.KubeletConfig.GetKubeletClientKey()
	env["KUBELET_CLIENT_CERT_CONTENT"] = config.KubeletConfig.GetKubeletClientCertContent()
	env["KUBELET_CONFIG_FILE_ENABLED"] = fmt.Sprintf("%v", config.KubeletConfig.GetEnableKubeletConfigFile())
	env["KUBELET_CONFIG_FILE_CONTENT"] = config.KubeletConfig.GetKubeletConfigFileContent()
	env["SWAP_FILE_SIZE_MB"] = fmt.Sprintf("%v", config.CustomLinuxOsConfig.GetSwapFileSize())
	env["GPU_DRIVER_VERSION"] = getGpuDriverVersion(config.VmSize)
	env["GPU_IMAGE_SHA"] = getGpuImageSha(config.VmSize)
	env["GPU_INSTANCE_PROFILE"] = config.GpuConfig.GetGpuInstanceProfile()
	env["CUSTOM_SEARCH_DOMAIN_NAME"] = config.CustomSearchDomainConfig.GetDomainName()
	env["CUSTOM_SEARCH_REALM_USER"] = config.CustomSearchDomainConfig.GetRealmUser()
	env["CUSTOM_SEARCH_REALM_PASSWORD"] = config.CustomSearchDomainConfig.GetRealmPassword()
	env["MESSAGE_OF_THE_DAY"] = config.GetMessageOfTheDay()
	env["HAS_KUBELET_DISK_TYPE"] = fmt.Sprintf("%v", getHasKubeletDiskType(config.KubeletConfig))
	env["NEEDS_CGROUPV2"] = fmt.Sprintf("%v", config.GetNeedsCgroupv2())
	env["TLS_BOOTSTRAP_TOKEN"] = getTLSBootstrapToken(config.TlsBootstrappingConfig)
	env["KUBELET_FLAGS"] = createSortedKeyValuePairs(config.KubeletConfig.GetKubeletFlags(), " ")
	env["NETWORK_POLICY"] = getStringFromNetworkPolicyType(config.NetworkConfig.GetNetworkPolicy())
	env["KUBELET_NODE_LABELS"] = createSortedKeyValuePairs(config.KubeletConfig.GetKubeletNodeLabels(), ",")
	env["AZURE_ENVIRONMENT_FILEPATH"] = getAzureEnvironmentFilepath(config)
	env["KUBE_CA_CRT"] = config.GetKubernetesCaCert()
	env["KUBENET_TEMPLATE"] = getKubenetTemplate()
	env["CONTAINERD_CONFIG_CONTENT"] = getContainerdConfig(config)
	env["IS_KATA"] = fmt.Sprintf("%v", config.GetIsKata())
	env["ARTIFACT_STREAMING_ENABLED"] = fmt.Sprintf("%v", config.GetEnableArtifactStreaming())
	env["SYSCTL_CONTENT"] = getSysctlContent(config.CustomLinuxOsConfig.GetSysctlConfig())
	env["PRIVATE_EGRESS_PROXY_ADDRESS"] = config.GetPrivateEgressProxyAddress()
	env["BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER"] = config.GetBootstrapProfileContainerRegistryServer()
	env["ENABLE_IMDS_RESTRICTION"] = fmt.Sprintf("%v", config.ImdsRestrictionConfig.GetEnableImdsRestriction())
	env["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"] = fmt.Sprintf("%v", config.ImdsRestrictionConfig.GetInsertImdsRestrictionRuleToMangleTable())

	return env
}

func BuildCSECmd(ctx context.Context, config *nbcontractv1.Configuration) (*exec.Cmd, error) {
	triggerBootstrapScript, err := executeBootstrapTemplate(config)
	if err != nil {
		return nil, fmt.Errorf("failed to execute the template: %w", err)
	}
	// Convert to one-liner
	triggerBootstrapScript = strings.ReplaceAll(triggerBootstrapScript, "\n", " ")
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", triggerBootstrapScript)
	for k, v := range getCSEEnv(config) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(cmd.Env) // produce deterministic output
	return cmd, nil
}
