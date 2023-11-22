[Unit]
Description=a script that checks containerd health and restarts if needed
After=containerd.service
[Service]
Restart=always
RestartSec=10
RemainAfterExit=yes
ExecStart=/usr/local/bin/health-monitor.sh container-runtime containerd
#EOF
