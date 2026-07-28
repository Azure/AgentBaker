#!/bin/bash -eux

systemctl daemon-reload
systemctl disable --now containerd

VULNERABLE_KERNEL_MODULE_DENY_PATTERN='^(install[[:space:]]+(algif_aead|esp4|esp6|rxrpc)[[:space:]]+[/]bin[/]false|blacklist[[:space:]]+(algif_aead|esp4|esp6|rxrpc))([[:space:]]+.*)?$'

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
  local ubuntu_release
  local kernel_release
  local fixed_kernel

  ubuntu_release="$(awk -F= '$1 == "VERSION_ID" { gsub(/"/, "", $2); print $2; exit }' /etc/os-release 2>/dev/null || true)"
  kernel_release="$(uname -r 2>/dev/null || true)"

  if [ -z "$ubuntu_release" ] || [ -z "$kernel_release" ]; then
    return 1
  fi

  case "$ubuntu_release" in
    20.04) return 1 ;;
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
    *) return 0 ;;
  esac

  kernelVersionGe "$kernel_release" "$fixed_kernel"
}

removeVulnerableKernelModuleDenyRulesFromModprobeDirectory() {
  local modprobe_conf
  local tmp_modprobe_conf

  for modprobe_conf in /etc/modprobe.d/*.conf; do
    [ -f "$modprobe_conf" ] || continue
    tmp_modprobe_conf="${modprobe_conf}.tmp.$$"
    sed -E "/$VULNERABLE_KERNEL_MODULE_DENY_PATTERN/d" "$modprobe_conf" > "$tmp_modprobe_conf" || {
      echo "Failed to update vulnerable module deny rules in ${modprobe_conf}"
      rm -f "$tmp_modprobe_conf"
      return 1
    }
    cat "$tmp_modprobe_conf" > "$modprobe_conf" || {
      echo "Failed to write updated vulnerable module deny rules to ${modprobe_conf}"
      rm -f "$tmp_modprobe_conf"
      return 1
    }
    rm -f "$tmp_modprobe_conf" || {
      echo "Failed to remove temporary modprobe file ${tmp_modprobe_conf}"
      return 1
    }
  done

  if grep -qsE "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" /etc/modprobe.d/*.conf 2>/dev/null; then
    echo "Failed to remove vulnerable module deny rules from /etc/modprobe.d/*.conf"
    grep -nE "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" /etc/modprobe.d/*.conf || true
    return 1
  fi
}

os_id="$(awk -F= '$1 == "ID" { gsub(/"/, "", $2); print $2; exit }' /etc/os-release 2>/dev/null || true)"
if [ "$os_id" = "ubuntu" ] && ubuntuKernelIncludesVulnerableModuleFixes; then
  echo "cleanup-vhd: removing vulnerable kernel module deny rules on fixed or future Ubuntu kernel"
  removeVulnerableKernelModuleDenyRulesFromModprobeDirectory
fi

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
