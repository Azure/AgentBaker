#!/bin/bash
OS=$(sort -r /etc/*-release | sed -n 's/^ID=//p' | head -n1 | tr -d '"' | tr '[:lower:]' '[:upper:]')
OS_VERSION=$(sort -r /etc/*-release | sed -n 's/^VERSION_ID=//p' | head -n1 | tr -d '"' | tr '[:lower:]' '[:upper:]')
OS_VARIANT=$(sort -r /etc/*-release | sed -n 's/^VARIANT_ID=//p' | head -n1 | tr -d '"' | tr '[:lower:]' '[:upper:]')
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

#the following sed removes all comments of the format {{/* */}}
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/provision_source.sh
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/tool_installs_distro.sh

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_benchmarks.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
PERFORMANCE_DATA_FILE=/opt/azure/vhd-build-performance-data.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

if isMarinerOrAzureLinux "$OS" || isACL "$OS" "$OS_VARIANT"; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

installJq || echo "WARNING: jq installation failed, VHD Build benchmarks will not be available for this build."
capture_benchmark "${SCRIPT_NAME}_source_packer_files_and_declare_variables"

copyPackerFiles

# Install required dependencies needed to build minimal images if needed (currently only Ubuntu 26.04)
if isMinimalImage && isUbuntu "$OS"; then
  installMinimalBuildDeps
fi

