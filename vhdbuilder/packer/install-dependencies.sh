#!/bin/bash
set -euo pipefail
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
MARINER_KATA_OS_NAME="MARINERKATA"
AZURELINUX_OS_NAME="AZURELINUX"

# Real world examples from the command outputs
# For Azure Linux V3: ID=azurelinux VERSION_ID="3.0"
# For Azure Linux V2: ID=mariner VERSION_ID="2.0"
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
source /home/packer/provision_source_benchmarks.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
PERFORMANCE_DATA_FILE=/opt/azure/vhd-build-performance-data.json
resolve_packages_source_url

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}
capture_benchmark "${SCRIPT_NAME}_source_packer_files_and_declare_variables"

echo "Logging the kernel after purge and reinstall + reboot: $(uname -r)"
# fix grub issue with cvm by reinstalling before other deps
# other VHDs use grub-pc, not grub-efi
if [ "$OS" = "$UBUNTU_OS_NAME" ] && echo "$FEATURE_FLAGS" | grep -q "cvm"; then
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  wait_for_apt_locks
  apt_get_install 30 1 600 grub-efi || exit 1
fi
capture_benchmark "${SCRIPT_NAME}_reinstall_grub_for_cvm"

if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
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

# If the IMG_SKU does not contain "minimal", installDeps normally
# shellcheck disable=SC3010
if [[ "$IMG_SKU" != *"minimal"* ]]; then
  installDeps
else
  updateAptWithMicrosoftPkg
  # The following packages are required for an Ubuntu Minimal Image to build and successfully run CSE
  # blobfuse2 and fuse3 - ubuntu 22.04 supports blobfuse2 and is fuse3 compatible
  BLOBFUSE2_VERSION="2.4.1"
  if [ "${OS_VERSION}" = "18.04" ]; then
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
if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
  if [ "${OS_VERSION}" = "18.04" ]; then
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
capture_benchmark "${SCRIPT_NAME}_install_deps_and_set_configs"

if [ "${CONTAINER_RUNTIME:-}" != "containerd" ]; then
  echo "Unsupported container runtime. Only containerd is supported for new VHD builds."
  exit 1
fi

if [ "$(isARM64)" -eq 1 ]; then
  # shellcheck disable=SC3010
  if [[ ${HYPERV_GENERATION,,} == "v1" ]]; then
    echo "No arm64 support on V1 VM, exiting..."
    exit 1
  fi

  if [ "${CONTAINER_RUNTIME,,}" = "docker" ]; then
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
capture_benchmark "${SCRIPT_NAME}_validate_container_runtime_and_override_ubuntu_net_config"

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
capture_benchmark "${SCRIPT_NAME}_set_ip_forwarding"

echo "set read ahead size to 15380 KB"
AWK_PATH=$(command -v awk)
cat > /etc/udev/rules.d/99-nfs.rules <<EOF
SUBSYSTEM=="bdi", ACTION=="add", PROGRAM="$AWK_PATH -v bdi=\$kernel 'BEGIN{ret=1} {if (\$4 == bdi){ret=0}} END{exit ret}' /proc/fs/nfsfs/volumes", ATTR{read_ahead_kb}="15380"
EOF

echo "install udev rules for v6 vm sku"
cat > /etc/udev/rules.d/80-azure-disk.rules <<EOF
ACTION!="add|change", GOTO="azure_disk_end"
SUBSYSTEM!="block", GOTO="azure_disk_end"

KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="Microsoft NVMe Direct Disk", GOTO="azure_disk_nvme_direct_v1"
KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="Microsoft NVMe Direct Disk v2", GOTO="azure_disk_nvme_direct_v2"
KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="MSFT NVMe Accelerator v1.0", GOTO="azure_disk_nvme_remote_v1"
ENV{ID_VENDOR}=="Msft", ENV{ID_MODEL}=="Virtual_Disk", GOTO="azure_disk_scsi"
GOTO="azure_disk_end"

