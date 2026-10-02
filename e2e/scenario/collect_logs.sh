#!/usr/bin/env bash
set -euo pipefail
umask 077

duration=$1
kill_after=$2
shift 2
for tool in timeout tar gzip; do
    command -v "$tool" >/dev/null || { printf 'missing diagnostic tool: %s\n' "$tool" >&2; exit 127; }
done
work_dir=$(mktemp -d)
trap 'rm -f -- "$work_dir"/*; rmdir -- "$work_dir"' EXIT

collect() {
    local index=$1 command=$2 status=0
    timeout --kill-after="$kill_after" "$duration" bash -c "$command" \
        >"$work_dir/$index.stdout" 2>"$work_dir/$index.stderr" || status=$?
    printf '%s' "$status" >"$work_dir/$index.exit"
}

index=0
for command in "$@"; do
    collect "$index" "$command" &
    index=$((index + 1))
done
wait
tar -cf - -C "$work_dir" . | gzip
