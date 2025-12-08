#!/bin/bash
# Script to generate disk usage diagnostics for VHD builds
# Used by both install-dependencies.sh and error-cleanup-provisioner

DISK_USAGE_FILE="/opt/azure/disk-usage.txt"

{
  echo "=== Disk Space Diagnostics ==="
  echo "Timestamp: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo ""
  echo "--- df -h ---"
  df -h
  echo ""
  echo "--- Containerd content store ---"
  du -sh /var/lib/containerd/io.containerd.content.v1.content/ 2>/dev/null || echo "Could not read content store"
  echo ""
  echo "--- Containerd snapshotter ---"
  du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/ 2>/dev/null || echo "Could not read snapshotter"
  echo ""
  echo "--- Kubernetes downloads ---"
  du -sh /opt/kubernetes/* 2>/dev/null || echo "Could not read /opt/kubernetes"
  echo ""
  echo "--- Container images (sorted by size, top 20) ---"
  ctr --namespace k8s.io images list 2>/dev/null | tail -n +2 | sort -k4 -h | tail -20 || echo "Could not list images"
  echo ""
  echo "=== End Disk Space Diagnostics ==="
} | tee "$DISK_USAGE_FILE"
