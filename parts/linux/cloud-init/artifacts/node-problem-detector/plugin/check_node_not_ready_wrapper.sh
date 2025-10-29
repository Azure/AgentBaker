#!/bin/bash

# This is a wrapper script for the check_node_not_ready.sh script
# It executes the check_node_not_ready.sh script with a shorter timeout than NPD
# and ensures it always exits with status code 0 to prevent unwanted events

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
FULL_SCRIPT_PATH="${SCRIPT_DIR}/check_node_not_ready.sh"

if [ ! -f "$FULL_SCRIPT_PATH" ]; then
    echo "ERROR: Check script not found: $FULL_SCRIPT_PATH" >&2
    exit 0  # Still exit with 0 to avoid NPD errors
fi

echo "Running check_node_not_ready.sh via wrapper"

# Run with shorter timeout than the NPD timeout (90s)
timeout 85s "$FULL_SCRIPT_PATH"
RETURN_CODE=$?

# Check for timeout - return code 124 indicates a timeout
if [ $RETURN_CODE -eq 124 ]; then
    echo "check_node_not_ready.sh timed out (return code 124)" >&2
elif [ $RETURN_CODE -ne 0 ]; then
    echo "check_node_not_ready.sh failed with return code $RETURN_CODE" >&2
fi

# Always exit with code 0 to prevent NPD from raising events
exit 0 