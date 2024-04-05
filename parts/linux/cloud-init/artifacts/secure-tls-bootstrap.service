[Unit]
Description=Runs the secure TLS bootstrapping client binary to generate a kubelet client credential
ConditionPathExists=/opt/azure/tlsbootstrap/tls-bootstrap-client
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/azure/tlsbootstrap/secure-tls-bootstrap.sh

#EOF