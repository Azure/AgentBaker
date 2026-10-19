#!/usr/bin/env bash
#
# localDNS live-patching handler, shared by the Ubuntu and Azure Linux reconcilers.
#
# Unlike securityPatch, nothing here is OS-specific: the payload schema and the Corefile render are
# delegated to aks-node-controller. This file therefore lives once and is sourced by both reconcilers
# rather than copied into each, so the outcome semantics below -- in particular that noCorefileData is
# a success and notFound is not -- cannot drift between the two.
#
# The VHD build bakes this file next to both reconcilers, which source it unconditionally like
# security-update.sh. A VHD baked before this file shipped also predates the reconciler's localDNS
# case, so its "ignore unknown components" rule skips the component there.
#
# How a patch reaches CoreDNS
# ---------------------------
# CoreDNS serves updated.localdns.corefile. localdns.sh builds it on every start from a source
# Corefile -- livepatched.localdns.corefile when present, else the baked localdns.corefile -- by
# replacing the Azure DNS placeholder (168.63.129.16) with the VNet DNS servers.
#
# This handler builds that same file itself and lets CoreDNS's reload plugin pick it up, instead of
# restarting localdns.service. A restart reruns localdns.sh, which tears down and rebuilds the node's
# DNS plumbing (a window with no DNS), and would also rerun its bootstrap-only steps. The VNet DNS
# servers come from the record localdns.sh wrote when it last started, not from resolv.conf, which
# lists the localdns listener while localdns runs -- so a patch never changes the forward servers the
# node is already using, only what the profile changed.
#
# Restarting stays the fallback whenever the in-place path cannot be carried out or verified: no
# usable forward-server record, a running Corefile without the reload plugin, CoreDNS metrics that
# cannot be read, or a reload that never shows up.

# The live-patched source Corefile. aks-node-controller writes it (plus a .version sidecar); localdns.sh
# prefers it over the baked Corefile on every start, so it is what makes a patch survive a reboot.
LOCALDNS_LIVEPATCHED_COREFILE="${LOCALDNS_LIVEPATCHED_COREFILE:-/opt/azure/containers/localdns/livepatched.localdns.corefile}"
# The Corefile localdns.sh regenerates from CustomData on every start; served again once a patch is revoked.
LOCALDNS_BAKED_COREFILE="${LOCALDNS_BAKED_COREFILE:-/opt/azure/containers/localdns/localdns.corefile}"
# The Corefile CoreDNS runs with (coredns -conf ...).
LOCALDNS_SERVED_COREFILE="${LOCALDNS_SERVED_COREFILE:-/opt/azure/containers/localdns/updated.localdns.corefile}"
# Written by localdns.sh (replace_azurednsip_in_corefile): what 168.63.129.16 resolved to on its last start.
LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE="${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE:-/run/localdns/vnet-dns-forward-servers}"
LOCALDNS_AKS_NODE_CONTROLLER="${LOCALDNS_AKS_NODE_CONTROLLER:-/opt/azure/containers/aks-node-controller}"
# CoreDNS's prometheus plugin (prometheus :9253 in every server block of the template).
LOCALDNS_METRICS_URL="${LOCALDNS_METRICS_URL:-http://127.0.0.1:9253/metrics}"
# The reload plugin checks every 30s +/- 7.5s of jitter, so a change lands within ~38s.
LOCALDNS_RELOAD_TIMEOUT_SECONDS="${LOCALDNS_RELOAD_TIMEOUT_SECONDS:-90}"
LOCALDNS_RELOAD_POLL_SECONDS="${LOCALDNS_RELOAD_POLL_SECONDS:-2}"
# Why this run's localDNS patch failed, reported as the component's "message" in the live-patching
# status. updateLocalDNS clears it on entry and sets it on every path that fails the patch.
LOCALDNS_FAILURE_MESSAGE=""

# Prints this node's agent pool from its Node object (the reconciler's node_json). The is-current
# check and the apply both take the pool from here, so they always select the same entry, and it
# works on every provisioning mode -- unlike the provisioning files aks-node-controller falls back to,
# which classic-CSE nodes do not have.
localDNSNodeAgentPool() {
    local node_json="$1"

    printf '%s' "${node_json}" | jq -er '.metadata.labels["kubernetes.azure.com/agentpool"] // empty' 2> /dev/null
}

