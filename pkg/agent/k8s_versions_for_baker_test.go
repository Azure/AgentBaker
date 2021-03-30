// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	dashboardImageReference                           string = "kubernetes-dashboard-amd64:v1.10.1"
	execHealthZImageReference                         string = "exechealthz-amd64:1.2"
	heapsterImageReference                            string = "heapster-amd64:v1.5.4"
	coreDNSImageReference                             string = "coredns:1.6.6"
	kubeDNSImageReference                             string = "k8s-dns-kube-dns-amd64:1.15.4"
	kubeDNSMasqNannyImageReference                    string = "k8s-dns-dnsmasq-nanny-amd64:1.15.4"
	kubeDNSSidecarImageReference                      string = "k8s-dns-sidecar-amd64:1.14.10"
	pauseImageReference                               string = "oss/kubernetes/pause:1.3.1"
	tillerImageReference                              string = "tiller:v2.13.1"
	reschedulerImageReference                         string = "rescheduler:v0.4.0"
	virtualKubeletImageReference                      string = "virtual-kubelet:latest"
	omsImageReference                                 string = "oms:ciprod01072020"
	azureCNINetworkMonitorImageReference              string = "networkmonitor:v1.1.8"
	nvidiaDevicePluginImageReference                  string = "k8s-device-plugin:1.11"
	blobfuseFlexVolumeImageReference                  string = "mcr.microsoft.com/k8s/flexvolume/blobfuse-flexvolume:1.0.8"
	smbFlexVolumeImageReference                       string = "mcr.microsoft.com/k8s/flexvolume/smb-flexvolume:1.0.2"
	keyvaultFlexVolumeImageReference                  string = "mcr.microsoft.com/k8s/flexvolume/keyvault-flexvolume:v0.0.13"
	ipMasqAgentImageReference                         string = "ip-masq-agent-amd64:v2.5.0"
	dnsAutoscalerImageReference                       string = "cluster-proportional-autoscaler-amd64:1.1.1"
	calicoTyphaImageReference                         string = "typha:v3.8.0"
	calicoCNIImageReference                           string = "cni:v3.8.0"
	calicoNodeImageReference                          string = "node:v3.8.0"
	calicoPod2DaemonImageReference                    string = "pod2daemon-flexvol:v3.8.0"
	calicoClusterProportionalAutoscalerImageReference string = "cluster-proportional-autoscaler-amd64:1.1.2-r2"
	ciliumAgentImageReference                         string = "docker.io/cilium/cilium:v1.4"
	ciliumCleanStateImageReference                    string = "docker.io/cilium/cilium-init:2018-10-16"
	ciliumOperatorImageReference                      string = "docker.io/cilium/operator:v1.4"
	ciliumEtcdOperatorImageReference                  string = "docker.io/cilium/cilium-etcd-operator:v2.0.5"
	antreaControllerImageReference                    string = "antrea/antrea-ubuntu:v0.3.0"
	antreaAgentImageReference                                = antreaControllerImageReference
	antreaOVSImageReference                                  = antreaControllerImageReference
	antreaInstallCNIImageReference                           = antreaControllerImageReference
	azureNPMContainerImageReference                   string = "mcr.microsoft.com/containernetworking/azure-npm:v1.2.8"
	aadPodIdentityNMIImageReference                   string = "mcr.microsoft.com/k8s/aad-pod-identity/nmi:1.2"
	aadPodIdentityMICImageReference                   string = "mcr.microsoft.com/k8s/aad-pod-identity/mic:1.2"
	azurePolicyImageReference                         string = "mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20191011.1"
	gatekeeperImageReference                          string = "quay.io/open-policy-agent/gatekeeper:v3.0.4-beta.2"
	nodeProblemDetectorImageReference                 string = "k8s.gcr.io/node-problem-detector:v0.8.0"
	csiProvisionerImageReference                      string = "oss/kubernetes-csi/csi-provisioner:v1.4.0"
	csiAttacherImageReference                         string = "oss/kubernetes-csi/csi-attacher:v1.2.0"
	csiClusterDriverRegistrarImageReference           string = "oss/kubernetes-csi/csi-cluster-driver-registrar:v1.0.1"
	csiLivenessProbeImageReference                    string = "oss/kubernetes-csi/livenessprobe:v1.1.0"
	csiNodeDriverRegistrarImageReference              string = "oss/kubernetes-csi/csi-node-driver-registrar:v1.1.0"
	csiSnapshotterImageReference                      string = "oss/kubernetes-csi/csi-snapshotter:v1.1.0"
	csiResizerImageReference                          string = "oss/kubernetes-csi/csi-resizer:v0.3.0"
	csiAzureDiskImageReference                        string = "k8s/csi/azuredisk-csi:v0.5.0"
	csiAzureFileImageReference                        string = "k8s/csi/azurefile-csi:v0.3.0"
	azureCloudControllerManagerImageReference         string = "oss/kubernetes/azure-cloud-controller-manager:v0.4.1"
	azureCloudNodeManagerImageReference               string = "oss/kubernetes/azure-cloud-node-manager:v0.4.1"
	kubeFlannelImageReference                         string = "quay.io/coreos/flannel:v0.8.0-amd64"
	flannelInstallCNIImageReference                   string = "quay.io/coreos/flannel:v0.10.0-amd64"
	KubeRBACProxyImageReference                       string = "gcr.io/kubebuilder/kube-rbac-proxy:v0.4.0"
	ScheduledMaintenanceManagerImageReference         string = "quay.io/awesomenix/drainsafe-manager:latest"
	// DefaultKubernetesCloudProviderBackoffRetries is 6, takes effect if DefaultKubernetesCloudProviderBackoff is true
	DefaultKubernetesCloudProviderBackoffRetries = 6
	// DefaultKubernetesCloudProviderBackoffJitter is 1, takes effect if DefaultKubernetesCloudProviderBackoff is true
	DefaultKubernetesCloudProviderBackoffJitter = 1.0
	// DefaultKubernetesCloudProviderBackoffDuration is 5, takes effect if DefaultKubernetesCloudProviderBackoff is true
	DefaultKubernetesCloudProviderBackoffDuration = 5
	// DefaultKubernetesCloudProviderBackoffExponent is 1.5, takes effect if DefaultKubernetesCloudProviderBackoff is true
	DefaultKubernetesCloudProviderBackoffExponent = 1.5
	// DefaultKubernetesCloudProviderRateLimitQPS is 3, takes effect if DefaultKubernetesCloudProviderRateLimit is true
	DefaultKubernetesCloudProviderRateLimitQPS = 3.0
	// DefaultKubernetesCloudProviderRateLimitQPSWrite is 1, takes effect if DefaultKubernetesCloudProviderRateLimit is true
	DefaultKubernetesCloudProviderRateLimitQPSWrite = 1.0
	// DefaultKubernetesCloudProviderRateLimitBucket is 10, takes effect if DefaultKubernetesCloudProviderRateLimit is true
	DefaultKubernetesCloudProviderRateLimitBucket = 10
	// DefaultKubernetesCloudProviderRateLimitBucketWrite is 10, takes effect if DefaultKubernetesCloudProviderRateLimit is true
	DefaultKubernetesCloudProviderRateLimitBucketWrite = DefaultKubernetesCloudProviderRateLimitBucket
)

