#!/bin/bash

set -euo pipefail

TLS_BOOTSTRAP_TOKEN="${TLS_BOOTSTRAP_TOKEN:-""}"

setKubeletTLSBootstrapFlags() {
  KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
  KUBELET_TLS_BOOTSTRAP_FLAGS="--kubeconfig /var/lib/kubelet/kubeconfig"

  if [ ! -f "$KUBECONFIG_FILE" ]; then
    # if we don't have a kubelet config file, meaning that we are either bootstrapping with vanilla TLS bootstrapping
    # or with secure TLS bootstrapping but the bootstrapping process failed, then specify the bootstrap-kubeconfig file
    # containing a TLS bootstrap token so kubelet can request its own certificate

    # fail fast if we don't have a TLS bootstrap token
    if [ -z "$TLS_BOOTSTRAP_TOKEN" ]; then
        echo "ERROR: unable to write bootstrap-kubeconfig: no TLS bootstrap token has been provided"
        exit 1
    fi

    # used in vanilla TLS bootstrapping cases and when secure TLS bootstrapping has failed to generate a kubeconfig
    # by the time we need to start kubelet
    BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
    mkdir -p "$(dirname "${BOOTSTRAP_KUBECONFIG_FILE}")"
    touch "${BOOTSTRAP_KUBECONFIG_FILE}"
    chmod 0644 "${BOOTSTRAP_KUBECONFIG_FILE}"
    
    tee "${BOOTSTRAP_KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: kubelet-bootstrap
  user:
    token: "${TLS_BOOTSTRAP_TOKEN}"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF

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