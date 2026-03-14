#!/bin/bash

CSE_STARTTIME=$(date)
CSE_STARTTIME_FORMATTED=$(date +"%F %T.%3N")
export CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s) # Export for child processes, used in early retry loop exits

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
mkdir -p $EVENTS_LOGGING_DIR

HOTFIX_LOG="/var/log/azure/hotfix-check.log"

# Ensure /opt/bin is in PATH (ORAS is installed there during VHD build)
case "${PATH}" in
    */opt/bin*) : ;;
    *) export PATH="/opt/bin:${PATH}" ;;
esac

# Source CSE helpers early for ORAS login functionality.
# This enables a single ORAS login that is reused for both hotfix detection
# and later ORAS operations (kubelet downloads, etc.) in cse_main.sh.
# The credentials persist in /etc/oras/config.yaml across processes.
if [ -n "${CSE_HELPERS_FILEPATH}" ]; then
    for i in $(seq 1 120); do
        if [ -s "${CSE_HELPERS_FILEPATH}" ]; then
            grep -Fq '#HELPERSEOF' "${CSE_HELPERS_FILEPATH}" && break
        fi
        if [ $i -eq 120 ]; then
            echo "$(date): cse_start: helpers file not ready after 120s" >> "$HOTFIX_LOG"
        else
            sleep 1
        fi
    done
    if [ -s "${CSE_HELPERS_FILEPATH}" ] && grep -Fq '#HELPERSEOF' "${CSE_HELPERS_FILEPATH}" 2>/dev/null; then
        # shellcheck disable=SC1090
        source "${CSE_HELPERS_FILEPATH}"
    fi
fi

# For NI clusters, perform ORAS login once. The credentials persist in
# ORAS_REGISTRY_CONFIG_FILE (/etc/oras/config.yaml) and are reused by
# both hotfix check below and cse_main.sh's ORAS operations.
if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ] && command -v oras &>/dev/null && type oras_login_with_managed_identity &>/dev/null; then
    _hotfix_registry_domain="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER%%/*}"
    oras_login_with_managed_identity "${_hotfix_registry_domain}" "$USER_ASSIGNED_IDENTITY_ID" "$TENANT_ID" >>"$HOTFIX_LOG" 2>&1 || true
    set +x 2>/dev/null  # oras_login_with_managed_identity enables set -x; restore
    unset _hotfix_registry_domain
fi