LABEL="azure_disk_scsi"
ATTRS{device_id}=="?00000000-0000-*", ENV{AZURE_DISK_TYPE}="os", GOTO="azure_disk_symlink"
ENV{DEVTYPE}=="partition", PROGRAM="/bin/sh -c 'readlink /sys/class/block/%k/../device|cut -d: -f4'", ENV{AZURE_DISK_LUN}="\$result"
ENV{DEVTYPE}=="disk", PROGRAM="/bin/sh -c 'readlink /sys/class/block/%k/device|cut -d: -f4'", ENV{AZURE_DISK_LUN}="\$result"
ATTRS{device_id}=="{f8b3781a-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_LUN}=="0", ENV{AZURE_DISK_TYPE}="os", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781b-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781c-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781d-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"

# Use "resource" type for local SCSI because some VM skus offer NVMe local disks in addition to a SCSI resource disk, e.g. LSv3 family.
# This logic is already in walinuxagent rules but we duplicate it here to avoid an unnecessary dependency for anyone requiring it.
ATTRS{device_id}=="?00000000-0001-*", ENV{AZURE_DISK_TYPE}="resource", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781a-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_LUN}=="1", ENV{AZURE_DISK_TYPE}="resource", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
GOTO="azure_disk_end"

LABEL="azure_disk_nvme_direct_v1"
LABEL="azure_disk_nvme_direct_v2"
ATTRS{nsid}=="?*", ENV{AZURE_DISK_TYPE}="local", ENV{AZURE_DISK_SERIAL}="\$env{ID_SERIAL_SHORT}"
GOTO="azure_disk_nvme_id"

LABEL="azure_disk_nvme_remote_v1"
ATTRS{nsid}=="1", ENV{AZURE_DISK_TYPE}="os", GOTO="azure_disk_nvme_id"
ATTRS{nsid}=="?*", ENV{AZURE_DISK_TYPE}="data", PROGRAM="/bin/sh -ec 'echo \$\$((%s{nsid}-2))'", ENV{AZURE_DISK_LUN}="\$result"

LABEL="azure_disk_nvme_id"
ATTRS{nsid}=="?*", IMPORT{program}="/usr/sbin/azure-nvme-id --udev"

LABEL="azure_disk_symlink"
# systemd v254 ships an updated 60-persistent-storage.rules that would allow
# these to be deduplicated using \$env{.PART_SUFFIX}
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="os|resource|root", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_INDEX}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-index/\$env{AZURE_DISK_INDEX}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_LUN}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-lun/\$env{AZURE_DISK_LUN}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_NAME}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-name/\$env{AZURE_DISK_NAME}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_SERIAL}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-serial/\$env{AZURE_DISK_SERIAL}"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="os|resource|root", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_INDEX}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-index/\$env{AZURE_DISK_INDEX}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_LUN}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-lun/\$env{AZURE_DISK_LUN}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_NAME}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-name/\$env{AZURE_DISK_NAME}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_SERIAL}=="?*", SYMLINK+="disk/azure/\$env{AZURE_DISK_TYPE}/by-serial/\$env{AZURE_DISK_SERIAL}-part%n"
LABEL="azure_disk_end"
EOF
udevadm control --reload
capture_benchmark "${SCRIPT_NAME}_set_udev_rules"

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
capture_benchmark "${SCRIPT_NAME}_handle_azurelinux_configs"

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

downloadAndInstallCriTools() {
  downloadDir=${1}
  evaluatedURL=${2}
  version=${3}

  # if downloadDir and evaluatedURL are not empty, download and install crictl by this override, which is the old way to install
  if [ ! -z "${downloadDir}" ] && [ ! -z "${evaluatedURL}" ]; then
    downloadCrictl "${downloadDir}" "${evaluatedURL}"
    echo "  - crictl version ${version}" >> ${VHD_LOGS_FILEPATH}
    # other steps are dependent on CRICTL_VERSION and CRICTL_VERSIONS
    # since we only have 1 entry in CRICTL_VERSIONS, we simply set both to the same value
    CRICTL_VERSION=${version} 
    KUBERNETES_VERSION=$CRICTL_VERSION installCrictl || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT
    return 0
  fi

  # this will call installCriCtlPackage function defined in cse_install_<OS>.sh based on the OS
  installCriCtlPackage "${version}"
}

echo "VHD will be built with containerd as the container runtime"
if [ "${OS}" = "${UBUNTU_OS_NAME}" ]; then
  updateAptWithMicrosoftPkg
  capture_benchmark "${SCRIPT_NAME}_update_apt_with_msft_pkg"
fi

