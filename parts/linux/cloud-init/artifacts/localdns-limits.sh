#!/bin/bash
#
# LocalDNS CPU and memory limit live patching, shared by localdns.sh (bootstrap) and
# localdns-livepatch-handler.sh (runtime). Both source this file; it only defines functions. It also
# holds the two Corefile steps both of them run: rendering the served Corefile and exporting its
# forward targets as metrics (at the end of this file).
#
# corefileVersion, the RP's hash of a pool's LocalDNS profile, covers the CPU and memory limits as
# well as the Corefile, so version X is only applied once localdns.slice runs with X's limits too.
# CSE writes the provisioning ("baked") limits to /etc/systemd/system/localdns.slice and nothing
# here changes that file. A patch goes on top with `systemctl set-property`, which applies a value to
# the running slice at once and saves it as a drop-in under /etc/systemd/system.control, so it
# survives a reboot. Removing those drop-ins brings the baked values back on the next boot.
#
# Each limit is read in three layers:
#   live   the slice's cgroup v2 cpu.max / memory.max: what the kernel enforces now
#   saved  the set-property drop-ins: what a reboot comes back with
#   baked  localdns.slice: what applies when there is no drop-in
#
# systemd behaviour this relies on (verified on systemd 249 and 255):
# - set-property exits 0 when it cannot write the drop-in, having changed only the live value, so a
#   patch is verified against the drop-ins as well as the kernel.
# - CPUQuota is saved truncated to a whole percent, so a saved CPU limit is compared in whole percent.
# - `systemctl show` formats CPUQuotaPerSecUSec in varying units (2s, 1.500000s, 333ms), so the live
#   values come from cpu.max and memory.max instead.
# Only cgroup v2 is supported: no OS that runs LocalDNS uses cgroup v1. On one that did, the live
# values could not be read and a patch would fail its preflight.
#
# Values are passed around in canonical units: CPU in hundredths of a percent of one CPU (150.5% is
# 15050), memory in bytes, and "max" for no limit. Functions that produce values print them on
# stdout; on failure they print the reason there instead, for the caller's status message. What they
# do along the way (each set-property call, a limit already in effect) is logged on stderr.

LOCALDNS_LIMITS_SLICE_UNIT="${LOCALDNS_LIMITS_SLICE_UNIT:-localdns.slice}"
# Written by CSE (cse_config_localdns.sh) from the provisioning LocalDNS profile.
LOCALDNS_LIMITS_BAKED_SLICE_FILE="${LOCALDNS_LIMITS_BAKED_SLICE_FILE:-/etc/systemd/system/localdns.slice}"
LOCALDNS_LIMITS_CGROUP_DIR="${LOCALDNS_LIMITS_CGROUP_DIR:-/sys/fs/cgroup/localdns.slice}"
# Where `systemctl set-property` (without --runtime) saves each property, as 50-<property>.conf.
LOCALDNS_LIMITS_SAVED_DIR="${LOCALDNS_LIMITS_SAVED_DIR:-/etc/systemd/system.control/localdns.slice.d}"
# CoreDNS's ready plugin (ready 169.254.10.10:8181 in the health-check block of the template).
LOCALDNS_READY_URL="${LOCALDNS_READY_URL:-http://169.254.10.10:8181/ready}"

# Prints a CPUQuota= value (150%, 150.5%) in hundredths of a percent; max when it is empty or infinity.
localdns_limits_parse_cpu() {
    local value="$1"

    case "${value}" in
        ""|infinity)
            echo max
            return 0
            ;;
    esac
    printf '%s\n' "${value}" | awk '
        /^[0-9]+(\.[0-9]+)?%$/ { sub(/%$/, ""); printf "%.0f\n", $0 * 100; ok = 1 }
        END { if (!ok) exit 1 }'
}

