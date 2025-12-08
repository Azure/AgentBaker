#!/bin/bash
# Script to generate disk usage diagnostics for VHD builds
# Used by packer provisioner and error-cleanup-provisioner

DISK_USAGE_FILE="/opt/azure/disk-usage.txt"

# Ensure the directory exists
mkdir -p /opt/azure

START_TIME=$(date +%s)

{
  echo "=== Disk Space Diagnostics ==="
  echo "Timestamp: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo ""
  echo "--- df -h ---"
  df -h
  echo ""
  echo "--- Directories over 100MB ---"
  du -h / 2>/dev/null | awk '$1 ~ /[0-9]+(G|[1-9][0-9][0-9]M)/' | sort -hr
  echo ""
  echo "--- /opt breakdown ---"
  du -sh /opt/*/ 2>/dev/null | sort -hr || echo "Could not read /opt"
  echo ""
  echo "--- Files over 100MB ---"
  find / -type f -size +100M -exec ls -lh {} \; 2>/dev/null | awk '{print $5, $9}' | sort -hr
  echo ""
  echo "--- Containerd content store ---"
  du -sh /var/lib/containerd/io.containerd.content.v1.content/ 2>/dev/null || echo "Could not read content store"
  echo ""
  echo "--- Containerd snapshotter (sorted by size, descending) ---"
  du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/*/ 2>/dev/null | sort -hr || echo "Could not read snapshotter"
  echo ""
  echo "--- Containerd snapshotter total ---"
  du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/ 2>/dev/null || echo "Could not read snapshotter"
  echo ""
  echo "--- Kubernetes downloads ---"
  du -sh /opt/kubernetes/* 2>/dev/null || echo "Could not read /opt/kubernetes"
  echo ""
  echo "--- Container images (sorted by size, descending) ---"
  ctr --namespace k8s.io images list 2>/dev/null | tail -n +2 | sort -k4 -hr || echo "Could not list images"
  echo ""
  END_TIME=$(date +%s)
  echo "Total collection time: $((END_TIME - START_TIME)) seconds"
  echo "=== End Disk Space Diagnostics ==="
} | tee "$DISK_USAGE_FILE"
