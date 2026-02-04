#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

IS_FLATCAR=0
IS_UBUNTU=0
# shellcheck disable=SC3010
if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    # shellcheck disable=SC3010
    if [[ $NAME == *"Ubuntu"* ]]; then
        IS_UBUNTU=1
    elif [[ $ID == *"flatcar"* ]]; then
        IS_FLATCAR=1
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

function init_ubuntu_main_repo_depot {
    local repodepot_endpoint="$1"
    # Initialize directory for keys
    mkdir -p /etc/apt/keyrings

    # This copies the updated bundle to the location used by OpenSSL which is commonly used
    echo "Copying updated bundle to OpenSSL .pem file..."
    cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem
    echo "Updated bundle copied."

    # Back up sources.list and sources.list.d contents
    mkdir -p /etc/apt/backup/
    if [ -f "/etc/apt/sources.list" ]; then
        mv /etc/apt/sources.list /etc/apt/backup/
    fi
    for sources_file in /etc/apt/sources.list.d/*; do
        if [ -f "$sources_file" ]; then
            mv "$sources_file" /etc/apt/backup/
        fi
    done

    # Set location of sources file
    . /etc/os-release
    aptSourceFile="/etc/apt/sources.list.d/ubuntu.sources"

    # Create main sources file
    cat <<EOF > /etc/apt/sources.list.d/ubuntu.sources

Types: deb
URIs: ${repodepot_endpoint}/ubuntu
Suites: ${VERSION_CODENAME} ${VERSION_CODENAME}-updates ${VERSION_CODENAME}-backports ${VERSION_CODENAME}-security
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
EOF

    # Update the apt sources file using the RepoDepot Ubuntu URL for this cloud. Update it by replacing
    # all urls with the RepoDepot Ubuntu url
    ubuntuUrl=${repodepot_endpoint}/ubuntu
    echo "Converting URLs in $aptSourceFile to RepoDepot URLs..."
    sed -i "s,https\?://.[^ ]*,$ubuntuUrl,g" $aptSourceFile
    echo "apt source URLs converted, see new file below:"
    echo ""
    echo "-----"
    cat $aptSourceFile
    echo "-----"
    echo ""
}

function check_url {
    local url=$1
    echo "Checking url: $url"

    # Use curl to check the URL and capture both stdout and stderr
    curl_exit_code=$(curl -s --head --request GET $url)
    # Check the exit status of curl
    # shellcheck disable=SC3010
    if [[ $? -ne 0 ]] || echo "$curl_exit_code" | grep -E "404 Not Found" > /dev/null; then
        echo "ERROR: $url is not available. Please manually check if the url is valid before re-running script"
        exit 1
    fi
}

function write_to_sources_file {
    local sources_list_d_file=$1
    local source_uri=$2
    shift 2
    local key_paths=("$@")

    sources_file_path="/etc/apt/sources.list.d/${sources_list_d_file}.sources"
    ubuntuDist=$(lsb_release -c | awk '{print $2}')

    tee -a $sources_file_path <<EOF

Types: deb
URIs: $source_uri
Suites: $ubuntuDist
Components: main
Arch: amd64
Signed-By: ${key_paths[*]}
EOF
}

function add_key_ubuntu {
    local key_name=$1

    key_url="${repodepot_endpoint}/keys/${key_name}"
    check_url $key_url
    echo "Adding $key_name key to keyring..."
    key_data=$(wget -O - $key_url)
    key_path=$(derive_key_paths $key_name)
    echo "$key_data" | gpg --dearmor | tee $key_path > /dev/null
    echo "$key_name key added to keyring."
}

function derive_key_paths {
    local key_names=("$@")
    local key_paths=()

    for key_name in "${key_names[@]}"; do
        key_paths+=("/etc/apt/keyrings/${key_name}.gpg")
    done

    echo "${key_paths[*]}"
}

function add_ms_keys {
    # Add the Microsoft package server keys to keyring.
    echo "Adding Microsoft keys to keyring..."

    add_key_ubuntu microsoft.asc
    add_key_ubuntu msopentech.asc
}

function aptget_update {
    echo "apt-get updating..."
    echo "note: depending on how many sources have been added this may take a couple minutes..."
    if apt-get update | grep -q "404 Not Found"; then
        echo "ERROR: apt-get update failed to find all sources. Please validate the sources or remove bad sources from your sources and try again."
        exit 1
    else
        echo "apt-get update complete!"
    fi
}

function init_ubuntu_pmc_repo_depot {
    local repodepot_endpoint="$1"
    # Add Microsoft packages source to the azure specific sources.list.
    echo "Adding the packages.microsoft.com Ubuntu-$ubuntuRel repo..."

    microsoftPackageSource="$repodepot_endpoint/microsoft/ubuntu/$ubuntuRel/prod"
    check_url $microsoftPackageSource
    write_to_sources_file microsoft-prod $microsoftPackageSource $(derive_key_paths microsoft.asc msopentech.asc)
    write_to_sources_file microsoft-prod-testing $microsoftPackageSource $(derive_key_paths microsoft.asc msopentech.asc)
    echo "Ubuntu ($ubuntuRel) repo added."
    echo "Adding packages.microsoft.com keys"
    add_ms_keys $repodepot_endpoint
}

if [ "$IS_UBUNTU" -eq 1 ]; then
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

    cloud-init status --wait
    rootRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
    # logic taken from https://repodepot.azure.com/scripts/cloud-init/setup_repodepot.sh
    ubuntuRel=$(lsb_release --release | awk '{print $2}')
    ubuntuDist=$(lsb_release -c | awk '{print $2}')
    # initialize archive.ubuntu.com repo
    init_ubuntu_main_repo_depot ${rootRepoDepotEndpoint}
    init_ubuntu_pmc_repo_depot ${rootRepoDepotEndpoint}
    # update apt list
    echo "Running apt-get update"
    aptget_update
elif [ "$IS_FLATCAR" -eq 1 ]; then
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
