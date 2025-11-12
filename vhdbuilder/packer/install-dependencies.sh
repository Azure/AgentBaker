#!/bin/bash
set -euo pipefail

K8S_DEVICE_PLUGIN_PKG="${K8S_DEVICE_PLUGIN_PKG:-nvidia-device-plugin}"
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
MARINER_KATA_OS_NAME="MARINERKATA"
AZURELINUX_OS_NAME="AZURELINUX"
AZURELINUX_KATA_OS_NAME="AZURELINUXKATA"
AZURELINUX_OSGUARD_OS_VARIANT="OSGUARD"

# Real world examples from the command outputs
# For Azure Linux V3: ID=azurelinux VERSION_ID="3.0"
# For Azure Linux V2: ID=mariner VERSION_ID="2.0"
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID=(.*))$/, a) { print toupper(a[2]); exit }')
OS_VARIANT=$(sort -r /etc/*-release | gawk 'match($0, /^(VARIANT_ID=(.*))$/, a) { print toupper(a[2]); exit }' | tr -d '"')
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
GRID_COMPATIBILITY_DATA_FILE=/opt/azure/vhd-grid-compatibility-data.json
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
  BLOBFUSE2_VERSION="2.5.1"
  required_pkg_list=("blobfuse2="${BLOBFUSE2_VERSION} fuse3)
  for apt_package in ${required_pkg_list[*]}; do
      if ! apt_get_install 30 1 600 $apt_package; then
          journalctl --no-pager -u $apt_package
          exit $ERR_APT_INSTALL_TIMEOUT
      fi
  done
fi

CHRONYD_DIR=/etc/systemd/system/chronyd.service.d

mkdir -p "${CHRONYD_DIR}"
cat >> "${CHRONYD_DIR}"/10-chrony-restarts.conf <<EOF
[Service]
Restart=always
RestartSec=5
EOF

tee -a /etc/systemd/journald.conf > /dev/null <<'EOF'
Compress=yes
Storage=persistent
SystemMaxUse=1G
RuntimeMaxUse=1G
ForwardToSyslog=yes
EOF
capture_benchmark "${SCRIPT_NAME}_install_deps_and_set_configs"

if [ "$(isARM64)" -eq 1 ]; then
  # shellcheck disable=SC3010
  if [[ ${HYPERV_GENERATION,,} == "v1" ]]; then
    echo "No arm64 support on V1 VM, exiting..."
    exit 1
  fi
fi

# Always override network config and disable NTP + Timesyncd and install Chrony
# Mariner does this differently, so only do it for Ubuntu
if ! isMarinerOrAzureLinux "$OS"; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi
capture_benchmark "${SCRIPT_NAME}_validate_container_runtime_and_override_ubuntu_net_config"

# Configure SSH service during VHD build for Ubuntu 22.10+
configureSSHService "$OS" "$OS_VERSION" || echo "##vso[task.logissue type=warning]SSH Service configuration failed, but continuing VHD build"

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

if isMarinerOrAzureLinux "$OS" && ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
    disableSystemdResolvedCache
    disableSystemdIptables || exit 1
    setMarinerNetworkdConfig
    fixCBLMarinerPermissions
    addMarinerNvidiaRepo
    updateDnfWithNvidiaPkg
    overrideNetworkConfig || exit 1
    if grep -q "kata" <<< "$FEATURE_FLAGS"; then
      installKataDeps
      if [ "${OS}" != "3.0" ]; then
        enableMarinerKata
      fi
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
  extract_tarball "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}"
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
  updateAptWithNvidiaPkg
  capture_benchmark "${SCRIPT_NAME}_update_apt_with_nvidia_pkg"
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
  # TODO(mheberling): Remove this once kata uses standard containerd. This OS is referenced
  # in file `parts/common/component.json` with the same ${MARINER_KATA_OS_NAME}.
  if isMariner "${OS}" && [ "${IS_KATA}" = "true" ]; then
    # This is temporary for kata-cc because it uses a modified version of containerd and
    # name is referenced in parts/common.json marinerkata.
    os=${MARINER_KATA_OS_NAME}
  fi
  if isAzureLinux "${OS}" && [ "${IS_KATA}" = "true" ]; then
    # This is temporary for kata-cc because it uses a modified version of containerd and
    # name is referenced in parts/common.json azurelinuxkata.
    os=${AZURELINUX_KATA_OS_NAME}
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
        elif isMarinerOrAzureLinux "$OS" && isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
          echo "Skipping containerd install on Azure Linux OS Guard, package preinstalled on immutable /usr"
          version=$(rpm -q containerd2)
        elif isMarinerOrAzureLinux "$OS"; then
          installStandaloneContainerd "${version}"
        elif isFlatcar "$OS"; then
          installStandaloneContainerd "${version}"
        fi
        echo "  - containerd version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "oras")
      for version in ${PACKAGE_VERSIONS[@]}; do
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        if isMarinerOrAzureLinux "$OS" && isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
          echo "Skipping Oras install on Azure Linux OS Guard, package preinstalled on immutable /usr"
          version=$(oras version | head -n1 | awk '{print $2}')
        else
          installOras "${downloadDir}" "${evaluatedURL}" "${version}"
        fi
        echo "  - oras version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "aks-secure-tls-bootstrap-client")
      for version in ${PACKAGE_VERSIONS[@]}; do
        # removed at provisioning time if secure TLS bootstrapping is disabled
        evaluatedURL=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadSecureTLSBootstrapClient "${downloadDir}" "${evaluatedURL}" "${version}"
        echo "  - aks-secure-tls-bootstrap-client version ${version}" >> ${VHD_LOGS_FILEPATH}
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
    "kubelet")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "kubelet" "${version}" "${downloadDir}"
        fi
        echo "  - kubelet version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "kubectl")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "kubectl" "${version}" "${downloadDir}"
        fi
        echo "  - kubectl version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "${K8S_DEVICE_PLUGIN_PKG}")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "${K8S_DEVICE_PLUGIN_PKG}" "${version}" "${downloadDir}"
        fi
        echo "  - ${K8S_DEVICE_PLUGIN_PKG} version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "azure-acr-credential-provider-pmc")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "azure-acr-credential-provider" "${version}" "${downloadDir}"
        fi
        echo "  - azure-acr-credential-provider version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "datacenter-gpu-manager-4-core")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
          echo "Skipping $name install on OS Guard"
        elif [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "datacenter-gpu-manager-4-core" "${version}" "${downloadDir}"
        fi
        echo "  - datacenter-gpu-manager-4-core version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "datacenter-gpu-manager-4-proprietary")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
          echo "Skipping $name install on OS Guard"
        elif [ "${OS}" = "${UBUNTU_OS_NAME}" ] || isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "datacenter-gpu-manager-4-proprietary" "${version}" "${downloadDir}"
        fi
        echo "  - datacenter-gpu-manager-4-proprietary version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "datacenter-gpu-manager-exporter")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if [ "${OS}" = "${UBUNTU_OS_NAME}" ]; then
          downloadPkgFromVersion "datacenter-gpu-manager-exporter" "${version}" "${downloadDir}"
        fi
        echo "  - datacenter-gpu-manager-exporter version ${version}" >> ${VHD_LOGS_FILEPATH}
      done
      ;;
    "dcgm-exporter")
      for version in ${PACKAGE_VERSIONS[@]}; do
        if isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
          echo "Skipping $name install on OS Guard"
        elif isMarinerOrAzureLinux "$OS"; then
          downloadPkgFromVersion "dcgm-exporter" "${version}" "${downloadDir}"
        fi
        echo "  - dcgm-exporter version ${version}" >> ${VHD_LOGS_FILEPATH}
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
  MIRROR_PROXY_VERSION='0.2.14'
  MIRROR_DOWNLOAD_PATH="./$1.$2"
  MIRROR_PROXY_URL="https://acrstreamingpackage.z5.web.core.windows.net/${MIRROR_PROXY_VERSION}/${PACKAGE_NAME}.${PACKAGE_EXTENSION}"
  retrycmd_curl_file 10 5 60 $MIRROR_DOWNLOAD_PATH $MIRROR_PROXY_URL || exit ${ERR_ARTIFACT_STREAMING_DOWNLOAD}
  if [ "$2" = "deb" ]; then
    apt_get_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  elif [ "$2" = "rpm" ]; then
    dnf_install 30 1 600 $MIRROR_DOWNLOAD_PATH || exit $ERR_ARTIFACT_STREAMING_DOWNLOAD
  fi
  rm $MIRROR_DOWNLOAD_PATH

  /opt/acr/tools/overlaybd/install.sh
  /opt/acr/tools/overlaybd/config-user-agent.sh azure
  /opt/acr/tools/overlaybd/enable-http-auth.sh
  /opt/acr/tools/overlaybd/config.sh download.enable false
  /opt/acr/tools/overlaybd/config.sh cacheConfig.cacheSizeGB 32
  /opt/acr/tools/overlaybd/config.sh exporterConfig.enable true
  /opt/acr/tools/overlaybd/config.sh exporterConfig.port 9863
  systemctl link /opt/overlaybd/overlaybd-tcmu.service /opt/overlaybd/snapshotter/overlaybd-snapshotter.service
}

UBUNTU_MAJOR_VERSION=$(echo $UBUNTU_RELEASE | cut -d. -f1)
# Artifact Streaming enabled for all supported Ubuntu versions including 24.04
if [ "$OS" = "$UBUNTU_OS_NAME" ] && [ "$(isARM64)" -ne 1 ] && [ "$UBUNTU_MAJOR_VERSION" -ge 20 ]; then
  installAndConfigureArtifactStreaming acr-mirror-${UBUNTU_RELEASE//.} deb
fi

# Artifact Streaming enabled for Azure Linux 2.0 and 3.0
if [ "$OS" = "$MARINER_OS_NAME" ] && [ "$OS_VERSION" = "2.0" ] && [ "$(isARM64)" -ne 1 ]; then
  installAndConfigureArtifactStreaming acr-mirror-mariner rpm
elif ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT" && [ "$OS" = "$AZURELINUX_OS_NAME" ] && [ "$OS_VERSION" = "3.0" ] && [ "$(isARM64)" -ne 1 ]; then
  installAndConfigureArtifactStreaming acr-mirror-azurelinux3 rpm
fi

capture_benchmark "${SCRIPT_NAME}_install_artifact_streaming"

# k8s will use images in the k8s.io namespaces - create it
ctr namespace create k8s.io
cliTool="ctr"


INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
capture_benchmark "${SCRIPT_NAME}_install_crictl"

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

echo "images pre-pulled:" >> ${VHD_LOGS_FILEPATH}
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
capture_benchmark "${SCRIPT_NAME}_caching_container_images"

retagAKSNodeCAWatcher() {
  # This function retags the aks-node-ca-watcher image to a static tag
  # The static tag is used to bootstrap custom CA trust when MCR egress may be intercepted by an untrusted TLS MITM firewall.
  # The image is never pulled, it is only retagged.

  watcher=$(jq '.ContainerImages[] | select(.downloadURL | contains("aks-node-ca-watcher"))' $COMPONENTS_FILEPATH)
  watcherBaseImg=$(echo $watcher | jq -r .downloadURL)
  watcherVersion=$(echo $watcher | jq -r .multiArchVersionsV2[0].latestVersion)
  watcherFullImg=${watcherBaseImg//\*/$watcherVersion}

  # this image will never get pulled, the tag must be the same across different SHAs.
  # it will only ever be upgraded via node image changes.
  # we do this because the image is used to bootstrap custom CA trust when MCR egress
  # may be intercepted by an untrusted TLS MITM firewall.
  watcherStaticImg=${watcherBaseImg//\*/static}

  # can't use $cliTool variable because crictl doesn't support retagging.
  retagContainerImage "ctr" ${watcherFullImg} ${watcherStaticImg}
}
retagAKSNodeCAWatcher
capture_benchmark "${SCRIPT_NAME}_retag_aks_node_ca_watcher"

