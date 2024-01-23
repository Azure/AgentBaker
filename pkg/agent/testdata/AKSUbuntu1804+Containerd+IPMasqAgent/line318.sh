[Unit]
Description=AKS Log Collector

[Service]
Type=oneshot
Slice=aks-log-collector.slice
ExecStart=/opt/azure/containers/aks-log-collector.sh
RemainAfterExit=no