# check if COMPONENTS_FILEPATH exists
if [ ! -f "$COMPONENTS_FILEPATH" ]; then
  echo "Components file not found at $COMPONENTS_FILEPATH. Exiting..."
  exit 1
fi

packages=$(jq ".Packages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
# Iterate over each element in the packages array
while IFS= read -r p; do
  #getting metadata for each package
  name=$(echo "${p}" | jq .name -r)
  PACKAGE_VERSIONS=()
  os=${OS}
  if isMarinerOrAzureLinux "${OS}" && [ "${IS_KATA}" = "true" ]; then
    os=${MARINER_KATA_OS_NAME}
  fi
  updatePackageVersions "${p}" "${os}" "${OS_VERSION}"
  PACKAGE_DOWNLOAD_URL=""
  updatePackageDownloadURL "${p}" "${os}" "${OS_VERSION}"
  echo "In components.json, processing components.packages \"${name}\" \"${PACKAGE_VERSIONS[@]}\" \"${PACKAGE_DOWNLOAD_URL}\""

  # if ${PACKAGE_VERSIONS[@]} count is 0 or if the first element of the array is <SKIP>, then skip and move on to next package
  if [ "${#PACKAGE_VERSIONS[@]}" -eq 0 ] || [ "${PACKAGE_VERSIONS[0]}" = "<SKIP>" ]; then
    echo "INFO: ${name} package versions array is either empty or the first element is <SKIP>. Skipping ${name} installation."
    continue
  fi
  downloadDir=$(echo "${p}" | jq .downloadLocation -r)
  #download the package
  case $name in
    "kubernetes-cri-tools")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadAndInstallCriTools "${downloadDir}" "${evaluatedURL}" "${version}"
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
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ]; then
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
      done
      ;;
    "azure-acr-credential-provider")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadCredentialProvider "${downloadDir}" "${evaluatedURL}" "${version}"
        echo "  - azure-acr-credential-provider version ${version}" >> ${VHD_LOGS_FILEPATH}
        # ORAS will be used to install other packages for network isolated clusters, it must go first.
      done
      ;;
    "containerd-wasm-shims")
      installContainerdWasmShims "${downloadDir}" "${PACKAGE_DOWNLOAD_URL}" "${PACKAGE_VERSIONS[@]}"
      echo "  - containerd-wasm-shims version ${PACKAGE_VERSIONS[@]}" >> ${VHD_LOGS_FILEPATH}
      ;;
    "spinkube")
      installSpinKube "${downloadDir}" "${PACKAGE_DOWNLOAD_URL}" "${PACKAGE_VERSIONS[@]}"
      echo "  - spinkube version ${PACKAGE_VERSIONS[@]}" >> ${VHD_LOGS_FILEPATH}
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
  capture_benchmark "${SCRIPT_NAME}_download_${name}"
done <<< "$packages"

installAndConfigureArtifactStreaming() {
  # arguments: package name, package extension
  PACKAGE_NAME=$1
  PACKAGE_EXTENSION=$2
  MIRROR_PROXY_VERSION='0.2.11'
  MIRROR_DOWNLOAD_PATH="./$1.$2"
  MIRROR_PROXY_URL="https://acrstreamingpackage.blob.core.windows.net/bin/${MIRROR_PROXY_VERSION}/${PACKAGE_NAME}.${PACKAGE_EXTENSION}"
  retrycmd_curl_file 10 5 60 $MIRROR_DOWNLOAD_PATH $MIRROR_PROXY_URL || exit ${ERR_ARTIFACT_STREAMING_DOWNLOAD}
  if [ "$2" = "deb" ]; then
    apt_get_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  elif [ "$2" = "rpm" ]; then
    dnf_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  fi
  rm $MIRROR_DOWNLOAD_PATH
}

