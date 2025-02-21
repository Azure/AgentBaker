#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
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
capture_benchmark "${SCRIPT_NAME}_source_packer_files_declare_variables_and_set_mariner_permissions"

copyPackerFiles

# Update rsyslog configuration
RSYSLOG_CONFIG_FILEPATH="/etc/rsyslog.d/60-CIS.conf"
if isMarinerOrAzureLinux "$OS"; then
    echo -e "\nnews.none                          -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
else
    echo -e "\n*.*;mail.none;news.none            -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
fi
systemctl daemon-reload
systemctlEnableAndStart systemd-journald || exit 1
systemctlEnableAndStart rsyslog || exit 1

systemctlEnableAndStart disk_queue || exit 1
capture_benchmark "${SCRIPT_NAME}_copy_packer_files_and_enable_rsyslog"

mkdir /opt/certs
chmod 1666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1
capture_benchmark "${SCRIPT_NAME}_make_directory_and_update_certs"

systemctlEnableAndStart ci-syslog-watcher.path || exit 1
systemctlEnableAndStart ci-syslog-watcher.service || exit 1

# enable AKS log collector
echo -e "\n# Disable WALA log collection because AKS Log Collector is installed.\nLogs.Collect=n" >> /etc/waagent.conf || exit 1
systemctlEnableAndStart aks-log-collector.timer || exit 1

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
systemctlEnableAndStart logrotate.timer || exit 1
rm -f /etc/cron.daily/logrotate

systemctlEnableAndStart sync-container-logs.service || exit 1
capture_benchmark "${SCRIPT_NAME}_enable_and_configure_logging_services"

# enable aks-node-controller.service
systemctl enable aks-node-controller.service

# First handle Mariner + FIPS
if isMarinerOrAzureLinux "$OS"; then
  dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
  dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
  if [[ "${ENABLE_FIPS,,}" == "true" && "${IMG_SKU,,}" != "azure-linux-3-arm64-gen2-fips" ]]; then
    # This is FIPS install for Mariner and has nothing to do with Ubuntu Advantage
    echo "Install FIPS for Mariner SKU"
    installFIPS
  fi
else
  # Enable ESM only for 18.04, 20.04, and FIPS
  if [[ "${UBUNTU_RELEASE}" == "18.04" ]] || [[ "${UBUNTU_RELEASE}" == "20.04" ]] || [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    set +x
    attachUA
    set -x
  fi

  if [[ -n "${VHD_BUILD_TIMESTAMP}" && "${OS_VERSION}" == "22.04" ]]; then
    sed -i "s#http://azure.archive.ubuntu.com/ubuntu/#https://snapshot.ubuntu.com/ubuntu/${VHD_BUILD_TIMESTAMP}#g" /etc/apt/sources.list
  fi

  # Run apt get update to refresh repo list
  # Run apt dist get upgrade to install packages/kernels
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT

  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS Install for Ubuntu, it purges non FIPS Kernel and attaches UA FIPS Updates
    echo "Install FIPS for Ubuntu SKU"
    installFIPS
  fi
fi
capture_benchmark "${SCRIPT_NAME}_upgrade_distro_and_resolve_fips_requirements"

# Handle Azure Linux + CgroupV2
# CgroupV2 is enabled by default in the AzureLinux 3.0 marketplace image
if [[ ${OS} == ${MARINER_OS_NAME} ]] && [[ "${ENABLE_CGROUPV2,,}" == "true" ]]; then
  enableCgroupV2forAzureLinux
fi
capture_benchmark "${SCRIPT_NAME}_handle_azureLinux_and_cgroupV2"

if [[ "${UBUNTU_RELEASE}" == "22.04" && "${ENABLE_FIPS,,}" != "true" ]]; then
  echo "Logging the currently running kernel: $(uname -r)"
  echo "Before purging kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*'

  # Purge all current kernels and dependencies
  DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y $(dpkg-query -W 'linux-*azure*' | awk '$2 != "" { print $1 }' | paste -s)
  echo "After purging kernel, dpkg list should be empty"; dpkg -l 'linux-*azure*'

  # Install lts-22.04 kernel
  DEBIAN_FRONTEND=noninteractive apt-get install -y linux-image-azure-lts-22.04 linux-cloud-tools-azure-lts-22.04 linux-headers-azure-lts-22.04 linux-modules-extra-azure-lts-22.04 linux-tools-azure-lts-22.04
  echo "After installing new kernel, here is a list of kernels/headers installed"; dpkg -l 'linux-*azure*'

  update-grub
fi
capture_benchmark "${SCRIPT_NAME}_purge_ubuntu_kernel_if_2204"
echo "pre-install-dependencies step finished successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks