#!/bin/bash

set -e

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

required_env_vars=(
    "IMG_SKU"
    "SKU_NAME"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    else
        echo "$v is set to '${!v}'"
    fi
done

FEATURE_FLAGS="${FEATURE_FLAGS:-}"

echo "Starting build on $(date)" > ${VHD_LOGS_FILEPATH}

source /opt/azure/containers/provision_source.sh

# Fixup repart config from base image
sed -i 's/Type=usr/Type=linux-generic/' /etc/repart.d/12-usr-a.conf
rm /etc/repart.d/15-boot-b.conf  /etc/repart.d/16-usr-b.conf  /etc/repart.d/17-usr-hash-b.conf || true

# Setup a symlink for lg-redirect-sysext
mkdir -p /etc/extensions/lg-redirect-sysext/usr/local/
mkdir -p /opt/bin
ln -s /opt/bin /etc/extensions/lg-redirect-sysext/usr/local/bin
# Bind mount /opt/bin to /usr/local/bin during the build since the redirect sysext is not running
mount --bind /opt/bin /usr/local/bin
trap "umount /usr/local/bin" EXIT

# Link /opt/azure/containers to /home/packer for postinstall
ln -s /opt/azure/containers /home/packer

### pre-install-dependencies ###
echo -e "\nnews.none                          -/var/log/messages" >> /etc/rsyslog.d/60-CIS.conf
# Create dir for update_certs.path
mkdir /opt/certs
chmod 755 /opt/certs
# Use AKS Log Collector instead of WALA log collections
echo -e "\n# Disable WALA log collection because AKS Log Collector is installed.\nLogs.Collect=n" >> /etc/waagent.conf

### install-dependencies ###
# Start containerd to allow container precaching
containerd &
CONTAINERD_PID=$!
echo "Started containerd with PID $CONTAINERD_PID"
# shellcheck disable=2064
trap "kill $CONTAINERD_PID" EXIT

# Precache packages and containers from components.json
/opt/azure/containers/install-dependencies.sh

# Apply CIS compliance changes
/opt/azure/containers/cis.sh

# List images for image-bom.json
/home/packer/list-images.sh

# Cleanup scripts only used during the build
rm /home/packer/install-dependencies.sh
rm /home/packer/provision_source_benchmarks.sh
rm /home/packer/tool_installs.sh
rm /home/packer/tool_installs_distro.sh
rm /home/packer/lister
rm /home/packer/list-images.sh
rm /home/packer/cis.sh
rm /home/packer

# Create release-notes.txt
mkdir -p /_imageconfigs/out

echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

echo -e "=== Installed Packages Begin" >> ${VHD_LOGS_FILEPATH}
echo -e "$(rpm -qa)" >> ${VHD_LOGS_FILEPATH}
echo -e "=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}

echo -e "=== os-release Begin" >> ${VHD_LOGS_FILEPATH}
cat /etc/os-release >> ${VHD_LOGS_FILEPATH}
echo -e "=== os-release End" >> ${VHD_LOGS_FILEPATH}

echo -e "=== OS Guard Info === Begin" >> ${VHD_LOGS_FILEPATH}
sha256sum /boot/efi/EFI/Linux/* >> ${VHD_LOGS_FILEPATH}
echo -e "=== OS Guard Info === End" >> ${VHD_LOGS_FILEPATH}

# Copy logs and BOM to the output directory
cp ${VHD_LOGS_FILEPATH} /_imageconfigs/out/release-notes.txt
cp /var/log/bcc_installation.log /_imageconfigs/out/bcc-tools-installation.log
chmod 644 /_imageconfigs/out/bcc-tools-installation.log
cp /opt/azure/containers/image-bom.json /_imageconfigs/out/image-bom.json
chmod 644 /_imageconfigs/out/image-bom.json
mv /opt/azure/vhd-build-performance-data.json /_imageconfigs/out/vhd-build-performance-data.json
chmod 644 /_imageconfigs/out/vhd-build-performance-data.json
