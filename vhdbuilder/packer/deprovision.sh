#!/bin/bash -eu
# Shared Packer cleanup helpers and AIB's final deprovision hook.
# Do not enable tracing: the agent installer uses SAS URLs.

# waagent stops at its first failed cleanup action but still exits 0, which can
# leave bake-time agent state, logs and the build user in the image.
deprovisionWALinuxAgent() {
    local lib_dir="${1:-/var/lib/waagent}"
    local unit output status=0

    # Azure Policy installs the Guest Configuration extension on build VMs. Its gcd.service runs
    # outside the extension slices and its workers write to /var/lib/waagent/events. The unit uses
    # KillMode=process, so a stop leaves the workers alive: kill the whole unit while it is active,
    # then stop it before Restart= fires. Disable it, because deprovision deletes its ExecStart.
    sudo systemctl disable gcd.service >/dev/null 2>&1 || true
    sudo systemctl kill --signal=SIGKILL gcd.service >/dev/null 2>&1 || true

    # waagent stops only its own service. Its log collector and extensions run in
    # separate units and can write under /var/lib/waagent while waagent deletes it.
    for unit in gcd.service walinuxagent.service waagent.service \
        azure-walinuxagent-logcollector.slice azure-vmextensions.slice; do
        sudo systemctl stop "${unit}" >/dev/null 2>&1 || true
    done

    # The install path differs by distro (/usr/sbin on Ubuntu, /usr/bin on Azure Linux and ACL).
    output=$(sudo waagent -force -deprovision+user 2>&1) || status=$?
    printf '%s\n' "${output}"
    if [ "${status}" -ne 0 ]; then
        return "${status}"
    fi
    # Do not retry: a partial run can delete ovf-env.xml, and waagent then skips build-user deletion.
    if printf '%s\n' "${output}" | grep -qF "Failed to run 'deprovision"; then
        echo "waagent deprovision failed" >&2
        return 1
    fi
    if [ -n "$(sudo ls -A "${lib_dir}" 2>/dev/null)" ]; then
        echo "waagent deprovision left files in ${lib_dir}" >&2
        return 1
    fi
}

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
    deprovisionWALinuxAgent || return $?
    export HISTSIZE=0
    installPinnedWALinuxAgent / || return $?
    cleanupNvPCRCredentials "${is_cvm}" / || return $?
    rm -f /opt/azure/containers/post-deprovision-walinuxagent.sh \
        /opt/azure/containers/install_walinuxagent.py \
        /opt/azure/containers/deprovision.sh \
        /opt/azure/containers/aib-deprovision.sh \
        /tmp/DeprovisioningScript.sh || return $?
    sync
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
