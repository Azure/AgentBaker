#!/bin/bash -eux

[ -z "${RUN_VHD_CLEANUP}" ] && echo "RUN_VHD_CLEANUP must be set for cleanup-vhd.sh to run" && exit 1

if [ "${RUN_VHD_CLEANUP,,}" == "true" ]; then
    # Cleanup packer SSH key and machine ID generated for this boot
    rm -f /root/.ssh/authorized_keys
    rm -f /home/packer/.ssh/authorized_keys
    rm -f /var/log/cloud-init.log /var/log/cloud-init-output.log 
    rm -f /etc/machine-id
    touch /etc/machine-id
    chmod 644 /etc/machine-id
fi