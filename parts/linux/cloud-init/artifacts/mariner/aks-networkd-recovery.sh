#!/bin/bash

recover_networkd_link() {
    local ifindex state

    if ! ifindex=$(cat /sys/class/net/eth0/ifindex); then
        echo "ERROR: Cannot find eth0; not attempting networkd recovery." >&2
        return 0
    fi
    if ! state=$(busctl --system --auto-start=no --timeout=5s get-property \
        org.freedesktop.network1 "/org/freedesktop/network1/link/_${ifindex}" \
        org.freedesktop.network1.Link AdministrativeState); then
        echo "ERROR: Cannot query eth0 administrative state; not attempting recovery." >&2
        return 0
    fi
    case "$state" in
        's "failed"') ;;
        's "pending"'|'s "initialized"'|'s "configuring"'|'s "configured"'|'s "unmanaged"'|'s "linger"')
            return 0
            ;;
        *)
            echo "ERROR: Unexpected eth0 administrative state: $state" >&2
            return 0
            ;;
    esac

    echo "eth0 is failed; requesting networkd reconfiguration (index $ifindex)."
    if ! timeout 20s networkctl reconfigure "$ifindex"; then
        echo "ERROR: eth0 reconfiguration request failed or timed out." >&2
        return 0
    fi
    echo "eth0 reconfiguration requested; connectivity recovery is not yet confirmed."
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    set -euo pipefail
    recover_networkd_link
fi