pinPodSandboxImages() {
  # This function pins the pod sandbox image(s) to avoid Kubelet's Garbage Collector (GC) from removing them.
  # This is achieved by setting the "io.cri-containerd.pinned" label on the image with a value of "pinned".
  # These images are critical for pod startup and aren't supported with private ACR since containerd won't be using azure-acr-credential to fetch them.

  # Get all pause images as individual JSON objects
  local pause_images
  pause_images=$(jq -c '.ContainerImages[] | select(.downloadURL | contains("pause"))' $COMPONENTS_FILEPATH)

  if [ -z "$pause_images" ]; then
    echo "Warning: No pause images found in components.json"
    return 0
  fi

  # Process each pause image separately
  while IFS= read -r podSandbox; do
    if [ -z "$podSandbox" ]; then
      continue
    fi

    local podSandboxBaseImg
    local podSandboxVersion
    local podSandboxFullImg

    podSandboxBaseImg=$(echo "$podSandbox" | jq -r '.downloadURL')
    podSandboxVersion=$(echo "$podSandbox" | jq -r '.multiArchVersionsV2[0].latestVersion')

    # Skip if we couldn't extract the required information
    if [ "$podSandboxBaseImg" = "null" ] || [ "$podSandboxVersion" = "null" ]; then
      echo "Warning: Could not extract downloadURL or latestVersion from pause image: $podSandbox"
      continue
    fi

    podSandboxFullImg=${podSandboxBaseImg//\*/$podSandboxVersion}

    echo "Pinning pause image: $podSandboxFullImg"
    labelContainerImage "${podSandboxFullImg}" "io.cri-containerd.pinned" "pinned"

  done <<< "$pause_images"
}
pinPodSandboxImages
capture_benchmark "${SCRIPT_NAME}_pin_pod_sandbox_image"

# IPv6 nftables rules are only available on Ubuntu or Mariner/AzureLinux
if [ $OS = $UBUNTU_OS_NAME ] || isMarinerOrAzureLinux "$OS"; then
  systemctlEnableAndStart ipv6_nftables 30 || exit 1
fi

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
	export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
	export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
	mkdir -p "${AZCOPY_LOG_LOCATION}"
	mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

  ./azcopy login --login-type=MSI

  cached_pkg="${K8S_PRIVATE_PACKAGES_CACHE_DIR}/${k8s_tgz_name}"
  echo "download private package ${kube_private_binary_url} and store as ${cached_pkg}"

  if ! ./azcopy copy "${kube_private_binary_url}" "${cached_pkg}"; then
    azExitCode=$?
    # loop through azcopy log files
    shopt -s nullglob
    for f in "${AZCOPY_LOG_LOCATION}"/*.log; do
      echo "Azcopy log file: $f"
      # upload the log file as an attachment to vso
      echo "##vso[build.uploadlog]$f"
      # check if the log file contains any errors
      if grep -q '"level":"Error"' "$f"; then
 	 	echo "log file $f contains errors"
        echo "##vso[task.logissue type=error]Azcopy log file $f contains errors"
        # print the log file
        cat "$f"
      fi
    done
    shopt -u nullglob
    exit $ERR_PRIVATE_K8S_PKG_ERR
  fi
}

if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
  # remove snapd, which is not used by container stack
  apt_get_purge 20 30 120 snapd || exit 1
  apt_get_purge 20 30 120 apache2-utils || exit 1
  # CIS: Ensure telnet (/ftp) client is not installed
  # CIS: Ufw is not used but interferes with log_martians rule
  apt_get_purge 20 30 120 telnet ftp ufw tnftp inetutils-telnet || exit 1

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

# Configure LSM modules to include BPF
configureLsmWithBpf() {
  echo "Configuring LSM modules to include BPF..."

  # Read current LSM modules
  if [ ! -f /sys/kernel/security/lsm ]; then
    echo "Warning: /sys/kernel/security/lsm not found, skipping LSM configuration"
    return 0
  fi

  local current_lsm
  current_lsm=$(cat /sys/kernel/security/lsm)
  echo "Current LSM modules: $current_lsm"

  # Prepend bpf to the LSM list if not already present
  if ! echo "$current_lsm" | grep -q bpf; then
    if [ "$IS_KATA" = "true" ] || echo "$FEATURE_FLAGS" | grep -q "cvm"; then
      echo "Warning: this is a Kata/CVM SKU - will not add BPF to LSM configuration"
      return 0
    fi

    if isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
      echo "Warning: Azure Linux OS Guard built with signed UKI, not enabling BPF LSM"
      return 0
    fi

    local new_lsm="bpf,$current_lsm"
    echo "New LSM configuration: $new_lsm"

    if [ "$OS" = "$UBUNTU_OS_NAME" ] && [ "$OS_VERSION" = "24.04" ]; then
      local grub_cfg="/etc/default/grub.d/50-cloudimg-settings.cfg"
      if [ -f "$grub_cfg" ]; then
        if grep -q "lsm=" "$grub_cfg"; then
          sed -i "s/lsm=[^[:space:]]*/lsm=$new_lsm/g" "$grub_cfg"
        else
          sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"lsm=$new_lsm /" "$grub_cfg"
        fi
        echo "Updating GRUB configuration for Ubuntu 24.04..."
        update-grub2 /boot/grub/grub.cfg || echo "Warning: Failed to update GRUB configuration"
      else
        echo "Warning: $grub_cfg not found, skipping LSM configuration"
      fi
    elif isMarinerOrAzureLinux "$OS" && [ "$OS_VERSION" = "3.0" ]; then
      if [ -f /etc/default/grub ]; then
        if grep -q "lsm=" /etc/default/grub; then
          sed -i "s/lsm=[^[:space:]]*/lsm=$new_lsm/g" /etc/default/grub
        else
          sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"lsm=$new_lsm /" /etc/default/grub
        fi
        echo "Updating GRUB configuration for Azure Linux 3.0..."
        grub2-mkconfig -o /boot/grub2/grub.cfg || echo "Warning: Failed to update GRUB configuration"
      else
        echo "Warning: /etc/default/grub not found, skipping LSM configuration"
      fi
    else
      echo "LSM BPF configuration is only enabled for Ubuntu 24.04 and Azure Linux 3.0, skipping"
    fi

    echo "LSM configuration update completed"
  else
    echo "BPF LSM already configured, skipping"
  fi
}

