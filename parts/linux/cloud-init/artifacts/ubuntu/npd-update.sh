#!/usr/bin/env bash

# NPD's handler for the RP npdConfig / ubuntuPackageVersions contract.
# The generic updater owns the goal/checkpoint/status; dpkg owns package state.
: "${NPD_OS_RELEASE_FILE:=/etc/os-release}"
: "${NPD_SKIP_FILE:=/etc/node-problem-detector.d/skip_vhd_npd}"
: "${NPD_PACKAGE_CACHE:=/opt/node-problem-detector/downloads}"
: "${NPD_UPDATE_DIR:=/var/lib/aks/npd-update}"

knead_emit_npd_config_event() {
    if declare -F knead_emit_event >/dev/null; then
        knead_emit_event "AKS.LivePatching.npdConfig.$1" "$2" "${3:-Informational}" || true
    fi
}

npd_fail() {
    echo "npdConfig failed: $1"
    knead_emit_npd_config_event Failed "reason=$1" Error
    return 1
}

npd_ubuntu_release() (
    # shellcheck disable=SC1090
    source "${NPD_OS_RELEASE_FILE}" || return 1
    [ "${ID:-}" = ubuntu ] || return 1
    printf '%s' "${VERSION_ID}"
)

npd_installed_version() {
    local status
    status="$(dpkg-query -W -f='${db:Status-Status}' node-problem-detector-aks-config 2>/dev/null)" || return 1
    [ "${status}" = installed ] || return 1
    dpkg-query -W -f='${Version}' node-problem-detector-aks-config
}

npd_desired_version() {
    local payload="$1" release="$2" version
    version="$(printf '%s' "${payload}" | jq -er --arg release "${release}" '
        if type != "object" then error("invalid NPD config")
        elif .ubuntuPackageVersions == null then ""
        elif (.ubuntuPackageVersions | type) != "object" then error("invalid version map")
        elif .ubuntuPackageVersions[$release] == null then ""
        elif (.ubuntuPackageVersions[$release] | type) != "string" then error("invalid version")
        else .ubuntuPackageVersions[$release] end')" || return 1
    if [ -n "${version}" ]; then
        [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+-ubuntu([0-9]{2}\.[0-9]{2})u[1-9][0-9]*$ ]] || return 1
        [ "${BASH_REMATCH[1]}" = "${release}" ] && [ "${#version}" -le 128 ] || return 1
    fi
    printf '%s' "${version}"
}

# A pending transaction must recover even if dpkg already records the new version.
npdConfigsIsCurrent() {
    local desired="$1" current="$2" release version installed
    [ ! -f "${NPD_UPDATE_DIR}/pending" ] || return 1
    [ -f "${NPD_SKIP_FILE}" ] || return 1
    release="$(npd_ubuntu_release)" || return 1
    version="$(npd_desired_version "${desired}" "${release}")" || return 1
    [ -n "${version}" ] || return 1
    [ "${version}" = "$(npd_desired_version "${current}" "${release}")" ] || return 1
    installed="$(npd_installed_version)" || return 1
    dpkg --compare-versions "${installed}" ge "${version}"
}

