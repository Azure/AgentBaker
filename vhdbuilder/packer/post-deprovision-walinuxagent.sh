#!/bin/bash -eu
# Post-deprovision WALinuxAgent install script.
# Called by packer inline block AFTER 'waagent -force -deprovision+user',
# which clears /var/lib/waagent/. This script re-installs the latest
# WALinuxAgent from the wireserver GAFamily manifest so the agent daemon
# can pick it up locally without downloading at provisioning time.
#
# NOTE: -x is intentionally omitted to avoid leaking SAS tokens from
# wireserver manifest/blob URLs in packer build logs.

# ---- resolv.conf state tracking (read by the EXIT trap) ----
# On Ubuntu, /etc/resolv.conf is often a symlink (e.g. -> /run/systemd/resolve/resolv.conf).
# We must preserve the exact original state: the raw symlink value, the target's content,
# or the file's absence. Writing through a symlink mutates its target, and mv-ing a regular
# file over a symlink destroys the link — both would silently regress DNS on the captured VHD.
RESOLV_CONF_MODIFIED=false
RESOLV_CONF_ORIGINAL_STATE=""      # "symlink", "file", or "absent"
RESOLV_CONF_SYMLINK_RAW=""         # raw symlink value (preserves relative paths for ln -sf)
RESOLV_CONF_SYMLINK_RESOLVED=""    # fully resolved target path (for reading/writing content)
RESOLV_CONF_TARGET_EXISTED=false   # whether the symlink target file existed (for dangling symlinks)
RESOLV_CONF_BAK="/etc/resolv.conf.pre-waagent-install"

# Ensure cleanup and sync always run, even if the script errors (bash -e).
# This guarantees resolv.conf is restored, VHD build files are removed,
# and writes are flushed before VHD capture regardless of success or failure.
cleanup() {
    # Restore /etc/resolv.conf to its exact pre-script state so the VHD ships clean.
    if [ "${RESOLV_CONF_MODIFIED}" = "true" ]; then
        case "${RESOLV_CONF_ORIGINAL_STATE}" in
            symlink)
                # Restore the symlink target's content.
                if [ -f "${RESOLV_CONF_BAK}" ]; then
                    cp "${RESOLV_CONF_BAK}" "${RESOLV_CONF_SYMLINK_RESOLVED}" 2>/dev/null || true
                elif [ "${RESOLV_CONF_TARGET_EXISTED}" = "true" ]; then
                    # Target existed but was empty before — truncate back to empty.
                    : > "${RESOLV_CONF_SYMLINK_RESOLVED}" 2>/dev/null || true
                else
                    # Dangling symlink — target did not exist before; remove what we created.
                    rm -f "${RESOLV_CONF_SYMLINK_RESOLVED}"
                fi
                # Reconstruct the symlink with its original raw value, in case anything
                # replaced it with a regular file during the script run.
                rm -f /etc/resolv.conf
                ln -sf "${RESOLV_CONF_SYMLINK_RAW}" /etc/resolv.conf
                echo "Restored /etc/resolv.conf symlink -> ${RESOLV_CONF_SYMLINK_RAW}"
                ;;
            file)
                if [ -f "${RESOLV_CONF_BAK}" ]; then
                    mv "${RESOLV_CONF_BAK}" /etc/resolv.conf
                else
                    # File existed but backup failed — restore as empty.
                    : > /etc/resolv.conf
                fi
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
# Flatcar is excluded at the packer config level (its JSON does not call this).
OS_VARIANT_ID=$(. /etc/os-release 2>/dev/null && echo "${VARIANT_ID:-}" | tr '[:lower:]' '[:upper:]' | tr -d '"')
if [ "$OS_VARIANT_ID" != "OSGUARD" ]; then

    # Configuration
    WALINUXAGENT_DOWNLOAD_DIR="/opt/walinuxagent/downloads"
    WALINUXAGENT_WIRESERVER_URL="http://168.63.129.16:80"

    # DNS will be broken on AzLinux after deprovision because
    # 'waagent -deprovision' clears /etc/resolv.conf.
    # Temporarily restore Azure DNS for manifest download.
    # Restoration is handled by the EXIT trap.
    #
    # We snapshot the exact state of /etc/resolv.conf (symlink with raw value,
    # regular file, or absent) so the cleanup trap can reconstruct it precisely.
    if [ ! -s /etc/resolv.conf ] || ! grep -q nameserver /etc/resolv.conf; then
        if [ -L /etc/resolv.conf ]; then
            # Symlink — save the raw link value (may be relative) and the resolved path.
            RESOLV_CONF_ORIGINAL_STATE="symlink"
            RESOLV_CONF_SYMLINK_RAW=$(readlink /etc/resolv.conf)
            RESOLV_CONF_SYMLINK_RESOLVED=$(readlink -f /etc/resolv.conf)
            # Track whether the target file actually exists (handles dangling symlinks).
            if [ -e "${RESOLV_CONF_SYMLINK_RESOLVED}" ]; then
                RESOLV_CONF_TARGET_EXISTED=true
                cp "${RESOLV_CONF_SYMLINK_RESOLVED}" "${RESOLV_CONF_BAK}" 2>/dev/null || true
            fi
            # Write temporary nameserver to the target file, preserving the symlink.
            echo "nameserver 168.63.129.16" > "${RESOLV_CONF_SYMLINK_RESOLVED}"
        elif [ -e /etc/resolv.conf ]; then
            # Regular file (possibly empty).
            RESOLV_CONF_ORIGINAL_STATE="file"
            cp /etc/resolv.conf "${RESOLV_CONF_BAK}" 2>/dev/null || true
            echo "nameserver 168.63.129.16" > /etc/resolv.conf
        else
            # File does not exist at all.
            RESOLV_CONF_ORIGINAL_STATE="absent"
            echo "nameserver 168.63.129.16" > /etc/resolv.conf
        fi
        RESOLV_CONF_MODIFIED=true
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

    echo "WALinuxAgent installed and waagent.conf configured post-deprovision"

else
    echo "Skipping WALinuxAgent manifest install on AzureLinux OSGuard"
fi
