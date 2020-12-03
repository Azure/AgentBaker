#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

kubectl label --overwrite nodes $NODE_NAME $KUBELET_NODE_LABELS
#EOF