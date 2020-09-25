#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# Bind mounts kubelet and container runtime directories to ephemeral
# disks as appropriate on startup.

{{if eq GetKubeletDisk "Temporary"}}
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

# echo "stopping container runtime: '${CONTAINER_RUNTIME}'"
# systemctl stop "${CONTAINER_RUNTIME}" || true

# echo "unmounting '/var/lib/${CONTAINER_RUNTIME}'"
# umount "/var/lib/${CONTAINER_RUNTIME}" || true
# mkdir -p "/var/lib/${CONTAINER_RUNTIME}"

KUBELET_DIR="/var/lib/kubelet"
mkdir -p "${MOUNT_POINT}/kubelet"
mkdir -p "${KUBELET_DIR}"
mount --bind "${MOUNT_POINT}" "${KUBELET_DIR}"
chmod a+w "${KUBELET_DIR}"
