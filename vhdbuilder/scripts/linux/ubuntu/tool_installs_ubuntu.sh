#!/bin/bash
{{/* FIPS-related error codes */}}
ERR_UA_TOOLS_INSTALL_TIMEOUT=180 {{/* Timeout waiting for ubuntu-advantage-tools install */}}
ERR_ADD_UA_APT_REPO=181 {{/* Error to add UA apt repository */}}
ERR_UA_ATTACH=182 {{/* Error attaching UA */}}
ERR_UA_DISABLE_LIVEPATCH=183 {{/* Error to disable UA livepatch */}}
ERR_UA_ENABLE_FIPS=184 {{/* Error to enable UA FIPS */}}
ERR_UA_DETACH=185 {{/* Error to detach UA */}}
ERR_LINUX_HEADER_INSTALL_TIMEOUT=186 {{/* Timeout to install linux header */}}
ERR_STRONGSWAN_INSTALL_TIMEOUT=187 {{/* Timeout to install strongswan */}}

ERR_NTP_INSTALL_TIMEOUT=10 {{/*Unable to install NTP */}}
ERR_NTP_START_TIMEOUT=11 {{/* Unable to start NTP */}}
ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT=12 {{/* Timeout waiting for systemd-timesyncd stop */}}
ERR_STOP_OR_DISABLE_NTP_TIMEOUT=13 {{/* Timeout waiting for ntp stop */}}
ERR_CHRONY_INSTALL_TIMEOUT=14 {{/*Unable to install CHRONY */}}
ERR_CHRONY_START_TIMEOUT=15 {{/* Unable to start CHRONY */}}


echo "Sourcing tool_installs_ubuntu.sh"

