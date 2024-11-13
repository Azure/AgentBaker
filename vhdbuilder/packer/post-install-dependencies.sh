#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_benchmarks.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
PERFORMANCE_DATA_FILE=/opt/azure/vhd-build-performance-data.json

# Hardcode the desired size of the OS disk so we don't accidently rely on extra disk space
MAX_BLOCK_COUNT=30298176 # 30 GB
capture_benchmark "${SCRIPT_NAME}_set_variables_and_source_packer_files"

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # shellcheck disable=SC2021
  current_kernel="$(uname -r | cut -d- -f-2)"
  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    dpkg --get-selections | grep -e "linux-\(headers\|modules\|image\)" | grep -v "$current_kernel" | grep -v "fips" | tr -s '[[:space:]]' | tr '\t' ' ' | cut -d' ' -f1 | xargs -I{} apt-get --purge remove -yq {}
  else
    dpkg --get-selections | grep -e "linux-\(headers\|modules\|image\)" | grep -v "linux-\(headers\|modules\|image\)-azure" | grep -v "$current_kernel" | tr -s '[[:space:]]' | tr '\t' ' ' | cut -d' ' -f1 | xargs -I{} apt-get --purge remove -yq {}
  fi

  # remove apport
  retrycmd_if_failure 10 2 60 apt-get purge --auto-remove apport open-vm-tools -y || exit 1

  # strip old kernels/packages
  retrycmd_if_failure 10 2 60 apt-get -y autoclean || exit 1
  retrycmd_if_failure 10 2 60 apt-get -y autoremove --purge || exit 1
  retrycmd_if_failure 10 2 60 apt-get -y clean || exit 1

  # Final step, if 18.04 or FIPS, log ua status, detach UA and clean up
  if [[ "${UBUNTU_RELEASE}" == "18.04" ]] || [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # 'ua status' for logging
    ua status
    detachAndCleanUpUA
  fi
fi
capture_benchmark "${SCRIPT_NAME}_log_and_detach_ua"

# Enable the aks-local-dns service
systemctl enable aks-local-dns.service

# shellcheck disable=SC2129
echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

echo -e "=== Installed Packages Begin\n$(listInstalledPackages)\n=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}
capture_benchmark "${SCRIPT_NAME}_list_installed_packages"

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}

# check the size of the OS disk after installing all dependencies: warn at 75% space taken, error at 99% space taken
os_device=$(readlink -f /dev/disk/azure/root)
used_blocks=$(df -P / | sed 1d | awk '{print $3}')
usage=$(awk -v used=${used_blocks} -v capacity=${MAX_BLOCK_COUNT} 'BEGIN{print (used/capacity) * 100}')
usage=${usage%.*}
[ ${usage} -ge 99 ] && echo "ERROR: root partition on OS device (${os_device}) already passed 99% of the 30GB cap!" && exit 1
[ ${usage} -ge 75 ] && echo "WARNING: root partition on OS device (${os_device}) already passed 75% of the 30GB cap!"
capture_benchmark "${SCRIPT_NAME}_determine_disk_usage"

echo -e "=== os-release Begin" >> ${VHD_LOGS_FILEPATH}
cat /etc/os-release >> ${VHD_LOGS_FILEPATH}
echo -e "=== os-release End" >> ${VHD_LOGS_FILEPATH}

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
capture_benchmark "${SCRIPT_NAME}_write_logs"

if [[ $(isARM64) != 1 ]]; then
  # no asc-baseline-1.1.0-268.arm64.deb
  installAscBaseline
fi
capture_benchmark "${SCRIPT_NAME}_install_asc_baseline"

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  if [[ ${ENABLE_FIPS,,} == "true" || ${CPU_ARCH} == "arm64" ]]; then
    relinkResolvConf
  fi
fi
capture_benchmark "${SCRIPT_NAME}_resolve_conf"
echo "post-install-dependencies step completed successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks