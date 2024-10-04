#!/bin/bash
set -x
# TODO: propagate exit status to bootstrapper
if [ ! -f /opt/azure/containers/nbc.json ]; then 
    exit 1
fi
/opt/azure/containers/nbcparser /opt/azure/containers/nbc.json > cse_cmd.sh
chmod +x cse_cmd.sh
./cse_cmd.sh