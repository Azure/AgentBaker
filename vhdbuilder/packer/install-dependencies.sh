#!/bin/bash

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

#the following sed removes all comments of the format {{/* */}}
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/provision_source.sh
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/tool_installs_distro.sh

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
cat ${THIS_DIR}/kube-proxy-images.json > ${KUBE_PROXY_IMAGES_FILEPATH}
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
  - blobfuse=1.3.7
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
  disableNtpAndTimesyncdInstallChrony || exit 1
fi

if [[ $OS == $MARINER_OS_NAME ]]; then
    disableSystemdResolvedCache
    disableSystemdIptables
    forceEnableIpForward
    networkdWorkaround
fi

downloadKrustlet
echo "  - krustlet ${KRUSTLET_VERSION}" >> ${VHD_LOGS_FILEPATH}

if [[ ${CONTAINER_RUNTIME:-""} == "containerd" ]]; then
  echo "VHD will be built with containerd as the container runtime"
  containerd_version="1.4.8"
  installStandaloneContainerd ${containerd_version}
  echo "  - [installed] containerd v${containerd_version}" >> ${VHD_LOGS_FILEPATH}
  if [[ $OS == $UBUNTU_OS_NAME ]]; then
    # also pre-cache containerd 1.4.4 (last used version)
    containerd_version="1.4.4"
    downloadContainerd ${containerd_version}
    echo "  - [cached] containerd v${containerd_version}" >> ${VHD_LOGS_FILEPATH}
  fi
  CRICTL_VERSIONS="
  1.19.0
  1.20.0
  1.21.0
  "
  for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
    downloadCrictl ${CRICTL_VERSION}
    echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
  # k8s will use images in the k8s.io namespaces - create it
  ctr namespace create k8s.io
  cliTool="ctr"

  # also pre-download Teleportd plugin for containerd
  downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.8.0"
else
  CONTAINER_RUNTIME="docker"
  MOBY_VERSION="19.03.14"
  installMoby
  echo "VHD will be built with docker as container runtime"
  echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
  cliTool="docker"
fi

INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}

## for ubuntu-based images, cache multiple versions of runc
if [[ $OS == $UBUNTU_OS_NAME ]]; then
  RUNC_VERSIONS="
  1.0.0-rc92
  1.0.0-rc95
  "
  for RUNC_VERSION in $RUNC_VERSIONS; do
    downloadDebPkgToFile "moby-runc" ${RUNC_VERSION/\-/\~} ${RUNC_DOWNLOADS_DIR}
    echo "  - [cached] runc ${RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
fi


installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

if [[ $OS == $UBUNTU_OS_NAME ]]; then
installGPUDrivers
echo "  - nvidia-docker2 nvidia-container-runtime" >> ${VHD_LOGS_FILEPATH}
retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" --download-only || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
echo "  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" >> ${VHD_LOGS_FILEPATH}
echo "  - nvidia-gpu-driver-version=${GPU_DV}" >> ${VHD_LOGS_FILEPATH}

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
1.4.9
1.4.7
1.4.0
1.2.7
"
for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-amd64-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# merge with above after two more version releases
SWIFT_CNI_VERSIONS="
1.4.9
1.4.7
1.4.0
1.2.7
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

    SGX_PLUGIN_VERSIONS="
    0.2
    0.4
    "
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="
    0.6
    0.9
    "
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_QUOTE_HELPER_VERSIONS="2.0"
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

# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.18.17
# v1.18.19
# v1.19.11
# v1.19.12
# v1.19.13
# v1.20.7
# v1.20.8
# v1.20.9
# v1.21.1
# v1.21.2
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
  KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.containerdKubeProxyImages.ContainerImages[0].versions[]' <"$THIS_DIR/kube-proxy-images.json")
else
  KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.dockerKubeProxyImages.ContainerImages[0].versions[]' <"$THIS_DIR/kube-proxy-images.json")
fi

for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  # use kube-proxy as well
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
# v1.18.17
# v1.18.19
# v1.19.11
# v1.19.13
# v1.20.7
# v1.20.9
# v1.21.1
# v1.21.2
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
# Please do not use the .1 suffix, because that's only for the base image patches
KUBE_BINARY_VERSIONS="
1.18.17-hotfix.20210505
1.18.19
1.19.11-hotfix.20210823
1.19.13-hotfix.20210830
1.20.7-hotfix.20210816
1.20.9-hotfix.20210830
1.21.1-hotfix.20210827
1.21.2-hotfix.20210830
"
for PATCHED_KUBE_BINARY_VERSION in ${KUBE_BINARY_VERSIONS}; do
  if (($(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    echo "Only need to store k8s components >= 1.19 for containerd VHDs"
    continue
  fi
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-amd64.tar.gz"
done

# shellcheck disable=SC2129
echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

echo -e "=== Installed Packages Begin\n$(listInstalledPackages)\n=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}

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

# remove snapd, which is not used by container stack
apt-get purge --auto-remove snapd -y

# update message-of-the-day to start after multi-user.target
# multi-user.target usually start at the end of the boot sequence
sed -i 's/After=network-online.target/After=multi-user.target/g' /lib/systemd/system/motd-news.service

# retag all the mcr for mooncake
# shellcheck disable=SC2207
allMCRImages=($(docker images | grep '^mcr.microsoft.com/' | awk '{str = sprintf("%s:%s", $1, $2)} {print str}'))
for mcrImage in "${allMCRImages[@]}"; do
  # in mooncake, the mcr endpoint is: mcr.azk8s.cn
  # shellcheck disable=SC2001
  retagMCRImage=$(echo ${mcrImage} | sed -e 's/^mcr.microsoft.com/mcr.azk8s.cn/g')
  retagContainerImage ${cliTool} ${mcrImage} ${retagMCRImage}
done
