#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

# For Flatcar: systemd timer instead of cron, skip cloud-init/apt ops, chronyd service name).
IS_FLATCAR=0
if [ -f /etc/os-release ] && grep -qi '^ID=flatcar' /etc/os-release; then
  IS_FLATCAR=1
fi

# http://168.63.129.16 is a constant for the host's wireserver endpoint
WIRESERVER_ENDPOINT="http://168.63.129.16"

# Function to make HTTP request with retry logic for rate limiting
make_request_with_retry() {
    local url="$1"
    local max_retries=10
    local retry_delay=3
    local attempt=1
    
    local response
    while [ $attempt -le $max_retries ]; do
        response=$(curl -f --no-progress-meter "$url")
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

# Function to process certificate operations from a given endpoint
process_cert_operations() {
    local endpoint_type="$1"
    local operation_response
    
    echo "Retrieving certificate operations for type: $endpoint_type"
    operation_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json")
    local request_status=$?
    if [ -z "$operation_response" ] || [ $request_status -ne 0 ]; then
        echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json"
        return
    fi
    
    # Extract ResourceFileName values from the JSON response
    local cert_filenames
    mapfile -t cert_filenames < <(echo "$operation_response" | grep -oP '(?<="ResouceFileName": ")[^"]*')
    
    if [ ${#cert_filenames[@]} -eq 0 ]; then
        echo "No certificate filenames found in response for $endpoint_type"
        return
    fi
    
    # Process each certificate file
    for cert_filename in "${cert_filenames[@]}"; do
        echo "Processing certificate file: $cert_filename"
        
        # Extract filename and extension
        local filename="${cert_filename%.*}"
        local extension="${cert_filename##*.}"
        
        echo "Downloading certificate: filename=$filename, extension=$extension"
        
        # Retrieve the actual certificate content with retry logic
        local cert_content
        cert_content=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension")
        local request_status=$?
        if [ -z "$cert_content" ] || [ $request_status -ne 0 ]; then
            echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension"
            continue
        fi
        
        if [ -n "$cert_content" ]; then
            # Save the certificate to the appropriate location
            echo "$cert_content" > "/root/AzureCACertificates/$cert_filename"
            echo "Successfully saved certificate: $cert_filename"
        else
            echo "Warning: Failed to retrieve certificate content for $cert_filename"
        fi
    done
}

# Process root certificates
process_cert_operations "operationrequestsroot"

# Process intermediate certificates  
process_cert_operations "operationrequestsintermediate"

if [ "${IS_FLATCAR}" -eq 0 ]; then
    # Copy all certificate files to the system certificate directory
    cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/

    # Update the system certificate store
    update-ca-certificates

    # This copies the updated bundle to the location used by OpenSSL which is commonly used
    cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem
else
    for cert in /root/AzureCACertificates/*.crt; do
        destcert="${cert##*/}"
        destcert="${destcert%.*}.pem"
        cp "$cert" /etc/ssl/certs/"$destcert"
    done
    update-ca-certificates
fi



# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
action=${1:-init}
if [ "$action" = "ca-refresh" ]; then
    exit
fi

if [ "$IS_FLATCAR" -eq 0 ]; then
    (crontab -l ; echo "0 19 * * * $0 ca-refresh") | crontab -
else
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
ExecStart=$script_path ca-refresh
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

#EOF