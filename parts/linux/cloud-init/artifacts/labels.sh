#!/usr/bin/env bash

# Update Labels for this Kubernetes node

set -euo pipefail

NODE_NAME=$(echo $(hostname) | tr "[:upper:]" "[:lower:]")

echo "updating labels for ${NODE_NAME}"

for kubelet_label in $(echo $KUBELET_NODE_LABELS | sed "s/,/ /g")
do
  echo "updating label: ${kubelet_label}"
  kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --overwrite nodes $NODE_NAME $kubelet_label
done
#EOF
