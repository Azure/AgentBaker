#!/bin/bash

script_start_stopwatch=$(date +%s)
section_start_stopwatch=$(date +%s)
SCRIPT_NAME=$(basename $0 .sh)
SCRIPT_NAME="${SCRIPT_NAME//-/_}"
declare -A benchmarks=()
declare -a benchmarks_order=()

UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
MARINER_KATA_OS_NAME="MARINERKATA"
AZURELINUX_OS_NAME="AZURELINUX"

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
IS_KATA="false"
if grep -q "kata" <<< "$FEATURE_FLAGS"; then
  IS_KATA="true"
fi
  
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')

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
VHD_BUILD_PERF_DATA=/opt/azure/vhd-build-performance-data.json

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}
capture_benchmark "declare_variables_and_source_packer_files"

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
capture_benchmark "purge_and_reinstall_ubuntu"

# If the IMG_SKU does not contain "minimal", installDeps normally
if [[ "$IMG_SKU" != *"minimal"* ]]; then
  installDeps
else
  updateAptWithMicrosoftPkg
  # The following packages are required for an Ubuntu Minimal Image to build and successfully run CSE
  # blobfuse2 and fuse3 - ubuntu 22.04 supports blobfuse2 and is fuse3 compatible
  BLOBFUSE2_VERSION="2.3.2"
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

CHRONYD_DIR=/etc/systemd/system/chronyd.service.d
if [[ "$OS" == "$UBUNTU_OS_NAME" ]]; then
  if [ "${OS_VERSION}" == "18.04" ]; then
    CHRONYD_DIR=/etc/systemd/system/chrony.service.d
  fi
fi

mkdir -p "${CHRONYD_DIR}"
cat >> "${CHRONYD_DIR}"/10-chrony-restarts.conf <<EOF
[Service]
Restart=always
RestartSec=5
EOF

tee -a /etc/systemd/journald.conf > /dev/null <<'EOF'
Storage=persistent
SystemMaxUse=1G
RuntimeMaxUse=1G
ForwardToSyslog=yes
EOF
capture_benchmark "install_dependencies"

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
if ! isMarinerOrAzureLinux "$OS"; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi
capture_benchmark "check_container_runtime_and_network_configurations"

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

if isMarinerOrAzureLinux "$OS"; then
    disableSystemdResolvedCache
    disableSystemdIptables || exit 1
    setMarinerNetworkdConfig
    fixCBLMarinerPermissions
    addMarinerNvidiaRepo
    overrideNetworkConfig || exit 1
    if grep -q "kata" <<< "$FEATURE_FLAGS"; then
      installKataDeps
      enableMarinerKata
    fi
    disableTimesyncd
    disableDNFAutomatic
    enableCheckRestart
    activateNfConntrack
fi

# doing this at vhd allows CSE to be faster with just mv 
unpackTgzToCNIDownloadsDIR() {
  local URL=$1
  CNI_TGZ_TMP=${URL##*/}
  CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}
  mkdir "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}"
  tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_DOWNLOADS_DIR/$CNI_DIR_TMP
  rm -rf ${CNI_DOWNLOADS_DIR:?}/${CNI_TGZ_TMP}
  echo "  - Ran tar -xzf on the CNI downloaded then rm -rf to clean up"
}

