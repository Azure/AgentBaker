[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU
#After=kubelet.service

[Service]
Restart=on-failure
ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/usr/bin/nvidia-smi mig -cgi 9,9 && /usr/bin/nvidia-smi nvidia-smi mig -cci
#/bin/bash /opt/azure/containers/mig-partition.sh

[Install]
WantedBy=multi-user.target
#EOF