# Prints this node's localDNS version from a payload: the corefileVersion of this pool's entry in
# agentPools, the value aks-node-controller records in the .version file. The RP computes it as a hash
# of the pool's LocalDNS profile, so it changes exactly when this pool's config does. Prints
# {"absent":true} when the payload has no entry for this pool, including a payload with no agentPools
# at all. Fails on an entry without a corefileVersion or a payload aks-node-controller would reject,
# so the caller runs the handler instead of skipping it.
localDNSAgentPoolVersion() {
    local payload="$1"
    local agent_pool="$2"

    printf '%s' "${payload}" | jq -ce --arg agentPool "${agent_pool}" '
        if type != "object" or ((.agentPools // {}) | type) != "object" then
            error("localDNS payload must be an object with an agentPools object")
        else
            (.agentPools // {})[$agentPool] as $entry
            | if $entry == null then {absent: true}
              elif ($entry | type) == "object" and ($entry.corefileVersion | type) == "string" and $entry.corefileVersion != "" then
                  {version: $entry.corefileVersion}
              else error("localDNS agent pool entry has no corefileVersion")
              end
        end' 2> /dev/null
}

# The payload carries every agent pool, so it changes whenever any pool's localDNS config changes.
# Compare only this node's pool, by its corefileVersion: a change to another pool must not run the
# handler here, where it can still restart localdns (no usable forward-server record) even though
# this node's Corefile would not change. A pool that disappears from the payload still differs
# (version -> absent), so the handler runs and revokes the earlier patch. Without a version to
# compare, the handler runs; it checks what is applied on disk and is a no-op when nothing changed.
localDNSIsCurrent() {
    local desired_payload="$1"
    local current_payload="$2"
    local node_json="$3"
    local agent_pool
    local desired_version
    local current_version

    if ! agent_pool="$(localDNSNodeAgentPool "${node_json}")"; then
        return 1
    fi
    if ! desired_version="$(localDNSAgentPoolVersion "${desired_payload}" "${agent_pool}")"; then
        return 1
    fi
    if ! current_version="$(localDNSAgentPoolVersion "${current_payload}" "${agent_pool}")"; then
        return 1
    fi

    [ "${desired_version}" = "${current_version}" ]
}

# Prints the VNet DNS forward servers localdns.sh recorded on its last start. Fails when the record is
# missing or unusable, so the caller falls back to a restart that redoes the substitution.
localDNSForwardServers() {
    local servers
    local server

    if [ ! -s "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}" ]; then
        return 1
    fi
    servers="$(tr -s '[:space:]' ' ' < "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}")"
    servers="${servers# }"
    servers="${servers% }"
    if [ -z "${servers}" ]; then
        return 1
    fi
    for server in ${servers}; do
        case "${server}" in
            169.254.10.10|169.254.10.11)
                # Forwarding to localdns's own listener would loop.
                return 1
                ;;
        esac
        if ! printf '%s\n' "${server}" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}$|^[0-9A-Fa-f:]*:[0-9A-Fa-f:]*$'; then
            return 1
        fi
    done
    printf '%s' "${servers}"
}

# Builds the Corefile CoreDNS should serve from a source Corefile. Same substitution as
# replace_azurednsip_in_corefile in localdns.sh, with the servers it recorded.
localDNSRenderServedCorefile() {
    local source_corefile="$1"
    local output="$2"
    local forward_servers="$3"

    sed -e "s|168.63.129.16|${forward_servers}|g" "${source_corefile}" > "${output}"
}

localDNSReloadMetrics() {
    curl -s --noproxy '*' --connect-timeout 2 --max-time 5 "${LOCALDNS_METRICS_URL}"
}

# Reads coredns_reload_failed_total from metrics text on stdin.
localDNSReloadFailedCount() {
    awk '$1 == "coredns_reload_failed_total" { print $2; found = 1 } END { if (!found) exit 1 }'
}

# Reads the hashes in coredns_reload_version_info from metrics text on stdin.
localDNSReloadVersions() {
    sed -n 's/^coredns_reload_version_info{.*value="\([0-9a-fA-F]*\)".*/\1/p' | sort
}