# Prints a MemoryMax= value (bytes, or with a K/M/G/T suffix to base 1024) in bytes; max when it is
# empty or infinity.
localdns_limits_parse_memory() {
    local value="$1"

    case "${value}" in
        ""|infinity)
            echo max
            return 0
            ;;
    esac
    printf '%s\n' "${value}" | awk '
        /^[0-9]+[KMGT]?$/ {
            n = $0
            m = 1
            unit = substr(n, length(n))
            if (unit == "K") m = 1024
            else if (unit == "M") m = 1048576
            else if (unit == "G") m = 1073741824
            else if (unit == "T") m = 1099511627776
            if (m > 1) n = substr(n, 1, length(n) - 1)
            printf "%.0f\n", n * m
            ok = 1
        }
        END { if (!ok) exit 1 }'
}

# Prints a CPU limit the way set-property takes it and the status message shows it: 150%, 150.5%,
# or unlimited.
localdns_limits_format_cpu() {
    case "$1" in
        max) echo unlimited ;;
        *[!0-9]*|"") echo "$1" ;;
        *) awk -v v="$1" 'BEGIN { s = sprintf("%.2f", v / 100); sub(/\.?0+$/, "", s); print s "%" }' ;;
    esac
}

# Prints a memory limit as the status message shows it: 2048M, or unlimited.
localdns_limits_format_memory() {
    case "$1" in
        max) echo unlimited ;;
        *[!0-9]*|"") echo "$1" ;;
        *) awk -v v="$1" 'BEGIN { if (v % 1048576 == 0) printf "%.0fM\n", v / 1048576; else printf "%.0f bytes\n", v }' ;;
    esac
}

# Prints the last value of key in a unit file or drop-in. Fails when the file does not set it.
localdns_limits_read_setting() {
    local file="$1"
    local key="$2"

    awk -v key="${key}" '
        {
            eq = index($0, "=")
            if (eq == 0) next
            k = substr($0, 1, eq - 1)
            v = substr($0, eq + 1)
            gsub(/^[ \t]+|[ \t]+$/, "", k)
            gsub(/^[ \t]+|[ \t]+$/, "", v)
            if (k == key) { value = v; found = 1 }
        }
        END { if (!found) exit 1; print value }' "${file}" 2>/dev/null
}

# Prints the baked "<cpu> <memory>" from localdns.slice.
localdns_limits_read_baked() {
    local file="${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
    local cpu
    local memory

    if ! cpu="$(localdns_limits_read_setting "${file}" CPUQuota)" || ! cpu="$(localdns_limits_parse_cpu "${cpu}")" ||
        ! memory="$(localdns_limits_read_setting "${file}" MemoryMax)" || ! memory="$(localdns_limits_parse_memory "${memory}")"; then
        echo "cannot read the baked limits from ${file}"
        return 1
    fi
    echo "${cpu} ${memory}"
}

# Prints the saved "<cpu> <memory>": "-" for a limit without a drop-in, "invalid" for one whose
# drop-in cannot be parsed (which never matches, so the next apply rewrites it).
localdns_limits_read_saved() {
    local cpu_file="${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf"
    local memory_file="${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf"
    local cpu="-"
    local memory="-"
    local value

    if [ -e "${cpu_file}" ]; then
        if ! value="$(localdns_limits_read_setting "${cpu_file}" CPUQuota)" || ! cpu="$(localdns_limits_parse_cpu "${value}")"; then
            cpu=invalid
        fi
    fi
    if [ -e "${memory_file}" ]; then
        if ! value="$(localdns_limits_read_setting "${memory_file}" MemoryMax)" || ! memory="$(localdns_limits_parse_memory "${value}")"; then
            memory=invalid
        fi
    fi
    echo "${cpu} ${memory}"
}

