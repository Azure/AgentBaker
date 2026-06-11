#!/bin/bash
set -x

# GA events directory — Azure Guest Agent monitors this directory and forwards
# JSON event files to Geneva/Kusto for off-node telemetry.
EVENTS_LOGGING_DIR="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"

# Lightweight logs_to_events for telemetry — wraps a command, records timing,
# and writes a JSON event file that GA picks up and ships to Kusto.
# Does NOT suppress stdout/stderr — existing log lines are preserved.
logs_to_events() {
    local task=$1; shift
    local eventsFileName
    eventsFileName=$(date +%s%3N)

    local startTime
    startTime=$(date +"%F %T.%3N")
    "${@}"
    local ret=$?
    local endTime
    endTime=$(date +"%F %T.%3N")

    local json_string
    json_string=$(jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "Completed: $*" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )

    mkdir -p "${EVENTS_LOGGING_DIR}"
    echo "${json_string}" > "${EVENTS_LOGGING_DIR}${eventsFileName}.json"

    if [ "$ret" -ne 0 ]; then
        return $ret
    fi
}

# Emit a custom telemetry event with a specific message (not wrapping a command).
emit_event() {
    local task=$1
    local message=$2
    local level=${3:-Informational}
    local eventsFileName
    eventsFileName=$(date +%s%3N)
    local timestamp
    timestamp=$(date +"%F %T.%3N")

    local json_string
    json_string=$(jq -n \
        --arg Timestamp   "${timestamp}" \
        --arg OperationId "${timestamp}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "${level}" \
        --arg Message     "${message}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )

    mkdir -p "${EVENTS_LOGGING_DIR}"
    echo "${json_string}" > "${EVENTS_LOGGING_DIR}${eventsFileName}.json"
}

IS_FLATCAR=0
IS_UBUNTU=0
IS_ACL=0
IS_MARINER=0
IS_AZURELINUX=0
# shellcheck disable=SC3010
if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    # shellcheck disable=SC3010
    if [[ $NAME == *"Ubuntu"* ]]; then
        IS_UBUNTU=1
    elif [[ $ID == *"flatcar"* ]]; then
        IS_FLATCAR=1
    elif [[ $ID == "azurecontainerlinux" ]] || { [[ $ID == "azurelinux" ]] && [[ ${VARIANT_ID:-} == "azurecontainerlinux" ]]; }; then
        IS_ACL=1
    elif [[ $NAME == *"Mariner"* ]]; then
        IS_MARINER=1
    elif [[ $NAME == *"Microsoft Azure Linux"* ]]; then
        IS_AZURELINUX=1
    else
        echo "Unknown Linux distribution"
        exit 1
    fi
else
    echo "Unsupported operating system"
    exit 1
fi

echo "distribution is $distribution"
echo "Running on $NAME"

# http://168.63.129.16 is a constant for the host's wireserver endpoint
WIRESERVER_ENDPOINT="http://168.63.129.16"

function make_request_with_retry {
    local url="$1"
    local max_retries=10
    local retry_delay=3
    local attempt=1

    local response
    local http_code
    local curl_output
    while [ $attempt -le $max_retries ]; do
        # capture response body + HTTP status code; -w appends the code after the body.
        # curl stderr (connection errors) flows to the script's log naturally.
        # http_code is 000 when wireserver is unreachable (connection refused/timeout).
        curl_output=$(curl --no-progress-meter --connect-timeout 10 --max-time 30 -w '\n%{http_code}' "$url") || true
        http_code=$(echo "$curl_output" | tail -1)
        response=$(echo "$curl_output" | sed '$d')

        if echo "$response" | grep -q "RequestRateLimitExceeded" && [ "$http_code" = "403" ]; then
            echo "wireserver rate limited (HTTP ${http_code}) on attempt ${attempt}/${max_retries}: ${url}" >&2
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
            attempt=$((attempt + 1))
        elif [ "$http_code" -ge 200 ] 2>/dev/null && [ "$http_code" -lt 300 ] 2>/dev/null; then
            echo "$response"
            return 0
        else
            echo "wireserver request failed (HTTP ${http_code}) on attempt ${attempt}/${max_retries}: ${url}" >&2
            if [ -n "$response" ]; then
                echo "wireserver error response: ${response}" >&2
            fi
            sleep $retry_delay
            attempt=$((attempt + 1))
        fi
    done

    echo "exhausted all retries for ${url} (last HTTP ${http_code}), last response: $response" >&2
    return 1
}

