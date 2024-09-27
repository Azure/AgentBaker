#!/bin/bash
set -x

cloud-init status --wait > /dev/null 2>&1
if [ $? -ne 0 ] ; then
    exit 1
fi
echo "cloud-init succeeded";
if [ ! -f /opt/azure/containers/nbc.json ]; then 
    exit 0
fi
/opt/azure/containers/nbcparser /opt/azure/containers/nbc.json > cse_cmd.sh
chmod +x cse_cmd.sh
./cse_cmd.sh