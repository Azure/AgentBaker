#!/bin/bash
set -euo pipefail

aclPrefetchPhase() {
    if [[ $# -eq 0 ]]; then
        echo workload
    elif [[ $# -eq 1 && "$1" == --deprovision ]]; then
        echo deprovision
    else
        echo "Usage: acl-prefetch-driver.sh [--deprovision]" >&2
        return 2
    fi
}

runACLPhase() {
    local phase="$1" prefetch="$2" cleanup="$3"
    if [[ "${phase}" == workload ]]; then
        systemctl start containerd || return $?
        /bin/bash "${prefetch}" || return $?
        return 0
    fi
    if ! command -v acl-image-prepare >/dev/null; then
        echo "ACL prefetch requires an ACL base image containing acl-image-prepare (coreos-init >= 0.0.1-5)" >&2
        return 1
    fi
    local waagent=/usr/sbin/waagent
    if command -v waagent >/dev/null; then
        waagent=waagent
    fi
    /bin/bash -eu "${cleanup}" || return $?
    acl-image-prepare || return $?
    "${waagent}" -force -deprovision+user || return $?
    rm -f "${cleanup}" || return $?
    export HISTSIZE=0
    sync
}

main() {
    local phase
    phase="$(aclPrefetchPhase "$@")" || return $?
    local ID VARIANT_ID
    # shellcheck source=/dev/null
    source /etc/os-release
    if [[ "${ID:-}" != azurecontainerlinux ]] &&
        ! [[ "${ID:-}" == azurelinux && "${VARIANT_ID:-}" == azurecontainerlinux ]]; then
        echo "ACL prefetch: refusing to run on a non-ACL image" >&2
        return 1
    fi

    local prefetch=/opt/azure/containers/prefetch.sh
    local cleanup=/opt/azure/containers/cleanup-vhd.sh
    if [ ! -s "${prefetch}" ] ||
        { [[ "${phase}" == deprovision ]] && [ ! -s "${cleanup}" ]; }; then
        echo "ACL prefetch: missing workload script or required deprovision cleanup script" >&2
        return 1
    fi

    runACLPhase "${phase}" "${prefetch}" "${cleanup}" || return $?
    if [[ "${phase}" == deprovision ]]; then
        rm -f /tmp/acl-prefetch-driver.sh || return $?
        sync || return $?
    fi
    printf 'ACL prefetch: completed %s phase\n' "${phase}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    if [[ "${EUID}" -ne 0 ]]; then
        exec sudo -n /bin/bash "${BASH_SOURCE[0]}" "$@"
    fi
    main "$@"
fi
