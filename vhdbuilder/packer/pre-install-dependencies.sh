#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID=(.*))$/, a) { print toupper(a[2]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

#the following sed removes all comments of the format {{/* */}}
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/provision_source.sh
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/tool_installs_distro.sh

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_benchmarks.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
PERFORMANCE_DATA_FILE=/opt/azure/vhd-build-performance-data.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
cat manifest.json > ${MANIFEST_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

if isMarinerOrAzureLinux "$OS"; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

installJq || echo "WARNING: jq installation failed, VHD Build benchmarks will not be available for this build."
capture_benchmark "${SCRIPT_NAME}_source_packer_files_and_declare_variables"

copyPackerFiles

# Update rsyslog configuration
RSYSLOG_CONFIG_FILEPATH="/etc/rsyslog.d/60-CIS.conf"
if isMarinerOrAzureLinux "$OS"; then
    echo -e "\nnews.none                          -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
else
    echo -e "\n*.*;mail.none;news.none            -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
fi
systemctl daemon-reload
systemctlEnableAndStart systemd-journald 30 || exit 1
if ! isFlatcar "$OS" ; then
    systemctlEnableAndStart rsyslog 30 || exit 1
fi

# ACL (Azure Container Linux) is Flatcar-based but its image is missing azure-vm-utils,
# and WALinuxAgent skips udev rule installation on Flatcar >= 3550. Install the rules
# from azure-vm-utils v0.7.0 so /dev/disk/azure/{root,os,resource} symlinks exist for disk_queue.
if isACL "$OS" && [ ! -e /usr/lib/udev/rules.d/80-azure-disk.rules ] && [ ! -e /etc/udev/rules.d/80-azure-disk.rules ]; then
    echo "ACL: Azure disk udev rules not found, installing to /etc/udev/rules.d/80-azure-disk.rules"
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
# Azure hosts will retry remote I/O requests for up to 120 seconds.  If I/O times out for longer than that, host will reboot OS.
# Set timeout for remote disks to 240 seconds, giving host time to handle the retry or reboot the VM.
ENV{DEVTYPE}=="disk", ATTRS{nsid}=="?*", ATTR{queue/io_timeout}="240000"

# For remote disks, namespace ID=1 is OS disk, ID=2+ are data disks with customer-configured lun=ID-2 (e.g. lun=0 will have nsid=2).
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
    udevadm control --reload-rules
    udevadm trigger --subsystem-match=block --action=change
    # Give udev a moment to create the /dev/disk/azure/ symlinks
    udevadm settle --timeout=10
fi

systemctlEnableAndStart disk_queue 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_copy_packer_files_and_enable_logging"

# This path is used by the Custom CA Trust feature only
mkdir -p /opt/certs
chmod 755 /opt/certs
systemctlEnableAndStart update_certs.path 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_make_certs_directory_and_update_certs"

systemctlEnableAndStart ci-syslog-watcher.path 30 || exit 1
systemctlEnableAndStart ci-syslog-watcher.service 30 || exit 1

if isFlatcar "$OS"; then
    # "copy-on-write"; this starts out as a symlink to a R/O location
    cp /etc/waagent.conf{,.new}
    mv /etc/waagent.conf{.new,}
fi
# enable AKS log collector
echo -e "\n# Disable WALA log collection because AKS Log Collector is installed.\nLogs.Collect=n" >> /etc/waagent.conf || exit 1
systemctlEnableAndStart aks-log-collector.timer 30 || exit 1

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
# ACL uses Azure Linux 3's logrotate RPM which creates /var/lib/logrotate at install time
# but does not ship a tmpfiles.d drop-in. On ACL's immutable rootfs with a separate /var
# partition, the directory is missing at runtime, causing logrotate.service to fail.
if isFlatcar "$OS"; then
    mkdir -p /var/lib/logrotate
fi
systemctlEnableAndStart logrotate.timer 30 || exit 1
rm -f /etc/cron.daily/logrotate

systemctlEnableAndStart sync-container-logs.service 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_enable_and_configure_logging_services"

# enable aks-node-controller.service
systemctl enable aks-node-controller.service

# First handle Mariner + FIPS
if isMarinerOrAzureLinux "$OS"; then
  dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
  dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
  if [ "${ENABLE_FIPS,,}" = "true" ] && [ "${IMG_SKU,,}" != "azure-linux-3-arm64-gen2-fips" ]; then
    # This is FIPS install for Mariner and has nothing to do with Ubuntu Advantage
    echo "Install FIPS for Mariner SKU"
    installFIPS
  fi
else
  # Enable ESM only for 20.04, and FIPS
  if [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${ENABLE_FIPS,,}" = "true" ]; then
    set +x
    attachUA
    set -x
  fi

  if [ -n "${VHD_BUILD_TIMESTAMP}" ] && [ "${OS_VERSION}" = "22.04" ]; then
    sed -i "s#http://azure.archive.ubuntu.com/ubuntu/#https://snapshot.ubuntu.com/ubuntu/${VHD_BUILD_TIMESTAMP}#g" /etc/apt/sources.list
  fi

  # Run apt get update to refresh repo list
  # Run apt dist get upgrade to install packages/kernels
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT

  # shellcheck disable=SC3010
  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS Install for Ubuntu, it purges non FIPS Kernel and attaches UA FIPS Updates
    echo "Install FIPS for Ubuntu SKU"
    installFIPS
  fi
fi
capture_benchmark "${SCRIPT_NAME}_upgrade_distro_and_resolve_fips_requirements"

# Handle Azure Linux + CgroupV2
# CgroupV2 is enabled by default in the AzureLinux 3.0 marketplace image
# shellcheck disable=SC3010
if [[ ${OS} == ${MARINER_OS_NAME} ]] && [[ "${ENABLE_CGROUPV2,,}" == "true" ]]; then
  enableCgroupV2forAzureLinux
fi
capture_benchmark "${SCRIPT_NAME}_enable_cgroupv2_for_azurelinux"

# shellcheck disable=SC3010
if [[ ${UBUNTU_RELEASE//./} -ge 2204 && "${ENABLE_FIPS,,}" != "true" ]]; then

  # Choose kernel packages based on Ubuntu version and architecture
  if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    KERNEL_IMAGE="linux-image-azure-fde-lts-${UBUNTU_RELEASE}"
    KERNEL_PACKAGES=(
      "linux-image-azure-fde-lts-${UBUNTU_RELEASE}"
      "linux-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-cloud-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-headers-azure-lts-${UBUNTU_RELEASE}"
      "linux-modules-extra-azure-lts-${UBUNTU_RELEASE}"
    )
    echo "Installing fde LTS kernel for CVM Ubuntu ${UBUNTU_RELEASE}"
  else
    # Use LTS kernel for other versions
    KERNEL_IMAGE="linux-image-azure-lts-${UBUNTU_RELEASE}"
    KERNEL_PACKAGES=(
      "linux-image-azure-lts-${UBUNTU_RELEASE}"
      "linux-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-cloud-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-headers-azure-lts-${UBUNTU_RELEASE}"
      "linux-modules-extra-azure-lts-${UBUNTU_RELEASE}"
    )
    echo "Installing LTS kernel for Ubuntu ${UBUNTU_RELEASE}"
  fi

  echo "Logging the currently running kernel: $(uname -r)"
  echo "Before purging kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*' || true

  if apt-cache show "$KERNEL_IMAGE" &>/dev/null; then
    echo "Kernel packages are available, proceeding with purging current kernel and installing new kernel..."

    # Purge nullboot package only for cvm
    if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
      wait_for_apt_locks
      DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y --allow-remove-essential nullboot
    fi

    # Purge all current kernels and dependencies
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y $(dpkg-query -W 'linux-*azure*' | awk '$2 != "" { print $1 }' | paste -s)
    echo "After purging kernel, dpkg list should be empty"; dpkg -l 'linux-*azure*' || true

    # Install new kernel packages
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y "${KERNEL_PACKAGES[@]}"
    echo "After installing new kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*' || true

    # Reinstall nullboot package only for cvm
    if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
      wait_for_apt_locks
      DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y nullboot
    fi

    # Cleanup
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get autoremove -y && DEBIAN_FRONTEND=noninteractive apt-get clean
  else
    echo "Kernel packages for Ubuntu ${UBUNTU_RELEASE} are not available. Skipping purging and subsequent installation."
  fi
  NVIDIA_KERNEL_PACKAGE="linux-azure-nvidia"
  if [[ "${CPU_ARCH}" == "arm64" && "${UBUNTU_RELEASE}" = "24.04" ]]; then
    # This is the ubuntu 2404arm64gen2containerd image.
    # Uncomment if we have trouble finding the kernel package.
    # sudo add-apt-repository ppa:canonical-kernel-team/ppa
    sudo apt update
    if apt-cache show "${NVIDIA_KERNEL_PACKAGE}" &> /dev/null; then
      echo "ARM64 image. Installing NVIDIA kernel and its packages alongside LTS kernel"
      wait_for_apt_locks
      sudo apt install --no-install-recommends -y "${NVIDIA_KERNEL_PACKAGE}"
      echo "after installation:"
      dpkg -l | grep "linux-.*-azure-nvidia" || true
    else
      echo "ARM64 image. NVIDIA kernel not available, skipping installation."
    fi
  fi
  wait_for_apt_locks
  if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    echo "update-grub not found (expected for CVM images using nullboot), skipping"
  else
    update-grub
  fi
fi
capture_benchmark "${SCRIPT_NAME}_purge_ubuntu_kernel_if_2204"
echo "pre-install-dependencies step finished successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
