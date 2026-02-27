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
    # Read the download location from components.json rather than hardcoding it
    WALINUXAGENT_DOWNLOAD_DIR=$(jq -r '.Packages[] | select(.name == "walinuxagent") | .downloadLocation' /opt/azure/components.json)
    if [ -z "$WALINUXAGENT_DOWNLOAD_DIR" ] || [ "$WALINUXAGENT_DOWNLOAD_DIR" = "null" ]; then
        echo "ERROR: walinuxagent downloadLocation not found in components.json"
        exit 1
    fi
    cat > /opt/azure/containers/post-deprovision-walinuxagent.sh << WALINUXAGENT_SCRIPT
#!/bin/bash -eux
# Post-deprovision WALinuxAgent install script.
# Sources the provisioning helpers and installs the GAFamily agent from wireserver,
# then configures waagent.conf to use the pre-cached agent from disk.

# After deprovision, DNS may be broken on some distros (e.g., Azure Linux)
# because 'waagent -deprovision' clears /etc/resolv.conf. Wireserver calls
# use IP 168.63.129.16 directly, but the manifest download needs DNS to
# resolve blob storage hostnames. Azure DNS at 168.63.129.16 is always
# available on Azure VMs.
if [ ! -s /etc/resolv.conf ] || ! grep -q nameserver /etc/resolv.conf; then
    echo "nameserver 168.63.129.16" > /etc/resolv.conf
    echo "Restored DNS resolution using Azure DNS after deprovision"
fi

source /opt/azure/containers/provision_source.sh
source /opt/azure/containers/provision_installs.sh

# Install WALinuxAgent from wireserver GAFamily manifest
installWALinuxAgent ${WALINUXAGENT_DOWNLOAD_DIR}

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

echo "WALinuxAgent installed and waagent.conf configured post-deprovision"
WALINUXAGENT_SCRIPT
    chmod 755 /opt/azure/containers/post-deprovision-walinuxagent.sh
    echo "Wrote post-deprovision WALinuxAgent install script"
fi