# Prints the live "<cpu> <memory>" the kernel enforces on the slice, or "- -" when the slice is not
# running (its cgroup does not exist). Fails when the cgroup exists but cannot be read.
localdns_limits_read_live() {
    local dir="${LOCALDNS_LIMITS_CGROUP_DIR}"
    local cpu
    local memory

    if [ ! -d "${dir}" ]; then
        echo "- -"
        return 0
    fi
    # cpu.max is "<quota> <period>" in microseconds, or "max <period>".
    if ! cpu="$(awk '
            NR == 1 && NF == 2 {
                if ($1 == "max") { print "max"; ok = 1 }
                else if ($1 ~ /^[0-9]+$/ && $2 ~ /^[0-9]+$/ && $2 > 0) { printf "%.0f\n", $1 * 10000 / $2; ok = 1 }
            }
            END { if (!ok) exit 1 }' "${dir}/cpu.max" 2>/dev/null)" ||
        ! memory="$(awk '
            NR == 1 && ($1 == "max" || $1 ~ /^[0-9]+$/) { print $1; ok = 1 }
            END { if (!ok) exit 1 }' "${dir}/memory.max" 2>/dev/null)"; then
        echo "cannot read the running limits from ${dir}"
        return 1
    fi
    echo "${cpu} ${memory}"
}

# Whether a saved (or baked) limit stands for the wanted one. CPUQuota is saved truncated to a whole
# percent, so CPU limits are compared in whole percent; memory limits exactly.
localdns_limits_saved_matches() {
    local kind="$1"
    local saved="$2"
    local want="$3"

    if [ "${kind}" != cpu ]; then
        [ "${saved}" = "${want}" ]
        return
    fi
    case "${saved}${want}" in
        *[!0-9]*|"")
            [ "${saved}" = "${want}" ]
            return
            ;;
    esac
    [ "$((saved / 100))" -eq "$((want / 100))" ]
}

localdns_limits_format() {
    local kind="$1"
    local value="$2"

    if [ "${kind}" = cpu ]; then
        localdns_limits_format_cpu "${value}"
    else
        localdns_limits_format_memory "${value}"
    fi
}

# Checks that one limit is in effect: the kernel enforces it (unless the slice is not running) and
# a reboot keeps it, through its drop-in or, without one, the baked value. Prints why not.
localdns_limits_check() {
    local kind="$1"
    local want="$2"
    local live="$3"
    local saved="$4"
    local baked="$5"
    local persisted="${saved}"

    if [ "${persisted}" = "-" ]; then
        persisted="${baked}"
    fi
    if [ "${live}" != "-" ] && [ "${live}" != "${want}" ]; then
        echo "not applied: live $(localdns_limits_format "${kind}" "${live}"), wanted $(localdns_limits_format "${kind}" "${want}")"
        return 1
    fi
    if ! localdns_limits_saved_matches "${kind}" "${persisted}" "${want}"; then
        if [ "${live}" = "-" ]; then
            echo "not saved: saved $(localdns_limits_format "${kind}" "${persisted}"), wanted $(localdns_limits_format "${kind}" "${want}")"
        else
            echo "applied but not saved: live $(localdns_limits_format "${kind}" "${live}"), saved $(localdns_limits_format "${kind}" "${persisted}")"
        fi
        return 1
    fi
}

# Sets one limit on the slice: one property per call, without --runtime (the drop-in is what makes it
# survive a reboot) and without a daemon-reload (set-property applies it at once).
localdns_limits_set_property() {
    local kind="$1"
    local value="$2"
    local assignment
    local output

    if [ "${kind}" = cpu ]; then
        # An empty CPUQuota= removes the quota.
        if [ "${value}" = max ]; then
            assignment="CPUQuota="
        else
            assignment="CPUQuota=$(localdns_limits_format_cpu "${value}")"
        fi
    else
        if [ "${value}" = max ]; then
            assignment="MemoryMax=infinity"
        else
            assignment="MemoryMax=${value}"
        fi
    fi
    echo "systemctl set-property ${LOCALDNS_LIMITS_SLICE_UNIT} ${assignment}" >&2
    if ! output="$(systemctl set-property "${LOCALDNS_LIMITS_SLICE_UNIT}" "${assignment}" 2>&1)"; then
        echo "systemctl set-property ${assignment} failed: $(printf '%s\n' "${output}" | head -n 1)"
        return 1
    fi
}

