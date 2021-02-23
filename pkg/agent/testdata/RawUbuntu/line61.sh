[Unit]
Description=Updates Labels for Kubernetes node
After=kubelet.service
[Service]
Restart=on-failure
RestartSec=30
EnvironmentFile=/etc/default/kubelet
ExecStart=/bin/bash /opt/azure/containers/update-node-labels.sh
#EOF
