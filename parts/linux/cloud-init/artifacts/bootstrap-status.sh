#!/bin/bash

UNIT_NAME="bootstrap.service"

while true; do
    # Check the active state of the unit
    UNIT_STATUS=$(systemctl is-active "$UNIT_NAME")
    
    # Check if the unit has completed
    if [ "$UNIT_STATUS" == "inactive" ] || [ "$UNIT_STATUS" == "failed" ] || [ "$UNIT_STATUS" == "active" ]; then
        echo "Unit has completed with status: $UNIT_STATUS"
        local exit_status=$(systemctl show $UNIT_NAME -p ExecMainStatus --value)
        exit $exit_status
    fi
    
    sleep 3
done
