#!/bin/bash
# Script to generate disk usage diagnostics for VHD builds.
# Pass --failure to also print guest systemd diagnostics during error cleanup.
# Used by packer provisioner and error-cleanup-provisioner.
# NOTE: This script is for diagnostic purposes only and is not critical.
# Failures in this script should not affect the VHD build process.

run_failure_diagnostic() {
  printf '\n--- %s ---\n' "$*"
  "$@" 2>&1 || echo "WARNING: diagnostic failed (exit $?): $*"
}

# Capture volatile failure state before the potentially lengthy disk scan.
if [ "${1:-}" = "--failure" ]; then
  echo "=============================================="
  echo "        SYSTEMD FAILURE DIAGNOSTICS"
  echo "=============================================="
  run_failure_diagnostic ps -p 1 -o pid,stat,comm,args
  run_failure_diagnostic readlink /proc/1/exe
  run_failure_diagnostic ss -xlpn
  run_failure_diagnostic ls -ld /run/systemd/private
  run_failure_diagnostic ls -ld /run/dbus/system_bus_socket
  run_failure_diagnostic journalctl --boot --since '15 minutes ago' --no-pager -o short-precise
  run_failure_diagnostic dmesg --ctime --since '15 minutes ago'
fi

DISK_USAGE_FILE="/opt/azure/disk-usage.txt"

# Ensure the directory exists and is accessible
mkdir -p /opt/azure
chmod 755 /opt/azure

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
    # ctr images list format: REF TYPE DIGEST SIZE UNIT PLATFORMS LABELS
    # We want SIZE+UNIT (col 4-5) and REF (col 1), filtering out sha256: digest refs
    ctr --namespace k8s.io images list 2>/dev/null | tail -n +2 | \
      awk '!/^sha256:/ {printf "%s %s\t%s\n", $4, $5, $1}' | \
      sort -t$'\t' -k1 -hr
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

# Make file readable for packer SCP download
chmod 644 "$DISK_USAGE_FILE"
