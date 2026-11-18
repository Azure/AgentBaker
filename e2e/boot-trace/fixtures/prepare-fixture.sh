#!/bin/bash
set +x
set -euo pipefail

target=/opt/aks-node-boot-trace/update-ca-certificates-rs
for component in kubelet kubectl; do
    if ! find "/opt/$component/downloads" -type f -path '*1.36.5*' -print -quit | grep -q .; then
        echo "ERROR: required VHD-cached $component 1.36.5 is missing" >&2
        exit 1
    fi
done
if [ -e "$target" ]; then
    printf '%s  %s\n' '@RUST_SHA256@' "$target" | sha256sum --check --status || {
        echo 'ERROR: existing Rust fixture does not match the pinned SHA-256' >&2
        exit 1
    }
    exit 0
fi
temporary=$(mktemp)
trap 'rm -f -- "$temporary"' EXIT
token=$(curl --fail --silent --show-error --noproxy '*' --retry 5 --retry-all-errors --connect-timeout 5 --max-time 15 \
    -H Metadata:true 'http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https%3A%2F%2Fstorage.azure.com%2F&mi_res_id=@IDENTITY_RESOURCE_ID@' |
    jq -er '.access_token | strings')
printf 'header = "Authorization: Bearer %s"\n' "$token" |
    curl --config - --fail --silent --show-error --retry 5 --retry-all-errors --retry-delay 3 --connect-timeout 10 --max-time 30 \
        -H 'x-ms-version: 2023-11-03' '@RUST_URL@' -o "$temporary"
unset token
printf '%s  %s\n' '@RUST_SHA256@' "$temporary" | sha256sum --check --status || {
    echo 'ERROR: downloaded Rust fixture does not match the pinned SHA-256' >&2
    exit 1
}
install -m 0755 "$temporary" "$target"
echo 'BOOT_TRACE_PINNED_RUST_FIXTURE_READY'
