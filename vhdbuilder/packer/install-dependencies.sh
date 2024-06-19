#!/bin/bash

script_start_timestamp=$(date +%H:%M:%S)
start_timestamp=$(date +%H:%M:%S)

capture_script_start=$(date +%s)
capture_time=$(date +%s)

declare -a benchmarks=()

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
AZURELINUX_OS_NAME="AZURELINUX"
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
stop_watch $capture_time "Declare Variables / Configure Environment" false
start_watch

echo "Logging the kernel after purge and reinstall + reboot: $(uname -r)"
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
stop_watch $capture_time "Purge and Re-install Ubuntu" false
start_watch

# If the IMG_SKU does not contain "minimal", installDeps normally
if [[ "$IMG_SKU" != *"minimal"* ]]; then
  installDeps
else
  updateAptWithMicrosoftPkg
  # The following packages are required for an Ubuntu Minimal Image to build and successfully run CSE
  # blobfuse2 and fuse3 - ubuntu 22.04 supports blobfuse2 and is fuse3 compatible
  BLOBFUSE2_VERSION="2.2.1"
  if [ "${OS_VERSION}" == "18.04" ]; then
    # keep legacy version on ubuntu 18.04
    BLOBFUSE2_VERSION="2.2.0"
  fi
  required_pkg_list=("blobfuse2="${BLOBFUSE2_VERSION} fuse3)
  for apt_package in ${required_pkg_list[*]}; do
      if ! apt_get_install 30 1 600 $apt_package; then
          journalctl --no-pager -u $apt_package
          exit $ERR_APT_INSTALL_TIMEOUT
      fi
  done
fi

tee -a /etc/systemd/journald.conf > /dev/null <<'EOF'
Storage=persistent
SystemMaxUse=1G
RuntimeMaxUse=1G
ForwardToSyslog=yes
EOF
stop_watch $capture_time "Install Dependencies" false
start_watch

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

# Since we do not build Ubuntu 16.04 images anymore, always override network config and disable NTP + Timesyncd and install Chrony
# Mariner does this differently, so only do it for Ubuntu
if [[ $OS != $MARINER_OS_NAME ]]  && [[ $OS != $AZURELINUX_OS_NAME ]]; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi
stop_watch $capture_time "Check Container Runtime / Network Configurations" false
start_watch

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

echo "set read ahead size to 15380 KB"
AWK_PATH=$(command -v awk)
cat > /etc/udev/rules.d/99-nfs.rules <<EOF
SUBSYSTEM=="bdi", ACTION=="add", PROGRAM="$AWK_PATH -v bdi=\$kernel 'BEGIN{ret=1} {if (\$4 == bdi){ret=0}} END{exit ret}' /proc/fs/nfsfs/volumes", ATTR{read_ahead_kb}="15380"
EOF
udevadm control --reload

if [[ $OS == $MARINER_OS_NAME ]] || [[ $OS == $AZURELINUX_OS_NAME ]]; then
    disableSystemdResolvedCache
    disableSystemdIptables || exit 1
    setMarinerNetworkdConfig
    fixCBLMarinerPermissions
    addMarinerNvidiaRepo
    overrideNetworkConfig || exit 1
    if grep -q "kata" <<< "$FEATURE_FLAGS"; then
      installKataDeps
      if [[ $OS == $MARINER_OS_NAME ]]; then
        enableMarinerKata
      fi
    fi
    disableTimesyncd
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
if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
  installed_version="$(echo ${containerd_manifest} | jq -r '.pinned."1804"')"
fi

containerd_version="$(echo "$installed_version" | cut -d- -f1)"
containerd_patch_version="$(echo "$installed_version" | cut -d- -f2)"
installStandaloneContainerd ${containerd_version} ${containerd_patch_version}
echo "  - [installed] containerd v${containerd_version}-${containerd_patch_version}" >> ${VHD_LOGS_FILEPATH}
stop_watch $capture_time "Create Containerd Service Directory, Download Shims, Configure Runtime and Network" false
start_watch

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
stop_watch $capture_time "Download Components, Determine / Download crictl Version" false
start_watch

installAndConfigureArtifactStreaming() {
  # arguments: package name, package extension
  PACKAGE_NAME=$1
  PACKAGE_EXTENSION=$2
  MIRROR_PROXY_VERSION='0.2.9'
  MIRROR_DOWNLOAD_PATH="./$1.$2"
  MIRROR_PROXY_URL="https://acrstreamingpackage.blob.core.windows.net/bin/${MIRROR_PROXY_VERSION}/${PACKAGE_NAME}.${PACKAGE_EXTENSION}"
  retrycmd_curl_file 10 5 60 $MIRROR_DOWNLOAD_PATH $MIRROR_PROXY_URL || exit ${ERR_ARTIFACT_STREAMING_DOWNLOAD}
  if [ "$2" == "deb" ]; then
    apt_get_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  elif [ "$2" == "rpm" ]; then
    dnf_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  fi
  rm $MIRROR_DOWNLOAD_PATH
}

