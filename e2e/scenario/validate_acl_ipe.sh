#!/bin/bash
# IPE (Integrity Policy Enforcement) checks executable code against a kernel policy.
# ACL's policy should trust the signed /usr filesystem, but not writable storage.
# Audit mode logs DENY decisions without blocking them: both probes must succeed,
# while only the writable copy should produce a denial. Successful execution
# alone would not prove that IPE is working.
set -euo pipefail

ipe_fail() {
    echo "IPE validation failed: $*" >&2
    return 1
}

ipe_check_audit_mode() {
    # enforce=0 means audit mode, but is meaningful only with an active policy.
    local root="$1" enforce active
    enforce=$(cat "$root/enforce") || return 1
    active=$(cat "$root/policies/acl_ipe_boot_policy/active") || return 1
    printf 'IPE policy active=%s enforce=%s\n' "$active" "$enforce"
    [[ "$active" == 1 && "$enforce" == 0 ]]
}

ipe_check_signed_verity() {
    # dm-verity protects filesystem contents using a root hash. This descriptor
    # shows a signature on that root hash, not a signature on each executable.
    # The later /usr probe checks whether IPE actually accepts execution from it.
    local table="$1" cmdline="$2" root_hash actual
    # A signed mapping must use the root hash selected at boot, not just any signed hash.
    root_hash=$(awk '{
        for (i = 1; i <= NF; i++) {
            if ($i ~ /^usrhash=/) {
                count++
                hash = substr($i, 9)
            }
        }
    } END { if (count != 1) exit 1; print tolower(hash) }' <<< "$cmdline") || {
        ipe_fail "expected exactly one usrhash parameter in the boot command line"
        return 1
    }
    [[ "$root_hash" =~ ^[0-9a-f]{64}$ ]] || {
        ipe_fail "expected a 64-digit SHA256 boot usrhash, got '$root_hash'"
        return 1
    }
    # dmsetup table fields: $3 is the target type, $11 the hash algorithm,
    # and $12 the root hash (the first two fields are sector offset and length).
    actual=$(awk '{print $3, $11, tolower($12)}' <<< "$table") || {
        ipe_fail "cannot read the /usr verity table fields"
        return 1
    }
    [[ "$actual" == "verity sha256 $root_hash" ]] || {
        ipe_fail "expected /usr mapping 'verity sha256 $root_hash', got '$actual'"
        return 1
    }
    grep -Eq '(^|[[:space:]])root_hash_sig_key_desc[[:space:]]+[^[:space:]]+' <<< "$table" ||
        ipe_fail "/usr verity mapping has no root-hash signature key descriptor"
}

ipe_check_containerd() {
    # EROFS is the read-only container-layer filesystem used by the IPE profile.
    # Inspect the effective config so a fallback to another profile cannot pass.
    awk '
        { gsub(/\047/, "\""); sub(/^[[:space:]]+/, ""); sub(/[[:space:]]+$/, "") }
        /^\[/ { section = $0 }
        section == "[plugins.\"io.containerd.cri.v1.images\"]" &&
            /^snapshotter =/ { snapshotter = ($0 == "snapshotter = \"erofs\"") }
        section == "[plugins.\"io.containerd.snapshotter.v1.erofs\"]" &&
            /^enable_dmverity_referrers =/ { referrers = ($0 == "enable_dmverity_referrers = true") }
        section == "[plugins.\"io.containerd.service.v1.diff-service\"]" &&
            /^default =/ { differ = ($0 == "default = [\"erofs\", \"walking\"]") }
        END { if (!snapshotter || !referrers || !differ) exit 1 }
    '
}

# Reads audit records from stdin and prints matching denials.
# Returns 0 if a denial is found, 1 if none is found, and another status on error.
# Finding a denial is required for the writable probe, but fails the /usr probe.
ipe_denial() {
    local pid="$1" path="${2:-}"
    [[ "$pid" =~ ^[1-9][0-9]*$ && ( -z "$path" || "$path" == /var/tmp/acl-ipe-* ) ]] || {
        ipe_fail "invalid probe PID or path"
        return 2
    }
    # For the writable probe, match its exact path and the exec hook (BPRM_CHECK).
    # For /usr, include all execution denials for that PID, including libraries.
    awk -v pid="$pid" -v path="$path" '{
        same_pid = $0 ~ ("(^|[[:space:]])pid=" pid "([[:space:]]|$)")
        same_path = (path == "" || index($0, "path=\"" path "\""))
        execute = /ipe_op=EXECUTE([[:space:]]|$)/
        audit_mode = /enforcing=0([[:space:]]|$)/
        expected_hook = (path == "" || /ipe_hook=BPRM_CHECK([[:space:]]|$)/)
        deny = /rule="[^"]*action=DENY"/
        if (same_pid && same_path && execute && audit_mode && expected_hook && deny) {
            print
            found = 1
        }
    }
    END { if (!found) exit 1 }
    '
}

