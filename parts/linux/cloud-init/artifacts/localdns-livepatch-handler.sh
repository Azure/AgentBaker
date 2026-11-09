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
#
# A patch also sets localdns.slice's CPU and memory limits (localdns-limits.sh), since the
# corefileVersion covers them too. updateLocalDNS runs the same sequence as localdns.sh at bootstrap:
#   0 preflight  systemctl responds, the baked limits parse, record the running limits
#   1 backup     the live-patched Corefile, its .version and the served Corefile
#   2 render     aks-node-controller -> rendered | alreadyCurrent; on rendered it has removed .version
#                (uncommit) before writing the new Corefile
#   3 limits     skip a limit already in effect, else set-property it; verify kernel and drop-ins
#   4 serve      reload (or restart) CoreDNS onto the new Corefile; skipped on alreadyCurrent. Left to
#                the next run when a localdns start is in progress, since it may serve the old one
#   5 commit     write .version
#   6 ready      CoreDNS /ready decides the result; .version stays either way
# A failure before the commit undoes the run: the limits go back to the ones recorded in step 0 and
# the files to the backup. Limits go first so that no failure path ever has to switch CoreDNS back
# to an older Corefile. A failure after the commit undoes nothing; the next run checks again.

# The live-patched source Corefile. aks-node-controller writes it; its .version sidecar is written here
# once the patch is in effect. localdns.sh prefers it over the baked Corefile on every start, so it is
# what makes a patch survive a reboot.
LOCALDNS_LIVEPATCHED_COREFILE="${LOCALDNS_LIVEPATCHED_COREFILE:-/opt/azure/containers/localdns/livepatched.localdns.corefile}"
# The Corefile localdns.sh regenerates from CustomData on every start; served again once a patch is revoked.
LOCALDNS_BAKED_COREFILE="${LOCALDNS_BAKED_COREFILE:-/opt/azure/containers/localdns/localdns.corefile}"
# The Corefile CoreDNS runs with (coredns -conf ...).
LOCALDNS_SERVED_COREFILE="${LOCALDNS_SERVED_COREFILE:-/opt/azure/containers/localdns/updated.localdns.corefile}"
# The served Corefile's forward IPs for the localdns metrics exporter (localdns_*_forward_info). localdns.sh
# writes it on every start; an in-place patch rewrites it once CoreDNS has reloaded.
LOCALDNS_FORWARD_IPS_PROM_FILE="${LOCALDNS_FORWARD_IPS_PROM_FILE:-/opt/azure/containers/localdns/forward_ips.prom}"
# Written by localdns.sh (replace_azurednsip_in_corefile): what 168.63.129.16 resolved to on its last start.
LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE="${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE:-/run/localdns/vnet-dns-forward-servers}"
LOCALDNS_AKS_NODE_CONTROLLER="${LOCALDNS_AKS_NODE_CONTROLLER:-/opt/azure/containers/aks-node-controller}"
# CoreDNS's prometheus plugin (prometheus :9253 in every server block of the template).
LOCALDNS_METRICS_URL="${LOCALDNS_METRICS_URL:-http://127.0.0.1:9253/metrics}"
# The reload plugin checks every 30s +/- 7.5s of jitter, so a change lands within ~38s.
LOCALDNS_RELOAD_TIMEOUT_SECONDS="${LOCALDNS_RELOAD_TIMEOUT_SECONDS:-90}"
LOCALDNS_RELOAD_POLL_SECONDS="${LOCALDNS_RELOAD_POLL_SECONDS:-2}"
# How long a patch waits for CoreDNS's /ready once it is committed; it checks every second, as
# localdns.sh does at startup.
LOCALDNS_READY_TIMEOUT_SECONDS="${LOCALDNS_READY_TIMEOUT_SECONDS:-60}"
# The limit functions shared with localdns.sh, installed next to it on every OS.
# Its functions log what they do on stderr, which the calls below fold into this handler's output.
LOCALDNS_LIMITS_SCRIPT="${LOCALDNS_LIMITS_SCRIPT:-/opt/azure/containers/localdns/localdns-limits.sh}"
# Why this run's localDNS patch failed, reported as the component's "message" in the live-patching
# status. updateLocalDNS clears it on entry and sets it on every path that fails the patch.
LOCALDNS_FAILURE_MESSAGE=""
# How this run served the Corefile (reload, restart, unchanged, next start), and the outcome and message of
# a run that succeeded. updateLocalDNS reports them as the run's guest agent event; a failed run reports
# LOCALDNS_FAILURE_MESSAGE instead.
LOCALDNS_SERVED_BY=""
LOCALDNS_EVENT_OUTCOME=""
LOCALDNS_EVENT_MESSAGE=""

