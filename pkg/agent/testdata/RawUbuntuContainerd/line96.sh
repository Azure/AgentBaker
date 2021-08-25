[Unit]
Description=a timer that delays containerd-monitor from starting too soon after boot
[Timer]
Unit=containerd-monitor.service
OnBootSec=10min
[Install]
WantedBy=multi-user.target
#EOF