# Temporary diagnostics to determine whether Intel TDX builders can expose the Azure host PTP clock.
if isUbuntu "$OS" && [ "$OS_VERSION" = "26.04" ] && grep -q "cvm-tdx-clock-test" <<< "${FEATURE_FLAGS:-}"; then
  set +e
  TDX_CLOCK_DIAGNOSTICS_LOG="/opt/azure/tdx-host-clock-diagnostics.log"
  mkdir -p "$(dirname "$TDX_CLOCK_DIAGNOSTICS_LOG")"
  : > "$TDX_CLOCK_DIAGNOSTICS_LOG"
  exec > >(tee -a "$TDX_CLOCK_DIAGNOSTICS_LOG") 2>&1

  dump_ptp_state() {
    local stage="$1"
    local ptp_path
    local ptp_name
    local clock_name

    echo "===== PTP state: ${stage} ====="
    ls -la /dev/ptp* 2>&1
    ls -la /sys/class/ptp 2>&1

    for ptp_path in /sys/class/ptp/ptp*; do
      [ -e "$ptp_path" ] || continue

      ptp_name="${ptp_path##*/}"
      clock_name="$(cat "${ptp_path}/clock_name" 2>/dev/null)"
      echo "${ptp_name}: clock_name=${clock_name:-unknown}"
      stat "/dev/${ptp_name}" 2>&1
      readlink -f "${ptp_path}/device/driver" 2>&1
      udevadm info --query=all --name="/dev/${ptp_name}" 2>&1
      find "$ptp_path" -maxdepth 1 -type f -print -exec sh -c 'printf "  "; cat "$1"' _ {} \; 2>&1
    done

    echo "Hyper-V PTP symlink:"
    ls -la /dev/ptp_hyperv 2>&1
    echo "===== End PTP state: ${stage} ====="
  }

  read_hyperv_phc() {
    python3 - <<'PY'
import ctypes
import glob
import os
import time


class Timespec(ctypes.Structure):
    _fields_ = [("tv_sec", ctypes.c_long), ("tv_nsec", ctypes.c_long)]


devices = []
for sysfs_path in glob.glob("/sys/class/ptp/ptp*"):
    try:
        with open(os.path.join(sysfs_path, "clock_name"), encoding="utf-8") as clock_name_file:
            if clock_name_file.read().strip() == "hyperv":
                devices.append("/dev/" + os.path.basename(sysfs_path))
    except OSError as error:
        print(f"Unable to inspect {sysfs_path}: {error}")

if os.path.exists("/dev/ptp_hyperv"):
    devices.insert(0, "/dev/ptp_hyperv")

devices = list(dict.fromkeys(os.path.realpath(device) for device in devices))
if not devices:
    print("No Hyper-V PHC device is available to read")
    raise SystemExit(1)

libc = ctypes.CDLL(None, use_errno=True)
success = False
for device in devices:
    try:
        fd = os.open(device, os.O_RDONLY)
    except OSError as error:
        print(f"{device}: open failed: {error}")
        continue

    try:
        # Linux dynamic POSIX clock ID derived from an open character-device file descriptor.
        clock_id = ((~fd) << 3) | 3
        sample = Timespec()
        if libc.clock_gettime(ctypes.c_int(clock_id), ctypes.byref(sample)) != 0:
            error_number = ctypes.get_errno()
            print(f"{device}: clock_gettime failed: {os.strerror(error_number)}")
            continue

        for sample_number in range(1, 4):
            if libc.clock_gettime(ctypes.c_int(clock_id), ctypes.byref(sample)) != 0:
                error_number = ctypes.get_errno()
                print(f"{device}: sample {sample_number} failed: {os.strerror(error_number)}")
                break

            phc_time = sample.tv_sec + sample.tv_nsec / 1_000_000_000
            system_time = time.time()
            print(
                f"{device}: sample={sample_number}, phc={phc_time:.9f}, "
                f"system={system_time:.9f}, offset_seconds={phc_time - system_time:.9f}"
            )
            success = True
            if sample_number < 3:
                time.sleep(1)
    finally:
        os.close(fd)

raise SystemExit(0 if success else 1)
PY
  }

  report_hyperv_phc_read() {
    local stage="$1"

    if read_hyperv_phc; then
      echo "PHC_READ_RESULT stage=${stage} result=readable"
    else
      echo "PHC_READ_RESULT stage=${stage} result=unavailable"
    fi
  }

  echo "===== Ubuntu 26.04 Intel TDX host-clock diagnostics ====="
  echo "Diagnostics log: ${TDX_CLOCK_DIAGNOSTICS_LOG}"
  echo "Timestamp: $(date --iso-8601=ns)"
  echo "OS release:"
  cat /etc/os-release 2>&1
  echo "Kernel: $(uname -a)"
  echo "Virtualization: $(systemd-detect-virt --vm 2>&1)"
  echo "CPU and TDX details:"
  lscpu 2>&1
  grep -Eio '(^| )(tdx_guest|hypervisor|constant_tsc|nonstop_tsc|tsc_reliable)( |$)' /proc/cpuinfo |
    sort -u 2>&1
  find /sys/firmware/tdx /sys/devices/virtual/misc/tdx_guest -maxdepth 3 -print 2>&1

  echo "Kernel clocksource state:"
  find /sys/devices/system/clocksource/clocksource0 -maxdepth 1 -type f -print \
    -exec sh -c 'printf "  "; cat "$1"' _ {} \; 2>&1

  echo "Azure instance metadata:"
  curl -fsS -H Metadata:true \
    "http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01" |
    jq '{location, name, platformFaultDomain, platformUpdateDomain, securityProfile, sku, vmId, vmScaleSetName, vmSize, zone}'

  echo "Relevant kernel configuration:"
  grep -Eh 'CONFIG_(HYPERV|HYPERV_UTILS|HYPERV_TIMER|HYPERV_TSCPAGE|PTP_1588_CLOCK)' \
    "/boot/config-$(uname -r)" 2>&1
  if [ -f /proc/config.gz ]; then
    zcat /proc/config.gz 2>/dev/null |
      grep -E 'CONFIG_(HYPERV|HYPERV_UTILS|HYPERV_TIMER|HYPERV_TSCPAGE|PTP_1588_CLOCK)'
  fi

  echo "Loaded Hyper-V and PTP modules:"
  lsmod | grep -E '^(hv_|hyperv|ptp)' || true
  echo "Hyper-V and PTP module metadata:"
  modinfo hv_utils 2>&1
  modinfo ptp 2>&1
  echo "Installed kernel and time synchronization packages:"
  dpkg-query -W 'linux-*' 'chrony*' 'systemd*' 2>&1 |
    grep -Ei 'linux-(image|modules|azure)|chrony|systemd'
  echo "Hyper-V VMBus devices and drivers:"
  find /sys/bus/vmbus/devices -maxdepth 2 -type l -name driver -print -exec readlink -f {} \; 2>&1
  find /sys/bus/vmbus/devices -maxdepth 2 -type f \
    \( -name class_id -o -name device_id -o -name modalias -o -name uevent \) \
    -print -exec sh -c 'printf "  "; cat "$1"' _ {} \; 2>&1
  find /sys/bus/vmbus/drivers -maxdepth 2 -print 2>&1

  dump_ptp_state "initial"
  echo "Initial Hyper-V PHC read:"
  report_hyperv_phc_read "initial"

  echo "Loading the PTP and Hyper-V utility drivers:"
  modprobe ptp 2>&1
  modprobe hv_utils 2>&1
  udevadm settle 2>&1
  dump_ptp_state "after modprobe"
  echo "Hyper-V PHC read after modprobe:"
  report_hyperv_phc_read "after_modprobe"

  echo "Reloading and triggering the image's existing udev rules:"
  udevadm control --reload-rules 2>&1
  udevadm trigger --subsystem-match=vmbus --action=add 2>&1
  udevadm trigger --subsystem-match=ptp --action=add 2>&1
  udevadm settle 2>&1
  dump_ptp_state "after existing udev rules"
  echo "Hyper-V PHC read after existing udev rules:"
  report_hyperv_phc_read "after_existing_udev_rules"

  echo "Existing Hyper-V PTP udev rules:"
  grep -RniE 'ptp_hyperv|ATTR\{clock_name\}.*hyperv' /usr/lib/udev/rules.d /lib/udev/rules.d /etc/udev/rules.d 2>&1

  echo "Installing and triggering the Microsoft-documented Hyper-V PTP udev rule:"
  cat > /etc/udev/rules.d/99-ptp_hyperv.rules <<'EOF'
