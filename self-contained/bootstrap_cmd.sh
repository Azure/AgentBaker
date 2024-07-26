PROVISION_OUTPUT="/var/log/azure/cluster-provision-cse-output.log";
echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if ShouldEnableCustomData}}
cloud-init status --wait > /dev/null 2>&1;
[ $? -ne 0 ] && echo 'cloud-init failed' >> ${PROVISION_OUTPUT} && exit 1;
echo "cloud-init succeeded" >> ${PROVISION_OUTPUT};
{{end}}
{{if IsAKSCustomCloud}}
REPO_DEPOT_ENDPOINT="{{AKSCustomCloudRepoDepotEndpoint}}"
{{GetInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
ADMINUSER={{GetParameter "linuxAdminUsername"}}
MOBY_VERSION={{GetParameter "mobyVersion"}}
TENANT_ID={{GetVariable "tenantID"}}
KUBERNETES_VERSION={{GetParameter "kubernetesVersion"}}
HYPERKUBE_URL={{GetParameter "kubernetesHyperkubeSpec"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
CUSTOM_KUBE_BINARY_URL={{GetParameter "customKubeBinaryURL"}}
PRIVATE_KUBE_BINARY_URL="{{GetLinuxPrivatePackageURL}}"
KUBEPROXY_URL={{GetParameter "kubeProxySpec"}}
APISERVER_PUBLIC_KEY={{GetParameter "apiServerCertificate"}}
SUBSCRIPTION_ID={{GetVariable "subscriptionId"}}
RESOURCE_GROUP={{GetVariable "resourceGroup"}}
LOCATION={{GetVariable "location"}}
VM_TYPE={{GetVariable "vmType"}}
SUBNET={{GetVariable "subnetName"}}
NETWORK_SECURITY_GROUP={{GetVariable "nsgName"}}
VIRTUAL_NETWORK={{GetVariable "virtualNetworkName"}}
VIRTUAL_NETWORK_RESOURCE_GROUP={{GetVariable "virtualNetworkResourceGroupName"}}
ROUTE_TABLE={{GetVariable "routeTableName"}}
PRIMARY_AVAILABILITY_SET={{GetVariable "primaryAvailabilitySetName"}}
PRIMARY_SCALE_SET={{GetVariable "primaryScaleSetName"}}
SERVICE_PRINCIPAL_CLIENT_ID={{GetParameter "servicePrincipalClientId"}}
NETWORK_PLUGIN={{GetParameter "networkPlugin"}}
NETWORK_POLICY={{GetParameter "networkPolicy"}}
VNET_CNI_PLUGINS_URL={{GetParameter "vnetCniLinuxPluginsURL"}}
CLOUDPROVIDER_BACKOFF={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoff"}}
CLOUDPROVIDER_BACKOFF_MODE={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffMode"}}
CLOUDPROVIDER_BACKOFF_RETRIES={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffRetries"}}
CLOUDPROVIDER_BACKOFF_EXPONENT={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffExponent"}}
CLOUDPROVIDER_BACKOFF_DURATION={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffDuration"}}
CLOUDPROVIDER_BACKOFF_JITTER={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffJitter"}}
CLOUDPROVIDER_RATELIMIT={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimit"}}
CLOUDPROVIDER_RATELIMIT_QPS={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitQPS"}}
CLOUDPROVIDER_RATELIMIT_QPS_WRITE={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitQPSWrite"}}
CLOUDPROVIDER_RATELIMIT_BUCKET={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitBucket"}}
CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitBucketWrite"}}
LOAD_BALANCER_DISABLE_OUTBOUND_SNAT={{GetParameterProperty "cloudproviderConfig" "cloudProviderDisableOutboundSNAT"}}
USE_MANAGED_IDENTITY_EXTENSION={{GetVariable "useManagedIdentityExtension"}}
USE_INSTANCE_METADATA={{GetVariable "useInstanceMetadata"}}
LOAD_BALANCER_SKU={{GetVariable "loadBalancerSku"}}
EXCLUDE_MASTER_FROM_STANDARD_LB={{GetVariable "excludeMasterFromStandardLB"}}
MAXIMUM_LOADBALANCER_RULE_COUNT={{GetVariable "maximumLoadBalancerRuleCount"}}
CONTAINER_RUNTIME={{GetParameter "containerRuntime"}}
CLI_TOOL={{GetParameter "cliTool"}}
CONTAINERD_DOWNLOAD_URL_BASE={{GetParameter "containerdDownloadURLBase"}}
NETWORK_MODE={{GetParameter "networkMode"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
USER_ASSIGNED_IDENTITY_ID={{GetVariable "userAssignedIdentityID"}}
API_SERVER_NAME={{GetKubernetesEndpoint}}
IS_VHD={{GetVariable "isVHD"}}
GPU_NODE={{GetVariable "gpuNode"}}
SGX_NODE={{GetVariable "sgxNode"}}
MIG_NODE={{GetVariable "migNode"}}
CONFIG_GPU_DRIVER_IF_NEEDED={{GetVariable "configGPUDriverIfNeeded"}}
ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED={{GetVariable "enableGPUDevicePluginIfNeeded"}}
TELEPORTD_PLUGIN_DOWNLOAD_URL={{GetParameter "teleportdPluginURL"}}
CONTAINERD_VERSION={{GetParameter "containerdVersion"}}
CONTAINERD_PACKAGE_URL={{GetParameter "containerdPackageURL"}}
RUNC_VERSION={{GetParameter "runcVersion"}}
RUNC_PACKAGE_URL={{GetParameter "runcPackageURL"}}
ENABLE_HOSTS_CONFIG_AGENT="{{EnableHostsConfigAgent}}"
DISABLE_SSH="{{ShouldDisableSSH}}"
NEEDS_CONTAINERD="{{NeedsContainerd}}"
TELEPORT_ENABLED="{{TeleportEnabled}}"
SHOULD_CONFIGURE_HTTP_PROXY="{{ShouldConfigureHTTPProxy}}"
SHOULD_CONFIGURE_HTTP_PROXY_CA="{{ShouldConfigureHTTPProxyCA}}"
HTTP_PROXY_TRUSTED_CA="{{GetHTTPProxyCA}}"
SHOULD_CONFIGURE_CUSTOM_CA_TRUST="{{ShouldConfigureCustomCATrust}}"
CUSTOM_CA_TRUST_COUNT="{{len GetCustomCATrustConfigCerts}}"
{{range $i, $cert := GetCustomCATrustConfigCerts}}
CUSTOM_CA_CERT_{{$i}}="{{$cert}}"
{{end}}
IS_KRUSTLET="{{IsKrustlet}}"
GPU_NEEDS_FABRIC_MANAGER="{{GPUNeedsFabricManager}}"
#NEEDS_DOCKER_LOGIN="{{and IsDockerContainerRuntime HasPrivateAzureRegistryServer}}" This field is no longer required for the new contract since Docker is out of support and its value depends on Container Runtime = Docker
IPV6_DUAL_STACK_ENABLED="{{IsIPv6DualStackFeatureEnabled}}"
OUTBOUND_COMMAND="{{GetOutboundCommand}}"
ENABLE_UNATTENDED_UPGRADES="{{EnableUnattendedUpgrade}}"
ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE="{{ and NeedsContainerd IsKubenet (not HasCalicoNetworkPolicy) }}"
SHOULD_CONFIG_SWAP_FILE="{{ShouldConfigSwapFile}}"
SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE="{{ShouldConfigTransparentHugePage}}"
SHOULD_CONFIG_CONTAINERD_ULIMITS="{{ShouldConfigContainerdUlimits}}"
CONTAINERD_ULIMITS="{{GetContainerdUlimitString}}"
{{/* both CLOUD and ENVIRONMENT have special values when IsAKSCustomCloud == true */}}
{{/* CLOUD uses AzureStackCloud and seems to be used by kubelet, k8s cloud provider */}}
{{/* target environment seems to go to ARM SDK config */}}
{{/* not sure why separate/inconsistent? */}}
{{/* see GetCustomEnvironmentJSON for more weirdness. */}}
TARGET_CLOUD="{{- if IsAKSCustomCloud -}} AzureStackCloud {{- else -}} {{GetTargetEnvironment}} {{- end -}}"
TARGET_ENVIRONMENT="{{GetTargetEnvironment}}"
CUSTOM_ENV_JSON="{{GetBase64EncodedEnvironmentJSON}}"
IS_CUSTOM_CLOUD="{{IsAKSCustomCloud}}"
CSE_HELPERS_FILEPATH="{{GetCSEHelpersScriptFilepath}}"
CSE_DISTRO_HELPERS_FILEPATH="{{GetCSEHelpersScriptDistroFilepath}}"
CSE_INSTALL_FILEPATH="{{GetCSEInstallScriptFilepath}}"
CSE_DISTRO_INSTALL_FILEPATH="{{GetCSEInstallScriptDistroFilepath}}"
CSE_CONFIG_FILEPATH="{{GetCSEConfigScriptFilepath}}"
AZURE_PRIVATE_REGISTRY_SERVER="{{GetPrivateAzureRegistryServer}}"
HAS_CUSTOM_SEARCH_DOMAIN="{{HasCustomSearchDomain}}"
CUSTOM_SEARCH_DOMAIN_FILEPATH="{{GetCustomSearchDomainsCSEScriptFilepath}}"
HTTP_PROXY_URLS="{{GetHTTPProxy}}"
HTTPS_PROXY_URLS="{{GetHTTPSProxy}}"
NO_PROXY_URLS="{{GetNoProxy}}"
PROXY_VARS="{{GetProxyVariables}}"
ENABLE_TLS_BOOTSTRAPPING="{{EnableTLSBootstrapping}}"
ENABLE_SECURE_TLS_BOOTSTRAPPING="{{EnableSecureTLSBootstrapping}}"
DHCPV6_SERVICE_FILEPATH="{{GetDHCPv6ServiceCSEScriptFilepath}}"
DHCPV6_CONFIG_FILEPATH="{{GetDHCPv6ConfigCSEScriptFilepath}}"
THP_ENABLED="{{GetTransparentHugePageEnabled}}"
THP_DEFRAG="{{GetTransparentHugePageDefrag}}"
SERVICE_PRINCIPAL_FILE_CONTENT="{{GetServicePrincipalSecret}}"
KUBELET_CLIENT_CONTENT="{{GetKubeletClientKey}}"
KUBELET_CLIENT_CERT_CONTENT="{{GetKubeletClientCert}}"
KUBELET_CONFIG_FILE_ENABLED="{{IsKubeletConfigFileEnabled}}"
KUBELET_CONFIG_FILE_CONTENT="{{GetKubeletConfigFileContentBase64}}"
SWAP_FILE_SIZE_MB="{{GetSwapFileSizeMB}}"
GPU_DRIVER_VERSION="{{GPUDriverVersion}}"
GPU_INSTANCE_PROFILE="{{GetGPUInstanceProfile}}"
CUSTOM_SEARCH_DOMAIN_NAME="{{GetSearchDomainName}}"
CUSTOM_SEARCH_REALM_USER="{{GetSearchDomainRealmUser}}"
CUSTOM_SEARCH_REALM_PASSWORD="{{GetSearchDomainRealmPassword}}"
MESSAGE_OF_THE_DAY="{{GetMessageOfTheDay}}"
HAS_KUBELET_DISK_TYPE="{{HasKubeletDiskType}}"
NEEDS_CGROUPV2="{{IsCgroupV2}}"
TLS_BOOTSTRAP_TOKEN="{{GetTLSBootstrapTokenForKubeConfig}}"
KUBELET_FLAGS="{{GetKubeletConfigKeyVals}}"
NETWORK_POLICY="{{GetParameter "networkPolicy"}}"
{{- if not (IsKubernetesVersionGe "1.17.0")}}
KUBELET_IMAGE="{{GetHyperkubeImageReference}}"
{{end}}
{{if IsKubernetesVersionGe "1.16.0"}}
KUBELET_NODE_LABELS="{{GetAgentKubernetesLabels . }}"
{{else}}
KUBELET_NODE_LABELS="{{GetAgentKubernetesLabelsDeprecated . }}"
{{end}}
AZURE_ENVIRONMENT_FILEPATH="{{- if IsAKSCustomCloud}}/etc/kubernetes/{{GetTargetEnvironment}}.json{{end}}"
KUBE_CA_CRT="{{GetParameter "caCertificate"}}"
KUBENET_TEMPLATE="{{GetKubenetTemplate}}"
CONTAINERD_CONFIG_CONTENT="{{GetContainerdConfigContent}}"
CONTAINERD_CONFIG_NO_GPU_CONTENT="{{GetContainerdConfigNoGPUContent}}"
IS_KATA="{{IsKata}}"
ARTIFACT_STREAMING_ENABLED="{{IsArtifactStreamingEnabled}}"
SYSCTL_CONTENT="{{GetSysctlContent}}"
PRIVATE_EGRESS_PROXY_ADDRESS="{{GetPrivateEgressProxyAddress}}"
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"