#!/bin/bash
# functions defined until "${__SOURCED__:+return}" are sourced and tested in -
# spec/parts/linux/cloud-init/artifacts/init_aks_cloud_spec.sh.

# Dependency note: `jq` is guaranteed to be present on every AKS VHD (baked in
# by vhdbuilder/packer/install-dependencies.sh and shipped in the Azure Linux
# base image), so functions in this script use it without an explicit install
# step. Do not flag jq usage here as "used before install" — matches the
# established pattern in cse_main.sh.

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

function init_ubuntu_main_repo_depot {
    local repodepot_endpoint="$1"
    local keyrings_dir="${APT_KEYRINGS_DIR:-/etc/apt/keyrings}"
    local ssl_certs_dir="${SSL_CERTS_DIR:-/etc/ssl/certs}"
    local ssl_cert_target="${SSL_CERT_TARGET:-/usr/lib/ssl/cert.pem}"
    local backup_dir="${APT_BACKUP_DIR:-/etc/apt/backup}"
    local sources_list="${APT_SOURCES_LIST:-/etc/apt/sources.list}"
    local sources_list_d="${APT_SOURCES_LIST_D_DIR:-/etc/apt/sources.list.d}"
    local os_release_file="${OS_RELEASE_FILE:-/etc/os-release}"

    # Initialize directories for keys and apt sources. mkdir -p is a no-op when the
    # default paths already exist; it makes the *_DIR overrides used by tests robust.
    mkdir -p "$keyrings_dir" "$sources_list_d"

    # This copies the updated bundle to the location used by OpenSSL which is commonly used.
    echo "Copying updated bundle to OpenSSL .pem file..."
    cp "${ssl_certs_dir}/ca-certificates.crt" "$ssl_cert_target"
    echo "Updated bundle copied."

    # Back up sources.list and sources.list.d contents
    mkdir -p "$backup_dir"
    if [ -f "$sources_list" ]; then
        mv "$sources_list" "$backup_dir/"
    fi
    for sources_file in "${sources_list_d}"/*; do
        if [ -f "$sources_file" ]; then
            mv "$sources_file" "$backup_dir/"
        fi
    done

    # Set location of sources file
    # shellcheck disable=SC1090
    . "$os_release_file"
    local aptSourceFile="${sources_list_d}/ubuntu.sources"

    # Create main sources file
    cat <<EOF > "$aptSourceFile"

Types: deb
URIs: ${repodepot_endpoint}/ubuntu
Suites: ${VERSION_CODENAME} ${VERSION_CODENAME}-updates ${VERSION_CODENAME}-backports ${VERSION_CODENAME}-security
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
EOF

    # Update the apt sources file using the RepoDepot Ubuntu URL for this cloud. Update it by replacing
    # all urls with the RepoDepot Ubuntu url
    local ubuntuUrl="${repodepot_endpoint}/ubuntu"
    echo "Converting URLs in $aptSourceFile to RepoDepot URLs..."
    sed -i "s,https\?://.[^ ]*,$ubuntuUrl,g" "$aptSourceFile"
    echo "apt source URLs converted, see new file below:"
    echo ""
    echo "-----"
    cat "$aptSourceFile"
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
        emit_event "AKS.CSE.customCloudRepoInit.checkUrlFailed" "url=$url not reachable" "Error"
        exit 1
    fi
}

function write_to_sources_file {
    local sources_list_d_file=$1
    local source_uri=$2
    shift 2
    local key_paths=("$@")
    local sources_list_d="${APT_SOURCES_LIST_D_DIR:-/etc/apt/sources.list.d}"
    mkdir -p "$sources_list_d"

    local sources_file_path="${sources_list_d}/${sources_list_d_file}.sources"
    local ubuntuDist
    ubuntuDist=$(lsb_release -c | awk '{print $2}')

    tee -a "$sources_file_path" <<EOF

Types: deb
URIs: $source_uri
Suites: $ubuntuDist
Components: main
Arch: amd64
Signed-By: ${key_paths[*]}
EOF
}

function add_key_ubuntu {
    local key_name="$1"
    local endpoint="$2"

    local key_url="${endpoint}/keys/${key_name}"
    check_url "$key_url"
    echo "Adding $key_name key to keyring..."
    local key_data
    key_data=$(wget -O - "$key_url")
    local key_path
    key_path=$(derive_key_paths "$key_name")
    echo "$key_data" | gpg --dearmor | tee "$key_path" > /dev/null
    echo "$key_name key added to keyring."
}

function derive_key_paths {
    local key_names=("$@")
    local key_paths=()
    local keyrings_dir="${APT_KEYRINGS_DIR:-/etc/apt/keyrings}"

    for key_name in "${key_names[@]}"; do
        key_paths+=("${keyrings_dir}/${key_name}.gpg")
    done

    echo "${key_paths[*]}"
}

function add_ms_keys {
    local endpoint="$1"
    # Add the Microsoft package server keys to keyring.
    echo "Adding Microsoft keys to keyring..."

    add_key_ubuntu microsoft.asc "$endpoint"
    add_key_ubuntu msopentech.asc "$endpoint"
}

function aptget_update {
    echo "apt-get updating..."
    echo "note: depending on how many sources have been added this may take a couple minutes..."
    if apt-get update | grep -q "404 Not Found"; then
        echo "ERROR: apt-get update failed to find all sources. Please validate the sources or remove bad sources from your sources and try again."
        emit_event "AKS.CSE.customCloudRepoInit.aptgetUpdateFailed" "apt-get update returned 404 for one or more sources" "Error"
        exit 1
    else
        echo "apt-get update complete!"
    fi
}

function init_ubuntu_pmc_repo_depot {
    local repodepot_endpoint="$1"
    # Add Microsoft packages source to the azure specific sources.list.
    echo "Adding the packages.microsoft.com Ubuntu-$ubuntuRel repo..."

    local microsoftPackageSource="$repodepot_endpoint/microsoft/ubuntu/$ubuntuRel/prod"
    check_url "$microsoftPackageSource"
    write_to_sources_file microsoft-prod "$microsoftPackageSource" $(derive_key_paths microsoft.asc msopentech.asc)
    write_to_sources_file microsoft-prod-testing "$microsoftPackageSource" $(derive_key_paths microsoft.asc msopentech.asc)
    echo "Ubuntu ($ubuntuRel) repo added."
    echo "Adding packages.microsoft.com keys"
    add_ms_keys "$repodepot_endpoint"
}

function init_mariner_repo_depot {
    local repodepot_endpoint="$1"
    local yum_repos_dir="${YUM_REPOS_DIR:-/etc/yum.repos.d}"
    mkdir -p "$yum_repos_dir"

    echo "Adding [extended] repo"
    cp "${yum_repos_dir}/mariner-extras.repo" "${yum_repos_dir}/mariner-extended.repo"
    sed -i -e "s|extras|extended|" "${yum_repos_dir}/mariner-extended.repo"
    sed -i -e "s|Extras|Extended|" "${yum_repos_dir}/mariner-extended.repo"

    echo "Adding [nvidia] repo"
    cp "${yum_repos_dir}/mariner-extras.repo" "${yum_repos_dir}/mariner-nvidia.repo"
    sed -i -e "s|extras|nvidia|" "${yum_repos_dir}/mariner-nvidia.repo"
    sed -i -e "s|Extras|Nvidia|" "${yum_repos_dir}/mariner-nvidia.repo"

    echo "Adding [cloud-native] repo"
    cp "${yum_repos_dir}/mariner-extras.repo" "${yum_repos_dir}/mariner-cloud-native.repo"
    sed -i -e "s|extras|cloud-native|" "${yum_repos_dir}/mariner-cloud-native.repo"
    sed -i -e "s|Extras|Cloud-Native|" "${yum_repos_dir}/mariner-cloud-native.repo"

    echo "Pointing Mariner repos at RepoDepot..."
    for f in "${yum_repos_dir}"/*.repo; do
        sed -i -e "s|https://packages.microsoft.com|${repodepot_endpoint}/mariner/packages.microsoft.com|" "$f"
        echo "$f modified."
    done
    echo "Mariner repo setup complete."
}

function init_azurelinux_repo_depot {
    local repodepot_endpoint="$1"
    local yum_repos_dir="${YUM_REPOS_DIR:-/etc/yum.repos.d}"
    local repos=("amd" "base" "cloud-native" "extended" "ms-non-oss" "ms-oss" "nvidia")
    mkdir -p "$yum_repos_dir"

    rm -f "${yum_repos_dir}"/azurelinux*

    for repo in "${repos[@]}"; do
        local output_file="${yum_repos_dir}/azurelinux-${repo}.repo"
        local repo_content=(
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

# shellcheck disable=SC2317
${__SOURCED__:+return}
set -x

# shellcheck disable=SC3010
if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    # shellcheck disable=SC3010
    if [[ $NAME = *"Ubuntu"* ]]; then
        IS_UBUNTU=1
    elif [[ $ID = *"flatcar"* ]]; then
        IS_FLATCAR=1
    elif [[ $ID = "azurecontainerlinux" ]] || { [[ $ID = "azurelinux" ]] && [[ ${VARIANT_ID:-} = "azurecontainerlinux" ]]; }; then
        IS_ACL=1
    elif [[ $NAME = *"Mariner"* ]]; then
        IS_MARINER=1
    elif [[ $NAME = *"Microsoft Azure Linux"* ]]; then
        IS_AZURELINUX=1
    else
        echo "Unknown Linux distribution"
        exit 1
    fi
else
    echo "Unsupported operating system"
    exit 1
fi

echo "Running on $NAME"


action=${1:-init}
refresh_location="${2:-${LOCATION}}"
script_path=$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")
script_dir=$(dirname -- "$script_path")
custom_cloud_certs_script="${script_dir}/init-aks-custom-cloud-certs.sh"
if [ ! -f "$custom_cloud_certs_script" ]; then
    echo "ERROR: custom cloud certificate script is missing: $custom_cloud_certs_script" >&2
    exit 1
fi

bash "$custom_cloud_certs_script" "$action" "$refresh_location" || exit $?

if [ "$action" = "ca-refresh" ]; then
    exit
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
            dnf_makecache || { echo "ERROR: dnf_makecache failed after retries; aborting custom cloud repo init (Mariner)"; emit_event "AKS.CSE.customCloudRepoInit.dnfMakecacheFailed" "dnf_makecache failed after retries (Mariner)" "Error"; exit 1; }
        else
            echo "Initializing Azure Linux repo depot settings..."
            init_azurelinux_repo_depot ${marinerRepoDepotEndpoint}
            dnf_makecache || { echo "ERROR: dnf_makecache failed after retries; aborting custom cloud repo init (Azure Linux)"; emit_event "AKS.CSE.customCloudRepoInit.dnfMakecacheFailed" "dnf_makecache failed after retries (Azure Linux)" "Error"; exit 1; }
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