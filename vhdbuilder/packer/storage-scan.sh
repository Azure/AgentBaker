#!/usr/bin/env bash
set -euxo pipefail

STORAGE_REPORT_PATH=/opt/azure/containers/storage-report.txt

mkdir -p "$(dirname "${STORAGE_REPORT_PATH}")"

echo "----" >> $STORAGE_REPORT_PATH
df -h 2>&1 >> $STORAGE_REPORT_PATH
echo "----" >> $STORAGE_REPORT_PATH

CUR_DIR=$(pwd)

mkdir -p /mnt/sdb1
mount /dev/sdb1 /mnt/sdb1
cd /mnt/sdb1

files_found=$(find . -type f -size +1M -exec du -h {} + 2>/dev/null | sort -rh)
if ! echo "$files_found" >> "$STORAGE_REPORT_PATH"; then
    echo "Error: $files_found" >&2
fi

cd "$CUR_DIR"
umount /mnt/sdb1
rmdir /mnt/sdb1


chmod a+r "${STORAGE_REPORT_PATH}"
