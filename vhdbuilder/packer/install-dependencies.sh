#!/bin/bash
source /home/packer/provision_installs.sh
source /home/packer/provision_source.sh
source /home/packer/packer_source.sh

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

copyPackerFiles

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

AUDITD_ENABLED=true
installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - auditd
  - blobfuse
  - ca-certificates
  - ceph-common
  - cgroup-lite
  - cifs-utils
  - conntrack
  - cracklib-runtime
  - ebtables
  - ethtool
  - fuse
  - git
  - glusterfs-client
  - init-system-helpers
  - iproute2
  - ipset
  - iptables
  - jq
  - libpam-pwquality
  - libpwquality-tools
  - mount
  - nfs-common
  - pigz socat
  - traceroute
  - util-linux
  - xz-utils
  - zip
EOF

if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  overrideNetworkConfig
  disableSystemdTimesyncdAndEnableNTP
fi

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

MOBY_VERSION="3.0.10"
installMoby
echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
installGPUDrivers
echo "  - nvidia-docker2 nvidia-container-runtime" >> ${VHD_LOGS_FILEPATH}
retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" --download-only || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
echo "  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" >> ${VHD_LOGS_FILEPATH}

if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    echo "  - ensureGPUDrivers" >> ${VHD_LOGS_FILEPATH}
    ensureGPUDrivers
fi

installBcc
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF

VNET_CNI_VERSIONS="
1.0.33
1.0.29
"
for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni/azure-vnet-cni-linux-amd64-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

CNI_PLUGIN_VERSIONS="
0.7.6
0.7.5
0.7.1
"
for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

CONTAINERD_VERSIONS="
1.2.4
1.1.6
1.1.5
"
CONTAINERD_DOWNLOAD_URL_BASE="https://storage.googleapis.com/cri-containerd-release/"
for CONTAINERD_VERSION in ${CONTAINERD_VERSIONS}; do
    downloadContainerd
    echo "  - containerd version ${CONTAINERD_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

installImg
echo "  - img" >> ${VHD_LOGS_FILEPATH}

echo "Docker images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

