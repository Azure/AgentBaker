KUBELET_FLAGS=--address=0.0.0.0 --anonymous-auth=false --authentication-token-webhook=true --authorization-mode=Webhook --azure-container-registry-config=/etc/kubernetes/azure.json --cgroups-per-qos=true --client-ca-file=/etc/kubernetes/certs/ca.crt --cloud-config=/etc/kubernetes/azure.json --cloud-provider=azure --cluster-dns=10.0.0.10 --cluster-domain=cluster.local --container-log-max-size=50M --enforce-node-allocatable=pods --event-qps=0 --eviction-hard=memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5% --feature-gates=PodPriority=true,RotateKubeletServerCertificate=true,a=false,x=false --image-gc-high-threshold=85 --image-gc-low-threshold=80 --kube-reserved=cpu=100m,memory=1638Mi --max-pods=110 --node-status-update-frequency=10s --pod-manifest-path=/etc/kubernetes/manifests --pod-max-pids=-1 --protect-kernel-defaults=true --read-only-port=10255 --resolv-conf=/etc/resolv.conf --rotate-certificates=true --streaming-connection-idle-timeout=4h0m0s --system-reserved=cpu=2,memory=1Gi --tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256 --tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key 
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_NODE_LABELS=agentpool=agent2,kubernetes.azure.com/agentpool=agent2