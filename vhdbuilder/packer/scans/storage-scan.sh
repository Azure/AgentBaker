#!/usr/bin/env bash
set -euo pipefail

STORAGE_REPORT_PATH=/opt/azure/containers/storage-report.txt

mkdir -p "$(dirname "${STORAGE_REPORT_PATH}")"

echo "----" >> $STORAGE_REPORT_PATH
{ df -h >> "$STORAGE_REPORT_PATH" ; } >> /dev/null 2>&1
echo "----" >> $STORAGE_REPORT_PATH

CUR_DIR=$(pwd)
cd /mnt/sda1

files_found=$(find . -type f -size +1M -exec du -h {} + 2>/dev/null | sort -rh)
if ! echo "$files_found" >> "$STORAGE_REPORT_PATH"; then
    echo "Error: $files_found" >&2
fi

cd "$CUR_DIR"
chmod a+r "${STORAGE_REPORT_PATH}"