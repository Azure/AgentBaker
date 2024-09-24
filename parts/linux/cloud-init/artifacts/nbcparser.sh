#!/bin/bash
set -x

/opt/azure/containers/nbcparser /opt/azure/containers/nbc.json > cse_cmd.sh
chmod +x cse_cmd.sh
./cse_cmd.sh