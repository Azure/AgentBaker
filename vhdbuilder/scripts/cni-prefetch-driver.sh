#!/bin/bash

CNI_Prefetch_PATH="/opt/azure/containers/cni-prefetch.sh"
if [[ ! -f "$CNI_Prefetch_PATH" ]]; then
    echo "CNI Prefetch file does not exist at: $CNI_Prefetch_PATH"
    exit 1
fi

chmod +x $CNI_Prefetch_PATH
echo "running cni-prefetch.sh ..."
$CNI_Prefetch_PATH