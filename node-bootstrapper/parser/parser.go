package parser

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

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

func getCSEEnv(config *nbcontractv1.Configuration) map[string]string {
	env := map[string]string{
		"PROVISION_OUTPUT":                               "/var/log/azure/cluster-provision.log",
		"MOBY_VERSION":                                   "",
		"CLOUDPROVIDER_BACKOFF":                          "true",
		"CLOUDPROVIDER_BACKOFF_MODE":                     "v2",
		"CLOUDPROVIDER_BACKOFF_RETRIES":                  "6",
		"CLOUDPROVIDER_BACKOFF_EXPONENT":                 "0",
		"CLOUDPROVIDER_BACKOFF_DURATION":                 "5",
		"CLOUDPROVIDER_BACKOFF_JITTER":                   "0",
		"CLOUDPROVIDER_RATELIMIT":                        "true",
		"CLOUDPROVIDER_RATELIMIT_QPS":                    "10",
		"CLOUDPROVIDER_RATELIMIT_QPS_WRITE":              "10",
		"CLOUDPROVIDER_RATELIMIT_BUCKET":                 "100",
		"CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE":           "100",
		"CONTAINER_RUNTIME":                              "containerd",
		"CLI_TOOL":                                       "ctr",
		"NETWORK_MODE":                                   "transparent",
		"NEEDS_CONTAINERD":                               "true",
		"NEEDS_DOCKER_LOGIN":                             "false",
		"ADMINUSER":                                      getLinuxAdminUsername(config.GetLinuxAdminUsername()),
		"TENANT_ID":                                      config.AuthConfig.GetTenantId(),
		"KUBERNETES_VERSION":                             config.GetKubernetesVersion(),
		"KUBE_BINARY_URL":                                config.KubeBinaryConfig.GetKubeBinaryUrl(),
		"CUSTOM_KUBE_BINARY_URL":                         config.KubeBinaryConfig.GetCustomKubeBinaryUrl(),
		"PRIVATE_KUBE_BINARY_URL":                        config.KubeBinaryConfig.GetPrivateKubeBinaryUrl(),
		"KUBEPROXY_URL":                                  config.GetKubeProxyUrl(),
		"APISERVER_PUBLIC_KEY":                           config.ApiServerConfig.GetApiServerPublicKey(),
		"SUBSCRIPTION_ID":                                config.AuthConfig.GetSubscriptionId(),
		"RESOURCE_GROUP":                                 config.ClusterConfig.GetResourceGroup(),
		"LOCATION":                                       config.ClusterConfig.GetLocation(),
		"VM_TYPE":                                        getStringFromVMType(config.ClusterConfig.GetVmType()),
		"SUBNET":                                         config.ClusterConfig.GetClusterNetworkConfig().GetSubnet(),
		"NETWORK_SECURITY_GROUP":                         config.ClusterConfig.GetClusterNetworkConfig().GetSecurityGroupName(),
		"VIRTUAL_NETWORK":                                config.ClusterConfig.GetClusterNetworkConfig().GetVnetName(),
		"VIRTUAL_NETWORK_RESOURCE_GROUP":                 config.ClusterConfig.GetClusterNetworkConfig().GetVnetResourceGroup(),
		"ROUTE_TABLE":                                    config.ClusterConfig.GetClusterNetworkConfig().GetRouteTable(),
		"PRIMARY_AVAILABILITY_SET":                       config.ClusterConfig.GetPrimaryAvailabilitySet(),
		"PRIMARY_SCALE_SET":                              config.ClusterConfig.GetPrimaryScaleSet(),
		"SERVICE_PRINCIPAL_CLIENT_ID":                    config.AuthConfig.GetServicePrincipalId(),
		"NETWORK_PLUGIN":                                 getStringFromNetworkPluginType(config.GetNetworkConfig().GetNetworkPlugin()),
		"VNET_CNI_PLUGINS_URL":                           config.GetNetworkConfig().GetVnetCniPluginsUrl(),
		"LOAD_BALANCER_DISABLE_OUTBOUND_SNAT":            fmt.Sprintf("%v", config.ClusterConfig.GetLoadBalancerConfig().GetDisableOutboundSnat()),
		"USE_MANAGED_IDENTITY_EXTENSION":                 fmt.Sprintf("%v", config.AuthConfig.GetUseManagedIdentityExtension()),
		"USE_INSTANCE_METADATA":                          fmt.Sprintf("%v", config.ClusterConfig.GetUseInstanceMetadata()),
		"LOAD_BALANCER_SKU":                              getStringFromLoadBalancerSkuType(config.ClusterConfig.GetLoadBalancerConfig().GetLoadBalancerSku()),
		"EXCLUDE_MASTER_FROM_STANDARD_LB":                fmt.Sprintf("%v", getExcludeMasterFromStandardLB(config.ClusterConfig.GetLoadBalancerConfig())),
		"MAXIMUM_LOADBALANCER_RULE_COUNT":                fmt.Sprintf("%v", getMaxLBRuleCount(config.ClusterConfig.GetLoadBalancerConfig())),
		"CONTAINERD_DOWNLOAD_URL_BASE":                   config.ContainerdConfig.GetContainerdDownloadUrlBase(),
		"USER_ASSIGNED_IDENTITY_ID":                      config.AuthConfig.GetAssignedIdentityId(),
		"API_SERVER_NAME":                                config.ApiServerConfig.GetApiServerName(),
		"IS_VHD":                                         fmt.Sprintf("%v", getIsVHD(config.IsVhd)),
		"GPU_NODE":                                       fmt.Sprintf("%v", getEnableNvidia(config)),
		"SGX_NODE":                                       fmt.Sprintf("%v", getIsSgxEnabledSKU(config.VmSize)),
		"MIG_NODE":                                       fmt.Sprintf("%v", getIsMIGNode(config.GpuConfig.GetGpuInstanceProfile())),
		"CONFIG_GPU_DRIVER_IF_NEEDED":                    fmt.Sprintf("%v", config.GpuConfig.GetConfigGpuDriver()),
		"ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED":             fmt.Sprintf("%v", config.GpuConfig.GetGpuDevicePlugin()),
		"TELEPORTD_PLUGIN_DOWNLOAD_URL":                  config.TeleportConfig.GetTeleportdPluginDownloadUrl(),
		"CREDENTIAL_PROVIDER_DOWNLOAD_URL":               config.KubeBinaryConfig.GetLinuxCredentialProviderUrl(),
		"CONTAINERD_VERSION":                             config.ContainerdConfig.GetContainerdVersion(),
		"CONTAINERD_PACKAGE_URL":                         config.ContainerdConfig.GetContainerdPackageUrl(),
		"RUNC_VERSION":                                   config.RuncConfig.GetRuncVersion(),
		"RUNC_PACKAGE_URL":                               config.RuncConfig.GetRuncPackageUrl(),
		"ENABLE_HOSTS_CONFIG_AGENT":                      fmt.Sprintf("%v", config.GetEnableHostsConfigAgent()),
		"DISABLE_SSH":                                    fmt.Sprintf("%v", getDisableSSH(config)),
		"TELEPORT_ENABLED":                               fmt.Sprintf("%v", config.TeleportConfig.GetStatus()),
		"SHOULD_CONFIGURE_HTTP_PROXY":                    fmt.Sprintf("%v", getShouldConfigureHTTPProxy(config.HttpProxyConfig)),
		"SHOULD_CONFIGURE_HTTP_PROXY_CA":                 fmt.Sprintf("%v", getShouldConfigureHTTPProxyCA(config.HttpProxyConfig)),
		"HTTP_PROXY_TRUSTED_CA":                          config.HttpProxyConfig.GetProxyTrustedCa(),
		"SHOULD_CONFIGURE_CUSTOM_CA_TRUST":               fmt.Sprintf("%v", getCustomCACertsStatus(config.GetCustomCaCerts())),
		"CUSTOM_CA_TRUST_COUNT":                          fmt.Sprintf("%v", len(config.GetCustomCaCerts())),
		"IS_KRUSTLET":                                    fmt.Sprintf("%v", getIsKrustlet(config.GetWorkloadRuntime())),
		"GPU_NEEDS_FABRIC_MANAGER":                       fmt.Sprintf("%v", getGPUNeedsFabricManager(config.VmSize)),
		"IPV6_DUAL_STACK_ENABLED":                        fmt.Sprintf("%v", config.GetIpv6DualStackEnabled()),
		"OUTBOUND_COMMAND":                               config.GetOutboundCommand(),
		"ENABLE_UNATTENDED_UPGRADES":                     fmt.Sprintf("%v", config.GetEnableUnattendedUpgrade()),
		"ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE":              fmt.Sprintf("%v", getEnsureNoDupePromiscuousBridge(config.GetNetworkConfig())),
		"SHOULD_CONFIG_SWAP_FILE":                        fmt.Sprintf("%v", getEnableSwapConfig(config.CustomLinuxOsConfig)),
		"SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE":            fmt.Sprintf("%v", getShouldConfigTransparentHugePage(config.CustomLinuxOsConfig)),
		"SHOULD_CONFIG_CONTAINERD_ULIMITS":               fmt.Sprintf("%v", getShouldConfigContainerdUlimits(config.CustomLinuxOsConfig.GetUlimitConfig())),
		"CONTAINERD_ULIMITS":                             getUlimitContent(config.CustomLinuxOsConfig.GetUlimitConfig()),
		"TARGET_CLOUD":                                   getTargetCloud(config),
		"TARGET_ENVIRONMENT":                             getTargetEnvironment(config),
		"CUSTOM_ENV_JSON":                                config.CustomCloudConfig.GetCustomEnvJsonContent(),
		"IS_CUSTOM_CLOUD":                                fmt.Sprintf("%v", getIsAksCustomCloud(config.CustomCloudConfig)),
		"AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX": config.CustomCloudConfig.GetContainerRegistryDnsSuffix(),
		"CSE_HELPERS_FILEPATH":                           getCSEHelpersFilepath(),
		"CSE_DISTRO_HELPERS_FILEPATH":                    getCSEDistroHelpersFilepath(),
		"CSE_INSTALL_FILEPATH":                           getCSEInstallFilepath(),
		"CSE_DISTRO_INSTALL_FILEPATH":                    getCSEDistroInstallFilepath(),
		"CSE_CONFIG_FILEPATH":                            getCSEConfigFilepath(),
		"AZURE_PRIVATE_REGISTRY_SERVER":                  config.GetAzurePrivateRegistryServer(),
		"HAS_CUSTOM_SEARCH_DOMAIN":                       fmt.Sprintf("%v", getHasSearchDomain(config.GetCustomSearchDomainConfig())),
		"CUSTOM_SEARCH_DOMAIN_FILEPATH":                  getCustomSearchDomainFilepath(),
		"HTTP_PROXY_URLS":                                config.HttpProxyConfig.GetHttpProxy(),
		"HTTPS_PROXY_URLS":                               config.HttpProxyConfig.GetHttpsProxy(),
		"NO_PROXY_URLS":                                  getStringifiedStringArray(config.HttpProxyConfig.GetNoProxyEntries(), ","),
		"PROXY_VARS":                                     getProxyVariables(config.HttpProxyConfig),
		"ENABLE_TLS_BOOTSTRAPPING":                       fmt.Sprintf("%v", getEnableTLSBootstrap(config.TlsBootstrappingConfig)),
		"ENABLE_SECURE_TLS_BOOTSTRAPPING":                fmt.Sprintf("%v", getEnableSecureTLSBootstrap(config.TlsBootstrappingConfig)),
		"CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID":  getCustomSecureTLSBootstrapAADServerAppID(config.TlsBootstrappingConfig),
		"DHCPV6_SERVICE_FILEPATH":                        getDHCPV6ServiceFilepath(),
		"DHCPV6_CONFIG_FILEPATH":                         getDHCPV6ConfigFilepath(),
		"THP_ENABLED":                                    config.CustomLinuxOsConfig.GetTransparentHugepageSupport(),
		"THP_DEFRAG":                                     config.CustomLinuxOsConfig.GetTransparentDefrag(),
		"SERVICE_PRINCIPAL_FILE_CONTENT":                 getServicePrincipalFileContent(config.AuthConfig),
		"KUBELET_CLIENT_CONTENT":                         config.KubeletConfig.GetKubeletClientKey(),
		"KUBELET_CLIENT_CERT_CONTENT":                    config.KubeletConfig.GetKubeletClientCertContent(),
		"KUBELET_CONFIG_FILE_ENABLED":                    fmt.Sprintf("%v", config.KubeletConfig.GetEnableKubeletConfigFile()),
		"KUBELET_CONFIG_FILE_CONTENT":                    config.KubeletConfig.GetKubeletConfigFileContent(),
		"SWAP_FILE_SIZE_MB":                              fmt.Sprintf("%v", config.CustomLinuxOsConfig.GetSwapFileSize()),
		"GPU_DRIVER_VERSION":                             getGpuDriverVersion(config.VmSize),
		"GPU_IMAGE_SHA":                                  getGpuImageSha(config.VmSize),
		"GPU_INSTANCE_PROFILE":                           config.GpuConfig.GetGpuInstanceProfile(),
		"CUSTOM_SEARCH_DOMAIN_NAME":                      config.CustomSearchDomainConfig.GetDomainName(),
		"CUSTOM_SEARCH_REALM_USER":                       config.CustomSearchDomainConfig.GetRealmUser(),
		"CUSTOM_SEARCH_REALM_PASSWORD":                   config.CustomSearchDomainConfig.GetRealmPassword(),
		"MESSAGE_OF_THE_DAY":                             config.GetMessageOfTheDay(),
		"HAS_KUBELET_DISK_TYPE":                          fmt.Sprintf("%v", getHasKubeletDiskType(config.KubeletConfig)),
		"NEEDS_CGROUPV2":                                 fmt.Sprintf("%v", config.GetNeedsCgroupv2()),
		"TLS_BOOTSTRAP_TOKEN":                            getTLSBootstrapToken(config.TlsBootstrappingConfig),
		"KUBELET_FLAGS":                                  createSortedKeyValuePairs(config.KubeletConfig.GetKubeletFlags(), " "),
		"NETWORK_POLICY":                                 getStringFromNetworkPolicyType(config.NetworkConfig.GetNetworkPolicy()),
		"KUBELET_NODE_LABELS":                            createSortedKeyValuePairs(config.KubeletConfig.GetKubeletNodeLabels(), ","),
		"AZURE_ENVIRONMENT_FILEPATH":                     getAzureEnvironmentFilepath(config),
		"KUBE_CA_CRT":                                    config.GetKubernetesCaCert(),
		"KUBENET_TEMPLATE":                               getKubenetTemplate(),
		"CONTAINERD_CONFIG_CONTENT":                      getContainerdConfig(config),
		"IS_KATA":                                        fmt.Sprintf("%v", config.GetIsKata()),
		"ARTIFACT_STREAMING_ENABLED":                     fmt.Sprintf("%v", config.GetEnableArtifactStreaming()),
		"SYSCTL_CONTENT":                                 getSysctlContent(config.CustomLinuxOsConfig.GetSysctlConfig()),
		"PRIVATE_EGRESS_PROXY_ADDRESS":                   config.GetPrivateEgressProxyAddress(),
		"BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER":    config.GetBootstrapProfileContainerRegistryServer(),
		"ENABLE_IMDS_RESTRICTION":                        fmt.Sprintf("%v", config.ImdsRestrictionConfig.GetEnableImdsRestriction()),
		"INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE":   fmt.Sprintf("%v", config.ImdsRestrictionConfig.GetInsertImdsRestrictionRuleToMangleTable()),
	}

	for i, cert := range config.CustomCaCerts {
		env[fmt.Sprintf("CUSTOM_CA_CERT_%d", i)] = cert
	}

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
	cmd.Env = append(cmd.Env, os.Environ()...) // append existing environment variables
	sort.Strings(cmd.Env)                      // produce deterministic output
	return cmd, nil
}
