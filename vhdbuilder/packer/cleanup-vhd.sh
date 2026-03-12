#!/bin/bash -eux

systemctl daemon-reload
systemctl disable --now containerd || exit 1

# Cleanup packer SSH key and machine ID generated for this boot
rm -f /root/.ssh/authorized_keys
rm -f /home/packer/.ssh/authorized_keys
rm -f /var/log/cloud-init.log /var/log/cloud-init-output.log
# aznfs pulls in stunnel4 which pollutes the log dir but aznfs configures stunnel to log to a private location
rm -rf /var/log/stunnel4/ /etc/logrotate.d/stunnel4
rm -f /etc/machine-id
touch /etc/machine-id
chmod 644 /etc/machine-id
# Cleanup disk usage diagnostics file (created by generate-disk-usage.sh)
rm -f /opt/azure/disk-usage.txt
# remove image-fetcher binary from the image since it's only needed during build and is not expected to be present on the final image
rm -f /opt/azure/containers/image-fetcher
# Cleanup IMDS instance metadata cache file
rm -f /opt/azure/containers/imds_instance_metadata_cache.json