configureLsmWithBpf
capture_benchmark "${SCRIPT_NAME}_configure_lsm_with_bpf"

# use the private_packages_url to download and cache packages
if [ -n "${PRIVATE_PACKAGES_URL:-}" ]; then
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

# Collect grid compatibility data (placeholder for now - will be extended later)
collect_grid_compatibility_data() {
  if [ -z "${GRID_COMPATIBILITY_DATA_FILE}" ] ; then
    return
  fi

  # Create basic grid compatibility data structure
  # This is scaffolding - the actual Kusto query and analysis will be added later
  local compatibility_data=$(jq -n \
    --arg os "${OS}" \
    --arg os_version "${OS_VERSION}" \
    --arg cpu_arch "${CPU_ARCH}" \
    --arg timestamp "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --arg feature_flags "${FEATURE_FLAGS:-}" \
    '{
      "grid_compatibility_check": {
        "timestamp": $timestamp,
        "os": $os,
        "os_version": $os_version,
        "cpu_architecture": $cpu_arch,
        "feature_flags": $feature_flags,
        "compatibility_status": "data_collected",
        "kusto_query_placeholder": "SELECT * FROM GridCompatibility WHERE timestamp > ago(1d)"
      }
    }')

  echo "${compatibility_data}" > "${GRID_COMPATIBILITY_DATA_FILE}"
  chmod 755 "${GRID_COMPATIBILITY_DATA_FILE}"
}

collect_grid_compatibility_data
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
