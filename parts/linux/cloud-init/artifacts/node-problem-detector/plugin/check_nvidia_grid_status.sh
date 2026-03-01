#!/bin/bash

# This plugin checks if Nvidia GRID License is valid.
set -euo pipefail

readonly OK=0
readonly NOTOK=1

license_status=$(sudo nvidia-smi -q | grep 'License Status' | grep 'Licensed' || true)

if [ -z "$license_status" ]; then
    echo "License status not valid or not found"
    exit  $NOTOK
else
    echo "License status is valid"
fi

active_status=$(sudo systemctl is-active nvidia-gridd || true)
if [ "$active_status" != "active" ]; then 
    echo "nvidia-gridd is not active: $active_status"
    exit  $NOTOK
else
    echo "nvidia-gridd is active"
fi

exit $OK