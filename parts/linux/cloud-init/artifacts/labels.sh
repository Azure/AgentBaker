#!/usr/bin/env bash

# Update Labels for Kubernetes nodes

set -euo pipefail

charliedmcb=${CHARLIEDMCB:-"false"}
if [ $charliedmcb == "true" ]; then
    echo "HOSTNAME:"
    echo $HOSTNAME
    LABELS={{GetAgentKubernetesLabels . }}
    echo "LABELS:"
    echo $LABELS
fi

kubectl label --overwrite nodes $HOSTNAME {{GetAgentKubernetesLabels . }}
#EOF