// k8sComponentVersions is a convenience map to make UT maintenance easier,
// at the expense of some add'l indirection in getK8sVersionComponents below
var k8sComponentVersions = map[string]map[string]string{
	"1.6": {
		"dashboard":      "kubernetes-dashboard-amd64:v1.6.3",
		"addon-resizer":  "addon-resizer:1.7",
		"heapster":       "heapster-amd64:v1.3.0",
		"metrics-server": "metrics-server-amd64:v0.2.1",
		"kube-dns":       "k8s-dns-kube-dns-amd64:1.14.5",
		"addon-manager":  "kube-addon-manager-amd64:v6.5",
		"dnsmasq":        "k8s-dns-dnsmasq-nanny-amd64:1.14.5",
		"rescheduler":    "rescheduler:v0.3.1",
	},
}

// K8sComponentsByVersionMap represents Docker images used for Kubernetes components based on Kubernetes versions (major.minor.patch)
var K8sComponentsByVersionMap map[string]map[string]string

func init() {
	K8sComponentsByVersionMap = getKubeConfigs()
}

func getKubeConfigs() map[string]map[string]string {
	ret := make(map[string]map[string]string)
	for _, version := range datamodel.GetAllSupportedKubernetesVersions(true, false) {
		ret[version] = getK8sVersionComponents(version, getVersionOverrides(version))
	}
	return ret
}

