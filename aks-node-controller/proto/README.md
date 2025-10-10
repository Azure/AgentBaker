This readme is to describe the new public data contract `AKSNodeConfig` between a bootstrap requester (client) and a Linux node to be bootstrapped and join an AKS cluster. The contract is defined in a set of proto files with [protobuf](https://protobuf.dev/). And we convert/compile all the proto files into specific programming languages. Currently we only convert to .go files for Go. We can convert to other languages if needed in the future. A simple way to compile the files to Go is to run this command at `AgentBaker/aks-node-controller` directory.
```
make proto-generate
```
Note: This command uses Docker to compile the proto files so you need to have Docker running otherwise you will see corresponing error message.

# Public data contract `AKSNodeConfig`
This table is describing the all the `AKSNodeConfig` Fields converted to .go files. The naming convention is a bit different in the .proto files. For example, in _config.proto_ file, you will see `api_server_config`, but in _config.pb.go_, it's automatically renamed to `ApiServerConfig`. In the following table, we will use the names defined in the .go files.

| AKSNodeConfig Fields | Types | Descriptions                                                                                                                                                                                                                                                             | OLD CSE env variables mapping |
|------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------|
| `Version` | `string` | Semantic version of this AKSNodeConfig                                                                                                                                                                                                                                   | N/A, new |
| `KubeBinaryConfig` | `KubeBinaryConfig` | Kubernetes binary URL configuration                                                                                                                                                                                                                                      | `KUBE_BINARY_URL`, `CUSTOM_KUBE_BINARY_URL`, `PRIVATE_KUBE_BINARY_URL` , `CREDENTIAL_PROVIDER_DOWNLOAD_URL` |
| `CustomCloudConfig` | `CustomCloudConfig` | Custom cloud configuration                                                                                                                                                                                                                                               | `IS_CUSTOM_CLOUD`, `AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX`, `REPO_DEPOT_ENDPOINT`, `CUSTOM_ENV_JSON` |
| `ApiServerConfig` | `ApiServerConfig` | Kubernetes API server configuration                                                                                                                                                                                                                                      | `APISERVER_PUBLIC_KEY`, `API_SERVER_NAME` |
| `ClusterConfig` | `ClusterConfig` | Various Kubernetes cluster level configuration                                                                                                                                                                                                                           | `RESOURCE_GROUP`, `LOCATION`, `VM_TYPE`, `PRIMARY_AVAILABILITY_SET`, `PRIMARY_SCALE_SET`, `USE_INSTANCE_METADATA` |
| -`ClusterNetworkConfig` | `ClusterNetworkConfig` | Cluster network config. We assumed network mode is always "transparent" now so it's removed from the contract.                                                                                                                                                           | `VIRTUAL_NETWORK`, `VIRTUAL_NETWORK_RESOURCE_GROUP`, `SUBNET`, `NETWORK_SECURITY_GROUP`, `ROUTE_TABLE` |
| -`LoadBalancerConfig` | `LoadBalancerConfig` | Load balancer config                                                                                                                                                                                                                                                     | `LOAD_BALANCER_SKU`, `EXCLUDE_MASTER_FROM_STANDARD_LB`, `MAXIMUM_LOADBALANCER_RULE_COUNT`, `LOAD_BALANCER_DISABLE_OUTBOUND_SNAT` |
| `BootstrappingConfig` | `BootstrappingConfig` | Bootstrap configuration                                                                                                                                                                                                                                                  | `ENABLE_SECURE_TLS_BOOTSTRAPPING`, `CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID` |
| `AuthConfig` | `AuthConfig` | Authentication configuration                                                                                                                                                                                                                                             | `TENANT_ID`, `SUBSCRIPTION_ID`, `SERVICE_PRINCIPAL_CLIENT_ID`, `SERVICE_PRINCIPAL_FILE_CONTENT`, `USER_ASSIGNED_IDENTITY_ID`, `USE_MANAGED_IDENTITY_EXTENSION` |
| `RuncConfig` | `RuncConfig` | The CLI tool runc configuration                                                                                                                                                                                                                                          | `RUNC_VERSION`, `RUNC_PACKAGE_URL` |
| `ContainerdConfig` | `ContainerdConfig` | Containerd configuration                                                                                                                                                                                                                                                 | `CONTAINERD_DOWNLOAD_URL_BASE`, `CONTAINERD_VERSION`, `CONTAINERD_PACKAGE_URL`, `CONTAINERD_CONFIG_CONTENT`,  `CONTAINERD_CONFIG_NO_GPU_CONTENT` |
| `TeleportConfig` | `TeleportConfig` | Teleport configuration                                                                                                                                                                                                                                                   | `TELEPORT_ENABLED`, `TELEPORTD_PLUGIN_DOWNLOAD_URL` |
| `KubeletConfig` | `KubeletConfig` | Kubelet configuration. Note that `KubeletConfig.KubeletConfigFileConfig` contains the complete contents that should be stored in the Kubelet config file /etc/default/kubeletconfig.json. The flags in `KubeletConfig.KubeletFlags` should be the same as KubeletConfig.KubeletConfigFileConfig but with different format. For example, ```KubeletFlags: map[string]string{"--address": "0.0.0.0", "--pod-manifest-path": "/etc/kubernetes/manifests"}```                                                                                                                                                                                            | `KUBELET_FLAGS`, `KUBELET_NODE_LABELS`, `HAS_KUBELET_DISK_TYPE`, `KUBELET_CONFIG_FILE_ENABLED`, `KUBELET_CONFIG_FILE_CONTENT`, `KUBELET_CLIENT_CONTENT`, `KUBELET_CLIENT_CERT_CONTENT`, `ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION` |
| `CustomSearchDomainConfig` | `CustomSearchDomainConfig` | Custom search domain configuration                                                                                                                                                                                                                                       | `CUSTOM_SEARCH_DOMAIN_NAME`, `CUSTOM_SEARCH_REALM_USER`, `CUSTOM_SEARCH_REALM_PASSWORD` |
| `CustomLinuxOSConfig` | `CustomLinuxOSConfig` | Custom Linux OS configurations including SwapFile, SysCtl configs, etc.                                                                                                                                                                                                  | `SYSCTL_CONTENT`, `CONTAINERD_ULIMITS`, `SHOULD_CONFIG_SWAP_FILE`, `SWAP_FILE_SIZE_MB`, `THP_ENABLED`, `THP_DEFRAG`, `SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE`, `SHOULD_CONFIG_CONTAINERD_ULIMITS` |
| `HTTPProxyConfig` | `HTTPProxyConfig` | HTTP/HTTPS proxy configuration for the node                                                                                                                                                                                                                              | `SHOULD_CONFIGURE_HTTP_PROXY`, `SHOULD_CONFIGURE_HTTP_PROXY_CA`, `HTTP_PROXY_TRUSTED_CA`, `HTTP_PROXY_URLS`, `HTTPS_PROXY_URLS`, `NO_PROXY_URLS`, `PROXY_VARS` |
| `GPUConfig` | `GPUConfig` | GPU configuration for the node                                                                                                                                                                                                                                           | `GPU_NODE`, `CONFIG_GPU_DRIVER_IF_NEEDED`, `ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED`, `MIG_NODE`, `GPU_INSTANCE_PROFILE` |
| `NetworkConfig` | `NetworkConfig` | Network configuration for the node                                                                                                                                                                                                                                       | `NETWORK_PLUGIN`, `NETWORK_POLICY`, `VNET_CNI_PLUGINS_URL`, `ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE` |
| `KubernetesCaCert` | `string` | Kubernetes certificate authority (CA) certificate, required by the node to establish TLS with the API server                                                                                                                                                             | `KUBE_CA_CRT` |
| `KubernetesVersion` | `string` | Kubernetes version                                                                                                                                                                                                                                                       | `KUBERNETES_VERSION` |
| `KubeProxyUrl` | `string` | Kube proxy URL                                                                                                                                                                                                                                                           | `KUBEPROXY_URL` |
| `VmSize` | `string` | The VM size of the node                                                                                                                                                                                                                                                  | N/A, new |
| `LinuxAdminUsername` | `string` | Linux admin username. If not specified, the default value is `azureuser`                                                                                                                                                                                                 | `ADMINUSER` |
| `IsVhd` | `bool` | Specifies whether the node is a VHD node. This is still needed for some customized scenarios. This is labeled as `optional` (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil.                                 | `IS_VHD` |
| `EnableSsh` | `bool` | Specifies if SSH is enabled on the VM node. This is labeled as `optional` (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil, but will be set to true on the VHD.                                               | `DISABLE_SSH` |
| `DisablePubkeyAuth` | `bool` | Specifies whether to disable ssh public key authentication on the VM node. This is labeled as optional (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil, but will be set to false on the VHD. That is, the default behavior is to enable ssh public key authentication.                                               | `DISABLE_PUBKEY_AUTH` |
| `EnableUnattendedUpgrade` | `bool` | Specifies whether unattended upgrade is enabled or disabled on the VM node                                                                                                                                                                                               | `ENABLE_UNATTENDED_UPGRADES` |
| `MessageOfTheDay` | `string` | The message of the day that is displayed on the VM node when a user logs in                                                                                                                                                                                              | `MESSAGE_OF_THE_DAY` |
| `EnableHostsConfigAgent` | `bool` | Specifies whether the hosts config agent is enabled or disabled on the VM node                                                                                                                                                                                           | `ENABLE_HOSTS_CONFIG_AGENT` |
| `CustomCaCerts` | `[]string` | Custom CA certificates to be added to the system trust store                                                                                                                                                                                                             | `SHOULD_CONFIGURE_CUSTOM_CA_TRUST`, `CUSTOM_CA_TRUST_COUNT`, `CUSTOM_CA_CERT_{{$i}}` |
| `ProvisionOutput` | `string` | A local file path where cluster provision cse output should be stored                                                                                                                                                                                                    | `PROVISION_OUTPUT` |
| `WorkloadRuntime` | `WorkloadRuntime` | Workload runtime, only "OCIContainer" currently.                                                                                                                                                                                                  | |
| `Ipv6DualStackEnabled` | `bool` | Specifies whether IPv6 dual stack is enabled or disabled on the VM node                                                                                                                                                                                                  | `IPV6_DUAL_STACK_ENABLED` |
| `OutboundCommand` | `bool` | Specifies whether IPv6 dual stack is enabled or disabled on the VM node                                                                                                                                                                                                  | `OUTBOUND_COMMAND` |
| `AzurePrivateRegistryServer` | `string` | Azure private registry server URI                                                                                                                                                                                                                                        | `AZURE_PRIVATE_REGISTRY_SERVER` |
| `PrivateEgressProxyAddress` | `string` | Private egress proxy address                                                                                                                                                                                                                                             | `PRIVATE_EGRESS_PROXY_ADDRESS` |
| `EnableArtifactStreaming` | `bool` | Specifies whether artifact streaming is enabled or disabled on the VM node                                                                                                                                                                                               | `ARTIFACT_STREAMING_ENABLED` |
| `IsKata` | `bool` | Specifies if it is a Kata node                                                                                                                                                                                                                                           | `IS_KATA` |
| `NeedsCgroupv2` | `*bool` | Specifies whether the node needs cgroupv2. Labeled as `optional` (explicit presence) so that we know whether it's set or not. If it's not set, the default value will be nil and it's defaulted to false. Future plan is to get the value from VHD during bootstrapping. | `NEEDS_CGROUPV2` |
| `BootstrapProfileContainerRegistryServer` | `string` | Bootstrap profile container registry server URI                                                                                                                                                                                                                          | `BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER` |
| `IMDSRestrictionConfig` | `IMDSRestrictionConfig` | IMDS restriction configuration                                                                                                                                                                                                                                           | `ENABLE_IMDS_RESTRICTION`, `INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE`|


Removed old environment variables from cse_cmd.sh:
`CSE_HELPERS_FILEPATH`, `CSE_DISTRO_HELPERS_FILEPATH`, `CSE_INSTALL_FILEPATH`, `CSE_DISTRO_INSTALL_FILEPATH`, `CSE_CONFIG_FILEPATH`, `DHCPV6_SERVICE_FILEPATH`, `DHCPV6_CONFIG_FILEPATH`, `CLI_TOOL`, `MOBY_VERSION`, `HYPERKUBE_URL`, `SGX_NODE`, `GPU_DRIVER_TYPE` and more.

Many variables are changed to optional and we have a builder function as a helper to provide default values. For example, the builder function defaults `LinuxAdminUsername` to value `azureuser`, `OutboundCommand` to a default outbound command `curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/`.

# Guideline to add a new variable to AKSNodeConfig
## Why Protobuf? (Feel free to skip)
We use `Protobuf`.`proto3` to define the data contract and make use of its benefits as follows:
- Support across different programming languages
- Schema definition in a structured way
- Easier to validate at compile time
- Natively support backward/forward compatibility

Protobuf provides another benefit that we are not planning to use yet, which is encoding/decoding the payload. Since we are only bootstrapping the node once at the first boot, the transfer speed is not the major concern of this project. In the future, we can still consider transferring encoded payload. The proposed design is flexible to adapt to this future change.

## Defining a variable in the contract
In protobuf, a variable can be defined as one of the general types: bool, string, a group of sub-level variables, an array of variables, etc. Here are some examples.
| In protobuf | In Go |
|-------------|-------|
|string var1|Var1 string|
|bool var2|Var2 bool|
|repeated string var3|Var3 []string|
|GroupType var4|Var4 *GroupType|
|optional bool var5|Var5 *bool|

## When to use the label `optional` specifically in `proto3`? (Feel free to come back to read this section when needed. You can skip to next section _High level Steps_)
For 90% of the cases, we don't need to add label `optional`.
In `proto3`, variable without `optional` label is considered as no presence and the one with `optional` label is explicit presence. Application Note: Field Presence | Protocol Buffers Documentation (protobuf.dev)
In an intuitive way to explain this,
1.	No presence (without `optional` label)
If this variable’s value is unset, the consumer (in our case, bootstrappers) will get a default value based on its type.
For example, if a bool variable’s value is unset, the consumer will get false.
The default value for an unset string variable is an empty string.
2.	Explicit presence (with `optional` label)
If this variable’s value is unset, the consumer will get a nil value. With that, the feature owner can use this additional state (besides true and false for a bool) to add some logic to it.

Considering an evolution scenario where we should be adding a label `optional`. We will explain what the effect of adding this label is.
There is a new feature AwesomeFeature, which will replace an old feature OldFeature gradually. It is still in a pre-production state and is not ready in the VHD provisioning process yet. A dev adds a new variable AwesomeFeature to the contract and set it as false. The label `optional` should be added to this variable.
An evolutional scenario will look like this,
1.	When AwesomeFeature is not yet available and the OldFeature is still running:
AwesomeFeature = false, OldFeature=true
2.	When AwesomeFeature is available in production and the OldFeature is also available:
AwesomeFeature = true, OldFeature=false
3.	When OldFeature is deprecated and AwesomeFeature=true is the only option:
The feature owner can of course request the producer of the contract payload (the bootstrapper) to always set AwesomeFeature = false. Given that we don’t allow the removal of a variable from the contract because it’s a breaking change that breaks compatibility, another more elegant way is to loosen this requirement. That is, even if the value is not set, we can still control the default value in the Go binary in VHD. Without `optional` label, the default value will be automatically assigned by the protobuf compiled codes so we can’t tell if the value is from defaulting or from the producer’s explicit assignment. But with `optional` label, if the value is not set, we will get a nil for bool, and so on. Therefore, the feature owner can add handling logic in the codes by making use of this additional feature state.

Notes: In proto3, all variables are _optional_, not required. Thus indeed, `optional` label in proto3 doesn’t really mean it’s an _optional_ variable. It’s saying that it’s explicit presence. (I know it’s confusing). The concept of `optional` label is to distinguish between these 2 cases.
-	A variable is not set, meaning assigned with no value. In case of no presence, it will be automatically assigned with the default value. Depending on different types of variables, `proto3` has different default values. Check more [here](https://protobuf.dev/programming-guides/proto3/#default) if interested.
-	A variable is explicitly assigned with a value, which happens to be the default value.
For some cases, knowing that the variable is not set is important. Then the feature owner can handle it with additional logic.
For the best practice, if the feature doesn’t require distinguishing the 2 cases above, please don’t add an `optional` label. If it’s needed to distinguish between, please add an `optional` label.
Nevertheless, it’s not a big harm to use `optional` even though it’s not needed. It’s just on the consumer side, you will need to either use the proto3 generated getters, which ensures non-nil value, or handle the nil value properly by yourself. But you may also want to let other people know that it could be nil value when they use the variable you added.


## High level Steps
1. Update corresponding .proto files to the data contract. Usually we start with `config.proto`.
2. From the `AgentBaker/aks-node-controller` directory run `make proto-generate` to compile the .proto definitions into `Go`; this regenerates the public API (the `AKSNodeConfig` Go types).
3. Tell how VHD should react to this new variable by updating the bootscripts as you do before. Basically you will be modifying shell scripts like `install-dependencies.sh`, `cse_install.sh`, `cse_helpers.sh`, etc. You may also want to add some unit tests to spec files like `cse_install_spec.sh`, `cse_helpers.sh` to find bugs earlier.
4. On the VHD side, we are still invoking the bootstrap scripts under the hood. To set the environment variables of the CSE trigger command, add the desired variable to `getCSEEnv()` in [parser.go](https://github.com/Azure/AgentBaker/blob/dev/aks-node-controller/parser/parser.go). If you need to add a corresponding file to the VHD, please generate the file in the bootstrap scripts rather than adding to [`nodecustomdata.yml`](https://github.com/Azure/AgentBaker/blob/dev/parts/linux/cloud-init/nodecustomdata.yml) as this file will eventually be deprecated. Here is an [example](https://github.com/Azure/AgentBaker/commit/81ce18fb7f53acab3c7fe8f3a70b635baf1f2f52) for generating the kube CA cert.

    Note: Node SIG is working on migrating all scripts to managable Go binary. Before it's done, the bootstrap scripts will still be used.

5. Set default values for your variables, if the existing defaulting provided by `proto3` doesn't fit your purpose. For example, if a bool variable is not set, `proto` will default it to `false`. However, if you want to default it to `true`, then you can set your own default function. `getDisableSSH` in [helper.go](https://github.com/Azure/AgentBaker/blob/dev/aks-node-controller/parser/helper.go) is 1 example.

## Detailed steps with example
Example: IMDSRestrictionConfig [Example PR](https://github.com/Azure/AgentBaker/pull/5154)
1. Create a proto file with name `imdsrestrictionconfig.proto` with the following contents.
    ```
      syntax = "proto3";
      package aksnodeconfig.v1;

      message IMDSRestrictionConfig {
        // Enable IMDS restriction for the node.
        bool enable_imds_restriction = 1;

        // Insert IMDS restriction rule to mangle table.
        bool insert_imds_restriction_rule_to_mangle_table = 2;
      }
    ```
2. In the root level .proto file `config.proto`, import the newly created file with `import "pkg/proto/aksnodeconfig/v1/imdsrestrictionconfig.proto";`. Add `IMDSRestrictionConfig` in the message body such as:
    ```
      // IMDS restriction configuration
      IMDSRestrictionConfig imds_restriction_config = 39;
    ```

3. Once you finished step 2, `proto3` actually created some getters that we can use. For example, in the `imdsrestrictionconfig.pb.go` that was automatically created, you can find `GetEnableImdsRestriction` and `GetInsertImdsRestrictionRuleToMangleTable`. Therefore, in `aks-node-controller/parser/parser.go`, which is a Go func that will be used to generate the bootstrap command, you can add the following lines: [parser.go](https://github.com/Azure/AgentBaker/blob/dev/aks-node-controller/parser/parser.go#L165-L166)

    ```
    "ENABLE_IMDS_RESTRICTION":                        fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetEnableImdsRestriction()),
        "INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE":   fmt.Sprintf("%v", config.GetImdsRestrictionConfig().GetInsertImdsRestrictionRuleToMangleTable()),
    ```

    **Default value behavior:**
If the client (such as AKS-RP) doesn't specify a value for `EnableImdsRestriction`, it will default to `false`. You can see this defaulting logic in the generated `GetEnableImdsRestriction` method in `imdsrestrictionconfig.pb.go`.

    This should fit most use cases. However, if you need to explicitly distinguish between a client setting `false` versus not setting the value at all (which defaults to `false`), you'll need to use the `optional` label for explicit presence. In that case, refer to the earlier section _When to use the label `optional` specifically in `proto3`?_

4. Add comprehensive tests to cover your changes.

   **Testing with AKSNodeConfig approach:**
   - Add test cases using the `AKSNodeConfig` approach, such as `Test_AzureLinuxV2_ARM64_Scriptless` in `e2e/scenario_test.go`
   - The key difference between the legacy and new approaches is the configuration interface:
     - **Legacy approach:** Uses `datamodel.NodeBootstrappingConfiguration`
     - **New approach:** Uses `AKSNodeConfig`
   - In e2e tests (`scenario_test.go`), this means:
     - **Legacy:** Use `BootstrapConfigMutator` to set configurations
     - **New:** Use `AKSNodeConfigMutator` to set configurations
