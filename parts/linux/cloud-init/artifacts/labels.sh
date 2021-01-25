#!/usr/bin/env bash

# Update Labels for this Kubernetes node

set -euo pipefail

echo "updating labels for ${HOSTNAME}"

for kubelet_label in $(echo $KUBELET_NODE_LABELS | sed "s/,/ /g")
do
  echo "updating label: ${kubelet_label}"
  kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --overwrite nodes $HOSTNAME $kubelet_label
done
#EOF
