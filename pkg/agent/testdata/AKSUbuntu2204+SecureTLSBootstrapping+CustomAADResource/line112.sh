[Unit]
Description=Runs the secure TLS bootstrapping client binary to generate a kubelet client credential
Wants=network-online.target
After=network-online.target
Before=kubelet.service

[Service]
Type=oneshot
ExecStartPre=/opt/azure/tlsbootstrap/secure-tls-bootstrap.sh download
ExecStart=/opt/azure/tlsbootstrap/secure-tls-bootstrap.sh bootstrap

#EOF