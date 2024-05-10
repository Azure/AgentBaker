#!/bin/bash

set -euo pipefail

KUBELET_CONFIG_FILE_FLAGS="${KUBELET_CONFIG_FILE_FLAGS:-""}"
KUBELET_CONTAINERD_FLAGS="${KUBELET_CONTAINERD_FLAGS:-""}"
KUBELET_CONTAINER_RUNTIME_FLAG="${KUBELET_CONTAINER_RUNTIME_FLAG:-""}"
KUBELET_CGROUP_FLAGS="${KUBELET_CGROUP_FLAGS:-""}"
KUBELET_FLAGS="${KUBELET_FLAGS:-""}"

setKubeletTLSBootstrapFlags() {
  KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
  BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
  KUBELET_TLS_BOOTSTRAP_FLAGS="--kubeconfig /var/lib/kubelet/kubeconfig"

  if [ -f "${KUBECONFIG_FILE}" ]; then
    rm -f "${BOOTSTRAP_KUBECONFIG_FILE}"
    return 0
  fi

  if [ -f "${BOOTSTRAP_KUBECONFIG_FILE}" ]; then
    KUBELET_TLS_BOOTSTRAP_FLAGS="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
  fi
}

setKubeletTLSBootstrapFlags

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