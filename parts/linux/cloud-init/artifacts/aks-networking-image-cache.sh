#!/bin/bash
set -uo pipefail

FEATURES_PATH="${FEATURES_PATH:-/opt/azure/containers/enabled_features.sh}"
LAUNCHER_PATH="${LAUNCHER_PATH:-/opt/azure/containers/aks-node-controller-launcher.sh}"
grep -qx 'ENABLE_NETWORKING_IMAGE_CACHE=true' "$FEATURES_PATH" 2>/dev/null || exit 0
exec "$LAUNCHER_PATH" prepull-networking-images
