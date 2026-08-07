#!/bin/bash
# Script to generate disk usage diagnostics for VHD builds.
# Used by packer provisioner and error-cleanup-provisioner.
# NOTE: This script is for diagnostic purposes only and is not critical.
# Failures in this script should not affect the VHD build process.

DISK_USAGE_FILE="${DISK_USAGE_FILE:-/opt/azure/disk-usage.txt}"
DISK_USAGE_SCAN_ROOT="${DISK_USAGE_SCAN_ROOT:-/}"
DISK_USAGE_TOTAL_TIMEOUT_SECONDS="${DISK_USAGE_TOTAL_TIMEOUT_SECONDS:-240}"
DISK_USAGE_COMMAND_TIMEOUT_SECONDS="${DISK_USAGE_COMMAND_TIMEOUT_SECONDS:-60}"
DISK_USAGE_KILL_AFTER_SECONDS="${DISK_USAGE_KILL_AFTER_SECONDS:-5}"
CONTAINERD_ROOT="${CONTAINERD_ROOT:-/var/lib/containerd}"

if [ "$DISK_USAGE_SCAN_ROOT" = "/" ]; then
  SCAN_CONTAINERD_PATH="/var/lib/containerd"
  SCAN_PROC_PATH="/proc"
  SCAN_SYS_PATH="/sys"
  SCAN_DEV_PATH="/dev"
  SCAN_RUN_PATH="/run"
  SCAN_MNT_PATH="/mnt"
else
  scan_root="${DISK_USAGE_SCAN_ROOT%/}"
  SCAN_CONTAINERD_PATH="${scan_root}/var/lib/containerd"
  SCAN_PROC_PATH="${scan_root}/proc"
  SCAN_SYS_PATH="${scan_root}/sys"
  SCAN_DEV_PATH="${scan_root}/dev"
  SCAN_RUN_PATH="${scan_root}/run"
  SCAN_MNT_PATH="${scan_root}/mnt"
fi

# Ensure the directory exists and is accessible
mkdir -p "$(dirname "$DISK_USAGE_FILE")"
chmod 755 "$(dirname "$DISK_USAGE_FILE")"

# Packer treats five minutes without SSH traffic as a disconnect. Keep this
# best-effort diagnostic comfortably below that limit and never fail the VHD
# build if collection is slow or broken.
if [ "${DISK_USAGE_COLLECTOR_CHILD:-0}" != "1" ] && command -v timeout >/dev/null 2>&1; then
  export DISK_USAGE_COLLECTOR_CHILD=1
  timeout \
    --signal=TERM \
    --kill-after="${DISK_USAGE_KILL_AFTER_SECONDS}s" \
    "${DISK_USAGE_TOTAL_TIMEOUT_SECONDS}s" \
    /bin/bash "$0" "$@"
  collector_status=$?

  if [ "$collector_status" -ne 0 ]; then
    if [ "$collector_status" -eq 124 ] || [ "$collector_status" -eq 137 ]; then
      warning="WARNING: disk usage diagnostics exceeded ${DISK_USAGE_TOTAL_TIMEOUT_SECONDS} seconds and were stopped"
    else
      warning="WARNING: disk usage diagnostics exited with status ${collector_status}"
    fi
    printf '%s\n' "$warning" | tee -a "$DISK_USAGE_FILE"
  fi

  chmod 644 "$DISK_USAGE_FILE" 2>/dev/null || true
  exit 0
fi

run_with_timeout() {
  local timeout_seconds="$1"
  shift

  if command -v timeout >/dev/null 2>&1; then
    timeout \
      --foreground \
      --signal=TERM \
      --kill-after="${DISK_USAGE_KILL_AFTER_SECONDS}s" \
      "${timeout_seconds}s" \
      "$@"
  else
    "$@"
  fi
}

report_timeout() {
  local description="$1"
  local status="$2"

  if [ "$status" -eq 124 ] || [ "$status" -eq 137 ]; then
    echo "WARNING: ${description} exceeded ${DISK_USAGE_COMMAND_TIMEOUT_SECONDS} seconds and was skipped"
  elif [ "$status" -ne 0 ]; then
    echo "WARNING: ${description} exited with status ${status}"
  fi
}

