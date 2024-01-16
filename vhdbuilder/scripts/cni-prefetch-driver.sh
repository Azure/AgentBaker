#!/bin/bash

CNI_PREFETCH_SCRIPT_PATH="/opt/azure/containers/cni-prefetch.sh"
if [[ ! -f "$CNI_PREFETCH_SCRIPT_PATH" ]]; then
    echo "CNI Prefetch file does not exist at: $CNI_PREFETCH_SCRIPT_PATH"
    exit 1
fi

chmod +x $CNI_PREFETCH_SCRIPT_PATH
echo "running cni-prefetch.sh ..."
$CNI_PREFETCH_SCRIPT_PATH