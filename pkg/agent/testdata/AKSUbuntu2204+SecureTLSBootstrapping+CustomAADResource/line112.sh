[Unit]
Description=Downloads the secure TLS bootstrapping client binary
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/azure/tlsbootstrap/download-secure-tls-bootstrap-client.sh

#EOF