ACTION!="add", GOTO="ptp_hyperv"
SUBSYSTEM=="ptp", ATTR{clock_name}=="hyperv", SYMLINK+="ptp_hyperv"
LABEL="ptp_hyperv"
EOF
  udevadm control --reload-rules 2>&1
  udevadm trigger --subsystem-match=ptp --action=add 2>&1
  udevadm settle 2>&1
  dump_ptp_state "after documented udev rule"
  echo "Hyper-V PHC read after documented udev rule:"
  report_hyperv_phc_read "after_documented_udev_rule"

  echo "Trying a direct symlink when the kernel exposed a Hyper-V PTP device:"
  for ptp_path in /sys/class/ptp/ptp*; do
    [ -e "$ptp_path" ] || continue
    if [ "$(cat "${ptp_path}/clock_name" 2>/dev/null)" = "hyperv" ]; then
      ln -sfn "/dev/${ptp_path##*/}" /dev/ptp_hyperv
      break
    fi
  done
  dump_ptp_state "final"
  echo "Final Hyper-V PHC read:"
  report_hyperv_phc_read "after_direct_symlink"

  echo "Testing the existing AgentBaker Chrony installation when the Hyper-V PHC is readable:"
  if read_hyperv_phc; then
    echo "CHRONY_TEST_CONDITION result=phc_readable action=run_existing_installer"
    echo "Existing installer definition:"
    type disableNtpAndTimesyncdInstallChrony 2>&1

    # The installer contains explicit exit calls. Run it in a subshell so a failure is
    # recorded without preventing the remaining diagnostics from reaching the build log.
    if (disableNtpAndTimesyncdInstallChrony); then
      echo "CHRONY_INSTALL_RESULT result=success"
    else
      chrony_install_exit_code=$?
      echo "CHRONY_INSTALL_RESULT result=failure exit_code=${chrony_install_exit_code}"
    fi

    echo "Chrony configuration produced by the existing AgentBaker installer:"
    sed -n '1,240p' /etc/chrony/chrony.conf 2>&1
    echo "Chrony service details after installation:"
    systemctl status chrony chronyd --no-pager 2>&1
    systemctl show chrony chronyd \
      -p Id -p LoadState -p ActiveState -p SubState -p Result -p ExecMainStatus 2>&1

    echo "Chrony source and tracking checks:"
    for attempt in 1 2 3 4 5; do
      echo "CHRONY_CHECK attempt=${attempt}"
      chronyc -n sources -v 2>&1
      chronyc -n sourcestats -v 2>&1
      chronyc tracking 2>&1
      sleep 1
    done

    echo "Chrony journal:"
    journalctl -b -u chrony -u chronyd --no-pager 2>&1
  else
    echo "CHRONY_TEST_CONDITION result=phc_unavailable action=skip_existing_installer"
    echo "CHRONY_INSTALL_RESULT result=skipped reason=hyperv_phc_unavailable"
  fi

  echo "Relevant kernel messages:"
  journalctl -k -b --no-pager | grep -Ei 'ptp|hyperv|hv_utils|vmbus|clocksource|tdx|confidential' || true
  echo "Relevant udev messages:"
  journalctl -b -u systemd-udevd --no-pager 2>&1 |
    grep -Ei 'ptp|hyperv|vmbus|clock' || true
  echo "Time synchronization service state:"
  timedatectl status 2>&1
  systemctl status chrony chronyd systemd-timesyncd --no-pager 2>&1
  echo "Existing Chrony configuration:"
  find /etc/chrony /etc/chrony.conf -maxdepth 2 -type f -print \
    -exec sed -n '1,240p' {} \; 2>&1
  echo "Final diagnostic result markers:"
  grep -E 'PHC_READ_RESULT|CHRONY_TEST_CONDITION|CHRONY_INSTALL_RESULT|CHRONY_CHECK' \
    "$TDX_CLOCK_DIAGNOSTICS_LOG" 2>&1
  echo "Diagnostic log statistics:"
  wc -l -c "$TDX_CLOCK_DIAGNOSTICS_LOG" 2>&1
  sha256sum "$TDX_CLOCK_DIAGNOSTICS_LOG" 2>&1
  echo "===== End Ubuntu 26.04 Intel TDX host-clock diagnostics ====="

  # This branch is diagnostic-only. Fail once after collecting every attempted configuration.
  exit 1
