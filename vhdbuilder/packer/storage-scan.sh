#!/usr/bin/env bash
set -euxo pipefail

STORAGE_REPORT_PATH=/opt/azure/containers/storage-report.txt

mkdir -p "$(dirname "${STORAGE_REPORT_PATH}")"

sudo find / -type f -size +1M -exec du -h {} + | sort -h >> $STORAGE_REPORT_PATH
echo "----" >> $STORAGE_REPORT_PATH
df -h >> $STORAGE_REPORT_PATH

chmod a+r "${STORAGE_REPORT_PATH}"