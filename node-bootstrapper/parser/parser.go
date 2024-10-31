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
		"TENANT_ID":                                      config.GetAuthConfig().GetTenantId(),
		"KUBERNETES_VERSION":                             config.GetKubernetesVersion(),
		"KUBE_BINARY_URL":                                config.GetKubeBinaryConfig().GetKubeBinaryUrl(),
		"CUSTOM_KUBE_BINARY_URL":                         config.GetKubeBinaryConfig().GetCustomKubeBinaryUrl(),
		"PRIVATE_KUBE_BINARY_URL":                        config.GetKubeBinaryConfig().GetPrivateKubeBinaryUrl(),
		"KUBEPROXY_URL":                                  config.GetKubeProxyUrl(),
		"APISERVER_PUBLIC_KEY":                           config.GetApiServerConfig().GetApiServerPublicKey(),
		"SUBSCRIPTION_ID":                                config.GetAuthConfig().GetSubscriptionId(),
		"RESOURCE_GROUP":                                 config.GetClusterConfig().GetResourceGroup(),
		"LOCATION":                                       config.GetClusterConfig().GetLocation(),
		"VM_TYPE":                                        getStringFromVMType(config.GetClusterConfig().GetVmType()),
		"SUBNET":                                         config.GetClusterConfig().GetClusterNetworkConfig().GetSubnet(),
		"NETWORK_SECURITY_GROUP":                         config.GetClusterConfig().GetClusterNetworkConfig().GetSecurityGroupName(),
		"VIRTUAL_NETWORK":                                config.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"VIRTUAL_NETWORK_RESOURCE_GROUP":                 config.GetClusterConfig().GetClusterNetworkConfig().GetVnetResourceGroup(),
		"ROUTE_TABLE":                                    config.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"PRIMARY_AVAILABILITY_SET":                       config.GetClusterConfig().GetPrimaryAvailabilitySet(),
		"PRIMARY_SCALE_SET":                              config.GetClusterConfig().GetPrimaryScaleSet(),
		"SERVICE_PRINCIPAL_CLIENT_ID":                    config.GetAuthConfig().GetServicePrincipalId(),
		"NETWORK_PLUGIN":                                 getStringFromNetworkPluginType(config.GetNetworkConfig().GetNetworkPlugin()),
		"VNET_CNI_PLUGINS_URL":                           config.GetNetworkConfig().GetVnetCniPluginsUrl(),
		"LOAD_BALANCER_DISABLE_OUTBOUND_SNAT":            fmt.Sprintf("%v", config.GetClusterConfig().GetLoadBalancerConfig().GetDisableOutboundSnat()),
		"USE_MANAGED_IDENTITY_EXTENSION":                 fmt.Sprintf("%v", config.GetAuthConfig().GetUseManagedIdentityExtension()),
		"USE_INSTANCE_METADATA":                          fmt.Sprintf("%v", config.GetClusterConfig().GetUseInstanceMetadata()),
		"LOAD_BALANCER_SKU":                              getStringFromLoadBalancerSkuType(config.GetClusterConfig().GetLoadBalancerConfig().GetLoadBalancerSku()),
		"EXCLUDE_MASTER_FROM_STANDARD_LB":                fmt.Sprintf("%v", getExcludeMasterFromStandardLB(config.GetClusterConfig().GetLoadBalancerConfig())),
		"MAXIMUM_LOADBALANCER_RULE_COUNT":                fmt.Sprintf("%v", getMaxLBRuleCount(config.GetClusterConfig().GetLoadBalancerConfig())),
		"CONTAINERD_DOWNLOAD_URL_BASE":                   config.GetContainerdConfig().GetContainerdDownloadUrlBase(),
		"USER_ASSIGNED_IDENTITY_ID":                      config.GetAuthConfig().GetAssignedIdentityId(),
		"API_SERVER_NAME":                                config.GetApiServerConfig().GetApiServerName(),
		"IS_VHD":                                         fmt.Sprintf("%v", getIsVHD(config.IsVhd)),
		"GPU_NODE":                                       fmt.Sprintf("%v", getEnableNvidia(config)),
		"SGX_NODE":                                       fmt.Sprintf("%v", getIsSgxEnabledSKU(config.GetVmSize())),
		"MIG_NODE":                                       fmt.Sprintf("%v", getIsMIGNode(config.GetGpuConfig().GetGpuInstanceProfile())),
		"CONFIG_GPU_DRIVER_IF_NEEDED":                    fmt.Sprintf("%v", config.GetGpuConfig().GetConfigGpuDriver()),
		"ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED":             fmt.Sprintf("%v", config.GetGpuConfig().GetGpuDevicePlugin()),
		"TELEPORTD_PLUGIN_DOWNLOAD_URL":                  config.GetTeleportConfig().GetTeleportdPluginDownloadUrl(),
		"CREDENTIAL_PROVIDER_DOWNLOAD_URL":               config.GetKubeBinaryConfig().GetLinuxCredentialProviderUrl(),
		"CONTAINERD_VERSION":                             config.GetContainerdConfig().GetContainerdVersion(),
		"CONTAINERD_PACKAGE_URL":                         config.GetContainerdConfig().GetContainerdPackageUrl(),
		"RUNC_VERSION":                                   config.GetRuncConfig().GetRuncVersion(),
		"RUNC_PACKAGE_URL":                               config.GetRuncConfig().GetRuncPackageUrl(),
		"ENABLE_HOSTS_CONFIG_AGENT":                      fmt.Sprintf("%v", config.GetEnableHostsConfigAgent()),
		"DISABLE_SSH":                                    fmt.Sprintf("%v", getDisableSSH(config)),
		"TELEPORT_ENABLED":                               fmt.Sprintf("%v", config.GetTeleportConfig().GetStatus()),
		"SHOULD_CONFIGURE_HTTP_PROXY":                    fmt.Sprintf("%v", getShouldConfigureHTTPProxy(config.GetHttpProxyConfig())),
		"SHOULD_CONFIGURE_HTTP_PROXY_CA":                 fmt.Sprintf("%v", getShouldConfigureHTTPProxyCA(config.GetHttpProxyConfig())),
		"HTTP_PROXY_TRUSTED_CA":                          config.GetHttpProxyConfig().GetProxyTrustedCa(),
		"SHOULD_CONFIGURE_CUSTOM_CA_TRUST":               fmt.Sprintf("%v", getCustomCACertsStatus(config.GetCustomCaCerts())),
		"CUSTOM_CA_TRUST_COUNT":                          fmt.Sprintf("%v", len(config.GetCustomCaCerts())),
		"IS_KRUSTLET":                                    fmt.Sprintf("%v", getIsKrustlet(config.GetWorkloadRuntime())),
		"GPU_NEEDS_FABRIC_MANAGER":                       fmt.Sprintf("%v", getGPUNeedsFabricManager(config.GetVmSize())),
		"IPV6_DUAL_STACK_ENABLED":                        fmt.Sprintf("%v", config.GetIpv6DualStackEnabled()),
		"OUTBOUND_COMMAND":                               config.GetOutboundCommand(),
		"ENABLE_UNATTENDED_UPGRADES":                     fmt.Sprintf("%v", config.GetEnableUnattendedUpgrade()),
		"ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE":              fmt.Sprintf("%v", getEnsureNoDupePromiscuousBridge(config.GetNetworkConfig())),
		"SHOULD_CONFIG_SWAP_FILE":                        fmt.Sprintf("%v", getEnableSwapConfig(config.GetCustomLinuxOsConfig())),
		"SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE":            fmt.Sprintf("%v", getShouldConfigTransparentHugePage(config.GetCustomLinuxOsConfig())),
		"SHOULD_CONFIG_CONTAINERD_ULIMITS":               fmt.Sprintf("%v", getShouldConfigContainerdUlimits(config.GetCustomLinuxOsConfig().GetUlimitConfig())),
		"CONTAINERD_ULIMITS":                             getUlimitContent(config.GetCustomLinuxOsConfig().GetUlimitConfig()),
		"TARGET_CLOUD":                                   getTargetCloud(config),
		"TARGET_ENVIRONMENT":                             getTargetEnvironment(config),
		"CUSTOM_ENV_JSON":                                config.GetCustomCloudConfig().GetCustomEnvJsonContent(),
		"IS_CUSTOM_CLOUD":                                fmt.Sprintf("%v", getIsAksCustomCloud(config.GetCustomCloudConfig())),
		"AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX": config.GetCustomCloudConfig().GetContainerRegistryDnsSuffix(),
		"CSE_HELPERS_FILEPATH":                           getCSEHelpersFilepath(),
		"CSE_DISTRO_HELPERS_FILEPATH":                    getCSEDistroHelpersFilepath(),
		"CSE_INSTALL_FILEPATH":                           getCSEInstallFilepath(),
		"CSE_DISTRO_INSTALL_FILEPATH":                    getCSEDistroInstallFilepath(),
		"CSE_CONFIG_FILEPATH":                            getCSEConfigFilepath(),
		"AZURE_PRIVATE_REGISTRY_SERVER":                  config.GetAzurePrivateRegistryServer(),
		"HAS_CUSTOM_SEARCH_DOMAIN":                       fmt.Sprintf("%v", getHasSearchDomain(config.GetCustomSearchDomainConfig())),
		"CUSTOM_SEARCH_DOMAIN_FILEPATH":                  getCustomSearchDomainFilepath(),
		"HTTP_PROXY_URLS":                                config.GetHttpProxyConfig().GetHttpProxy(),
		"HTTPS_PROXY_URLS":                               config.GetHttpProxyConfig().GetHttpsProxy(),
		"NO_PROXY_URLS":                                  getStringifiedStringArray(config.GetHttpProxyConfig().GetNoProxyEntries(), ","),
		"PROXY_VARS":                                     getProxyVariables(config.GetHttpProxyConfig()),
		"ENABLE_TLS_BOOTSTRAPPING":                       fmt.Sprintf("%v", getEnableTLSBootstrap(config.GetTlsBootstrappingConfig())),
		"ENABLE_SECURE_TLS_BOOTSTRAPPING":                fmt.Sprintf("%v", getEnableSecureTLSBootstrap(config.GetTlsBootstrappingConfig())),
		"CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID":  getCustomSecureTLSBootstrapAADServerAppID(config.GetTlsBootstrappingConfig()),
		"DHCPV6_SERVICE_FILEPATH":                        getDHCPV6ServiceFilepath(),
		"DHCPV6_CONFIG_FILEPATH":                         getDHCPV6ConfigFilepath(),
		"THP_ENABLED":                                    config.GetCustomLinuxOsConfig().GetTransparentHugepageSupport(),
		"THP_DEFRAG":                                     config.GetCustomLinuxOsConfig().GetTransparentDefrag(),
		"SERVICE_PRINCIPAL_FILE_CONTENT":                 getServicePrincipalFileContent(config.AuthConfig),
		"KUBELET_CLIENT_CONTENT":                         config.GetKubeletConfig().GetKubeletClientKey(),
		"KUBELET_CLIENT_CERT_CONTENT":                    config.GetKubeletConfig().GetKubeletClientCertContent(),
		"KUBELET_CONFIG_FILE_ENABLED":                    fmt.Sprintf("%v", config.GetKubeletConfig().GetEnableKubeletConfigFile()),
		"KUBELET_CONFIG_FILE_CONTENT":                    config.GetKubeletConfig().GetKubeletConfigFileContent(),
		"SWAP_FILE_SIZE_MB":                              fmt.Sprintf("%v", config.GetCustomLinuxOsConfig().GetSwapFileSize()),
		"GPU_DRIVER_VERSION":                             getGpuDriverVersion(config.GetVmSize()),
		"GPU_IMAGE_SHA":                                  getGpuImageSha(config.GetVmSize()),
		"GPU_INSTANCE_PROFILE":                           config.GetGpuConfig().GetGpuInstanceProfile(),
		"CUSTOM_SEARCH_DOMAIN_NAME":                      config.GetCustomSearchDomainConfig().GetDomainName(),
		"CUSTOM_SEARCH_REALM_USER":                       config.GetCustomSearchDomainConfig().GetRealmUser(),
		"CUSTOM_SEARCH_REALM_PASSWORD":                   config.GetCustomSearchDomainConfig().GetRealmPassword(),
		"MESSAGE_OF_THE_DAY":                             config.GetMessageOfTheDay(),
		"HAS_KUBELET_DISK_TYPE":                          fmt.Sprintf("%v", getHasKubeletDiskType(config.GetKubeletConfig())),
		"NEEDS_CGROUPV2":                                 fmt.Sprintf("%v", config.GetNeedsCgroupv2()),
		"TLS_BOOTSTRAP_TOKEN":                            getTLSBootstrapToken(config.GetTlsBootstrappingConfig()),
		"KUBELET_FLAGS":                                  createSortedKeyValuePairs(config.GetKubeletConfig().GetKubeletFlags(), " "),
		"NETWORK_POLICY":                                 getStringFromNetworkPolicyType(config.GetNetworkConfig().GetNetworkPolicy()),
		"KUBELET_NODE_LABELS":                            createSortedKeyValuePairs(config.GetKubeletConfig().GetKubeletNodeLabels(), ","),
		"AZURE_ENVIRONMENT_FILEPATH":                     getAzureEnvironmentFilepath(config),
		"KUBE_CA_CRT":                                    config.GetKubernetesCaCert(),
		"KUBENET_TEMPLATE":                               getKubenetTemplate(),
		"CONTAINERD_CONFIG_CONTENT":                      getContainerdConfig(config),
		"IS_KATA":                                        fmt.Sprintf("%v", config.GetIsKata()),
		"ARTIFACT_STREAMING_ENABLED":                     fmt.Sprintf("%v", config.GetEnableArtifactStreaming()),
		"SYSCTL_CONTENT":                                 getSysctlContent(config.GetCustomLinuxOsConfig().GetSysctlConfig()),
		"PRIVATE_EGRESS_PROXY_ADDRESS":                   config.GetPrivateEgressProxyAddress(),
		"BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER":    config.GetBootstrapProfileContainerRegistryServer(),
		"ENABLE_IMDS_RESTRICTION":                        fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetEnableImdsRestriction()),
		"INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE":   fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetInsertImdsRestrictionRuleToMangleTable()),
	}

	for i, cert := range config.CustomCaCerts {
		env[fmt.Sprintf("CUSTOM_CA_CERT_%d", i)] = cert
	}
	return env
}

func mapToEnviron(input map[string]string) []string {
	var env []string
	for k, v := range input {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(env) // produce deterministic output
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
	env := mapToEnviron(getCSEEnv(config))
	cmd.Env = append(os.Environ(), env...) // append existing environment variables
	sort.Strings(cmd.Env)
	return cmd, nil
}