# Returns:
#   0 - opted in (wireserver confirmed IsOptedInForRootCerts=true)
#   1 - not opted in (wireserver responded with false; valid, skip certs)
#   2 - wireserver unreachable after retries (caller must treat as fatal)
#
# Wireserver unreachable must be fatal (return 2) rather than silently skipping certs.
# If the subscription is opted in for hardened root certs but we silently fall back to
# the distro's default trust store, we leave a security hole — the node would trust CAs
# that the customer explicitly intended to replace. Failing hard surfaces the problem
# immediately instead of letting the node run with an insecure certificate configuration.
function is_opted_in_for_root_certs {
    local opt_in_response

    opt_in_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/acms/isOptedInForRootCerts")
    local request_status=$?
    if [ $request_status -ne 0 ] || [ -z "$opt_in_response" ]; then
        echo "ERROR: wireserver unreachable or returned empty response for IsOptedInForRootCerts"
        return 2
    fi

    # Wireserver may return JSON ({"IsOptedInForRootCerts":true}) or key=value
    # (IsOptedInForRootCerts=true). Use jq for proper JSON parsing.
    if echo "$opt_in_response" | jq -e '.IsOptedInForRootCerts == true' > /dev/null 2>&1; then
        echo "IsOptedInForRootCerts=true"
        return 0
    fi

    echo "Skipping custom cloud root cert installation because IsOptedInForRootCerts is not true"
    return 1
}

function get_trust_store_dir {
    if [ "$IS_ACL" -eq 1 ] || [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
        echo "/etc/pki/ca-trust/source/anchors"
    elif [ "$IS_FLATCAR" -eq 1 ]; then
        echo "/etc/ssl/certs"
    else
        echo "/usr/local/share/ca-certificates"
    fi
}

function debug_print_trust_store {
    local stage="$1"
    local trust_store_dir

    trust_store_dir=$(get_trust_store_dir)
    echo "Trust store contents ${stage} cert copy: ${trust_store_dir}"
    ls -al "$trust_store_dir" || true
}

function retrieve_legacy_certs {
    local certs
    local cert_names
    local cert_bodies
    local i

    certs=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=cacertificates&ext=json")
    if [ -z "$certs" ]; then
        echo "Warning: failed to retrieve legacy custom cloud certificates"
        return 1
    fi

    IFS_backup=$IFS
    IFS=$'\r\n'
    cert_names=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
    cert_bodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
    for i in ${!cert_bodies[@]}; do
        echo ${cert_bodies[$i]} | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${cert_names[$i]} | sed 's/.cer/.crt/g')"
    done
    IFS=$IFS_backup
}