DASHBOARD_VERSIONS="1.10.1"
for DASHBOARD_VERSION in ${DASHBOARD_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kubernetes-dashboard:v${DASHBOARD_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NEW_DASHBOARD_VERSIONS="2.0.0-beta8"
for DASHBOARD_VERSION in ${NEW_DASHBOARD_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/dashboard:v${DASHBOARD_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NEW_DASHBOARD_METRICS_SCRAPER_VERSIONS="1.0.2"
for DASHBOARD_VERSION in ${NEW_DASHBOARD_METRICS_SCRAPER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/metrics-scraper:v${DASHBOARD_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

EXECHEALTHZ_VERSIONS="1.2"
for EXECHEALTHZ_VERSION in ${EXECHEALTHZ_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/exechealthz:${EXECHEALTHZ_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

ADDON_RESIZER_VERSIONS="
1.8.5
1.8.4
1.8.1
1.7
"
for ADDON_RESIZER_VERSION in ${ADDON_RESIZER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:${ADDON_RESIZER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

HEAPSTER_VERSIONS="
1.5.4
1.5.3
1.5.1
"
for HEAPSTER_VERSION in ${HEAPSTER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/heapster:v${HEAPSTER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

METRICS_SERVER_VERSIONS="
0.3.5
"
for METRICS_SERVER_VERSION in ${METRICS_SERVER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/metrics-server:v${METRICS_SERVER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KUBE_DNS_VERSIONS="
1.15.4
1.15.0
1.14.13
1.14.5
"
for KUBE_DNS_VERSION in ${KUBE_DNS_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/k8s-dns-kube-dns:${KUBE_DNS_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KUBE_DNS_MASQ_VERSIONS="
1.15.4
1.15.0
1.14.10
1.14.8
1.14.5
"
for KUBE_DNS_MASQ_VERSION in ${KUBE_DNS_MASQ_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/k8s-dns-dnsmasq-nanny:${KUBE_DNS_MASQ_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

MCR_PAUSE_VERSIONS="1.2.0"
for PAUSE_VERSION in ${MCR_PAUSE_VERSIONS}; do
    # Pull the arch independent MCR pause image which is built for Linux and Windows
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/core/pause:${PAUSE_VERSION}"
    pullContainerImage "docker" "${CONTAINER_IMAGE}"
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

GCR_PAUSE_VERSIONS="
1.2.0
"
for PAUSE_VERSION in ${GCR_PAUSE_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/pause:${PAUSE_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

TILLER_VERSIONS="
2.13.1
2.11.0
2.8.1
"
for TILLER_VERSION in ${TILLER_VERSIONS}; do
    CONTAINER_IMAGE="gcr.io/kubernetes-helm/tiller:v${TILLER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

K8S_DNS_SIDECAR_VERSIONS="
1.14.10
1.14.8
1.14.7
"
for K8S_DNS_SIDECAR_VERSION in ${K8S_DNS_SIDECAR_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/k8s-dns-sidecar:${K8S_DNS_SIDECAR_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CORE_DNS_VERSIONS="
1.6.6
1.6.5
1.5.0
1.3.1
1.2.6
"
for CORE_DNS_VERSION in ${CORE_DNS_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/coredns:${CORE_DNS_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

RESCHEDULER_VERSIONS="
0.4.0
0.3.1
"
for RESCHEDULER_VERSION in ${RESCHEDULER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/rescheduler:v${RESCHEDULER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

VIRTUAL_KUBELET_VERSIONS="latest"
for VIRTUAL_KUBELET_VERSION in ${VIRTUAL_KUBELET_VERSIONS}; do
    CONTAINER_IMAGE="microsoft/virtual-kubelet:${VIRTUAL_KUBELET_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZURE_CNIIMAGEBASE="mcr.microsoft.com/containernetworking"
AZURE_CNI_NETWORKMONITOR_VERSIONS="
0.0.7
0.0.6
"
for AZURE_CNI_NETWORKMONITOR_VERSION in ${AZURE_CNI_NETWORKMONITOR_VERSIONS}; do
    CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/networkmonitor:v${AZURE_CNI_NETWORKMONITOR_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZURE_NPM_VERSIONS="
1.1.0
1.0.33
1.0.32
1.0.30
"
for AZURE_NPM_VERSION in ${AZURE_NPM_VERSIONS}; do
    CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/azure-npm:v${AZURE_NPM_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZURE_VNET_TELEMETRY_VERSIONS="
1.0.30
"
for AZURE_VNET_TELEMETRY_VERSION in ${AZURE_VNET_TELEMETRY_VERSIONS}; do
    CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/azure-vnet-telemetry:v${AZURE_VNET_TELEMETRY_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NVIDIA_DEVICE_PLUGIN_VERSIONS="
1.11
1.10
"
for NVIDIA_DEVICE_PLUGIN_VERSION in ${NVIDIA_DEVICE_PLUGIN_VERSIONS}; do
    CONTAINER_IMAGE="nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# GPU device plugin
if grep -q "fullgpu" <<< "$FEATURE_FLAGS" && grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
    kubeletDevicePluginPath="/var/lib/kubelet/device-plugins"
    mkdir -p $kubeletDevicePluginPath
    echo "  - $kubeletDevicePluginPath" >> ${VHD_LOGS_FILEPATH}

    DEST="/usr/local/nvidia/bin"
    mkdir -p $DEST
    docker run --rm --entrypoint "" -v $DEST:$DEST "nvidia/k8s-device-plugin:1.11" /bin/bash -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
    chmod a+x $DEST/nvidia-device-plugin
    echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
    ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

    systemctlEnableAndStart nvidia-device-plugin || exit 1
fi

TUNNELFRONT_VERSIONS="v1.9.2-v3.0.11 v1.9.2-v4.0.11"
for TUNNELFRONT_VERSION in ${TUNNELFRONT_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/hcp-tunnel-front:${TUNNELFRONT_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KUBE_SVC_REDIRECT_VERSIONS="1.0.7"
for KUBE_SVC_REDIRECT_VERSION in ${KUBE_SVC_REDIRECT_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/kube-svc-redirect:v${KUBE_SVC_REDIRECT_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# oms agent used by AKS
OMS_AGENT_IMAGES="ciprod01072020 ciprod03022020"
for OMS_AGENT_IMAGE in ${OMS_AGENT_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/azuremonitor/containerinsights/ciprod:${OMS_AGENT_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# calico images used by AKS
CALICO_CNI_IMAGES="v3.5.0"
for CALICO_CNI_IMAGE in ${CALICO_CNI_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/cni:${CALICO_CNI_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CALICO_NODE_IMAGES="v3.5.0"
for CALICO_NODE_IMAGE in ${CALICO_NODE_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/node:${CALICO_NODE_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CALICO_TYPHA_IMAGES="v3.5.0"
for CALICO_TYPHA_IMAGE in ${CALICO_TYPHA_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/typha:${CALICO_TYPHA_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# Cluster Proportional Autoscaler
CPA_IMAGES="
1.3.0
1.3.0_v0.0.5
1.7.1
1.7.1-hotfix.20200403
"
for CPA_IMAGE in ${CPA_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/autoscaler/cluster-proportional-autoscaler:${CPA_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KV_FLEXVOLUME_VERSIONS="0.0.13"
for KV_FLEXVOLUME_VERSION in ${KV_FLEXVOLUME_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/flexvolume/keyvault-flexvolume:v${KV_FLEXVOLUME_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

BLOBFUSE_FLEXVOLUME_VERSIONS="1.0.8"
for BLOBFUSE_FLEXVOLUME_VERSION in ${BLOBFUSE_FLEXVOLUME_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/flexvolume/blobfuse-flexvolume:${BLOBFUSE_FLEXVOLUME_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# this is the patched images which AKS are using.
AKS_IP_MASQ_AGENT_VERSIONS="
2.0.0_v0.0.5
"
for IP_MASQ_AGENT_VERSION in ${AKS_IP_MASQ_AGENT_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/ip-masq-agent:v${IP_MASQ_AGENT_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NGINX_VERSIONS="1.13.12-alpine"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    CONTAINER_IMAGE="nginx:${NGINX_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KMS_PLUGIN_VERSIONS="0.0.9"
for KMS_PLUGIN_VERSION in ${KMS_PLUGIN_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/kms/keyvault:v${KMS_PLUGIN_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

pullContainerImage "docker" "busybox"
echo "  - busybox" >> ${VHD_LOGS_FILEPATH}


# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
K8S_VERSIONS="
1.17.0
1.16.6
1.16.4
1.16.1
1.12.8_v0.0.5
1.13.10_v0.0.5
1.13.11_v0.0.5
1.13.12_f0.0.2
1.14.6_v0.0.5
1.14.7-hotfix.20200326
1.14.8-hotfix.20200326
1.15.3_v0.0.5
1.15.4_v0.0.5
1.15.5_f0.0.2
1.15.7-hotfix.20200326
1.15.10-hotfix.20200326
1.16.0_v0.0.5
1.16.7-hotfix.20200408
1.17.3-hotfix.20200408
"
for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do
  HYPERKUBE_URL="mcr.microsoft.com/oss/kubernetes/hyperkube:v${PATCHED_KUBERNETES_VERSION}"
  # NOTE: the KUBERNETES_VERSION will be used to tag the extracted kubelet/kubectl in /usr/local/bin
  # it should match the KUBERNETES_VERSION format(just version number, e.g. 1.15.7, no prefix v)
  # in installKubeletAndKubectl() executed by cse, otherwise cse will need to download the kubelet/kubectl again
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1)
  # extractHyperkube will extract the kubelet/kubectl binary from the image: ${HYPERKUBE_URL}
  # and put them to /usr/local/bin/kubelet-${KUBERNETES_VERSION}
  extractHyperkube "docker"
done
ls -ltr /usr/local/bin >> ${VHD_LOGS_FILEPATH}

# pull patched hyperkube image for AKS
# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
PATCHED_HYPERKUBE_IMAGES="
1.12.8_v0.0.5
1.13.10_v0.0.5
1.13.11_v0.0.5
1.13.12_f0.0.2
1.14.6_v0.0.5
1.14.7_v0.0.5
1.14.7-hotfix.20200326
1.14.8_f0.0.4
1.14.8-hotfix.20200326
1.15.3_v0.0.5
1.15.4_v0.0.5
1.15.5_f0.0.2
1.15.7_f0.0.2
1.15.7-hotfix.20200326
1.15.10_f0.0.1
1.15.10-hotfix.20200326
1.16.0_v0.0.5
1.16.7-hotfix.20200326
1.16.7-hotfix.20200408
1.17.3-hotfix.20200326
1.17.3-hotfix.20200408
"
for KUBERNETES_VERSION in ${PATCHED_HYPERKUBE_IMAGES}; do
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/hyperkube:v${KUBERNETES_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

ADDON_IMAGES="
mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v2.0.1
mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v3.1.0-beta.7
mcr.microsoft.com/oss/kubernetes/external-dns:v0.6.0-hotfix-20200228
mcr.microsoft.com/oss/kubernetes/defaultbackend:1.4
mcr.microsoft.com/oss/kubernetes/ingress/nginx-ingress-controller:0.19.0
mcr.microsoft.com/oss/virtual-kubelet/virtual-kubelet
mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20200325.1
mcr.microsoft.com/azure-application-gateway/kubernetes-ingress:1.0.1-rc3
"
for ADDON_IMAGE in ${ADDON_IMAGES}; do
  pullContainerImage "docker" ${ADDON_IMAGE}
  echo "  - ${ADDON_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZUREDISK_CSI_VERSIONS="
0.4.0
"
for AZUREDISK_CSI_VERSION in ${AZUREDISK_CSI_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azuredisk-csi:v${AZUREDISK_CSI_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZUREFILE_CSI_VERSIONS="
0.3.0
"
for AZUREFILE_CSI_VERSION in ${AZUREFILE_CSI_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azurefile-csi:v${AZUREFILE_CSI_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_ATTACHER_VERSIONS="
1.0.1
"
for CSI_ATTACHER_VERSION in ${CSI_ATTACHER_VERSIONS}; do
  CONTAINER_IMAGE="quay.io/k8scsi/csi-attacher:v${CSI_ATTACHER_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_CLUSTER_DRIVER_REGISTRAR_VERSIONS="
1.0.1
"
for CSI_CLUSTER_DRIVER_REGISTRAR_VERSION in ${CSI_CLUSTER_DRIVER_REGISTRAR_VERSIONS}; do
  CONTAINER_IMAGE="quay.io/k8scsi/csi-cluster-driver-registrar:v${CSI_CLUSTER_DRIVER_REGISTRAR_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_NODE_DRIVER_REGISTRAR_VERSIONS="
1.1.0
"
for CSI_NODE_DRIVER_REGISTRAR_VERSION in ${CSI_NODE_DRIVER_REGISTRAR_VERSIONS}; do
  CONTAINER_IMAGE="quay.io/k8scsi/csi-node-driver-registrar:v${CSI_NODE_DRIVER_REGISTRAR_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_PROVISIONER_VERSIONS="
1.0.1
"
for CSI_PROVISIONER_VERSION in ${CSI_PROVISIONER_VERSIONS}; do
  CONTAINER_IMAGE="quay.io/k8scsi/csi-provisioner:v${CSI_PROVISIONER_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

LIVENESSPROBE_VERSIONS="
1.1.0
"
for LIVENESSPROBE_VERSION in ${LIVENESSPROBE_VERSIONS}; do
  CONTAINER_IMAGE="quay.io/k8scsi/livenessprobe:v${LIVENESSPROBE_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NODE_PROBLEM_DETECTOR_VERSIONS="
0.8.0
"
for NODE_PROBLEM_DETECTOR_VERSION in ${NODE_PROBLEM_DETECTOR_VERSIONS}; do
  CONTAINER_IMAGE="k8s.gcr.io/node-problem-detector:v${NODE_PROBLEM_DETECTOR_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH}

df -h

# warn at 75% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 75 {print}') ] || echo "WARNING: 75% of /dev/sda1 is used" >> ${VHD_LOGS_FILEPATH}
# error at 95% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 95 {print}') ] || exit 1

echo "Using kernel:" >> ${VHD_LOGS_FILEPATH}
tee -a ${VHD_LOGS_FILEPATH} < /proc/version
{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER}"
  echo "VSTS Build ID: ${BUILD_ID}"
  echo "Commit: ${COMMIT}"
  echo "Feature flags: ${FEATURE_FLAGS}"
} >> ${VHD_LOGS_FILEPATH}