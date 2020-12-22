#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

kubectl label --overwrite nodes $HOSTNAME KUBELET_NODE_LABELS
#EOF
