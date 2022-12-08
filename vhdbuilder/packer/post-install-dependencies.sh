#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

# Hardcode the desired size the OS disk so we don't accidently rely on extra space temporarily added during builds
MAX_BLOCK_COUNT=30298176 # 30 GB

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # shellcheck disable=SC2021
  current_kernel="$(uname -r | cut -d- -f-2)"
  dpkg --get-selections | grep -e "linux-\(headers\|modules\|image\)" | grep -v "$current_kernel" | tr -s '[[:space:]]' | tr '\t' ' ' | cut -d' ' -f1 | xargs -I{} apt-get remove -yq {}

  # remove apport
  apt-get purge --auto-remove apport open-vm-tools -y

  # strip old kernels/packages
  apt-get -y autoclean || exit 1
  apt-get -y autoremove --purge || exit 1
  apt-get -y clean || exit 1
fi

# shellcheck disable=SC2129
echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

echo -e "=== Installed Packages Begin\n$(listInstalledPackages)\n=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}

# check the size of the OS disk after installing all dependencies: warn at 75% space taken, error at 99% space taken
os_disk=$(readlink -f /dev/disk/azure/root-part1)
used_blocks=$(df -P | grep -w "${os_disk}" | awk '{print $3}')
usage=$(echo "scale = 2; (${used_blocks} / ${MAX_BLOCK_COUNT}) * 100" | bc)
usage=${usage%.*}
[ ${usage} -ge 99 ] && echo "ERROR: OS disk (${os_disk}) is already 99% used!" && exit 1
[ ${usage} -ge 75 ] && echo "WARNING: OS disk (${os_disk}) is already 75% used!" >> ${VHD_LOGS_FILEPATH}

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

echo "post-install-dependencies step completed successfully"
