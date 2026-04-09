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
# preset-all on every unit with [Install]. ACL's 'disable *' catch-all lives
# in the oem-azure sysext, invisible at early boot before systemd-sysext merges.
# Write it to /etc/ instead — visible at earliest boot, persists in the VHD,
# and takes priority over /usr/lib/ per systemd lookup order.
#
# Services explicitly enabled during VHD build (pre-install-dependencies.sh)
# must be listed here with 'enable' BEFORE the 'disable *' catch-all,
# otherwise preset-all will disable them on first boot.
#
# containerd.service and kubelet.service are enabled later by CSE, but their
# unit files (with [Install] WantedBy=multi-user.target) are present on the
# VHD at first-boot time. Without explicit enable directives here, preset-all
# would disable them before CSE runs, causing kubelet to remain stuck in
# "activating" state (waiting for the containerd socket that never appears).
configureFirstBootPresets() {
    systemctl stop docker.socket || true
    systemctl mask docker.socket || true

    mkdir -p /etc/systemd/system-preset
    cat > /etc/systemd/system-preset/99-default-disable.preset <<'EOF'
enable aks-node-controller.service
enable containerd.service
enable kubelet.service
enable disk_queue.service
enable systemd-journald.service
enable update_certs.path
enable ci-syslog-watcher.path
enable ci-syslog-watcher.service
enable aks-log-collector.timer
enable logrotate.timer
enable sync-container-logs.service
enable chronyd.service
enable cgroup-memory-telemetry.timer
enable cgroup-pressure-telemetry.timer
enable resolv-uplink-override.service
disable *
EOF
}