fi

# Update rsyslog configuration
RSYSLOG_CONFIG_FILEPATH="/etc/rsyslog.d/60-CIS.conf"
if isMarinerOrAzureLinux "$OS"; then
    echo -e "\nnews.none                          -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
else
    echo -e "\n*.*;mail.none;news.none            -/var/log/messages" >> ${RSYSLOG_CONFIG_FILEPATH}
fi
systemctl daemon-reload
systemctlEnableAndStart systemd-journald 30 || exit 1
if ! isFlatcar "$OS" && ! isACL "$OS" "$OS_VARIANT" ; then
    systemctlEnableAndStart rsyslog 30 || exit 1
fi

systemctlEnableAndStart disk_queue 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_copy_packer_files_and_enable_logging"

# This path is used by the Custom CA Trust feature only
mkdir -p /opt/certs
chmod 755 /opt/certs
systemctlEnableAndStart update_certs.path 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_make_certs_directory_and_update_certs"

systemctlEnableAndStart ci-syslog-watcher.path 30 || exit 1
systemctlEnableAndStart ci-syslog-watcher.service 30 || exit 1

if isFlatcar "$OS" || isACL "$OS" "$OS_VARIANT"; then
    # "copy-on-write"; this starts out as a symlink to a R/O location
    cp /etc/waagent.conf{,.new}
    mv /etc/waagent.conf{.new,}
