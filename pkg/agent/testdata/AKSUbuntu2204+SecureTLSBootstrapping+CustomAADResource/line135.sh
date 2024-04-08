[Unit]
Description=Runs the secure TLS bootstrapping client binary to generate a kubelet client credential
ConditionPathExists=/opt/azure/tlsbootstrap/tls-bootstrap-client
Wants=network-online.target download-secure-tls-bootstrap-client.service
After=network-online.target download-secure-tls-bootstrap-client.service

[Service]
Type=oneshot
ExecStart=/opt/azure/tlsbootstrap/secure-tls-bootstrap.sh

#EOF