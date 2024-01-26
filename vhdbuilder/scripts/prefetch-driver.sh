#!/bin/bash
set -eux

CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH="/opt/azure/containers/prefetch.sh"

if [ ! -f "$CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH" ]; then
    echo "container image prefetch script path does not exist at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH, exiting early..."
    exit 0
fi

echo "running container image prefetch script at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH..."
sudo chmod +x $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH || exit $?
sudo /bin/bash $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH || exit $?
echo "container image prefetch script completed successfully"

echo "removing container image prefetch script at $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH..."
sudo rm -f $CONTAINER_IMAGE_PREFETCH_SCRIPT_PATH || exit $?
echo "removed container image prefetch script"
