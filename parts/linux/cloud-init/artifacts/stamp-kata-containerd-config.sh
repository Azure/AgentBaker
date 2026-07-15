#!/bin/bash
# stamp-kata-containerd-config.sh
#
# One-shot, once-per-node helper that overwrites /etc/containerd/config.toml with
# the desired Kata + erofs containerd configuration and restarts containerd.
#
# This is triggered by stamp-kata-containerd-config.path, which only activates the
# accompanying service once /opt/azure/containers/provision.complete exists (i.e.
# after node provisioning/CSE has finished). A marker file guarantees the stamp is
# applied at most once per node, even across reboots when the .path unit re-fires.
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

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

stampKataContainerdConfig
