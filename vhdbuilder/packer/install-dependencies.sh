#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

# fix grub issue with cvm by reinstalling before other deps
# other VHDs use grub-pc, not grub-efi
if [[ "${UBUNTU_RELEASE}" == "20.04" ]] && [[ "$IMG_SKU" == "20_04-lts-cvm" ]]; then
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT 
  wait_for_apt_locks
  apt_get_install 30 1 600 grub-efi || exit 1
fi

if [[ "$OS" == "$UBUNTU_OS_NAME" ]]; then
  # disable and mask all UU timers/services
  # save some background io/latency
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

installDeps

tee -a /etc/systemd/journald.conf > /dev/null <<'EOF'
Storage=persistent
SystemMaxUse=1G
RuntimeMaxUse=1G
ForwardToSyslog=yes
EOF

if [[ ${CONTAINER_RUNTIME:-""} != "containerd" ]]; then
  echo "Unsupported container runtime. Only containerd is supported for new VHD builds."
  exit 1
fi

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

if [[ "${UBUNTU_RELEASE}" == "18.04" || "${UBUNTU_RELEASE}" == "20.04" || "${UBUNTU_RELEASE}" == "22.04" ]]; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi

CONTAINERD_SERVICE_DIR="/etc/systemd/system/containerd.service.d"
mkdir -p "${CONTAINERD_SERVICE_DIR}"
tee "${CONTAINERD_SERVICE_DIR}/exec_start.conf" > /dev/null <<EOF
[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
EOF

tee "/etc/sysctl.d/99-force-bridge-forward.conf" > /dev/null <<EOF 
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
EOF

if [[ $OS == $MARINER_OS_NAME ]]; then
    disableSystemdResolvedCache
    disableSystemdIptables || exit 1
    setMarinerNetworkdConfig
    fixCBLMarinerPermissions
    addMarinerNvidiaRepo
    overrideNetworkConfig || exit 1
    if grep -q "kata" <<< "$FEATURE_FLAGS"; then
      enableMarinerKata
    fi
    disableDNFAutomatic
    enableCheckRestart
    activateNfConntrack
fi

downloadContainerdWasmShims
echo "  - containerd-wasm-shims ${CONTAINERD_WASM_VERSIONS}" >> ${VHD_LOGS_FILEPATH}

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
      CRICTL_VERSIONS=$(echo -e "$CRICTL_VERSIONS" | tail -n 2 | head -n 1 | tr -d ' ')
    fi
    break
  fi
done
echo $CRICTL_VERSIONS

for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
  downloadCrictl ${CRICTL_VERSION}
  echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

KUBERNETES_VERSION=$CRICTL_VERSIONS installCrictl || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT

# k8s will use images in the k8s.io namespaces - create it
ctr namespace create k8s.io
cliTool="ctr"

# also pre-download Teleportd plugin for containerd
downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.8.0"

INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
  gpu_action="copy"
  export NVIDIA_DRIVER_IMAGE_TAG="cuda-525.85.12-${NVIDIA_DRIVER_IMAGE_SHA}"

  mkdir -p /opt/{actions,gpu}
  ctr image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
  if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install" 
    ret=$?
    if [[ "$ret" != "0" ]]; then
      echo "Failed to install GPU driver, exiting..."
      exit $ret
    fi
  fi    
fi

systemctlEnableAndStart containerd-monitor.timer || exit $ERR_SYSTEMCTL_START_FAIL

ls -ltr /opt/gpu/* >> ${VHD_LOGS_FILEPATH}

installBpftrace
echo "  - $(bpftrace --version)" >> ${VHD_LOGS_FILEPATH}

cat << EOF >> ${VHD_LOGS_FILEPATH}
  - nvidia-driver=${NVIDIA_DRIVER_IMAGE_TAG}
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
retagContainerImage "ctr" ${watcherFullImg} ${watcherStaticImg}

# doing this at vhd allows CSE to be faster with just mv
unpackAzureCNI() {
  local URL=$1
  CNI_TGZ_TMP=${URL##*/}
  CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}
  mkdir "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" 
  tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_DOWNLOADS_DIR/$CNI_DIR_TMP
  rm -rf ${CNI_DOWNLOADS_DIR:?}/${CNI_TGZ_TMP}
  echo "  - Ran tar -xzf on the CNI downloaded then rm -rf to clean up"
}

#must be both amd64/arm64 images
VNET_CNI_VERSIONS="
1.4.43
1.4.35
"


