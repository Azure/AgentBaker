[Unit]
Description=Add dudup ebtable rules for promisc mode
After=kubelet.service
[Service]
Restart=on-failure
RestartSec=30
ExecStart=/bin/bash /opt/azure/containers/ensure_no_dup.sh
#EOF
