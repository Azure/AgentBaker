#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

# TODO(charliedmcb): replace template <Node Name>. Still trying to find how to access the current node's name. 
kubectl label --overwrite nodes <Node Name> $KUBELET_NODE_LABELS
#EOF
