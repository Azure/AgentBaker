#!/bin/bash

isMGLRUDefaultDisabled() {
  isUbuntu "$1" || isMarinerOrAzureLinux "$1" "$2" || isACL "$1" "$2"
}

configureImageFiles() {
  local overlays=()
  if [ ! -f /etc/systemd/system/logrotate.timer ] && [ ! -f /usr/lib/systemd/system/logrotate.timer ]; then
    overlays+=(--overlay logrotate)
  fi
  if [ "${UBUNTU_RELEASE:-}" = "22.04" ] && [ "${ENABLE_FIPS,,}" = "true" ]; then
    overlays+=(--overlay ubuntu2204-fips)
  fi
  if { [ "${UBUNTU_RELEASE:-}" = "24.04" ] && [ "$CPU_ARCH" = "arm64" ]; } ||
    { isAzureLinuxArm64BaseImage "$OS" "$CPU_ARCH" "$OS_VARIANT" && [ "$OS_VERSION" = "3.0" ] && [ "${ENABLE_FIPS,,}" != "true" ]; }; then
    overlays+=(--overlay nvidia-grub)
  fi
  if grep -q "NVIDIA_GB" <<< "$FEATURE_FLAGS"; then
    overlays+=(--overlay gb)
  fi
  /home/packer/aks-node-controller unpack-scripts "${overlays[@]}" || exit 1

  chmod 0600 /etc/ssh/sshd_config /etc/security/pwquality.conf || exit 1
  chmod 0444 /NOTICE.txt || exit 1
  # Public driver repositories are build inputs, not part of the node's hotfix tree.
  if grep -q "NVIDIA_GB" <<< "$FEATURE_FLAGS"; then
    install -D -m 0644 /opt/azure/image-build/gb/nvidia.list /etc/apt/sources.list.d/nvidia.list || exit 1
    install -D -m 0644 /opt/azure/image-build/gb/nvidia.pub /etc/apt/keyrings/nvidia.pub || exit 1
    install -D -m 0644 /opt/azure/image-build/gb/doca-net.list /etc/apt/sources.list.d/doca-net.list || exit 1
    install -D -m 0644 /opt/azure/image-build/gb/doca-net.pub /etc/apt/keyrings/doca-net.pub || exit 1
  fi

  # These payloads are produced outside ANC, not embedded static files.
  install -m 0755 /home/packer/image-fetcher /opt/azure/containers/image-fetcher || exit 1
  install -m 0755 /home/packer/aks-node-controller /opt/azure/containers/aks-node-controller || exit 1
  install -m 0644 /home/packer/prefetch.sh /opt/azure/containers/prefetch.sh || exit 1

  installKataPayload
}

installKataPayload() {
  if grep -q "kata" <<< "$FEATURE_FLAGS"; then
    local kata_config_dir katacc_config_dir=/opt/confidential-containers/share/kata-containers
    case "$OS_VERSION" in
      2.0) kata_config_dir=/var/cache/kata-containers/osbuilder-images/kernel-uvm ;;
      3.0) kata_config_dir=/usr/share/kata-containers ;;
      *) echo "Unexpected OS version '$OS_VERSION' for Kata" >&2; exit 1 ;;
    esac
    mkdir -p "$kata_config_dir" || exit 1
    if [ "$OS_VERSION" = "2.0" ]; then
      mkdir -p "$katacc_config_dir" || exit 1
      install -m 0755 /home/packer/kata-containers-initrd-base.img "$kata_config_dir/kata-containers-initrd.img" || exit 1
      install -m 0755 /home/packer/kata-containers.img "$katacc_config_dir/kata-containers.img" || exit 1
    else
      install -m 0755 /home/packer/kata-containers.img "$kata_config_dir/kata-containers.img" || exit 1
      if [[ ",${FEATURE_FLAGS}," == *",kata-cc,"* ]]; then
        mkdir -p "$katacc_config_dir" || exit 1
        local file
        for file in kata-containers.img kata-containers-cc.img kata-containers-igvm.img kata-containers-igvm-debug.img igvm-measurement.cose reference-info-base64; do
          install -m 0644 "/home/packer/kata-cc/$file" "$katacc_config_dir/$file" || exit 1
        done
      fi
    fi
  fi
}

# Package upgrades can replace the login banner with the distro default.
reapplyBanners() {
  printf '\nAuthorized uses only. All activity may be monitored and reported.\n' > /etc/issue
  cp /etc/issue /etc/issue.net
  chmod 0644 /etc/issue /etc/issue.net
}
