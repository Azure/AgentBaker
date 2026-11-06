#!/bin/bash
# shellcheck disable=SC2034
#
# Test doubles for the systemd and cgroup side of localdns-limits.sh, shared by the LocalDNS specs.
#
# setup_fake_localdns_slice <dir> [baked CPUQuota] [baked MemoryMax] writes a baked localdns.slice
# under <dir>, a running slice's cgroup files at those values, and points localdns-limits.sh at
# them. fake_localdns_systemctl stands in for the `systemctl show` and `systemctl set-property` calls
# localdns-limits.sh makes; a spec's own systemctl mock hands those subcommands to it. Like systemd,
# set-property applies the value to the cgroup and saves it as a drop-in, with CPUQuota saved
# truncated to a whole percent and MemoryMax in bytes.
#
# Knobs, all read at call time:
#   FAKE_SYSTEMCTL_DOWN=true            `systemctl show` fails (systemd does not respond)
#   FAKE_SET_PROPERTY_FAIL=<property>   set-property of that property (or "all") fails
#   FAKE_SET_PROPERTY_FAIL_FROM=<n>     the n-th set-property call of the test and every later one fail
#   FAKE_SET_PROPERTY_UNSAVED=<property> set-property of that property (or "all") exits 0 but only
#                                       changes the live value: the drop-in cannot be written
# Every set-property call is appended to ${FAKE_SET_PROPERTY_LOG}.

setup_fake_localdns_slice() {
    local dir="$1"
    local cpu="${2:-200.8%}"
    local memory="${3:-128M}"

    mkdir -p "${dir}/cgroup/localdns.slice" "${dir}/system.control"
    LOCALDNS_LIMITS_BAKED_SLICE_FILE="${dir}/localdns.slice"
    LOCALDNS_LIMITS_CGROUP_DIR="${dir}/cgroup/localdns.slice"
    LOCALDNS_LIMITS_SAVED_DIR="${dir}/system.control/localdns.slice.d"
    FAKE_SET_PROPERTY_LOG="${dir}/set-property.log"
    FAKE_SYSTEMCTL_DOWN=false
    FAKE_SET_PROPERTY_FAIL=""
    FAKE_SET_PROPERTY_FAIL_FROM=""
    FAKE_SET_PROPERTY_UNSAVED=""
    : > "${FAKE_SET_PROPERTY_LOG}"
    cat > "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}" <<EOF
[Unit]
Description=localdns Slice
DefaultDependencies=no
Before=slices.target
Requires=system.slice
After=system.slice
[Slice]
MemoryMax=${memory}
CPUQuota=${cpu}
EOF
    fake_localdns_apply_live "CPUQuota=${cpu}"
    fake_localdns_apply_live "MemoryMax=${memory}"
}

# Sets a live (kernel) value the way systemd would: cpu.max against a 100ms period, memory.max in bytes.
fake_localdns_apply_live() {
    local assignment="$1"
    local value="${assignment#*=}"

    case "${assignment}" in
        CPUQuota=*)
            if [ -z "${value}" ]; then
                echo "max 100000" > "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max"
            else
                awk -v v="${value%\%}" 'BEGIN { printf "%.0f 100000\n", v * 1000 }' > "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max"
            fi
            ;;
        MemoryMax=*)
            if [ "${value}" = infinity ]; then
                echo max > "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max"
            else
                localdns_limits_parse_memory "${value}" > "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max"
            fi
            ;;
    esac
}

fake_localdns_save() {
    local assignment="$1"
    local value="${assignment#*=}"

    mkdir -p "${LOCALDNS_LIMITS_SAVED_DIR}"
    case "${assignment}" in
        CPUQuota=*)
            if [ -n "${value}" ]; then
                value="$(awk -v v="${value%\%}" 'BEGIN { printf "%d%%", v }')"
            fi
            printf '# This is a drop-in unit file extension, created via "systemctl set-property"\n# or an equivalent operation. Do not edit.\n[Slice]\nCPUQuota=%s\n' "${value}" > "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf"
            ;;
        MemoryMax=*)
            if [ "${value}" != infinity ]; then
                value="$(localdns_limits_parse_memory "${value}")"
            fi
            printf '# This is a drop-in unit file extension, created via "systemctl set-property"\n# or an equivalent operation. Do not edit.\n[Slice]\nMemoryMax=%s\n' "${value}" > "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf"
            ;;
    esac
}

fake_localdns_systemctl() {
    local assignment
    local property
    local calls

    case "$1" in
        show)
            [ "${FAKE_SYSTEMCTL_DOWN}" != true ]
            return
            ;;
        set-property)
            assignment="$3"
            property="${assignment%%=*}"
            echo "${assignment}" >> "${FAKE_SET_PROPERTY_LOG}"
            calls="$(wc -l < "${FAKE_SET_PROPERTY_LOG}")"
            if [ "${FAKE_SET_PROPERTY_FAIL}" = all ] || [ "${FAKE_SET_PROPERTY_FAIL}" = "${property}" ] ||
                { [ -n "${FAKE_SET_PROPERTY_FAIL_FROM}" ] && [ "${calls}" -ge "${FAKE_SET_PROPERTY_FAIL_FROM}" ]; }; then
                echo "Failed to set unit properties on localdns.slice: Access denied"
                return 1
            fi
            fake_localdns_apply_live "${assignment}"
            if [ "${FAKE_SET_PROPERTY_UNSAVED}" != all ] && [ "${FAKE_SET_PROPERTY_UNSAVED}" != "${property}" ]; then
                fake_localdns_save "${assignment}"
            fi
            return 0
            ;;
    esac
    return 0
}

# The set-property calls made so far, one per line.
fake_set_property_calls() {
    cat "${FAKE_SET_PROPERTY_LOG}"
}
