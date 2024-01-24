#!/bin/bash
set -eux

CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH="/opt/azure/containers/prefetch.sh"

if [ ! -f "$CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH" ]; then
    echo "container image prefetch script path does not exist at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH, exiting early..."
    exit 0
fi

chmod +x $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH
echo "running container image prefetch script at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH..."
sudo /bin/bash $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH
echo "container image prefetch script completed successfully"

echo "deleting container image prefetch script at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH..."
rm -f $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH
echo "container image prefetch script deleted"
