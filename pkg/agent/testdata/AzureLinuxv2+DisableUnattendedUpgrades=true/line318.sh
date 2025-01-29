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
Slice=aks-local-dns.slice
EnvironmentFile=-/etc/default/aks-local-dns
ExecStart=/opt/azure/aks-local-dns/aks-local-dns.sh "${COREDNS_IMAGE_URL}" "${NODE_LISTENER_IP}" "${CLUSTER_LISTENER_IP}"

[Install]
WantedBy=multi-user.target