npd_find_cached_package() {
    local version="$1" package
    for package in "${NPD_PACKAGE_CACHE}"/*.deb; do
        [ -f "${package}" ] || continue
        if [ "$(dpkg-deb -f "${package}" Package)" = node-problem-detector-aks-config ] &&
           [ "$(dpkg-deb -f "${package}" Version)" = "${version}" ] &&
           [ "$(dpkg-deb -f "${package}" Architecture)" = "$(dpkg --print-architecture)" ]; then
            printf '%s' "${package}"
            return 0
        fi
    done
    return 1
}

npd_refresh_metadata() {
    apt-get -o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
        -o APT::Update::Error-Mode=any update
}

npd_download_package() {
    local version="$1"
    (cd "${NPD_PACKAGE_CACHE}" && apt-get -o Acquire::Retries=3 \
        -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
        download "node-problem-detector-aks-config=${version}")
}

npd_validate_package() {
    local package="$1" stage file
    stage="$(mktemp -d "${NPD_UPDATE_DIR}/validate.XXXXXX")" || return 1
    if ! dpkg-deb -x "${package}" "${stage}"; then
        rm -rf "${stage}"
        return 1
    fi
    local result=0
    test -x "${stage}/opt/bin/node-problem-detector-startup.sh" || result=1
    test -s "${stage}/etc/systemd/system/node-problem-detector.service" || result=1
    test -s "${stage}/etc/node-problem-detector.d/custom-plugin-monitor/custom-kubelet-monitor.json" || result=1
    while IFS= read -r -d '' file; do
        case "${file}" in
            *.json) jq -e 'type == "object"' "${file}" >/dev/null || result=1 ;;
            *.sh) bash -n "${file}" || result=1 ;;
        esac
    done < <(find "${stage}" -type f -print0)
    rm -rf "${stage}"
    return "${result}"
}

npd_install_package() {
    local package="$1"
    # The config package is held against unattended updates; this exact local
    # transaction is authorized to change it, including restoring the old deb.
    DEBIAN_FRONTEND=noninteractive apt-get -y --no-remove --no-install-recommends \
        --allow-change-held-packages --allow-downgrades --reinstall \
        -o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
        -o DPkg::Lock::Timeout=120 -o Dpkg::Options::=--force-confnew \
        -o Dpkg::Options::=--force-confmiss install "${package}" || return 1
    apt-mark hold node-problem-detector-aks-config || return 1
    # dpkg retains removed conffiles. NPD discovers monitor JSONs by directory,
    # so obsolete package-owned monitors must be removed on upgrades AND rollback.
    # Never delete unrelated files such as the extension ownership marker.
    local conffiles path _checksum obsolete
    conffiles="$(dpkg-query -W -f='${Conffiles}' node-problem-detector-aks-config)" || return 1
    while read -r path _checksum obsolete; do
        if [ "${obsolete}" = obsolete ]; then
            case "${path}" in
                /etc/node-problem-detector.d/skip_vhd_npd) return 1 ;;
                /etc/node-problem-detector.d/*) rm -f -- "${path}" || return 1 ;;
            esac
        fi
    done <<< "${conffiles}"
}

npd_restart_and_check() {
    systemctl daemon-reload || return 1
    systemctl restart node-problem-detector.service || return 1
    local attempt
    for ((attempt=0; attempt<30; attempt++)); do
        if systemctl is-active --quiet node-problem-detector.service &&
           curl --noproxy '*' -fsS --max-time 2 http://127.0.0.1:20256/healthz >/dev/null; then
            return 0
        fi
        sleep 2
    done
    return 1
}

# The durable pending file records the version to restore, not a successful goal.
# Reinstalling that cached deb repairs both package-manager state and live files.
npd_recover_pending() {
    local old_version package
    [ -f "${NPD_UPDATE_DIR}/pending" ] || return 0
    old_version="$(cat "${NPD_UPDATE_DIR}/pending")" || return 1
    package="$(npd_find_cached_package "${old_version}")" || return 1
    npd_install_package "${package}" && npd_restart_and_check || return 1
    rm -f "${NPD_UPDATE_DIR}/pending"
}

updateNPDConfigs() (
    # securityPatch is sourced into the same reconciler and exports APT_CONFIG.
    # Keep its snapshot-only repositories out of NPD package operations.
    unset APT_CONFIG
    local payload="$1" release desired installed package
    if [ ! -f "${NPD_SKIP_FILE}" ] || ! release="$(npd_ubuntu_release)"; then
        echo "npdConfig: no action for an unsupported or extension-managed node"
        knead_emit_npd_config_event NoAction 'Node is not Ubuntu with baked NPD'
        return 0
    fi
    # Require an installed config package for normal onboarding; a pending apt
    # transaction may temporarily leave dpkg in a partially configured state.
    if [ ! -f "${NPD_UPDATE_DIR}/pending" ] && ! npd_installed_version >/dev/null; then
        echo "npdConfig: no AgentBaker config package; no action"
        return 0
    fi
    mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}" || { npd_fail WorkDirectory; return 1; }
    npd_recover_pending || { npd_fail RecoveryFailed; return 1; }
    desired="$(npd_desired_version "${payload}" "${release}")" || { npd_fail InvalidConfig; return 1; }
    if [ -z "${desired}" ]; then
        echo "npdConfig: no package selected for Ubuntu ${release}; no action"
        knead_emit_npd_config_event NoAction "No package selected for Ubuntu ${release}"
        return 0
    fi
    installed="$(npd_installed_version)" || { npd_fail InstalledVersion; return 1; }
    if dpkg --compare-versions "${installed}" ge "${desired}"; then
        echo "npdConfig: installed ${installed} satisfies ${desired}; no action"
        knead_emit_npd_config_event NoAction "Installed ${installed} satisfies ${desired}"
        return 0
    fi
    # The old package is cached during baking or the previous successful update.
    npd_find_cached_package "${installed}" >/dev/null || { npd_fail RecoveryPackageMissing; return 1; }
    if ! package="$(npd_find_cached_package "${desired}")"; then
        if ! npd_refresh_metadata || ! npd_download_package "${desired}"; then
            npd_fail PackageDownload
            return 1
        fi
        package="$(npd_find_cached_package "${desired}")" || { npd_fail PackageIdentity; return 1; }
    fi
    npd_validate_package "${package}" || { npd_fail PackageValidation; return 1; }
    if ! printf '%s\n' "${installed}" > "${NPD_UPDATE_DIR}/pending.tmp" ||
       ! mv "${NPD_UPDATE_DIR}/pending.tmp" "${NPD_UPDATE_DIR}/pending"; then
        npd_fail TransactionState
        return 1
    fi
    if ! npd_install_package "${package}" || ! npd_restart_and_check; then
        npd_recover_pending || knead_emit_npd_config_event Failed 'reason=RecoveryFailed' Error
        npd_fail PackageActivation
        return 1
    fi
    rm -f "${NPD_UPDATE_DIR}/pending" || { npd_fail TransactionState; return 1; }
    echo "npdConfig updated successfully: ${desired}"
    knead_emit_npd_config_event Succeeded "Installed ${desired}"
)
