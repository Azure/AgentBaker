#!/bin/sh
# Post-installation script to import gadgets into IG
# This script is executed after the IG package is installed

set -e

log() {
    printf "\n\033[33m[%s] %s\033[0m\n" "$(date +'%Y-%m-%d %T')" "$1"
}

readonly GADGETS_PATH="/usr/share/inspektor-gadget/gadgets"
readonly TRACKING_FILE="/var/lib/ig/imported-gadgets.txt"

log "Starting gadget import process..."

# Check if the gadgets path exists
if [ ! -d "$GADGETS_PATH" ]; then
    echo "ERROR: Gadgets path $GADGETS_PATH does not exist"
    exit 1
fi

# Ensure the tracking file directory exists
TRACKING_DIR="$(dirname "$TRACKING_FILE")"
if ! mkdir -p "$TRACKING_DIR"; then
    echo "ERROR: Failed to create tracking directory $TRACKING_DIR"
    exit 1
fi

# Clear previous tracking file if it exists
true > "$TRACKING_FILE"

# Import gadgets
GADGET_COUNT=0
for GADGET_PATH in "$GADGETS_PATH"/*; do
    # Skip if not a regular file (directories, symlinks, etc.)
    [ ! -f "$GADGET_PATH" ] && continue

    GADGET_COUNT=$((GADGET_COUNT + 1))
    GADGET_NAME=$(basename "$GADGET_PATH")
    log "Importing gadget $GADGET_NAME..."

    # Capture the import output to get the actual image name
    IMPORT_OUTPUT=$(ig image import "$GADGET_PATH" 2>&1 || echo "")

    if [ -n "$IMPORT_OUTPUT" ] && echo "$IMPORT_OUTPUT" | grep -q "Successfully imported images"; then
        # Extract the actual imported image name from the output
        # IG typically outputs looks like this:
        # Successfully imported images:
        #   mcr.microsoft.com/oss/v2/inspektor-gadget/gadget/top_process:v1.2.3
        IMPORTED_IMAGE=$(echo "$IMPORT_OUTPUT" | grep "$GADGET_NAME" | awk '{print $NF}')

        if [ -n "$IMPORTED_IMAGE" ]; then
            echo "$IMPORTED_IMAGE" >> "$TRACKING_FILE"
            log "Tracked imported image: $IMPORTED_IMAGE"
        else
            echo "WARNING: Couldn't extract image name from import output for $GADGET_NAME"
        fi
    else
        echo "WARNING: Failed to import $GADGET_NAME"
    fi
done

# Count total files in gadgets directory for comparison
TOTAL_FILES=$(find "$GADGETS_PATH" -maxdepth 1 -type f | wc -l)

if [ "$GADGET_COUNT" -ne "$TOTAL_FILES" ]; then
    echo "WARNING: Processed $GADGET_COUNT gadgets but found $TOTAL_FILES files in $GADGETS_PATH"
    echo "WARNING: Some files in $GADGETS_PATH may not be regular files or were skipped:"
    ls -la "$GADGETS_PATH"
    exit 1
fi

log "Gadget import process completed: $GADGET_COUNT/$TOTAL_FILES gadgets were imported"
