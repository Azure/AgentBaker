#!/bin/bash

CNI_PREFETCH_SCRIPT_PATH="/opt/azure/containers/cni-prefetch.sh"
if [ ! -f "$CNI_PREFETCH_SCRIPT_PATH" ]; then
    echo "CNI prefetch driver script does not exist at: $CNI_PREFETCH_SCRIPT_PATH"
    exit 1
fi

chmod +x $CNI_PREFETCH_SCRIPT_PATH
echo "running CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
$CNI_PREFETCH_SCRIPT_PATH
echo "CNI prefetch driver script completed successfully"

echo "deleting CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
rm -- "$0"
