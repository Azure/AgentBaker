[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU
#After=mig-enable.service

[Service]
Restart=on-failure

ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh 

[Install]
WantedBy=multi-user.target
#EOF