# Waits for CoreDNS's reload plugin to act on a changed served Corefile.
# Returns 0 when it reloaded, 1 when it rejected the Corefile, 2 when no reload showed up in time.
#
# The plugin hashes the parsed Corefile, not the file bytes, so the shell cannot predict the new hash.
# Instead it watches for the published version to change. The plugin publishes the new hash before
# it bumps coredns_reload_failed_total on a failed reload, so the failure counter is re-read after a
# short settle before a version change counts as success.
localDNSWaitForReload() {
    local failed_before="$1"
    local versions_before="$2"
    local deadline=$((SECONDS + LOCALDNS_RELOAD_TIMEOUT_SECONDS))
    local metrics
    local failed
    local versions

    while [ "${SECONDS}" -lt "${deadline}" ]; do
        sleep "${LOCALDNS_RELOAD_POLL_SECONDS}"
        metrics="$(localDNSReloadMetrics)" || continue
        failed="$(printf '%s\n' "${metrics}" | localDNSReloadFailedCount)" || continue
        if [ "${failed}" != "${failed_before}" ]; then
            return 1
        fi
        versions="$(printf '%s\n' "${metrics}" | localDNSReloadVersions)"
        if [ "${versions}" = "${versions_before}" ]; then
            continue
        fi
        sleep "${LOCALDNS_RELOAD_POLL_SECONDS}"
        if metrics="$(localDNSReloadMetrics)" && failed="$(printf '%s\n' "${metrics}" | localDNSReloadFailedCount)" &&
            [ "${failed}" != "${failed_before}" ]; then
            return 1
        fi
        return 0
    done
    return 2
}

localDNSRestart() {
    if ! systemctl restart localdns.service; then
        echo "failed to restart localdns.service"
        return 1
    fi
    echo "restarted localdns.service"
}

# Makes CoreDNS serve the Corefile built from source_corefile.
# Returns 0 once it is served (or will be on the next start), 1 when CoreDNS rejected it and is still
# serving the previous Corefile, and 2 when a fallback restart failed and localdns may be down.
localDNSServe() {
    local source_corefile="$1"
    local forward_servers
    local desired
    local metrics
    local failed_before
    local versions_before
    local reload_result

    if ! systemctl is-active --quiet localdns.service; then
        echo "localdns.service is not running; its next start builds the served corefile from ${source_corefile}"
        return 0
    fi
    if [ ! -s "${source_corefile}" ]; then
        echo "source corefile ${source_corefile} is missing; restarting localdns.service to rebuild it"
        localDNSRestart || return 2
        return 0
    fi
    if ! forward_servers="$(localDNSForwardServers)"; then
        echo "no usable VNet DNS forward servers recorded at ${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}; restarting localdns.service to apply the corefile"
        localDNSRestart || return 2
        return 0
    fi

    # Render next to the served Corefile so the final rename is atomic: the reload plugin must never
    # read a half-written file.
    if ! desired="$(mktemp "${LOCALDNS_SERVED_COREFILE}.XXXXXX")"; then
        echo "failed to create a temp file next to ${LOCALDNS_SERVED_COREFILE}"
        LOCALDNS_FAILURE_MESSAGE="could not create a temp file for the served corefile"
        return 1
    fi
    if ! localDNSRenderServedCorefile "${source_corefile}" "${desired}" "${forward_servers}"; then
        rm -f "${desired}"
        echo "failed to render the served localDNS corefile"
        LOCALDNS_FAILURE_MESSAGE="could not render the served corefile"
        return 1
    fi
    if cmp -s "${desired}" "${LOCALDNS_SERVED_COREFILE}"; then
        rm -f "${desired}"
        echo "localDNS served corefile is already current"
        return 0
    fi

    # The template's reload plugin is the bare `reload` line; the hosts plugin's `reload 5s` is a
    # different thing. Without it the running CoreDNS never looks at the file again.
    if ! grep -Eq '^[[:space:]]*reload[[:space:]]*$' "${LOCALDNS_SERVED_COREFILE}" 2>/dev/null; then
        rm -f "${desired}"
        echo "running localDNS corefile has no reload plugin; restarting localdns.service to apply the corefile"
        localDNSRestart || return 2
        return 0
    fi
    if ! metrics="$(localDNSReloadMetrics)" || ! failed_before="$(printf '%s\n' "${metrics}" | localDNSReloadFailedCount)"; then
        rm -f "${desired}"
        echo "CoreDNS reload metrics are unavailable at ${LOCALDNS_METRICS_URL}; restarting localdns.service to apply the corefile"
        localDNSRestart || return 2
        return 0
    fi
    versions_before="$(printf '%s\n' "${metrics}" | localDNSReloadVersions)"

    if ! chmod 0644 "${desired}" || ! mv -f "${desired}" "${LOCALDNS_SERVED_COREFILE}"; then
        rm -f "${desired}"
        echo "failed to write ${LOCALDNS_SERVED_COREFILE}"
        LOCALDNS_FAILURE_MESSAGE="could not write the served corefile"
        return 1
    fi
    echo "wrote ${LOCALDNS_SERVED_COREFILE}; waiting for CoreDNS to reload it"

    localDNSWaitForReload "${failed_before}" "${versions_before}"
    reload_result=$?
    case "${reload_result}" in
        0)
            echo "CoreDNS reloaded the localDNS corefile"
            return 0
            ;;
        1)
            echo "CoreDNS rejected the localDNS corefile and kept serving the previous one"
            LOCALDNS_FAILURE_MESSAGE="CoreDNS rejected the new corefile and kept serving the previous one"
            return 1
            ;;
        *)
            echo "CoreDNS did not reload the localDNS corefile within ${LOCALDNS_RELOAD_TIMEOUT_SECONDS}s; restarting localdns.service to apply it"
            localDNSRestart || return 2
            return 0
            ;;
    esac
}