# Sourced unconditionally like this file itself; updateLocalDNS fails the patch if it is missing.
# shellcheck source=parts/linux/cloud-init/artifacts/localdns-limits.sh
if ! source "${LOCALDNS_LIMITS_SCRIPT}"; then
    echo "failed to source ${LOCALDNS_LIMITS_SCRIPT}"
fi

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
    LOCALDNS_SERVED_BY="restart"
}

# Makes CoreDNS serve the Corefile built from source_corefile.
# Returns 0 once it is served (or will be on the next start), 1 when it is not served and the caller
# puts the previous files back (CoreDNS rejected it, or a localdns start was in progress), and 2 when
# a fallback restart failed and localdns may be down.
localDNSServe() {
    local source_corefile="$1"
    local state
    local forward_servers
    local desired
    local metrics
    local failed_before
    local versions_before
    local reload_result

    # Only a start that has not begun yet is sure to build the served Corefile from the source on disk.
    # One already running may have built it before this run wrote the source, so leave this run to the
    # next one.
    if ! state="$(systemctl show --property=ActiveState --value localdns.service)"; then
        LOCALDNS_FAILURE_MESSAGE="could not read the state of localdns.service"
        return 1
    fi
    case "${state}" in
        active|reloading)
            ;;
        activating)
            echo "localdns.service is starting; leaving the patch to the next run"
            LOCALDNS_FAILURE_MESSAGE="localdns.service was starting; the next run applies the patch"
            return 1
            ;;
        *)
            echo "localdns.service is ${state}; its next start builds the served corefile from ${source_corefile}"
            LOCALDNS_SERVED_BY="next start"
            return 0
            ;;
    esac
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
    if ! localdns_render_served_corefile "${source_corefile}" "${desired}" "${forward_servers}"; then
        rm -f "${desired}"
        echo "failed to render the served localDNS corefile"
        LOCALDNS_FAILURE_MESSAGE="could not render the served corefile"
        return 1
    fi
    if cmp -s "${desired}" "${LOCALDNS_SERVED_COREFILE}"; then
        rm -f "${desired}"
        echo "localDNS served corefile is already current"
        LOCALDNS_SERVED_BY="unchanged"
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
            # The patch may add or drop zones, or move one between VNet DNS and CoreDNS, so refresh the
            # forward_info metrics; on a restart localdns.sh does this itself.
            localdns_export_forward_ips "${LOCALDNS_SERVED_COREFILE}" "${LOCALDNS_FORWARD_IPS_PROM_FILE}"
            LOCALDNS_SERVED_BY="reload"
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
# last patch applied here, from the .version file a patch writes once its Corefile and limits are
# in effect. A failed patch restores the previous file or removes it, and a revoke deletes it, so
# this never names a version the node does not run. Prints nothing when no live patch is applied,
# which leaves "current" out of the status.
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

# The files a patch touches. Each is backed up under its own name, so a restore names the files it
# puts back and never depends on this order.
localDNSPatchedFiles() {
    printf '%s\n' "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
}

localDNSBackup() {
    local backup_dir="$1"
    local file

    while IFS= read -r file; do
        if [ -e "${file}" ] && ! cp -p "${file}" "${backup_dir}/$(basename "${file}")"; then
            return 1
        fi
    done < <(localDNSPatchedFiles)
}

# Puts the named patched files back the way they were before this run, removing a file that did not
# exist then. With no files named, it restores every file in localDNSPatchedFiles. The served
# Corefile goes back through a rename so the reload plugin only ever sees whole files.
localDNSRestore() {
    local backup_dir="$1"
    local file
    local backup
    local tmp

    shift
    if [ "$#" -eq 0 ]; then
        while IFS= read -r file; do
            set -- "$@" "${file}"
        done < <(localDNSPatchedFiles)
    fi
    for file in "$@"; do
        backup="${backup_dir}/$(basename "${file}")"
        if [ -e "${backup}" ]; then
            if tmp="$(mktemp "${file}.XXXXXX")" && cp -p "${backup}" "${tmp}" && mv -f "${tmp}" "${file}"; then
                :
            else
                rm -f "${tmp:-}"
                echo "failed to restore ${file}"
            fi
        else
            rm -f "${file}"
        fi
    done
}