UBUNTU_MAJOR_VERSION=$(echo $UBUNTU_RELEASE | cut -d. -f1)
# Artifact Streaming currently not supported for 24.04, the deb file isnt present in acs-mirror
# TODO(amaheshwari/aganeshkumar): Remove the conditional when Artifact Streaming is enabled for 24.04
if [ $OS == $UBUNTU_OS_NAME ] && [ $(isARM64)  != 1 ] && [ $UBUNTU_MAJOR_VERSION -ge 20 ] && [ ${UBUNTU_RELEASE} != "24.04" ]; then
  installAndConfigureArtifactStreaming acr-mirror-${UBUNTU_RELEASE//.} deb
fi

if [ $OS == $MARINER_OS_NAME ]  && [ $OS_VERSION == "2.0" ] && [ $(isARM64)  != 1 ]; then
  installAndConfigureArtifactStreaming acr-mirror-mariner rpm
fi

KUBERNETES_VERSION=$CRICTL_VERSIONS installCrictl || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT

# k8s will use images in the k8s.io namespaces - create it
ctr namespace create k8s.io
cliTool="ctr"

# also pre-download Teleportd plugin for containerd
downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.8.0"

INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
stop_watch $capture_time "Artifact Streaming, Download Containerd Plugins" false
start_watch

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
  gpu_action="copy"
  NVIDIA_DRIVER_IMAGE_SHA="sha-2d4c96"
  export NVIDIA_DRIVER_IMAGE_TAG="cuda-550.54.15-${NVIDIA_DRIVER_IMAGE_SHA}"

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

  cat << EOF >> ${VHD_LOGS_FILEPATH}
  - nvidia-driver=${NVIDIA_DRIVER_IMAGE_TAG}
EOF
fi

ls -ltr /opt/gpu/* >> ${VHD_LOGS_FILEPATH}

installBpftrace
echo "  - $(bpftrace --version)" >> ${VHD_LOGS_FILEPATH}

PRESENT_DIR=$(pwd)
# run installBcc in a subshell and continue on with container image pull in order to decrease total build time
(
  cd $PRESENT_DIR || { echo "Subshell in the wrong directory" >&2; exit 1; }

  installBcc

  exit $?
) > /var/log/bcc_installation.log 2>&1 &

BCC_PID=$!

echo "${CONTAINER_RUNTIME} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}
stop_watch $capture_time "Pull NVIDIA driver image (mcr), Start installBcc subshell" false
start_watch

string_replace() {
  echo ${1//\*/$2}
}

# Limit number of parallel pulls to 2 less than number of processor cores in order to prevent issues with network, CPU, and disk resources
# Account for possibility that number of cores is 3 or less
num_proc=$(nproc)
if [[ $num_proc -gt 3 ]]; then
  parallel_container_image_pull_limit=$(nproc --ignore=2)
else
  parallel_container_image_pull_limit=1
fi
echo "Limit for parallel container image pulls set to $parallel_container_image_pull_limit"

declare -a image_pids=()

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
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE} &
    image_pids+=($!)
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    while [[ $(jobs -p | wc -l) -ge $parallel_container_image_pull_limit ]]; do
      wait -n
    done
  done
done
wait ${image_pids[@]}

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
stop_watch $capture_time "Pull and Re-tag Container Images" false
start_watch

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
1.4.54
1.5.28
"


for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    unpackAzureCNI $VNET_CNI_PLUGINS_URL
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# After v0.7.6, URI was changed to renamed to https://acs-mirror.azureedge.net/cni-plugins/v*/binaries/cni-plugins-linux-arm64-v*.tgz
MULTI_ARCH_CNI_PLUGIN_VERSIONS="
1.4.1
"
CNI_PLUGIN_VERSIONS="${MULTI_ARCH_CNI_PLUGIN_VERSIONS}"

for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-${CPU_ARCH}-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    unpackAzureCNI $CNI_PLUGINS_URL
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# TODO: update comment
# IPv6 nftables rules are only available on Ubuntu or Mariner v2
if [[ $OS == $UBUNTU_OS_NAME || ( $OS == $MARINER_OS_NAME && $OS_VERSION == "2.0" ) || ( $OS == $AZURELINUX_OS_NAME && $OS_VERSION == "3.0" ) ]]; then
  systemctlEnableAndStart ipv6_nftables || exit 1
fi
stop_watch $capture_time "Configure Networking and Interface" false
start_watch

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
fi
stop_watch $capture_time "GPU Device plugin" false
start_watch

