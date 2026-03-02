#!/bin/bash -eux

# Cleanup packer SSH key and machine ID generated for this boot
rm -f /root/.ssh/authorized_keys
rm -f /home/packer/.ssh/authorized_keys
rm -f /var/log/cloud-init.log /var/log/cloud-init-output.log
# aznfs pulls in stunnel4 which pollutes the log dir but aznfs configures stunnel to log to a private location
rm -rf /var/log/stunnel4/ /etc/logrotate.d/stunnel4
rm -f /etc/machine-id
touch /etc/machine-id
chmod 644 /etc/machine-id
# Cleanup disk usage diagnostics file (created by generate-disk-usage.sh)
rm -f /opt/azure/disk-usage.txt
# Cleanup IMDS instance metadata cache file
rm -f /opt/azure/containers/imds_instance_metadata_cache.json

# Write post-deprovision WALinuxAgent install script.
# The deprovision step (waagent -force -deprovision+user) clears /var/lib/waagent/,
# so we install the GAFamily agent AFTER deprovision. This script is called from the
# packer inline block after the deprovision command completes.
# Skip on Flatcar and AzureLinux OSGuard which use their OS-packaged version.
OS_ID=$(. /etc/os-release 2>/dev/null && echo "${ID:-}" | tr '[:lower:]' '[:upper:]')
OS_VARIANT_ID=$(. /etc/os-release 2>/dev/null && echo "${VARIANT_ID:-}" | tr '[:lower:]' '[:upper:]' | tr -d '"')
if [ "$OS_ID" != "FLATCAR" ] && [ "$OS_VARIANT_ID" != "OSGUARD" ]; then
    # Read the download location and wireserver URL from components.json
    WALINUXAGENT_DOWNLOAD_DIR=$(jq -r '.Packages[] | select(.name == "walinuxagent") | .downloadLocation' /opt/azure/components.json)
    if [ -z "$WALINUXAGENT_DOWNLOAD_DIR" ] || [ "$WALINUXAGENT_DOWNLOAD_DIR" = "null" ]; then
        echo "ERROR: walinuxagent downloadLocation not found in components.json"
        exit 1
    fi
    WALINUXAGENT_WIRESERVER_URL=$(jq -r '.Packages[] | select(.name == "walinuxagent") | .wireserverURL' /opt/azure/components.json)
    if [ -z "$WALINUXAGENT_WIRESERVER_URL" ] || [ "$WALINUXAGENT_WIRESERVER_URL" = "null" ]; then
        echo "ERROR: walinuxagent wireserverURL not found in components.json"
        exit 1
    fi
    cat > /opt/azure/containers/post-deprovision-walinuxagent.sh << WALINUXAGENT_SCRIPT
#!/bin/bash -eu
# Post-deprovision WALinuxAgent install script.
# NOTE: -x is intentionally omitted to avoid leaking SAS tokens from
# wireserver manifest/blob URLs in packer build logs.
# Sources the provisioning helpers and installs the GAFamily agent from wireserver,
# then configures waagent.conf to use the pre-cached agent from disk.

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

source /opt/azure/containers/provision_source.sh
source /opt/azure/containers/provision_installs.sh

# Install WALinuxAgent from wireserver GAFamily manifest
installWALinuxAgent ${WALINUXAGENT_DOWNLOAD_DIR} ${WALINUXAGENT_WIRESERVER_URL}

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
if [ -n "\${RESOLV_CONF_BAK}" ] && [ -f "\${RESOLV_CONF_BAK}" ]; then
    mv "\${RESOLV_CONF_BAK}" /etc/resolv.conf
    echo "Restored /etc/resolv.conf to post-deprovision state"
fi

echo "WALinuxAgent installed and waagent.conf configured post-deprovision"
WALINUXAGENT_SCRIPT
    chmod 755 /opt/azure/containers/post-deprovision-walinuxagent.sh
    echo "Wrote post-deprovision WALinuxAgent install script"
fi