# Undoes a patch that failed before its commit: sets the limits back to the ones recorded in the
# preflight and restores the patched files named after the limit values (none for a run that changed
# no file). When the limits cannot be set back, their drop-ins are removed instead and so is
# .version, since the node no longer runs any version's limits. keep_version=false removes .version
# in any case. Sets the status message, starting with failure.
localDNSUndo() {
    local backup_dir="$1"
    local keep_version="$2"
    local failure="$3"
    local previous_cpu="$4"
    local previous_memory="$5"
    local baked_cpu="$6"
    local baked_memory="$7"
    local detail
    local reverted=true

    shift 7
    if ! { detail="$(localdns_limits_revert "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}")"; } 2>&1; then
        reverted=false
    fi
    echo "localDNS limits after the undo: ${detail}"
    if [ "$#" -gt 0 ]; then
        localDNSRestore "${backup_dir}" "$@"
    fi
    if [ "${reverted}" != true ] || [ "${keep_version}" != true ]; then
        rm -f "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
    fi
    LOCALDNS_FAILURE_MESSAGE="$(localdns_limits_failure_message "${failure}" "${reverted}" "${detail}")"
}

# Revokes an earlier patch when the payload has nothing for this node: drops the live-patched
# Corefile, resets the limits to the baked ones and serves the baked Corefile. managed is false on a
# node that never had LocalDNS set up (no localdns.slice), which has no limits to reset.
localDNSRevoke() {
    local backup_dir="$1"
    local managed="$2"
    local baked_cpu="$3"
    local baked_memory="$4"
    local previous_cpu="$5"
    local previous_memory="$6"
    local limits_error
    local state
    local revoke_message=""
    local serve_result
    local revoked_version
    local had_patch=false

    echo "localDNS has no config applicable to this node; serving the baked corefile"
    if [ -e "${LOCALDNS_LIVEPATCHED_COREFILE}" ]; then
        had_patch=true
        revoked_version="$(localDNSCurrent)"
    fi
    rm -f "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
    if [ "${managed}" = true ] && ! { limits_error="$(localdns_limits_apply "${baked_cpu}" "${baked_memory}" "${baked_cpu}" "${baked_memory}")"; } 2>&1; then
        # The control plane no longer publishes a config for this node, so the Corefile revoke goes
        # ahead regardless; the limits are left for the next boot to bring back.
        state="$(localdns_limits_fall_back_to_baked "${baked_cpu}" "${baked_memory}")"
        echo "failed to reset the localDNS limits to the baked ones: ${limits_error}; ${state}"
        revoke_message="$(printf '%.512s' "resetting to the baked limits: ${limits_error}; ${state}")"
    fi

    localDNSServe "${LOCALDNS_BAKED_COREFILE}"
    serve_result=$?
    case "${serve_result}" in
        0)
            if [ -n "${revoke_message}" ]; then
                LOCALDNS_FAILURE_MESSAGE="${revoke_message}"
                return 1
            fi
            echo "localDNS update completed successfully"
            if [ "${had_patch}" = true ]; then
                LOCALDNS_EVENT_OUTCOME="Revoked"
                LOCALDNS_EVENT_MESSAGE="corefileVersion=${revoked_version:-uncommitted} revoked; serving the baked corefile; served=${LOCALDNS_SERVED_BY}"
            else
                LOCALDNS_EVENT_OUTCOME="NoAction"
                LOCALDNS_EVENT_MESSAGE="no localDNS config for this node"
            fi
            return 0
            ;;
        1)
            if [ "${managed}" = true ]; then
                localDNSUndo "${backup_dir}" true "${LOCALDNS_FAILURE_MESSAGE:-could not serve the baked corefile}" \
                    "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}" \
                    "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
            else
                : "${LOCALDNS_FAILURE_MESSAGE:=could not serve the baked corefile}"
                localDNSRestore "${backup_dir}"
            fi
            return 1
            ;;
        *)
            if [ "${managed}" = true ]; then
                localDNSUndo "${backup_dir}" true "localdns.service restart failed" \
                    "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}" \
                    "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
            else
                LOCALDNS_FAILURE_MESSAGE="localdns.service restart failed; restored the previous corefile"
                localDNSRestore "${backup_dir}"
            fi
            if ! systemctl restart localdns.service; then
                echo "failed to restart localdns.service on the restored corefile"
            fi
            return 1
            ;;
    esac
}

# Guest agent event for a localDNS run, AKS.LivePatching.localDNS.<outcome>, the convention securityPatch
# and npdConfig follow. localdns.sh writes the same event for the bootstrap (LPS) path; the message's
# path= tells them apart. A run outside the reconciler has no knead_emit_event and writes none.
knead_emit_localdns_event() {
    local outcome="$1"
    local message="$2"
    local level="${3:-Informational}"

    if ! declare -F knead_emit_event > /dev/null; then
        return 0
    fi
    if ! knead_emit_event "AKS.LivePatching.localDNS.${outcome}" "path=runtime ${message}" "${level}"; then
        echo "failed to write the localDNS ${outcome} event"
    fi
}