func getVersionOverrides(v string) map[string]string {
	switch v {
	case "1.19.1":
		return map[string]string{"windowszip": "v1.19.1-hotfix.20200923/windowszip/v1.19.1-hotfix.20200923-1int.zip"}
	case "1.18.4":
		return map[string]string{"windowszip": "v1.18.4-hotfix.20200624/windowszip/v1.18.4-hotfix.20200624-1int.zip"}
	case "1.18.2":
		return map[string]string{"windowszip": "v1.18.2-hotfix.20200624/windowszip/v1.18.2-hotfix.20200624-1int.zip"}
	case "1.17.9":
		return map[string]string{"windowszip": "v1.17.9-hotfix.20200714/windowszip/v1.17.9-hotfix.20200714-1int.zip"}
	case "1.17.7":
		return map[string]string{"windowszip": "v1.17.7-hotfix.20200714/windowszip/v1.17.7-hotfix.20200714-1int.zip"}
	case "1.16.13":
		return map[string]string{"windowszip": "v1.16.13-hotfix.20200714/windowszip/v1.16.13-hotfix.20200714-1int.zip"}
	case "1.16.11":
		return map[string]string{"windowszip": "v1.16.11-hotfix.20200617/windowszip/v1.16.11-hotfix.20200617-1int.zip"}
	case "1.16.10":
		return map[string]string{"windowszip": "v1.16.10-hotfix.20200714/windowszip/v1.16.10-hotfix.20200714-1int.zip"}
	case "1.15.12":
		return map[string]string{"windowszip": "v1.15.12-hotfix.20200714/windowszip/v1.15.12-hotfix.20200714-1int.zip"}
	case "1.15.11":
		return map[string]string{"windowszip": "v1.15.11-hotfix.20200714/windowszip/v1.15.11-hotfix.20200714-1int.zip"}
	case "1.8.11":
		return map[string]string{"kube-dns": "k8s-dns-kube-dns-amd64:1.14.9"}
	case "1.8.9":
		return map[string]string{"windowszip": "v1.8.9-2int.zip"}
	case "1.8.6":
		return map[string]string{"windowszip": "v1.8.6-2int.zip"}
	case "1.8.2":
		return map[string]string{"windowszip": "v1.8.2-2int.zip"}
	case "1.8.1":
		return map[string]string{"windowszip": "v1.8.1-2int.zip"}
	case "1.8.0":
		return map[string]string{"windowszip": "v1.8.0-2int.zip"}
	case "1.7.16":
		return map[string]string{"windowszip": "v1.7.16-1int.zip"}
	case "1.7.15":
		return map[string]string{"windowszip": "v1.7.15-1int.zip"}
	case "1.7.14":
		return map[string]string{"windowszip": "v1.7.14-1int.zip"}
	case "1.7.13":
		return map[string]string{"windowszip": "v1.7.13-1int.zip"}
	case "1.7.12":
		return map[string]string{"windowszip": "v1.7.12-2int.zip"}
	case "1.7.10":
		return map[string]string{"windowszip": "v1.7.10-1int.zip"}
	case "1.7.9":
		return map[string]string{"windowszip": "v1.7.9-2int.zip"}
	case "1.7.7":
		return map[string]string{"windowszip": "v1.7.7-2int.zip"}
	case "1.7.5":
		return map[string]string{"windowszip": "v1.7.5-4int.zip"}
	case "1.7.4":
		return map[string]string{"windowszip": "v1.7.4-2int.zip"}
	case "1.7.2":
		return map[string]string{"windowszip": "v1.7.2-1int.zip"}
	default:
		return nil
	}
}

