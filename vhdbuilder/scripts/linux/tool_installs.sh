#!/bin/bash
{{/* BCC/BPF-related error codes */}} 
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 {{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 {{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 {{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 {{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 {{/* Failed to download bpftrace default programs */}}
{{/* FIPS-related error codes */}}
ERR_UA_TOOLS_INSTALL_TIMEOUT=173 {{/* Timeout waiting for ubuntu-advantage-tools install */}}
ERR_ADD_UA_APT_REPO=174 {{/* Error to add UA apt repository */}}
ERR_AUTO_UA_ATTACH=175 {{/* Error to auto UA attach */}}
ERR_UA_DISABLE_LIVEPATCH=176 {{/* Error to disable UA livepatch */}}
ERR_UA_ENABLE_FIPS=177 {{/* Error to enable UA FIPS */}}
ERR_UA_DETACH=178 {{/* Error to detach UA */}}
LINUX_HEADER_INSTALL_TIMEOUT=179 {{/* Timeout to install linux header */}}
ERR_STRONGSWAN_INSTALL_TIMEOUT=180 {{/* Timeout to install strongswan */}}

BPFTRACE_DOWNLOADS_DIR="/opt/bpftrace/downloads"
UBUNTU_CODENAME=$(lsb_release -c -s)

installAscBaseline() {
   echo "Installing ASC Baseline tools..."
   ASC_BASELINE_TMP=asc-baseline.deb
   retrycmd_if_failure_no_stats 120 5 25 dpkg -i $ASC_BASELINE_TMP
   sudo cp /opt/microsoft/asc-baseline/baselines/oms_audits.xml /opt/microsoft/asc-baseline/oms_audits.xml
   cd /opt/microsoft/asc-baseline
   sudo ./ascbaseline -d .
   sudo ./ascremediate -d . -m all
   sudo ./ascbaseline -d . ​| grep -B2 -A6 "FAIL"
   cd -
   echo "Check UDF"
   cat /etc/modprobe.d/*.conf | grep udf
   echo "Finished Setting up ASC Baseline"
}

installBcc() {
    echo "Installing BCC tools..."
    wait_for_apt_locks
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    apt_get_install 120 5 300 build-essential git bison cmake flex  libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev python zlib1g-dev libelf-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    if [ "$VERSION" = "18.04" ]; then
        apt_get_install 120 5 300 python3-distutils libfl-dev
    fi
    mkdir -p /tmp/bcc
    pushd /tmp/bcc
    git clone https://github.com/iovisor/bcc.git
    mkdir bcc/build; cd bcc/build
    cmake ..
    make
    sudo make install
    cmake -DPYTHON_CMD=python3 .. # build python3 binding
    pushd src/python/
    make
    sudo make install
    popd
    popd
    # we explicitly do not remove build-essential or git
    # these are standard packages we want to keep, they should usually be in the final build anyway.
    # only ensuring they are installed above.
    apt_get_purge 120 5 300 bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev zlib1g-dev libelf-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    if [ "$VERSION" = "18.04" ]; then
        apt_get_purge 120 5 25 python3-distutils libfl-dev
    fi
}

installBpftrace() {
    local version="v0.9.4"
    local bpftrace_bin="bpftrace"
    local bpftrace_tools="bpftrace-tools.tar"
    local bpftrace_url="https://upstreamartifacts.azureedge.net/$bpftrace_bin/$version"
    local bpftrace_filepath="/usr/local/bin/$bpftrace_bin"
    local tools_filepath="/usr/local/share/$bpftrace_bin"
    if [[ -f "$bpftrace_filepath" ]]; then
        installed_version="$($bpftrace_bin -V | cut -d' ' -f2)"
        if [[ "$version" == "$installed_version" ]]; then
            return
        fi
        rm "$bpftrace_filepath"
        if [[ -d "$tools_filepath" ]]; then
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

configGPUDrivers() {
    rmmod nouveau
    echo blacklist nouveau >> /etc/modprobe.d/blacklist.conf
    retrycmd_if_failure_no_stats 120 5 25 update-initramfs -u || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    tmpDir=$GPU_DEST/tmp
    (
      set -e -o pipefail
      cd "${tmpDir}"
      wait_for_apt_locks
      dpkg-deb -R ./nvidia-docker2*.deb "${tmpDir}/pkg" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
      cp -r ${tmpDir}/pkg/usr/* /usr/ || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    )
    rm -rf $GPU_DEST/tmp
    if [[ "${CONTAINER_RUNTIME}" == "docker" ]]; then
        retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    else
        retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
    mkdir -p $GPU_DEST/lib64 $GPU_DEST/overlay-workdir
    retrycmd_if_failure 120 5 25 mount -t overlay -o lowerdir=/usr/lib/x86_64-linux-gnu,upperdir=${GPU_DEST}/lib64,workdir=${GPU_DEST}/overlay-workdir none /usr/lib/x86_64-linux-gnu || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    retrycmd_if_failure 3 1 600 sh $GPU_DEST/nvidia-drivers-$GPU_DV --silent --accept-license --no-drm --dkms --utility-prefix="${GPU_DEST}" --opengl-prefix="${GPU_DEST}" || exit $ERR_GPU_DRIVERS_START_FAIL
    mv ${GPU_DEST}/bin/* /usr/bin
    echo "${GPU_DEST}/lib64" > /etc/ld.so.conf.d/nvidia.conf
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
    umount -l /usr/lib/x86_64-linux-gnu
    retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
}

ensureGPUDrivers() {
    configGPUDrivers
    systemctlEnableAndStart nvidia-modprobe || exit $ERR_GPU_DRIVERS_START_FAIL
}

disableSystemdTimesyncdAndEnableNTP() {
    # disable systemd-timesyncd
    systemctl_stop 20 30 120 systemd-timesyncd || exit $ERR_STOP_SYSTEMD_TIMESYNCD_TIMEOUT
    systemctl disable systemd-timesyncd

    # install ntp
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_install 20 30 120 ntp || exit $ERR_NTP_INSTALL_TIMEOUT

    # enable ntp
    systemctlEnableAndStart ntp || exit $ERR_NTP_START_TIMEOUT
}

installFIPS() {
    echo "Installing FIPS..."
    wait_for_apt_locks

    echo "adding ua repository..."
    retrycmd_if_failure 5 10 120 add-apt-repository -y ppa:ua-client/stable || exit $ERR_ADD_UA_APT_REPO
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    echo "installing ua tools..."
    apt_get_install 5 10 120 ubuntu-advantage-tools || exit $ERR_UA_TOOLS_INSTALL_TIMEOUT

    echo "auto attaching ua..."
    retrycmd_if_failure 5 10 120 ua auto-attach || exit $ERR_AUTO_UA_ATTACH

    echo "disabling ua livepatch..."
    retrycmd_if_failure 5 10 300 echo y | ua disable livepatch

    echo "enabling ua fips..."
    retrycmd_if_failure 5 10 1200 echo y | ua enable fips || exit $ERR_UA_ENABLE_FIPS

    echo "installing strongswan..."
    apt_get_install 5 10 120 strongswan || exit $ERR_STRONGSWAN_INSTALL_TIMEOUT

    # workaround to make GPU provisioning in CSE work
    # under /usr/src/linux-headers-4.15.0-1002-azure-fips there are some dangling symlinks to non-existing linux-azure-headers-4.15.0-1002
    # this causes command '/usr/sbin/dkms build -m nvidia -v 450.51.06 -k 4.15.0-1002-azure-fips' for GPU provisioning in CSE to fail
    # however linux-headers-4.15.0-1002-azure doesn't exist any more, install closest 1011 to workaround
    if [[ ! -d /usr/src/linux-azure-headers-4.15.0-1002 ]]; then
        apt_get_install 5 10 120 linux-headers-fips || exit $LINUX_HEADER_INSTALL_TIMEOUT
        ln -s /usr/src/linux-fips-headers-4.15.0-1011 /usr/src/linux-azure-headers-4.15.0-1002
    fi

    # now the fips packages/kernel are installed, clean up apt settings in the vhd,
    # the VMs created on customers' subscriptions don't have access to UA repo
    echo "detaching ua..."
    retrycmd_if_failure 5 10 120 echo y | ua detach || $ERR_UA_DETACH

    echo "removing ua tools..."
    apt_get_purge 5 10 120 ubuntu-advantage-tools
    rm -f /etc/apt/trusted.gpg.d/ua-client_ubuntu_stable.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-apps.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-infra-trusty.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-fips.gpg
    rm -f /etc/apt/sources.list.d/ua-client-ubuntu-stable-bionic.list
    rm -f /etc/apt/sources.list.d/ubuntu-esm-apps.list
    rm -f /etc/apt/sources.list.d/ubuntu-esm-infra.list
    rm -f /etc/apt/sources.list.d/ubuntu-fips.list
    rm -f /etc/apt/auth.conf.d/90ubuntu-advantage
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    resolvconf=$(readlink -f /etc/resolv.conf)
    # /run/systemd/resolve/stub-resolv.conf contains local nameserver 127.0.0.53
    if [[ "${resolvconf}" == */run/systemd/resolve/stub-resolv.conf ]]; then
        unlink /etc/resolv.conf
        ln -s /run/systemd/resolve/resolv.conf /etc/resolv.conf
    fi
}