UBUNTU_MAJOR_VERSION=$(echo $UBUNTU_RELEASE | cut -d. -f1)
# Artifact Streaming currently not supported for 24.04, the deb file isnt present in acs-mirror
# TODO(amaheshwari/aganeshkumar): Remove the conditional when Artifact Streaming is enabled for 24.04
if [ "$OS" = "$UBUNTU_OS_NAME" ] && [ "$(isARM64)" -ne 1 ] && [ "$UBUNTU_MAJOR_VERSION" -ge 20 ] && [ "${UBUNTU_RELEASE}" != "24.04" ]; then
  installAndConfigureArtifactStreaming acr-mirror-${UBUNTU_RELEASE//.} deb
fi

# TODO(aadagarwal): Enable Artifact Streaming for AzureLinux 3.0
if [ "$OS" = "$MARINER_OS_NAME" ]  && [ "$OS_VERSION" = "2.0" ] && [ "$(isARM64)"  -ne 1 ]; then
  installAndConfigureArtifactStreaming acr-mirror-mariner rpm
fi

# k8s will use images in the k8s.io namespaces - create it
ctr namespace create k8s.io
cliTool="ctr"


INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
capture_benchmark "${SCRIPT_NAME}_configure_artifact_streaming_and_install_crictl"

GPUContainerImages=$(jq  -c '.GPUContainerImages[]' $COMPONENTS_FILEPATH)

NVIDIA_DRIVER_IMAGE=""
NVIDIA_DRIVER_IMAGE_TAG=""

if [ $OS = $UBUNTU_OS_NAME ] && [ "$(isARM64)" -ne 1 ]; then  # No ARM64 SKU with GPU now
  gpu_action="copy"

  while IFS= read -r imageToBePulled; do
    downloadURL=$(echo "${imageToBePulled}" | jq -r '.downloadURL')
    # shellcheck disable=SC2001
    imageName=$(echo "$downloadURL" | sed 's/:.*$//')

    if [ "$imageName" = "mcr.microsoft.com/aks/aks-gpu-cuda" ]; then
      latestVersion=$(echo "${imageToBePulled}" | jq -r '.gpuVersion.latestVersion')
      NVIDIA_DRIVER_IMAGE="$imageName"
      NVIDIA_DRIVER_IMAGE_TAG="$latestVersion"
      break  # Exit the loop once we find the image
    fi
  done <<< "$GPUContainerImages"

  # Check if the NVIDIA_DRIVER_IMAGE and NVIDIA_DRIVER_IMAGE_TAG were found
  if [ -z "$NVIDIA_DRIVER_IMAGE" ] || [ -z "$NVIDIA_DRIVER_IMAGE_TAG" ]; then
    echo "Error: Unable to find aks-gpu-cuda image in components.json"
    exit 1
  fi

  mkdir -p /opt/{actions,gpu}

  ctr -n k8s.io image pull "$NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG"

    cat << EOF >> ${VHD_LOGS_FILEPATH}
  - nvidia-driver=${NVIDIA_DRIVER_IMAGE_TAG}
EOF

fi

if [ -d "/opt/gpu" ] && [ "$(ls -A /opt/gpu)" ]; then
  ls -ltr /opt/gpu/* >> ${VHD_LOGS_FILEPATH}
fi

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
capture_benchmark "${SCRIPT_NAME}_pull_nvidia_driver_and_start_ebpf_downloads"

string_replace() {
  echo ${1//\*/$2}
}

# Limit number of parallel pulls to 2 less than number of processor cores in order to prevent issues with network, CPU, and disk resources
# Account for possibility that number of cores is 3 or less
num_proc=$(nproc)
if [ "$num_proc" -gt 3 ]; then
  parallel_container_image_pull_limit=$(nproc --ignore=2)
else
  parallel_container_image_pull_limit=1
fi
echo "Limit for parallel container image pulls set to $parallel_container_image_pull_limit"

declare -a image_pids=()

