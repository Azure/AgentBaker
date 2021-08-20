[Unit]
Description=Krustlet

[Service]
Restart=on-failure
RestartSec=5s
EnvironmentFile=/etc/default/kubelet
Environment=KUBECONFIG=/var/lib/kubelet/kubeconfig
Environment=KRUSTLET_CERT_FILE=/etc/kubernetes/certs/kubeletserver.crt
Environment=KRUSTLET_PRIVATE_KEY_FILE=/etc/kubernetes/certs/kubeletserver.key
Environment=KRUSTLET_DATA_DIR=/etc/krustlet
Environment=RUST_LOG=wasi_provider=info,main=info
Environment=KRUSTLET_BOOTSTRAP_FILE=/var/lib/kubelet/bootstrap-kubeconfig
ExecStart=/usr/local/bin/krustlet-wasi --node-labels="${KUBELET_NODE_LABELS}" --max-pods="110"

[Install]
WantedBy=multi-user.target
