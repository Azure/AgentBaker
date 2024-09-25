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
	kubernetesWindowsAgentCustomDataPS1 = "windows/kuberneteswindowssetup.ps1"
	/* Windows CSE helper scripts. These should all be listed in
	baker.go:func GetKubernetesWindowsAgentFunctions. */
	kubernetesWindowsCSEHelperPS1 = "windows/windowscsehelper.ps1"
	/* Windows script to upload CSE logs. These should all be listed in
	baker.go:func GetKubernetesWindowsAgentFunctions. */
	kubernetesWindowsSendLogsPS1 = "windows/sendlogs.ps1"
)

// cloud-init (i.e. ARM customData) source file references.
const (
	kubernetesNodeCustomDataYaml        = "linux/cloud-init/nodecustomdata.yml"
	kubernetesCSECommandString          = "linux/cloud-init/artifacts/cse_cmd.sh"
	kubernetesCSEStartScript            = "linux/cloud-init/artifacts/cse_start.sh"
	kubernetesCSEMainScript             = "linux/cloud-init/artifacts/cse_main.sh"
	kubernetesCSEHelpersScript          = "linux/cloud-init/artifacts/cse_helpers.sh"
	kubernetesCSEHelpersScriptUbuntu    = "linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
	kubernetesCSEHelpersScriptMariner   = "linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"
	kubernetesCSEInstall                = "linux/cloud-init/artifacts/cse_install.sh"
	kubernetesCSEInstallUbuntu          = "linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
	kubernetesCSEInstallMariner         = "linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"
	kubernetesCSEConfig                 = "linux/cloud-init/artifacts/cse_config.sh"
	kubernetesCSESendLogs               = "linux/cloud-init/artifacts/cse_send_logs.py"
	kubernetesCSERedactCloudConfig      = "linux/cloud-init/artifacts/cse_redact_cloud_config.py"
	kubernetesCISScript                 = "linux/cloud-init/artifacts/cis.sh"
	kubernetesCustomSearchDomainsScript = "linux/cloud-init/artifacts/setup-custom-search-domains.sh"
	kubeletSystemdService               = "linux/cloud-init/artifacts/kubelet.service"
	kmsSystemdService                   = "linux/cloud-init/artifacts/kms.service"
	aptPreferences                      = "linux/cloud-init/artifacts/apt-preferences"
	dockerClearMountPropagationFlags    = "linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf"
	reconcilePrivateHostsScript         = "linux/cloud-init/artifacts/reconcile-private-hosts.sh"
	reconcilePrivateHostsService        = "linux/cloud-init/artifacts/reconcile-private-hosts.service"
	bindMountScript                     = "linux/cloud-init/artifacts/bind-mount.sh"
	bindMountSystemdService             = "linux/cloud-init/artifacts/bind-mount.service"
	snapshotUpdateScript                = "linux/cloud-init/artifacts/ubuntu/ubuntu-snapshot-update.sh"
	snapshotUpdateSystemdService        = "linux/cloud-init/artifacts/ubuntu/snapshot-update.service"
	snapshotUpdateSystemdTimer          = "linux/cloud-init/artifacts/ubuntu/snapshot-update.timer"
	packageUpdateScriptMariner          = "linux/cloud-init/artifacts/mariner/mariner-package-update.sh"
	packageUpdateSystemdServiceMariner  = "linux/cloud-init/artifacts/mariner/package-update.service"
	packageUpdateSystemdTimerMariner    = "linux/cloud-init/artifacts/mariner/package-update.timer"
	migPartitionScript                  = "linux/cloud-init/artifacts/mig-partition.sh"
	migPartitionSystemdService          = "linux/cloud-init/artifacts/mig-partition.service"
	ensureIMDSRestrictionScript         = "linux/cloud-init/artifacts/ensure_imds_restriction.sh"

	// scripts and service for enabling ipv6 dual stack.
	dhcpv6SystemdService            = "linux/cloud-init/artifacts/dhcpv6.service"
	dhcpv6ConfigurationScript       = "linux/cloud-init/artifacts/enable-dhcpv6.sh"
	initAKSCustomCloudScript        = "linux/cloud-init/artifacts/init-aks-custom-cloud.sh"
	initAKSCustomCloudMarinerScript = "linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh"

	ensureNoDupEbtablesScript  = "linux/cloud-init/artifacts/ensure-no-dup.sh"
	ensureNoDupEbtablesService = "linux/cloud-init/artifacts/ensure-no-dup.service"

	// drop ins.
	containerdKubeletDropin = "linux/cloud-init/artifacts/10-containerd.conf"
	cgroupv2KubeletDropin   = "linux/cloud-init/artifacts/10-cgroupv2.conf"
	componentConfigDropin   = "linux/cloud-init/artifacts/10-componentconfig.conf"
	tlsBootstrapDropin      = "linux/cloud-init/artifacts/10-tlsbootstrap.conf"
	bindMountDropin         = "linux/cloud-init/artifacts/10-bindmount.conf"
	httpProxyDropin         = "linux/cloud-init/artifacts/10-httpproxy.conf"
	componentManifestFile   = "linux/cloud-init/artifacts/manifest.json"
)

// cloud-init destination file references.
const (
	cseHelpersScriptFilepath             = "/opt/azure/containers/provision_source.sh"
	cseHelpersScriptDistroFilepath       = "/opt/azure/containers/provision_source_distro.sh"
	cseInstallScriptFilepath             = "/opt/azure/containers/provision_installs.sh"
	cseInstallScriptDistroFilepath       = "/opt/azure/containers/provision_installs_distro.sh"
	cseConfigScriptFilepath              = "/opt/azure/containers/provision_configs.sh"
	customSearchDomainsCSEScriptFilepath = "/opt/azure/containers/setup-custom-search-domains.sh"
	dhcpV6ServiceCSEScriptFilepath       = "/etc/systemd/system/dhcpv6.service"
	dhcpV6ConfigCSEScriptFilepath        = "/opt/azure/containers/enable-dhcpv6.sh"
	initAKSCustomCloudFilepath           = "/opt/azure/containers/init-aks-custom-cloud.sh"
)

const (
	// AADPodIdentityAddonName is the name of the aad-pod-identity addon deployment.
	AADPodIdentityAddonName = "aad-pod-identity"
	// ACIConnectorAddonName is the name of the aci-connector addon deployment.
	ACIConnectorAddonName = "aci-connector"
)
