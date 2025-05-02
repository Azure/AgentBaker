#!/bin/bash

echo "Sourcing tool_installs_mariner.sh"

installAscBaseline() {
   echo "Mariner TODO: installAscBaseline"
}

installBcc() {
    echo "Installing BCC tools..."
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_install 120 5 25 bcc-tools || exit $ERR_BCC_INSTALL_TIMEOUT
    echo "Installing BCC examples..."
    dnf_install 120 5 25 bcc-examples || exit $ERR_BCC_INSTALL_TIMEOUT
}

installBpftrace() {
    echo "Installing bpftrace ..."
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_install 120 5 25 bpftrace || exit $ERR_BCC_INSTALL_TIMEOUT
}

addMarinerNvidiaRepo() {
    if [ "$OS_VERSION" = "2.0" ]; then 
        MARINER_NVIDIA_REPO_FILEPATH="/etc/yum.repos.d/mariner-nvidia.repo"
        touch "${MARINER_NVIDIA_REPO_FILEPATH}"
        cat << EOF > "${MARINER_NVIDIA_REPO_FILEPATH}"
[mariner-official-nvidia]
name=CBL-Mariner Official Nvidia 2.0 x86_64
baseurl=https://packages.microsoft.com/cbl-mariner/2.0/prod/nvidia/x86_64
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
    fi

  if [ "$OS_VERSION" = "3.0" ]; then
        AZURELINUX_NVIDIA_REPO_FILEPATH="/etc/yum.repos.d/azurelinux-nvidia.repo"
        touch "${AZURELINUX_NVIDIA_REPO_FILEPATH}"
        cat << EOF > "${AZURELINUX_NVIDIA_REPO_FILEPATH}"
[azurelinux-official-nvidia]
name=Azure Linux Official Nvidia 3.0 x86_64
baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/nvidia/x86_64/
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
    fi
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

# The default 99-dhcp-en config on Mariner/AzureLinux attempts to assign an IP address
# to the eth1 virtual function device, which delays cluster setup by 2 minutes.
# This workaround makes it so that dhcp is only enabled on eth0.
setMarinerNetworkdConfig() {
    CONFIG_FILEPATH="/etc/systemd/network/99-dhcp-en.network"
    touch ${CONFIG_FILEPATH}
    cat << EOF > ${CONFIG_FILEPATH} 
    [Match]
    Name=eth0

    [Network]
    DHCP=yes
    IPv6AcceptRA=no
EOF
# On Mariner/AzureLinux Marketplace images, the default systemd network config
# has an additional change that prevents Mariner/AzureLinux from changing IP addresses
# every reboot
    cat << EOF >> ${CONFIG_FILEPATH}

    [DHCPv4]
    UseDomains=true
    SendRelease=false
EOF
}


listInstalledPackages() {
    rpm -qa
}

# disable and mask all UU timers/services
disableDNFAutomatic() {
    # Make sure dnf-automatic is running with the notify timer rather than the auto install timer
    systemctlEnableAndStart dnf-automatic-notifyonly.timer 30 || exit $ERR_SYSTEMCTL_START_FAIL

    # Ensure the automatic install timer is disabled. 
    # systemctlDisableAndStop adds .service to the end which doesn't work on timers.
    systemctl disable dnf-automatic-install.service || exit 1
    systemctl mask dnf-automatic-install.service || exit 1

    systemctl stop dnf-automatic-install.timer || exit 1
    systemctl disable dnf-automatic-install.timer || exit 1
    systemctl mask dnf-automatic-install.timer || exit 1
}

disableTimesyncd() {
    # Disable and Mask timesyncd to prevent it from interfering with chronyd's work
    systemctl stop systemd-timesyncd || exit 1
    systemctl disable systemd-timesyncd || exit 1
    systemctl mask systemd-timesyncd || exit 1

    # Update chrony configuration
    cat > /etc/chrony/chrony.conf <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usable directives.

# Load configuration file dropins
confdir /etc/chrony/conf.d /etc/chrony/conf.override.d

# Load NTP sources
sourcedir /etc/chrony/sources.d /etc/chrony/sources.override.d

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Dump characteristics on sources when shut down to speed sync on restart.
dumpdir /var/lib/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# Local clocks might not be very good.
maxclockerror 10.0

# Allow combining of sources with different latencies. This allows for a 
# difference of 50x in the source latencies. Since the PTP Hyper-V clock always
# reports a delay of 5ms, this will allow an NTP server with up to 250ms
# latency to be combined in. The sources line that is configured by default in
# AKS has maxdelay 100, so no servers should actually be above that.
combinelimit 50

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Step the system clock instead of slewing it if the adjustment is larger than
# one second, with no limit to how many clock updates have occurred. This allows
# for rapid clock fixing after a VM freeze event.
makestep 1.0 -1

# Get TAI-UTC offset and leap seconds from the system tz database.
# This directive must be commented out when using time sources serving
# leap-smeared time.
leapsectz right/UTC

# Allow hwtimestamp if the NIC supports it.
hwtimestamp * minpoll -1 rxfilter all

# Log files location.
logdir /var/log/chrony

# Uncomment the following line to turn logging on.
#log tracking measurements statistics rtc

# Don't output the log banner lines (headers)
#logbanner 0

# Azure hosts are synchronized to internal Microsoft time servers that
# take their time from Microsoft-owned Stratum 1 devices.  The Hyper-V
# drivers surface this time source as a PTP-based time source in the
# guest. This configures chrony to use it.  This also causes chronyd
# to require the /dev/ptp_hyperv device; chronyd will fail to start if
# it is not present. If this line is removed (so chronyd no longer
# uses the /dev/ptp_hyperv device), also remove (or comment out) the
# /etc/systemd/system/chronyd.service.d/wait-for-ptp-hyperv.conf file.
#
# poll 3: get a measurement every 8s from the ptp clock
# dpoll -1: poll the clock every 2^-1s (0.5s) for a measurement; all of
#   the measurements between polls will be smoothed into a single 
#   measurement
# offset 0: don't add a fixed offset
# stratum 3: Azure hosts sync via NTP to stratum 2 time servers that
#   then sync from stratum 1 GPS devices, so this source is stratum 3.
# delay 0.1: the Hyper-V ptp device doesn't communicate its root delay,
#   which throws off the math in chrony. Average one-way delay is 0.4ms.
#   The delay here is for the round trip, so it's doubled and rounded
#   up to 1ms to make the math work well.
refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0 stratum 3 delay 0.01

# Add the closest twc host as determined by Azure Traffic Manager
server time.windows.com iburst burst minpoll 4 maxpoll 8 prefer

# Add the pool so we have more backups, albeit further away, but with a
# maxdelay to stop too much latency from occurring
pool pool.time.windows.com iburst burst minpoll 4 maxpoll 8 maxsources 3 maxdelay 100 prefer

# Temporary fix as of 2025/05 - the Azure PHC clock can experience some
# drift issues. While this is corrected on the backend, the NTP sources
# are set to "prefer", meaning that if NTP servers are available the 
# PHC will not be used.
EOF
    
    # Before we return, make sure that chronyd is running
    systemctlEnableAndStart chronyd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

# Regardless of UU mode, ensure check-restart is running
enableCheckRestart() {
  # Even if UU is disabled, we should still run check-restart so that kured
  # will work as expected if it is installed.
  # At 8:000:00 UTC check if a reboot-required package was installed
  # Touch /var/run/reboot-required if a reboot required package was installed.
  # This helps avoid a Mariner/AzureLinux specific reboot check command in kured.
  systemctlEnableAndStart check-restart.timer 30 || exit $ERR_SYSTEMCTL_START_FAIL
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
# This occurs because the umask in Mariner/AzureLinux is 0027 and packer_source.sh created the folder
# Future base images will already have rsyslog installed with 755 /etc/rsyslog.d
    chmod 755 /etc/rsyslog.d
}

enableMarinerKata() {
    echo "Contents of blkid output"
    my_blkid=$(blkid)
    export my_blkid
    echo $my_blkid

    boot_uuid=$(sudo grep -o -m 1 '[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}' /boot/efi/boot/grub2/grub.cfg)
    export boot_uuid

    sudo sed -i -e 's@load_env -f \$bootprefix\/mariner.cfg@load_env -f \$bootprefix\/mariner-mshv.cfg\nload_env -f $bootprefix\/mariner.cfg\n@'  /boot/grub2/grub.cfg
    sudo sed -i -e 's@menuentry "CBL-Mariner"@menuentry "Dom0" {\n    search --no-floppy --set=root --file /HvLoader.efi\n    chainloader /HvLoader.efi lxhvloader.dll MSHV_ROOT=\\\\Windows MSHV_ENABLE=TRUE MSHV_SCHEDULER_TYPE=ROOT MSHV_X2APIC_POLICY=ENABLE MSHV_SEV_SNP=TRUE MSHV_LOAD_OPTION=INCLUDETRACEMETADATA=1\n    boot\n    search --no-floppy --fs-uuid '"$boot_uuid"' --set=root\n    linux $bootprefix/$mariner_linux_mshv $mariner_cmdline_mshv $systemd_cmdline root=$rootdevice\n    if [ -f $bootprefix/$mariner_initrd_mshv ]; then\n    initrd $bootprefix/$mariner_initrd_mshv\n    fi\n}\n\nmenuentry "CBL-Mariner"@'  /boot/grub2/grub.cfg
}

activateNfConntrack() {
    # explicitly activate nf_conntrack module so associated sysctls can be properly set 
    echo nf_conntrack >> /etc/modules-load.d/contrack.conf
}

installFIPS() {

    echo "Installing FIPS..."

    # Install necessary rpm pacakages
    dnf_install 120 5 25 grubby || exit $ERR_APT_INSTALL_TIMEOUT
    dnf_install 120 5 25 dracut-fips || exit $ERR_APT_INSTALL_TIMEOUT

    # Add the boot= cmd line parameter if the boot dir is not the same as the root dir
    boot_dev="$(df /boot/ | tail -1 | cut -d' ' -f1)"
    root_dev="$(df / | tail -1 | cut -d' ' -f1)"
    if [ ! "$root_dev" = "$boot_dev" ]; then
        boot_uuid="UUID=$(blkid $boot_dev -s UUID -o value)"

        # Enable FIPS mode and modify boot directory
        if grub2-editenv - list | grep -q kernelopts;then
                grub2-editenv - set "$(grub2-editenv - list | grep kernelopts) fips=1 boot=$boot_uuid"
        else
                grubby --update-kernel=ALL --args="fips=1 boot=$boot_uuid"
        fi
    fi
    
}
