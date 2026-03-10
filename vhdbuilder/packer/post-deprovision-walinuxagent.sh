#!/bin/bash -eu
# Post-deprovision WALinuxAgent install script.
# Called by packer inline block AFTER 'waagent -force -deprovision+user',
# which clears /var/lib/waagent/. This script reads the target WALinuxAgent
# version from components.json and installs it from the wireserver manifest
# so the agent daemon can pick it up locally without downloading at
# provisioning time.
#
# NOTE: -x is intentionally omitted to avoid leaking SAS tokens from
# wireserver manifest/blob URLs in packer build logs.

# ---- resolv.conf state tracking (read by the EXIT trap) ----
RESOLV_CONF_ORIGINAL_STATE=""      # "symlink", "file", or "absent"; empty = not modified
RESOLV_CONF_SYMLINK_RAW=""         # raw symlink value (preserves relative paths for ln -sf)
RESOLV_CONF_SYMLINK_RESOLVED=""    # resolved target path (for reading/writing content)
RESOLV_CONF_BAK="/etc/resolv.conf.pre-waagent-install"

# Ensure cleanup and sync always run, even if the script errors (bash -e).
# This guarantees resolv.conf is restored, VHD build files are removed,
# and writes are flushed before VHD capture regardless of success or failure.
cleanup() {
    # Restore /etc/resolv.conf to its exact pre-script state so the VHD ships clean.
    if [ -n "${RESOLV_CONF_ORIGINAL_STATE}" ]; then
        case "${RESOLV_CONF_ORIGINAL_STATE}" in
            symlink)
                # Restore the symlink target's content.
                if [ -f "${RESOLV_CONF_BAK}" ]; then
                    cp "${RESOLV_CONF_BAK}" "${RESOLV_CONF_SYMLINK_RESOLVED}" 2>/dev/null || true
                else
                    # Target existed but was empty before — truncate back to empty.
                    : > "${RESOLV_CONF_SYMLINK_RESOLVED}" 2>/dev/null || true
                fi
                # Reconstruct the symlink with its original raw value, in case anything
                # replaced it with a regular file during the script run.
                rm -f /etc/resolv.conf
                ln -sf "${RESOLV_CONF_SYMLINK_RAW}" /etc/resolv.conf
                echo "Restored /etc/resolv.conf symlink -> ${RESOLV_CONF_SYMLINK_RAW}"
                ;;
            file)
                mv "${RESOLV_CONF_BAK}" /etc/resolv.conf
                echo "Restored /etc/resolv.conf (regular file)"
                ;;
            absent)
                rm -f /etc/resolv.conf
                echo "Removed /etc/resolv.conf (did not exist before script ran)"
                ;;
        esac
        rm -f "${RESOLV_CONF_BAK}"
    fi
    rm -f /opt/azure/containers/install_walinuxagent.py
    rm -f /opt/azure/containers/post-deprovision-walinuxagent.sh
    sync
}
trap cleanup EXIT

# Skip on AzureLinux OSGuard which uses its OS-packaged waagent version.
# Flatcar and ACL are excluded at the packer config level (their JSONs do not call this).
OS_VARIANT_ID=$(. /etc/os-release 2>/dev/null && echo "${VARIANT_ID:-}" | tr '[:lower:]' '[:upper:]' | tr -d '"')
if [ "$OS_VARIANT_ID" != "OSGUARD" ]; then

    # Configuration
    WALINUXAGENT_DOWNLOAD_DIR="/opt/walinuxagent/downloads"
    WALINUXAGENT_WIRESERVER_URL="http://168.63.129.16:80"
    COMPONENTS_FILEPATH="/opt/azure/components.json"

    # Read WALinuxAgent version from components.json.
    WALINUXAGENT_VERSION=$(jq -r '.Packages[] | select(.name == "walinuxagent") | .downloadURIs.default.current.versionsV2[0].latestVersion' "${COMPONENTS_FILEPATH}")
    if [ -z "${WALINUXAGENT_VERSION}" ] || [ "${WALINUXAGENT_VERSION}" = "null" ] || [ "${WALINUXAGENT_VERSION}" = "<SKIP>" ]; then
        echo "ERROR: Could not read walinuxagent version from ${COMPONENTS_FILEPATH}" >&2
        exit 1
    fi
    echo "WALinuxAgent target version from components.json: ${WALINUXAGENT_VERSION}"

    # DNS will be broken on AzLinux after deprovision because
    # 'waagent -deprovision' clears /etc/resolv.conf.
    # Temporarily restore Azure DNS for manifest download.
    #
    # We snapshot the exact state of /etc/resolv.conf (symlink with raw value,
    # regular file, or absent) so the cleanup trap can reconstruct it precisely.
    if [ ! -s /etc/resolv.conf ] || ! grep -q nameserver /etc/resolv.conf; then
        if [ -L /etc/resolv.conf ]; then
            # Symlink — save the raw link value (may be relative) and the resolved path.
            RESOLV_CONF_ORIGINAL_STATE="symlink"
            RESOLV_CONF_SYMLINK_RAW=$(readlink /etc/resolv.conf)
            RESOLV_CONF_SYMLINK_RESOLVED=$(readlink -f /etc/resolv.conf)
            # Back up the content of the symlink target (not the link itself).
            cp "${RESOLV_CONF_SYMLINK_RESOLVED}" "${RESOLV_CONF_BAK}" 2>/dev/null || true
            # Write temporary nameserver to the target file, preserving the symlink.
            echo "nameserver 168.63.129.16" > "${RESOLV_CONF_SYMLINK_RESOLVED}"
        elif [ -e /etc/resolv.conf ]; then
            # Regular file (possibly empty).
            RESOLV_CONF_ORIGINAL_STATE="file"
            cp /etc/resolv.conf "${RESOLV_CONF_BAK}"
            echo "nameserver 168.63.129.16" > /etc/resolv.conf
        else
            # File does not exist at all.
            RESOLV_CONF_ORIGINAL_STATE="absent"
            echo "nameserver 168.63.129.16" > /etc/resolv.conf
        fi
        echo "Temporarily set DNS to Azure DNS for manifest download"
    fi

    # Install WALinuxAgent from wireserver manifest using the version from components.json.
    # Uses a standalone Python script (stdlib only) for wireserver HTTP, XML parsing,
    # and zip extraction.
    python3 /opt/azure/containers/install_walinuxagent.py "${WALINUXAGENT_DOWNLOAD_DIR}" "${WALINUXAGENT_WIRESERVER_URL}" "${WALINUXAGENT_VERSION}"

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

    # Log the installed version to VHD release notes
    VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
    echo "  - WALinuxAgent version ${WALINUXAGENT_VERSION}" >> ${VHD_LOGS_FILEPATH}

else
    echo "Skipping WALinuxAgent manifest install on AzureLinux OSGuard"
fi
