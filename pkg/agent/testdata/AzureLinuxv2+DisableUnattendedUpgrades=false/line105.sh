[Unit]
Description=Kubelet
ConditionPathExists=/usr/local/bin/kubelet
ConditionPathExists=/opt/azure/containers/start-kubelet.sh
Wants=network-online.target containerd.service
After=network-online.target containerd.service

[Service]
Restart=always
RestartSec=2
TimeoutStartSec=270 
EnvironmentFile=/etc/default/kubelet
SuccessExitStatus=143

ExecStart=/opt/azure/containers/start-kubelet.sh

[Install]
WantedBy=multi-user.target
