
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# Bind mounts kubelet and container runtime directories to ephemeral
# disks as appropriate on startup.

{{if eq GetKubeletDiskType "Temporary"}}
MOUNT_POINT="/mnt/aks"
{{end}}

# {{if IsDockerContainerRuntime}}
#     echo "setting CONTAINER_RUNTIME to docker"
#     CONTAINER_RUNTIME="docker"
# {{end}}

# {{if NeedsContainerd}}
#     echo "setting CONTAINER_RUNTIME to containerd"
#     CONTAINER_RUNTIME="containerd"
# {{end}}

# echo "stopping container runtime: ${CONTAINER_RUNTIME}"
# systemctl stop "${CONTAINER_RUNTIME}" || true

# echo "unmounting /var/lib/${CONTAINER_RUNTIME}"
# umount "/var/lib/${CONTAINER_RUNTIME}" || true
# mkdir -p "/var/lib/${CONTAINER_RUNTIME}"

set -x

KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet"

mkdir -p "${MOUNT_POINT}"

# only move the kubelet directory to alternate location on first boot.
if [ ! -e "$SENTINEL_FILE" ]; then
    local SENTINEL_FILE="/opt/azure/containers/bind-sentinel"
    mv "$KUBELET_DIR" "$MOUNT_POINT"
    touch "$SENTINEL_FILE"
fi

# on every boot, bind mound the kubelet directory back to the expected
# location before kubelet itself may start.
mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}" 
chmod a+w "${KUBELET_DIR}"
