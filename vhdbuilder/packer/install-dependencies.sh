#!/bin/bash

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
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

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
cat manifest.json > ${MANIFEST_FILEPATH}
cat ${THIS_DIR}/kube-proxy-images.json > ${KUBE_PROXY_IMAGES_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

if [[ $OS == $MARINER_OS_NAME ]]; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

copyPackerFiles
systemctlEnableAndStart disk_queue || exit 1

mkdir /opt/certs
chmod 666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1
systemctlEnableAndStart update_certs.timer || exit 1

systemctlEnableAndStart ci-syslog-watcher.path || exit 1
systemctlEnableAndStart ci-syslog-watcher.service || exit 1

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - blobfuse=1.4.4
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
  - netcat
  - dnsutils
  - zip
EOF

if [[ $(isARM64) == 1 ]]; then
  if [[ ${ENABLE_FIPS,,} == "true" ]]; then
    echo "No FIPS support on arm64, exiting..."
    exit 1
  fi
  if [[ ${HYPERV_GENERATION,,} == "v1" ]]; then
    echo "No arm64 support on V1 VM, exiting..."
    exit 1
  fi

  if [[ ${CONTAINER_RUNTIME,,} == "docker" ]]; then
    echo "No dockerd is allowed on arm64 vhd, exiting..."
    exit 1
  fi
fi

if [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
  installFIPS
elif [[ ${ENABLE_FIPS,,} == "true" ]]; then
  echo "AKS enables FIPS on Ubuntu 18.04 only, exiting..."
  exit 1
fi

if [[ "${UBUNTU_RELEASE}" == "18.04" || "${UBUNTU_RELEASE}" == "20.04" || "${UBUNTU_RELEASE}" == "22.04" ]]; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi

if [[ $OS == $MARINER_OS_NAME ]]; then
    disableSystemdResolvedCache
    disableSystemdIptables || exit 1
    forceEnableIpForward
    setMarinerNetworkdConfig
    fixCBLMarinerPermissions
    addMarinerNvidiaRepo
    overrideNetworkConfig || exit 1
    if grep -q "kata" <<< "$FEATURE_FLAGS"; then
      enableMarinerKata
    else
      # Leave automatic package update disabled for the kata image
      enableDNFAutomatic
    fi
fi

downloadContainerdWasmShims
echo "  - krustlet ${CONTAINERD_WASM_VERSION}" >> ${VHD_LOGS_FILEPATH}

if [[ ${CONTAINER_RUNTIME:-""} == "containerd" ]]; then
  echo "VHD will be built with containerd as the container runtime"
  updateAptWithMicrosoftPkg
  containerd_manifest="$(jq .containerd manifest.json)" || exit $?
  installed_version="$(echo ${containerd_manifest} | jq -r '.edge')"
  containerd_version="$(echo "$installed_version" | cut -d- -f1)"
  containerd_patch_version="$(echo "$installed_version" | cut -d- -f2)"
  installStandaloneContainerd ${containerd_version} ${containerd_patch_version}
  echo "  - [installed] containerd v${containerd_version}-${containerd_patch_version}" >> ${VHD_LOGS_FILEPATH}

  DOWNLOAD_FILES=$(jq ".DownloadFiles" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
  for componentToDownload in ${DOWNLOAD_FILES[*]}; do
    fileName=$(echo "${componentToDownload}" | jq .fileName -r)
    if [ $fileName == "crictl-v*-linux-amd64.tar.gz" ]; then
      CRICTL_VERSIONS_STR=$(echo "${componentToDownload}" | jq .versions -r)
      CRICTL_VERSIONS=""
      if [[ ${CRICTL_VERSIONS_STR} != null ]]; then
        CRICTL_VERSIONS=$(echo "${CRICTL_VERSIONS_STR}" | jq -r ".[]")
      fi
      break
    fi
  done
  echo $CRICTL_VERSIONS

  for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
    downloadCrictl ${CRICTL_VERSION}
    echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
  
  KUBERNETES_VERSION=$(echo -e "$CRICTL_VERSIONS" | tail -n 2 | head -n 1 | tr -d ' ') installCrictl || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT

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
  1.0.3
  "
  if [[ $(isARM64) == 1 ]]; then
    # RUNC versions of 1.0.3 later might not be available in Ubuntu AMD64/ARM64 repo at the same time
    # so use different version set for different arch to avoid affecting each other during VHD build
    RUNC_VERSIONS="
    1.0.3
    "
  fi
  for RUNC_VERSION in $RUNC_VERSIONS; do
    downloadDebPkgToFile "moby-runc" ${RUNC_VERSION/\-/\~} ${RUNC_DOWNLOADS_DIR}
    echo "  - [cached] runc ${RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
fi

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
  gpu_action="copy"
  export NVIDIA_DRIVER_IMAGE_TAG="cuda-510.47.03-${NVIDIA_DRIVER_IMAGE_SHA}"
  if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    gpu_action="install"
  fi

  mkdir -p /opt/{actions,gpu}
  if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    ctr image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
    bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh $gpu_action" 
    ret=$?
    if [[ "$ret" != "0" ]]; then
      echo "Failed to install GPU driver, exiting..."
      exit $ret
    fi
  else
    bash -c "$DOCKER_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG $gpu_action"
    ret=$?
    if [[ "$ret" != "0" ]]; then
      echo "Failed to install GPU driver, exiting..."
      exit $ret
    fi
  fi
fi

ls -ltr /opt/gpu/* >> ${VHD_LOGS_FILEPATH}

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

cat << EOF >> ${VHD_LOGS_FILEPATH}
  - nvidia-docker2=${NVIDIA_DOCKER_VERSION}
  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}
  - nvidia-gpu-driver-version=${GPU_DV}
  - nvidia-fabricmanager=${GPU_DV}
EOF

installBcc
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF

echo "${CONTAINER_RUNTIME} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

string_replace() {
  echo ${1//\*/$2}
}

ContainerImages=$(jq ".ContainerImages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for imageToBePulled in ${ContainerImages[*]}; do
  downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
  amd64OnlyVersionsStr=$(echo "${imageToBePulled}" | jq .amd64OnlyVersions -r)
  multiArchVersionsStr=$(echo "${imageToBePulled}" | jq .multiArchVersions -r)

  amd64OnlyVersions=""
  if [[ ${amd64OnlyVersionsStr} != null ]]; then
    amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
  fi
  multiArchVersions=""
  if [[ ${multiArchVersionsStr} != null ]]; then
    multiArchVersions=$(echo "${multiArchVersionsStr}" | jq -r ".[]")
  fi

  if [[ $(isARM64) == 1 ]]; then
    versions="${multiArchVersions}"
  else
    versions="${amd64OnlyVersions} ${multiArchVersions}"
  fi

  for version in ${versions}; do
    CONTAINER_IMAGE=$(string_replace $downloadURL $version)
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
  done
done

watcher=$(jq '.ContainerImages[] | select(.downloadURL | contains("aks-node-ca-watcher"))' $COMPONENTS_FILEPATH)
watcherBaseImg=$(echo $watcher | jq -r .downloadURL)
watcherVersion=$(echo $watcher | jq -r .multiArchVersions[0])
watcherFullImg=${watcherBaseImg//\*/$watcherVersion}

# this image will never get pulled, the tag must be the same across different SHAs.
# it will only ever be upgraded via node image changes.
# we do this because the image is used to bootstrap custom CA trust when MCR egress
# may be intercepted by an untrusted TLS MITM firewall.
watcherStaticImg=${watcherBaseImg//\*/static}

# can't use cliTool because crictl doesn't support retagging.
if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    retagContainerImage "ctr" ${watcherFullImg} ${watcherStaticImg}
else
    retagContainerImage "docker" ${watcherFullImg} ${watcherStaticImg}
fi


#must be both amd64/arm64 images
VNET_CNI_VERSIONS="
1.4.32
1.4.35
"


for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI

    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/}
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}
    mkdir "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" 
    tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_DOWNLOADS_DIR/$CNI_DIR_TMP
    rm -rf ${CNI_DOWNLOADS_DIR:?}/${CNI_TGZ_TMP}
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

#UNITE swift and overlay versions?
#Please add new version (>=1.4.13) in this section in order that it can be pulled by both AMD64/ARM64 vhd
SWIFT_CNI_VERSIONS="
1.4.32
1.4.35
"

for VNET_CNI_VERSION in $SWIFT_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-swift-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure Swift CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

OVERLAY_CNI_VERSIONS="
1.4.32
1.4.35
"

for VNET_CNI_VERSION in $OVERLAY_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-overlay-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure Overlay CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done


# After v0.7.6, URI was changed to renamed to https://acs-mirror.azureedge.net/cni-plugins/v*/binaries/cni-plugins-linux-arm64-v*.tgz
MULTI_ARCH_CNI_PLUGIN_VERSIONS="
1.1.1
"
CNI_PLUGIN_VERSIONS="${MULTI_ARCH_CNI_PLUGIN_VERSIONS}"

for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-${CPU_ARCH}-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
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
    0.5
    "
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="
    1.0
    "
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_QUOTE_HELPER_VERSIONS="3.1"
    for SGX_QUOTE_HELPER_VERSION in ${SGX_QUOTE_HELPER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-attestation:${SGX_QUOTE_HELPER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi
fi

NGINX_VERSIONS="1.13.12-alpine"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    if [[ $(isARM64) == 1 ]]; then
        CONTAINER_IMAGE="docker.io/library/nginx:${NGINX_VERSION}"  # nginx in MCR is not 'multi-arch', pull it from docker.io now, upsteam team is building 'multi-arch' nginx for MCR
    else
        CONTAINER_IMAGE="mcr.microsoft.com/oss/nginx/nginx:${NGINX_VERSION}"
    fi
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.

# kube-proxy regular versions >=v1.17.0  hotfixes versions >= 20211009 are 'multi-arch'. All versions in kube-proxy-images.json are 'multi-arch' version now.
if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
  KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.containerdKubeProxyImages.ContainerImages[0].multiArchVersions[]' <"$THIS_DIR/kube-proxy-images.json")
else
  echo "Unsupported container runtime"
  exit 1
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
  echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
done

apt-get -y autoclean
apt-get -y autoremove --purge
apt-get -y clean

# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
# Please do not use the .1 suffix, because that's only for the base image patches
# regular version >= v1.17.0 or hotfixes >= 20211009 has arm64 binaries. 
KUBE_BINARY_VERSIONS="$(jq -r .kubernetes.versions[] manifest.json)"

for PATCHED_KUBE_BINARY_VERSION in ${KUBE_BINARY_VERSIONS}; do
  if (($(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    echo "Only need to store k8s components >= 1.19 for containerd VHDs"
    continue
  fi
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
done

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # remove apport
  apt-get purge --auto-remove apport open-vm-tools -y
fi

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

if [[ $(isARM64) != 1 ]]; then
  # no asc-baseline-1.1.0-268.arm64.deb
  installAscBaseline
fi

if [[ ${UBUNTU_RELEASE} == "18.04" || ${UBUNTU_RELEASE} == "22.04" ]]; then
  if [[ ${ENABLE_FIPS,,} == "true" || ${CPU_ARCH} == "arm64" ]]; then
    relinkResolvConf
  fi
fi

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # remove snapd, which is not used by container stack
  apt-get purge --auto-remove snapd -y
  # update message-of-the-day to start after multi-user.target
  # multi-user.target usually start at the end of the boot sequence
  sed -i 's/After=network-online.target/After=multi-user.target/g' /lib/systemd/system/motd-news.service

  # disable and mask all UU timers/services
  systemctl mask apt-daily.service apt-daily-upgrade.service || exit 1
  systemctl disable apt-daily.service apt-daily-upgrade.service || exit 1
  systemctl disable apt-daily.timer apt-daily-upgrade.timer || exit 1

  tee /etc/apt/apt.conf.d/99periodic > /dev/null <<EOF || exit 1
APT::Periodic::Update-Package-Lists "0";
APT::Periodic::Download-Upgradeable-Packages "0";
APT::Periodic::AutocleanInterval "0";
APT::Periodic::Unattended-Upgrade "0";
EOF

fi
