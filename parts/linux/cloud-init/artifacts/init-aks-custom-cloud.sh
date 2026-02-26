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

# The purpose of RCV 1P is to reliably distribute root and intermediate certificates at scale to
# only Microsoft 1st party (1P) virtual machines (VM) and virtual machine scale sets (VMSS).
# This is critical for initiatives such as Microsoft PKI. RCV 1P ensures that these certificates
# are installed on the node at creation time. This eliminates the need for your VM to be connected
# to the internet and ping an endpoint to receive certificate packages. The feature also eliminates
# the dependency on updates to AzSecPack to receive the latest root and intermediate certs.
# RCV 1P is designed to work completely autonomously from the user perspective on all Azure 1st
# party VMs.

# Below code calls the wireserver to get the list of CA certs for this cloud and saves them to /root/AzureCACertificates. Then it copies them to the appropriate location based on distro and updates the CA bundle.

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

IFS_backup=$IFS
IFS=$'\r\n'

# First check via curl "http://168.63.129.16/acms/isOptedInForRootCerts" and JSON response for
# {"IsOptedInForRootCerts":true}. The value captured in optInCheck indicates whether THIS VM
# is opted in for the RCV 1P PKI setup described above. If not opted in, skip the RCV 1P pull
# path and use the default cert retrieval flow instead. This check can be removed if you want to
# attempt to pull certs regardless of opt-in status, but it may result in errors in the logs if
# not opted in.
# https://eng.ms/docs/products/onecert-certificates-key-vault-and-dsms/onecert-customer-guide/autorotationandecr/rcv1ptsg

optInCheck=""
if optInCheck=$(curl -sS --fail "http://168.63.129.16/acms/isOptedInForRootCerts"); then
    :
else
    echo "Warning: failed to query root cert opt-in status; defaulting to non-opt-in flow"
fi

if echo "$optInCheck" | grep -Eq '"IsOptedInForRootCerts"[[:space:]]*:[[:space:]]*true'; then
    echo "Opted in for root certs, proceeding with CA cert pull and install"
    # Process root certificates
    process_cert_operations "operationrequestsroot"

    # Process intermediate certificates
    process_cert_operations "operationrequestsintermediate"
    echo "successfully pulled in root certs"
else
    echo "Not opted in for root certs, skipping CA cert pull and install"
    # http://168.63.129.16 is a constant for the host's wireserver endpoint
    certs=$(curl "http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json")
    certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
    certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
    ext=".crt"
    if [ "$IS_FLATCAR" -eq 1 ]; then
        ext=".pem"
    fi
    for i in ${!certBodies[@]}; do
        echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${certNames[$i]} | sed "s/.cer/.${ext}/g")"
    done
    echo "successfully pulled in default certs"
fi

IFS=$IFS_backup

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
    scriptPath=$0
    # Determine an absolute, canonical path to this script for use in cron.
    if command -v readlink >/dev/null 2>&1; then
        # Use readlink -f when available to resolve the canonical path; fall back to $0 on error.
        scriptPath="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
    fi

    if ! crontab -l 2>/dev/null | grep -q "\"$scriptPath\" ca-refresh"; then
        # Quote the script path in the cron entry to avoid issues with spaces or special characters.
        if ! (crontab -l 2>/dev/null ; printf '%s\n' "0 19 * * * \"$scriptPath\" ca-refresh") | crontab -; then
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

# Disable systemd-timesyncd and install chrony and uses local time source
chrony_conf="/etc/chrony/chrony.conf"
if [ "$IS_UBUNTU" -eq 1 ]; then
    systemctl stop systemd-timesyncd
    systemctl disable systemd-timesyncd

    if [ ! -e "$chrony_conf" ]; then
        apt-get update
        apt-get install chrony -y
    fi
elif [ "$IS_FLATCAR" -eq 1 ]; then
    rm -f ${chrony_conf}
fi

cat > $chrony_conf <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usuable directives.

# This will use (up to):
# - 4 sources from ntp.ubuntu.com which some are ipv6 enabled
# - 2 sources from 2.ubuntu.pool.ntp.org which is ipv6 enabled as well
# - 1 source from [01].ubuntu.pool.ntp.org each (ipv4 only atm)
# This means by default, up to 6 dual-stack and up to 2 additional IPv4-only
# sources will be used.
# At the same time it retains some protection against one of the entries being
# down (compare to just using one of the lines). See (LP: #1754358) for the
# discussion.
#
# About using servers from the NTP Pool Project in general see (LP: #104525).
# Approved by Ubuntu Technical Board on 2011-02-08.
# See http://www.pool.ntp.org/join.html for more information.
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

if [ "$IS_UBUNTU" -eq 1 ]; then
    systemctl restart chrony
elif [ "$IS_FLATCAR" -eq 1 ]; then
    systemctl restart chronyd
fi

#EOF
