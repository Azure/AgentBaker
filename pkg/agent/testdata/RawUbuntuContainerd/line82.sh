[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU

[Service]
Restart=on-failure

ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh

[Install]
WantedBy=multi-user.target
#EOF