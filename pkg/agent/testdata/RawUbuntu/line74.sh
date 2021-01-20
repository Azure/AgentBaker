#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

for kubelet_label in $(echo $KUBELET_NODE_LABELS | sed "s/,/ /g")
do
  kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --overwrite nodes $HOSTNAME $kubelet_label
done
#EOF
