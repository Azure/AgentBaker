#!/bin/bash
set -euo pipefail

SCRIPT_PATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
COMPONENTS_REL_PATH="$SCRIPT_PATH/../../../parts/linux/cloud-init/artifacts/components.json"

main() {
    if [ ! -f "$COMPONENTS_REL_PATH" ]; then
        echo "unable to generate fixtures, components.json does not exist at: $COMPONENTS_REL_PATH"
        exit 1
    fi

    echo "generating prefetch fixtures from $COMPONENTS_REL_PATH"
    CONTAINER_IMAGE_FIXTURE_PATH="$SCRIPT_PATH/../internal/containerimage/fixtures"
    mkdir -p "$CONTAINER_IMAGE_FIXTURE_PATH"
    echo "copying $COMPONENTS_REL_PATH to $CONTAINER_IMAGE_FIXTURE_PATH"
    cp -r "$COMPONENTS_REL_PATH" "$CONTAINER_IMAGE_FIXTURE_PATH"
}

main "$@"