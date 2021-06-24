[Unit]
Description=Enable MIG configuration on Nvidia A100 GPU

[Service]
Type=oneshot
RemainAfterExit=true
ExecStart=/usr/bin/nvidia-smi -mig 1

[Install]
WantedBy=multi-user.target
#EOF