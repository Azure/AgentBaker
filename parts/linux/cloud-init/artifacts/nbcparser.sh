#!/bin/bash
set -x
if [ ! -f /opt/azure/containers/nbc.json ]; then 
    exit 0
fi
/opt/azure/containers/nbcparser /opt/azure/containers/nbc.json > cse_cmd.sh
chmod +x cse_cmd.sh
./cse_cmd.sh