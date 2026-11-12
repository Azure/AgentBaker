#!/bin/sh
# Pre-uninstall script to remove gadgets from IG
# This script is executed before the IG package is uninstalled

set -e

log() {
    printf "\n\033[33m[%s] %s\033[0m\n" "$(date +'%Y-%m-%d %T')" "$1"
}

readonly TRACKING_FILE="/var/lib/ig/imported-gadgets.txt"

log "Starting gadget removal process..."

# Check if tracking file exists and has content
if [ ! -f "$TRACKING_FILE" ] || [ ! -s "$TRACKING_FILE" ]; then
    echo "WARNING: No tracking file found or file is empty. No gadgets will be removed."
    exit 0
fi

GADGET_COUNT=0

# Read each imported image name from the tracking file
while IFS= read -r IMPORTED_IMAGE; do
    # Skip empty lines
    [ -z "$IMPORTED_IMAGE" ] && continue

    GADGET_COUNT=$((GADGET_COUNT + 1))
    log "Removing gadget: $IMPORTED_IMAGE"

    # Try to remove the gadget. If it fails, print a warning but continue
    if ! ig image remove "$IMPORTED_IMAGE" > /dev/null; then
        echo "WARNING: Failed to remove $IMPORTED_IMAGE"
    fi
done < "$TRACKING_FILE"

TRACKING_FILE_LINES=$(wc -l < "$TRACKING_FILE")

if [ "$GADGET_COUNT" -ne "$TRACKING_FILE_LINES" ]; then
    echo "WARNING: Processed $GADGET_COUNT gadgets but tracking file has $TRACKING_FILE_LINES lines"
    echo "WARNING: Some lines may have been empty or invalid:"
    cat "$TRACKING_FILE"

    # Clean up tracking file
    rm -f "$TRACKING_FILE"
    exit 1
fi

log "Gadget removal process completed: $GADGET_COUNT/$TRACKING_FILE_LINES gadgets were removed"

# Clean up tracking file
rm -f "$TRACKING_FILE"