ContainerImages=$(jq ".ContainerImages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
while IFS= read -r imageToBePulled; do
  downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
  amd64OnlyVersionsStr=$(echo "${imageToBePulled}" | jq .amd64OnlyVersions -r)
  MULTI_ARCH_VERSIONS=()
  updateMultiArchVersions "${imageToBePulled}"
  amd64OnlyVersions=""
  if [ "${amd64OnlyVersionsStr}" != "null" ]; then
    amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
  fi

  if [ "$(isARM64)" -eq 1 ]; then
    versions="${MULTI_ARCH_VERSIONS[*]}"
  else
    versions="${amd64OnlyVersions} ${MULTI_ARCH_VERSIONS[*]}"
  fi

  for version in ${versions}; do
    CONTAINER_IMAGE=$(string_replace $downloadURL $version)
    pullContainerImage "${cliTool}" "${CONTAINER_IMAGE}" &
    image_pids+=($!)
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    while [ "$(jobs -p | wc -l)" -ge "$parallel_container_image_pull_limit" ]; do
      wait -n || { 
        ret=$?
        echo "A background job pullContainerImage failed: ${ret}, ${CONTAINER_IMAGE}. Exiting..." >&2
        for pid in "${image_pids[@]}"; do
          kill -9 "$pid" 2>/dev/null || echo "Failed to kill process $pid"
        done
        exit "${ret}"
    }
    done
  done
done <<< "$ContainerImages"
echo "Waiting for container image pulls to finish. PID: ${image_pids[@]}"
wait ${image_pids[@]}

watcher=$(jq '.ContainerImages[] | select(.downloadURL | contains("aks-node-ca-watcher"))' $COMPONENTS_FILEPATH)
watcherBaseImg=$(echo $watcher | jq -r .downloadURL)
watcherVersion=$(echo $watcher | jq -r .multiArchVersionsV2[0].latestVersion)
watcherFullImg=${watcherBaseImg//\*/$watcherVersion}

# this image will never get pulled, the tag must be the same across different SHAs.
# it will only ever be upgraded via node image changes.
# we do this because the image is used to bootstrap custom CA trust when MCR egress
# may be intercepted by an untrusted TLS MITM firewall.
watcherStaticImg=${watcherBaseImg//\*/static}

# can't use cliTool because crictl doesn't support retagging.
retagContainerImage "ctr" ${watcherFullImg} ${watcherStaticImg}

# IPv6 nftables rules are only available on Ubuntu or Mariner/AzureLinux
if [ $OS = $UBUNTU_OS_NAME ] || isMarinerOrAzureLinux "$OS"; then
  systemctlEnableAndStart ipv6_nftables 30 || exit 1
fi
capture_benchmark "${SCRIPT_NAME}_pull_and_retag_container_images"

mkdir -p /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events

# Disable cgroup-memory-telemetry on AzureLinux due to incompatibility with cgroup2fs driver and absence of required azure.slice directory
if ! isMarinerOrAzureLinux "$OS"; then
  systemctlEnableAndStart cgroup-memory-telemetry.timer 30 || exit 1
  systemctl enable cgroup-memory-telemetry.service || exit 1
  systemctl restart cgroup-memory-telemetry.service
fi

CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then
  systemctlEnableAndStart cgroup-pressure-telemetry.timer 30 || exit 1
  systemctl enable cgroup-pressure-telemetry.service || exit 1
  systemctl restart cgroup-pressure-telemetry.service
fi

if [ -d "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/" ] && [ "$(ls -A /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/)" ]; then
  cat /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*
  rm -r /var/log/azure/Microsoft.Azure.Extensions.CustomScript || exit 1
fi
capture_benchmark "${SCRIPT_NAME}_configure_telemetry"

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

if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
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
capture_benchmark "${SCRIPT_NAME}_purge_and_update_ubuntu"

wait $BCC_PID
BCC_EXIT_CODE=$?
chmod 644 /var/log/bcc_installation.log

if [ "$BCC_EXIT_CODE" -eq 0 ]; then
  echo "Bcc tools successfully installed."
  cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF
else
  echo "Error: installBcc subshell failed with exit code $BCC_EXIT_CODE" >&2
  exit $BCC_EXIT_CODE
fi
capture_benchmark "${SCRIPT_NAME}_finish_installing_bcc_tools"

# use the private_packages_url to download and cache packages
if [ -n "${PRIVATE_PACKAGES_URL}" ]; then
  IFS=',' read -ra PRIVATE_URLS <<< "${PRIVATE_PACKAGES_URL}"

  for private_url in "${PRIVATE_URLS[@]}"; do
    echo "download kube package from ${private_url}"
    cacheKubePackageFromPrivateUrl "$private_url"
  done
fi

LOCALDNS_BINARY_PATH="/opt/azure/containers/localdns/binary"
# This function extracts CoreDNS binary from cached coredns images (n-1 image version and latest revision version)
# and copies it to - /opt/azure/containers/localdns/binary/coredns.
# The binary is later used by localdns systemd unit.
# The function also handles the cleanup of temporary directories and unmounting of images.
extractAndCacheCoreDnsBinary() {
  local coredns_image_list=($(ctr -n k8s.io images list -q | grep coredns))
  if [ "${#coredns_image_list[@]}" -eq 0 ]; then
    echo "Error: No coredns images found."
    exit 1
  fi

  rm -rf "${LOCALDNS_BINARY_PATH}" || exit 1
  mkdir -p "${LOCALDNS_BINARY_PATH}" || exit 1

  cleanup_coredns_imports() {
    set +e
    if [ -n "${ctr_temp}" ]; then
      ctr -n k8s.io images unmount "${ctr_temp}" >/dev/null
      rm -rf "${ctr_temp}"
    fi
  }
  trap cleanup_coredns_imports EXIT ABRT ERR INT PIPE QUIT TERM

  # Extract available coredns image tags (v1.12.0-1 format) and sort them in descending order.
  local sorted_coredns_tags=($(for image in "${coredns_image_list[@]}"; do echo "${image##*:}"; done | sort -V -r))

  # Function to check version format (vMajor.Minor.Patch).
  validate_version_format() {
    local version=$1
    # shellcheck disable=SC3010
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "Error: Invalid coredns version format. Expected vMajor.Minor.Patch, got $version" >> "${VHD_LOGS_FILEPATH}"
      return 1
    fi
    return 0
  }

  # Determine latest version (eg. v1.12.0-1).
  local latest_coredns_tag="${sorted_coredns_tags[0]}"
  # Extract major.minor.patch (removes -revision. eg - v1.12.0).
  local latest_vMajorMinorPatch="${latest_coredns_tag%-*}"

  local previous_coredns_tag=""
  # Iterate through the sorted list to find the next highest major-minor version.
  for tag in "${sorted_coredns_tags[@]}"; do
    # Extract major.minor.patch (eg - v1.12.0).
    local vMajorMinorPatch="${tag%-*}"
    if ! validate_version_format "$vMajorMinorPatch"; then
      exit 1
    fi

    if [ "${vMajorMinorPatch}" != "${latest_vMajorMinorPatch}" ]; then
      previous_coredns_tag="$tag"
      # Break the loop after next highest major-minor version is found.
      break
    fi
  done

  if [ -z "${previous_coredns_tag}" ]; then
    echo "Warning: Previous version not found, using the latest version: $latest_coredns_tag" >> "${VHD_LOGS_FILEPATH}"
    previous_coredns_tag="$latest_coredns_tag"
  fi

  # Extract the CoreDNS binary for the selected version.
  for coredns_image_url in "${coredns_image_list[@]}"; do
    if [ "${coredns_image_url##*:}" != "${previous_coredns_tag}" ]; then
      continue
    fi

    ctr_temp="$(mktemp -d)"
    local max_retries=3
    local retry_count=0
    while [ $retry_count -lt $max_retries ]; do
      if ctr -n k8s.io images mount "${coredns_image_url}" "${ctr_temp}" >/dev/null; then
        break
      fi
      echo "Warning: Failed to mount ${coredns_image_url}, retrying..." >> "${VHD_LOGS_FILEPATH}"
      sleep 2
      ((retry_count++))
    done

    if [ "$retry_count" -eq "$max_retries" ]; then
      echo "Error: Failed to mount ${coredns_image_url} after ${max_retries} attempts." >> "${VHD_LOGS_FILEPATH}"
      exit 1
    fi

    local coredns_binary="${ctr_temp}/usr/bin/coredns"
    if [ -f "${coredns_binary}" ]; then
      cp "${coredns_binary}" "${LOCALDNS_BINARY_PATH}/coredns" || {
        echo "Error: Failed to copy coredns binary of ${previous_coredns_tag}" >> "${VHD_LOGS_FILEPATH}"
        exit 1
      }
      echo "Successfully copied coredns binary of ${previous_coredns_tag}" >> "${VHD_LOGS_FILEPATH}"
    else
      echo "Coredns binary not found for ${coredns_image_url}" >> "${VHD_LOGS_FILEPATH}"
    fi

    ctr -n k8s.io images unmount "${ctr_temp}" >/dev/null
    rm -rf "${ctr_temp}"
  done

  # Clear the trap.
  trap - EXIT ABRT ERR INT PIPE QUIT TERM
}

extractAndCacheCoreDnsBinary

rm -f ./azcopy # cleanup immediately after usage will return in two downloads
echo "install-dependencies step completed successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
