#!/bin/bash

echo "Sourcing tool_installs_mariner.sh"

installAscBaseline() {
   echo "Mariner TODO: installAscBaseline"
}

installBcc() {
    echo "Installing BCC tools..."
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_install 120 5 25 bcc-tools || exit $ERR_BCC_INSTALL_TIMEOUT
}

configGPUDrivers() {
    echo "Not installing GPU drivers on Mariner"
}

forceEnableIpForward() {
    CONFIG_FILEPATH="/etc/sysctl.d/99-force-bridge-forward.conf"
    touch ${CONFIG_FILEPATH}
    cat << EOF >> ${CONFIG_FILEPATH}
    net.ipv4.ip_forward = 1
    net.ipv4.conf.all.forwarding = 1
    net.ipv6.conf.all.forwarding = 1
    net.bridge.bridge-nf-call-iptables = 1
EOF
}

# The default 99-dhcp-en config on Mariner attempts to assign an IP address
# to the eth1 virtual function device, which delays cluster setup by 2 minutes.
# This workaround makes it so that dhcp is only enabled on eth0.
networkdWorkaround() {
    sed -i "s/Name=e\*/Name=eth0/g" /etc/systemd/network/99-dhcp-en.network
}

listInstalledPackages() {
    rpm -qa
}

# By default the audit service is disabled on Mariner.
# Ensure that it is enabled explicitly to satisfy ASC scanning rules.
enableSystemdAuditd() {
    systemctlEnableAndStart auditd || exit $ERR_SYSTEMCTL_START_FAIL
}

# By default the dnf-automatic is service is notify only in Mariner.
# Enable the automatic install timer and the check-restart timer.
enableDNFAutomatic() {
    # Stop the notify only dnf timer since we've enabled the auto install one.
    # systemctlDisableAndStop adds .service to the end which doesn't work on timers.
    systemctl disable dnf-automatic-notifyonly.timer
    systemctl stop dnf-automatic-notifyonly.timer
    # At 6:00:00 UTC (1 hour random fuzz) download and install package updates.
    systemctlEnableAndStart dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
    # At 8:000:00 UTC check if a reboot-required package was installed
    # Touch /var/run/reboot-required if a reboot required pacakge was installed.
    # This helps avoid a Mariner specific reboot check command in kured.
    systemctlEnableAndStart check-restart.timer || exit $ERR_SYSTEMCTL_START_FAIL
}