# Kubelet credential provider plugins
CREDENTIAL_PROVIDER_VERSIONS="
1.29.2
1.30.0
"
for CREDENTIAL_PROVIDER_VERSION in $CREDENTIAL_PROVIDER_VERSIONS; do
    CREDENTIAL_PROVIDER_DOWNLOAD_URL="https://acs-mirror.azureedge.net/cloud-provider-azure/v${CREDENTIAL_PROVIDER_VERSION}/binaries/azure-acr-credential-provider-linux-${CPU_ARCH}-v${CREDENTIAL_PROVIDER_VERSION}.tar.gz"
    downloadCredentalProvider $CREDENTIAL_PROVIDER_DOWNLOAD_URL
    echo "  - Kubelet credential provider version ${CREDENTIAL_PROVIDER_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

mkdir -p /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events

systemctlEnableAndStart cgroup-memory-telemetry.timer || exit 1
systemctl enable cgroup-memory-telemetry.service || exit 1
systemctl restart cgroup-memory-telemetry.service

CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then
  systemctlEnableAndStart cgroup-pressure-telemetry.timer || exit 1
  systemctl enable cgroup-pressure-telemetry.service || exit 1
  systemctl restart cgroup-pressure-telemetry.service
fi

cat /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*
rm -r /var/log/azure/Microsoft.Azure.Extensions.CustomScript || exit 1

# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.

# kube-proxy regular versions >=v1.17.0  hotfixes versions >= 20211009 are 'multi-arch'. All versions in kube-proxy-images.json are 'multi-arch' version now.

KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.containerdKubeProxyImages.ContainerImages[0].multiArchVersions[]' <"$THIS_DIR/kube-proxy-images.json")

declare -a kube_proxy_pids=()

for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBE_PROXY_IMAGE_VERSION}"
  pullContainerImage ${cliTool} ${CONTAINER_IMAGE} &
  kube_proxy_pids+=($!)
  while [[ $(jobs -p | wc -l) -ge $parallel_container_image_pull_limit ]]; do
      wait -n
  done
done
wait ${kube_proxy_pids[@]} # Wait for all parallel pulls to finish

for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBE_PROXY_IMAGE_VERSION}"
  ctr --namespace k8s.io run --rm ${CONTAINER_IMAGE} checkTask /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"

  # shellcheck disable=SC2181
  echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
done
stop_watch $capture_time "Configure Telemetry, Create Logging Directory, Kube-proxy" false
start_watch

# download kubernetes package from the given URL using MSI for auth for azcopy
# if it is a kube-proxy package, extract image from the downloaded package
cacheKubePackageFromPrivateUrl() {
  local kube_private_binary_url="$1"

  echo "process private package url: $kube_private_binary_url"

  mkdir -p ${K8S_PRIVATE_PACKAGES_CACHE_DIR} # /opt/kubernetes/downloads/private-packages

  # save kube pkg with version number from the url path, this convention is used to find the cached package at run-time
  local k8s_tgz_name
  k8s_tgz_name=$(echo "$kube_private_binary_url" | grep -o -P '(?<=\/kubernetes\/).*(?=\/binaries\/)').tar.gz

  # use azcopy with MSI instead of curl to download packages
  getAzCopyCurrentPath

  export AZCOPY_AUTO_LOGIN_TYPE="MSI"
  export AZCOPY_MSI_RESOURCE_STRING="$LINUX_MSI_RESOURCE_IDS"

  cached_pkg="${K8S_PRIVATE_PACKAGES_CACHE_DIR}/${k8s_tgz_name}"
  echo "download private package ${kube_private_binary_url} and store as ${cached_pkg}"
  if ! ./azcopy copy "${kube_private_binary_url}" "${cached_pkg}"; then
    exit $ERR_PRIVATE_K8S_PKG_ERR
  fi
}

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

wait $BCC_PID
BCC_EXIT_CODE=$?

if [ $BCC_EXIT_CODE -eq 0 ]; then
  echo "Bcc tools successfully installed."
  cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF
else
  echo "Error: installBcc subshell failed with exit code $BCC_EXIT_CODE" >&2
fi

# use the private_packages_url to download and cache packages
if [[ -n ${PRIVATE_PACKAGES_URL} ]]; then
  IFS=',' read -ra PRIVATE_URLS <<< "${PRIVATE_PACKAGES_URL}"

  for private_url in "${PRIVATE_URLS[@]}"; do
    echo "download kube package from ${private_url}"
    cacheKubePackageFromPrivateUrl "$private_url"
  done
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
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz" false
done

rm -f ./azcopy # cleanup immediately after usage will return in two downloads
stop_watch $capture_time "Download and Process Kubernetes Packages / Extract Binaries" false

echo "install-dependencies step completed successfully"
stop_watch $capture_script_start "install-dependencies.sh" true
show_benchmarks
