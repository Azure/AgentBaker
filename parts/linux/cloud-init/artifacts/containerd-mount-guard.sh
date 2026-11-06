#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

waitForContainerdMount() {
    local config
    # Use containerd's parser so imports, defaults, and preserved GPU configs
    # select the same root as the daemon, without initializing its plugins.
    if ! config=$(containerd config dump); then
        echo "Unable to read containerd configuration before checking the resource disk" >&2
        return 1
    fi

    # Only inspect top-level keys. Containerd versions emit either TOML quote style.
    if ! sed '/^\[/,$d' <<< "$config" | grep -E "^root[[:space:]]*=[[:space:]]*['\"]/mnt(/|['\"])" >/dev/null; then
        return 0
    fi

    local attempt
    for ((attempt = 0; attempt < 120; attempt++)); do
        if mountpoint -q /mnt; then
            return 0
        fi
        sleep 1
    done

    echo "Timed out waiting for /mnt to be mounted before starting containerd" >&2
    return 1
}

${__SOURCED__:+return}

waitForContainerdMount
