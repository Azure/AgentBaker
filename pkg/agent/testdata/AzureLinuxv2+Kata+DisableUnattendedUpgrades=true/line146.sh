[Unit]
Description=Runs package update script periodically

[Timer]
OnBootSec=10min
OnUnitActiveSec=10min

[Install]
WantedBy=multi-user.target