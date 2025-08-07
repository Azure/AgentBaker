#!/bin/bash

set -e

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

source /opt/azure/containers/provision_source.sh

CNI_DOWNLOADS_DIR="/opt/cni/downloads"
CRICTL_DOWNLOAD_DIR="/opt/crictl/downloads"
CRICTL_BIN_DIR="/opt/bin"
SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR="/opt/aks-secure-tls-bootstrap-client/downloads"
SECURE_TLS_BOOTSTRAP_CLIENT_BIN_DIR="/opt/bin"
TELEPORTD_PLUGIN_DOWNLOAD_DIR="/opt/teleportd/downloads"
CREDENTIAL_PROVIDER_DOWNLOAD_DIR="/opt/credentialprovider/downloads"
CREDENTIAL_PROVIDER_BIN_DIR="/var/lib/kubelet/credential-provider"

# Recreate variables from the pipeline build environment for install-dependencies.sh
export IMG_SKU="azure-linux-osguard-3"
export CONTAINER_RUNTIME="containerd"
export IS_OSGUARD="true"
export SKU_NAME="V3gen2fips"
export FEATURE_FLAGS=""

# Setup a symlink for lg-redirect-sysext
mkdir -p /etc/extensions/lg-redirect-sysext/usr/local/
mkdir -p /opt/bin
ln -s /opt/bin /etc/extensions/lg-redirect-sysext/usr/local/bin
# Bind mount /opt/bin to /usr/local/bin during the build since the redirect sysext is not running
mount --bind /opt/bin /usr/local/bin
trap "umount /usr/local/bin" EXIT

# Link /opt/azure/containers to /home/packer for postinstall
ln -s /opt/azure/containers /home/packer

containerd &
CONTAINERD_PID=$!
echo "Started containerd with PID $CONTAINERD_PID"
trap "kill $CONTAINERD_PID" EXIT
/opt/azure/containers/install-dependencies.sh

/opt/azure/containers/cis.sh

# Disable waagent autoupdate
echo AutoUpdate.Enabled=n >> /etc/waagent.conf

# Disable default eth0 dhcp rule
truncate -s 0 /etc/systemd/network/99-dhcp-en.network

# Create empty dir for compatibility with dir or create containers trying to mount from host like managed prometheus
mkdir -p /usr/local/share/ca-certificates/

# Place ci-syslog.watcher.sh into the /usr/local/bin overlay
mv /opt/scripts/ci-syslog-watcher.sh /opt/bin/ci-syslog-watcher.sh

# List images for image-bom.json
/home/packer/list-images.sh

# Create release-notes.txt
mkdir -p /_imageconfigs/out
echo "release notes stub" >> /_imageconfigs/out/release-notes.txt

echo -e "=== Installed Packages Begin" >> ${VHD_LOGS_FILEPATH}
echo -e "$(rpm -qa)" >> ${VHD_LOGS_FILEPATH}
echo -e "=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}

echo -e "=== os-release Begin" >> ${VHD_LOGS_FILEPATH}
cat /etc/os-release >> ${VHD_LOGS_FILEPATH}
echo -e "=== os-release End" >> ${VHD_LOGS_FILEPATH}

cp ${VHD_LOGS_FILEPATH} /_imageconfigs/out/release-notes.txt
cp /var/log/bcc_installation.log /_imageconfigs/out/bcc-tools-installation.log
chmod 644 /_imageconfigs/out/bcc-tools-installation.log
cp /opt/azure/containers/image-bom.json /_imageconfigs/out/image-bom.json
chmod 644 /_imageconfigs/out/image-bom.json
mv /opt/azure/vhd-build-performance-data.json /_imageconfigs/out/vhd-build-performance-data.json
chmod 644 /_imageconfigs/out/vhd-build-performance-data.json

# Clean up build time redirections
rm /home/packer