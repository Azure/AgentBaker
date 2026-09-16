#!/bin/bash

anc_hotfix_log() {
    local message="$1"
    logger -t "${LOGGER_TAG:-aks-node-controller-launcher}" "$message"
    echo "$message"
}

anc_hotfix_read_feature_flags() {
    local features_path="$1"

    if [ ! -f "$features_path" ]; then
        return 0
    fi

    anc_hotfix_log "Reading feature flags from ${features_path}"
    while IFS='=' read -r _key _val || [ -n "$_key" ]; do
        case "$_key" in
        ''|\#*) continue ;;
        [!a-zA-Z_]*|*[!a-zA-Z0-9_]*) continue ;;
        esac
        export "${_key}=${_val}"
    done <"$features_path"
}

anc_run_hotfix_flow() {
    local bin_path="$1"
    local hotfix_bin="$2"
    local hotfix_json="$3"
    local features_path="$4"

    anc_hotfix_read_feature_flags "$features_path"

    # check-hotfix refreshes the on-disk hotfix pointer (its own default path, mirrored by
    # $hotfix_json) that download-hotfix reads below, so it must run first. Gated default-off
    # behind ENABLE_PROVISIONING_HOTFIX (only the literal "true" enables it) - the on-node
    # terminal of the EnableProvisioningHotfix aks-rp region toggle. Wrapped defensively: it is
    # fail-open, but an older ANC binary predating the subcommand exits non-zero.
    if [ "${ENABLE_PROVISIONING_HOTFIX:-}" = "true" ]; then
        anc_hotfix_log "ENABLE_PROVISIONING_HOTFIX=true; running check-hotfix to refresh hotfix pointer"
        if "$bin_path" check-hotfix; then
            anc_hotfix_log "ANC check-hotfix completed; hotfix pointer refresh attempted"
        else
            anc_hotfix_log "ANC check-hotfix failed; continuing (fail-open)"
        fi
    fi

    if [ -f "$hotfix_json" ]; then
        anc_hotfix_log "Found ANC hotfix config at ${hotfix_json}; running download-hotfix"
        if "$bin_path" download-hotfix; then
            anc_hotfix_log "ANC download-hotfix completed; binary selection follows"
        else
            anc_hotfix_log "ANC download-hotfix failed; binary selection follows"
        fi
    fi

    ANC_HOTFIX_SELECTED_BIN="$bin_path"
    if [ -x "$hotfix_bin" ]; then
        ANC_HOTFIX_SELECTED_BIN="$hotfix_bin"
        anc_hotfix_log "Using hotfix binary: $hotfix_bin"
    else
        anc_hotfix_log "Using VHD-baked binary: $bin_path"
    fi

    if [ -x "$hotfix_bin" ]; then
        if "$hotfix_bin" apply-embedded-hotfix; then
            anc_hotfix_log "ANC apply-embedded-hotfix completed"
        else
            anc_hotfix_log "ANC apply-embedded-hotfix failed"
        fi
    fi

    export ANC_HOTFIX_SELECTED_BIN
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    BIN_PATH="${BIN_PATH:-/opt/azure/containers/aks-node-controller}"
    HOTFIX_BIN="${HOTFIX_BIN:-${BIN_PATH}-hotfix}"
    HOTFIX_JSON="${HOTFIX_JSON:-/opt/azure/containers/aks-node-controller-hotfix.json}"
    FEATURES_PATH="${FEATURES_PATH:-/opt/azure/containers/enabled_features.sh}"
    anc_run_hotfix_flow "$BIN_PATH" "$HOTFIX_BIN" "$HOTFIX_JSON" "$FEATURES_PATH"
    printf '%s\n' "$ANC_HOTFIX_SELECTED_BIN"
fi