# Applies the CPU and then the memory limit, skipping one already in effect, and verifies both against
# the kernel and the drop-ins. On failure prints which limit failed and why, as
# "the cpu limit failed (<reason>)", with the reason cut to 200 characters.
localdns_limits_apply() {
    local want_cpu="$1"
    local want_memory="$2"
    local baked_cpu="$3"
    local baked_memory="$4"
    local live
    local live_cpu
    local live_memory
    local saved_cpu
    local saved_memory
    local reason
    local cpu_reason=""
    local memory_reason=""

    if ! live="$(localdns_limits_read_live)"; then
        printf 'the cpu and memory limits failed (%.200s)\n' "${live}"
        return 1
    fi
    read -r live_cpu live_memory <<< "${live}"
    read -r saved_cpu saved_memory <<< "$(localdns_limits_read_saved)"

    if localdns_limits_check cpu "${want_cpu}" "${live_cpu}" "${saved_cpu}" "${baked_cpu}" > /dev/null; then
        echo "cpu limit $(localdns_limits_format_cpu "${want_cpu}") is already in effect" >&2
    elif ! reason="$(localdns_limits_set_property cpu "${want_cpu}")"; then
        printf 'the cpu limit failed (%.200s)\n' "${reason}"
        return 1
    fi
    if localdns_limits_check memory "${want_memory}" "${live_memory}" "${saved_memory}" "${baked_memory}" > /dev/null; then
        echo "memory limit $(localdns_limits_format_memory "${want_memory}") is already in effect" >&2
    elif ! reason="$(localdns_limits_set_property memory "${want_memory}")"; then
        printf 'the memory limit failed (%.200s)\n' "${reason}"
        return 1
    fi

    if ! live="$(localdns_limits_read_live)"; then
        printf 'the cpu and memory limits failed (%.200s)\n' "${live}"
        return 1
    fi
    read -r live_cpu live_memory <<< "${live}"
    read -r saved_cpu saved_memory <<< "$(localdns_limits_read_saved)"
    cpu_reason="$(localdns_limits_check cpu "${want_cpu}" "${live_cpu}" "${saved_cpu}" "${baked_cpu}")" || true
    memory_reason="$(localdns_limits_check memory "${want_memory}" "${live_memory}" "${saved_memory}" "${baked_memory}")" || true
    if [ -n "${cpu_reason}" ] && [ -n "${memory_reason}" ]; then
        printf 'the cpu and memory limits failed (%.200s)\n' "cpu ${cpu_reason}; memory ${memory_reason}"
        return 1
    fi
    if [ -n "${cpu_reason}" ]; then
        printf 'the cpu limit failed (%.200s)\n' "${cpu_reason}"
        return 1
    fi
    if [ -n "${memory_reason}" ]; then
        printf 'the memory limit failed (%.200s)\n' "${memory_reason}"
        return 1
    fi
    echo "localdns.slice limits in effect: cpu $(localdns_limits_format_cpu "${want_cpu}"), memory $(localdns_limits_format_memory "${want_memory}")" >&2
}

