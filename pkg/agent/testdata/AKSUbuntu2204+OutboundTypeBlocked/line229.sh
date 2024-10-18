[Unit]
Description=teleportd teleport runtime
After=network.target
[Service]
ExecStart=/usr/local/bin/teleportd --metrics --aksConfig /etc/kubernetes/azure.json
Delegate=yes
KillMode=process
Restart=always
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=1048576
TasksMax=infinity
[Install]
WantedBy=multi-user.target
