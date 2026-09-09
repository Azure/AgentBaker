#!/bin/bash
# One-shot, once-per-node helper that overwrites /etc/containerd/config.toml with
# the desired Kata + erofs containerd configuration and restarts containerd.
#
# The path unit triggers this after node provisioning finishes. A marker file
# ensures the config is stamped at most once per node, including across reboots.
set -uo pipefail

MARKER_FILE="${MARKER_FILE:-/opt/azure/containers/.kata-containerd-config-stamped}"
CONFIG_SRC="${CONFIG_SRC:-/opt/azure/containers/kata-containerd-config.toml}"
CONFIG_DEST="${CONFIG_DEST:-/etc/containerd/config.toml}"

stampKataContainerdConfig() {
    if [ -f "${MARKER_FILE}" ]; then
        echo "kata containerd config already stamped (found ${MARKER_FILE}), nothing to do"
        return 0
    fi

    if [ ! -f "${CONFIG_SRC}" ]; then
        echo "desired kata containerd config not found at ${CONFIG_SRC}, cannot stamp" >&2
        return 1
    fi

    echo "stamping ${CONFIG_DEST} with desired kata containerd config from ${CONFIG_SRC}"
    if ! install -m 0644 "${CONFIG_SRC}" "${CONFIG_DEST}"; then
        echo "failed to write ${CONFIG_DEST}" >&2
        return 1
    fi

    echo "restarting containerd to apply new config"
    if ! systemctl restart containerd; then
        echo "failed to restart containerd" >&2
        return 1
    fi

    touch "${MARKER_FILE}"
    echo "kata containerd config stamped successfully"
}

# This prevents ShellSpec from executing the entry point when sourcing the file.
${__SOURCED__:+return}

stampKataContainerdConfig