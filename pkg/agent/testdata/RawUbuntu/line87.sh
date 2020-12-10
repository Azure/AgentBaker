[Unit]
Description=a timer that delays docker-monitor from starting too soon after boot
[Timer]
Unit=docker-monitor.service
OnBootSec=10min
[Install]
WantedBy=multi-user.target
#EOF
