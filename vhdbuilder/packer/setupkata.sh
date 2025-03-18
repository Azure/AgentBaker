# Wait for the commented 'root' config to be removed from containerd config through clount-init overwrite
while grep -q '#root = "/var/lib/containerd"' /etc/containerd/config.toml; do
  echo "Waiting for containerd config update..."
  sleep 5
done

sleep 3

CONFIG_PATH="/etc/containerd/config.toml"

# Check if kata-blk runtime already exists
if grep -q '^\[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-blk\]' "$CONFIG_PATH"; then
  echo "kata-blk runtime already exists in config. No changes needed."
else
  echo "Adding kata-blk runtime configuration..."
  
  # Append the new runtime configuration at the end of the file
  cat >> "$CONFIG_PATH" << EOF
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-blk]
  runtime_type = "io.containerd.kata.v2"
  snapshotter = "tardev"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-blk.options]
    ConfigPath = "/usr/share/defaults/kata-containers/configuration-blk.toml"
EOF

  echo "kata-blk runtime added to containerd config"
  
  # Restart containerd to apply changes
  systemctl restart containerd
fi

  echo "Containerd has kata config enabled"