func getK8sVersionComponents(version string, overrides map[string]string) map[string]string {
	s := strings.Split(version, ".")
	majorMinor := strings.Join(s[:2], ".")
	var ret map[string]string
	k8sComponent := k8sComponentVersions[majorMinor]
	switch majorMinor {
	case "1.19":
		ret = map[string]string{
			"kube-apiserver":                         "kube-apiserver:v" + version,
			"kube-controller-manager":                "kube-controller-manager:v" + version,
			"kube-proxy":                             "kube-proxy:v" + version,
			"kube-scheduler":                         "kube-scheduler:v" + version,
			"ccm":                                    azureCloudControllerManagerImageReference,
			"cloud-node-manager":                     azureCloudNodeManagerImageReference,
			"windowszip":                             "v" + version + "/windowszip/v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               heapsterImageReference,
			"metrics-server":                         k8sComponent["metrics-server"],
			"coredns":                                coreDNSImageReference,
			"kube-dns":                               kubeDNSImageReference,
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                kubeDNSMasqNannyImageReference,
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            reschedulerImageReference,
			"aci-connector":                          virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"cluster-autoscaler":                     k8sComponent["cluster-autoscaler"],
			"k8s-dns-sidecar":                        kubeDNSSidecarImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                          aadPodIdentityNMIImageReference,
			"mic":                          aadPodIdentityMICImageReference,
			"azure-policy":                 azurePolicyImageReference,
			"gatekeeper":                   gatekeeperImageReference,
			"node-problem-detector":        nodeProblemDetectorImageReference,
			"csi-provisioner":              csiProvisionerImageReference,
			"csi-attacher":                 csiAttacherImageReference,
			"csi-cluster-driver-registrar": csiClusterDriverRegistrarImageReference,
			"livenessprobe":                csiLivenessProbeImageReference,
			"csi-node-driver-registrar":    csiNodeDriverRegistrarImageReference,
			"csi-snapshotter":              csiSnapshotterImageReference,
			"csi-resizer":                  csiResizerImageReference,
			"azuredisk-csi":                csiAzureDiskImageReference,
			"azurefile-csi":                csiAzureFileImageReference,
			"kube-flannel":                 kubeFlannelImageReference,
			"flannel" + "install-cni":      flannelInstallCNIImageReference,
			"kube-rbac-proxy":              KubeRBACProxyImageReference,
			"manager":                      ScheduledMaintenanceManagerImageReference,
			"backoffretries":               strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":                strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":              strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":              strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":                 strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":              strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
			"nvidia-device-plugin":         nvidiaDevicePluginImageReference,
		}
	case "1.18":
		ret = map[string]string{
			"kube-apiserver":                         "kube-apiserver:v" + version,
			"kube-controller-manager":                "kube-controller-manager:v" + version,
			"kube-proxy":                             "kube-proxy:v" + version,
			"kube-scheduler":                         "kube-scheduler:v" + version,
			"ccm":                                    azureCloudControllerManagerImageReference,
			"cloud-node-manager":                     azureCloudNodeManagerImageReference,
			"windowszip":                             "v" + version + "/windowszip/v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               heapsterImageReference,
			"metrics-server":                         k8sComponent["metrics-server"],
			"coredns":                                coreDNSImageReference,
			"kube-dns":                               kubeDNSImageReference,
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                kubeDNSMasqNannyImageReference,
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            reschedulerImageReference,
			"aci-connector":                          virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"cluster-autoscaler":                     k8sComponent["cluster-autoscaler"],
			"k8s-dns-sidecar":                        kubeDNSSidecarImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                          aadPodIdentityNMIImageReference,
			"mic":                          aadPodIdentityMICImageReference,
			"azure-policy":                 azurePolicyImageReference,
			"gatekeeper":                   gatekeeperImageReference,
			"node-problem-detector":        nodeProblemDetectorImageReference,
			"csi-provisioner":              csiProvisionerImageReference,
			"csi-attacher":                 csiAttacherImageReference,
			"csi-cluster-driver-registrar": csiClusterDriverRegistrarImageReference,
			"livenessprobe":                csiLivenessProbeImageReference,
			"csi-node-driver-registrar":    csiNodeDriverRegistrarImageReference,
			"csi-snapshotter":              csiSnapshotterImageReference,
			"csi-resizer":                  csiResizerImageReference,
			"azuredisk-csi":                csiAzureDiskImageReference,
			"azurefile-csi":                csiAzureFileImageReference,
			"kube-flannel":                 kubeFlannelImageReference,
			"flannel" + "install-cni":      flannelInstallCNIImageReference,
			"kube-rbac-proxy":              KubeRBACProxyImageReference,
			"manager":                      ScheduledMaintenanceManagerImageReference,
			"backoffretries":               strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":                strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":              strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":              strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":                 strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":              strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
			"nvidia-device-plugin":         nvidiaDevicePluginImageReference,
		}
	case "1.17":
		ret = map[string]string{
			"kube-apiserver":                         "kube-apiserver:v" + version,
			"kube-controller-manager":                "kube-controller-manager:v" + version,
			"kube-proxy":                             "kube-proxy:v" + version,
			"kube-scheduler":                         "kube-scheduler:v" + version,
			"ccm":                                    azureCloudControllerManagerImageReference,
			"cloud-node-manager":                     azureCloudNodeManagerImageReference,
			"windowszip":                             "v" + version + "/windowszip/v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               heapsterImageReference,
			"metrics-server":                         k8sComponent["metrics-server"],
			"coredns":                                coreDNSImageReference,
			"kube-dns":                               kubeDNSImageReference,
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                kubeDNSMasqNannyImageReference,
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            reschedulerImageReference,
			ACIConnectorAddonName:                    virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"cluster-autoscaler":                     k8sComponent["cluster-autoscaler"],
			"k8s-dns-sidecar":                        kubeDNSSidecarImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                          aadPodIdentityNMIImageReference,
			"mic":                          aadPodIdentityMICImageReference,
			"azure-policy":                 azurePolicyImageReference,
			"gatekeeper":                   gatekeeperImageReference,
			"node-problem-detector":        nodeProblemDetectorImageReference,
			"csi-provisioner":              csiProvisionerImageReference,
			"csi-attacher":                 csiAttacherImageReference,
			"csi-cluster-driver-registrar": csiClusterDriverRegistrarImageReference,
			"livenessprobe":                csiLivenessProbeImageReference,
			"csi-node-driver-registrar":    csiNodeDriverRegistrarImageReference,
			"csi-snapshotter":              csiSnapshotterImageReference,
			"csi-resizer":                  csiResizerImageReference,
			"azuredisk-csi":                csiAzureDiskImageReference,
			"azurefile-csi":                csiAzureFileImageReference,
			"kube-flannel":                 kubeFlannelImageReference,
			"flannel" + "install-cni":      flannelInstallCNIImageReference,
			"kube-rbac-proxy":              KubeRBACProxyImageReference,
			"manager":                      ScheduledMaintenanceManagerImageReference,
			"backoffretries":               strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":                strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":              strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":              strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":                 strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":              strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
			"nvidia-device-plugin":         nvidiaDevicePluginImageReference,
		}
	case "1.16":
		ret = map[string]string{
			"hyperkube":                              "hyperkube-amd64:v" + version,
			"kube-proxy":                             "hyperkube-amd64:v" + version,
			"ccm":                                    azureCloudControllerManagerImageReference,
			"cloud-node-manager":                     azureCloudNodeManagerImageReference,
			"windowszip":                             "v" + version + "/windowszip/v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               heapsterImageReference,
			"metrics-server":                         k8sComponent["metrics-server"],
			"coredns":                                coreDNSImageReference,
			"kube-dns":                               kubeDNSImageReference,
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                kubeDNSMasqNannyImageReference,
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            reschedulerImageReference,
			ACIConnectorAddonName:                    virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"cluster-autoscaler":                     k8sComponent["cluster-autoscaler"],
			"k8s-dns-sidecar":                        kubeDNSSidecarImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                          aadPodIdentityNMIImageReference,
			"mic":                          aadPodIdentityMICImageReference,
			"azure-policy":                 azurePolicyImageReference,
			"gatekeeper":                   gatekeeperImageReference,
			"node-problem-detector":        nodeProblemDetectorImageReference,
			"csi-provisioner":              csiProvisionerImageReference,
			"csi-attacher":                 csiAttacherImageReference,
			"csi-cluster-driver-registrar": csiClusterDriverRegistrarImageReference,
			"livenessprobe":                csiLivenessProbeImageReference,
			"csi-node-driver-registrar":    csiNodeDriverRegistrarImageReference,
			"csi-snapshotter":              csiSnapshotterImageReference,
			"csi-resizer":                  csiResizerImageReference,
			"azuredisk-csi":                csiAzureDiskImageReference,
			"azurefile-csi":                csiAzureFileImageReference,
			"kube-flannel":                 kubeFlannelImageReference,
			"flannel" + "install-cni":      flannelInstallCNIImageReference,
			"kube-rbac-proxy":              KubeRBACProxyImageReference,
			"manager":                      ScheduledMaintenanceManagerImageReference,
			"backoffretries":               strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":                strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":              strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":              strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":                 strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":              strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
			"nvidia-device-plugin":         nvidiaDevicePluginImageReference,
		}
	case "1.15":
		ret = map[string]string{
			"hyperkube":                              "hyperkube-amd64:v" + version,
			"kube-proxy":                             "hyperkube-amd64:v" + version,
			"ccm":                                    "cloud-controller-manager-amd64:v" + version,
			"windowszip":                             "v" + version + "/windowszip/v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               heapsterImageReference,
			"metrics-server":                         k8sComponent["metrics-server"],
			"coredns":                                coreDNSImageReference,
			"kube-dns":                               kubeDNSImageReference,
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                kubeDNSMasqNannyImageReference,
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            reschedulerImageReference,
			ACIConnectorAddonName:                    virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"cluster-autoscaler":                     k8sComponent["cluster-autoscaler"],
			"k8s-dns-sidecar":                        kubeDNSSidecarImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                          aadPodIdentityNMIImageReference,
			"mic":                          aadPodIdentityMICImageReference,
			"azure-policy":                 azurePolicyImageReference,
			"gatekeeper":                   gatekeeperImageReference,
			"node-problem-detector":        nodeProblemDetectorImageReference,
			"csi-provisioner":              csiProvisionerImageReference,
			"csi-attacher":                 csiAttacherImageReference,
			"csi-cluster-driver-registrar": csiClusterDriverRegistrarImageReference,
			"livenessprobe":                csiLivenessProbeImageReference,
			"csi-node-driver-registrar":    csiNodeDriverRegistrarImageReference,
			"csi-snapshotter":              csiSnapshotterImageReference,
			"csi-resizer":                  csiResizerImageReference,
			"azuredisk-csi":                csiAzureDiskImageReference,
			"azurefile-csi":                csiAzureFileImageReference,
			"kube-flannel":                 kubeFlannelImageReference,
			"flannel" + "install-cni":      flannelInstallCNIImageReference,
			"kube-rbac-proxy":              KubeRBACProxyImageReference,
			"manager":                      ScheduledMaintenanceManagerImageReference,
			"backoffretries":               strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":                strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":              strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":              strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":                 strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":              strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
			"nvidia-device-plugin":         nvidiaDevicePluginImageReference,
		}
	case "1.8":
		ret = map[string]string{
			"hyperkube":                              "hyperkube-amd64:v" + version,
			"kube-proxy":                             "hyperkube-amd64:v" + version,
			"ccm":                                    "cloud-controller-manager-amd64:v" + version,
			"windowszip":                             "v" + version + "-1int.zip",
			"kubernetes-dashboard":                   dashboardImageReference,
			"exechealthz":                            execHealthZImageReference,
			"addonresizer":                           k8sComponent["addon-resizer"],
			"heapster":                               k8sComponent["heapster"],
			"metrics-server":                         k8sComponent["metrics-server"],
			"kube-dns":                               k8sComponent["kube-dns"],
			"addonmanager":                           k8sComponent["addon-manager"],
			"dnsmasq":                                k8sComponent["dnsmasq"],
			"pause":                                  pauseImageReference,
			"tiller":                                 tillerImageReference,
			"rescheduler":                            k8sComponent["rescheduler"],
			ACIConnectorAddonName:                    virtualKubeletImageReference,
			"container-monitoring":                   omsImageReference,
			"azure-cni-networkmonitor":               azureCNINetworkMonitorImageReference,
			"blobfuse-flexvolume":                    blobfuseFlexVolumeImageReference,
			"smb-flexvolume":                         smbFlexVolumeImageReference,
			"keyvault-flexvolume":                    keyvaultFlexVolumeImageReference,
			datamodel.IPMASQAgentAddonName:           ipMasqAgentImageReference,
			"dns-autoscaler":                         dnsAutoscalerImageReference,
			"azure-npm-daemonset":                    azureNPMContainerImageReference,
			"calico-typha":                           calicoTyphaImageReference,
			"calico-cni":                             calicoCNIImageReference,
			"calico-node":                            calicoNodeImageReference,
			"calico-pod2daemon":                      calicoPod2DaemonImageReference,
			"calico-cluster-proportional-autoscaler": calicoClusterProportionalAutoscalerImageReference,
			"cilium-agent":                           ciliumAgentImageReference,
			"clean-cilium-state":                     ciliumCleanStateImageReference,
			"cilium-operator":                        ciliumOperatorImageReference,
			"cilium-etcd-operator":                   ciliumEtcdOperatorImageReference,
			"antrea-controller":                      antreaControllerImageReference,
			"antrea-agent":                           antreaAgentImageReference,
			"antrea-ovs":                             antreaOVSImageReference,
			"antrea" + "install-cni":                 antreaInstallCNIImageReference,
			"nmi":                     aadPodIdentityNMIImageReference,
			"mic":                     aadPodIdentityMICImageReference,
			"azure-policy":            azurePolicyImageReference,
			"gatekeeper":              gatekeeperImageReference,
			"kube-flannel":            kubeFlannelImageReference,
			"flannel" + "install-cni": flannelInstallCNIImageReference,
			"kube-rbac-proxy":         KubeRBACProxyImageReference,
			"manager":                 ScheduledMaintenanceManagerImageReference,
			"backoffretries":          strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":           strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":         strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":         strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":            strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":       strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":         strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":    strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
		}
	case "1.7":
		ret = map[string]string{
			"hyperkube":                "hyperkube-amd64:v" + version,
			"kube-proxy":               "hyperkube-amd64:v" + version,
			"kubernetes-dashboard":     k8sComponent["dashboard"],
			"exechealthz":              execHealthZImageReference,
			"addonresizer":             k8sComponent["addon-resizer"],
			"heapster":                 k8sComponent["heapster"],
			"metrics-server":           k8sComponent["metrics-server"],
			"kube-dns":                 k8sComponent["kube-dns"],
			"addonmanager":             k8sComponent["addon-manager"],
			"dnsmasq":                  k8sComponent["dnsmasq"],
			"pause":                    pauseImageReference,
			"tiller":                   tillerImageReference,
			"rescheduler":              k8sComponent["rescheduler"],
			ACIConnectorAddonName:      virtualKubeletImageReference,
			"container-monitoring":     omsImageReference,
			"azure-cni-networkmonitor": azureCNINetworkMonitorImageReference,
			"backoffretries":           strconv.Itoa(DefaultKubernetesCloudProviderBackoffRetries),
			"backoffjitter":            strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffJitter, 'f', -1, 64),
			"backoffduration":          strconv.Itoa(DefaultKubernetesCloudProviderBackoffDuration),
			"backoffexponent":          strconv.FormatFloat(DefaultKubernetesCloudProviderBackoffExponent, 'f', -1, 64),
			"ratelimitqps":             strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPS, 'f', -1, 64),
			"ratelimitqpswrite":        strconv.FormatFloat(DefaultKubernetesCloudProviderRateLimitQPSWrite, 'f', -1, 64),
			"ratelimitbucket":          strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucket),
			"ratelimitbucketwrite":     strconv.Itoa(DefaultKubernetesCloudProviderRateLimitBucketWrite),
		}

	default:
		ret = nil
	}
	for k, v := range overrides {
		ret[k] = v
	}
	return ret
}
