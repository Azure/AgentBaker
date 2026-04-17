#!/bin/bash
set -x

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
    while [ $attempt -le $max_retries ]; do
        response=$(curl -f --no-progress-meter --connect-timeout 10 --max-time 30 "$url")
        local request_status=$?

        if echo "$response" | grep -q "RequestRateLimitExceeded"; then
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
            attempt=$((attempt + 1))
        elif [ $request_status -ne 0 ]; then
            sleep $retry_delay
            attempt=$((attempt + 1))
        else
            echo "$response"
            return 0
        fi
    done

    echo "exhausted all retries, last response: $response"
    return 1
}

function is_opted_in_for_root_certs {
    local opt_in_response
    local request_status
    local poll_attempt=1
    local max_poll_attempts=30
    local poll_interval=10

    # Poll wireserver for up to ~5 minutes to allow platform metadata to sync.
    # The VM instance tag triggers a Fabric Controller goal state (CCF) update,
    # which must propagate to the host agent before wireserver can reflect it.
    # FC goal state propagation can take several minutes in practice.
    while [ $poll_attempt -le $max_poll_attempts ]; do
        echo "is_opted_in_for_root_certs: poll attempt ${poll_attempt}/${max_poll_attempts}"

        opt_in_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/acms/isOptedInForRootCerts")
        request_status=$?

        echo "is_opted_in_for_root_certs: wireserver response (status=${request_status}): '${opt_in_response}'"

        if [ $request_status -ne 0 ] || [ -z "$opt_in_response" ]; then
            echo "Warning: failed to determine IsOptedInForRootCerts state on attempt ${poll_attempt}"
        elif echo "$opt_in_response" | grep -q "IsOptedInForRootCerts=true"; then
            echo "IsOptedInForRootCerts=true (found on attempt ${poll_attempt})"
            return 0
        fi

        if [ $poll_attempt -lt $max_poll_attempts ]; then
            echo "is_opted_in_for_root_certs: not opted in yet, waiting ${poll_interval}s before retry..."
            sleep $poll_interval
        fi

        poll_attempt=$((poll_attempt + 1))
    done

    echo "Skipping custom cloud root cert installation because IsOptedInForRootCerts is not true after ${max_poll_attempts} attempts"
    echo "Last wireserver response: '${opt_in_response}'"
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
    done
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
# - Wireserver failures are treated as non-fatal, and cert trust-store updates are skipped gracefully.

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
install_ca_refresh_schedule=0
mkdir -p /root/AzureCACertificates
rm -f /root/AzureCACertificates/*
if [ "$cert_endpoint_mode" = "legacy" ]; then
    install_ca_refresh_schedule=1
    if retrieve_legacy_certs; then
        install_certs_to_trust_store
    else
        echo "Warning: failed to retrieve legacy certificates from wireserver; continuing without trust store updates"
    fi
elif [ "$cert_endpoint_mode" = "rcv1p" ]; then
    if is_opted_in_for_root_certs; then
        install_ca_refresh_schedule=1
        if retrieve_rcv1p_certs; then
            install_certs_to_trust_store
        else
            echo "Warning: failed to retrieve rcv1p certificates from wireserver; continuing without trust store updates"
        fi
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

if [ "$IS_UBUNTU" -eq 1 ] || [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
    scriptPath=$0
    # Determine an absolute, canonical path to this script for use in cron.
    if command -v readlink >/dev/null 2>&1; then
        # Use readlink -f when available to resolve the canonical path; fall back to $0 on error.
        scriptPath="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
    fi

    if [ "$install_ca_refresh_schedule" -eq 1 ]; then
        if ! crontab -l 2>/dev/null | grep -q "\"$scriptPath\" ca-refresh"; then
            # Quote the script path in the cron entry to avoid issues with spaces or special characters.
            if ! (crontab -l 2>/dev/null ; printf '%s\n' "0 19 * * * \"$scriptPath\" ca-refresh \"$LOCATION\"") | crontab -; then
                echo "Failed to install ca-refresh cron job via crontab" >&2
            fi
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
