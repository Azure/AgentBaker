#!/bin/bash
# This script handles repo depot initialization and chrony configuration for
# AKS custom cloud environments. It is sourced by init-aks-custom-cloud.sh and
# inherits all variables from it (IS_UBUNTU, IS_MARINER, IS_AZURELINUX,
# IS_FLATCAR, IS_ACL, REPO_DEPOT_ENDPOINT, etc.).
#
# This script is only included in custom cloud images to keep the base
# customData size small for non-custom-cloud scenarios.

set -x

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
# real-time clock. Note that it can't be used along with the 'rtcfile' directive.
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
# real-time clock. Note that it can't be used along with the 'rtcfile' directive.
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