# check_for_script_hotfix — detect and apply provisioning script hotfixes
# published as OCI artifacts.
# Supports both NI clusters (private ACR via BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER)
# and non-NI clusters (MCR anonymous pull).
# Non-fatal: any failure silently falls back to baked scripts.
check_for_script_hotfix() {
    local hotfix_log="${HOTFIX_LOG:-/var/log/azure/hotfix-check.log}"
    local baked_version_file="/opt/azure/containers/.provisioning-scripts-version"
    local sku=""

    # Determine SKU from OS
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        case "${ID}-${VERSION_ID}" in
            ubuntu-22.04) sku="ubuntu-2204" ;;
            ubuntu-24.04) sku="ubuntu-2404" ;;
            mariner-2*)   sku="azurelinux-v2" ;;
            azurelinux-3*) sku="azurelinux-v3" ;;
            *) echo "$(date): Hotfix check: unknown SKU ${ID}-${VERSION_ID}, skipping" >> "$hotfix_log"; return 0 ;;
        esac
    else
        echo "$(date): Hotfix check: /etc/os-release not found, skipping" >> "$hotfix_log"
        return 0
    fi

    # Determine registry, repo, and auth args based on cluster type:
    #   NI cluster  → private ACR mirror (ORAS login already performed above)
    #   Non-NI      → MCR direct (anonymous pull, no auth needed)
    #   HOTFIX_REGISTRY override → explicit registry (e.g., testing)
    local registry=""
    local repo=""
    local oras_auth_args=""

    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        # NI cluster: ORAS login already performed; reuse credentials from config file
        registry="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER%%/*}"
        repo="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/aks/provisioning-scripts/${sku}"
        local oras_cfg="${ORAS_REGISTRY_CONFIG_FILE:-/etc/oras/config.yaml}"
        if [ -s "$oras_cfg" ]; then
            oras_auth_args="--registry-config ${oras_cfg}"
        else
            echo "$(date): Hotfix check: NI cluster but no ORAS credentials, skipping" >> "$hotfix_log"
            return 0
        fi
    elif [ -n "${HOTFIX_REGISTRY}" ]; then
        registry="${HOTFIX_REGISTRY}"
        repo="${registry}/aks/provisioning-scripts/${sku}"
    else
        registry="mcr.microsoft.com"
        repo="${registry}/aks/provisioning-scripts/${sku}"
    fi

    # Read baked version
    if [ ! -f "$baked_version_file" ]; then
        echo "$(date): Hotfix check: no version stamp at ${baked_version_file}, skipping" >> "$hotfix_log"
        return 0
    fi
    local baked_version
    baked_version=$(cat "$baked_version_file")
    if [ -z "$baked_version" ]; then
        echo "$(date): Hotfix check: empty version stamp, skipping" >> "$hotfix_log"
        return 0
    fi

    if ! command -v oras &>/dev/null; then
        echo "$(date): Hotfix check: ORAS not available, skipping" >> "$hotfix_log"
        return 0
    fi

    local hotfix_tag="${baked_version}-hotfix"
    echo "$(date): Hotfix check: version=${baked_version} sku=${sku} tag=${hotfix_tag} registry=${registry}" >> "$hotfix_log"

    # Query registry for the specific hotfix tag using oras manifest fetch.
    # This is a single lightweight HEAD-like request — no listing of all tags.
    # shellcheck disable=SC2086
    if ! timeout 30 oras manifest fetch ${oras_auth_args} "${repo}:${hotfix_tag}" > /dev/null 2>&1; then
        echo "$(date): Hotfix check: no hotfix tag '${hotfix_tag}' found (normal case)" >> "$hotfix_log"
        return 0
    fi

    echo "$(date): Hotfix check: found hotfix ${hotfix_tag}, pulling..." >> "$hotfix_log"

    local staging_dir="/opt/azure/containers/.hotfix-staging"
    local applied_marker="/opt/azure/containers/.hotfix-applied"

    # Skip if already applied (idempotency for retries)
    if [ -f "$applied_marker" ]; then
        echo "$(date): Hotfix check: hotfix already applied, skipping" >> "$hotfix_log"
        return 0
    fi

    # Pull the hotfix artifact
    mkdir -p "$staging_dir"
    # shellcheck disable=SC2086
    if ! timeout 60 oras pull ${oras_auth_args} "${repo}:${hotfix_tag}" -o "$staging_dir" 2>> "$hotfix_log"; then
        echo "$(date): Hotfix check: pull failed, using baked scripts" >> "$hotfix_log"
        rm -rf "$staging_dir"
        return 0
    fi

    # Verify metadata
    local metadata="$staging_dir/hotfix-metadata.json"
    if [ ! -f "$metadata" ]; then
        echo "$(date): Hotfix check: no metadata in artifact, using baked scripts" >> "$hotfix_log"
        rm -rf "$staging_dir"
        return 0
    fi

    # Defense in depth: verify affectedVersion matches baked version
    local meta_version
    meta_version=$(jq -r '.affectedVersion' "$metadata" 2>/dev/null)
    if [ "$meta_version" != "$baked_version" ]; then
        echo "$(date): Hotfix check: metadata version '${meta_version}' != baked '${baked_version}', skipping" >> "$hotfix_log"
        rm -rf "$staging_dir"
        return 0
    fi

    # Extract tarball — overlays corrected scripts onto the filesystem
    local tarball
    tarball=$(find "$staging_dir" -name "*.tar.gz" -print -quit 2>/dev/null)
    if [ -n "$tarball" ]; then
        if tar -xzf "$tarball" -C / --no-same-owner --no-overwrite-dir 2>> "$hotfix_log"; then
            echo "$hotfix_tag" > "$applied_marker"
            echo "$(date): Hotfix check: applied ${hotfix_tag} successfully" >> "$hotfix_log"
        else
            echo "$(date): Hotfix check: tar extraction failed, using baked scripts" >> "$hotfix_log"
        fi
    else
        echo "$(date): Hotfix check: no tarball in artifact, using baked scripts" >> "$hotfix_log"
    fi

    rm -rf "$staging_dir"
    return 0
}

