#!/bin/bash
# Ordinary ACL nodes should keep overlayfs when the IPE profile is not requested.
# Inspect the running daemon's effective config, not a potentially unused file.
set -euo pipefail

fail() {
    printf 'ACL snapshotter validation failed: %s\n' "$*" >&2
    exit 1
}

acl_check_overlayfs_config() {
    # Match the CRI image default, not a runtime-specific snapshotter override.
    awk '
        { gsub(/\047/, "\""); sub(/^[[:space:]]+/, ""); sub(/[[:space:]]+$/, "") }
        /^\[/ { section = $0 }
        section == "[plugins.\"io.containerd.cri.v1.images\"]" &&
            /^snapshotter =/ { found = 1; actual = $0; overlayfs = ($0 == "snapshotter = \"overlayfs\"") }
        END {
            if (!found) {
                print "Missing CRI images snapshotter; expected overlayfs" > "/dev/stderr"
                exit 1
            }
            if (!overlayfs) {
                print "Expected CRI images snapshotter = \"overlayfs\", got: " actual > "/dev/stderr"
                exit 1
            }
        }
    '
}

# Unit tests source the predicate without inspecting a live daemon.
if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
fi

systemctl is-active --quiet containerd || fail "containerd is not active"
pid=$(systemctl show containerd --property=MainPID --value) || fail "cannot read containerd MainPID"
[[ "$pid" =~ ^[1-9][0-9]*$ ]] || fail "expected a positive containerd PID, got '$pid'"
binary=$(readlink -e "/proc/$pid/exe") || fail "cannot resolve containerd executable for PID $pid"
mapfile -d '' -t args < "/proc/$pid/cmdline" || fail "cannot read containerd arguments for PID $pid"
[[ "${#args[@]}" -gt 0 ]] || fail "containerd command line is empty for PID $pid"

# No flag means containerd uses its standard configuration path.
config=/etc/containerd/config.toml
for ((i=1; i<${#args[@]}; i++)); do
    case "${args[i]}" in
        --config|-c)
            i=$((i + 1))
            [[ "$i" -lt "${#args[@]}" ]] || fail "containerd config flag has no path"
            config="${args[i]}"
            ;;
        --config=*|-c=*) config="${args[i]#*=}" ;;
        -c?*) config="${args[i]#-c}" ;;
        --) break ;;
    esac
done

[[ -r "$config" ]] || fail "containerd config is not readable: '$config'"
dump=$("$binary" --config "$config" config dump) || fail "cannot dump containerd config: '$config'"
printf 'Running containerd config: %s\n%s\n' "$config" "$dump"
acl_check_overlayfs_config <<< "$dump"