# Reports this node's localDNS "current" for the live-patching status: the corefileVersion of the
# last patch applied here, from the .version file aks-node-controller writes with it. A failed patch
# restores the previous file and a revoke deletes it, so this is always what the node serves from.
# Prints nothing when no live patch is applied, which leaves "current" out of the status.
localDNSCurrent() {
    local version_file="${LOCALDNS_LIVEPATCHED_COREFILE}.version"

    if [ ! -s "${version_file}" ]; then
        return 0
    fi
    tr -d '[:space:]' < "${version_file}"
}

# Reports why this run's localDNS patch failed, for the "message" field of the live-patching status.
# Prints nothing when the patch succeeded or did not run, which leaves "message" out.
localDNSFailureMessage() {
    printf '%.512s' "${LOCALDNS_FAILURE_MESSAGE:-}"
}

# The files a patch touches, in a fixed order so backups map back to them by index.
localDNSPatchedFiles() {
    printf '%s\n' "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
}

localDNSBackup() {
    local backup_dir="$1"
    local index=0
    local file

    while IFS= read -r file; do
        if [ -e "${file}" ] && ! cp -p "${file}" "${backup_dir}/${index}"; then
            return 1
        fi
        index=$((index + 1))
    done < <(localDNSPatchedFiles)
}

# Puts every patched file back the way it was before this run, removing files that did not exist.
# The served Corefile goes back through a rename so the reload plugin only ever sees whole files.
localDNSRestore() {
    local backup_dir="$1"
    local index=0
    local file
    local tmp

    while IFS= read -r file; do
        if [ -e "${backup_dir}/${index}" ]; then
            if tmp="$(mktemp "${file}.XXXXXX")" && cp -p "${backup_dir}/${index}" "${tmp}" && mv -f "${tmp}" "${file}"; then
                :
            else
                rm -f "${tmp:-}"
                echo "failed to restore ${file}"
            fi
        else
            rm -f "${file}"
        fi
        index=$((index + 1))
    done < <(localDNSPatchedFiles)
}

