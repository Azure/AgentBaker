[Unit]
Description=AKS Log Collector Timer

[Timer]
OnBootSec=5min
OnUnitActiveSec=1m

[Install]
WantedBy=timers.target
