[Unit]
Description=Runs the secure TLS bootstrapping client binary to generate a kubelet client credential
Wants=network-online.target
After=network-online.target
Before=kubelet.service

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/secure-tls-bootstrap.sh
RemainAfterExit=No
