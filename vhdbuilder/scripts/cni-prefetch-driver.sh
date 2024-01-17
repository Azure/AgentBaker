#!/bin/bash
CNI_PREFETCH_SCRIPT_PATH="/opt/azure/containers/prefetch.sh"

if [ ! -f "$CNI_PREFETCH_SCRIPT_PATH" ]; then
    err "$test: CNI prefetch script does not exist at $CNI_PREFETCH_SCRIPT_PATH"
    return 1
fi

chmod +x $CNI_PREFETCH_SCRIPT_PATH
echo "running CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
sudo /bin/bash $CNI_PREFETCH_SCRIPT_PATH
echo "CNI prefetch driver script completed successfully"

echo "deleting CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
rm -- "$0"
echo "CNI prefetch driver script deleted"
