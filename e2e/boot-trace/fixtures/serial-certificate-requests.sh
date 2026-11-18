#!/bin/bash
# Historical serial control, used only by the boot-trace comparison matrix.

function make_request_with_retry {
    local url="$1"
    local max_retries=10
    local retry_delay=3
    local attempt=1
    local response http_code curl_output
    while [ $attempt -le $max_retries ]; do
        curl_output=$(curl --no-progress-meter --connect-timeout 10 --max-time 30 -w '\n%{http_code}' "$url") || true
        http_code=$(echo "$curl_output" | tail -1)
        response=$(echo "$curl_output" | sed '$d')
        if echo "$response" | grep -q "RequestRateLimitExceeded" && [ "$http_code" = "403" ]; then
            echo "wireserver rate limited (HTTP ${http_code}) on attempt ${attempt}/${max_retries}: ${url}" >&2
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
        elif [ "$http_code" -ge 200 ] 2>/dev/null && [ "$http_code" -lt 300 ] 2>/dev/null; then
            echo "$response"
            return 0
        else
            echo "wireserver request failed (HTTP ${http_code}) on attempt ${attempt}/${max_retries}: ${url}" >&2
            sleep $retry_delay
        fi
        attempt=$((attempt + 1))
    done
    echo "exhausted all retries for ${url} (last HTTP ${http_code})" >&2
    return 1
}

function process_cert_operations {
    local endpoint_type="$1"
    local operation_response
    operation_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json")
    local request_status=$?
    if [ -z "$operation_response" ] || [ $request_status -ne 0 ]; then
        echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json"
        return 1
    fi
    local -a cert_filenames
    mapfile -t cert_filenames < <(echo "$operation_response" | grep -oP '(?<="ResouceFileName": ")[^"]*')
    if [ ${#cert_filenames[@]} -eq 0 ]; then
        echo "No certificate filenames found in response for $endpoint_type"
        return 1
    fi
    local cert_filename sanitized_filename filename extension cert_content
    for cert_filename in "${cert_filenames[@]}"; do
        sanitized_filename=$(basename -- "$cert_filename")
        if [ "$sanitized_filename" != "$cert_filename" ] || [ -z "$sanitized_filename" ] ||
           [ "$sanitized_filename" = "." ] || [ "$sanitized_filename" = ".." ]; then
            echo "Warning: rejecting certificate filename with path separators or traversal: '$cert_filename'"
            continue
        fi
        filename="${sanitized_filename%.*}"
        extension="${sanitized_filename##*.}"
        cert_content=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension")
        request_status=$?
        if [ -z "$cert_content" ] || [ $request_status -ne 0 ]; then
            echo "Warning: No response received or request failed for: ${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension"
            continue
        fi
        echo "$cert_content" > "/root/AzureCACertificates/$sanitized_filename"
    done
}
