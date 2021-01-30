#!/usr/bin/env bash

# Update Labels for this Kubernetes node

set -euo pipefail

NODE_NAME=$(echo $(hostname) | tr "[:upper:]" "[:lower:]")

echo "updating labels for ${NODE_NAME}"

FORMATTED_CURRENT_NODE_LABELS=$(kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --list=true node $NODE_NAME | sort)

echo "current node labels (sorted): ${FORMATTED_CURRENT_NODE_LABELS}"

FORMATTED_NODE_LABELS_TO_ENSURE=$(echo $KUBELET_NODE_LABELS | tr ',' '\n' | sort)

echo "node labels to ensure (formatted+sorted): ${FORMATTED_NODE_LABELS_TO_ENSURE}"

MISSING_LABELS=$(comm -32 <(echo $FORMATTED_NODE_LABELS_TO_ENSURE | tr ' ' '\n') <(echo $FORMATTED_CURRENT_NODE_LABELS | tr ' ' '\n'))

echo "missing labels: ${MISSING_LABELS}"

if [ ! -z "$MISSING_LABELS" ]; then
  kubectl label --kubeconfig /var/lib/kubelet/kubeconfig --overwrite node $NODE_NAME $MISSING_LABELS
fi
#EOF
