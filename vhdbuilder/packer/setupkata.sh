while [ ! -f /opt/azure/containers/provision.complete ]
do
  echo "Waiting for provisioning to complete"
  sleep 5
done

sleep 10

cat /etc/containerd/config.toml | grep kata > /dev/null
if [[ $? != 0 ]]; then
  echo "kata config needs to be applied to containerd"
  cat << EOF >> /etc/containerd/config.toml
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli]
  runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli.options]
  NoPivotRoot = false
  NoNewKeyring = false
  ShimCgroup = ""
  IoUid = 0
  IoGid = 0
  BinaryName = "/usr/bin/kata-runtime"
  Root = ""
  CriuPath = ""
  SystemdCgroup = false
EOF

  echo "Config change applied, restarting containerd"
  systemctl restart containerd
fi

  echo "Containerd has kata config enabled"
