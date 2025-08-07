#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

# http://168.63.129.16 is a constant for the host's wireserver endpoint
WIRESERVER_ENDPOINT="http://168.63.129.16"

# Function to make HTTP request with retry logic for rate limiting
make_request_with_retry() {
    local url="$1"
    local max_retries=5
    local retry_delay=3
    local attempt=1
    
    while [ $attempt -le $max_retries ]; do
        local response
        response=$(curl -s "$url")
        
        # Check if response contains rate limiting error
        if echo "$response" | grep -q "RequestRateLimitExceeded"; then
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))  # Exponential backoff
            attempt=$((attempt + 1))
        else
            # Request succeeded or failed with different error
            echo "$response"
            return 0
        fi
    done
    
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
        echo "Warning: No response received or request failed for $endpoint_type"
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
            echo "Warning: No response received or request failed for $cert_filename"
            return
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

# Copy all certificate files to the Mariner/AzureLinux system certificate directory
if [ -n "$(find /root/AzureCACertificates -name '*.crt' 2>/dev/null)" ]; then
    cp /root/AzureCACertificates/*.crt /etc/pki/ca-trust/source/anchors/
    echo "Copied certificate files to /etc/pki/ca-trust/source/anchors/"
else
    echo "Warning: No .crt files found to copy"
fi

# Update the system certificate store using Mariner/AzureLinux command
/usr/bin/update-ca-trust

#EOF