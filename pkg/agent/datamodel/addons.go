// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/helpers"
	"github.com/Azure/go-autorest/autorest/to"
	log "github.com/sirupsen/logrus"
)

func (cs *ContainerService) setAddonsConfig(isUpgrade bool) {
	o := cs.Properties.OrchestratorProfile
	clusterDNSPrefix := "aks-engine-cluster"
	if cs != nil && cs.Properties != nil && cs.Properties.MasterProfile != nil && cs.Properties.MasterProfile.DNSPrefix != "" {
		clusterDNSPrefix = cs.Properties.MasterProfile.DNSPrefix
	}
	cloudSpecConfig := cs.GetCloudSpecConfig()
	k8sComponents := api.K8sComponentsByVersionMap[o.OrchestratorVersion]
	specConfig := cloudSpecConfig.KubernetesSpecConfig
	omsagentImage := "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:ciprod01072020"
	var workspaceDomain string
	if cs.Properties.IsAzureStackCloud() {
		dependenciesLocation := string(cs.Properties.CustomCloudProfile.DependenciesLocation)
		workspaceDomain = helpers.GetLogAnalyticsWorkspaceDomain(dependenciesLocation)
		if strings.EqualFold(dependenciesLocation, "china") {
			omsagentImage = "dockerhub.azk8s.cn/microsoft/oms:ciprod01072020"
		}
	} else {
		workspaceDomain = helpers.GetLogAnalyticsWorkspaceDomain(cloudSpecConfig.CloudName)
		if strings.EqualFold(cloudSpecConfig.CloudName, "AzureChinaCloud") {
			omsagentImage = "dockerhub.azk8s.cn/microsoft/oms:ciprod01072020"
		}
	}
	workspaceDomain = base64.StdEncoding.EncodeToString([]byte(workspaceDomain))
	defaultsHeapsterAddonsConfig := api.KubernetesAddon{
		Name:    common.HeapsterAddonName,
		Enabled: to.BoolPtr(api.DefaultHeapsterAddonEnabled),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.HeapsterAddonName,
				Image:          specConfig.KubernetesImageBase + k8sComponents["heapster"],
				CPURequests:    "88m",
				MemoryRequests: "204Mi",
				CPULimits:      "88m",
				MemoryLimits:   "204Mi",
			},
			{
				Name:           "heapster-nanny",
				Image:          specConfig.KubernetesImageBase + k8sComponents["addonresizer"],
				CPURequests:    "88m",
				MemoryRequests: "204Mi",
				CPULimits:      "88m",
				MemoryLimits:   "204Mi",
			},
		},
	}

	defaultTillerAddonsConfig := api.KubernetesAddon{
		Name:    common.TillerAddonName,
		Enabled: to.BoolPtr(api.DefaultTillerAddonEnabled),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.TillerAddonName,
				CPURequests:    "50m",
				MemoryRequests: "150Mi",
				CPULimits:      "50m",
				MemoryLimits:   "150Mi",
				Image:          specConfig.TillerImageBase + k8sComponents[common.TillerAddonName],
			},
		},
		Config: map[string]string{
			"max-history": strconv.Itoa(api.DefaultTillerMaxHistory),
		},
	}

	defaultACIConnectorAddonsConfig := api.KubernetesAddon{
		Name:    common.ACIConnectorAddonName,
		Enabled: to.BoolPtr(api.DefaultACIConnectorAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Config: map[string]string{
			"region":   "westus",
			"nodeName": "aci-connector",
			"os":       "Linux",
			"taint":    "azure.com/aci",
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.ACIConnectorAddonName,
				CPURequests:    "50m",
				MemoryRequests: "150Mi",
				CPULimits:      "50m",
				MemoryLimits:   "150Mi",
				Image:          specConfig.ACIConnectorImageBase + k8sComponents[common.ACIConnectorAddonName],
			},
		},
	}

	defaultClusterAutoscalerAddonsConfig := api.KubernetesAddon{
		Name:    common.ClusterAutoscalerAddonName,
		Enabled: to.BoolPtr(api.DefaultClusterAutoscalerAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Mode:    api.AddonModeEnsureExists,
		Config: map[string]string{
			"scan-interval":                         "1m",
			"expendable-pods-priority-cutoff":       "-10",
			"ignore-daemonsets-utilization":         "false",
			"ignore-mirror-pods-utilization":        "false",
			"max-autoprovisioned-node-group-count":  "15",
			"max-empty-bulk-delete":                 "10",
			"max-failing-time":                      "15m0s",
			"max-graceful-termination-sec":          "600",
			"max-inactivity":                        "10m0s",
			"max-node-provision-time":               "15m0s",
			"max-nodes-total":                       "0",
			"max-total-unready-percentage":          "45",
			"memory-total":                          "0:6400000",
			"min-replica-count":                     "0",
			"new-pod-scale-up-delay":                "0s",
			"node-autoprovisioning-enabled":         "false",
			"ok-total-unready-count":                "3",
			"scale-down-candidates-pool-min-count":  "50",
			"scale-down-candidates-pool-ratio":      "0.1",
			"scale-down-delay-after-add":            "10m0s",
			"scale-down-delay-after-delete":         "1m",
			"scale-down-delay-after-failure":        "3m0s",
			"scale-down-enabled":                    "true",
			"scale-down-non-empty-candidates-count": "30",
			"scale-down-unneeded-time":              "10m0s",
			"scale-down-unready-time":               "20m0s",
			"scale-down-utilization-threshold":      "0.5",
			"skip-nodes-with-local-storage":         "false",
			"skip-nodes-with-system-pods":           "true",
			"stderrthreshold":                       "2",
			"unremovable-node-recheck-timeout":      "5m0s",
			"v":                                     "3",
			"write-status-configmap":                "true",
			"balance-similar-node-groups":           "true",
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.ClusterAutoscalerAddonName,
				CPURequests:    "100m",
				MemoryRequests: "300Mi",
				CPULimits:      "100m",
				MemoryLimits:   "300Mi",
				Image:          specConfig.KubernetesImageBase + k8sComponents[common.ClusterAutoscalerAddonName],
			},
		},
		Pools: makeDefaultClusterAutoscalerAddonPoolsConfig(cs),
	}

	defaultBlobfuseFlexVolumeAddonsConfig := api.KubernetesAddon{
		Name:    common.BlobfuseFlexVolumeAddonName,
		Enabled: to.BoolPtr(api.DefaultBlobfuseFlexVolumeAddonEnabled && common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.8.0") && !cs.Properties.HasCoreOS() && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.BlobfuseFlexVolumeAddonName,
				CPURequests:    "50m",
				MemoryRequests: "100Mi",
				CPULimits:      "50m",
				MemoryLimits:   "100Mi",
				Image:          k8sComponents[common.BlobfuseFlexVolumeAddonName],
			},
		},
	}

	defaultSMBFlexVolumeAddonsConfig := api.KubernetesAddon{
		Name:    common.SMBFlexVolumeAddonName,
		Enabled: to.BoolPtr(api.DefaultSMBFlexVolumeAddonEnabled && common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.8.0") && !cs.Properties.HasCoreOS() && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.SMBFlexVolumeAddonName,
				CPURequests:    "50m",
				MemoryRequests: "100Mi",
				CPULimits:      "50m",
				MemoryLimits:   "100Mi",
				Image:          k8sComponents[common.SMBFlexVolumeAddonName],
			},
		},
	}

	defaultKeyVaultFlexVolumeAddonsConfig := api.KubernetesAddon{
		Name:    common.KeyVaultFlexVolumeAddonName,
		Enabled: to.BoolPtr(api.DefaultKeyVaultFlexVolumeAddonEnabled && !cs.Properties.HasCoreOS() && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.KeyVaultFlexVolumeAddonName,
				CPURequests:    "50m",
				MemoryRequests: "100Mi",
				CPULimits:      "50m",
				MemoryLimits:   "100Mi",
				Image:          k8sComponents[common.KeyVaultFlexVolumeAddonName],
			},
		},
	}

	defaultDashboardAddonsConfig := api.KubernetesAddon{
		Name:    common.DashboardAddonName,
		Enabled: to.BoolPtr(api.DefaultDashboardAddonEnabled),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.DashboardAddonName,
				CPURequests:    "300m",
				MemoryRequests: "150Mi",
				CPULimits:      "300m",
				MemoryLimits:   "150Mi",
				Image:          specConfig.KubernetesImageBase + k8sComponents[common.DashboardAddonName],
			},
		},
	}

	defaultReschedulerAddonsConfig := api.KubernetesAddon{
		Name:    common.ReschedulerAddonName,
		Enabled: to.BoolPtr(api.DefaultReschedulerAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.ReschedulerAddonName,
				CPURequests:    "10m",
				MemoryRequests: "100Mi",
				CPULimits:      "10m",
				MemoryLimits:   "100Mi",
				Image:          specConfig.KubernetesImageBase + k8sComponents[common.ReschedulerAddonName],
			},
		},
	}

	defaultMetricsServerAddonsConfig := api.KubernetesAddon{
		Name:    common.MetricsServerAddonName,
		Enabled: to.BoolPtr(api.DefaultMetricsServerAddonEnabled),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.MetricsServerAddonName,
				Image: specConfig.KubernetesImageBase + k8sComponents[common.MetricsServerAddonName],
			},
		},
	}

	defaultNVIDIADevicePluginAddonsConfig := api.KubernetesAddon{
		Name:    common.NVIDIADevicePluginAddonName,
		Enabled: to.BoolPtr(cs.Properties.IsNvidiaDevicePluginCapable() && !cs.Properties.HasCoreOS() && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name: common.NVIDIADevicePluginAddonName,
				// from https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/device-plugins/nvidia-gpu/daemonset.yaml#L44
				CPURequests:    "50m",
				MemoryRequests: "100Mi",
				CPULimits:      "50m",
				MemoryLimits:   "100Mi",
				Image:          specConfig.NVIDIAImageBase + k8sComponents[common.NVIDIADevicePluginAddonName],
			},
		},
	}

	defaultContainerMonitoringAddonsConfig := api.KubernetesAddon{
		Name:    common.ContainerMonitoringAddonName,
		Enabled: to.BoolPtr(api.DefaultContainerMonitoringAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Config: map[string]string{
			"omsAgentVersion":       "1.10.0.1",
			"dockerProviderVersion": "8.0.0-2",
			"schema-versions":       "v1",
			"clusterName":           clusterDNSPrefix,
			"workspaceDomain":       workspaceDomain,
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           "omsagent",
				CPURequests:    "150m",
				MemoryRequests: "250Mi",
				CPULimits:      "1",
				MemoryLimits:   "750Mi",
				Image:          omsagentImage,
			},
		},
	}

	defaultIPMasqAgentAddonsConfig := api.KubernetesAddon{
		Name: common.IPMASQAgentAddonName,
		Enabled: to.BoolPtr(api.DefaultIPMasqAgentAddonEnabled &&
			(o.KubernetesConfig.NetworkPlugin != api.NetworkPluginCilium &&
				o.KubernetesConfig.NetworkPlugin != api.NetworkPluginAntrea)),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.IPMASQAgentAddonName,
				CPURequests:    "50m",
				MemoryRequests: "50Mi",
				CPULimits:      "50m",
				MemoryLimits:   "250Mi",
				Image:          specConfig.KubernetesImageBase + k8sComponents[common.IPMASQAgentAddonName],
			},
		},
		Config: map[string]string{
			"non-masquerade-cidr":           cs.Properties.GetNonMasqueradeCIDR(),
			"non-masq-cni-cidr":             cs.Properties.GetAzureCNICidr(),
			"secondary-non-masquerade-cidr": cs.Properties.GetSecondaryNonMasqueradeCIDR(),
			"enable-ipv6": strconv.FormatBool(cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6DualStack") ||
				cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6Only")),
		},
	}

	defaultAzureCNINetworkMonitorAddonsConfig := api.KubernetesAddon{
		Name:    common.AzureCNINetworkMonitorAddonName,
		Enabled: to.BoolPtr(o.IsAzureCNI() && o.KubernetesConfig.NetworkPolicy != api.NetworkPolicyCalico),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.AzureCNINetworkMonitorAddonName,
				Image: specConfig.AzureCNIImageBase + k8sComponents[common.AzureCNINetworkMonitorAddonName],
			},
		},
	}

	defaultAzureNetworkPolicyAddonsConfig := api.KubernetesAddon{
		Name:    common.AzureNetworkPolicyAddonName,
		Enabled: to.BoolPtr(o.KubernetesConfig.NetworkPlugin == api.NetworkPluginAzure && o.KubernetesConfig.NetworkPolicy == api.NetworkPolicyAzure),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.AzureNetworkPolicyAddonName,
				Image:          k8sComponents[common.AzureNetworkPolicyAddonName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "100m",
				MemoryLimits:   "200Mi",
			},
		},
	}

	defaultCloudNodeManagerAddonsConfig := api.KubernetesAddon{
		Name:    common.CloudNodeManagerAddonName,
		Enabled: to.BoolPtr(common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.16.0") && to.Bool(o.KubernetesConfig.UseCloudControllerManager)),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.CloudNodeManagerAddonName,
				Image: specConfig.MCRKubernetesImageBase + k8sComponents[common.CloudNodeManagerAddonName],
			},
		},
	}

	defaultDNSAutoScalerAddonsConfig := api.KubernetesAddon{
		Name: common.DNSAutoscalerAddonName,
		// TODO enable this when it has been smoke tested
		Enabled: to.BoolPtr(api.DefaultDNSAutoscalerAddonEnabled),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.DNSAutoscalerAddonName,
				Image:          specConfig.KubernetesImageBase + k8sComponents[common.DNSAutoscalerAddonName],
				CPURequests:    "20m",
				MemoryRequests: "100Mi",
			},
		},
	}

	defaultsCalicoDaemonSetAddonsConfig := api.KubernetesAddon{
		Name:    common.CalicoAddonName,
		Enabled: to.BoolPtr(o.KubernetesConfig.NetworkPolicy == api.NetworkPolicyCalico),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  "calico-typha",
				Image: specConfig.CalicoImageBase + k8sComponents["calico-typha"],
			},
			{
				Name:  "calico-cni",
				Image: specConfig.CalicoImageBase + k8sComponents["calico-cni"],
			},
			{
				Name:  "calico-node",
				Image: specConfig.CalicoImageBase + k8sComponents["calico-node"],
			},
			{
				Name:  "calico-pod2daemon",
				Image: specConfig.CalicoImageBase + k8sComponents["calico-pod2daemon"],
			},
			{
				Name:  "calico-cluster-proportional-autoscaler",
				Image: specConfig.KubernetesImageBase + k8sComponents["calico-cluster-proportional-autoscaler"],
			},
		},
	}

	defaultsCiliumAddonsConfig := api.KubernetesAddon{
		Name:    common.CiliumAddonName,
		Enabled: to.BoolPtr(o.KubernetesConfig.NetworkPolicy == api.NetworkPolicyCilium),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.CiliumAgentContainerName,
				Image: k8sComponents[common.CiliumAgentContainerName],
			},
			{
				Name:  common.CiliumCleanStateContainerName,
				Image: k8sComponents[common.CiliumCleanStateContainerName],
			},
			{
				Name:  common.CiliumOperatorContainerName,
				Image: k8sComponents[common.CiliumOperatorContainerName],
			},
			{
				Name:  common.CiliumEtcdOperatorContainerName,
				Image: k8sComponents[common.CiliumEtcdOperatorContainerName],
			},
		},
	}

	defaultsAntreaDaemonSetAddonsConfig := api.KubernetesAddon{
		Name:    common.AntreaAddonName,
		Enabled: to.BoolPtr(o.KubernetesConfig.NetworkPlugin == api.NetworkPluginAntrea),
		Config: map[string]string{
			"serviceCidr": o.KubernetesConfig.ServiceCIDR,
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.AntreaControllerContainerName,
				Image: k8sComponents[common.AntreaControllerContainerName],
			},
			{
				Name:  common.AntreaAgentContainerName,
				Image: k8sComponents[common.AntreaAgentContainerName],
			},
			{
				Name:  common.AntreaOVSContainerName,
				Image: k8sComponents[common.AntreaOVSContainerName],
			},
			{
				Name:  common.AntreaInstallCNIContainerName,
				Image: k8sComponents["antrea"+common.AntreaInstallCNIContainerName],
			},
		},
	}

	defaultsAADPodIdentityAddonsConfig := api.KubernetesAddon{
		Name:    common.AADPodIdentityAddonName,
		Enabled: to.BoolPtr(api.DefaultAADPodIdentityAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.NMIContainerName,
				Image:          k8sComponents[common.NMIContainerName],
				CPURequests:    "100m",
				MemoryRequests: "300Mi",
				CPULimits:      "100m",
				MemoryLimits:   "300Mi",
			},
			{
				Name:           common.MICContainerName,
				Image:          k8sComponents[common.MICContainerName],
				CPURequests:    "100m",
				MemoryRequests: "300Mi",
				CPULimits:      "100m",
				MemoryLimits:   "300Mi",
			},
		},
	}

	defaultsAzurePolicyAddonsConfig := api.KubernetesAddon{
		Name:    common.AzurePolicyAddonName,
		Enabled: to.BoolPtr(api.DefaultAzurePolicyAddonEnabled && !cs.Properties.IsAzureStackCloud()),
		Config: map[string]string{
			"auditInterval":             "30",
			"constraintViolationsLimit": "20",
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.AzurePolicyAddonName,
				Image:          k8sComponents[common.AzurePolicyAddonName],
				CPURequests:    "30m",
				MemoryRequests: "50Mi",
				CPULimits:      "100m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.GatekeeperContainerName,
				Image:          k8sComponents[common.GatekeeperContainerName],
				CPURequests:    "100m",
				MemoryRequests: "256Mi",
				CPULimits:      "100m",
				MemoryLimits:   "512Mi",
			},
		},
	}

	defaultNodeProblemDetectorConfig := api.KubernetesAddon{
		Name:    common.NodeProblemDetectorAddonName,
		Enabled: to.BoolPtr(api.DefaultNodeProblemDetectorAddonEnabled),
		Config: map[string]string{
			"customPluginMonitor": "/config/kernel-monitor-counter.json,/config/systemd-monitor-counter.json",
			"systemLogMonitor":    "/config/kernel-monitor.json,/config/docker-monitor.json,/config/systemd-monitor.json",
			"systemStatsMonitor":  "/config/system-stats-monitor.json",
			"versionLabel":        "v0.8.0",
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.NodeProblemDetectorAddonName,
				Image:          k8sComponents[common.NodeProblemDetectorAddonName],
				CPURequests:    "20m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "100Mi",
			},
		},
	}

	defaultAppGwAddonsConfig := api.KubernetesAddon{
		Name:    common.AppGwIngressAddonName,
		Enabled: to.BoolPtr(api.DefaultAppGwIngressAddonEnabled),
		Config: map[string]string{
			"appgw-subnet":     "",
			"appgw-sku":        "WAF_v2",
			"appgw-private-ip": "",
		},
	}

	defaultAzureDiskCSIDriverAddonsConfig := api.KubernetesAddon{
		Name:    common.AzureDiskCSIDriverAddonName,
		Enabled: to.BoolPtr(api.DefaultAzureDiskCSIDriverAddonEnabled && to.Bool(o.KubernetesConfig.UseCloudControllerManager)),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.CSIProvisionerContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIProvisionerContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIAttacherContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIAttacherContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIClusterDriverRegistrarContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIClusterDriverRegistrarContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSILivenessProbeContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSILivenessProbeContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSINodeDriverRegistrarContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSINodeDriverRegistrarContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSISnapshotterContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSISnapshotterContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIResizerContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIResizerContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIAzureDiskContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIAzureDiskContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
		},
	}

	defaultAzureFileCSIDriverAddonsConfig := api.KubernetesAddon{
		Name:    common.AzureFileCSIDriverAddonName,
		Enabled: to.BoolPtr(api.DefaultAzureFileCSIDriverAddonEnabled && to.Bool(o.KubernetesConfig.UseCloudControllerManager)),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           common.CSIProvisionerContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIProvisionerContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIAttacherContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIAttacherContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIClusterDriverRegistrarContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIClusterDriverRegistrarContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSILivenessProbeContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSILivenessProbeContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSINodeDriverRegistrarContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSINodeDriverRegistrarContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
			{
				Name:           common.CSIAzureFileContainerName,
				Image:          specConfig.MCRKubernetesImageBase + k8sComponents[common.CSIAzureFileContainerName],
				CPURequests:    "10m",
				MemoryRequests: "20Mi",
				CPULimits:      "200m",
				MemoryLimits:   "200Mi",
			},
		},
	}

	defaultKubeDNSAddonsConfig := api.KubernetesAddon{
		Name:    common.KubeDNSAddonName,
		Enabled: to.BoolPtr(api.DefaultKubeDNSAddonEnabled),
		Config: map[string]string{
			"domain":    o.KubernetesConfig.KubeletConfig["--cluster-domain"],
			"clusterIP": o.KubernetesConfig.DNSServiceIP,
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  "kubedns",
				Image: specConfig.KubernetesImageBase + k8sComponents["kube-dns"],
			},
			{
				Name:  "dnsmasq",
				Image: specConfig.KubernetesImageBase + k8sComponents["dnsmasq"],
			},
			{
				Name:  "sidecar",
				Image: specConfig.KubernetesImageBase + k8sComponents["k8s-dns-sidecar"],
			},
		},
	}

	defaultCorednsAddonsConfig := api.KubernetesAddon{
		Name:    common.CoreDNSAddonName,
		Enabled: to.BoolPtr(api.DefaultCoreDNSAddonEnabled),
		Config: map[string]string{
			"domain":    o.KubernetesConfig.KubeletConfig["--cluster-domain"],
			"clusterIP": o.KubernetesConfig.DNSServiceIP,
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.CoreDNSAddonName,
				Image: specConfig.KubernetesImageBase + k8sComponents[common.CoreDNSAddonName],
			},
		},
	}

	// set host network to true for single stack IPv6 as the the nameserver is currently
	// IPv4 only. By setting it to host network, we can leverage the host routes to successfully
	// resolve dns.
	if cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6Only") {
		defaultCorednsAddonsConfig.Config["use-host-network"] = "true"
	}

	// If we have any explicit coredns or kube-dns configuration in the addons array
	if getAddonsIndexByName(o.KubernetesConfig.Addons, common.KubeDNSAddonName) != -1 || getAddonsIndexByName(o.KubernetesConfig.Addons, common.CoreDNSAddonName) != -1 {
		// Ensure we don't we don't prepare an addons spec w/ both kube-dns and coredns enabled
		if o.KubernetesConfig.IsAddonEnabled(common.KubeDNSAddonName) {
			defaultCorednsAddonsConfig.Enabled = to.BoolPtr(false)
		}
	}

	defaultKubeProxyAddonsConfig := api.KubernetesAddon{
		Name:    common.KubeProxyAddonName,
		Enabled: to.BoolPtr(api.DefaultKubeProxyAddonEnabled),
		Config: map[string]string{
			"cluster-cidr": o.KubernetesConfig.ClusterSubnet,
			"proxy-mode":   string(o.KubernetesConfig.ProxyMode),
			"featureGates": cs.Properties.GetKubeProxyFeatureGates(),
		},
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.KubeProxyAddonName,
				Image: specConfig.KubernetesImageBase + k8sComponents[common.KubeProxyAddonName],
			},
		},
	}

	// set bind address, healthz and metric bind address to :: explicitly for
	// single stack IPv6 cluster as it is single stack IPv6 on dual stack host
	if cs.Properties.FeatureFlags.IsFeatureEnabled("EnableIPv6Only") {
		defaultKubeProxyAddonsConfig.Config["bind-address"] = "::"
		defaultKubeProxyAddonsConfig.Config["healthz-bind-address"] = "::"
		defaultKubeProxyAddonsConfig.Config["metrics-bind-address"] = "::1"
	}

	defaultPodSecurityPolicyAddonsConfig := api.KubernetesAddon{
		Name:    common.PodSecurityPolicyAddonName,
		Enabled: to.BoolPtr(common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.15.0") || to.Bool(o.KubernetesConfig.EnablePodSecurityPolicy)),
	}

	defaultAuditPolicyAddonsConfig := api.KubernetesAddon{
		Name:    common.AuditPolicyAddonName,
		Enabled: to.BoolPtr(true),
	}

	defaultAzureCloudProviderAddonsConfig := api.KubernetesAddon{
		Name:    common.AzureCloudProviderAddonName,
		Enabled: to.BoolPtr(true),
	}

	defaultAADDefaultAdminGroupAddonsConfig := api.KubernetesAddon{
		Name:    common.AADAdminGroupAddonName,
		Enabled: to.BoolPtr(cs.Properties.HasAADAdminGroupID()),
		Config: map[string]string{
			"adminGroupID": cs.Properties.GetAADAdminGroupID(),
		},
	}

	defaultFlannelAddonsConfig := api.KubernetesAddon{
		Name:    common.FlannelAddonName,
		Enabled: to.BoolPtr(o.KubernetesConfig.NetworkPlugin == api.NetworkPluginFlannel),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.KubeFlannelContainerName,
				Image: k8sComponents[common.KubeFlannelContainerName],
			},
			{
				Name:  common.FlannelInstallCNIContainerName,
				Image: k8sComponents["flannel"+common.FlannelInstallCNIContainerName],
			},
		},
	}

	defaultScheduledMaintenanceAddonsConfig := api.KubernetesAddon{
		Name:    common.ScheduledMaintenanceAddonName,
		Enabled: to.BoolPtr(false),
		Containers: []api.KubernetesContainerSpec{
			{
				Name:  common.KubeRBACProxyContainerName,
				Image: k8sComponents[common.KubeRBACProxyContainerName],
			},
			{
				Name:  common.ScheduledMaintenanceManagerContainerName,
				Image: k8sComponents[common.ScheduledMaintenanceManagerContainerName],
			},
		},
	}

	// Allow folks to simply enable kube-dns at cluster creation time without also requiring that coredns be explicitly disabled
	if !isUpgrade && o.KubernetesConfig.IsAddonEnabled(common.KubeDNSAddonName) {
		defaultCorednsAddonsConfig.Enabled = to.BoolPtr(false)
	}

	defaultAddons := []api.KubernetesAddon{
		defaultsHeapsterAddonsConfig,
		defaultTillerAddonsConfig,
		defaultACIConnectorAddonsConfig,
		defaultClusterAutoscalerAddonsConfig,
		defaultBlobfuseFlexVolumeAddonsConfig,
		defaultSMBFlexVolumeAddonsConfig,
		defaultKeyVaultFlexVolumeAddonsConfig,
		defaultDashboardAddonsConfig,
		defaultReschedulerAddonsConfig,
		defaultMetricsServerAddonsConfig,
		defaultNVIDIADevicePluginAddonsConfig,
		defaultContainerMonitoringAddonsConfig,
		defaultAzureCNINetworkMonitorAddonsConfig,
		defaultAzureNetworkPolicyAddonsConfig,
		defaultCloudNodeManagerAddonsConfig,
		defaultIPMasqAgentAddonsConfig,
		defaultDNSAutoScalerAddonsConfig,
		defaultsCalicoDaemonSetAddonsConfig,
		defaultsCiliumAddonsConfig,
		defaultsAADPodIdentityAddonsConfig,
		defaultAppGwAddonsConfig,
		defaultAzureDiskCSIDriverAddonsConfig,
		defaultAzureFileCSIDriverAddonsConfig,
		defaultsAzurePolicyAddonsConfig,
		defaultNodeProblemDetectorConfig,
		defaultKubeDNSAddonsConfig,
		defaultCorednsAddonsConfig,
		defaultKubeProxyAddonsConfig,
		defaultPodSecurityPolicyAddonsConfig,
		defaultAuditPolicyAddonsConfig,
		defaultAzureCloudProviderAddonsConfig,
		defaultAADDefaultAdminGroupAddonsConfig,
		defaultsAntreaDaemonSetAddonsConfig,
		defaultFlannelAddonsConfig,
		defaultScheduledMaintenanceAddonsConfig,
	}
	// Add default addons specification, if no user-provided spec exists
	if o.KubernetesConfig.Addons == nil {
		o.KubernetesConfig.Addons = defaultAddons
	} else {
		for _, addon := range defaultAddons {
			o.KubernetesConfig.Addons = appendAddonIfNotPresent(o.KubernetesConfig.Addons, addon)
		}
	}

	// Ensure cloud-node-manager and CSI components are enabled on appropriate upgrades
	if isUpgrade && to.Bool(o.KubernetesConfig.UseCloudControllerManager) &&
		common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.16.0") {
		componentry := map[string]api.KubernetesAddon{
			common.AzureDiskCSIDriverAddonName: defaultAzureDiskCSIDriverAddonsConfig,
			common.AzureFileCSIDriverAddonName: defaultAzureFileCSIDriverAddonsConfig,
			common.CloudNodeManagerAddonName:   defaultCloudNodeManagerAddonsConfig,
		}
		for name, config := range componentry {
			if i := getAddonsIndexByName(o.KubernetesConfig.Addons, name); i > -1 {
				if !to.Bool(o.KubernetesConfig.Addons[i].Enabled) {
					o.KubernetesConfig.Addons[i] = config
				}
			}
		}
	}

	// Back-compat for older addon specs of cluster-autoscaler
	if isUpgrade {
		i := getAddonsIndexByName(o.KubernetesConfig.Addons, common.ClusterAutoscalerAddonName)
		if i > -1 && to.Bool(o.KubernetesConfig.Addons[i].Enabled) {
			if o.KubernetesConfig.Addons[i].Pools == nil {
				log.Warnf("This cluster upgrade operation will enable the per-pool cluster-autoscaler addon.\n")
				var pools []api.AddonNodePoolsConfig
				for i, p := range cs.Properties.AgentPoolProfiles {
					pool := api.AddonNodePoolsConfig{
						Name: p.Name,
						Config: map[string]string{
							"min-nodes": strconv.Itoa(p.Count),
							"max-nodes": strconv.Itoa(p.Count),
						},
					}
					if i == 0 {
						originalMinNodes := o.KubernetesConfig.Addons[i].Config["min-nodes"]
						originalMaxNodes := o.KubernetesConfig.Addons[i].Config["max-nodes"]
						if originalMinNodes != "" {
							pool.Config["min-nodes"] = originalMinNodes
							delete(o.KubernetesConfig.Addons[i].Config, "min-nodes")
						}
						if originalMaxNodes != "" {
							pool.Config["max-nodes"] = originalMaxNodes
							delete(o.KubernetesConfig.Addons[i].Config, "max-nodes")
						}
					}
					log.Warnf("cluster-autoscaler will configure pool \"%s\" with min-nodes=%s, and max-nodes=%s.\n", pool.Name, pool.Config["min-nodes"], pool.Config["max-nodes"])
					pools = append(pools, pool)
				}
				o.KubernetesConfig.Addons[i].Pools = pools
				log.Warnf("You may modify the pool configurations via `kubectl edit deployment cluster-autoscaler -n kube-system`.\n")
				log.Warnf("Look for the `--nodes=` configuration flags (see below) in the deployment spec:\n")
				log.Warnf("\n%s", GetClusterAutoscalerNodesConfig(o.KubernetesConfig.Addons[i], cs))
			}
		}
	}

	// Back-compat for pre-1.12 clusters built before kube-dns and coredns were converted to user-configurable addons
	// Migrate to coredns unless coredns is explicitly set to false
	if isUpgrade && common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.12.0") {
		// If we don't have coredns in our addons array at all, this means we're in a legacy scenario and we want to migrate from kube-dns to coredns
		if i := getAddonsIndexByName(o.KubernetesConfig.Addons, common.CoreDNSAddonName); i == -1 {
			o.KubernetesConfig.Addons[i].Enabled = to.BoolPtr(true)
			// Ensure we don't we don't prepare an addons spec w/ both kube-dns and coredns enabled
			if j := getAddonsIndexByName(o.KubernetesConfig.Addons, common.KubeDNSAddonName); j > -1 {
				o.KubernetesConfig.Addons[j].Enabled = to.BoolPtr(false)
			}
		}
	}

	for _, addon := range defaultAddons {
		synthesizeAddonsConfig(o.KubernetesConfig.Addons, addon, isUpgrade)
	}

	if len(o.KubernetesConfig.PodSecurityPolicyConfig) > 0 && isUpgrade {
		if base64Data, ok := o.KubernetesConfig.PodSecurityPolicyConfig["data"]; ok {
			if i := getAddonsIndexByName(o.KubernetesConfig.Addons, common.PodSecurityPolicyAddonName); i > -1 {
				if o.KubernetesConfig.Addons[i].Data == "" {
					o.KubernetesConfig.Addons[i].Data = base64Data
				}
			}
		}
	}

	// Specific back-compat business logic for calico addon
	// Ensure addon is set to Enabled w/ proper containers config no matter what if NetworkPolicy == calico
	i := getAddonsIndexByName(o.KubernetesConfig.Addons, common.CalicoAddonName)
	if isUpgrade && o.KubernetesConfig.NetworkPolicy == api.NetworkPolicyCalico && i > -1 && o.KubernetesConfig.Addons[i].Enabled != to.BoolPtr(true) {
		j := getAddonsIndexByName(defaultAddons, common.CalicoAddonName)
		// Ensure calico is statically set to enabled
		o.KubernetesConfig.Addons[i].Enabled = to.BoolPtr(true)
		// Assume addon configuration was pruned due to an inherited enabled=false, so re-apply default values
		o.KubernetesConfig.Addons[i] = assignDefaultAddonVals(o.KubernetesConfig.Addons[i], defaultAddons[j], isUpgrade)
	}

	// Support back-compat configuration for Azure NetworkPolicy, which no longer ships with a "telemetry" container starting w/ 1.16.0
	if isUpgrade && o.KubernetesConfig.NetworkPolicy == api.NetworkPolicyAzure && common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.16.0") {
		i = getAddonsIndexByName(o.KubernetesConfig.Addons, common.AzureNetworkPolicyAddonName)
		var hasTelemetryContainerConfig bool
		var prunedContainersConfig []api.KubernetesContainerSpec
		if i > -1 {
			for _, c := range o.KubernetesConfig.Addons[i].Containers {
				if c.Name == common.AzureVnetTelemetryContainerName {
					hasTelemetryContainerConfig = true
				} else {
					prunedContainersConfig = append(prunedContainersConfig, c)
				}
			}
			if hasTelemetryContainerConfig {
				o.KubernetesConfig.Addons[i].Containers = prunedContainersConfig
			}
		}
	}

	// Specific back-compat business logic for deprecated "kube-proxy-daemonset" addon
	if i := getAddonsIndexByName(o.KubernetesConfig.Addons, "kube-proxy-daemonset"); i > -1 {
		if to.Bool(o.KubernetesConfig.Addons[i].Enabled) {
			if j := getAddonsIndexByName(o.KubernetesConfig.Addons, common.KubeProxyAddonName); j > -1 {
				// Copy data from deprecated addon spec to the current "kube-proxy" addon
				o.KubernetesConfig.Addons[j] = api.KubernetesAddon{
					Name:    common.KubeProxyAddonName,
					Enabled: to.BoolPtr(true),
					Data:    o.KubernetesConfig.Addons[i].Data,
				}
			}
		}
		// Remove deprecated "kube-proxy-daemonset addon"
		o.KubernetesConfig.Addons = append(o.KubernetesConfig.Addons[:i], o.KubernetesConfig.Addons[i+1:]...)
	}

	// Enable pod-security-policy addon during upgrade to 1.15 or greater scenarios, unless explicitly disabled
	if isUpgrade && common.IsKubernetesVersionGe(o.OrchestratorVersion, "1.15.0") && !o.KubernetesConfig.IsAddonDisabled(common.PodSecurityPolicyAddonName) {
		if i := getAddonsIndexByName(o.KubernetesConfig.Addons, common.PodSecurityPolicyAddonName); i > -1 {
			o.KubernetesConfig.Addons[i].Enabled = to.BoolPtr(true)
		}
	}
}

