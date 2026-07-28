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
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.4.56" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.4.59" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.5.38" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.5.35" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.6.13" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cni:v1.6.18" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.4.56" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.4.59" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.5.38" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.5.35" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.6.13" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.6.18" "/usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/containernetworking/azure-ipam:v0.0.7" "/dropgz"
prefetch "mcr.microsoft.com/containernetworking/azure-ipam:v0.2.0" "/dropgz"

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
