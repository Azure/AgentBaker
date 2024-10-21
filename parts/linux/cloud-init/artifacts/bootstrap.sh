#!/bin/bash
set -euxo pipefail
# TODO: propagate exit status to bootstrapper
if [ ! -f /opt/azure/containers/nbc.json ]; then 
    exit 1
fi
mkdir -p /var/log/azure/aks
/opt/azure/containers/nbcparser --bootstrap-config=/opt/azure/containers/nbc.json > /var/log/azure/aks/bootstrap.log 2>&1