installAscBaseline() {
   echo "Installing ASC Baseline tools..."
   ASC_BASELINE_TMP=/home/packer/asc-baseline.deb
   retrycmd_silent 120 5 25 dpkg -i $ASC_BASELINE_TMP || exit $ERR_APT_INSTALL_TIMEOUT
   cd /opt/microsoft/asc-baseline
   sudo ./ascbaseline -d baselines
   sudo ./ascremediate -d baselines -m all
   sudo ./ascbaseline -d baselines | grep -B2 -A6 "FAIL"
   cd -
   echo "Check UDF"
   cat /etc/modprobe.d/*.conf | grep udf
   echo "Finished Setting up ASC Baseline"
   apt_get_purge 20 30 120 asc-baseline || exit $ERR_APT_PURGE_TIMEOUT
}

installBcc() {
    echo "Installing BCC tools..."
    wait_for_apt_locks
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    if [ "${VERSION}" = "22.04" ] || [ "${VERSION}" = "24.04" ]; then
        apt_get_install 120 5 300 build-essential git bison cmake flex libedit-dev libllvm14 llvm-14-dev libclang-14-dev python3 zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    else
        apt_get_install 120 5 300 build-essential git bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev python zlib1g-dev libelf-dev python3-distutils libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # Installing it separately here because python3-distutils is not present in the Ubuntu packages for 24.04
    if [ "${VERSION}" = "22.04" ]; then
      apt_get_install 120 5 300 python3-distutils || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # libPolly.a is needed for the make target that runs later, which is not present in the default patch version of llvm-14 that is downloaded for 24.04
    if [ "${VERSION}" = "24.04" ]; then
      apt_get_install 120 5 300 libpolly-14-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    mkdir -p /tmp/bcc
    pushd /tmp/bcc
    git clone https://github.com/iovisor/bcc.git
    mkdir bcc/build; cd bcc/build

    if [ "${VERSION}" = "18.04" ]; then
      git checkout v0.24.0
    else
      # v0.24.0 is not supported for kernels 6.x and there are some python packages not available in 18.04 repository that are needed to build v0.24.0
      # Hence this distinction
      git checkout v0.29.0
    fi

    cmake -DENABLE_EXAMPLES=off .. || exit 1
    make
    sudo make install || exit 1
    cmake -DPYTHON_CMD=python3 .. || exit 1 # build python3 binding 
    pushd src/python/
    make
    sudo make install || exit 1
    popd
    popd
    # we explicitly do not remove build-essential or git
    # these are standard packages we want to keep, they should usually be in the final build anyway.
    # only ensuring they are installed above.
    if [ "${VERSION}" = "22.04" ] || [ "${VERSION}" = "24.04" ]; then
        apt_get_purge 120 5 300 bison cmake flex libedit-dev libllvm14 llvm-14-dev libclang-14-dev zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    else
        apt_get_purge 120 5 300 bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # libPolly.a is needed for the make target that runs later, which is not present in the default patch version of llvm-14 that is downloaded for 24.04
    if [ "${VERSION}" = "24.04" ]; then
      apt_get_purge 120 5 300 libpolly-14-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    rm -rf /tmp/bcc
}

installBpftrace() {
    local version="v0.9.4"
    local bpftrace_bin="bpftrace"
    local bpftrace_tools="bpftrace-tools.tar"
    local bpftrace_url="https://upstreamartifacts.azureedge.net/$bpftrace_bin/$version"
    local bpftrace_filepath="/usr/local/bin/$bpftrace_bin"
    local tools_filepath="/usr/local/share/$bpftrace_bin"
    if [ "$(isARM64)" -eq 1 ]; then
        # install bpftrace tool using default bpftrace apt package
        # the binary at "$bpftrace_url/$bpftrace_bin" is not for arm64
        if [ ! -f "/usr/sbin/bpftrace" ]; then
            apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
            apt_get_install 120 5 300 bpftrace || exit $ERR_BPFTRACE_TOOLS_INSTALL_TIMEOUT
        fi
        return
    fi

    if [ -f "$bpftrace_filepath" ]; then
        installed_version="$($bpftrace_bin -V | cut -d' ' -f2)"
        if [ "$version" = "$installed_version" ]; then
            return
        fi
        rm "$bpftrace_filepath"
        if [ -d "$tools_filepath" ]; then
            rm -r  "$tools_filepath"
        fi
    fi
    mkdir -p "$tools_filepath"
    install_dir="$BPFTRACE_DOWNLOADS_DIR/$version"
    mkdir -p "$install_dir"
    download_path="$install_dir/$bpftrace_tools"
    retrycmd_if_failure 30 5 60 curl -fSL -o "$bpftrace_filepath" "$bpftrace_url/$bpftrace_bin" || exit $ERR_BPFTRACE_BIN_DOWNLOAD_FAIL
    retrycmd_if_failure 30 5 60 curl -fSL -o "$download_path" "$bpftrace_url/$bpftrace_tools" || exit $ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL
    tar -xvf "$download_path" -C "$tools_filepath"
    chmod +x "$bpftrace_filepath"
    chmod -R +x "$tools_filepath/tools"
}

disableNtpAndTimesyncdInstallChrony() {
    # Disable systemd-timesyncd if present
    status=$(systemctl show -p SubState --value systemd-timesyncd)
    if [ "$status" = 'dead' ]; then
        echo "systemd-timesyncd is removed, no need to disable"
    else
        systemctl_stop 20 30 120 systemd-timesyncd || exit $ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT
        systemctl disable systemd-timesyncd || exit $ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT
    fi
    
    # Disable ntp if present
    status=$(systemctl show -p SubState --value ntp)
    if [ "$status" = 'dead' ]; then
        echo "ntp is removed, no need to disable"
    else
        systemctl_stop 20 30 120 ntp || exit $ERR_STOP_OR_DISABLE_NTP_TIMEOUT
        systemctl disable ntp || exit $ERR_STOP_OR_DISABLE_NTP_TIMEOUT
    fi

    # Install chrony
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_install 20 30 120 chrony || exit $ERR_CHRONY_INSTALL_TIMEOUT
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

    systemctlEnableAndStart chrony 30 || exit $ERR_CHRONY_START_TIMEOUT
}

installFIPS() {
    echo "Installing FIPS..."
    wait_for_apt_locks

    # installing fips kernel doesn't remove non-fips kernel now, purge current linux-image-azure
    echo "purging linux-image-azure..."
    linuxImages=$(apt list --installed | grep linux-image- | grep azure | cut -d '/' -f 1)
    for image in $linuxImages; do
        echo "Removing non-fips kernel ${image}..."
        if [ "${image}" != "linux-image-$(uname -r)" ]; then
            apt_get_purge 5 10 120 ${image} || exit 1
        fi
    done

    echo "enabling ua fips-updates..."
    retrycmd_if_failure 5 10 1200 yes | ua enable fips-updates || exit $ERR_UA_ENABLE_FIPS
}

relinkResolvConf() {
    # /run/systemd/resolve/stub-resolv.conf contains local nameserver 127.0.0.53
    # remove this block after toggle disable-1804-systemd-resolved is enabled prod wide
    resolvconf=$(readlink -f /etc/resolv.conf)
    # shellcheck disable=SC3010
    if [[ "${resolvconf}" == */run/systemd/resolve/stub-resolv.conf ]]; then
        unlink /etc/resolv.conf
        ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
    fi
}

listInstalledPackages() {
    apt list --installed
}

attachUA() {
    echo "attaching ua..."
    retrycmd_silent 5 10 120 ua attach $UA_TOKEN || exit $ERR_UA_ATTACH

    echo "disabling ua livepatch..."
    retrycmd_if_failure 5 10 300 echo y | ua disable livepatch
}

detachAndCleanUpUA() {
    echo "detaching ua..."
    retrycmd_if_failure 5 10 120 printf "y\nN" | ua detach || $ERR_UA_DETACH

    # now that the ESM/FIPS packages are installed, clean up apt settings in the vhd,
    # the VMs created on customers' subscriptions don't have access to UA repo
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-apps.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-infra-trusty.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-fips.gpg
    rm -f /etc/apt/sources.list.d/ubuntu-esm-apps.list
    rm -f /etc/apt/sources.list.d/ubuntu-esm-infra.list
    rm -f /etc/apt/sources.list.d/ubuntu-fips-updates.list
    rm -f /etc/apt/auth.conf.d/*ubuntu-advantage
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}