for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    unpackAzureCNI $VNET_CNI_PLUGINS_URL
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

#UNITE swift and overlay versions?
#Please add new version (>=1.4.13) in this section in order that it can be pulled by both AMD64/ARM64 vhd
SWIFT_CNI_VERSIONS="
1.4.43
1.4.35
"

for SWIFT_CNI_VERSION in $SWIFT_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${SWIFT_CNI_VERSION}/binaries/azure-vnet-cni-swift-linux-${CPU_ARCH}-v${SWIFT_CNI_VERSION}.tgz"
    downloadAzureCNI
    unpackAzureCNI $VNET_CNI_PLUGINS_URL
    echo "  - Azure Swift CNI version ${SWIFT_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

OVERLAY_CNI_VERSIONS="
1.4.43
1.4.35
"

for OVERLAY_CNI_VERSION in $OVERLAY_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${OVERLAY_CNI_VERSION}/binaries/azure-vnet-cni-overlay-linux-${CPU_ARCH}-v${OVERLAY_CNI_VERSION}.tgz"
    downloadAzureCNI
    unpackAzureCNI $VNET_CNI_PLUGINS_URL
    echo "  - Azure Overlay CNI version ${OVERLAY_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done


# After v0.7.6, URI was changed to renamed to https://acs-mirror.azureedge.net/cni-plugins/v*/binaries/cni-plugins-linux-arm64-v*.tgz
MULTI_ARCH_CNI_PLUGIN_VERSIONS="
1.1.1
"
CNI_PLUGIN_VERSIONS="${MULTI_ARCH_CNI_PLUGIN_VERSIONS}"

for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-${CPU_ARCH}-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    unpackAzureCNI $CNI_PLUGINS_URL
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# IPv6 nftables rules are only available on Ubuntu or Mariner v2
if [[ $OS == $UBUNTU_OS_NAME || ( $OS == $MARINER_OS_NAME && $OS_VERSION == "2.0" ) ]]; then
  systemctlEnableAndStart ipv6_nftables || exit 1
fi

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
NVIDIA_DEVICE_PLUGIN_VERSIONS="
v0.13.0.7
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
  ctr --namespace k8s.io run --rm --mount type=bind,src=${DEST},dst=${DEST},options=bind:rw --cwd ${DEST} "mcr.microsoft.com/oss/nvidia/k8s-device-plugin:v0.13.0.7" plugingextract /bin/sh -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
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
    1.1
    "
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="
    1.1
    "
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_QUOTE_HELPER_VERSIONS="3.3"
    for SGX_QUOTE_HELPER_VERSION in ${SGX_QUOTE_HELPER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-attestation:${SGX_QUOTE_HELPER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi
fi

NGINX_VERSIONS="1.21.6"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/nginx/nginx:${NGINX_VERSION}"
    pullContainerImage ${cliTool} mcr.microsoft.com/oss/nginx/nginx:${NGINX_VERSION}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.

# kube-proxy regular versions >=v1.17.0  hotfixes versions >= 20211009 are 'multi-arch'. All versions in kube-proxy-images.json are 'multi-arch' version now.

KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.containerdKubeProxyImages.ContainerImages[0].multiArchVersions[]' <"$THIS_DIR/kube-proxy-images.json")

for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  # use kube-proxy as well
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBE_PROXY_IMAGE_VERSION}"
  pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
  ctr --namespace k8s.io run --rm ${CONTAINER_IMAGE} checkTask /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
  
  # shellcheck disable=SC2181
  echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
done

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # remove snapd, which is not used by container stack
  apt_get_purge 20 30 120 snapd || exit 1
  apt_get_purge 20 30 120 apache2-utils || exit 1

  apt-get -y autoclean || exit 1
  apt-get -y autoremove --purge || exit 1
  apt-get -y clean || exit 1
  # update message-of-the-day to start after multi-user.target
  # multi-user.target usually start at the end of the boot sequence
  sed -i 's/After=network-online.target/After=multi-user.target/g' /lib/systemd/system/motd-news.service
fi

# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
# Please do not use the .1 suffix, because that's only for the base image patches
# regular version >= v1.17.0 or hotfixes >= 20211009 has arm64 binaries. 
KUBE_BINARY_VERSIONS="$(jq -r .kubernetes.versions[] manifest.json)"

for PATCHED_KUBE_BINARY_VERSION in ${KUBE_BINARY_VERSIONS}; do
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
done

echo "install-dependencies step completed successfully"
