#!/bin/bash

{{/* BCC/BPF-related error codes */}}
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 {{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 {{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 {{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 {{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 {{/* Failed to download bpftrace default programs */}}

BPFTRACE_DOWNLOADS_DIR="/opt/bpftrace/downloads"
UBUNTU_CODENAME=$(lsb_release -c -s)

installAscBaseline() {
  echo "Installing ASC Baseline tools..."
  ASC_BASELINE_TMP=asc-baseline.deb
  retrycmd_if_failure_no_stats 120 5 25 dpkg -i $ASC_BASELINE_TMP
  sudo cp /opt/microsoft/asc-baseline/baselines/oms_audits.xml /opt/microsoft/asc-baseline/oms_audits.xml
  cd /opt/microsoft/asc-baseline
  sudo ./ascbaseline -d .​
  sudo ./ascremediate -d . -m all​
  sudo ./ascbaseline -d . ​| grep -B2 -A6 "FAIL"
  cd -
  echo "Finished Setting up ASC Baseline"
}

installBcc() {
  echo "Installing BCC tools..."
  IOVISOR_KEY_TMP=/tmp/iovisor-release.key
  IOVISOR_URL=https://repo.iovisor.org/GPG-KEY
  retrycmd_if_failure_no_stats 120 5 25 curl -fsSL $IOVISOR_URL >$IOVISOR_KEY_TMP || exit $ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT
  wait_for_apt_locks
  retrycmd_if_failure 30 5 30 apt-key add $IOVISOR_KEY_TMP || exit $ERR_IOVISOR_APT_KEY_TIMEOUT
  echo "deb https://repo.iovisor.org/apt/${UBUNTU_CODENAME} ${UBUNTU_CODENAME} main" >/etc/apt/sources.list.d/iovisor.list
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  apt_get_install 120 5 25 bcc-tools libbcc-examples linux-headers-$(uname -r) || exit $ERR_BCC_INSTALL_TIMEOUT
  apt-key del "$(gpg --with-colons $IOVISOR_KEY_TMP 2>/dev/null | head -n 1 | cut -d':' -f5)"
  rm -f /etc/apt/sources.list.d/iovisor.list
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
      rm -r "$tools_filepath"
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
  echo blacklist nouveau >>/etc/modprobe.d/blacklist.conf
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
  retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
  mkdir -p $GPU_DEST/lib64 $GPU_DEST/overlay-workdir
  retrycmd_if_failure 120 5 25 mount -t overlay -o lowerdir=/usr/lib/x86_64-linux-gnu,upperdir=${GPU_DEST}/lib64,workdir=${GPU_DEST}/overlay-workdir none /usr/lib/x86_64-linux-gnu || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
  retrycmd_if_failure 3 1 600 sh $GPU_DEST/nvidia-drivers-$GPU_DV --silent --accept-license --no-drm --dkms --utility-prefix="${GPU_DEST}" --opengl-prefix="${GPU_DEST}" || exit $ERR_GPU_DRIVERS_START_FAIL
  mv ${GPU_DEST}/bin/* /usr/bin
  echo "${GPU_DEST}/lib64" >/etc/ld.so.conf.d/nvidia.conf
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
