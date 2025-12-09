#!/bin/bash
# Script to generate disk usage diagnostics for VHD builds
# Used by packer provisioner and error-cleanup-provisioner

DISK_USAGE_FILE="/opt/azure/disk-usage.txt"

# Ensure the directory exists
mkdir -p /opt/azure

START_TIME=$(date +%s)

{
  echo "=============================================="
  echo "        DISK SPACE DIAGNOSTICS REPORT"
  echo "=============================================="
  echo "Timestamp: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo ""

  echo "----------------------------------------------"
  echo "FILESYSTEM USAGE"
  echo "----------------------------------------------"
  df -h | grep -E '^Filesystem|^/dev/'
  echo ""

  echo "----------------------------------------------"
  echo "CONTAINER IMAGES (manifest size)"
  echo "----------------------------------------------"
  echo "Note: Sizes shown are compressed manifest sizes, not actual disk usage."
  echo "Actual unpacked size is in CONTAINERD STORAGE SUMMARY below."
  echo ""
  if command -v ctr &>/dev/null; then
    ctr --namespace k8s.io images list 2>/dev/null | tail -n +2 | \
      awk '{print $1, $4, $5}' | \
      grep -v '^sha256:' | \
      sort -k2 -hr
  else
    echo "ctr not available"
  fi
  echo ""

  echo "----------------------------------------------"
  echo "LARGEST DIRECTORIES (over 100MB)"
  echo "----------------------------------------------"
  du -h / 2>/dev/null | awk '$1 ~ /[0-9]+(G|[1-9][0-9][0-9]M)/' | sort -hr
  echo ""

  echo "----------------------------------------------"
  echo "LARGEST FILES (over 100MB)"
  echo "----------------------------------------------"
  find / -type f -size +100M ! -path "/proc/*" -exec ls -lh {} \; 2>/dev/null | awk '{print $5, $9}' | sort -hr
  echo ""

  echo "----------------------------------------------"
  echo "/opt BREAKDOWN"
  echo "----------------------------------------------"
  du -sh /opt/*/ 2>/dev/null | sort -hr || echo "Could not read /opt"
  echo ""

  echo "----------------------------------------------"
  echo "CONTAINERD STORAGE SUMMARY"
  echo "----------------------------------------------"
  content_size=$(du -sh /var/lib/containerd/io.containerd.content.v1.content/ 2>/dev/null | awk '{print $1}')
  snap_size=$(du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/ 2>/dev/null | awk '{print $1}')
  echo "Content store (compressed blobs): ${content_size:-N/A}"
  echo "Snapshotter (unpacked layers):    ${snap_size:-N/A}"
  echo ""

  END_TIME=$(date +%s)
  echo "----------------------------------------------"
  echo "Total collection time: $((END_TIME - START_TIME)) seconds"
  echo "=============================================="
} | tee "$DISK_USAGE_FILE"
