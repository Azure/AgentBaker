#!/bin/sh

ROOT_ARGUMENT=$(getarg root=)
if [ -z "$ROOT_ARGUMENT" ]; then
    ROOT_ARGUMENT=$(awk '{
        for (i = 1; i <= NF; i++) {
            if ($i ~ /^root=/) {
                sub(/^root=/, "", $i)
                print $i
                exit
            }
        }
    }' /proc/cmdline)
fi

if ! /sbin/aks-root-journal-resize "$ROOT_ARGUMENT"; then
    die "AKS root journal sizing failed; refusing to mount the root filesystem"
fi
