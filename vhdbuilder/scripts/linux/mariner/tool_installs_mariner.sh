#!/bin/bash

echo "Sourcing tool_installs_mariner.sh"

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
        cat << 'EOF' > "${AZURELINUX_NVIDIA_REPO_FILEPATH}"
[azurelinux-official-nvidia]
name=Azure Linux Official Nvidia 3.0 $basearch
baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/nvidia/$basearch/
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
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

# Azure Linux 3.0 only (Mariner 2.0 reports ID=mariner and is not targeted). The build-time dnf update installs a
# newer "kernel" next to the base image kernel and reboots into it, leaving the superseded kernel on the VHD where
# customer scanners report its CVEs.
# Safeguards:
# - Only the package named exactly "kernel" is removed, and only when it owns the running kernel and the running kernel
#   is its single newest install. Kata (kernel-mshv) and OSGuard (kernel-ipe) boot other packages and are skipped.
#   kernel-hwe (ARM64 dual boot), kernel-uki (CVM), kernel-uvm and installkernel are never targeted.
# - rpm -e is used instead of dnf so nothing is autoremoved, and a kernel that another package requires is kept.
# - The kernel %postun only regenerates a grub2-mkconfig generated GRUB config, so an old kernel referenced by any
#   other GRUB config is kept. The regeneration ignores grub2-mkconfig failures, so the GRUB config is validated.
# History:
# - https://github.com/Azure/AgentBaker/pull/3196 added old kernel removal with dnf autoremove.
# - https://github.com/Azure/AgentBaker/pull/3220 and https://github.com/Azure/AgentBaker/pull/3236 reverted it
#   because autoremove also removed installkernel and broke provisioning.
# - https://github.com/Azure/AgentBaker/pull/3223 added the pre-reboot dnf update that installs the newer kernel.
removeOldAzureLinuxKernels() {
    local boot_dir="${BOOT_DIR:-/boot}"
    local grub_config="${boot_dir}/grub2/grub.cfg"
    local package_format='%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH} %{VERSION}-%{RELEASE}'
    local running_release running_owner running_kernel installed_kernels newest_time newest_kernels
    local install_time old_kernel old_release test_output removed_releases=""
    local grub_generated=false grub_has_running=false

    running_release=$(uname -r)
    running_owner=$(rpm -qf --queryformat "%{NAME} ${package_format}\n" "${boot_dir}/vmlinuz-${running_release}" 2>/dev/null) || running_owner=""
    if [ "${running_owner%% *}" != "kernel" ] || [ "$(wc -l <<< "$running_owner")" -ne 1 ]; then
        echo "Running kernel ${running_release} is not owned by the 'kernel' package (${running_owner:-unowned}); skipping old kernel removal"
        return 0
    fi
    running_kernel=$(cut -d' ' -f2 <<< "$running_owner")

    installed_kernels=$(rpm -q --queryformat "%{INSTALLTIME} ${package_format}\n" kernel) || return 1
    newest_time=$(cut -d' ' -f1 <<< "$installed_kernels" | sort -n | tail -n 1)
    newest_kernels=$(awk -v time="$newest_time" '$1 == time { print $2 }' <<< "$installed_kernels")
    if [ "$newest_kernels" != "$running_kernel" ]; then
        echo "Running kernel ${running_kernel} is not the only newest installed kernel (${newest_kernels//$'\n'/ }); skipping old kernel removal"
        return 0
    fi

    if [ -f "$grub_config" ]; then
        grep -q "automatically generated by grub2-mkconfig" "$grub_config" && grub_generated=true
        grep -Fq "vmlinuz-${running_release}" "$grub_config" && grub_has_running=true
    fi

    while read -r install_time old_kernel old_release; do
        if [ "$old_kernel" = "$running_kernel" ]; then
            continue
        fi
        if [ "$grub_generated" = false ] && [ -f "$grub_config" ] && grep -Fq "vmlinuz-${old_release}" "$grub_config"; then
            echo "Keeping ${old_kernel} because ${grub_config} references it and is not generated by grub2-mkconfig"
            continue
        fi
        if ! test_output=$(rpm -e --test "$old_kernel" 2>&1); then
            echo "Keeping ${old_kernel} because other packages depend on it: ${test_output}"
            continue
        fi
        echo "Removing superseded kernel ${old_kernel} installed at ${install_time} (running ${running_kernel})"
        rpm -e "$old_kernel" || return 1
        removed_releases+=" ${old_release}"
    done <<< "$installed_kernels"

    if [ -z "$removed_releases" ]; then
        return 0
    fi
    if [ ! -s "${boot_dir}/vmlinuz-${running_release}" ] || ! rpm -q "$running_kernel" >/dev/null; then
        echo "Running kernel ${running_kernel} is missing after old kernel removal" >&2
        return 1
    fi
    if [ "$grub_has_running" = true ]; then
        if ! grep -Fq "vmlinuz-${running_release}" "$grub_config"; then
            echo "${grub_config} no longer references running kernel ${running_release} after old kernel removal" >&2
            return 1
        fi
        if command -v grub2-script-check >/dev/null && ! grub2-script-check "$grub_config"; then
            echo "${grub_config} is invalid after old kernel removal" >&2
            return 1
        fi
    fi
    if [ "$grub_generated" = true ]; then
        for old_release in $removed_releases; do
            if grep -Fq "vmlinuz-${old_release}" "$grub_config"; then
                echo "${grub_config} still references removed kernel ${old_release}" >&2
                return 1
            fi
        done
    fi
}

# disable and mask all UU timers/services
disableDNFAutomatic() {
    # Ensure the automatic notifyonly timer is disabled.
    systemctl stop dnf-automatic-notifyonly.timer || exit 1
    systemctl disable dnf-automatic-notifyonly.timer || exit 1
    systemctl mask dnf-automatic-notifyonly.timer || exit 1

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

# Remove lockdown=integrity from kernel cmdline for Azure Linux 3.0.
# The AzureLinux 3.0 kernel has an OOT patch that automatically enables
# lockdown when secure boot is detected.
disableKernelLockdownCmdline() {
    echo "Removing lockdown=integrity from kernel cmdline..."
    if [ -f /etc/default/grub ]; then
        sed -i 's/lockdown=integrity//g' /etc/default/grub
        grub2-mkconfig -o /boot/grub2/grub.cfg || exit 1
    else
        echo "Warning: /etc/default/grub not found, skipping lockdown removal"
    fi
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