fi
# disable AKS log collector and waagent collection
echo -e "\n# Disable WALA log collection because AKS Log Collector is installed.\nLogs.Collect=n" >> /etc/waagent.conf || exit 1
systemctl disable --now aks-log-collector.service || exit 1
systemctl disable --now aks-log-collector.timer || exit 1

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
systemctlEnableAndStart logrotate.timer 30 || exit 1
rm -f /etc/cron.daily/logrotate

systemctlEnableAndStart sync-container-logs.service 30 || exit 1
capture_benchmark "${SCRIPT_NAME}_enable_and_configure_logging_services"

# Keep aks-node-controller.service disabled in the VHD image. The unit now has
# DefaultDependencies=no (see aks-node-controller.service), so if it were enabled
# via WantedBy=basic.target it could be auto-started by systemd before the
# boothook has written the provision config/nbc-cmd files, causing the wrapper's
# graceful no-op exit to mark the oneshot unit "active (exited)" - after which
# the boothook's own explicit "systemctl start" would be a no-op and ANC would
# never actually run with the real config. The boothook's explicit
# "systemctl start --no-block aks-node-controller.service" call (issued only
# after those files exist) remains the sole trigger for this unit.
# Sometimes its also started diretly in boothook
systemctl disable aks-node-controller.service

# Pulled in by kubelet.service via WantedBy=kubelet.service, so CSE does not need to start it.
systemctl enable emit-kubelet-active-flags.service

# First handle Mariner + FIPS
if isMarinerOrAzureLinux "$OS"; then
  dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
  dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
  if [ "${ENABLE_FIPS,,}" = "true" ] && [ "${IMG_SKU,,}" != "azure-linux-3-arm64-gen2-fips" ]; then
    # This is FIPS install for Mariner and has nothing to do with Ubuntu Advantage
    echo "Install FIPS for Mariner SKU"
    installFIPS
  fi
elif isACL "$OS" "$OS_VARIANT"; then
  if [ "${ENABLE_FIPS,,}" = "true" ]; then
    echo "Install FIPS for AzureContainerLinux SKU"
    installFIPS
  fi
