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
# Without this, VMs created from this VHD won't get flatcar.first_boot=detected on the kernel cmdline
if [ -f /boot/acl/uki-addons/firstboot.addon.efi ] && [ ! -f /boot/EFI/Linux/acl.efi.extra.d/firstboot.addon.efi ]; then
  install -D -m 0644 /boot/acl/uki-addons/firstboot.addon.efi /boot/EFI/Linux/acl.efi.extra.d/firstboot.addon.efi
fi
# Cleanup disk usage diagnostics file (created by generate-disk-usage.sh)
rm -f /opt/azure/disk-usage.txt
# remove image-fetcher binary from the image since it's only needed during build and is not expected to be present on the final image
rm -f /opt/azure/containers/image-fetcher
# Security: remove compiler toolchain from Ubuntu VHDs to prevent on-node exploit compilation.
# gcc/make are needed at build time (dkms, kernel module compilation) but should not ship.
# Azure Linux already does not include gcc. See AB#37878492.
if command -v apt-get &>/dev/null; then
  apt-get purge -y --auto-remove gcc gcc-[0-9]* cpp cpp-[0-9]* make 2>/dev/null || true
fi
# Cleanup IMDS instance metadata cache file
rm -f /opt/azure/containers/imds_instance_metadata_cache.json
