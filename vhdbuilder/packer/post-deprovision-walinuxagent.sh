#!/bin/bash -eu
# Post-deprovision WALinuxAgent install script.
# Called by packer inline block AFTER 'waagent -force -deprovision+user',
# which clears /var/lib/waagent/. This script re-installs the latest
# WALinuxAgent from the wireserver GAFamily manifest so the agent daemon
# can pick it up locally without downloading at provisioning time.
#
# NOTE: -x is intentionally omitted to avoid leaking SAS tokens from
# wireserver manifest/blob URLs in packer build logs.

# Skip on AzureLinux OSGuard which uses its OS-packaged waagent version.
# Flatcar is excluded at the packer config level (its JSON does not call this).
OS_VARIANT_ID=$(. /etc/os-release 2>/dev/null && echo "${VARIANT_ID:-}" | tr '[:lower:]' '[:upper:]' | tr -d '"')
if [ "$OS_VARIANT_ID" = "OSGUARD" ]; then
    echo "Skipping WALinuxAgent manifest install on AzureLinux OSGuard"
    exit 0
fi

# Configuration
WALINUXAGENT_DOWNLOAD_DIR="/opt/walinuxagent/downloads"
WALINUXAGENT_WIRESERVER_URL="http://168.63.129.16:80"

# DNS will be broken on AzLinux after deprovision because
# 'waagent -deprovision' clears /etc/resolv.conf.
# Temporarily restore Azure DNS for manifest download
# and then remove before the VHD is finalized to keep the image clean.
RESOLV_CONF_BAK=""
if [ ! -s /etc/resolv.conf ] || ! grep -q nameserver /etc/resolv.conf; then
    cp /etc/resolv.conf /etc/resolv.conf.bak 2>/dev/null || true
    RESOLV_CONF_BAK="/etc/resolv.conf.bak"
    echo "nameserver 168.63.129.16" > /etc/resolv.conf
    echo "Temporarily set DNS to Azure DNS for manifest download"
fi

# Install WALinuxAgent from wireserver GAFamily manifest.
# Uses a standalone Python script (stdlib only) for wireserver HTTP, XML parsing,
# and zip extraction — replacing inline python3 one-liners that were in bash.
python3 /opt/azure/containers/install_walinuxagent.py "${WALINUXAGENT_DOWNLOAD_DIR}" "${WALINUXAGENT_WIRESERVER_URL}"

# Configure waagent.conf to pick up the pre-cached agent from disk:
# - AutoUpdate.Enabled=y tells the daemon to look for newer agent versions on disk
# - AutoUpdate.UpdateToLatestVersion=n prevents downloading updates from the network
sed -i 's/AutoUpdate.Enabled=n/AutoUpdate.Enabled=y/g' /etc/waagent.conf
if ! grep -q '^AutoUpdate.Enabled=' /etc/waagent.conf; then
    echo 'AutoUpdate.Enabled=y' >> /etc/waagent.conf
fi
sed -i 's/AutoUpdate.UpdateToLatestVersion=y/AutoUpdate.UpdateToLatestVersion=n/g' /etc/waagent.conf
if ! grep -q '^AutoUpdate.UpdateToLatestVersion=' /etc/waagent.conf; then
    echo 'AutoUpdate.UpdateToLatestVersion=n' >> /etc/waagent.conf
fi

# Restore resolv.conf to its post-deprovision state so the VHD ships clean
if [ -n "${RESOLV_CONF_BAK}" ] && [ -f "${RESOLV_CONF_BAK}" ]; then
    mv "${RESOLV_CONF_BAK}" /etc/resolv.conf
    echo "Restored /etc/resolv.conf to post-deprovision state"
fi

echo "WALinuxAgent installed and waagent.conf configured post-deprovision"

# Self-delete: these scripts are only needed during VHD build
rm -f /opt/azure/containers/install_walinuxagent.py
rm -f "${BASH_SOURCE[0]}"
