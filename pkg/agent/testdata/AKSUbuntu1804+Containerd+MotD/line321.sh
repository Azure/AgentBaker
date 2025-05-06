#!/bin/bash

if [ -f "/opt/azure/containers/validate-kubelet-credentials.sh" ]; then
    if ! /bin/bash /opt/azure/containers/validate-kubelet-credentials.sh; then
        echo "kubelet credential validation faled, will still continue to start kubelet"
    fi
fi

/usr/local/bin/kubelet \
    --enable-server \
    --node-labels="${KUBELET_NODE_LABELS}" \
    --v=2 \
    --volume-plugin-dir=/etc/kubernetes/volumeplugins \
    $KUBELET_TLS_BOOTSTRAP_FLAGS \
    $KUBELET_CONFIG_FILE_FLAGS \
    $KUBELET_CONTAINERD_FLAGS \
    $KUBELET_CONTAINER_RUNTIME_FLAG \
    $KUBELET_CGROUP_FLAGS \
    $KUBELET_FLAGS