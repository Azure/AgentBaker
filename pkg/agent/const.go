// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

const (
	// DefaultVNETCIDR is the default CIDR block for the VNET.
	DefaultVNETCIDR = "10.0.0.0/8"
	// DefaultVNETCIDRIPv6 is the default IPv6 CIDR block for the VNET.
	DefaultVNETCIDRIPv6 = "2001:1234:5678:9a00::/56"
	// NetworkPolicyCalico is the string expression for calico network policy config option.
	NetworkPolicyCalico = "calico"
	// NetworkPolicyCilium is the string expression for cilium network policy config option.
	NetworkPolicyCilium = "cilium"
	// NetworkPluginCilium is the string expression for cilium network plugin config option.
	NetworkPluginCilium = NetworkPolicyCilium
	// NetworkPolicyAntrea is the string expression for antrea network policy config option.
	NetworkPolicyAntrea = "antrea"
	// NetworkPolicyAzure is the string expression for Azure CNI network policy manager.
	NetworkPolicyAzure = "azure"
	// NetworkPluginAzure is the string expression for Azure CNI plugin.
	NetworkPluginAzure = "azure"
	// NetworkPluginKubenet is the string expression for kubenet network plugin.
	NetworkPluginKubenet = "kubenet"
	// NetworkPluginFlannel is the string expression for flannel network plugin.
	NetworkPluginFlannel = "flannel"
)

const (
	// kubernetesWindowsAgentCSECommandPS1 privides the command of Windows CSE.
	kubernetesWindowsAgentCSECommandPS1 = "windows/csecmd.ps1"
	// kubernetesWindowsAgentCustomDataPS1 is used for generating the customdata of Windows VM.
	kubernetesWindowsAgentCustomDataPS1 = "windows/kuberneteswindowssetup.ps1.template"
	/* Windows CSE helper scripts. These should all be listed in
	baker.go:func GetKubernetesWindowsAgentFunctions. */
	kubernetesWindowsCSEHelperPS1 = "windows/windowscsehelper.ps1"
	/* Windows script to upload CSE logs. These should all be listed in
	baker.go:func GetKubernetesWindowsAgentFunctions. */
	kubernetesWindowsSendLogsPS1 = "windows/sendlogs.ps1"
)

// cloud-init (i.e. ARM customData) source file references.
const (
	kubernetesFlatcarNodeCustomDataYaml   = "linux/cloud-init/flatcar.yml"
	kubernetesACLNodeCustomDataYaml       = "linux/cloud-init/acl.yml"
	kubernetesNodeCustomDataYaml          = "linux/cloud-init/nodecustomdata.yml"
	kubernetesCSECommandString            = "linux/cloud-init/artifacts/cse_cmd.sh"
	kubernetesCSEStartScript              = "root/common/opt/azure/containers/provision_start.sh"
	kubernetesCSEMainScript               = "root/common/opt/azure/containers/provision.sh"
	kubernetesCSEHelpersScript            = "root/common/opt/azure/containers/provision_source.sh"
	kubernetesCSEHelpersScriptUbuntu      = "root/ubuntu/opt/azure/containers/provision_source_distro.sh"
	kubernetesCSEHelpersScriptMariner     = "root/mariner/opt/azure/containers/provision_source_distro.sh"
	kubernetesCSEHelpersScriptAzlOSGuard  = "root/azlosguard/opt/azure/containers/provision_source_distro.sh"
	kubernetesCSEHelpersScriptFlatcar     = "root/flatcar/opt/azure/containers/provision_source_distro.sh"
	kubernetesCSEHelpersScriptACL         = "root/acl/opt/azure/containers/provision_source_distro.sh"
	kubernetesCSEInstall                  = "root/common/opt/azure/containers/provision_installs.sh"
	kubernetesCSEInstallUbuntu            = "root/ubuntu/opt/azure/containers/provision_installs_distro.sh"
	kubernetesCSEInstallMariner           = "root/mariner/opt/azure/containers/provision_installs_distro.sh"
	kubernetesCSEInstallAzlOSGuard        = "root/azlosguard/opt/azure/containers/provision_installs_distro.sh"
	kubernetesCSEInstallFlatcar           = "root/flatcar/opt/azure/containers/provision_installs_distro.sh"
	kubernetesCSEInstallACL               = "root/acl/opt/azure/containers/provision_installs_distro.sh"
	kubernetesCSEConfig                   = "root/common/opt/azure/containers/provision_configs.sh"
	kubernetesCSEConfigGPU                = "root/common/opt/azure/containers/provision_configs_gpu.sh"
	kubernetesCSEConfigLocalDNS           = "root/common/opt/azure/containers/provision_configs_localdns.sh"
	kubernetesCSEConfigKubelet            = "root/common/opt/azure/containers/provision_configs_kubelet.sh"
	kubernetesCSEConfigNetwork            = "root/common/opt/azure/containers/provision_configs_network.sh"
	kubernetesCSEConfigChrony             = "root/common/opt/azure/containers/provision_configs_chrony.sh"
	kubernetesCSESendLogs                 = "root/common/opt/azure/containers/provision_send_logs.py"
	kubernetesCSERedactCloudConfig        = "root/common/opt/azure/containers/provision_redact_cloud_config.py"
	kubernetesCustomSearchDomainsScript   = "root/linux/opt/azure/containers/setup-custom-search-domains.sh"
	kubeletSystemdService                 = "root/common/etc/systemd/system/kubelet.service"
	kmsSystemdService                     = "root/common/etc/systemd/system/kms.service"
	aptPreferences                        = "root/linux/etc/apt/preferences"
	dockerClearMountPropagationFlags      = "root/ubuntu/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf"
	reconcilePrivateHostsScript           = "root/common/opt/azure/containers/reconcilePrivateHosts.sh"
	reconcilePrivateHostsService          = "root/common/etc/systemd/system/reconcile-private-hosts.service"
	bindMountScript                       = "root/common/opt/azure/containers/bind-mount.sh"
	bindMountSystemdService               = "root/common/etc/systemd/system/bind-mount.service"
	snapshotUpdateScript                  = "root/ubuntu/opt/azure/containers/ubuntu-snapshot-update.sh"
	snapshotUpdateSystemdService          = "root/linux/etc/systemd/system/snapshot-update.service"
	snapshotUpdateSystemdTimer            = "root/linux/etc/systemd/system/snapshot-update.timer"
	packageUpdateScriptMariner            = "root/mariner/opt/azure/containers/mariner-package-update.sh"
	packageUpdateSystemdServiceMariner    = "root/mariner/etc/systemd/system/snapshot-update.service"
	packageUpdateSystemdTimerMariner      = "root/mariner/etc/systemd/system/snapshot-update.timer"
	migPartitionScript                    = "root/linux/opt/azure/containers/mig-partition.sh"
	migPartitionSystemdService            = "root/linux/etc/systemd/system/mig-partition.service"
	ensureIMDSRestrictionScript           = "root/common/opt/azure/containers/ensure_imds_restriction.sh"
	validateKubeletCredentialsScript      = "root/common/opt/azure/containers/validate-kubelet-credentials.sh"
	secureTLSBootstrapService             = "root/common/etc/systemd/system/secure-tls-bootstrap.service"
	cloudInitStatusCheckScript            = "root/common/opt/azure/containers/cloud-init-status-check.sh"
	measureTLSBootstrappingLatencyScript  = "root/common/opt/azure/containers/measure-tls-bootstrapping-latency.sh"
	measureTLSBootstrappingLatencyService = "root/common/etc/systemd/system/measure-tls-bootstrapping-latency.service"

	// scripts and service for enabling ipv6 dual stack.
	dhcpv6SystemdService      = "root/common/etc/systemd/system/dhcpv6.service"
	dhcpv6ConfigurationScript = "root/common/opt/azure/containers/enable-dhcpv6.sh"

	// script for initializing the AKS node cloud environment (cert bootstrap + repo init); runs on all clouds.
	initAKSCloudScript = "root/common/opt/azure/containers/init-aks-cloud.sh"

	ensureNoDupEbtablesScript  = "root/common/opt/azure/containers/ensure-no-dup.sh"
	ensureNoDupEbtablesService = "root/common/etc/systemd/system/ensure-no-dup.service"

	// Azure network configuration files.
	configureAzureNetworkScript = "root/common/opt/azure-network/configure-azure-network.sh"
	azureNetworkUdevRule        = "root/common/etc/udev/rules.d/99-azure-network.rules"

	componentManifestFile = "linux/cloud-init/artifacts/manifest.json"

	hotfixJSONFile = "linux/cloud-init/artifacts/aks-node-controller-hotfix.json"
)

