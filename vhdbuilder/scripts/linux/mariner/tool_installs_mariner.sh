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

# There are several issues in default file permissions when trying to run AMA and ASA extensions.
# These will be resolved in an upcoming base image. Work around them here for now.
fixCBLMarinerPermissions() {
# Set the dmi/id/product_uuid permissions to 444 so that mdsd can read the VM unique ID
# https://github.com/Azure/WALinuxAgent/blob/develop/config/99-azure-product-uuid.rules
# Future base images will include this config in the WALinuxAgent package.
    CONFIG_FILEPATH="/etc/udev/rules.d/99-azure-product-uuid.rules"
    touch ${CONFIG_FILEPATH}
    cat << EOF > ${CONFIG_FILEPATH}
SUBSYSTEM!="dmi", GOTO="product_uuid-exit"
ATTR{sys_vendor}!="Microsoft Corporation", GOTO="product_uuid-exit"
ATTR{product_name}!="Virtual Machine", GOTO="product_uuid-exit"
TEST!="/sys/devices/virtual/dmi/id/product_uuid", GOTO="product_uuid-exit"

RUN+="/bin/chmod 0444 /sys/devices/virtual/dmi/id/product_uuid"

LABEL="product_uuid-exit"

EOF

# /etc/rsyslog.d is 750 but should be 755 so non root users can read the configs
# This occurs because the umask in Mariner is 0027 and packer_source.sh created the folder
# Future base images will already have rsyslog installed with 755 /etc/rsyslog.d
    chmod 755 /etc/rsyslog.d
}
