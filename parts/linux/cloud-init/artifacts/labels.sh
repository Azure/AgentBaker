#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

# TODO(charliedmcb): confirm that NODE_NAME and KUBELET_NODE_LABELS are correct, and are accessible here.
kubectl label --overwrite nodes $NODE_NAME $KUBELET_NODE_LABELS
#EOF
