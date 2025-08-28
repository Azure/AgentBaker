#!/bin/bash

echo "Waiting for cloud-init to finish..."
cloud-init status --wait

echo "Confirmed cloud-init finished, writing nvidia-specific containerd configuration..."
cp /opt/azure/containerd-nvidia.toml /etc/containerd/config.toml

if [ $? -ne 0 ]; then
    echo "Failed to write /etc/containerd/config.toml"
    exit 1
else
    echo "Wrote /etc/containerd/config.toml"
fi

echo "Restarting containerd to apply new configuration"
systemctl restart containerd
systemctl is-active --quiet containerd
echo "Restarted containerd"