#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

for kubelet_label in $(echo $KUBELET_NODE_LABELS | sed "s/,/ /g")
do
  kubectl label --overwrite nodes $HOSTNAME $kubelet_label
done
#EOF
