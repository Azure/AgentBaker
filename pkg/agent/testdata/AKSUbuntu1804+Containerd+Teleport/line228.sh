[Unit]
Description=Add dedup ebtable rules for kubenet bridge in promiscuous mode
After=containerd.service
After=kubelet.service
[Service]
Restart=on-failure
RestartSec=1
ExecStart=/bin/bash /opt/azure/containers/ensure_no_dup.sh
#EOF
