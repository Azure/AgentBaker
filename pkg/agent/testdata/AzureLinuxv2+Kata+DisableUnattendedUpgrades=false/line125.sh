[Unit]
Description=Package Update Service

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/mariner-package-update.sh