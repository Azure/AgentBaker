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

if ! find / -type f -size +1M -exec du -h {} + 2>/dev/null | sort -rh >> "$STORAGE_REPORT_PATH"; then
    error_message=$(find / -type f -size +1M -exec du -h {} + 2>&1 >/dev/null | sort -rh)
    echo "Error: $error_message" >&2
fi

cd "$CUR_DIR"
umount /mnt/sdb1
rmdir /mnt/sdb1


chmod a+r "${STORAGE_REPORT_PATH}"
