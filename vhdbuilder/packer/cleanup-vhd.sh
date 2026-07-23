#!/bin/bash -eux

VULNERABLE_KERNEL_MODULE_DENY_PATTERN='^(install|blacklist)[[:space:]]+(algif_aead|esp4|esp6|rxrpc)([[:space:]]|$)'

get_os_release_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key { gsub(/"/, "", $2); print $2; exit }' /etc/os-release 2>/dev/null || true
}

kernelVersionGe() {
  local version_a="$1"
  local version_b="$2"
  local sorted
  local highest_version

  sorted=$(printf "%s\n%s\n" "$version_a" "$version_b" | sort -V)
  highest_version=$(printf "%s\n" "$sorted" | tail -n 1)
  [ "$version_a" = "$highest_version" ]
}

ubuntuKernelIncludesVulnerableModuleFixes() {
  local os_id
  local os_version
  local kernel_release
  local fixed_kernel

  os_id="$(get_os_release_value ID)"
  os_version="$(get_os_release_value VERSION_ID)"
  kernel_release="$(uname -r 2>/dev/null || true)"

  if [ "$os_id" != "ubuntu" ] || [ -z "$kernel_release" ]; then
    return 1
  fi

  case "$os_version" in
    22.04)
      case "$kernel_release" in
        *-azure) fixed_kernel="5.15.0-1116-azure" ;;
        *-generic) fixed_kernel="5.15.0-181-generic" ;;
        *) return 1 ;;
      esac
      ;;
    24.04)
      case "$kernel_release" in
        *-azure) fixed_kernel="6.8.0-1058-azure" ;;
        *-generic) fixed_kernel="6.8.0-124-generic" ;;
        *) return 1 ;;
      esac
      ;;
    *) return 1 ;;
  esac

  kernelVersionGe "$kernel_release" "$fixed_kernel"
}

vulnerableKernelModuleDenyRulesRemain() {
  local modprobe_conf

  for modprobe_conf in /etc/modprobe.d/*.conf; do
    [ -f "$modprobe_conf" ] || continue
    if grep -Eq "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" "$modprobe_conf"; then
      return 0
    fi
  done

  return 1
}

removeVulnerableKernelModuleDenyRulesFromModprobeDirectory() {
  local modprobe_conf
  local tmp_modprobe_conf

  for modprobe_conf in /etc/modprobe.d/*.conf; do
    [ -f "$modprobe_conf" ] || continue
    tmp_modprobe_conf="${modprobe_conf}.tmp"
    sed -E "/$VULNERABLE_KERNEL_MODULE_DENY_PATTERN/d" "$modprobe_conf" > "$tmp_modprobe_conf"
    cat "$tmp_modprobe_conf" > "$modprobe_conf"
    rm -f "$tmp_modprobe_conf"
  done
}

cleanupFixedUbuntuVulnerableKernelModuleDenyRules() {
  if ! ubuntuKernelIncludesVulnerableModuleFixes; then
    return 0
  fi

  echo "cleanup-vhd: removing Copy Fail / DirtyFrag / Fragnesia module deny rules from fixed Ubuntu kernel image"
  removeVulnerableKernelModuleDenyRulesFromModprobeDirectory
  if vulnerableKernelModuleDenyRulesRemain; then
    echo "cleanup-vhd: vulnerable kernel module deny rules remain after cleanup" >&2
    grep -RE "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" /etc/modprobe.d/*.conf >&2 || true
    exit 1
  fi
}

cleanupFixedUbuntuVulnerableKernelModuleDenyRules

systemctl daemon-reload
systemctl disable --now containerd

# Cleanup packer SSH key and machine ID generated for this boot
rm -f /root/.ssh/authorized_keys
rm -f /home/packer/.ssh/authorized_keys
rm -f /var/log/cloud-init.log /var/log/cloud-init-output.log
# aznfs pulls in stunnel4 which pollutes the log dir but aznfs configures stunnel to log to a private location
rm -rf /var/log/stunnel4/ /etc/logrotate.d/stunnel4
rm -f /etc/machine-id
touch /etc/machine-id
chmod 644 /etc/machine-id
# Restore the UKI firstboot addon consumed by ignition-quench during this build
# Without this, VMs created from this VHD won't get flatcar.first_boot=detected on the kernel cmdline.
# The active UKI follows UAPI naming (vmlinuz-<version>.efi) on newer ACL images and was
# previously named acl.efi -- discover it dynamically rather than hardcoding either name.
if [ -f /boot/acl/uki-addons/firstboot.addon.efi ]; then
  uki_path="$(find /boot/EFI/Linux -maxdepth 1 -type f \
        \( -name 'vmlinuz-*.efi' -o -name 'acl.efi' \) 2>/dev/null \
        | sort | head -n1)"
  if [ -z "${uki_path}" ]; then
    echo "cleanup-vhd: No UKI found under /boot/EFI/Linux (expected acl.efi or vmlinuz-*.efi); firstboot addon not restored" >&2
    exit 1
  fi
  uki_name="$(basename "${uki_path}")"
  addon_dir="/boot/EFI/Linux/${uki_name}.extra.d"
  if [ ! -f "${addon_dir}/firstboot.addon.efi" ]; then
    install -D -m 0644 /boot/acl/uki-addons/firstboot.addon.efi "${addon_dir}/firstboot.addon.efi"
  fi
fi
# Cleanup disk usage diagnostics file (created by generate-disk-usage.sh)
rm -f /opt/azure/disk-usage.txt
# remove image-fetcher binary from the image since it's only needed during build and is not expected to be present on the final image
rm -f /opt/azure/containers/image-fetcher
# Cleanup IMDS instance metadata cache file
rm -f /opt/azure/containers/imds_instance_metadata_cache.json
