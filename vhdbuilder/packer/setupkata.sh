# Wait for the commented 'root' config to be removed from containerd config through clount-init overwrite
while grep -q '#root = "/var/lib/containerd"' /etc/containerd/config.toml; do
  echo "Waiting for containerd config update..."
  sleep 5
done

sleep 3

CONFIG_PATH="/etc/containerd/config.toml"

# Check if snapshotter = "tardev" exists under [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
if grep -A 1 '^\[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata\]' "$CONFIG_PATH" | grep -q 'snapshotter *= *"tardev"'; then
  echo "snapshotter = \"tardev\" is already set under [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata]. No changes needed."
else
  echo "Adding missing snapshotter = \"tardev\" under [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata]..."
  
  # Use sed to insert snapshotter = "tardev" right after the [kata] runtime section
  sed -i '/\[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata\]/a \  snapshotter = "tardev"' "$CONFIG_PATH"

  echo "Config change applied, restarting containerd..."
  systemctl restart containerd
fi

  echo "Containerd has kata config enabled"