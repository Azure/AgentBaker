#!/usr/bin/env bash
set -x
KUBE_VERSION="v1.24.9"
CNI_PLUGINS_VERSION="v1.2.0"
AZURE_CNI_VERSION="v1.4.45"
RUNC_VERSION="v1.1.5"
CONTAINERD_VERSION="1.6.20"

NODE_NAME=$(hostname)
ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf

mkdir -p /var/lib/cni
mkdir -p /opt/cni/bin
mkdir -p /etc/cni/net.d
mkdir -p /etc/kubernetes/volumeplugins
mkdir -p /etc/kubernetes/certs
mkdir -p /etc/containerd
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /var/lib/kubelet

curl -LO https://dl.k8s.io/${KUBE_VERSION}/kubernetes-node-linux-amd64.tar.gz
tar -xvzf kubernetes-node-linux-amd64.tar.gz kubernetes/node/bin/{kubelet,kubectl,kubeadm}
mv kubernetes/node/bin/{kubelet,kubectl,kubeadm} /usr/local/bin
rm kubernetes-node-linux-amd64.tar.gz

# azure cni (only)
curl -LO https://github.com/Azure/azure-container-networking/releases/download/${AZURE_CNI_VERSION}/azure-vnet-cni-linux-amd64-${AZURE_CNI_VERSION}.tgz
tar -xvzf azure-vnet-cni-linux-amd64-${AZURE_CNI_VERSION}.tgz -C /opt/cni/bin
sed -i 's#"mode":"bridge"#"mode":"transparent"#g' /opt/cni/bin/10-azure.conflist
# only when actually using azure cni
# mv /opt/cni/bin/10-azure.conflist /etc/cni/net.d/10-azure.conflist

# cni plugins
curl -LO https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-amd64-${CNI_PLUGINS_VERSION}.tgz
tar -xvzf cni-plugins-linux-amd64-${CNI_PLUGINS_VERSION}.tgz -C /opt/cni/bin/
rm cni-plugins-linux-amd64-${CNI_PLUGINS_VERSION}.tgz

curl -o runc -L https://github.com/opencontainers/runc/releases/download/${RUNC_VERSION}/runc.amd64
install -m 0555 runc /usr/bin/runc
rm runc

# containerd
curl -LO https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz
tar -xvzf containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz -C /usr
rm containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz

tee /etc/systemd/system/containerd.service > /dev/null <<EOF
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target
[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=infinity
# Comment TasksMax if your systemd version does not supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
OOMScoreAdjust=-999
[Install]
WantedBy=multi-user.target
EOF

tee /etc/containerd/config.toml > /dev/null <<EOF
version = 2
oom_score = 0
[plugins."io.containerd.grpc.v1.cri"]
	sandbox_image = "mcr.microsoft.com/oss/kubernetes/pause:3.6"
	[plugins."io.containerd.grpc.v1.cri".containerd]
		default_runtime_name = "runc"
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
			runtime_type = "io.containerd.runc.v2"
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
			BinaryName = "/usr/bin/runc"
			SystemdCgroup = true
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
			runtime_type = "io.containerd.runc.v2"
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
			BinaryName = "/usr/bin/runc"
	[plugins."io.containerd.grpc.v1.cri".cni]
		bin_dir = "/opt/cni/bin"
		conf_dir = "/etc/cni/net.d"
		conf_template = "/etc/containerd/kubenet_template.conf"
	[plugins."io.containerd.grpc.v1.cri".registry]
		config_path = "/etc/containerd/certs.d"
	[plugins."io.containerd.grpc.v1.cri".registry.headers]
		X-Meta-Source-Client = ["azure/aks"]
[metrics]
	address = "0.0.0.0:10257"
EOF

# for kubenet
tee /etc/containerd/kubenet_template.conf > /dev/null <<'EOF'
{
    "cniVersion": "0.3.1",
    "name": "kubenet",
    "plugins": [{
    "type": "bridge",
    "bridge": "cbr0",
    "mtu": 1500,
    "addIf": "eth0",
    "isGateway": true,
    "ipMasq": false,
    "promiscMode": true,
    "hairpinMode": false,
    "ipam": {
        "type": "host-local",
        "ranges": [{{range $i, $range := .PodCIDRRanges}}{{if $i}}, {{end}}[{"subnet": "{{$range}}"}]{{end}}],
        "routes": [{{range $i, $route := .Routes}}{{if $i}}, {{end}}{"dst": "{{$route}}"}{{end}}]
    }
    },
    {
    "type": "portmap",
    "capabilities": {"portMappings": true},
    "externalSetMarkChain": "KUBE-MARK-MASQ"
    }]
}
EOF

tee /etc/sysctl.d/999-sysctl-aks.conf > /dev/null <<EOF
# container networking
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1

# refer to https://github.com/kubernetes/kubernetes/blob/75d45bdfc9eeda15fb550e00da662c12d7d37985/pkg/kubelet/cm/container_manager_linux.go#L359-L397
vm.overcommit_memory = 1
kernel.panic = 10
kernel.panic_on_oops = 1
# to ensure node stability, we set this to the PID_MAX_LIMIT on 64-bit systems: refer to https://kubernetes.io/docs/concepts/policy/pid-limiting/
kernel.pid_max = 4194304
# https://github.com/Azure/AKS/issues/772
fs.inotify.max_user_watches = 1048576
# Ubuntu 22.04 has inotify_max_user_instances set to 128, where as Ubuntu 18.04 had 1024. 
fs.inotify.max_user_instances = 1024

# This is a partial workaround to this upstream Kubernetes issue:
# https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
net.ipv4.tcp_retries2=8
net.core.message_burst=80
net.core.message_cost=40
net.core.somaxconn=16384
net.ipv4.tcp_max_syn_backlog=16384
net.ipv4.neigh.default.gc_thresh1=4096
net.ipv4.neigh.default.gc_thresh2=8192
net.ipv4.neigh.default.gc_thresh3=16384
EOF

# adust flags as desired
tee /etc/default/kubelet > /dev/null <<EOF
KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=nodepool1,kubernetes.azure.com/mode=system,kubernetes.azure.com/role=agent"
KUBELET_FLAGS="--address=0.0.0.0 --anonymous-auth=false --authentication-token-webhook=true --authorization-mode=Webhook --cgroup-driver=systemd --cgroups-per-qos=true --client-ca-file=/etc/kubernetes/certs/ca.crt --cloud-config=/etc/kubernetes/azure.json --cloud-provider=azure --cluster-dns=10.0.0.10 --cluster-domain=cluster.local --enforce-node-allocatable=pods --event-qps=0 --eviction-hard=memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5% --feature-gates=DisableAcceleratorUsageMetrics=false,RotateKubeletServerCertificate=true --image-gc-high-threshold=85 --image-gc-low-threshold=80 --keep-terminated-pod-volumes=false --kube-reserved=cpu=100m,memory=1638Mi --kubeconfig=/var/lib/kubelet/kubeconfig --max-pods=110 --node-status-update-frequency=10s --pod-infra-container-image=mcr.microsoft.com/oss/kubernetes/pause:3.6 --pod-manifest-path=/etc/kubernetes/manifests --pod-max-pids=-1 --protect-kernel-defaults=true --read-only-port=0 --resolv-conf=/run/systemd/resolve/resolv.conf --rotate-certificates=true --rotate-server-certificates=true  --streaming-connection-idle-timeout=4h --tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256 --tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key"
EOF

# can simplify this + 2 following files by merging together
tee /etc/systemd/system/kubelet.service.d/10-containerd.conf > /dev/null <<'EOF'
[Service]
Environment=KUBELET_CONTAINERD_FLAGS="--container-runtime=remote --runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock"
EOF

tee /etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf > /dev/null <<'EOF'
[Service]
Environment=KUBELET_TLS_BOOTSTRAP_FLAGS="--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
EOF

tee /etc/systemd/system/kubelet.service > /dev/null <<'EOF'
[Unit]
Description=Kubelet
ConditionPathExists=/usr/local/bin/kubelet
[Service]
Restart=always
EnvironmentFile=/etc/default/kubelet
SuccessExitStatus=143
# Ace does not recall why this is done
ExecStartPre=/bin/bash -c "if [ $(mount | grep \"/var/lib/kubelet\" | wc -l) -le 0 ] ; then /bin/mount --bind /var/lib/kubelet /var/lib/kubelet ; fi"
ExecStartPre=/bin/mount --make-shared /var/lib/kubelet
ExecStartPre=-/sbin/ebtables -t nat --list
ExecStartPre=-/sbin/iptables -t nat --numeric --list
ExecStart=/usr/local/bin/kubelet \
        --enable-server \
        --node-labels="${KUBELET_NODE_LABELS}" \
        --v=2 \
        --volume-plugin-dir=/etc/kubernetes/volumeplugins \
        $KUBELET_TLS_BOOTSTRAP_FLAGS \
        $KUBELET_CONFIG_FILE_FLAGS \
        $KUBELET_CONTAINERD_FLAGS \
        $KUBELET_FLAGS
[Install]
WantedBy=multi-user.target
EOF

# only dynamic data?
# tee /var/lib/kubelet/bootstrap-kubeconfig > /dev/null <<EOF
# <<BOOTSTRAP_KUBECONFIG>>
# EOF

tee /var/lib/kubelet/bootstrap-kubeconfig > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: "FQDN_PLACE_HOLDER"
users:
- name: kubelet-bootstrap
  user:
    token: "TOKEN_PLACE_HOLDER"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF

echo 'KUBE_CA_CERT_PLACE_HOLDER' | base64 -d > /etc/kubernetes/certs/ca.crt > /dev/null

AZURE_JSON_PATH="/etc/kubernetes/azure.json"
touch "${AZURE_JSON_PATH}"
chmod 0600 "${AZURE_JSON_PATH}"
chown root:root "${AZURE_JSON_PATH}"

KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"
openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 4096
openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=system:node:${NODE_NAME}"

sysctl --system
systemctl enable --now containerd
systemctl enable --now kubelet

# sanity check? might be uninitialized at this point
# timeout 30s grep -q 'NodeReady' <(journalctl -u kubelet -f --no-tail)
