#!/usr/bin/env bash
# Entry points set NPD_COMPONENT in a subshell on purpose so components stay isolated.
# shellcheck disable=SC2030,SC2031

# NPD's handlers for the RP npdConfig and npdBinary contracts on Ubuntu and Azure
# Linux 3. npdConfig updates node-problem-detector-aks-config and npdBinary updates
# node-problem-detector-kubernetes. They are independent components: each has its
# own package, transaction marker, cached rollback package and status, and one
# never moves the other's package. The generic updaters own the
# goal/checkpoint/status; dpkg or rpm owns package state. Only the package
# primitives differ by OS; the transaction is shared.
: "${NPD_OS_RELEASE_FILE:=/etc/os-release}"
: "${NPD_SKIP_FILE:=/etc/node-problem-detector.d/skip_vhd_npd}"
: "${NPD_PACKAGE_CACHE:=/opt/node-problem-detector/downloads}"
: "${NPD_UPDATE_DIR:=/var/lib/aks/npd-update}"
: "${NPD_TDNF_CONFIG:=/etc/tdnf/tdnf.conf}"
: "${NPD_TDNF_LOCK_FILE:=/etc/tdnf/locks.d/node-problem-detector.conf}"
: "${NPD_CONFIG_DIR:=/etc/node-problem-detector.d}"
: "${NPD_UNIT_FILE:=/etc/systemd/system/node-problem-detector.service}"
: "${NPD_REPORT_ATTEMPTS:=24}"

# The component being processed. The update and comparator entry points set it in
# a subshell, so it never leaks between components.
NPD_COMPONENT=npdConfig

npd_package_name() {
    case "${NPD_COMPONENT}" in
        npdBinary) printf node-problem-detector-kubernetes ;;
        *) printf node-problem-detector-aks-config ;;
    esac
}

# Durable per-component transaction marker recording the version to restore.
npd_pending_file() {
    case "${NPD_COMPONENT}" in
        npdBinary) printf '%s' "${NPD_UPDATE_DIR}/pending-binary" ;;
        *) printf '%s' "${NPD_UPDATE_DIR}/pending" ;;
    esac
}

knead_emit_npd_config_event() {
    if declare -F knead_emit_event >/dev/null; then
        knead_emit_event "AKS.LivePatching.${NPD_COMPONENT}.$1" "$2" "${3:-Informational}" || true
    fi
}

npd_fail() {
    echo "${NPD_COMPONENT} failed: $1"
    knead_emit_npd_config_event Failed "reason=$1" Error
    return 1
}

# Prints "<ubuntu|azurelinux> <VERSION_ID>" for OS families that can own baked NPD.
npd_platform() (
    # shellcheck disable=SC1090
    source "${NPD_OS_RELEASE_FILE}" || return 1
    case "${ID:-}" in
        ubuntu) ;;
        # Azure Container Linux reports ID=azurelinux but is not RPM-managed.
        azurelinux) [ "${VARIANT_ID:-}" != azurecontainerlinux ] || return 1 ;;
        *) return 1 ;;
    esac
    [ -n "${VERSION_ID:-}" ] || return 1
    printf '%s %s' "${ID}" "${VERSION_ID}"
)

npd_package_format() {
    case "$(npd_platform)" in
        ubuntu\ *) printf deb ;;
        azurelinux\ *) printf rpm ;;
        *) return 1 ;;
    esac
}

npd_installed_version() {
    local status
    case "$(npd_package_format)" in
        deb)
            status="$(dpkg-query -W -f='${db:Status-Status}' "$(npd_package_name)" 2>/dev/null)" || return 1
            [ "${status}" = installed ] || return 1
            dpkg-query -W -f='${Version}' "$(npd_package_name)"
            ;;
        rpm) rpm -q --qf '%{VERSION}-%{RELEASE}' "$(npd_package_name)" 2>/dev/null ;;
        *) return 1 ;;
    esac
}