directory_size() {
  local path="$1"
  local output
  local status

  # Snapshotter directories can contain mounted layer filesystems. Restrict
  # accounting to the backing filesystem so du never walks mounted fs trees.
  output=$(run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" du -x -sh "$path" 2>/dev/null)
  status=$?
  if [ "$status" -eq 0 ] && [ -n "$output" ]; then
    printf '%s\n' "$output" | awk '{print $1}'
  elif [ "$status" -eq 124 ] || [ "$status" -eq 137 ]; then
    printf 'timed out\n'
  else
    printf 'N/A\n'
  fi
}

shallow_file_size() {
  local path="$1"
  local max_depth="$2"
  local bytes
  local status

  bytes=$(
    run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" \
      find "$path" -xdev -mindepth 1 -maxdepth "$max_depth" -type f -printf '%b\n' 2>/dev/null | \
      awk '{ blocks += $1 } END { printf "%.0f\n", blocks * 512 }'
    exit "${PIPESTATUS[0]}"
  )
  status=$?

  if [ "$status" -eq 0 ] && [ -n "$bytes" ]; then
    awk -v bytes="$bytes" '
      BEGIN {
        split("B K M G T", units, " ")
        unit = 1
        while (bytes >= 1024 && unit < 5) {
          bytes /= 1024
          unit++
        }
        if (unit == 1) {
          printf "%.0f%s\n", bytes, units[unit]
        } else {
          printf "%.1f%s\n", bytes, units[unit]
        }
      }
    '
  elif [ "$status" -eq 124 ] || [ "$status" -eq 137 ]; then
    printf 'timed out\n'
  else
    printf 'N/A\n'
  fi
}

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
    run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" \
      ctr --namespace k8s.io images list 2>/dev/null | tail -n +2 | \
      awk '!/^sha256:/ {printf "%s %s\t%s\n", $4, $5, $1}' | \
      sort -t$'\t' -k1 -hr
    report_timeout "container image listing" "${PIPESTATUS[0]}"
  else
    echo "ctr not available"
  fi
  echo ""

  echo "----------------------------------------------"
  echo "LARGEST DIRECTORIES (over 100MB)"
  echo "----------------------------------------------"
  # Stay on the writable root filesystem and skip containerd, which is
  # summarized separately below. The old unbounded `du -h / | sort` traversed
  # every preloaded image and emitted nothing while sort buffered its output.
  run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" \
    du -x -h --max-depth=4 --exclude="$SCAN_CONTAINERD_PATH" "$DISK_USAGE_SCAN_ROOT" 2>/dev/null | \
    awk '$1 ~ /[0-9]+(G|[1-9][0-9][0-9]M)/' | sort -hr
  report_timeout "largest-directory scan" "${PIPESTATUS[0]}"
  echo ""

  echo "----------------------------------------------"
  echo "LARGEST FILES (over 100MB)"
  echo "----------------------------------------------"
  run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" \
    find "$DISK_USAGE_SCAN_ROOT" -xdev \
      \( -path "$SCAN_CONTAINERD_PATH" -o -path "$SCAN_PROC_PATH" -o -path "$SCAN_SYS_PATH" -o -path "$SCAN_DEV_PATH" -o -path "$SCAN_RUN_PATH" -o -path "$SCAN_MNT_PATH" \) -prune -o \
      -type f -size +100M -printf '%s\t%p\n' 2>/dev/null | \
    sort -t$'\t' -k1,1nr | \
    awk -F '\t' '
      function human(bytes, value, unit) {
        split("B KiB MiB GiB TiB", units, " ")
        value = bytes
        unit = 1
        while (value >= 1024 && unit < 5) {
          value /= 1024
          unit++
        }
        return sprintf("%.1f %s", value, units[unit])
      }
      { print human($1) "\t" $2 }
    '
  report_timeout "largest-file scan" "${PIPESTATUS[0]}"
  echo ""

  echo "----------------------------------------------"
  echo "/opt BREAKDOWN"
  echo "----------------------------------------------"
  run_with_timeout "$DISK_USAGE_COMMAND_TIMEOUT_SECONDS" du -x -sh /opt/*/ 2>/dev/null | sort -hr
  report_timeout "/opt breakdown" "${PIPESTATUS[0]}"
  echo ""

  echo "----------------------------------------------"
  echo "CONTAINERD STORAGE SUMMARY"
  echo "----------------------------------------------"
  content_size=$(directory_size "$CONTAINERD_ROOT/io.containerd.content.v1.content/")
  overlayfs_size=$(directory_size "$CONTAINERD_ROOT/io.containerd.snapshotter.v1.overlayfs/")
  # EROFS snapshots store backing files at snapshots/<id>/. A fixed-depth
  # search accounts for those files without entering mounted snapshots/<id>/fs.
  erofs_size=$(shallow_file_size "$CONTAINERD_ROOT/io.containerd.snapshotter.v1.erofs/snapshots" 2)
  echo "Content store (compressed blobs): ${content_size:-N/A}"
  echo "Overlayfs snapshotter:            ${overlayfs_size:-N/A}"
  echo "EROFS snapshotter backing files:  ${erofs_size:-N/A}"
  echo ""

  END_TIME=$(date +%s)
  echo "----------------------------------------------"
  echo "Total collection time: $((END_TIME - START_TIME)) seconds"
  echo "=============================================="
} | tee "$DISK_USAGE_FILE"

# Make file readable for packer SCP download
chmod 644 "$DISK_USAGE_FILE"
