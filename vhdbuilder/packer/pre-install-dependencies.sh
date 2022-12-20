#!/bin/bash
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
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
chmod 666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1

systemctlEnableAndStart ci-syslog-watcher.path || exit 1
systemctlEnableAndStart ci-syslog-watcher.service || exit 1

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
systemctlEnableAndStart logrotate.timer || exit 1
rm -f /etc/cron.daily/logrotate

if [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
  installFIPS
elif [[ ${ENABLE_FIPS,,} == "true" ]]; then
  echo "AKS enables FIPS on Ubuntu 18.04 only, exiting..."
  exit 1
fi

echo "pre-install-dependencies step finished successfully"