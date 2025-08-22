#!/bin/bash

# Script to update KUBELET_NODE_LABELS in /etc/default/kubelet
# This script modifies the kubelet configuration file in-place to correct node labels
# that may have been overwritten by custom data processes.

set -euo pipefail

KUBELET_DEFAULT_FILE="/etc/default/kubelet"
BACKUP_FILE="/etc/default/kubelet.backup"

# Function to log messages
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [update-kubelet-node-labels] $*" >&2
}

# Check if the kubelet default file exists
if [ ! -f "$KUBELET_DEFAULT_FILE" ]; then
    log "ERROR: $KUBELET_DEFAULT_FILE does not exist"
    exit 1
fi

# Create a backup of the original file if it doesn't exist
if [ ! -f "$BACKUP_FILE" ]; then
    cp "$KUBELET_DEFAULT_FILE" "$BACKUP_FILE"
    log "Created backup at $BACKUP_FILE"
fi

# Extract KUBELET_NODE_LABELS from the file
KUBELET_NODE_LABELS=$(grep "^KUBELET_NODE_LABELS=" "$KUBELET_DEFAULT_FILE" | cut -d'"' -f2)

log "Current KUBELET_NODE_LABELS: $KUBELET_NODE_LABELS"

# Process the labels if they exist
if [ -n "$KUBELET_NODE_LABELS" ]; then
    NEW_LABELS=""

    # Split by comma and process each label
    OLD_IFS="$IFS"
    IFS=','
    for label in $KUBELET_NODE_LABELS; do
        IFS="$OLD_IFS"

        # Skip empty labels
        if [ -n "$label" ]; then
            # Split by equal sign to get key and value
            key="${label%%=*}"
            value="${label#*=}"

            # Truncate value if longer than 63 characters
            if [ ${#value} -gt 63 ]; then
                value="${value:0:63}"
                log "WARNING: Truncated value for key '$key' to 63 characters"
            fi

            # Reassemble the label
            processed_label="${key}=${value}"

            # Add to new labels list
            if [ -n "$NEW_LABELS" ]; then
                NEW_LABELS="${NEW_LABELS},${processed_label}"
            else
                NEW_LABELS="$processed_label"
            fi
        fi
        IFS=','
    done
    IFS="$OLD_IFS"

    KUBELET_NODE_LABELS="$NEW_LABELS"
fi

log "Final KUBELET_NODE_LABELS: $KUBELET_NODE_LABELS"

TEMP_FILE=$(mktemp)
trap 'rm -f "$TEMP_FILE"' EXIT

while IFS= read -r line; do
    case "$line" in
        KUBELET_NODE_LABELS=*)
            echo "KUBELET_NODE_LABELS=\"$KUBELET_NODE_LABELS\""
            ;;
        *)
            echo "$line"
            ;;
    esac
done < "$KUBELET_DEFAULT_FILE" > "$TEMP_FILE"

mv "$TEMP_FILE" "$KUBELET_DEFAULT_FILE"

chmod 644 "$KUBELET_DEFAULT_FILE"

log "Successfully updated $KUBELET_DEFAULT_FILE"