// cloud-init destination file references.
const (
	cseHelpersScriptFilepath             = "/opt/azure/containers/provision_source.sh"
	cseHelpersScriptDistroFilepath       = "/opt/azure/containers/provision_source_distro.sh"
	cseInstallScriptFilepath             = "/opt/azure/containers/provision_installs.sh"
	cseInstallScriptDistroFilepath       = "/opt/azure/containers/provision_installs_distro.sh"
	cseConfigScriptFilepath              = "/opt/azure/containers/provision_configs.sh"
	cseConfigGPUScriptFilepath           = "/opt/azure/containers/provision_configs_gpu.sh"
	cseConfigLocalDNSScriptFilepath      = "/opt/azure/containers/provision_configs_localdns.sh"
	cseConfigKubeletScriptFilepath       = "/opt/azure/containers/provision_configs_kubelet.sh"
	cseConfigNetworkScriptFilepath       = "/opt/azure/containers/provision_configs_network.sh"
	cseConfigChronyScriptFilepath        = "/opt/azure/containers/provision_configs_chrony.sh"
	customSearchDomainsCSEScriptFilepath = "/opt/azure/containers/setup-custom-search-domains.sh"
	dhcpV6ServiceCSEScriptFilepath       = "/etc/systemd/system/dhcpv6.service"
	dhcpV6ConfigCSEScriptFilepath        = "/opt/azure/containers/enable-dhcpv6.sh"
	initAKSCloudFilepath                 = "/opt/azure/containers/init-aks-cloud.sh"
	aksNbcCmdFilepath                    = "/opt/azure/containers/aks-node-controller-nbc-cmd.sh"
	aksNodeCustomDataFilepath            = "/opt/azure/containers/nodecustomdata.yml"
	aksNodeConfigFilepath                = "/opt/azure/containers/aks-node-controller-config.json"
	aksHotfixJSONFilepath                = "/opt/azure/containers/aks-node-controller-hotfix.json"
	// enabledFeaturesFilepath is the KEY=VALUE feature-flag file the aks-node-controller wrapper
	// reads (its FEATURES_PATH). Scriptless custom data drops it only when a feature toggle is set.
	enabledFeaturesFilepath = "/opt/azure/containers/enabled_features.sh"
)

const (
	// AADPodIdentityAddonName is the name of the aad-pod-identity addon deployment.
	AADPodIdentityAddonName = "aad-pod-identity"
	// ACIConnectorAddonName is the name of the aci-connector addon deployment.
	ACIConnectorAddonName = "aci-connector"
)