# Returns success when installed version $1 is at least desired version $2.
npd_version_ge() {
    local result
    case "$(npd_package_format)" in
        deb) dpkg --compare-versions "$1" ge "$2" ;;
        rpm)
            result="$(rpm --define "_npd_installed $1" --define "_npd_desired $2" \
                --eval '%{lua: print(rpm.vercmp(rpm.expand("%{_npd_installed}"), rpm.expand("%{_npd_desired}")))}')" || return 1
            [ "${result}" -ge 0 ] 2>/dev/null
            ;;
        *) return 1 ;;
    esac
}

# The config package content is the same on every release. Ubuntu releases
# without their own config build use another release's package: they read that
# release's payload entry and download from its repository. Prints
# "<release> <codename>" of the source, or nothing when the node has its own build.
npd_ubuntu_config_source() {
    case "$1" in
        26.04) printf '24.04 noble' ;;
    esac
}

# Prints the release whose payload entry and package version this node follows
# for the current component. Only the config package is shared across releases;
# every release has its own binary build.
npd_component_release() {
    if [ "${NPD_COMPONENT}" = npdConfig ]; then
        npd_config_release "$@"
    else
        printf '%s' "$2"
    fi
}

# Prints the release whose config payload entry and package version this node follows.
npd_config_release() {
    local distro="$1" release="$2" source
    if [ "${distro}" = ubuntu ]; then
        source="$(npd_ubuntu_config_source "${release}")"
        [ -z "${source}" ] || release="${source%% *}"
    fi
    printf '%s' "${release}"
}

# Prints the version selected for this node, or nothing when it is not targeted.
# Ubuntu: ubuntuPackageVersions["22.04"] = "1.0.0-ubuntu22.04u1".
# Azure Linux: azureLinuxPackageVersions["3.0"] = "1.0.0-1.azl3".
npd_desired_version() {
    local payload="$1" distro="$2" release="$3" key pattern version
    case "${distro}" in
        ubuntu)
            key=ubuntuPackageVersions
            pattern='^[0-9]+\.[0-9]+\.[0-9]+-ubuntu[0-9]{2}\.[0-9]{2}u[1-9][0-9]*\z'
            ;;
        azurelinux)
            key=azureLinuxPackageVersions
            pattern='^[0-9]+\.[0-9]+\.[0-9]+-[1-9][0-9]*\.azl[0-9]+\z'
            ;;
        *) return 1 ;;
    esac
    # Validate the JSON string before Bash command substitution can drop trailing newlines.
    version="$(printf '%s' "${payload}" | jq -er --arg key "${key}" --arg release "${release}" --arg pattern "${pattern}" '
        if type != "object" then error("invalid NPD config")
        elif .[$key] == null then ""
        elif (.[$key] | type) != "object" then error("invalid version map")
        elif .[$key][$release] == null then ""
        elif (.[$key][$release] | type) != "string" then error("invalid version")
        elif (.[$key][$release] | test($pattern) | not) then error("invalid version")
        else .[$key][$release] end')" || return 1
    if [ -n "${version}" ]; then
        [ "${#version}" -le 128 ] || return 1
        # The package release must match the node release that selected it.
        case "${distro}" in
            ubuntu)
                [[ "${version}" =~ -ubuntu([0-9]{2}\.[0-9]{2})u[1-9][0-9]*$ ]] || return 1
                [ "${BASH_REMATCH[1]}" = "${release}" ] || return 1
                ;;
            azurelinux)
                [[ "${version}" =~ \.azl([0-9]+)$ ]] || return 1
                [ "${BASH_REMATCH[1]}.0" = "${release}" ] || return 1
                ;;
        esac
    fi
    printf '%s' "${version}"
}

