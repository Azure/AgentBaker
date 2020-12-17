#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

charliedmcb=${CHARLIEDMCB:-"false"}
if [ $charliedmcb == "true" ]; then
    echo "HOSTNAME:"
    echo $HOSTNAME
    echo "NODE_LABELS:"
    echo $NODE_LABELS
fi

kubectl label --overwrite nodes $HOSTNAME $NODE_LABELS
#EOF
