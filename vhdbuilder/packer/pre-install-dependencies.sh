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

# Azure Linux currently marks kernel and kernel-hwe as conflicting even though
# their boot files do not overlap. Use dnf5 to co-install them until that package
# conflict is removed upstream.
if [ "$OS" = "$AZURELINUX_OS_NAME" ] && [ "$OS_VERSION" = "3.0" ] && [ "$CPU_ARCH" = "arm64" ] && [ -z "$OS_VARIANT" ] && [ "${ENABLE_FIPS,,}" != "true" ]; then
  if ! rpm -q kernel-hwe &>/dev/null; then
    dnf5_was_installed=false
    if rpm -q dnf5 &>/dev/null; then
      dnf5_was_installed=true
    else
      dnf_install 30 1 600 dnf5 || exit "$ERR_APT_INSTALL_TIMEOUT"
    fi

    dnf5 install -y kernel-hwe || exit "$ERR_APT_INSTALL_TIMEOUT"

    if ! $dnf5_was_installed; then
      tdnf remove -y dnf5 || true
    fi
  fi

  for kernel_package in kernel kernel-hwe; do
    if ! rpm -q "$kernel_package" &>/dev/null || ! rpm -ql "$kernel_package" | grep -q '^/boot/vmlinuz-'; then
      echo "ARM64 Azure Linux: $kernel_package does not provide a bootable kernel" >&2
      exit "$ERR_APT_INSTALL_TIMEOUT"
    fi
  done

  # The signed ARM64 EFI binary does not embed smbios, and grub2-efi installs
  # its dynamic modules outside the boot prefix. Stage the required closure
  # until Azure Linux provides smbios at boot directly.
  dnf_install 30 1 600 grub2-efi || exit "$ERR_APT_INSTALL_TIMEOUT"
  grub_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}' grub2)
  grub_efi_binary_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}' grub2-efi-binary)
  grub_efi_modules_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}' grub2-efi)
  if [ "$grub_version" != "$grub_efi_binary_version" ] || [ "$grub_version" != "$grub_efi_modules_version" ]; then
    echo "ARM64 Azure Linux: GRUB package versions do not match: grub2=$grub_version, binary=$grub_efi_binary_version, modules=$grub_efi_modules_version" >&2
    exit "$ERR_APT_INSTALL_TIMEOUT"
  fi

  grub_module_source=/usr/lib/grub/arm64-efi
  grub_module_destination=/boot/grub2/arm64-efi
  for grub_module_file in extcmd.mod smbios.mod moddep.lst; do
    if [ ! -s "$grub_module_source/$grub_module_file" ]; then
      echo "ARM64 Azure Linux: required GRUB file $grub_module_source/$grub_module_file is missing" >&2
      exit "$ERR_APT_INSTALL_TIMEOUT"
    fi
  done
  if ! grep -q '^smbios: extcmd$' "$grub_module_source/moddep.lst"; then
    echo "ARM64 Azure Linux: unexpected smbios module dependencies" >&2
    exit "$ERR_APT_INSTALL_TIMEOUT"
  fi
  install -d -m 0755 "$grub_module_destination"
  install -m 0644 \
    "$grub_module_source/extcmd.mod" \
    "$grub_module_source/smbios.mod" \
    "$grub_module_source/moddep.lst" \
    "$grub_module_destination/"

  grub2-mkconfig -o /boot/grub2/grub.cfg
fi
capture_benchmark "${SCRIPT_NAME}_install_kernel_hwe_arm64"

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
