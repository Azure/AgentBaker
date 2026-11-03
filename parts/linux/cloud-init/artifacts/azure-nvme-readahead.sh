#!/bin/sh

# Namespace rescans can restore 64 MiB without a udev event for existing disks.
status=0
for device in /sys/block/nvme*n*; do
    [ -d "$device" ] || continue
    if ! read -r model < "$device/device/model"; then
        [ -d "$device" ] || continue
        echo "Cannot read NVMe model for $device" >&2
        status=1
        continue
    fi
    [ "$model" = "MSFT NVMe Accelerator v1.0" ] || continue

    queue="$device/queue/read_ahead_kb"
    if ! read -r current < "$queue"; then
        [ -d "$device" ] || continue
        echo "Cannot read NVMe read-ahead for $device" >&2
        status=1
        continue
    fi
    # Preserve other defaults and workload-specific tuning.
    [ "$current" = 65536 ] || continue
    if ! printf '%s\n' 256 > "$queue"; then
        [ -d "$device" ] || continue
        echo "Cannot set NVMe read-ahead for $device" >&2
        status=1
        continue
    fi
    if ! read -r current < "$queue" || [ "$current" != 256 ]; then
        [ -d "$device" ] || continue
        echo "NVMe read-ahead verification failed for $device" >&2
        status=1
        continue
    fi
    echo "NVMe read-ahead: $device 65536 -> 256 KiB"
done
exit "$status"
