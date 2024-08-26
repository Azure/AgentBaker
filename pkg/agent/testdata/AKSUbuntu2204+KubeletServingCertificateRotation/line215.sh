[Unit]
Description=Add dedup ebtable rules for kubenet bridge in promiscuous mode
After=containerd.service
After=kubelet.service
[Service]
Restart=on-failure
RestartSec=2
ExecStart=/bin/bash /opt/azure/containers/ensure-no-dup.sh
#EOF