func appendAddonIfNotPresent(addons []api.KubernetesAddon, addon api.KubernetesAddon) []api.KubernetesAddon {
	i := getAddonsIndexByName(addons, addon.Name)
	if i < 0 {
		return append(addons, addon)
	}
	return addons
}

func getAddonsIndexByName(addons []api.KubernetesAddon, name string) int {
	for i := range addons {
		if addons[i].Name == name {
			return i
		}
	}
	return -1
}

// assignDefaultAddonVals will assign default values to addon from defaults, for each property in addon that has a zero value
func assignDefaultAddonVals(addon, defaults api.KubernetesAddon, isUpgrade bool) api.KubernetesAddon {
	if addon.Enabled == nil {
		addon.Enabled = defaults.Enabled
	}
	if !to.Bool(addon.Enabled) {
		return api.KubernetesAddon{
			Name:    addon.Name,
			Enabled: addon.Enabled,
		}
	}
	if addon.Data != "" {
		return api.KubernetesAddon{
			Name:    addon.Name,
			Enabled: addon.Enabled,
			Data:    addon.Data,
		}
	}
	if addon.Mode == "" {
		addon.Mode = defaults.Mode
	}
	for i := range defaults.Containers {
		c := addon.GetAddonContainersIndexByName(defaults.Containers[i].Name)
		if c < 0 {
			addon.Containers = append(addon.Containers, defaults.Containers[i])
		} else {
			if addon.Containers[c].Image == "" || isUpgrade {
				addon.Containers[c].Image = defaults.Containers[i].Image
			}
			if addon.Containers[c].CPURequests == "" {
				addon.Containers[c].CPURequests = defaults.Containers[i].CPURequests
			}
			if addon.Containers[c].MemoryRequests == "" {
				addon.Containers[c].MemoryRequests = defaults.Containers[i].MemoryRequests
			}
			if addon.Containers[c].CPULimits == "" {
				addon.Containers[c].CPULimits = defaults.Containers[i].CPULimits
			}
			if addon.Containers[c].MemoryLimits == "" {
				addon.Containers[c].MemoryLimits = defaults.Containers[i].MemoryLimits
			}
		}
	}
	// For pools-specific configuration, we only take the defaults if we have zero user-provided pools configuration
	if len(addon.Pools) == 0 {
		for i := range defaults.Pools {
			addon.Pools = append(addon.Pools, defaults.Pools[i])
		}
	}
	for key, val := range defaults.Config {
		if addon.Config == nil {
			addon.Config = make(map[string]string)
		}
		if v, ok := addon.Config[key]; !ok || v == "" {
			addon.Config[key] = val
		}
	}
	return addon
}

