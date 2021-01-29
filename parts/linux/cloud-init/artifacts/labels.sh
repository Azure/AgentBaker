#!/usr/bin/env bash

# Update Labels for this Kubernetes node

set -euo pipefail

NODE_NAME=$(echo $(hostname) | tr "[:upper:]" "[:lower:]")

echo "updating labels for ${NODE_NAME}"

CURRENT_NODE_LABELS=$(kubectl label --list=true $NODE_NAME)

echo "current node labels: ${CURRENT_NODE_LABELS}"

FORMATTED_NODE_LABELS_TO_ENSURE=$(echo $KUBELET_NODE_LABELS | sed "s/,/ /g")

echo "formatted node labels to ensure: ${FORMATTED_NODE_LABELS_TO_ENSURE}"

MISSING_LABELS=$(comm -32 <(printf $FORMATTED_KUBELET_NODE_LABELS) <(printf $CURRENT_NODE_LABELS))

echo "missing labels: ${MISSING_LABELS}"

kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --overwrite nodes $NODE_NAME $MISSING_LABELS
#EOF
