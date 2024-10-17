#!/bin/bash
set -x
# Check cloud-init status until it is done
while true; do
    status=$(cloud-init status)

    if [[ "$status" == *"done"* ]]; then
        echo "Cloud-init has completed."
        break
    fi
    sleep 1
done
# TODO: propagate exit status to bootstrapper
if [ ! -f /opt/azure/containers/nbc.json ]; then 
    exit 1
fi
mkdir -p /var/log/azure/aks
/opt/azure/containers/nbcparser --bootstrap-config=/opt/azure/containers/nbc.json > /var/log/azure/aks/bootstrap.log 2>&1