func synthesizeAddonsConfig(addons []api.KubernetesAddon, addon api.KubernetesAddon, isUpgrade bool) {
	i := getAddonsIndexByName(addons, addon.Name)
	if i >= 0 {
		addons[i] = assignDefaultAddonVals(addons[i], addon, isUpgrade)
	}
}

func makeDefaultClusterAutoscalerAddonPoolsConfig(cs *ContainerService) []api.AddonNodePoolsConfig {
	var ret []api.AddonNodePoolsConfig
	for _, pool := range cs.Properties.AgentPoolProfiles {
		ret = append(ret, api.AddonNodePoolsConfig{
			Name: pool.Name,
			Config: map[string]string{
				"min-nodes": strconv.Itoa(pool.Count),
				"max-nodes": strconv.Itoa(pool.Count),
			},
		})
	}
	return ret
}

// GetClusterAutoscalerNodesConfig returns the cluster-autoscaler runtime configuration flag for a nodepool
func GetClusterAutoscalerNodesConfig(addon api.KubernetesAddon, cs *ContainerService) string {
	var ret string
	for _, pool := range addon.Pools {
		nodepoolName := cs.Properties.GetAgentVMPrefix(cs.Properties.GetAgentPoolByName(pool.Name), cs.Properties.GetAgentPoolIndexByName(pool.Name))
		ret += fmt.Sprintf("        - --nodes=%s:%s:%s\n", pool.Config["min-nodes"], pool.Config["max-nodes"], nodepoolName)
	}
	if ret != "" {
		ret = strings.TrimRight(ret, "\n")
	}
	return ret
}