ipe_check_usr_audit() {
    local denial status
    if denial=$(ipe_denial "$1"); then
        printf 'Unexpected IPE denial for the /usr executable or its dependencies:\n%s\n' "$denial" >&2
        return 1
    else
        status=$?
    fi
    # Only "no match" is success here; a parser/command error is not proof of trust.
    [[ "$status" == 1 ]]
}

ipe_check_audit_window() {
    # Missing denials are not credible evidence if the VM rebooted or logs were lost.
    local before="$1" after="$2" logs="$3"
    [[ -n "$before" && "$before" == "$after" ]] || return 1
    # printk's shared rate limiter can name another caller when reporting lost records.
    ! grep -Ei \
        -e 'audit_lost=[1-9][0-9]*' \
        -e 'audit.*(backlog limit exceeded|rate limit exceeded|queue overflow)' \
        -e 'suppressed [0-9]+ messages' \
        -e '[0-9]+ callbacks suppressed' \
        -e 'missed [0-9]+ kernel messages' <<< "$logs"
}

ipe_check_signed_usr() {
    # A signed mapping must actually back /usr, not merely exist on the node.
    local usr_device source table cmdline mounted=false
    usr_device=$(readlink -e /dev/mapper/usr)
    while IFS= read -r source; do
        source="${source%%\[*}"
        if [[ "$(readlink -e "$source")" == "$usr_device" ]]; then mounted=true; fi
    done < <(findmnt -rn -o SOURCE /usr)
    [[ "$mounted" == true ]] || ipe_fail "/usr is not backed by its verity mapping"
    table=$(dmsetup table usr)
    cmdline=$(cat /proc/cmdline)
    printf 'Active /usr mapping: %s\n' "$table"
    ipe_check_signed_verity "$table" "$cmdline" ||
        ipe_fail "/usr verity mapping is unsigned or does not match the boot usrhash"
}

ipe_check_erofs_runtime() {
    local containerd_pid containerd_binary dump
    systemctl is-active --quiet containerd || ipe_fail "containerd is not active"
    containerd_pid=$(systemctl show containerd --property=MainPID --value) ||
        ipe_fail "cannot read containerd MainPID"
    [[ "$containerd_pid" =~ ^[1-9][0-9]*$ ]] ||
        ipe_fail "expected a positive containerd PID, got '$containerd_pid'"
    grep -zFxq -- /run/containerd/acl-config.toml "/proc/$containerd_pid/cmdline" ||
        ipe_fail "containerd PID $containerd_pid does not reference the expected /run/containerd/acl-config.toml"
    containerd_binary=$(readlink -e "/proc/$containerd_pid/exe") ||
        ipe_fail "cannot resolve containerd executable for PID $containerd_pid"
    dump=$("$containerd_binary" --config /run/containerd/acl-config.toml config dump) ||
        ipe_fail "cannot dump the effective IPE containerd configuration"
    printf 'Effective containerd configuration:\n%s\n' "$dump"
    ipe_check_containerd <<< "$dump" || ipe_fail "containerd did not select the EROFS IPE profile"
}

# Bookmark each transport separately. Events can be in audit or kernel;
# journald reports its own loss/suppression notices on driver.
# Cursors exclude notices from before the probes.
ipe_journal_cursor() {
    local boot_id="$1" transport="$2"
    journalctl --boot="${boot_id//-/}" "_TRANSPORT=$transport" -n 1 --show-cursor --no-pager |
        sed -n 's/^-- cursor: //p'
}

ipe_read_journal() {
    local boot_id="$1" transport="$2" cursor="$3"
    local -a position=()
    # A transport with no entries yet has no cursor; the boot still scopes it.
    if [[ -n "$cursor" ]]; then position=(--after-cursor="$cursor"); fi
    journalctl --boot="${boot_id//-/}" "_TRANSPORT=$transport" "${position[@]}" --no-pager -o cat
}

ipe_collect_audit_logs() {
    local boot_id="$1" audit_cursor="$2" kernel_cursor="$3" driver_cursor="$4"
    local audit_logs kernel_logs driver_logs logs current_boot
    # Propagate read errors: an unreadable journal must not look like no denials.
    audit_logs=$(ipe_read_journal "$boot_id" audit "$audit_cursor") || return 1
    kernel_logs=$(ipe_read_journal "$boot_id" kernel "$kernel_cursor") || return 1
    driver_logs=$(ipe_read_journal "$boot_id" driver "$driver_cursor") || return 1
    logs="$audit_logs"$'\n'"$kernel_logs"$'\n'"$driver_logs"
    current_boot=$(cat /proc/sys/kernel/random/boot_id) || return 1
    ipe_check_audit_window "$boot_id" "$current_boot" "$logs" >&2 || {
        ipe_fail "boot changed or audit records were lost"
        return 1
    }
    printf '%s\n' "$logs"
}