#this is the reference cni it is only ever downloaded in caching for build not at provisioning time
#but conceptually it is very similiar to downloadAzureCNI in that it takes a url and puts in CNI_DOWNLOADS_DIR
downloadCNI() {
    downloadDir=${1}
    mkdir -p $downloadDir
    CNI_PLUGINS_URL=${2}
    cniTgzTmp=${CNI_PLUGINS_URL##*/}
    retrycmd_get_tarball 120 5 "$downloadDir/${cniTgzTmp}" ${CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}


downloadContainerdWasmShims
echo "  - containerd-wasm-shims ${CONTAINERD_WASM_VERSIONS}" >> ${VHD_LOGS_FILEPATH}

echo "VHD will be built with containerd as the container runtime"
updateAptWithMicrosoftPkg
capture_benchmark "create_containerd_service_directory_download_shims_configure_runtime_and_network"

packages=$(jq ".Packages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for p in ${packages[*]}; do
  #getting metadata for each package
  name=$(echo "${p}" | jq .name -r)
  PACKAGE_VERSIONS=()
  os=${OS}
  if [[ "${OS}" == "${MARINER_OS_NAME}" && "${IS_KATA}" == "true" ]]; then
    os=${MARINER_KATA_OS_NAME}
  fi
  returnPackageVersions ${p} ${os} ${OS_VERSION}
  PACKAGE_DOWNLOAD_URL=""
  returnPackageDownloadURL ${p} ${os} ${OS_VERSION}
  echo "In components.json, processing components.packages \"${name}\" \"${PACKAGE_VERSIONS[@]}\" \"${PACKAGE_DOWNLOAD_URL}\""
  
  # if ${PACKAGE_VERSIONS[@]} count is 0 or if the first element of the array is <SKIP>, then skip and move on to next package
  if [[ ${#PACKAGE_VERSIONS[@]} -eq 0 || ${PACKAGE_VERSIONS[0]} == "<SKIP>" ]]; then
    echo "INFO: ${name} package versions array is either empty or the first element is <SKIP>. Skipping ${name} installation."
    continue
  fi
  downloadDir=$(echo ${p} | jq .downloadLocation -r)
  #download the package
  case $name in
    "cri-tools")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadCrictl "${downloadDir}" "${evaluatedURL}"
        echo "  - crictl version ${version}" >> ${VHD_LOGS_FILEPATH}
        # other steps are dependent on CRICTL_VERSION and CRICTL_VERSIONS
        # since we only have 1 entry in CRICTL_VERSIONS, we simply set both to the same value
        CRICTL_VERSION=${version} 
        CRICTL_VERSIONS=${version}
      done
      ;;
    "azure-cni")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadAzureCNI "${downloadDir}" "${evaluatedURL}"
        unpackTgzToCNIDownloadsDIR "${evaluatedURL}" #alternatively we could put thus directly in CNI_BIN_DIR to avoid provisioing time move
        echo "  - Azure CNI version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "cni-plugins")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadCNI "${downloadDir}" "${evaluatedURL}"
        unpackTgzToCNIDownloadsDIR "${evaluatedURL}"
        echo "  - CNI plugin version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "runc")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        ensureRunc "${version}" "${evaluatedURL}" "${downloadDir}"
        echo "  - runc version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "containerd")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        if [[ "${OS}" == "${UBUNTU_OS_NAME}" ]]; then
          installContainerd "${downloadDir}" "${evaluatedURL}" "${version}"
        elif isMarinerOrAzureLinux "$OS"; then
          installStandaloneContainerd "${version}"
        fi
        echo "  - containerd version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "oras")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        installOras "${downloadDir}" "${evaluatedURL}" "${version}"
        echo "  - oras version ${version}" >> ${VHD_LOGS_FILEPATH}
        # ORAS will be used to install other packages for network isolated clusters, it must go first.
      done
      ;;
    "kubernetes-binaries")
      # kubelet and kubectl
      # need to cover previously supported version for VMAS scale up scenario
      # So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
      # NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
      # Please do not use the .1 suffix, because that's only for the base image patches
      # regular version >= v1.17.0 or hotfixes >= 20211009 has arm64 binaries.
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        extractKubeBinaries "${version}" "${evaluatedURL}" false "${downloadDir}"
        echo "  - kubernetes-binaries version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    *)
      echo "Package name: ${name} not supported for download. Please implement the download logic in the script."
      # We can add a common function to download a generic package here.
      # However, installation could be different for different packages.
      ;;
  esac
  capture_benchmark "download_${name}"
done

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

# TODO(aadagarwal): Enable Artifact Streaming for AzureLinux 3.0
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
capture_benchmark "artifact_streaming_and_download_teleportd"

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
  gpu_action="copy"
  NVIDIA_DRIVER_IMAGE_SHA="sha-b40b85"
  export NVIDIA_DRIVER_IMAGE_TAG="cuda-550.90.07-${NVIDIA_DRIVER_IMAGE_SHA}"

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
capture_benchmark "pull_nvidia_driver_image_and_run_installBcc_in_subshell"

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
capture_benchmark "pull_and_retag_container_images"

# IPv6 nftables rules are only available on Ubuntu or Mariner/AzureLinux
if [[ $OS == $UBUNTU_OS_NAME ]] || isMarinerOrAzureLinux "$OS"; then
  systemctlEnableAndStart ipv6_nftables || exit 1
fi
capture_benchmark "configure_networking_and_interface"

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
NVIDIA_DEVICE_PLUGIN_VERSION="v0.14.5"

DEVICE_PLUGIN_CONTAINER_IMAGE="mcr.microsoft.com/oss/nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
pullContainerImage ${cliTool} ${DEVICE_PLUGIN_CONTAINER_IMAGE}
echo "  - ${DEVICE_PLUGIN_CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}

# GPU device plugin
if grep -q "fullgpu" <<< "$FEATURE_FLAGS" && grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
  kubeletDevicePluginPath="/var/lib/kubelet/device-plugins"
  mkdir -p $kubeletDevicePluginPath
  echo "  - $kubeletDevicePluginPath" >> ${VHD_LOGS_FILEPATH}

  DEST="/usr/local/nvidia/bin"
  mkdir -p $DEST
  ctr --namespace k8s.io run --rm --mount type=bind,src=${DEST},dst=${DEST},options=bind:rw --cwd ${DEST} $DEVICE_PLUGIN_CONTAINER_IMAGE plugingextract /bin/sh -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  chmod a+x $DEST/nvidia-device-plugin
  echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
  ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

  systemctlEnableAndStart nvidia-device-plugin || exit 1
  ctr --namespace k8s.io images rm $DEVICE_PLUGIN_CONTAINER_IMAGE || exit 1
fi
fi
capture_benchmark "download_gpu_device_plugin"

# Kubelet credential provider plugins
CREDENTIAL_PROVIDER_VERSIONS="
1.29.2
1.30.0
"
for CREDENTIAL_PROVIDER_VERSION in $CREDENTIAL_PROVIDER_VERSIONS; do
    downloadCredentialProvider
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


capture_benchmark "configure_telemetry_create_logging_directory"

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

  ./azcopy login --login-type=MSI

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
capture_benchmark "finish_installing_bcc_tools"

# use the private_packages_url to download and cache packages
if [[ -n ${PRIVATE_PACKAGES_URL} ]]; then
  IFS=',' read -ra PRIVATE_URLS <<< "${PRIVATE_PACKAGES_URL}"

  for private_url in "${PRIVATE_URLS[@]}"; do
    echo "download kube package from ${private_url}"
    cacheKubePackageFromPrivateUrl "$private_url"
  done
fi

rm -f ./azcopy # cleanup immediately after usage will return in two downloads
echo "install-dependencies step completed successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
