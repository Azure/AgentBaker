#!/usr/bin/env bash
set -eux

prefetch() {
    local image=$1
    local files=$2
    
    mount_dir=$(mktemp -d)
    ctr -n k8s.io images mount "$image" "$mount_dir"

    for f in $files; do
        echo "prefetching $f in $image"
        path="${mount_dir}${f}"
        stat -c %s "$path"
        cat "$path" > /dev/null
    done

    ctr -n k8s.io images unmount "$mount_dir"
}
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.6.44-0" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.7.17-0" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/v2/azure-cni:v1.8.13" "/usr/bin/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.6.44-0" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.7.17-0" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/v2/azure-cns:v1.8.13" "/usr/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-ipam:v0.2.1" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-ipam:v0.3.0" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-ipam:v0.4.0-0" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-iptables-monitor:v0.0.5-0" "/azure-iptables-monitor /azure-block-iptables"
prefetch "mcr.microsoft.com/containernetworking/cilium/cilium-distroless:v1.18.14-260923" "/usr/bin/cilium-agent"
prefetch "mcr.microsoft.com/containernetworking/cilium/cilium-distroless:v1.19.8-260923" "/usr/bin/cilium-agent"
prefetch "mcr.microsoft.com/containernetworking/cilium/cilium-distroless-init:v1.18.14-260923" "/opt/cni/bin/cilium-cni"
prefetch "mcr.microsoft.com/containernetworking/cilium/cilium-distroless-init:v1.19.8-260923" "/opt/cni/bin/cilium-cni"

# cse_preload.sh warms binaries and containerd caches needed by CSE early in
# boot so that node provisioning runs against an already-warm page cache.
# This is best-effort: every command is backgrounded and its output and exit
# status are intentionally ignored. It must never block or fail provisioning.
preload() {
    "$@" >/dev/null 2>&1 &
}

preload /opt/azure/containers/aks-node-controller version
preload /usr/bin/containerd --version
preload cat /var/lib/containerd/io.containerd.metadata.v1.bolt/meta.db
preload find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots -maxdepth 1
preload /sbin/modprobe overlay
preload cat /opt/bin/aks-secure-tls-bootstrap-client
preload /opt/azure/containers/localdns/binary/coredns --version

wait || true
