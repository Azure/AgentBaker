#!/bin/bash

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
