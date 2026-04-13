#!/bin/bash

echo "Sourcing tool_installs_acl.sh"

stub() {
    echo "${FUNCNAME[1]} stub"
}

installBcc() {
    stub
}

installBpftrace() {
    stub
}

listInstalledPackages() {
    stub
}

disableNtpAndTimesyncdInstallChrony() {
    # On ACL, chronyd is preinstalled and ntp does not exist, so we only need to
    # mask timesyncd (to prevent conflicts) and enable+start chronyd.

    systemctl stop systemd-timesyncd || exit 1
    systemctl disable systemd-timesyncd || exit 1
    systemctl mask systemd-timesyncd || exit 1

    systemctlEnableAndStart chronyd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

# Removing /etc/machine-id triggers systemd first-boot detection which runs
# preset-all on every unit with [Install]. ACL builds systemd with
# -Dfirst-boot-full-preset=true, so both enable and disable rules apply.
# The 'disable *' catch-all will actively disable every unit not explicitly
# listed with 'enable'.
#
# This preset must live in /etc/systemd/system-preset/ because:
# 1. It is visible at earliest boot, before systemd-sysext merges /usr/lib/.
# 2. /etc/ takes priority over /usr/lib/ per systemd lookup order.
# 3. It persists in the VHD across reboots.
#
# The allowlist below covers two categories:
# - AKS services enabled during VHD build or CSE whose unit files exist on the
#   VHD at first-boot time and are NOT already covered by a higher-priority
#   OS preset (90-default.preset, 90-systemd.preset).
# - OS services not in any preset but required for boot (systemd-sysext,
#   ensure-sysext).
#
# Services already covered by 90-default.preset (e.g. chronyd, waagent,
# logrotate.timer) do NOT need to be listed here — they match at higher
# priority before our file is consulted.
#
# systemd-sysext.service and ensure-sysext.service are critical: without them
# the sysext overlay that provides kubelet/kubectl binaries never merges,
# and kubelet stays stuck in "activating" state.
configureFirstBootPresets() {
    systemctl stop docker.socket || true
    systemctl mask docker.socket || true

    # Mask ignition-file-extract and ignition-bootcmds so that first-boot
    # preset-all cannot re-enable them. Ignition recreates 20-ignition.preset
    # at every boot (in initramfs), so deleting the preset file is not enough.
    # Masking (symlink to /dev/null) prevents systemctl enable from working
    # regardless of preset rules.
    # Without this, the tar extraction overwrites VHD scripts with
    # baker-rendered versions that may use wrong install methods (e.g.
    # rpm2cpio instead of mergeSysexts for ACL BYOI).
    systemctl mask ignition-file-extract.service || true
    systemctl mask ignition-bootcmds.service || true

    mkdir -p /etc/systemd/system-preset
    cat > /etc/systemd/system-preset/99-default-disable.preset <<'EOF'
# AKS services (not covered by OS presets)
enable aks-node-controller.service
enable disk_queue.service
enable ci-syslog-watcher.path
enable ci-syslog-watcher.service
enable update_certs.path
enable aks-log-collector.timer
enable sync-container-logs.service
enable cgroup-memory-telemetry.timer
enable cgroup-pressure-telemetry.timer
enable resolv-uplink-override.service
enable snapshot-update.timer
enable measure-tls-bootstrapping-latency.service

# OS services not in any preset but required for sysext overlay
enable systemd-sysext.service
enable ensure-sysext.service

disable *
EOF
}
