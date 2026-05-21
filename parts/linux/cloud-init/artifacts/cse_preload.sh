#!/bin/bash

# cse_preload.sh warms binaries and containerd caches needed by CSE early in
# boot so that node provisioning runs against an already-warm page cache.
# This is best-effort: every command is backgrounded and its output and exit
# status are intentionally ignored. It must never block or fail provisioning.

/usr/bin/containerd --version >/dev/null 2>&1 &
cat /var/lib/containerd/io.containerd.metadata.v1.bolt/meta.db >/dev/null 2>&1 &
find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots -maxdepth 1 >/dev/null 2>&1 &
/sbin/modprobe overlay >/dev/null 2>&1 &

wait
