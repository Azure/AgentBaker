#!/bin/bash -eux

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