# Step 0 of a patch. Checks that systemctl responds and the baked limits parse, and records the
# limits the slice runs with now -- or, when it is not running, the ones it would start with -- which
# a failed patch reverts to. Prints "<baked cpu> <baked memory> <previous cpu> <previous memory>".
# Returns 2 when the node has no localdns.slice at all, i.e. LocalDNS was never set up here, and 1
# on any other failure; the reason printed then is the status message.
localdns_limits_preflight() {
    local baked
    local live
    local baked_cpu
    local baked_memory
    local live_cpu
    local live_memory
    local saved_cpu
    local saved_memory
    local previous_cpu
    local previous_memory

    if [ ! -e "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}" ]; then
        echo "cannot read the baked limits from ${LOCALDNS_LIMITS_BAKED_SLICE_FILE}; limits and corefile left unchanged"
        return 2
    fi
    if ! systemctl show --property=Id "${LOCALDNS_LIMITS_SLICE_UNIT}" > /dev/null 2>&1; then
        echo "systemctl did not respond; limits and corefile left unchanged"
        return 1
    fi
    if ! baked="$(localdns_limits_read_baked)"; then
        echo "${baked}; limits and corefile left unchanged"
        return 1
    fi
    if ! live="$(localdns_limits_read_live)"; then
        echo "${live}; limits and corefile left unchanged"
        return 1
    fi
    read -r baked_cpu baked_memory <<< "${baked}"
    read -r live_cpu live_memory <<< "${live}"
    read -r saved_cpu saved_memory <<< "$(localdns_limits_read_saved)"

    previous_cpu="${live_cpu}"
    if [ "${previous_cpu}" = "-" ]; then
        case "${saved_cpu}" in
            -|invalid) previous_cpu="${baked_cpu}" ;;
            *) previous_cpu="${saved_cpu}" ;;
        esac
    fi
    previous_memory="${live_memory}"
    if [ "${previous_memory}" = "-" ]; then
        case "${saved_memory}" in
            -|invalid) previous_memory="${baked_memory}" ;;
            *) previous_memory="${saved_memory}" ;;
        esac
    fi
    echo "${baked_cpu} ${baked_memory} ${previous_cpu} ${previous_memory}"
}

# Reads aks-node-controller's rendered/alreadyCurrent output: "version: X", then "cpuQuota: 150.0%"
# and "memoryMax: 512M" for the limits the profile sets. A limit it leaves out means the baked one.
# Prints "<version> <cpu> <memory>".
localdns_limits_from_anc_output() {
    local output="$1"
    local baked_cpu="$2"
    local baked_memory="$3"
    local version
    local value
    local cpu="${baked_cpu}"
    local memory="${baked_memory}"

    version="$(printf '%s\n' "${output}" | sed -n 's/^version: *//p' | tail -n 1 | tr -d '[:space:]')"
    if [ -z "${version}" ]; then
        echo "aks-node-controller printed no corefileVersion"
        return 1
    fi
    value="$(printf '%s\n' "${output}" | sed -n 's/^cpuQuota: *//p' | tail -n 1 | tr -d '[:space:]')"
    if [ -n "${value}" ] && ! cpu="$(localdns_limits_parse_cpu "${value}")"; then
        printf 'cannot parse the cpu limit %.50s from aks-node-controller\n' "${value}"
        return 1
    fi
    value="$(printf '%s\n' "${output}" | sed -n 's/^memoryMax: *//p' | tail -n 1 | tr -d '[:space:]')"
    if [ -n "${value}" ] && ! memory="$(localdns_limits_parse_memory "${value}")"; then
        printf 'cannot parse the memory limit %.50s from aks-node-controller\n' "${value}"
        return 1
    fi
    echo "${version} ${cpu} ${memory}"
}

# Removes the set-property drop-ins, so the next boot comes back on the baked limits. No
# daemon-reload: until then systemd keeps enforcing what it has.
localdns_limits_remove_saved() {
    rm -f "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf" "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf"
}

# The last resort when the limits cannot be set back: removes the drop-ins and prints where that
# leaves each layer, for the status message.
localdns_limits_fall_back_to_baked() {
    local baked_cpu="$1"
    local baked_memory="$2"
    local live
    local live_cpu
    local live_memory
    local in_effect
    local saved="removed, baked applies after a reboot"

    if ! localdns_limits_remove_saved; then
        saved="could not be removed"
    fi
    if ! live="$(localdns_limits_read_live)"; then
        in_effect="unknown"
    else
        read -r live_cpu live_memory <<< "${live}"
        if [ "${live_cpu}" = "-" ]; then
            in_effect="${LOCALDNS_LIMITS_SLICE_UNIT} is not running"
        else
            in_effect="cpu $(localdns_limits_format_cpu "${live_cpu}"), memory $(localdns_limits_format_memory "${live_memory}")"
        fi
    fi
    echo "in effect: ${in_effect}; saved: ${saved}; baked: cpu $(localdns_limits_format_cpu "${baked_cpu}"), memory $(localdns_limits_format_memory "${baked_memory}")"
}

