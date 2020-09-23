#!/bin/bash
source /home/packer/provision_installs.sh
source /home/packer/provision_source.sh
source /home/packer/tool_installs.sh
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
  - blobfuse=1.1.1
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
  overrideNetworkConfig || exit 1
  disableSystemdTimesyncdAndEnableNTP || exit 1
fi

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

MOBY_VERSION="19.03.12"
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
1.1.6
1.1.3
1.0.33
"
for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-amd64-v${VNET_CNI_VERSION}.tgz"
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

CNI_PLUGIN_VERSIONS="
0.8.6
"
for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-amd64-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
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

NEW_DASHBOARD_VERSIONS="
2.0.0-beta8
2.0.0-rc3
2.0.0-rc7
2.0.1
"
for DASHBOARD_VERSION in ${NEW_DASHBOARD_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/dashboard:v${DASHBOARD_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

NEW_DASHBOARD_METRICS_SCRAPER_VERSIONS="
1.0.2
1.0.3
1.0.4
"
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

METRICS_SERVER_VERSIONS="
0.3.6
0.3.5
"
for METRICS_SERVER_VERSION in ${METRICS_SERVER_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/metrics-server:v${METRICS_SERVER_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

LEGACY_MCR_PAUSE_VERSIONS="1.2.0"
for PAUSE_VERSION in ${LEGACY_MCR_PAUSE_VERSIONS}; do
    # Pull the arch independent MCR pause image which is built for Linux and Windows
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/core/pause:${PAUSE_VERSION}"
    pullContainerImage "docker" "${CONTAINER_IMAGE}"
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

MCR_PAUSE_VERSIONS="
1.2.0
1.3.1
1.4.0
"
for PAUSE_VERSION in ${MCR_PAUSE_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/pause:${PAUSE_VERSION}"
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
1.1.7
1.1.5
1.1.4
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
installSGX=${SGX_DEVICE_PLUGIN_INSTALL:-"False"}
if [[ ${installSGX} == "True" ]]; then
    SGX_DEVICE_PLUGIN_VERSIONS="1.0"
    for SGX_DEVICE_PLUGIN_VERSION in ${SGX_DEVICE_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-device-plugin:${SGX_DEVICE_PLUGIN_VERSION}"
        pullContainerImage "docker" ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi

TUNNELFRONT_VERSIONS="
v1.9.2-v3.0.17
v1.9.2-v3.0.18
v1.9.2-v4.0.15
v1.9.2-v4.0.16
"
for TUNNELFRONT_VERSION in ${TUNNELFRONT_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/hcp-tunnel-front:${TUNNELFRONT_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# 1.0.10 is for the ipv6 fix
# 1.0.11 is for the cve fix
OPENVPN_VERSIONS="
1.0.8
1.0.10
1.0.11
"
for OPENVPN_VERSION in ${OPENVPN_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/tunnel-openvpn:${OPENVPN_VERSION}"
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
# keeping last released image (ciprod07152020 - hotfix) and current to be released image (ciprod08072020)
OMS_AGENT_IMAGES="ciprod07152020 ciprod08072020"
for OMS_AGENT_IMAGE in ${OMS_AGENT_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/azuremonitor/containerinsights/ciprod:${OMS_AGENT_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# calico images used by AKS
CALICO_CNI_IMAGES="
v3.5.0
v3.8.0
"
for CALICO_CNI_IMAGE in ${CALICO_CNI_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/cni:${CALICO_CNI_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CALICO_NODE_IMAGES="
v3.5.0
v3.8.0
"
for CALICO_NODE_IMAGE in ${CALICO_NODE_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/node:${CALICO_NODE_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CALICO_TYPHA_IMAGES="
v3.5.0
v3.8.0
"
for CALICO_TYPHA_IMAGE in ${CALICO_TYPHA_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/typha:${CALICO_TYPHA_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CALICO_POD2DAEMON_IMAGES="v3.8.0"
for CALICO_POD2DAEMON_IMAGE in ${CALICO_POD2DAEMON_IMAGES}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/pod2daemon-flexvol:${CALICO_POD2DAEMON_IMAGE}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# Cluster Proportional Autoscaler
CPA_IMAGES="
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

BLOBFUSE_FLEXVOLUME_VERSIONS="1.0.13"
for BLOBFUSE_FLEXVOLUME_VERSION in ${BLOBFUSE_FLEXVOLUME_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/flexvolume/blobfuse-flexvolume:${BLOBFUSE_FLEXVOLUME_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# this is the patched images which AKS are using.
AKS_IP_MASQ_AGENT_VERSIONS="
2.5.0
2.5.0.1
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


# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.16.13-hotfix.20200824.1
# v1.16.15-hotfix.20200903
# v1.17.9-hotfix.20200824.1
# v1.17.11-hotfix.20200901
# v1.18.6-hotfix.20200723.1
# v1.18.8
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
K8S_VERSIONS="
1.15.7-hotfix.20200326
1.15.10-hotfix.20200408.1
1.15.11-hotfix.20200824.1
1.15.12-hotfix.20200824.1
1.16.9-hotfix.20200529.1
1.16.10-hotfix.20200824.1
1.16.13-hotfix.20200824.1
1.16.15-hotfix.20200903
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200817.1
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901
1.18.2-hotfix.20200624.1
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.8
1.19.0
"
for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do
  if (($(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f2) < 17)); then
    HYPERKUBE_URL="mcr.microsoft.com/oss/kubernetes/hyperkube:v${PATCHED_KUBERNETES_VERSION}"
    # NOTE: the KUBERNETES_VERSION will be used to tag the extracted kubelet/kubectl in /usr/local/bin
    # it should match the KUBERNETES_VERSION format(just version number, e.g. 1.15.7, no prefix v)
    # in installKubeletAndKubectl() executed by cse, otherwise cse will need to download the kubelet/kubectl again
    KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    # extractHyperkube will extract the kubelet/kubectl binary from the image: ${HYPERKUBE_URL}
    # and put them to /usr/local/bin/kubelet-${KUBERNETES_VERSION}
    extractHyperkube "docker"
    # remove hyperkube here as the one that we really need is pulled later
    docker image rm $HYPERKUBE_URL
  else
    # strip the last .1 as that is for base image patch for hyperkube
    if grep -iq hotfix <<< ${PATCHED_KUBERNETES_VERSION}; then
      # shellcheck disable=SC2006
      PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3,4`;
    else
      PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3`;
    fi
    KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBERNETES_VERSION}/binaries/kubernetes-node-linux-amd64.tar.gz"
  fi
done
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# pull patched hyperkube image for AKS
# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.16.13-hotfix.20200824.1
# v1.16.15-hotfix.20200903
# v1.17.9-hotfix.20200824.1
# v1.17.11-hotfix.20200901
# v1.18.6-hotfix.20200723.1
# v1.18.8
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP
PATCHED_HYPERKUBE_IMAGES="
1.15.11-hotfix.20200529.1
1.15.12-hotfix.20200623.1
1.15.12-hotfix.20200714.1
1.16.9-hotfix.20200529.1
1.16.10-hotfix.20200623.1
1.16.10-hotfix.20200714.1
1.16.10-hotfix.20200817.1
1.16.10-hotfix.20200824.1
1.16.13-hotfix.20200714.1
1.16.13-hotfix.20200817.1
1.16.13-hotfix.20200824.1
1.16.15-hotfix.20200903
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200624
1.17.7-hotfix.20200714.1
1.17.7-hotfix.20200714.2
1.17.9-hotfix.20200714.1
1.17.9-hotfix.20200817.1
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901
1.18.4-hotfix.20200624
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.6
1.18.8
1.19.0
"
for KUBERNETES_VERSION in ${PATCHED_HYPERKUBE_IMAGES}; do
  # TODO: after CCP chart is done, change below to get hyperkube only for versions less than 1.17 only
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) < 19)); then
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/hyperkube:v${KUBERNETES_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    docker run --rm --entrypoint "" ${CONTAINER_IMAGE}  /bin/sh -c "iptables --version" | grep -v nf_tables && echo "Hyperkube contains no nf_tables"
    # shellcheck disable=SC2181
    if [[ $? != 0 ]]; then
      echo "Hyperkube contains nf_tables, exiting..."
      exit 99
    fi
  fi

  # from 1.17 onwards start using kube-proxy as well
  # strip the last .1 as that is for base image patch for hyperkube
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)); then
    if grep -iq hotfix <<< ${KUBERNETES_VERSION}; then
      KUBERNETES_VERSION=`echo ${KUBERNETES_VERSION} | cut -d"." -f1,2,3,4`;
    else
      KUBERNETES_VERSION=`echo ${KUBERNETES_VERSION} | cut -d"." -f1,2,3`;
    fi
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBERNETES_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    docker run --rm --entrypoint "" ${CONTAINER_IMAGE}  /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
    # shellcheck disable=SC2181
    if [[ $? != 0 ]]; then
      echo "Hyperkube contains nf_tables, exiting..."
      exit 99
    fi
    echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
  fi
done

ADDON_IMAGES="
mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v2.0.1
mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v3.1.0-beta.12
mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v3.1.0
mcr.microsoft.com/oss/kubernetes/external-dns:v0.6.0-hotfix-20200228
mcr.microsoft.com/oss/kubernetes/defaultbackend:1.4
mcr.microsoft.com/oss/kubernetes/ingress/nginx-ingress-controller:0.19.0
mcr.microsoft.com/oss/virtual-kubelet/virtual-kubelet:1.2.1.1
mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20200804.1
mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20200901.1
mcr.microsoft.com/azure-policy/policy-kubernetes-webhook:prod_20200505.3
mcr.microsoft.com/azure-application-gateway/kubernetes-ingress:1.0.1-rc3
"
for ADDON_IMAGE in ${ADDON_IMAGES}; do
  pullContainerImage "docker" ${ADDON_IMAGE}
  echo "  - ${ADDON_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZUREDISK_CSI_VERSIONS="
0.7.0
0.9.0
"
for AZUREDISK_CSI_VERSION in ${AZUREDISK_CSI_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azuredisk-csi:v${AZUREDISK_CSI_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

AZUREFILE_CSI_VERSIONS="
0.7.0
0.9.0
"
for AZUREFILE_CSI_VERSION in ${AZUREFILE_CSI_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azurefile-csi:v${AZUREFILE_CSI_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_LIVENESSPROBE_VERSIONS="
1.1.0
"
for CSI_LIVENESSPROBE_VERSION in ${CSI_LIVENESSPROBE_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v${CSI_LIVENESSPROBE_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

CSI_NODE_DRIVER_REGISTRAR_VERSIONS="
1.2.0
"
for CSI_NODE_DRIVER_REGISTRAR_VERSION in ${CSI_NODE_DRIVER_REGISTRAR_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v${CSI_NODE_DRIVER_REGISTRAR_VERSION}"
  pullContainerImage "docker" ${CONTAINER_IMAGE}
  echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

df -h

# warn at 75% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 75 {print}') ] || echo "WARNING: 75% of /dev/sda1 is used" >> ${VHD_LOGS_FILEPATH}
# error at 99% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 99 {print}') ] || exit 1

echo "Using kernel:" >> ${VHD_LOGS_FILEPATH}
tee -a ${VHD_LOGS_FILEPATH} < /proc/version
{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER}"
  echo "VSTS Build ID: ${BUILD_ID}"
  echo "Commit: ${COMMIT}"
  echo "Ubuntu version: ${UBUNTU_RELEASE}"
  echo "Hyperv generation: ${HYPERV_GENERATION}"
  echo "Feature flags: ${FEATURE_FLAGS}"
} >> ${VHD_LOGS_FILEPATH}
