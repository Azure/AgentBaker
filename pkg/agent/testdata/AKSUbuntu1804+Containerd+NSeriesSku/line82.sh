[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU
After=mig-enable.service

[Service]
# Type=oneshot
# RemainAfterExit=yes
Restart=on-failure

#ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh 
#TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
#EOF