# Sets the limits back to the ones a failed patch started from (or, for a revoke, the baked ones).
# Prints them and returns 0 when that worked. Otherwise falls back to baked for the next boot,
# prints where each layer stands, and returns 1.
localdns_limits_revert() {
    local cpu="$1"
    local memory="$2"
    local baked_cpu="$3"
    local baked_memory="$4"
    local reason

    if reason="$(localdns_limits_apply "${cpu}" "${memory}" "${baked_cpu}" "${baked_memory}")"; then
        echo "cpu $(localdns_limits_format_cpu "${cpu}"), memory $(localdns_limits_format_memory "${memory}")"
        return 0
    fi
    echo "reverting the localdns.slice limits failed: ${reason}" >&2
    localdns_limits_fall_back_to_baked "${baked_cpu}" "${baked_memory}"
    return 1
}

# Builds the status message for a patch that failed before it was committed and was undone:
# "<failure>; reverted to the previous corefile and limits: cpu 400%, memory 2048M", or, when the
# limits could not be reverted, what localdns_limits_revert reported instead. Capped at 512
# characters, the live-patching status limit.
localdns_limits_failure_message() {
    local failure="$1"
    local reverted="$2"
    local detail="$3"

    if [ "${reverted}" = true ]; then
        printf '%.512s' "${failure}; reverted to the previous corefile and limits: ${detail}"
    else
        printf '%.512s' "${failure}; previous corefile restored; reverting to the previous limits also failed; ${detail}"
    fi
}

# Writes the .version file that commits a patch: version X's Corefile and limits are applied.
# Only the caller that applied both writes it; aks-node-controller only removes it.
localdns_write_corefile_version() {
    local version_file="$1"
    local version="$2"
    local tmp

    if ! tmp="$(mktemp "${version_file}.XXXXXX")"; then
        return 1
    fi
    if ! printf '%s\n' "${version}" > "${tmp}" || ! chmod 0600 "${tmp}" || ! mv -f "${tmp}" "${version_file}"; then
        rm -f "${tmp}"
        return 1
    fi
}

# Whether CoreDNS reports ready. --noproxy keeps an http-proxy cluster's proxy away from the
# link-local listener.
localdns_ready() {
    [ "$(curl -s --noproxy '*' --connect-timeout 2 --max-time 5 "${LOCALDNS_READY_URL}" 2>/dev/null)" = "OK" ]
}

# Waits up to timeout seconds for CoreDNS to report ready, checking every poll seconds.
localdns_wait_ready() {
    local timeout="$1"
    local poll="${2:-2}"
    local deadline=$((SECONDS + timeout))

    while ! localdns_ready; do
        if [ "${SECONDS}" -ge "${deadline}" ]; then
            return 1
        fi
        sleep "${poll}"
    done
}

# Builds the Corefile CoreDNS serves from a source Corefile. The Corefile is rendered with the Azure DNS
# IP standing in for the VNet's DNS servers in every VnetDNS forward; this replaces it with the servers
# localdns.sh learned from DHCP and recorded at startup (Azure DNS itself on an Azure DNS VNet).
# localdns.sh uses it at startup and the live-patch handler for an in-place patch, so both serve the
# same Corefile for the same source.
localdns_render_served_corefile() {
    local source_corefile="$1"
    local output="$2"
    local forward_servers="$3"
    local azure_dns_ip="${4:-168.63.129.16}"

    sed -e "s|${azure_dns_ip}|${forward_servers}|g" "${source_corefile}" > "${output}"
}

