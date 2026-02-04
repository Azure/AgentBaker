#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

IS_MARINER=0
IS_AZURELINUX=0
# shellcheck disable=SC3010
if [[ -f /etc/os-release ]]; then
        . /etc/os-release
    # shellcheck disable=SC3010
    if [[ $NAME == *"Mariner"* ]]; then
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

# Copy all certificate files to the Mariner/AzureLinux system certificate directory
cp /root/AzureCACertificates/*.crt /etc/pki/ca-trust/source/anchors/

# Update the system certificate store using Mariner/AzureLinux command
/usr/bin/update-ca-trust

# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
action=${1:-init}
if [ "$action" = "ca-refresh" ]; then
    exit
fi

# Determine an absolute, canonical path to this script for use in cron.
if command -v readlink >/dev/null 2>&1; then
    # Use readlink -f when available to resolve the canonical path; fall back to $0 on error.
    SCRIPT_PATH="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
fi

if ! crontab -l 2>/dev/null | grep -q "\"$SCRIPT_PATH\" ca-refresh"; then
    # Quote the script path in the cron entry to avoid issues with spaces or special characters.
    if ! (crontab -l 2>/dev/null ; printf '%s\n' "0 19 * * * \"$SCRIPT_PATH\" ca-refresh") | crontab -; then
        echo "Failed to install ca-refresh cron job via crontab" >&2
    fi
fi

function init_mariner_repo_depot {
    local repodepot_endpoint=$1
    echo "Adding [extended] repo"
    cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-extended.repo
    sed -i -e "s|extras|extended|" /etc/yum.repos.d/mariner-extended.repo
    sed -i -e "s|Extras|Extended|" /etc/yum.repos.d/mariner-extended.repo

    echo "Adding [nvidia] repo"
    cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-nvidia.repo
    sed -i -e "s|extras|nvidia|" /etc/yum.repos.d/mariner-nvidia.repo
    sed -i -e "s|Extras|Nvidia|" /etc/yum.repos.d/mariner-nvidia.repo

    echo "Adding [cloud-native] repo"
    cp /etc/yum.repos.d/mariner-extras.repo /etc/yum.repos.d/mariner-cloud-native.repo
    sed -i -e "s|extras|cloud-native|" /etc/yum.repos.d/mariner-cloud-native.repo
    sed -i -e "s|Extras|Cloud-Native|" /etc/yum.repos.d/mariner-cloud-native.repo

    echo "Pointing Mariner repos at RepoDepot..."
    for f in /etc/yum.repos.d/*.repo
    do
        sed -i -e "s|https://packages.microsoft.com|${repodepot_endpoint}/mariner/packages.microsoft.com|" $f
        echo "$f modified."
    done
    echo "Mariner repo setup complete."
}

function init_azurelinux_repo_depot {
    local repodepot_endpoint=$1
    repos=("amd" "base" "cloud-native" "extended" "ms-non-oss" "ms-oss" "nvidia")

    # tbd maybe we do this a bit nicer
    rm -f /etc/yum.repos.d/azurelinux*

    for repo in "${repos[@]}"; do
        output_file="/etc/yum.repos.d/azurelinux-${repo}.repo"
        repo_content=(
            "[azurelinux-official-$repo]"
            "name=Azure Linux Official $repo \$releasever \$basearch"
            "baseurl=$repodepot_endpoint/azurelinux/\$releasever/prod/$repo/\$basearch"
            "gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY"
            "gpgcheck=1"
            "repo_gpgcheck=1"
            "enabled=1"
            "skip_if_unavailable=True"
            "sslverify=1"
        )

        rm -f "$output_file"

        for line in "${repo_content[@]}"; do
            echo "$line" >> "$output_file"
        done

        echo "File '$output_file' has been created."
    done
}

cloud-init status --wait

marinerRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
if [ -z "$marinerRepoDepotEndpoint" ]; then
  >&2 echo "repo depot endpoint empty while running custom-cloud init script"
else
  # logic taken from https://repodepot.azure.com/scripts/cloud-init/setup_repodepot.sh
  if [ "$IS_MARINER" -eq 1 ]; then
      echo "Initializing Mariner repo depot settings..."
      init_mariner_repo_depot ${marinerRepoDepotEndpoint}
  elif [ "$IS_AZURELINUX" -eq 1 ]; then
      echo "Initializing Azure Linux repo depot settings..."
      init_azurelinux_repo_depot ${marinerRepoDepotEndpoint}
  else
      echo "No customizations for distribution: $NAME"
  fi
fi

#EOF