# A pending transaction must recover even if the package manager already records the new version.
npd_component_is_current() {
    local desired="$1" current="$2" platform distro release version installed
    [ ! -f "$(npd_pending_file)" ] || return 1
    [ -f "${NPD_SKIP_FILE}" ] || return 1
    platform="$(npd_platform)" || return 1
    read -r distro release <<< "${platform}"
    release="$(npd_component_release "${distro}" "${release}")"
    version="$(npd_desired_version "${desired}" "${distro}" "${release}")" || return 1
    [ -n "${version}" ] || return 1
    [ "${version}" = "$(npd_desired_version "${current}" "${distro}" "${release}")" ] || return 1
    installed="$(npd_installed_version)" || return 1
    npd_version_ge "${installed}" "${version}"
}

npdConfigsIsCurrent() (
    NPD_COMPONENT=npdConfig
    npd_component_is_current "$@"
)

npdBinaryIsCurrent() (
    NPD_COMPONENT=npdBinary
    npd_component_is_current "$@"
)

npd_package_fields() {
    case "$(npd_package_format)" in
        deb) printf '%s\t%s\t%s' "$(dpkg-deb -f "$1" Package)" "$(dpkg-deb -f "$1" Version)" "$(dpkg-deb -f "$1" Architecture)" ;;
        rpm) rpm -qp --qf '%{NAME}\t%{VERSION}-%{RELEASE}\t%{ARCH}' "$1" 2>/dev/null ;;
        *) return 1 ;;
    esac
}

npd_native_arch() {
    case "$(npd_package_format)" in
        deb) dpkg --print-architecture ;;
        rpm) uname -m ;;
        *) return 1 ;;
    esac
}

