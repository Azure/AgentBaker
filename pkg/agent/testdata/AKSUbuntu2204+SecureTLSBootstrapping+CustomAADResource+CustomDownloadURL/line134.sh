[Unit]
Description=Snapshot Update Service

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/ubuntu-snapshot-update.sh