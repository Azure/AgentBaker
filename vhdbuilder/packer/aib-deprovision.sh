#!/bin/bash -eu
# AIB's final deprovision hook. Do not enable tracing: the agent installer uses SAS URLs.

installPinnedWALinuxAgent() {
    local root_dir="${1%/}"
    local ID
    # shellcheck source=/dev/null
    . "${root_dir}/etc/os-release" || return $?

    # Flatcar is excluded by the Packer templates; AIB uses a shared template.
    # The existing installer also preserves OSGuard and Azure Container Linux exclusions.
    if [ "${ID}" = "flatcar" ]; then
        echo "Skipping WALinuxAgent manifest install (ID=flatcar)"
        return 0
    fi
    sudo /bin/bash -eu /opt/azure/containers/post-deprovision-walinuxagent.sh
}

cleanupNvPCRCredentials() {
    local is_cvm="$1"
    local root_dir="${2%/}"
    local ID VERSION_ID credential_dir

    [ "${is_cvm}" = "true" ] || return 0
    # shellcheck source=/dev/null
    . "${root_dir}/etc/os-release" || return $?
    if [ "${ID}" != "ubuntu" ] || [ "${VERSION_ID}" != "26.04" ]; then
        return 0
    fi

    # systemd 259's anchor is bound to the build VM's TPM. Leave SRK, NV indices,
    # disk-encryption keys and unrelated system credentials intact.
    echo "Removing build-VM NvPCR anchor credentials from Ubuntu 26.04 CVM"
    rm -f -- \
        "${root_dir}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" \
        "${root_dir}/run/systemd/nvpcr/nvpcr-anchor.cred" || return $?
    for credential_dir in \
        "${root_dir}/boot/loader/credentials" \
        "${root_dir}/boot/efi/loader/credentials" \
        "${root_dir}/efi/loader/credentials"; do
        [ -d "${credential_dir}" ] || continue
        find "${credential_dir}" -maxdepth 1 \( -type f -o -type l \) \
            -iname 'nvpcr-anchor.*.cred' -print -delete || return $?
    done
    # /run/credentials/@encrypted is a read-only boot import without the .cred
    # suffix. It is RAM-only; removing its boot source above prevents reimport.
}

main() {
    local is_cvm="${1:-}"
    case "${is_cvm}" in
        true|false) ;;
        *)
            echo "Expected CVM build argument: true or false" >&2
            return 1
            ;;
    esac

    # Keep AIB's default generalization, then reapply the same agent pin as Packer.
    sudo waagent -force -deprovision+user || return $?
    export HISTSIZE=0
    installPinnedWALinuxAgent / || return $?
    cleanupNvPCRCredentials "${is_cvm}" / || return $?
    rm -f /opt/azure/containers/post-deprovision-walinuxagent.sh \
        /opt/azure/containers/install_walinuxagent.py \
        /tmp/agentbaker-deprovision.sh /tmp/DeprovisioningScript.sh || return $?
    sync
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
