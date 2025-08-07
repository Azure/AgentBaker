#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

# http://168.63.129.16 is a constant for the host's wireserver endpoint
WIRESERVER_ENDPOINT="http://168.63.129.16"

# Function to process certificate operations from a given endpoint
process_cert_operations() {
    local endpoint_type="$1"
    local operation_response
    
    echo "Retrieving certificate operations for type: $endpoint_type"
    operation_response=$(curl "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$endpoint_type&ext=json")
    
    if [ -z "$operation_response" ]; then
        echo "Warning: No response received for $endpoint_type"
        return
    fi
    
    # Extract ResourceFileName values from the JSON response
    local cert_filenames
    cert_filenames=($(echo "$operation_response" | grep -oP '(?<="ResouceFileName": ")[^"]*'))
    
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
        
        # Retrieve the actual certificate content
        local cert_content
        cert_content=$(curl "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=$filename&ext=$extension")
        
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

# Copy all certificate files to the system certificate directory
if [ -n "$(find /root/AzureCACertificates -name '*.crt' 2>/dev/null)" ]; then
    cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/
    echo "Copied certificate files to /usr/local/share/ca-certificates/"
else
    echo "Warning: No .crt files found to copy"
fi

# Update the system certificate store
/usr/sbin/update-ca-certificates

# This copies the updated bundle to the location used by OpenSSL which is commonly used
cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem

# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
action=${1:-init}
if [ "$action" = "ca-refresh" ]; then
    exit
fi

(crontab -l ; echo "0 19 * * * $0 ca-refresh") | crontab -

#EOF