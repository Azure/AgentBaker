#!/bin/bash

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

if [[ $OS == $MARINER_OS_NAME ]]; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

copyPackerFiles

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - blobfuse=1.3.5
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

if [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
  installFIPS
elif [[ ${ENABLE_FIPS,,} == "true" ]]; then
  echo "AKS enables FIPS on Ubuntu 18.04 only, exiting..."
  exit 1
fi

if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  overrideNetworkConfig || exit 1
  disableSystemdTimesyncdAndEnableNTP || exit 1
fi

if [[ ${CONTAINER_RUNTIME:-""} == "containerd" ]]; then
  echo "VHD will be built with containerd as the container runtime"
  CONTAINERD_VERSION="1.5.0-beta.git31a0f92df"
  installStandaloneContainerd
  echo "  - containerd v${CONTAINERD_VERSION}" >> ${VHD_LOGS_FILEPATH}
  CRICTL_VERSIONS="1.19.0"
  for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
    downloadCrictl ${CRICTL_VERSION}
    echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
  # k8s will use images in the k8s.io namespaces - create it
  ctr namespace create k8s.io
  cliTool="ctr"

  # also pre-download Teleportd plugin for containerd
  downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.6.0"
else
  CONTAINER_RUNTIME="docker"
  MOBY_VERSION="19.03.14"
  installMoby
  echo "VHD will be built with docker as container runtime"
  echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
  cliTool="docker"
fi

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

if [[ $OS == $UBUNTU_OS_NAME ]]; then
installGPUDrivers
echo "  - nvidia-docker2 nvidia-container-runtime" >> ${VHD_LOGS_FILEPATH}
retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" --download-only || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
echo "  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" >> ${VHD_LOGS_FILEPATH}

if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    echo "  - ensureGPUDrivers" >> ${VHD_LOGS_FILEPATH}
    ensureGPUDrivers
fi
fi

installBcc
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF

installImg
echo "  - img" >> ${VHD_LOGS_FILEPATH}

echo "${CONTAINER_RUNTIME} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

string_replace() {
  echo ${1//\*/$2}
}

ContainerImages=$(jq ".ContainerImages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for imageToBePulled in ${ContainerImages[*]}; do
  downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
  versions=$(echo "${imageToBePulled}" | jq .versions -r | jq -r ".[]")

  for version in ${versions}; do
    CONTAINER_IMAGE=$(string_replace $downloadURL $version)
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
  done
done

VNET_CNI_VERSIONS="
1.2.7
1.2.6
1.2.0_hotfix
"
for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-amd64-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# merge with above after two more version releases
SWIFT_CNI_VERSIONS="
1.2.7
1.2.6
"

for VNET_CNI_VERSION in $SWIFT_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-swift-linux-amd64-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure Swift CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
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

if [[ $OS == $UBUNTU_OS_NAME ]]; then
NVIDIA_DEVICE_PLUGIN_VERSIONS="
v0.9.0
"
for NVIDIA_DEVICE_PLUGIN_VERSION in ${NVIDIA_DEVICE_PLUGIN_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# GPU device plugin
if grep -q "fullgpu" <<< "$FEATURE_FLAGS" && grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
  kubeletDevicePluginPath="/var/lib/kubelet/device-plugins"
  mkdir -p $kubeletDevicePluginPath
  echo "  - $kubeletDevicePluginPath" >> ${VHD_LOGS_FILEPATH}

  DEST="/usr/local/nvidia/bin"
  mkdir -p $DEST
  if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    ctr --namespace k8s.io run --rm --mount type=bind,src=${DEST},dst=${DEST},options=bind:rw --cwd ${DEST} "mcr.microsoft.com/oss/nvidia/k8s-device-plugin:v0.9.0" plugingextract /bin/sh -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  else
    docker run --rm --entrypoint "" -v $DEST:$DEST "mcr.microsoft.com/oss/nvidia/k8s-device-plugin:v0.9.0" /bin/bash -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  fi
  chmod a+x $DEST/nvidia-device-plugin
  echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
  ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

  systemctlEnableAndStart nvidia-device-plugin || exit 1
fi

installSGX=${SGX_INSTALL:-"False"}
if [[ ${installSGX} == "True" ]]; then
    SGX_DEVICE_PLUGIN_VERSIONS="1.0"
    for SGX_DEVICE_PLUGIN_VERSION in ${SGX_DEVICE_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-device-plugin:${SGX_DEVICE_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_PLUGIN_VERSIONS="0.2"
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="0.6"
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_QUOTE_HELPER_VERSIONS="1.0"
    for SGX_QUOTE_HELPER_VERSION in ${SGX_QUOTE_HELPER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-attestation:${SGX_QUOTE_HELPER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi
fi

NGINX_VERSIONS="1.13.12-alpine"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    if [[ "${cliTool}" == "ctr" ]]; then
      # containerd/ctr doesn't auto-resolve to docker.io
      CONTAINER_IMAGE="docker.io/library/nginx:${NGINX_VERSION}"
    else
      CONTAINER_IMAGE="nginx:${NGINX_VERSION}"
    fi
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

KUBE_PROXY_IMAGE_VERSIONS="
1.17.13
1.17.16
1.18.8-hotfix.20200924
1.18.10-hotfix.20210118
1.18.14-hotfix.20210118
1.18.14-hotfix.20210322
1.18.17-hotfix.20210322
1.19.1-hotfix.20200923
1.19.3
1.19.6-hotfix.20210118
1.19.7-hotfix.20210310
1.19.9-hotfix.20210322
1.20.2
1.20.2-hotfix.20210310
1.20.5-hotfix.20210322
"
for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  if [[ ${CONTAINER_RUNTIME} == "containerd" ]] && (($(echo ${KUBE_PROXY_IMAGE_VERSIONS} | cut -d"." -f2) < 19)) ; then
    echo "Only need to store k8s components >= 1.19 for containerd VHDs"
    continue
  fi
  # use kube-proxy as well
  # strip the last .1 as that is for base image patch for hyperkube
  if grep -iq hotfix <<< ${KUBE_PROXY_IMAGE_VERSION}; then
    KUBE_PROXY_IMAGE_VERSION=`echo ${KUBE_PROXY_IMAGE_VERSION} | cut -d"." -f1,2,3,4`;
  else
    KUBE_PROXY_IMAGE_VERSION=`echo ${KUBE_PROXY_IMAGE_VERSION} | cut -d"." -f1,2,3`;
  fi
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBE_PROXY_IMAGE_VERSION}"
  pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
  if [[ ${cliTool} == "docker" ]]; then
      docker run --rm --entrypoint "" ${CONTAINER_IMAGE} /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
  else
      ctr --namespace k8s.io run --rm ${CONTAINER_IMAGE} checkTask /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
  fi
  # shellcheck disable=SC2181
  if [[ $? != 0 ]]; then
  echo "Hyperkube contains nf_tables, exiting..."
  exit 99
  fi
  echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
done

# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.18.14
# v1.18.17
# v1.19.7
# v1.19.9
# v1.20.2
# v1.20.5
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
K8S_VERSIONS="
1.17.13
1.17.16
1.18.8-hotfix.20200924
1.18.10-hotfix.20210118
1.18.14-hotfix.20210322
1.18.17-hotfix.20210322
1.19.1-hotfix.20200923
1.19.3
1.19.6-hotfix.20210118
1.19.7-hotfix.20210310
1.19.9-hotfix.20210322
1.20.2-hotfix.20210310
1.20.5-hotfix.20210322
"
for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do
  if (($(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    echo "Only need to store k8s components >= 1.19 for containerd VHDs"
    continue
  fi
  # strip the last .1 as that is for base image patch for hyperkube
  if grep -iq hotfix <<< ${PATCHED_KUBERNETES_VERSION}; then
    # shellcheck disable=SC2006
    PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3,4`;
  else
    PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3`;
  fi
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBERNETES_VERSION}/binaries/kubernetes-node-linux-amd64.tar.gz"
done

# shellcheck disable=SC2129
echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}
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
  echo "Container runtime: ${CONTAINER_RUNTIME}"
  echo "FIPS enabled: ${ENABLE_FIPS}"
} >> ${VHD_LOGS_FILEPATH}

installAscBaseline

if [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
  relinkResolvConf
fi
