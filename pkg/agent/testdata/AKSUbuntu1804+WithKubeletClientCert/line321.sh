[Unit]
Description=AKS Local DNS
Wants=network.target
After=network.target
After=cloud-config.service
Before=kubelet.service
Before=containerd.service
ConditionKernelVersion=>=5.15 

[Service]
Type=notify
NotifyAccess=all
WatchdogSec=60
Restart=on-failure
KillMode=mixed
TimeoutStopSec=30
Slice=akslocaldns.slice
EnvironmentFile=-/etc/default/akslocaldns/akslocaldns.envfile
ExecStart=/opt/azure/akslocaldns/akslocaldns.sh

[Install]
WantedBy=multi-user.target