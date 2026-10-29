#!/bin/bash
set -euo pipefail

verifyAndRearmACLFirstBoot() {
    local boot_dir="${1:-/boot}"
    local template="${boot_dir}/acl/uki-addons/firstboot.addon.efi"
    local uki_name addon

    if [ ! -s "${template}" ]; then
        echo "ACL prefetch: missing firstboot addon template: ${template}" >&2
        return 1
    fi
    uki_name="$(awk '$1 == "default" { print $2 }' "${boot_dir}/loader/loader.conf")" || return 1
    case "${uki_name}" in
        acl.efi|vmlinuz-*.efi) ;;
        *)
            echo "ACL prefetch: unsupported or ambiguous default UKI: ${uki_name}" >&2
            return 1
            ;;
    esac
    if [[ "${uki_name}" == */* ]] || [ ! -f "${boot_dir}/EFI/Linux/${uki_name}" ]; then
        echo "ACL prefetch: default UKI does not exist: ${uki_name}" >&2
        return 1
    fi
    addon="${boot_dir}/EFI/Linux/${uki_name}.extra.d/firstboot.addon.efi"
    # cleanup-vhd.sh owns restoration; verify it restored the UKI actually selected for boot.
    if ! cmp -s "${template}" "${addon}"; then
        echo "ACL prefetch: cleanup did not restore the default UKI's firstboot addon" >&2
        return 1
    fi
    mkdir -p "${boot_dir}/flatcar" || return 1
    touch "${boot_dir}/flatcar/first_boot" || return 1
}

main() {
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
    if [ ! -s "${prefetch}" ] || [ ! -s "${cleanup}" ]; then
        echo "ACL prefetch: both baked prefetch.sh and cleanup-vhd.sh are required" >&2
        return 1
    fi

    systemctl start containerd
    /bin/bash "${prefetch}"
    /bin/bash -eu "${cleanup}"
    verifyAndRearmACLFirstBoot
    rm -f "${prefetch}" "${cleanup}"
    sync
    printf 'ACL prefetch: rearmed first boot after workload; optimizer boot ID: '
    cat /proc/sys/kernel/random/boot_id
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    if [[ "${EUID}" -ne 0 ]]; then
        exec sudo -n /bin/bash "${BASH_SOURCE[0]}" "$@"
    fi
    main "$@"
fi