else
  # Enable ESM only for 20.04, and FIPS
  if [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${ENABLE_FIPS,,}" = "true" ]; then
    set +x
    attachUA
    set -x
  fi

  if [ -n "${VHD_BUILD_TIMESTAMP}" ] && [ "${OS_VERSION}" = "22.04" ]; then
    sed -i "s#http://azure.archive.ubuntu.com/ubuntu/#https://snapshot.ubuntu.com/ubuntu/${VHD_BUILD_TIMESTAMP}#g" /etc/apt/sources.list
  fi

  # Run apt get update to refresh repo list
  # Run apt dist get upgrade to install packages/kernels
  apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT

  # shellcheck disable=SC3010
  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS Install for Ubuntu, it purges non FIPS Kernel and attaches UA FIPS Updates
    echo "Install FIPS for Ubuntu SKU"
    installFIPS
  fi
fi
capture_benchmark "${SCRIPT_NAME}_upgrade_distro_and_resolve_fips_requirements"

# Handle Azure Linux + CgroupV2
# CgroupV2 is enabled by default in the AzureLinux 3.0 marketplace image
# shellcheck disable=SC3010
if [[ ${OS} == ${MARINER_OS_NAME} ]] && [[ "${ENABLE_CGROUPV2,,}" == "true" ]]; then
  enableCgroupV2forAzureLinux
fi
capture_benchmark "${SCRIPT_NAME}_enable_cgroupv2_for_azurelinux"

if { isUbuntu "$OS" || isAzureLinux "$OS"; }; then
  echo "nodelay" | tee -a /etc/dhcpcd.conf
  tee /etc/systemd/system/cache-warmup.service > /dev/null << 'EOF'
[Unit]
Description=Preload Critical Binaries into Page Cache
DefaultDependencies=no

[Service]
Type=simple
ExecStart=/bin/bash /opt/azure/containers/provision_preload.sh

[Install]
WantedBy=sysinit.target
EOF

  systemctl daemon-reload
  systemctl enable cache-warmup.service
fi

# Remove lockdown=integrity from kernel cmdline for Azure Linux 3.0
# The kernel has an OOT patch that auto-enables lockdown when secure boot is detected
if isMarinerOrAzureLinux "$OS" && [ "$OS_VERSION" = "3.0" ]; then
  disableKernelLockdownCmdline
fi
capture_benchmark "${SCRIPT_NAME}_disable_kernel_lockdown_cmdline"

# shellcheck disable=SC3010
if [[ ${UBUNTU_RELEASE//./} -ge 2204 && "${ENABLE_FIPS,,}" != "true" ]]; then

  # Choose kernel packages based on Ubuntu version and architecture
  if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    KERNEL_IMAGE="linux-image-azure-fde-lts-${UBUNTU_RELEASE}"
    KERNEL_PACKAGES=(
      "linux-image-azure-fde-lts-${UBUNTU_RELEASE}"
      "linux-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-cloud-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-headers-azure-lts-${UBUNTU_RELEASE}"
    )
    echo "Installing fde LTS kernel for CVM Ubuntu ${UBUNTU_RELEASE}"
  else
    # Use LTS kernel for other versions
    KERNEL_IMAGE="linux-image-azure-lts-${UBUNTU_RELEASE}"
    KERNEL_PACKAGES=(
      "linux-image-azure-lts-${UBUNTU_RELEASE}"
      "linux-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-cloud-tools-azure-lts-${UBUNTU_RELEASE}"
      "linux-headers-azure-lts-${UBUNTU_RELEASE}"
    )
    echo "Installing LTS kernel for Ubuntu ${UBUNTU_RELEASE}"
  fi

  # Add modules-extra only when the package exists in the current apt repo
  MODULES_EXTRA_PKG="linux-modules-extra-azure-lts-${UBUNTU_RELEASE}"
  if apt-cache show "${MODULES_EXTRA_PKG}" &>/dev/null; then
    KERNEL_PACKAGES+=("${MODULES_EXTRA_PKG}")
  else
    echo "Package ${MODULES_EXTRA_PKG} not available - skipping"
  fi

  echo "Logging the currently running kernel: $(uname -r)"
  echo "Before purging kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*' || true

  if apt-cache show "$KERNEL_IMAGE" &>/dev/null; then
    echo "Kernel packages are available, proceeding with purging current kernel and installing new kernel..."

    # Purge nullboot package only for cvm
    if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
      wait_for_apt_locks
      DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y --allow-remove-essential nullboot
    fi

    # Purge all current kernels and dependencies
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y $(dpkg-query -W 'linux-*azure*' | awk '$2 != "" { print $1 }' | paste -s)
    echo "After purging kernel, dpkg list should be empty"; dpkg -l 'linux-*azure*' || true

    # Install new kernel packages
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y "${KERNEL_PACKAGES[@]}"
    echo "After installing new kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*' || true

    # Reinstall nullboot package only for cvm
    if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
      wait_for_apt_locks
      DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y nullboot
    fi

    # Cleanup
    wait_for_apt_locks
    DEBIAN_FRONTEND=noninteractive apt-get autoremove -y && DEBIAN_FRONTEND=noninteractive apt-get clean
  else
    echo "Kernel packages for Ubuntu ${UBUNTU_RELEASE} are not available. Skipping purging and subsequent installation."
  fi
  NVIDIA_KERNEL_PACKAGE="linux-azure-nvidia"
  if [[ "${CPU_ARCH}" == "arm64" && "${UBUNTU_RELEASE}" = "24.04" ]]; then
    # This is the ubuntu 2404arm64gen2containerd image or the 2404arm64gb image
    # The Ubuntu PPA has early access to new kernels, such as the one in the GB300 CRD.
    # Uncomment if we have trouble finding the kernel package.
    # add-apt-repository ppa:canonical-kernel-team/ppa
    if grep -q "NVIDIA_GB" <<< "$FEATURE_FLAGS"; then
      add-apt-repository ppa:canonical-kernel-team/ppa
      apt-get update
      BOM_PATH="gb-mai-bom.json"
      if [ -n "$(jq -r '.["kernel-versions"] | keys[]' $BOM_PATH)" ]; then
        NVIDIA_KERNEL_PACKAGE=$(jq -r '.["kernel-versions"] | to_entries[] | "\(.key)=\(.value)"' $BOM_PATH)
      fi
      if apt-get install -s "${NVIDIA_KERNEL_PACKAGE}" &> /dev/null; then
      	echo "ARM64 image. Installing NVIDIA kernel and its packages alongside LTS kernel"
      	  wait_for_apt_locks
      	  apt install --no-install-recommends -y "${NVIDIA_KERNEL_PACKAGE}"
      	  echo "after installation:"
      	  dpkg -l | grep "linux-.*-azure-nvidia" || true
    	else
    	  echo "ARM64 image. NVIDIA kernel not available from repo, fetching and installing dpkgs by hand"
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-modules-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb > /tmp/linux-modules-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-azure-nvidia-6.14-cloud-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb > /tmp/linux-azure-nvidia-6.14-cloud-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-azure-nvidia-6.14-cloud-tools-common_6.14.0-1003.3_all.deb > /tmp/linux-azure-nvidia-6.14-cloud-tools-common_6.14.0-1003.3_all.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-azure-nvidia-6.14-headers-6.14.0-1003_6.14.0-1003.3_all.deb > /tmp/linux-azure-nvidia-6.14-headers-6.14.0-1003_6.14.0-1003.3_all.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-azure-nvidia-6.14-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb > /tmp/linux-azure-nvidia-6.14-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-cloud-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb > /tmp/linux-cloud-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-headers-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb > /tmp/linux-headers-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb > /tmp/linux-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb

    	  curl -fsSL https://ports.ubuntu.com/pool/main/l/linux-azure-nvidia-6.14/linux-image-unsigned-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb > /tmp/linux-image-unsigned-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb

    	  dpkg -i /tmp/linux-modules-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-azure-nvidia-6.14-cloud-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-azure-nvidia-6.14-cloud-tools-common_6.14.0-1003.3_all.deb
    	  dpkg -i /tmp/linux-azure-nvidia-6.14-headers-6.14.0-1003_6.14.0-1003.3_all.deb
    	  dpkg -i /tmp/linux-azure-nvidia-6.14-tools-6.14.0-1003_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-cloud-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-headers-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-tools-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb
    	  dpkg -i /tmp/linux-image-unsigned-6.14.0-1003-azure-nvidia_6.14.0-1003.3_arm64.deb

    	  rm /tmp/*.deb
      fi
      add-apt-repository --remove ppa:canonical-kernel-team/ppa
    else
      apt-get update
      if apt-cache show "${NVIDIA_KERNEL_PACKAGE}" &> /dev/null; then
        echo "ARM64 image. Installing NVIDIA kernel and its packages alongside LTS kernel"
        wait_for_apt_locks
        apt install --no-install-recommends -y "${NVIDIA_KERNEL_PACKAGE}"
        echo "after installation:"
        dpkg -l | grep "linux-.*-azure-nvidia" || true
      else
        echo "ARM64 image. NVIDIA kernel not available, skipping installation."
      fi
    fi
  fi
  wait_for_apt_locks
  if grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    echo "update-grub not found (expected for CVM images using nullboot), skipping"
  else
    update-grub
  fi
fi
capture_benchmark "${SCRIPT_NAME}_purge_ubuntu_kernel_if_2204"
echo "pre-install-dependencies step finished successfully"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
