echo $(date),$(hostname);
{{GetVariable "outBoundCmd"}}
for i in $(seq 1 1200); do
grep -Fq "EOF" /opt/azure/containers/provision.sh && break;
if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi;
done;
{{if IsAKSCustomCloud}}
for i in $(seq 1 1200); do
grep -Fq "EOF" {{GetInitAKSCustomCloudFilepath}} && break;
if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi;
done;
{{GetInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
ADMINUSER={{GetParameter "linuxAdminUsername"}}
CONTAINERD_VERSION={{GetParameter "containerdVersion"}}
MOBY_VERSION={{GetParameter "mobyVersion"}}
TENANT_ID={{GetVariable "tenantID"}}
KUBERNETES_VERSION={{GetParameter "kubernetesVersion"}}
HYPERKUBE_URL={{GetParameter "kubernetesHyperkubeSpec"}}
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
SERVICE_PRINCIPAL_CLIENT_SECRET='{{GetParameter "servicePrincipalClientSecret"}}'
KUBELET_PRIVATE_KEY={{GetParameter "clientPrivateKey"}}
NETWORK_PLUGIN={{GetParameter "networkPlugin"}}
NETWORK_POLICY={{GetParameter "networkPolicy"}}
VNET_CNI_PLUGINS_URL={{GetParameter "vnetCniLinuxPluginsURL"}}
CNI_PLUGINS_URL={{GetParameter "cniPluginsURL"}}
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
CONTAINERD_DOWNLOAD_URL_BASE={{GetParameter "containerdDownloadURLBase"}}
NETWORK_MODE={{GetParameter "networkMode"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
USER_ASSIGNED_IDENTITY_ID={{GetVariable "userAssignedIdentityID"}}
API_SERVER_NAME={{GetKubernetesEndpoint}}
IS_VHD={{GetVariable "isVHD"}}
GPU_NODE={{GetVariable "gpuNode"}}
SGX_NODE={{GetVariable "sgxNode"}}
AUDITD_ENABLED={{GetVariable "auditdEnabled"}} 
CONFIG_GPU_DRIVER_IF_NEEDED={{GetVariable "configGPUDriverIfNeeded"}}
ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED={{GetVariable "enableGPUDevicePluginIfNeeded"}}
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1; systemctl --no-pager -l status kubelet 2>&1 | head -n 100"