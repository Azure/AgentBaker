#!/bin/bash
set -euo pipefail

SCRIPT_PATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
COMPONENTS_RELPATH="$SCRIPT_PATH/../../../parts/linux/cloud-init/artifacts/components.json"

main() {
    if [ ! -f "$COMPONENTS_RELPATH" ]; then
        echo "unable to generate testdata, components.json does not exist at: $COMPONENTS_RELPATH"
        exit 1
    fi

    echo "generating prefetch testdata from $COMPONENTS_RELPATH"
    TESTDATA_PATH="$SCRIPT_PATH/../internal/containerimage/testdata"
    mkdir -p "$TESTDATA_PATH"
    cp -r "$COMPONENTS_RELPATH" "$TESTDATA_PATH"
}

main "$@"