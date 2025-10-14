#!/bin/bash

# This is a wrapper script for the check_egress.sh script
# It executes the check_egress.sh script with a shorter timeout than NPD
# and ensures it always exits with status code 0 to prevent unwanted events

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
FULL_SCRIPT_PATH="${SCRIPT_DIR}/check_egress.sh"
JSON_CONFIG_PATH="${SCRIPT_DIR}/../custom-plugin-monitor/custom-egress-monitor.json"

# Default timeout value in case of any issues reading monitor
DEFAULT_TIMEOUT_SECONDS=115

if [ ! -f "$FULL_SCRIPT_PATH" ]; then
    echo "ERROR: Check script not found: $FULL_SCRIPT_PATH" >&2
    exit 0  # Still exit with 0 to avoid NPD errors
fi

if [ ! -f "$JSON_CONFIG_PATH" ]; then
    echo "Warning: JSON config not found: $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
else
    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        echo "Warning: jq is not installed. Cannot parse JSON config. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
    else
        NPD_TIMEOUT_STRING=$(jq -r '.pluginConfig.timeout' "$JSON_CONFIG_PATH")

        if [ -z "$NPD_TIMEOUT_STRING" ] || [ "$NPD_TIMEOUT_STRING" == "null" ]; then
            echo "Warning: Could not read timeout from $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS" >&2
        else
            # Remove 's' suffix and convert to integer
            NPD_TIMEOUT_SECONDS=${NPD_TIMEOUT_STRING%s}
            if ! [[ "$NPD_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
                echo "Warning: Invalid timeout value '$NPD_TIMEOUT_STRING' in $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
            fi
        fi
    fi
fi

# If NPD_TIMEOUT_SECONDS is not set, use the default timeout
if [ -z "$NPD_TIMEOUT_SECONDS" ]; then
    SCRIPT_TIMEOUT_SECONDS=$DEFAULT_TIMEOUT_SECONDS
else
    # Subtract 5 seconds from NPD timeout for the actual script timeout
    SCRIPT_TIMEOUT_SECONDS=$((NPD_TIMEOUT_SECONDS - 5))
fi

# Run with shorter timeout than the NPD timeout
timeout "${SCRIPT_TIMEOUT_SECONDS}s" "$FULL_SCRIPT_PATH"
RETURN_CODE=$?

# Check for timeout - return code 124 indicates a timeout
if [ $RETURN_CODE -eq 124 ]; then
    echo "check_egress.sh timed out (return code 124) after ${SCRIPT_TIMEOUT_SECONDS}s. Still exiting with 0." >&2
    exit 0
fi

# retain original exit code so we still raise an event if the checks fail
exit $RETURN_CODE