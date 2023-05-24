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
chmod 1666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1

systemctlEnableAndStart ci-syslog-watcher.path || exit 1
systemctlEnableAndStart ci-syslog-watcher.service || exit 1

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
systemctlEnableAndStart logrotate.timer || exit 1
rm -f /etc/cron.daily/logrotate

systemctlEnableAndStart sync-container-logs.service || exit 1

# First handle Mariner + FIPS
if [[ ${OS} == ${MARINER_OS_NAME} ]]; then
  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS install for Mariner and has nothing to do with Ubuntu Advantage
    echo "Install FIPS for Mariner SKU"
    installFIPS
  fi
else
  # Handle FIPS and ESM for Ubuntu
  if [[ "${UBUNTU_RELEASE}" == "18.04" ]] || [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    autoAttachUA
  fi

  # Run apt get update to refresh repo list
  # Run apt dist get upgrade to install packages/kernels

  # CVM breaks on kernel image updates due to nullboot package post-install.
  # it relies on boot measurements from real tpm hardware.
  # building on a real CVM would solve this, but packer doesn't support it.
  # we could make upstream changes but that takes time, and we are broken now.
  # so we just hold the kernel image packages for now on CVM.
  # this still allows us base image and package updates on a weekly cadence.
  if [[ "$IMG_SKU" != "20_04-lts-cvm" ]]; then
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT    
  fi

  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS Install for Ubuntu, it purges non FIPS Kernel and attaches UA FIPS Updates
    echo "Install FIPS for Ubuntu SKU"
    installFIPS
  fi

  # Final step, if 1804 or FIPS, log ua status, detach UA and clean up
  if [[ "${UBUNTU_RELEASE}" == "18.04" ]] || [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # 'ua status' for logging
    ua status
    detachAndCleanUpUA
  fi
fi

echo "pre-install-dependencies step finished successfully"