# Run hotfix check before provisioning — must happen before any scripts are sourced.
# Failures are non-fatal; we always proceed with whatever scripts are on disk.
check_for_script_hotfix || true

# this is the "global" CSE execution timeout - we allow CSE to run for some time (default 15 minutes) before timeout will attempt to kill the script. We exit early from some of the retry loops using `check_cse_timeout` in `cse_helpers.sh`.`
timeout -k5s "${CSE_TIMEOUT:-15m}" /bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(tail -c 3000 "/var/log/azure/cluster-provision.log")
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
KERNEL_STARTTIME_FORMATTED=$(date -d "${KERNEL_STARTTIME}" +"%F %T.%3N" )
CLOUDINITLOCAL_STARTTIME=$(systemctl show cloud-init-local -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITLOCAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITLOCAL_STARTTIME}" +"%F %T.%3N" )
CLOUDINIT_STARTTIME=$(systemctl show cloud-init -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINIT_STARTTIME_FORMATTED=$(date -d "${CLOUDINIT_STARTTIME}" +"%F %T.%3N" )
CLOUDINITFINAL_STARTTIME=$(systemctl show cloud-final -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITFINAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITFINAL_STARTTIME}" +"%F %T.%3N" )
NETWORKD_STARTTIME=$(systemctl show systemd-networkd -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
NETWORKD_STARTTIME_FORMATTED=$(date -d "${NETWORKD_STARTTIME}" +"%F %T.%3N" )
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
GUEST_AGENT_STARTTIME_FORMATTED=$(date -d "${GUEST_AGENT_STARTTIME}" +"%F %T.%3N" )
KUBELET_START_TIME=$(systemctl show kubelet.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
KUBELET_START_TIME_FORMATTED=$(date -d "${KUBELET_START_TIME}" +"%F %T.%3N" )
KUBELET_READY_TIME_FORMATTED="$(date -d "$(journalctl -u kubelet | grep NodeReady | cut -d' ' -f1-3)" +"%F %T.%3N")"
SYSTEMD_SUMMARY=$(systemd-analyze || true)
CSE_ENDTIME_FORMATTED=$(date +"%F %T.%3N")
EVENTS_FILE_NAME=$(date +%s%3N)
EXECUTION_DURATION=$(($(date +%s) - $(date -d "$CSE_STARTTIME" +%s)))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg cinitl "$CLOUDINITLOCAL_STARTTIME" \
                  --arg cinit "$CLOUDINIT_STARTTIME" \
                  --arg cf "$CLOUDINITFINAL_STARTTIME" \
                  --arg ns "$NETWORKD_STARTTIME" \
                  --arg cse "$CSE_STARTTIME" \
                  --arg ga "$GUEST_AGENT_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  --arg kubelet "$KUBELET_START_TIME" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CloudInitLocalStartTime: $cinitl, CloudInitStartTime: $cinit, CloudFinalStartTime: $cf, NetworkdStartTime: $ns, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, KubeletStartTime: $kubelet }}' )
mkdir -p /var/log/azure/aks
echo $JSON_STRING | tee /var/log/azure/aks/provision.json

# Cleanup cache file
rm -f /opt/azure/containers/imds_instance_metadata_cache.json || true

# Create stage marker for two-stage workflow
if [ "${PRE_PROVISION_ONLY}" = "true" ]; then
    # Stage 1: Create marker indicating Stage 2 is needed
    mkdir -p /opt/azure/containers && touch /opt/azure/containers/base_prep.complete
    echo "Stage 1 complete - kubelet configuration skipped, Stage 2 required" >> /var/log/azure/cluster-provision.log
    echo "Created base_prep.complete marker file" >> /var/log/azure/cluster-provision.log
    exit 0
fi

# provision.complete is the marker for the second stage of the workflow
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

# messsage_string is here because GA only accepts strings in Message.
message_string=$( jq -n \
--arg EXECUTION_DURATION                  "${EXECUTION_DURATION}" \
--arg EXIT_CODE                           "${EXIT_CODE}" \
--arg KERNEL_STARTTIME_FORMATTED          "${KERNEL_STARTTIME_FORMATTED}" \
--arg CLOUDINITLOCAL_STARTTIME_FORMATTED  "${CLOUDINITLOCAL_STARTTIME_FORMATTED}" \
--arg CLOUDINIT_STARTTIME_FORMATTED       "${CLOUDINIT_STARTTIME_FORMATTED}" \
--arg CLOUDINITFINAL_STARTTIME_FORMATTED  "${CLOUDINITFINAL_STARTTIME_FORMATTED}" \
--arg NETWORKD_STARTTIME_FORMATTED        "${NETWORKD_STARTTIME_FORMATTED}" \
--arg GUEST_AGENT_STARTTIME_FORMATTED     "${GUEST_AGENT_STARTTIME_FORMATTED}" \
--arg KUBELET_START_TIME_FORMATTED        "${KUBELET_START_TIME_FORMATTED}" \
--arg KUBELET_READY_TIME_FORMATTED       "${KUBELET_READY_TIME_FORMATTED}" \
'{ExitCode: $EXIT_CODE, E2E: $EXECUTION_DURATION, KernelStartTime: $KERNEL_STARTTIME_FORMATTED, CloudInitLocalStartTime: $CLOUDINITLOCAL_STARTTIME_FORMATTED, CloudInitStartTime: $CLOUDINIT_STARTTIME_FORMATTED, CloudFinalStartTime: $CLOUDINITFINAL_STARTTIME_FORMATTED, NetworkdStartTime: $NETWORKD_STARTTIME_FORMATTED, GuestAgentStartTime: $GUEST_AGENT_STARTTIME_FORMATTED, KubeletStartTime: $KUBELET_START_TIME_FORMATTED, KubeletReadyTime: $KUBELET_READY_TIME_FORMATTED } | tostring'
)
# this clean up brings me no joy, but removing extra "\" and then removing quotes at the end of the string
# allows parsing to happening without additional manipulation
message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

# arg names are defined by GA and all these are required to be correctly read by GA
# EventPid, EventTid are required to be int. No use case for them at this point.
EVENT_JSON=$( jq -n \
    --arg Timestamp     "${CSE_STARTTIME_FORMATTED}" \
    --arg OperationId   "${CSE_ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.CSE.cse_start" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)
echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json

# force a log upload to the host after the provisioning script finishes
# if we failed, wait for the upload to complete so that we don't remove
# the VM before it finishes. if we succeeded, upload in the background
# so that the provisioning script returns success more quickly
upload_logs() {
    # if the VHD has the AKS log collector installed, use it instead. Otherwise
    # fall back to WALA collector
    if test -x /opt/azure/containers/aks-log-collector.sh; then
        # Call AKS Log Collector
        /opt/azure/containers/aks-log-collector.sh >/var/log/azure/aks/cse-aks-log-collector.log 2>&1
    else
        # find the most recent version of WALinuxAgent and use it to collect logs per
        # https://supportability.visualstudio.com/AzureIaaSVM/_wiki/wikis/AzureIaaSVM/495009/Log-Collection_AGEX?anchor=manually-collect-logs
        PYTHONPATH=$(find /var/lib/waagent -name WALinuxAgent\*.egg | sort -rV | head -n1)
        python3 $PYTHONPATH -collect-logs -full >/dev/null 2>&1
        python3 /opt/azure/containers/provision_send_logs.py >/dev/null 2>&1
    fi
}
if [ "$EXIT_CODE" -ne 0 ]; then
    upload_logs
else
    upload_logs &
fi

exit "$EXIT_CODE"
