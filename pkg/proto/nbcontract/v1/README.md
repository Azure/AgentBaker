This readme is to describe the new public data contract `AKSNodeConfig` between a bootstrap requester (client) and a Linux node to be bootstrapped and join an AKS cluster. The contract is defined in a set of proto files with [protobuf](https://protobuf.dev/). And we convert/compile all the proto files into specific programming languages. Currently we only convert to .go files for Go. We can convert to other languages if needed in the future. A simple way to compile the files to Go is to run this command at `AgentBaker` root directory.
```
make compile-proto-files
``` 


# Public data contract `AKSNodeConfig`
This table is describing the all the AKSNodeConfig Fields converted to .go files. The naming convention is a bit different in the .proto files. For example, in _config.proto_ file, you will see `api_server_config`, but in _config.pb.go_, it's automatically renamed to `ApiServerConfig`. In the following table, we will use the names defined in the .go files.

| AKSNodeConfig Fields | Types | Descriptions | OLD CSE env variables mapping |
|------------|------------|--------------|-------------------------------|
| `Version` | `string` | Semantic version of this node bootstrap contract | N/A, new |
| `KubeBinaryConfig` | `KubeBinaryConfig` | Kubernetes binary URL configuration | `KUBE_BINARY_URL`, `CUSTOM_KUBE_BINARY_URL`, `PRIVATE_KUBE_BINARY_URL` , `CREDENTIAL_PROVIDER_DOWNLOAD_URL` |
| `CustomCloudConfig` | `CustomCloudConfig` | Custom cloud configuration | `IS_CUSTOM_CLOUD`, `AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX`, `REPO_DEPOT_ENDPOINT`, `CUSTOM_ENV_JSON` |
| `ApiServerConfig` | `ApiServerConfig` | Kubernetes API server configuration | `APISERVER_PUBLIC_KEY`, `API_SERVER_NAME` |
| `ClusterConfig` | `ClusterConfig` | Various Kubernetes cluster level configuration | `RESOURCE_GROUP`, `LOCATION`, `VM_TYPE`, `PRIMARY_AVAILABILITY_SET`, `PRIMARY_SCALE_SET`, `USE_INSTANCE_METADATA` |
| -`ClusterNetworkConfig` | `ClusterNetworkConfig` | Cluster network config. We assumed network mode is always "transparent" now so it's removed from the contract. | `VIRTUAL_NETWORK`, `VIRTUAL_NETWORK_RESOURCE_GROUP`, `SUBNET`, `NETWORK_SECURITY_GROUP`, `ROUTE_TABLE` |
| -`LoadBalancerConfig` | `LoadBalancerConfig` | Load balancer config | `LOAD_BALANCER_SKU`, `EXCLUDE_MASTER_FROM_STANDARD_LB`, `MAXIMUM_LOADBALANCER_RULE_COUNT`, `LOAD_BALANCER_DISABLE_OUTBOUND_SNAT` |
| `TlsBootstrappingConfig` | `TLSBootstrappingConfig` | TLS bootstrap configuration | `ENABLE_TLS_BOOTSTRAPPING`, `ENABLE_SECURE_TLS_BOOTSTRAPPING`, `CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID` |
| `AuthConfig` | `AuthConfig` | Authentication configuration | `TENANT_ID`, `SUBSCRIPTION_ID`, `SERVICE_PRINCIPAL_CLIENT_ID`, `SERVICE_PRINCIPAL_FILE_CONTENT`, `USER_ASSIGNED_IDENTITY_ID`, `USE_MANAGED_IDENTITY_EXTENSION` |
| `RuncConfig` | `RuncConfig` | The CLI tool runc configuration | `RUNC_VERSION`, `RUNC_PACKAGE_URL` |
| `ContainerdConfig` | `ContainerdConfig` | Containerd configuration | `CONTAINERD_DOWNLOAD_URL_BASE`, `CONTAINERD_VERSION`, `CONTAINERD_PACKAGE_URL` |
| `TeleportConfig` | `TeleportConfig` | Teleport configuration | `TELEPORT_ENABLED`, `TELEPORTD_PLUGIN_DOWNLOAD_URL` |
| `KubeletConfig` | `KubeletConfig` | Kubelet configuration | `KUBELET_FLAGS`, `KUBELET_NODE_LABELS`, `HAS_KUBELET_DISK_TYPE`, `KUBELET_CONFIG_FILE_ENABLED`, `KUBELET_CONFIG_FILE_CONTENT`, `KUBELET_CLIENT_CONTENT`, `KUBELET_CLIENT_CERT_CONTENT` |
| `CustomSearchDomainConfig` | `CustomSearchDomainConfig` | Custom search domain configuration | `CUSTOM_SEARCH_DOMAIN_NAME`, `CUSTOM_SEARCH_REALM_USER`, `CUSTOM_SEARCH_REALM_PASSWORD` |
| `CustomLinuxOSConfig` | `CustomLinuxOSConfig` | Custom Linux OS configurations including SwapFile, SysCtl configs, etc. | `SYSCTL_CONTENT`, `CONTAINERD_ULIMITS`, `SHOULD_CONFIG_SWAP_FILE`, `SWAP_FILE_SIZE_MB`, `THP_ENABLED`, `THP_DEFRAG`, `SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE`, `SHOULD_CONFIG_CONTAINERD_ULIMITS` |
| `HTTPProxyConfig` | `HTTPProxyConfig` | HTTP/HTTPS proxy configuration for the node | `SHOULD_CONFIGURE_HTTP_PROXY`, `SHOULD_CONFIGURE_HTTP_PROXY_CA`, `HTTP_PROXY_TRUSTED_CA`, `HTTP_PROXY_URLS`, `HTTPS_PROXY_URLS`, `NO_PROXY_URLS`, `PROXY_VARS` |
| `GPUConfig` | `GPUConfig` | GPU configuration for the node | `GPU_NODE`, `CONFIG_GPU_DRIVER_IF_NEEDED`, `ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED`, `MIG_NODE`, `GPU_INSTANCE_PROFILE` |
| `NetworkConfig` | `NetworkConfig` | Network configuration for the node | `NETWORK_PLUGIN`, `NETWORK_POLICY`, `VNET_CNI_PLUGINS_URL`, `ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE` |
| `KubernetesCaCert` | `string` | Kubernetes certificate authority (CA) certificate, required by the node to establish TLS with the API server | `KUBE_CA_CRT` |
| `KubernetesVersion` | `string` | Kubernetes version | `KUBERNETES_VERSION` |
| `KubeProxyUrl` | `string` | Kube proxy URL | `KUBEPROXY_URL` |
| `VmSize` | `string` | The VM size of the node | N/A, new |
| `LinuxAdminUsername` | `string` | Linux admin username. If not specified, the default value is `azureuser` | `ADMINUSER` |
| `IsVhd` | `bool` | Specifies whether the node is a VHD node. This is still needed for some customized scenarios. This is labeled as optional (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil. | `IS_VHD` |
| `EnableSsh` | `bool` | Specifies if SSH is enabled on the VM node. This is labeled as optional (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil, but will be set to true on the VHD. | `DISABLE_SSH` |
| `EnableUnattendedUpgrade` | `bool` | Specifies whether unattended upgrade is enabled or disabled on the VM node | `ENABLE_UNATTENDED_UPGRADES` |
| `MessageOfTheDay` | `string` | The message of the day that is displayed on the VM node when a user logs in | `MESSAGE_OF_THE_DAY` |
| `EnableHostsConfigAgent` | `bool` | Specifies whether the hosts config agent is enabled or disabled on the VM node | `ENABLE_HOSTS_CONFIG_AGENT` |
| `CustomCaCerts` | `[]string` | Custom CA certificates to be added to the system trust store | `SHOULD_CONFIGURE_CUSTOM_CA_TRUST`, `CUSTOM_CA_TRUST_COUNT`, `CUSTOM_CA_CERT_{{$i}}` |
| `ProvisionOutput` | `string` | A local file path where cluster provision cse output should be stored | `PROVISION_OUTPUT` |
| `WorkloadRuntime` | `WorkloadRuntime` | Workload runtime, e.g., either "OCIContainer" or "WasmWasi", currently. | `IS_KRUSTLET` |
| `Ipv6DualStackEnabled` | `bool` | Specifies whether IPv6 dual stack is enabled or disabled on the VM node | `IPV6_DUAL_STACK_ENABLED` |
| `OutboundCommand` | `bool` | Specifies whether IPv6 dual stack is enabled or disabled on the VM node | `OUTBOUND_COMMAND` |
| `AzurePrivateRegistryServer` | `string` | Azure private registry server URI | `AZURE_PRIVATE_REGISTRY_SERVER` |
| `PrivateEgressProxyAddress` | `string` | Private egress proxy address | `PRIVATE_EGRESS_PROXY_ADDRESS` |
| `PrivateEgressProxyAddress` | `bool` | Specifies whether artifact streaming is enabled or disabled on the VM node | `ARTIFACT_STREAMING_ENABLED` |
| `IsKata` | `bool` | Specifies if it is a Kata node | `IS_KATA` |
| `NeedsCgroupv2` | `*bool` | Specifies whether the node needs cgroupv2. Labeled as optional (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil and it's defaulted to false. Future plan is to get the value from VHD during bootstrapping. | `NEEDS_CGROUPV2` |