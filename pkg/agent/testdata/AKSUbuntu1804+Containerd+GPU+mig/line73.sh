[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU

[Service]
Restart=on-failure
ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh mig-3g

[Install]
WantedBy=multi-user.target
#EOF