function process_cert_operations {
    local endpoint_type="$1"
    local operation_response

    echo "Retrieving certificate operations for type: $endpoint_type"
    operation_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json")
    local request_status=$?
    if [ -z "$operation_response" ] || [ $request_status -ne 0 ]; then
        echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json"
        return 1
    fi

    local cert_filenames
    mapfile -t cert_filenames < <(echo "$operation_response" | grep -oP '(?<="ResouceFileName": ")[^"]*')

    if [ ${#cert_filenames[@]} -eq 0 ]; then
        echo "No certificate filenames found in response for $endpoint_type"
        return 1
    fi

    local saved_count=0
    for cert_filename in "${cert_filenames[@]}"; do
        echo "Processing certificate file: $cert_filename"

        local filename="${cert_filename%.*}"
        local extension="${cert_filename##*.}"
        local cert_content

        cert_content=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension")
        local request_status=$?
        if [ -z "$cert_content" ] || [ $request_status -ne 0 ]; then
            echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension"
            continue
        fi

        echo "$cert_content" > "/root/AzureCACertificates/$cert_filename"
        echo "Successfully saved certificate: $cert_filename"
        saved_count=$((saved_count + 1))
    done

    if [ $saved_count -eq 0 ]; then
        echo "Error: all certificate content fetches failed for $endpoint_type (${#cert_filenames[@]} filenames found but 0 saved)"
        return 1
    fi
    echo "Saved $saved_count/${#cert_filenames[@]} certificates for $endpoint_type"
}

function retrieve_rcv1p_certs {
    process_cert_operations "operationrequestsroot" || return 1
    process_cert_operations "operationrequestsintermediate" || return 1
}

function install_certs_to_trust_store {
    mkdir -p /root/AzureCACertificates

    debug_print_trust_store "before"

    if [ "$IS_ACL" -eq 1 ] || [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
        cp /root/AzureCACertificates/*.crt /etc/pki/ca-trust/source/anchors/
        update-ca-trust
    elif [ "$IS_FLATCAR" -eq 1 ]; then
        for cert in /root/AzureCACertificates/*.crt; do
            destcert="${cert##*/}"
            destcert="${destcert%.*}.pem"
            cp "$cert" /etc/ssl/certs/"$destcert"
        done
        update-ca-certificates
    else
        cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/
        update-ca-certificates

        # This copies the updated bundle to the location used by OpenSSL which is commonly used
        cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem
    fi

    debug_print_trust_store "after"
}

# Certificate refresh behavior summary:
# - legacy mode directly attempts certificate download from wireserver and only in ussec and usnat regions.
# - rcv1p mode first checks IsOptedInForRootCerts, then downloads only when opted in.
# - Wireserver failures are fatal — cert installation must succeed for the selected mode.

refresh_location="${2:-${LOCATION}}"

location_normalized="${refresh_location,,}"
location_normalized="${location_normalized//[[:space:]]/}"
if [ -z "$location_normalized" ]; then
    echo "Warning: LOCATION is empty; defaulting custom cloud certificate endpoint mode to rcv1p"
fi

cert_endpoint_mode="rcv1p"
case "$location_normalized" in
    ussec*|usnat*) cert_endpoint_mode="legacy" ;;
esac

echo "Using custom cloud certificate endpoint mode: ${cert_endpoint_mode}"
emit_event "AKS.CSE.rcv1p.certEndpointMode" "mode=${cert_endpoint_mode}, location=${location_normalized}"
install_ca_refresh_schedule=0
mkdir -p /root/AzureCACertificates
rm -f /root/AzureCACertificates/*
if [ "$cert_endpoint_mode" = "legacy" ]; then
    install_ca_refresh_schedule=1
    if logs_to_events "AKS.CSE.rcv1p.retrieveLegacyCerts" retrieve_legacy_certs; then
        logs_to_events "AKS.CSE.rcv1p.installCertsToTrustStore" install_certs_to_trust_store
    else
        echo "ERROR: failed to retrieve legacy certificates from wireserver after retries"
        exit 1
    fi
elif [ "$cert_endpoint_mode" = "rcv1p" ]; then
    logs_to_events "AKS.CSE.rcv1p.isOptedIn" is_opted_in_for_root_certs
    opt_in_result=$?
    if [ $opt_in_result -eq 2 ]; then
        # Fatal: wireserver was unreachable after retries. We cannot determine whether
        # the node should use hardened certs or the default trust store. Silently
        # falling back to the distro trust store would be a security hole if the
        # customer intended hardened certs, so we fail hard here.
        echo "ERROR: cannot provision node — wireserver unreachable for cert opt-in check"
        emit_event "AKS.CSE.rcv1p.optInCheckFailed" "wireserver unreachable after retries" "Error"
        exit 1
    elif [ $opt_in_result -eq 0 ]; then
        install_ca_refresh_schedule=1
        emit_event "AKS.CSE.rcv1p.optedIn" "IsOptedInForRootCerts=true"
        if logs_to_events "AKS.CSE.rcv1p.retrieveCerts" retrieve_rcv1p_certs; then
            cert_count=$(find /root/AzureCACertificates -name '*.crt' 2>/dev/null | wc -l)
            emit_event "AKS.CSE.rcv1p.certCount" "downloaded ${cert_count} certificates"
            logs_to_events "AKS.CSE.rcv1p.installCertsToTrustStore" install_certs_to_trust_store
        else
            echo "ERROR: failed to retrieve rcv1p certificates from wireserver after retries"
            emit_event "AKS.CSE.rcv1p.retrieveCertsFailed" "failed to retrieve rcv1p certificates" "Error"
            exit 1
        fi
    else
        emit_event "AKS.CSE.rcv1p.notOptedIn" "IsOptedInForRootCerts=false, skipping cert installation"
    fi
fi

# In ca-refresh mode (invoked by the scheduled cron/systemd task with the location as arg),
# only the cert refresh above is needed; exit before running the full init path.
# Action values:
# - init (default): full provisioning path
# - ca-refresh <location>: periodic refresh path; location is passed as arg to avoid env dependency
action=${1:-init}
if [ "$action" = "ca-refresh" ]; then
    exit
fi

if [ "$install_ca_refresh_schedule" -eq 1 ] && [ -z "$LOCATION" ]; then
    echo "ERROR: LOCATION is required to install ca-refresh schedule but is empty"
    exit 1
fi

if [ "$IS_UBUNTU" -eq 1 ] || [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
    scriptPath=$0
    # Determine an absolute, canonical path to this script for use in cron.
    if command -v readlink >/dev/null 2>&1; then
        # Use readlink -f when available to resolve the canonical path; fall back to $0 on error.
        scriptPath="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
    fi

    if [ "$install_ca_refresh_schedule" -eq 1 ]; then
        # Remove any existing ca-refresh entry for this script (may lack the location argument
        # from older VHDs on custom clouds like AGC/Delos) and re-add with the explicit location.
        # Without the location argument, ca-refresh defaults endpoint mode to rcv1p which is
        # wrong for ussec/usnat legacy environments.
        new_entry="0 19 * * * \"$scriptPath\" ca-refresh \"$LOCATION\""
        existing=$(crontab -l 2>/dev/null || true)
        filtered=$(printf '%s\n' "$existing" | grep -v "\"$scriptPath\" ca-refresh" || true)
        if ! (printf '%s\n' "$filtered"; printf '%s\n' "$new_entry") | sed '/^$/d' | crontab -; then
            echo "Failed to install ca-refresh cron job via crontab" >&2
        fi
    fi
elif [ "$IS_FLATCAR" -eq 1 ] || [ "$IS_ACL" -eq 1 ]; then
    if [ "$install_ca_refresh_schedule" -eq 1 ]; then
        script_path="$(readlink -f "$0")"
        svc="/etc/systemd/system/azure-ca-refresh.service"
        tmr="/etc/systemd/system/azure-ca-refresh.timer"

        cat >"$svc" <<EOF
[Unit]
Description=Refresh Azure Custom Cloud CA certificates
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=$script_path ca-refresh $LOCATION
EOF

        cat >"$tmr" <<EOF
[Unit]
Description=Daily refresh of Azure Custom Cloud CA certificates

[Timer]
OnCalendar=19:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
EOF

        systemctl daemon-reload
        systemctl enable --now azure-ca-refresh.timer
    fi
fi

# Source the repo depot and chrony initialization script if present.
# This script is only included in custom cloud images and handles repo depot
# configuration and chrony setup. It inherits all variables from this script.
REPOS_SCRIPT="$(dirname "$(readlink -f "$0")")/init-aks-custom-cloud-repos.sh"
if [ -f "$REPOS_SCRIPT" ] && [ -s "$REPOS_SCRIPT" ]; then
    source "$REPOS_SCRIPT"
fi

#EOF