updateLocalDNS() {
    local component_payload="${1:-}"
    local node_json="${2:-}"
    local agent_pool
    local apply_args
    local outcome
    local apply_outcome
    local apply_error
    local source_corefile
    local backup_dir
    local serve_result

    LOCALDNS_FAILURE_MESSAGE=""

    if [ ! -x "${LOCALDNS_AKS_NODE_CONTROLLER}" ]; then
        echo "aks-node-controller binary is required for localDNS live patching"
        LOCALDNS_FAILURE_MESSAGE="aks-node-controller binary not found"
        return 1
    fi
    if ! backup_dir="$(mktemp -d)"; then
        echo "failed to create a backup directory for localDNS live patching"
        LOCALDNS_FAILURE_MESSAGE="could not create a backup directory"
        return 1
    fi
    if ! localDNSBackup "${backup_dir}"; then
        rm -rf "${backup_dir}"
        echo "failed to back up the current localDNS corefiles"
        LOCALDNS_FAILURE_MESSAGE="could not back up the current localDNS corefiles"
        return 1
    fi

    apply_args=(apply-localdns-config --config-file - --output "${LOCALDNS_LIVEPATCHED_COREFILE}")
    # Without the label, aks-node-controller falls back to the node's provisioning files.
    if agent_pool="$(localDNSNodeAgentPool "${node_json}")"; then
        apply_args+=(--agent-pool "${agent_pool}")
    fi
    if ! outcome="$(printf '%s' "${component_payload}" | "${LOCALDNS_AKS_NODE_CONTROLLER}" "${apply_args[@]}")"; then
        echo "localDNS config apply failed"
        [ -z "${outcome}" ] || printf '%s\n' "${outcome}"
        # aks-node-controller prints the reason as "error: <reason>" before its outcome line.
        apply_error="$(printf '%s\n' "${outcome}" | sed -n 's/^error: //p' | tail -n 1)"
        LOCALDNS_FAILURE_MESSAGE="aks-node-controller apply-localdns-config failed${apply_error:+: ${apply_error}}"
        localDNSRestore "${backup_dir}"
        rm -rf "${backup_dir}"
        return 1
    fi
    printf '%s\n' "${outcome}"
    apply_outcome="$(printf '%s\n' "${outcome}" | tail -n 1)"

    case "${apply_outcome}" in
        applied|alreadyCurrent)
            source_corefile="${LOCALDNS_LIVEPATCHED_COREFILE}"
            ;;
        noCorefileData)
            # Nothing in this payload applies to this node: either no pool in the cluster has
            # localDNS enabled, or this node's pool does not. That is an authoritative answer from the
            # control plane, not a failed apply, so it succeeds -- reporting failure would peg those
            # nodes at localDNS=Failed and re-dispatch every component on every tick. It also revokes
            # any earlier patch, the same way the bootstrap path does, so the node serves its baked
            # Corefile again instead of a config the control plane no longer publishes.
            echo "localDNS has no config applicable to this node; serving the baked corefile"
            rm -f "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            source_corefile="${LOCALDNS_BAKED_COREFILE}"
            ;;
        notFound)
            # In the runtime apply path the payload is supplied inline, so notFound is not expected;
            # treat it as a failure so the reconciler does not checkpoint an unapplied config as succeeded.
            echo "localDNS LPS config is not available"
            LOCALDNS_FAILURE_MESSAGE="unexpected apply outcome notFound for an inline payload"
            localDNSRestore "${backup_dir}"
            rm -rf "${backup_dir}"
            return 1
            ;;
        *)
            echo "unexpected localDNS apply outcome: ${outcome}"
            LOCALDNS_FAILURE_MESSAGE="unexpected apply outcome: ${apply_outcome}"
            localDNSRestore "${backup_dir}"
            rm -rf "${backup_dir}"
            return 1
            ;;
    esac

    localDNSServe "${source_corefile}"
    serve_result=$?
    case "${serve_result}" in
        0)
            echo "localDNS update completed successfully"
            rm -rf "${backup_dir}"
            return 0
            ;;
        1)
            # CoreDNS is still serving the previous Corefile. Put the files back so they describe it
            # again; the reload plugin sees the served file change and settles on it.
            : "${LOCALDNS_FAILURE_MESSAGE:=could not serve the new corefile}"
            localDNSRestore "${backup_dir}"
            rm -rf "${backup_dir}"
            return 1
            ;;
        *)
            # A fallback restart failed, so localdns may be down on the new Corefile. Put the previous
            # files back and start it on those; anything further is left to the unit's own
            # Restart=/OnFailure= handling.
            LOCALDNS_FAILURE_MESSAGE="localdns.service restart failed; restored the previous corefile"
            localDNSRestore "${backup_dir}"
            rm -rf "${backup_dir}"
            if ! systemctl restart localdns.service; then
                echo "failed to restart localdns.service on the restored corefile"
            fi
            return 1
            ;;
    esac
}
