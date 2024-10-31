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
		"ADMINUSER": getLinuxAdminUsername(config.GetLinuxAdminUsername()),
		"AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX": config.GetCustomCloudConfig().GetContainerRegistryDnsSuffix(),
		"APISERVER_PUBLIC_KEY":                           config.GetApiServerConfig().GetApiServerPublicKey(),
		"API_SERVER_NAME":                                config.GetApiServerConfig().GetApiServerName(),
		"ARTIFACT_STREAMING_ENABLED":                     fmt.Sprintf("%v", config.GetEnableArtifactStreaming()),
		"AZURE_ENVIRONMENT_FILEPATH":                     getAzureEnvironmentFilepath(config),
		"AZURE_PRIVATE_REGISTRY_SERVER":                  config.GetAzurePrivateRegistryServer(),
		"BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER":    config.GetBootstrapProfileContainerRegistryServer(),
		"CLI_TOOL":                                      "ctr",
		"CLOUDPROVIDER_BACKOFF":                         "true",
		"CLOUDPROVIDER_BACKOFF_DURATION":                "5",
		"CLOUDPROVIDER_BACKOFF_EXPONENT":                "0",
		"CLOUDPROVIDER_BACKOFF_JITTER":                  "0",
		"CLOUDPROVIDER_BACKOFF_MODE":                    "v2",
		"CLOUDPROVIDER_BACKOFF_RETRIES":                 "6",
		"CLOUDPROVIDER_RATELIMIT":                       "true",
		"CLOUDPROVIDER_RATELIMIT_BUCKET":                "100",
		"CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE":          "100",
		"CLOUDPROVIDER_RATELIMIT_QPS":                   "10",
		"CLOUDPROVIDER_RATELIMIT_QPS_WRITE":             "10",
		"CONFIG_GPU_DRIVER_IF_NEEDED":                   fmt.Sprintf("%v", config.GetGpuConfig().GetConfigGpuDriver()),
		"CONTAINERD_CONFIG_CONTENT":                     getContainerdConfig(config),
		"CONTAINERD_DOWNLOAD_URL_BASE":                  config.GetContainerdConfig().GetContainerdDownloadUrlBase(),
		"CONTAINERD_PACKAGE_URL":                        config.GetContainerdConfig().GetContainerdPackageUrl(),
		"CONTAINERD_ULIMITS":                            getUlimitContent(config.GetCustomLinuxOsConfig().GetUlimitConfig()),
		"CONTAINERD_VERSION":                            config.GetContainerdConfig().GetContainerdVersion(),
		"CONTAINER_RUNTIME":                             "containerd",
		"CREDENTIAL_PROVIDER_DOWNLOAD_URL":              config.GetKubeBinaryConfig().GetLinuxCredentialProviderUrl(),
		"CSE_CONFIG_FILEPATH":                           getCSEConfigFilepath(),
		"CSE_DISTRO_HELPERS_FILEPATH":                   getCSEDistroHelpersFilepath(),
		"CSE_DISTRO_INSTALL_FILEPATH":                   getCSEDistroInstallFilepath(),
		"CSE_HELPERS_FILEPATH":                          getCSEHelpersFilepath(),
		"CSE_INSTALL_FILEPATH":                          getCSEInstallFilepath(),
		"CUSTOM_CA_TRUST_COUNT":                         fmt.Sprintf("%v", len(config.GetCustomCaCerts())),
		"CUSTOM_ENV_JSON":                               config.GetCustomCloudConfig().GetCustomEnvJsonContent(),
		"CUSTOM_KUBE_BINARY_URL":                        config.GetKubeBinaryConfig().GetCustomKubeBinaryUrl(),
		"CUSTOM_SEARCH_DOMAIN_FILEPATH":                 getCustomSearchDomainFilepath(),
		"CUSTOM_SEARCH_DOMAIN_NAME":                     config.GetCustomSearchDomainConfig().GetDomainName(),
		"CUSTOM_SEARCH_REALM_PASSWORD":                  config.GetCustomSearchDomainConfig().GetRealmPassword(),
		"CUSTOM_SEARCH_REALM_USER":                      config.GetCustomSearchDomainConfig().GetRealmUser(),
		"CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID": getCustomSecureTLSBootstrapAADServerAppID(config.GetTlsBootstrappingConfig()),
		"DHCPV6_CONFIG_FILEPATH":                        getDHCPV6ConfigFilepath(),
		"DHCPV6_SERVICE_FILEPATH":                       getDHCPV6ServiceFilepath(),
		"DISABLE_SSH":                                   fmt.Sprintf("%v", getDisableSSH(config)),
		"ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED":            fmt.Sprintf("%v", config.GetGpuConfig().GetGpuDevicePlugin()),
		"ENABLE_HOSTS_CONFIG_AGENT":                     fmt.Sprintf("%v", config.GetEnableHostsConfigAgent()),
		"ENABLE_IMDS_RESTRICTION":                       fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetEnableImdsRestriction()),
		"ENABLE_SECURE_TLS_BOOTSTRAPPING":               fmt.Sprintf("%v", getEnableSecureTLSBootstrap(config.GetTlsBootstrappingConfig())),
		"ENABLE_TLS_BOOTSTRAPPING":                      fmt.Sprintf("%v", getEnableTLSBootstrap(config.GetTlsBootstrappingConfig())),
		"ENABLE_UNATTENDED_UPGRADES":                    fmt.Sprintf("%v", config.GetEnableUnattendedUpgrade()),
		"ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE":             fmt.Sprintf("%v", getEnsureNoDupePromiscuousBridge(config.GetNetworkConfig())),
		"EXCLUDE_MASTER_FROM_STANDARD_LB":               fmt.Sprintf("%v", getExcludeMasterFromStandardLB(config.GetClusterConfig().GetLoadBalancerConfig())),
		"GPU_DRIVER_VERSION":                            getGpuDriverVersion(config.GetVmSize()),
		"GPU_IMAGE_SHA":                                 getGpuImageSha(config.GetVmSize()),
		"GPU_INSTANCE_PROFILE":                          config.GetGpuConfig().GetGpuInstanceProfile(),
		"GPU_NEEDS_FABRIC_MANAGER":                      fmt.Sprintf("%v", getGPUNeedsFabricManager(config.GetVmSize())),
		"GPU_NODE":                                      fmt.Sprintf("%v", getEnableNvidia(config)),
		"HAS_CUSTOM_SEARCH_DOMAIN":                      fmt.Sprintf("%v", getHasSearchDomain(config.GetCustomSearchDomainConfig())),
		"HAS_KUBELET_DISK_TYPE":                         fmt.Sprintf("%v", getHasKubeletDiskType(config.GetKubeletConfig())),
		"HTTPS_PROXY_URLS":                              config.GetHttpProxyConfig().GetHttpsProxy(),
		"HTTP_PROXY_TRUSTED_CA":                         config.GetHttpProxyConfig().GetProxyTrustedCa(),
		"HTTP_PROXY_URLS":                               config.GetHttpProxyConfig().GetHttpProxy(),
		"INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE":  fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetInsertImdsRestrictionRuleToMangleTable()),
		"IPV6_DUAL_STACK_ENABLED":                       fmt.Sprintf("%v", config.GetIpv6DualStackEnabled()),
		"IS_CUSTOM_CLOUD":                               fmt.Sprintf("%v", getIsAksCustomCloud(config.GetCustomCloudConfig())),
		"IS_KATA":                                       fmt.Sprintf("%v", config.GetIsKata()),
		"IS_KRUSTLET":                                   fmt.Sprintf("%v", getIsKrustlet(config.GetWorkloadRuntime())),
		"IS_VHD":                                        fmt.Sprintf("%v", getIsVHD(config.IsVhd)),
		"KUBELET_CLIENT_CERT_CONTENT":                   config.GetKubeletConfig().GetKubeletClientCertContent(),
		"KUBELET_CLIENT_CONTENT":                        config.GetKubeletConfig().GetKubeletClientKey(),
		"KUBELET_CONFIG_FILE_CONTENT":                   config.GetKubeletConfig().GetKubeletConfigFileContent(),
		"KUBELET_CONFIG_FILE_ENABLED":                   fmt.Sprintf("%v", config.GetKubeletConfig().GetEnableKubeletConfigFile()),
		"KUBELET_FLAGS":                                 createSortedKeyValuePairs(config.GetKubeletConfig().GetKubeletFlags(), " "),
		"KUBELET_NODE_LABELS":                           createSortedKeyValuePairs(config.GetKubeletConfig().GetKubeletNodeLabels(), ","),
		"KUBENET_TEMPLATE":                              getKubenetTemplate(),
		"KUBEPROXY_URL":                                 config.GetKubeProxyUrl(),
		"KUBERNETES_VERSION":                            config.GetKubernetesVersion(),
		"KUBE_BINARY_URL":                               config.GetKubeBinaryConfig().GetKubeBinaryUrl(),
		"KUBE_CA_CRT":                                   config.GetKubernetesCaCert(),
		"LOAD_BALANCER_DISABLE_OUTBOUND_SNAT":           fmt.Sprintf("%v", config.GetClusterConfig().GetLoadBalancerConfig().GetDisableOutboundSnat()),
		"LOAD_BALANCER_SKU":                             getStringFromLoadBalancerSkuType(config.GetClusterConfig().GetLoadBalancerConfig().GetLoadBalancerSku()),
		"LOCATION":                                      config.GetClusterConfig().GetLocation(),
		"MAXIMUM_LOADBALANCER_RULE_COUNT":               fmt.Sprintf("%v", getMaxLBRuleCount(config.GetClusterConfig().GetLoadBalancerConfig())),
		"MESSAGE_OF_THE_DAY":                            config.GetMessageOfTheDay(),
		"MIG_NODE":                                      fmt.Sprintf("%v", getIsMIGNode(config.GetGpuConfig().GetGpuInstanceProfile())),
		"MOBY_VERSION":                                  "",
		"NEEDS_CGROUPV2":                                fmt.Sprintf("%v", config.GetNeedsCgroupv2()),
		"NEEDS_CONTAINERD":                              "true",
		"NEEDS_DOCKER_LOGIN":                            "false",
		"NETWORK_MODE":                                  "transparent",
		"NETWORK_PLUGIN":                                getStringFromNetworkPluginType(config.GetNetworkConfig().GetNetworkPlugin()),
		"NETWORK_POLICY":                                getStringFromNetworkPolicyType(config.GetNetworkConfig().GetNetworkPolicy()),
		"NETWORK_SECURITY_GROUP":                        config.GetClusterConfig().GetClusterNetworkConfig().GetSecurityGroupName(),
		"NO_PROXY_URLS":                                 getStringifiedStringArray(config.GetHttpProxyConfig().GetNoProxyEntries(), ","),
		"OUTBOUND_COMMAND":                              config.GetOutboundCommand(),
		"PRIMARY_AVAILABILITY_SET":                      config.GetClusterConfig().GetPrimaryAvailabilitySet(),
		"PRIMARY_SCALE_SET":                             config.GetClusterConfig().GetPrimaryScaleSet(),
		"PRIVATE_EGRESS_PROXY_ADDRESS":                  config.GetPrivateEgressProxyAddress(),
		"PRIVATE_KUBE_BINARY_URL":                       config.GetKubeBinaryConfig().GetPrivateKubeBinaryUrl(),
		"PROVISION_OUTPUT":                              "/var/log/azure/cluster-provision.log",
		"PROXY_VARS":                                    getProxyVariables(config.GetHttpProxyConfig()),
		"RESOURCE_GROUP":                                config.GetClusterConfig().GetResourceGroup(),
		"ROUTE_TABLE":                                   config.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"RUNC_PACKAGE_URL":                              config.GetRuncConfig().GetRuncPackageUrl(),
		"RUNC_VERSION":                                  config.GetRuncConfig().GetRuncVersion(),
		"SERVICE_PRINCIPAL_CLIENT_ID":                   config.GetAuthConfig().GetServicePrincipalId(),
		"SERVICE_PRINCIPAL_FILE_CONTENT":                getServicePrincipalFileContent(config.AuthConfig),
		"SGX_NODE":                                      fmt.Sprintf("%v", getIsSgxEnabledSKU(config.GetVmSize())),
		"SHOULD_CONFIGURE_CUSTOM_CA_TRUST":              fmt.Sprintf("%v", getCustomCACertsStatus(config.GetCustomCaCerts())),
		"SHOULD_CONFIGURE_HTTP_PROXY":                   fmt.Sprintf("%v", getShouldConfigureHTTPProxy(config.GetHttpProxyConfig())),
		"SHOULD_CONFIGURE_HTTP_PROXY_CA":                fmt.Sprintf("%v", getShouldConfigureHTTPProxyCA(config.GetHttpProxyConfig())),
		"SHOULD_CONFIG_CONTAINERD_ULIMITS":              fmt.Sprintf("%v", getShouldConfigContainerdUlimits(config.GetCustomLinuxOsConfig().GetUlimitConfig())),
		"SHOULD_CONFIG_SWAP_FILE":                       fmt.Sprintf("%v", getEnableSwapConfig(config.GetCustomLinuxOsConfig())),
		"SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE":           fmt.Sprintf("%v", getShouldConfigTransparentHugePage(config.GetCustomLinuxOsConfig())),
		"SUBNET":                                        config.GetClusterConfig().GetClusterNetworkConfig().GetSubnet(),
		"SUBSCRIPTION_ID":                               config.GetAuthConfig().GetSubscriptionId(),
		"SWAP_FILE_SIZE_MB":                             fmt.Sprintf("%v", config.GetCustomLinuxOsConfig().GetSwapFileSize()),
		"SYSCTL_CONTENT":                                getSysctlContent(config.GetCustomLinuxOsConfig().GetSysctlConfig()),
		"TARGET_CLOUD":                                  getTargetCloud(config),
		"TARGET_ENVIRONMENT":                            getTargetEnvironment(config),
		"TELEPORTD_PLUGIN_DOWNLOAD_URL":                 config.GetTeleportConfig().GetTeleportdPluginDownloadUrl(),
		"TELEPORT_ENABLED":                              fmt.Sprintf("%v", config.GetTeleportConfig().GetStatus()),
		"TENANT_ID":                                     config.GetAuthConfig().GetTenantId(),
		"THP_DEFRAG":                                    config.GetCustomLinuxOsConfig().GetTransparentDefrag(),
		"THP_ENABLED":                                   config.GetCustomLinuxOsConfig().GetTransparentHugepageSupport(),
		"TLS_BOOTSTRAP_TOKEN":                           getTLSBootstrapToken(config.GetTlsBootstrappingConfig()),
		"USER_ASSIGNED_IDENTITY_ID":                     config.GetAuthConfig().GetAssignedIdentityId(),
		"USE_INSTANCE_METADATA":                         fmt.Sprintf("%v", config.GetClusterConfig().GetUseInstanceMetadata()),
		"USE_MANAGED_IDENTITY_EXTENSION":                fmt.Sprintf("%v", config.GetAuthConfig().GetUseManagedIdentityExtension()),
		"VIRTUAL_NETWORK":                               config.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"VIRTUAL_NETWORK_RESOURCE_GROUP":                config.GetClusterConfig().GetClusterNetworkConfig().GetVnetResourceGroup(),
		"VM_TYPE":                                       getStringFromVMType(config.GetClusterConfig().GetVmType()),
		"VNET_CNI_PLUGINS_URL":                          config.GetNetworkConfig().GetVnetCniPluginsUrl(),
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