# Exports the forward targets of a served Corefile to prom_file for the localdns metrics exporter: one
# localdns_vnetdns_forward_info line per forward in a block bound to the node listener, one
# localdns_kubedns_forward_info line per forward in a block bound to the cluster listener, each labeled
# with its block. localdns.sh exports at startup and the live-patch handler after an in-place reload,
# so the metrics follow the Corefile CoreDNS serves. Best-effort: a failure is logged on stdout and
# never returned, so metrics never affect DNS. Exporting this way avoids parsing the Corefile on every
# metrics scrape.
localdns_export_forward_ips() {
    local served_corefile="$1"
    local prom_file="$2"
    local node_listener_ip="${3:-169.254.10.10}"
    local cluster_listener_ip="${4:-169.254.10.11}"
    local forward_entries bind_ip block fwd_ip
    local vnetdns_found=false kubedns_found=false
    local tmp

    # Single-pass awk outputs "bind_ip|zone|forward_ip" triples:
    #   - Tracks current zone from block headers like ".:53 {" or "cluster.local:53 {"
    #   - Tracks bind IP from "bind 169.254.10.10" or "bind 169.254.10.11"
    #   - Extracts forward IPs from "forward . <ip> ..." lines
    #   - Resets state on block close "}"
    forward_entries=$(awk '
        /^[^ #].*:53 / {
            block = $1
            bind_ip = ""
        }
        /bind / {
            for (i=2; i<=NF; i++) {
                if ($i ~ /^169\.254\.10\.(10|11)$/) bind_ip = $i
            }
        }
        /forward \. / && bind_ip != "" {
            for (i=3; i<=NF; i++) {
                if ($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/)
                    print bind_ip "|" block "|" $i
            }
        }
        /^}/ { block = ""; bind_ip = "" }
    ' "${served_corefile}")

    # Write the metrics to a temp file, then rename it into place, so the exporter never reads a
    # partially written file during a scrape.
    if ! tmp="$(mktemp "${prom_file}.XXXXXX")"; then
        echo "WARNING: Failed to create temp file for ${prom_file}. Metrics export skipped."
    elif ! {
        echo "# HELP localdns_vnetdns_forward_info VnetDNS forward plugin IP address from corefile"
        echo "# TYPE localdns_vnetdns_forward_info gauge"
        while IFS='|' read -r bind_ip block fwd_ip; do
            [ "$bind_ip" = "${node_listener_ip}" ] || continue
            echo "localdns_vnetdns_forward_info{ip=\"${fwd_ip}\",block=\"${block}\",status=\"ok\"} 1"
            vnetdns_found=true
        done <<< "$forward_entries"
        if [ "$vnetdns_found" = false ]; then
            echo 'localdns_vnetdns_forward_info{ip="unknown",block="none",status="missing"} 0'
        fi
        echo "# HELP localdns_kubedns_forward_info KubeDNS forward plugin IP address from corefile"
        echo "# TYPE localdns_kubedns_forward_info gauge"
        while IFS='|' read -r bind_ip block fwd_ip; do
            [ "$bind_ip" = "${cluster_listener_ip}" ] || continue
            echo "localdns_kubedns_forward_info{ip=\"${fwd_ip}\",block=\"${block}\",status=\"ok\"} 1"
            kubedns_found=true
        done <<< "$forward_entries"
        if [ "$kubedns_found" = false ]; then
            echo 'localdns_kubedns_forward_info{ip="unknown",block="none",status="missing"} 0'
        fi
    } > "$tmp"; then
        echo "WARNING: Failed to write metrics to temp file $tmp. Metrics export skipped."
        rm -f "$tmp"
    elif ! chmod 0644 "$tmp"; then
        echo "WARNING: Failed to set permissions on temp file $tmp. Metrics export skipped."
        rm -f "$tmp"
    elif ! mv -f "$tmp" "${prom_file}"; then
        echo "WARNING: Failed to move temp file to ${prom_file}. Metrics export skipped."
        rm -f "$tmp"
    else
        echo "Successfully exported forward IPs to ${prom_file}"
    fi
    return 0
}
