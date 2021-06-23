[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU
After=kubelet.service

[Service]
Restart=on-failure
ExecStart=/usr/bin/nvidia-smi -mig 1
#/bin/bash /opt/azure/containers/mig-partition.sh

[Install]
WantedBy=multi-user.target
#EOF