ipe_check_execution() {
    local boot_id="$1" probe="$2" usr_probe options usr_pid probe_pid
    local audit_cursor kernel_cursor driver_cursor logs attempt
    usr_probe=$(readlink -e /usr/bin/true)
    [[ "$usr_probe" == /usr/* ]] || ipe_fail "base executable is not on /usr"
    options=$(findmnt -rn -o OPTIONS -T "$probe")
    [[ ",$options," != *,noexec,* ]] || ipe_fail "probe storage is noexec"
    # Same executable bytes, different filesystem trust.
    cp "$usr_probe" "$probe"
    chmod 0755 "$probe"

    audit_cursor=$(ipe_journal_cursor "$boot_id" audit)
    kernel_cursor=$(ipe_journal_cursor "$boot_id" kernel)
    driver_cursor=$(ipe_journal_cursor "$boot_id" driver)
    [[ -n "$kernel_cursor" ]] || ipe_fail "kernel journal has no audit-loss observation cursor"
    [[ "$(cat /proc/sys/kernel/random/boot_id)" == "$boot_id" ]]

    # exec preserves the child PID captured by $!, allowing exact audit correlation.
    (exec "$usr_probe") &
    usr_pid=$!
    wait "$usr_pid" || ipe_fail "/usr execution failed"
    printf 'Base executable exited 0: pid=%s path=%s\n' "$usr_pid" "$usr_probe"
    (exec "$probe") &
    probe_pid=$!
    wait "$probe_pid" || ipe_fail "writable execution was blocked in audit mode"
    printf 'Writable probe exited 0: pid=%s path=%s\n' "$probe_pid" "$probe"

    # The writable denial proves logging worked. Wait for it before judging /usr.
    for ((attempt=0; attempt<15; attempt++)); do
        logs=$(ipe_collect_audit_logs "$boot_id" "$audit_cursor" "$kernel_cursor" "$driver_cursor")
        if ipe_denial "$probe_pid" "$probe" <<< "$logs"; then break; fi
        sleep 1
    done
    ipe_denial "$probe_pid" "$probe" <<< "$logs" || ipe_fail "missing fresh IPE denial for the writable probe"

    # Allow delayed records from the earlier /usr execution to arrive.
    sleep 2
    logs=$(ipe_collect_audit_logs "$boot_id" "$audit_cursor" "$kernel_cursor" "$driver_cursor")
    ipe_denial "$probe_pid" "$probe" <<< "$logs" || ipe_fail "probe record disappeared"
    ipe_check_usr_audit "$usr_pid" <<< "$logs" || ipe_fail "/usr execution audit check failed"
    printf 'No IPE denial for the base executable or its dependencies: pid=%s path=%s\n' "$usr_pid" "$usr_probe"
}

ipe_cleanup() {
    local status="$1" probe="$2"
    if [[ -n "$probe" ]] && ! rm -f -- "$probe"; then
        echo "Could not remove IPE probe $probe" >&2
        status=1
    fi
    if [[ "$status" != 0 ]]; then
        journalctl -b --no-pager -n 500 -u acl-ipe-load.service -u containerd.service ||
            echo "Could not collect IPE/containerd diagnostics" >&2
    fi
    exit "$status"
}

# Unit tests source the helpers without running the live test below.
if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
fi

# Live test flow. The trap owns the temporary file, including on early failures.
boot_id=$(cat /proc/sys/kernel/random/boot_id)
probe=""
trap 'ipe_cleanup "$?" "$probe"' EXIT

echo "Checking IPE audit mode (boot=$boot_id)"
ipe_check_audit_mode /sys/kernel/security/ipe || ipe_fail "expected an active policy with enforcement off"

echo "Checking signed /usr"
ipe_check_signed_usr

echo "Checking EROFS runtime configuration"
ipe_check_erofs_runtime

echo "Checking base and writable execution"
probe=$(mktemp /var/tmp/acl-ipe-audit.XXXXXX)
ipe_check_execution "$boot_id" "$probe"

ipe_check_audit_mode /sys/kernel/security/ipe || ipe_fail "policy state changed during the probe"
echo "PASS: functional IPE audit integration (not full signature or host-trust qualification)"
