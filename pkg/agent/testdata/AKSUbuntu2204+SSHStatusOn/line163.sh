[Unit]
Description=Bind mount kubelet data
Requires=mnt.mount
After=mnt.mount

[Service]
Restart=on-failure
RemainAfterExit=yes
Type=oneshot
ExecStart=/bin/bash /opt/azure/containers/bind-mount.sh

[Install]
WantedBy=multi-user.target