npd_find_cached_package() {
    local version="$1" format arch package
    format="$(npd_package_format)" || return 1
    arch="$(npd_native_arch)" || return 1
    for package in "${NPD_PACKAGE_CACHE}"/*."${format}"; do
        [ -f "${package}" ] || continue
        if [ "$(npd_package_fields "${package}")" = "$(printf '%s\t%s\t%s' "$(npd_package_name)" "${version}" "${arch}")" ]; then
            printf '%s' "${package}"
            return 0
        fi
    done
    return 1
}

# The tdnf lock holds NPD against security patching and other updates. tdnf
# reads locks.d beside its configuration, so downloads use an otherwise
# identical copy; installation goes through rpm, which does not read tdnf locks.
npd_tdnf() {
    local config_dir="${NPD_UPDATE_DIR}/tdnf"
    mkdir -p "${config_dir}" || return 1
    cp "${NPD_TDNF_CONFIG}" "${config_dir}/tdnf.conf" || return 1
    tdnf -c "${config_dir}/tdnf.conf" "$@"
}

# Prints a private apt configuration that reads only the config source release's
# Microsoft repository, or nothing when the node has its own config build. System
# apt sources, lists and caches are untouched; packages are still signature-checked
# against the keys apt already trusts.
npd_config_apt_conf() {
    local platform source source_release codename dir
    [ "${NPD_COMPONENT}" = npdConfig ] || return 0
    platform="$(npd_platform)" || return 1
    [ "${platform%% *}" = ubuntu ] || return 0
    source="$(npd_ubuntu_config_source "${platform#* }")"
    [ -n "${source}" ] || return 0
    read -r source_release codename <<< "${source}"
    dir="${NPD_UPDATE_DIR}/apt"
    mkdir -p "${dir}/lists/partial" "${dir}/cache/archives/partial" || return 1
    printf 'deb [arch=amd64,arm64] https://packages.microsoft.com/ubuntu/%s/prod %s main\n' \
        "${source_release}" "${codename}" > "${dir}/sources.list" || return 1
    printf 'Dir::Etc::SourceList "%s/sources.list";\nDir::Etc::SourceParts "-";\nDir::State::Lists "%s/lists";\nDir::Cache "%s/cache";\n' \
        "${dir}" "${dir}" "${dir}" > "${dir}/apt.conf" || return 1
    printf '%s' "${dir}/apt.conf"
}

# apt-get for package metadata and downloads; only the config package may use a
# shared release's repository.
npd_config_apt_get() {
    local conf
    conf="$(npd_config_apt_conf)" || return 1
    if [ -n "${conf}" ]; then
        APT_CONFIG="${conf}" apt-get "$@"
    else
        apt-get "$@"
    fi
}

npd_refresh_metadata() {
    case "$(npd_package_format)" in
        deb)
            npd_config_apt_get -o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
                -o APT::Update::Error-Mode=any update
            ;;
        rpm) npd_tdnf makecache --refresh ;;
        *) return 1 ;;
    esac
}

npd_download_package() {
    local version="$1"
    case "$(npd_package_format)" in
        deb)
            (cd "${NPD_PACKAGE_CACHE}" && npd_config_apt_get -o Acquire::Retries=3 \
                -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
                download "$(npd_package_name)=${version}")
            ;;
        rpm)
            npd_tdnf install -y --downloadonly --downloaddir "${NPD_PACKAGE_CACHE}" \
                "$(npd_package_name)-${version}"
            ;;
        *) return 1 ;;
    esac
}

npd_extract_package() {
    case "$(npd_package_format)" in
        deb) dpkg-deb -x "$1" "$2" ;;
        rpm) rpm2archive - < "$1" | tar -xzf - -C "$2" ;;
        *) return 1 ;;
    esac
}

npd_validate_package() {
    local package="$1" stage file
    stage="$(mktemp -d "${NPD_UPDATE_DIR}/validate.XXXXXX")" || return 1
    if ! npd_extract_package "${package}" "${stage}"; then
        rm -rf "${stage}"
        return 1
    fi
    local result=0
    if [ "${NPD_COMPONENT}" = npdBinary ]; then
        npd_validate_binary_stage "${stage}" || result=1
        rm -rf "${stage}"
        return "${result}"
    fi
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

# The binary package must carry the NPD executables, and the new binary must run on
# this node before it replaces the working one.
npd_validate_binary_stage() {
    local stage="$1" executable
    for executable in node-problem-detector npd-log-counter npd-health-checker; do
        test -x "${stage}/usr/bin/${executable}" || return 1
    done
    "${stage}/usr/bin/node-problem-detector" --version 2>&1 | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+'
}

npd_hold_packages() {
    case "$(npd_package_format)" in
        # dpkg selections avoid apt's slow initialization (see holdWALinuxAgent).
        deb) printf '%s hold\n' "$(npd_package_name)" | dpkg --set-selections ;;
        rpm)
            mkdir -p "$(dirname "${NPD_TDNF_LOCK_FILE}")" || return 1
            grep -qsx "$(npd_package_name)" "${NPD_TDNF_LOCK_FILE}" ||
                printf '%s\n' "$(npd_package_name)" >> "${NPD_TDNF_LOCK_FILE}"
            ;;
        *) return 1 ;;
    esac
}

npd_install_package() {
    local package="$1"
    # NPD packages are held against other updates; this exact local transaction
    # is authorized to change one, including restoring its old package.
    case "$(npd_package_format)" in
        deb)
            DEBIAN_FRONTEND=noninteractive apt-get -y --no-remove --no-install-recommends \
                --allow-change-held-packages --allow-downgrades --reinstall \
                -o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 \
                -o DPkg::Lock::Timeout=120 -o Dpkg::Options::=--force-confnew \
                -o Dpkg::Options::=--force-confmiss install "${package}" || return 1
            ;;
        rpm)
            rpm -U --oldpackage --replacepkgs "${package}" || return 1
            npd_remove_stale_rpm_entries "${package}" || return 1
            ;;
        *) return 1 ;;
    esac
    npd_hold_packages || return 1
    npd_remove_obsolete_configs
}

# An interrupted rpm upgrade can leave both versions in the rpm database. The
# package just installed owns the files, so only stale database entries remain.
npd_remove_stale_rpm_entries() {
    local keep entry
    keep="$(rpm -qp --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}' "$1" 2>/dev/null)" || return 1
    while IFS= read -r entry; do
        if [ -n "${entry}" ] && [ "${entry}" != "${keep}" ]; then
            rpm -e --justdb --nodeps --noscripts "${entry}" || return 1
        fi
    done < <(rpm -q --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}\n' "$(npd_package_name)" 2>/dev/null)
}

# An interrupted dpkg run blocks every later apt transaction until its journal
# is finished. Use the safe conffile defaults: unrelated packages may be pending,
# and the NPD package is reinstalled with its own options afterwards.
npd_repair_package_state() {
    case "$(npd_package_format)" in
        deb)
            DEBIAN_FRONTEND=noninteractive timeout 300 dpkg --force-confdef --force-confold --configure -a
            ;;
        rpm) return 0 ;;
        *) return 1 ;;
    esac
}

# Package-manager backups of NPD config files and the unit.
npd_find_config_backups() {
    find "${NPD_CONFIG_DIR}" "$(dirname "${NPD_UNIT_FILE}")" -maxdepth 4 \
        \( -path "${NPD_CONFIG_DIR}/*" -o -path "${NPD_UNIT_FILE}.*" \) \
        \( "$@" \) -print0 2>/dev/null
}

# NPD discovers monitor JSONs by directory, so leftover package configs must not
# survive upgrades or rollback. Never delete unrelated files such as the marker.
npd_remove_obsolete_configs() {
    local conffiles path _checksum obsolete file
    # Only the config package owns the monitor directory NPD reads.
    [ "${NPD_COMPONENT}" = npdConfig ] || return 0
    case "$(npd_package_format)" in
        deb)
            # dpkg retains removed conffiles.
            conffiles="$(dpkg-query -W -f='${Conffiles}' node-problem-detector-aks-config)" || return 1
            while read -r path _checksum obsolete; do
                if [ "${obsolete}" = obsolete ]; then
                    case "${path}" in
                        "${NPD_SKIP_FILE}") return 1 ;;
                        "${NPD_CONFIG_DIR}"/*) rm -f -- "${path}" || return 1 ;;
                    esac
                fi
            done <<< "${conffiles}"
            # --force-confnew keeps locally modified files as *.dpkg-old.
            while IFS= read -r -d '' file; do
                rm -f -- "${file}" || return 1
            done < <(npd_find_config_backups -name '*.dpkg-old' -o -name '*.dpkg-dist')
            ;;
        rpm)
            # rpm removes unmodified files dropped by the new package. Match
            # dpkg --force-confnew for locally modified %config(noreplace) files.
            while IFS= read -r -d '' file; do
                case "${file}" in
                    *.rpmnew) mv -f -- "${file}" "${file%.rpmnew}" || return 1 ;;
                    *.rpmsave) rm -f -- "${file}" || return 1 ;;
                esac
            done < <(npd_find_config_backups -name '*.rpmnew' -o -name '*.rpmsave')
            ;;
        *) return 1 ;;
    esac
}

# A process can be active with a healthy /healthz while every apiserver call fails
# (for example a TLS incompatibility between a new binary and the node's OpenSSL).
# After a binary update, require NPD to refresh its node condition heartbeat.
npd_check_reporting() {
    local node_json="$1" since="$2" node_name heartbeat attempt
    local kubectl_cmd="${KUBECTL:-/opt/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig}"
    node_name="$(printf '%s' "${node_json}" | jq -r '.metadata.name // empty')" || return 1
    [ -n "${node_name}" ] || return 1
    for ((attempt=0; attempt<NPD_REPORT_ATTEMPTS; attempt++)); do
        # shellcheck disable=SC2086
        heartbeat="$(${kubectl_cmd} get node "${node_name}" -o json 2>/dev/null | jq -r '
            [.status.conditions[]? | select(.type == "KernelDeadlock" or .type == "ReadonlyFilesystem")
             | .lastHeartbeatTime | fromdateiso8601] | max // 0' 2>/dev/null)" || heartbeat=0
        if [ "${heartbeat:-0}" -ge "${since}" ] 2>/dev/null; then
            return 0
        fi
        sleep 5
    done
    echo "${NPD_COMPONENT}: NPD did not report node conditions after the update" >&2
    return 1
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
# Reinstalling that cached package repairs both package-manager state and live files.
npd_recover_pending() {
    local old_version package pending
    pending="$(npd_pending_file)"
    [ -f "${pending}" ] || return 0
    old_version="$(cat "${pending}")" || return 1
    package="$(npd_find_cached_package "${old_version}")" || return 1
    # A repair can fail when a package's maintainer script keeps failing. Still
    # attempt the rollback: reinstalling the prior package replaces a broken NPD
    # package, and the install and health checks keep pending until it succeeds.
    npd_repair_package_state || echo "${NPD_COMPONENT}: package-manager repair failed; attempting rollback"
    npd_install_package "${package}" && npd_restart_and_check || return 1
    rm -f "${pending}"
}

# The shared transaction for the current component.
npd_update_component() {
    # Ubuntu securityPatch is sourced into the same reconciler and exports APT_CONFIG.
    # Keep its snapshot-only repositories out of NPD package operations.
    unset APT_CONFIG
    local payload="$1" node_json="${2:-}" platform distro release desired installed package pending since
    pending="$(npd_pending_file)"
    if [ ! -f "${NPD_SKIP_FILE}" ] || ! platform="$(npd_platform)"; then
        echo "${NPD_COMPONENT}: no action for an unsupported or extension-managed node"
        knead_emit_npd_config_event NoAction 'Node does not own baked NPD'
        return 0
    fi
    read -r distro release <<< "${platform}"
    # Require the baked package for normal onboarding; a pending transaction may
    # temporarily leave the package manager in a partial state.
    if [ ! -f "${pending}" ] && ! npd_installed_version >/dev/null; then
        echo "${NPD_COMPONENT}: no AgentBaker $(npd_package_name) package; no action"
        return 0
    fi
    mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}" || { npd_fail WorkDirectory; return 1; }
    npd_recover_pending || { npd_fail RecoveryFailed; return 1; }
    # Releases without their own config build follow their config source's entry.
    release="$(npd_component_release "${distro}" "${release}")"
    desired="$(npd_desired_version "${payload}" "${distro}" "${release}")" || { npd_fail InvalidConfig; return 1; }
    if [ -z "${desired}" ]; then
        echo "${NPD_COMPONENT}: no package selected for ${distro} ${release}; no action"
        knead_emit_npd_config_event NoAction "No package selected for ${distro} ${release}"
        return 0
    fi
    installed="$(npd_installed_version)" || { npd_fail InstalledVersion; return 1; }
    if npd_version_ge "${installed}" "${desired}"; then
        echo "${NPD_COMPONENT}: installed ${installed} satisfies ${desired}; no action"
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
    if ! printf '%s\n' "${installed}" > "${pending}.tmp" ||
       ! mv "${pending}.tmp" "${pending}"; then
        npd_fail TransactionState
        return 1
    fi
    since="$(date +%s)"
    if ! npd_install_package "${package}" || ! npd_restart_and_check ||
       { [ "${NPD_COMPONENT}" = npdBinary ] && ! npd_check_reporting "${node_json}" "${since}"; }; then
        npd_recover_pending || knead_emit_npd_config_event Failed 'reason=RecoveryFailed' Error
        npd_fail PackageActivation
        return 1
    fi
    rm -f "${pending}" || { npd_fail TransactionState; return 1; }
    echo "${NPD_COMPONENT} updated successfully: ${desired}"
    knead_emit_npd_config_event Succeeded "Installed ${desired}"
}

updateNPDConfigs() (
    NPD_COMPONENT=npdConfig
    npd_update_component "$@"
)

updateNPDBinary() (
    NPD_COMPONENT=npdBinary
    npd_update_component "$@"
)
