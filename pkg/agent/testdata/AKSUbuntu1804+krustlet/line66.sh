[Unit]
Description=Krustlet

[Service]
Restart=on-failure
RestartSec=5s
Environment=KUBECONFIG=/var/lib/kubelet/kubeconfig
Environment=KRUSTLET_CERT_FILE=/etc/kubernetes/certs/kubeletserver.crt
Environment=KRUSTLET_PRIVATE_KEY_FILE=/etc/kubernetes/certs/kubeletserver.key
Environment=KRUSTLET_DATA_DIR=/etc/krustlet
Environment=RUST_LOG=wasi_provider=info,main=info
Environment=KRUSTLET_BOOTSTRAP_FILE=/etc/krustlet/config/bootstrap.conf
ExecStart=/usr/local/bin/krustlet-wasi

[Install]
WantedBy=multi-user.target