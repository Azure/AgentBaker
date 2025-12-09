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
  echo "CONTAINER IMAGES BY UNPACKED SIZE"
  echo "----------------------------------------------"
  echo "(Showing actual disk usage per image)"
  echo ""
  # Build a mapping of snapshot parents to find root snapshots for each image
  # Then calculate total size per image
  if command -v ctr &>/dev/null; then
    # Get list of images with their digests
    while IFS= read -r line; do
      image_name=$(echo "$line" | awk '{print $1}')
      # Skip sha256 references (duplicates)
      if [[ "$image_name" == sha256:* ]]; then
        continue
      fi
      # Get the unpacked size by checking the snapshot
      size=$(ctr --namespace k8s.io images usage "$image_name" 2>/dev/null | tail -1 | awk '{print $1}')
      if [[ -n "$size" && "$size" != "0" ]]; then
        echo "$size $image_name"
      fi
    done < <(ctr --namespace k8s.io images list 2>/dev/null | tail -n +2) | sort -hr
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
  find / -type f -size +100M -exec ls -lh {} \; 2>/dev/null | awk '{print $5, $9}' | grep -v '^128T' | sort -hr
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
