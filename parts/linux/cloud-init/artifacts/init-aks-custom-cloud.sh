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

function is_opted_in_for_root_certs {
    local opt_in_response

    opt_in_response=$(make_request_with_retry "${WIRESERVER_ENDPOINT}/acms/isOptedInForRootCerts")
    local request_status=$?
    if [ $request_status -ne 0 ] || [ -z "$opt_in_response" ]; then
        echo "Warning: failed to determine IsOptedInForRootCerts state"
        return 1
    fi

    if echo "$opt_in_response" | grep -q "IsOptedInForRootCerts=true"; then
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

# Action values:
# - init: normal provisioning path
# - ca-refresh: scheduled refresh path
action=${1:-init}
requested_cert_endpoint_mode="${2:-}"

cert_endpoint_mode=""
if [ "$action" = "ca-refresh" ] && [ -n "$requested_cert_endpoint_mode" ]; then
    cert_endpoint_mode="${requested_cert_endpoint_mode,,}"
else
    location_normalized="${LOCATION,,}"
    location_normalized="${location_normalized//[[:space:]]/}"
    if [ -z "$location_normalized" ]; then
        echo "Warning: LOCATION is empty; defaulting custom cloud certificate endpoint mode to rcv1p"
    fi

    cert_endpoint_mode="rcv1p"
    case "$location_normalized" in
        ussec*|usnat*) cert_endpoint_mode="legacy" ;;
    esac
fi

echo "Using custom cloud certificate endpoint mode: ${cert_endpoint_mode}"
rm -f /root/AzureCACertificates/*
if [ "$cert_endpoint_mode" = "legacy" ]; then
    if retrieve_legacy_certs; then
        install_certs_to_trust_store
    else
        echo "Warning: failed to retrieve legacy certificates from wireserver; continuing without trust store updates"
    fi
elif [ "$cert_endpoint_mode" = "rcv1p" ]; then
    if is_opted_in_for_root_certs; then
        if retrieve_rcv1p_certs; then
            install_certs_to_trust_store
        else
            echo "Warning: failed to retrieve rcv1p certificates from wireserver; continuing without trust store updates"
        fi
    fi
fi

# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
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
    for f in /etc/yum.repos.d/*.repo; do
        sed -i -e "s|https://packages.microsoft.com|${repodepot_endpoint}/mariner/packages.microsoft.com|" $f
        echo "$f modified."
    done
    echo "Mariner repo setup complete."
}

function init_azurelinux_repo_depot {
    local repodepot_endpoint=$1
    local repos=("amd" "base" "cloud-native" "extended" "ms-non-oss" "ms-oss" "nvidia")

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
    echo "Azure Linux repo setup complete."
}

function dnf_makecache {
    local retries=10
    local dnf_makecache_output=/tmp/dnf-makecache.out
    local i
    for i in $(seq 1 $retries); do
        ! (dnf makecache -y 2>&1 | tee $dnf_makecache_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_makecache_output && break || \
        cat $dnf_makecache_output
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep 5
        fi
    done
    echo "Executed dnf makecache -y $i times"
}

if [ "$IS_UBUNTU" -eq 1 ] || [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
    scriptPath=$0
    # Determine an absolute, canonical path to this script for use in cron.
    if command -v readlink >/dev/null 2>&1; then
        # Use readlink -f when available to resolve the canonical path; fall back to $0 on error.
        scriptPath="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
    fi

    if ! crontab -l 2>/dev/null | grep -q "\"$scriptPath\" ca-refresh"; then
        # Quote the script path in the cron entry to avoid issues with spaces or special characters.
        if ! (crontab -l 2>/dev/null ; printf '%s\n' "0 19 * * * \"$scriptPath\" ca-refresh \"$cert_endpoint_mode\"") | crontab -; then
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
elif [ "$IS_FLATCAR" -eq 1 ] || [ "$IS_ACL" -eq 1 ]; then
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
ExecStart=$script_path ca-refresh $cert_endpoint_mode
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

if [ "$IS_UBUNTU" -eq 1 ]; then
    rootRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
    if [ -n "$rootRepoDepotEndpoint" ]; then
        cloud-init status --wait
        ubuntuRel=$(lsb_release --release | awk '{print $2}')
        ubuntuDist=$(lsb_release -c | awk '{print $2}')
        init_ubuntu_main_repo_depot ${rootRepoDepotEndpoint}
        init_ubuntu_pmc_repo_depot ${rootRepoDepotEndpoint}
        echo "Running apt-get update"
        aptget_update
    else
        echo "REPO_DEPOT_ENDPOINT empty, skipping Ubuntu RepoDepot initialization"
    fi
elif [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
    cloud-init status --wait

    marinerRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
    if [ -z "$marinerRepoDepotEndpoint" ]; then
        >&2 echo "repo depot endpoint empty while running custom-cloud init script"
    else
        if [ "$IS_MARINER" -eq 1 ]; then
            echo "Initializing Mariner repo depot settings..."
            init_mariner_repo_depot ${marinerRepoDepotEndpoint}
            dnf_makecache || exit 1
        else
            echo "Initializing Azure Linux repo depot settings..."
            init_azurelinux_repo_depot ${marinerRepoDepotEndpoint}
            dnf_makecache || exit 1
        fi
    fi
fi

# Disable systemd-timesyncd and install chrony and uses local time source
# ACL has PTP clock config compiled into chronyd with no config file or sourcedir directives,
# so it uses only the local PTP clock and has no DHCP-injectable NTP sources.
if [ "$IS_ACL" -eq 1 ]; then
    echo "Skipping chrony configuration for ACL (PTP clock baked into chronyd, no external NTP sources)"
elif [ "$IS_MARINER" -eq 1 ] || [ "$IS_AZURELINUX" -eq 1 ]; then
cat > /etc/chrony.conf <<EOF
# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/drift

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

systemctl restart chronyd
else
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
fi

#EOF