# The reconciler's localDNS handler: applies the payload (localDNSUpdate) and reports the run as one
# guest agent event.
updateLocalDNS() {
    local status=0

    LOCALDNS_SERVED_BY=""
    LOCALDNS_EVENT_OUTCOME=""
    LOCALDNS_EVENT_MESSAGE=""
    localDNSUpdate "$@" || status=$?
    if [ "${status}" -ne 0 ]; then
        knead_emit_localdns_event Failed "${LOCALDNS_FAILURE_MESSAGE:-the localDNS update failed}" Error
    else
        knead_emit_localdns_event "${LOCALDNS_EVENT_OUTCOME:-Succeeded}" "${LOCALDNS_EVENT_MESSAGE}"
    fi
    return "${status}"
}

localDNSUpdate() {
    local component_payload="${1:-}"
    local node_json="${2:-}"
    local agent_pool
    local apply_args
    local outcome
    local apply_outcome
    local apply_error
    local backup_dir
    local serve_result
    local preflight
    local preflight_status=0
    local managed=true
    local baked_cpu="-"
    local baked_memory="-"
    local previous_cpu="-"
    local previous_memory="-"
    local desired
    local desired_version
    local desired_cpu
    local desired_memory
    local limits_error

    LOCALDNS_FAILURE_MESSAGE=""

    if [ ! -x "${LOCALDNS_AKS_NODE_CONTROLLER}" ]; then
        echo "aks-node-controller binary is required for localDNS live patching"
        LOCALDNS_FAILURE_MESSAGE="aks-node-controller binary not found"
        return 1
    fi
    if ! command -v localdns_limits_preflight > /dev/null 2>&1; then
        echo "${LOCALDNS_LIMITS_SCRIPT} is required for localDNS live patching"
        LOCALDNS_FAILURE_MESSAGE="localdns-limits.sh not found"
        return 1
    fi

    # Step 0: preflight. Nothing is written yet, so a failure leaves everything as it was. A node
    # without localdns.slice never had LocalDNS set up; that only matters if the payload has a
    # config for it, since a revoke there has no limits to reset.
    preflight="$(localdns_limits_preflight)" || preflight_status=$?
    case "${preflight_status}" in
        0)
            read -r baked_cpu baked_memory previous_cpu previous_memory <<< "${preflight}"
            echo "localDNS limits: baked cpu $(localdns_limits_format_cpu "${baked_cpu}"), memory $(localdns_limits_format_memory "${baked_memory}");" \
                "running cpu $(localdns_limits_format_cpu "${previous_cpu}"), memory $(localdns_limits_format_memory "${previous_memory}")"
            ;;
        2)
            echo "${LOCALDNS_LIMITS_BAKED_SLICE_FILE} does not exist; LocalDNS is not set up on this node"
            managed=false
            ;;
        *)
            echo "localDNS limits preflight failed: ${preflight}"
            LOCALDNS_FAILURE_MESSAGE="${preflight}"
            return 1
            ;;
    esac

    # Step 1: back up.
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

    # Step 2: render. On rendered, aks-node-controller has removed .version before writing the new
    # Corefile, so a run that stops anywhere before the commit never claims the new version.
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
        rendered|alreadyCurrent)
            if [ "${managed}" != true ]; then
                echo "localDNS has a config for this node, but localdns.slice is not set up here"
                LOCALDNS_FAILURE_MESSAGE="${preflight}"
                localDNSRestore "${backup_dir}"
                rm -rf "${backup_dir}"
                return 1
            fi
            if ! desired="$(localdns_limits_from_anc_output "${outcome}" "${baked_cpu}" "${baked_memory}")"; then
                echo "failed to read the localDNS patch from aks-node-controller: ${desired}"
                LOCALDNS_FAILURE_MESSAGE="${desired}"
                localDNSRestore "${backup_dir}"
                rm -rf "${backup_dir}"
                return 1
            fi
            read -r desired_version desired_cpu desired_memory <<< "${desired}"
            ;;
        noCorefileData)
            # Nothing in this payload applies to this node: either no pool in the cluster has
            # localDNS enabled, or this node's pool does not. That is an authoritative answer from the
            # control plane, not a failed apply, so it succeeds -- reporting failure would peg those
            # nodes at localDNS=Failed and re-dispatch every component on every tick. It also revokes
            # any earlier patch, the same way the bootstrap path does, so the node serves its baked
            # Corefile and limits again instead of a config the control plane no longer publishes.
            localDNSRevoke "${backup_dir}" "${managed}" "${baked_cpu}" "${baked_memory}" "${previous_cpu}" "${previous_memory}"
            serve_result=$?
            rm -rf "${backup_dir}"
            return "${serve_result}"
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

    # Step 3: limits. This run has not touched CoreDNS yet, but a localdns start since step 2 may have
    # built the served Corefile from the new source, so the undo restores that too; when nothing rebuilt
    # it, the same content goes back and the reload plugin ignores it. An alreadyCurrent run changed no
    # file and has nothing to restore; it only drops .version, since that version's limits are no
    # longer in effect.
    if ! { limits_error="$(localdns_limits_apply "${desired_cpu}" "${desired_memory}" "${baked_cpu}" "${baked_memory}")"; } 2>&1; then
        echo "live patching the localDNS limits failed: ${limits_error}"
        if [ "${apply_outcome}" = alreadyCurrent ]; then
            localDNSUndo "${backup_dir}" false "live patching ${limits_error}" \
                "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}"
        else
            localDNSUndo "${backup_dir}" true "live patching ${limits_error}" \
                "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}" \
                "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
        fi
        rm -rf "${backup_dir}"
        return 1
    fi

    # Step 4: serve. localDNSServe itself leaves CoreDNS alone when the served Corefile would not change.
    if [ "${apply_outcome}" = rendered ]; then
        localDNSServe "${LOCALDNS_LIVEPATCHED_COREFILE}"
        serve_result=$?
        case "${serve_result}" in
            0)
                ;;
            1)
                # CoreDNS is still serving the previous Corefile. Put the files and limits back so they
                # describe it again; the reload plugin sees the served file change and settles on it.
                localDNSUndo "${backup_dir}" true "${LOCALDNS_FAILURE_MESSAGE:-could not serve the new corefile}" \
                    "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}" \
                    "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
                rm -rf "${backup_dir}"
                return 1
                ;;
            *)
                # A fallback restart failed, so localdns may be down on the new Corefile. Put the previous
                # files and limits back and start it on those; anything further is left to the unit's own
                # Restart=/OnFailure= handling.
                localDNSUndo "${backup_dir}" true "localdns.service restart failed" \
                    "${previous_cpu}" "${previous_memory}" "${baked_cpu}" "${baked_memory}" \
                    "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${LOCALDNS_SERVED_COREFILE}"
                rm -rf "${backup_dir}"
                if ! systemctl restart localdns.service; then
                    echo "failed to restart localdns.service on the restored corefile"
                fi
                return 1
                ;;
        esac
    fi
    rm -rf "${backup_dir}"

    # Step 5: commit. From here on nothing is undone: this version's Corefile and limits are in effect,
    # and a run that finds CoreDNS unready is retried by the next one, which checks again.
    if ! localdns_write_corefile_version "${LOCALDNS_LIVEPATCHED_COREFILE}.version" "${desired_version}"; then
        echo "failed to write ${LOCALDNS_LIVEPATCHED_COREFILE}.version"
        LOCALDNS_FAILURE_MESSAGE="could not write the corefile version; corefileVersion ${desired_version} applied, committed again on the next run"
        return 1
    fi
    echo "committed localDNS corefileVersion ${desired_version}"

    # Step 6: readiness decides the result. This also catches CoreDNS being OOM-killed under a lower
    # MemoryMax.
    if ! systemctl is-active --quiet localdns.service; then
        echo "localdns.service is not running"
        LOCALDNS_FAILURE_MESSAGE="localdns.service is not running; corefileVersion ${desired_version} applied, readiness checked again on the next run"
        return 1
    fi
    if ! wait_for_localdns_ready "${LOCALDNS_READY_TIMEOUT_SECONDS}" "${LOCALDNS_READY_TIMEOUT_SECONDS}"; then
        echo "CoreDNS did not report ready within ${LOCALDNS_READY_TIMEOUT_SECONDS}s"
        LOCALDNS_FAILURE_MESSAGE="CoreDNS did not report ready within ${LOCALDNS_READY_TIMEOUT_SECONDS}s; corefileVersion ${desired_version} applied, readiness checked again on the next run"
        return 1
    fi
    echo "localDNS update completed successfully"
    LOCALDNS_EVENT_OUTCOME="Succeeded"
    if [ "${apply_outcome}" = alreadyCurrent ]; then
        LOCALDNS_EVENT_MESSAGE="corefileVersion=${desired_version} served=alreadyCurrent"
    else
        LOCALDNS_EVENT_MESSAGE="corefileVersion=${desired_version} served=${LOCALDNS_SERVED_BY}"
    fi
    return 0
}
