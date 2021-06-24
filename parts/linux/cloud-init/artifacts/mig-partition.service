[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU
#After=kubelet.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh 
#$MIG_PARTITION
#TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
#EOF