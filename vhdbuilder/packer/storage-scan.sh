#!/usr/bin/env bash
set -euxo pipefail

STORAGE_REPORT_PATH=/opt/azure/containers/storage-report.txt

mkdir -p "$(dirname "${STORAGE_REPORT_PATH}")"

echo "----" >> $STORAGE_REPORT_PATH
df -h 2>&1 >> $STORAGE_REPORT_PATH
echo "----" >> $STORAGE_REPORT_PATH
find / -type f -size +1M -exec du -h {} + 2>/dev/null | sort -rh >> $STORAGE_REPORT_PATH


chmod a+r "${STORAGE_REPORT_PATH}"