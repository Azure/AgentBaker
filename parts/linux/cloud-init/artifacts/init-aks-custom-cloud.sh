#!/bin/bash
[ -n "${__SOURCED__:-}" ] || set -x

# Path constants — overridable for testing via env vars; defaults preserve
# production behavior. functions defined until "${__SOURCED__:+return}" are
# sourced and tested in spec/parts/linux/cloud-init/artifacts/init_aks_custom_cloud_spec.sh.
: "${OS_RELEASE_FILE:=/etc/os-release}"
: "${AZURE_CA_CERTS_DIR:=/root/AzureCACertificates}"
: "${CA_TRUST_ANCHORS_DIR:=/etc/pki/ca-trust/source/anchors}"
: "${SSL_CERTS_DIR:=/etc/ssl/certs}"
: "${LOCAL_SHARE_CA_CERTS_DIR:=/usr/local/share/ca-certificates}"
: "${OPENSSL_CERT_FILE:=/usr/lib/ssl/cert.pem}"
: "${APT_SOURCES_LIST:=/etc/apt/sources.list}"
: "${APT_SOURCES_LIST_D_DIR:=/etc/apt/sources.list.d}"
: "${APT_KEYRINGS_DIR:=/etc/apt/keyrings}"
: "${APT_BACKUP_DIR:=/etc/apt/backup}"
: "${SYSTEMD_SYSTEM_DIR:=/etc/systemd/system}"
: "${CHRONY_CONF_FILE:=/etc/chrony/chrony.conf}"
# http://168.63.129.16 is a constant for the host's wireserver endpoint
: "${WIRESERVER_ENDPOINT:=http://168.63.129.16}"

detect_distro() {
    IS_FLATCAR=0
    IS_UBUNTU=0
    IS_ACL=0
    # shellcheck disable=SC3010
    if [[ -f "${OS_RELEASE_FILE}" ]]; then
        . "${OS_RELEASE_FILE}"
        # shellcheck disable=SC3010
        if [[ $NAME == *"Ubuntu"* ]]; then
            IS_UBUNTU=1
        elif [[ $ID == *"flatcar"* ]]; then
            IS_FLATCAR=1
        elif [[ $ID == "azurecontainerlinux" ]] || { [[ $ID == "azurelinux" ]] && [[ ${VARIANT_ID:-} == "azurecontainerlinux" ]]; }; then
            IS_ACL=1
        else
            echo "Unknown Linux distribution"
            exit 1
        fi
    else
        echo "Unsupported operating system"
        exit 1
    fi

    echo "Running on $NAME"
}

fetch_and_install_ca_certs() {
    mkdir -p "${AZURE_CA_CERTS_DIR}"
    certs=$(curl "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=cacertificates&ext=json")
    IFS_backup=$IFS
    IFS=$'\r\n'
    certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
    certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
    ext=".crt"
    if [ "$IS_FLATCAR" -eq 1 ]; then
        ext=".pem"
    fi
    for i in ${!certBodies[@]}; do
        echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "${AZURE_CA_CERTS_DIR}/$(echo ${certNames[$i]} | sed "s/.cer/.${ext}/g")"
    done
    IFS=$IFS_backup

    if [ "$IS_ACL" -eq 1 ]; then
        cp "${AZURE_CA_CERTS_DIR}"/*.crt "${CA_TRUST_ANCHORS_DIR}/"
        update-ca-trust
    elif [ "$IS_FLATCAR" -eq 1 ]; then
        cp "${AZURE_CA_CERTS_DIR}"/*.pem "${SSL_CERTS_DIR}/"
        update-ca-certificates
    else
        cp "${AZURE_CA_CERTS_DIR}"/*.crt "${LOCAL_SHARE_CA_CERTS_DIR}/"
        update-ca-certificates

        # This copies the updated bundle to the location used by OpenSSL which is commonly used
        cp "${SSL_CERTS_DIR}/ca-certificates.crt" "${OPENSSL_CERT_FILE}"
    fi
}

init_ubuntu_main_repo_depot() {
    local repodepot_endpoint="$1"
    # Initialize directory for keys
    mkdir -p "${APT_KEYRINGS_DIR}"

    # This copies the updated bundle to the location used by OpenSSL which is commonly used
    echo "Copying updated bundle to OpenSSL .pem file..."
    cp "${SSL_CERTS_DIR}/ca-certificates.crt" "${OPENSSL_CERT_FILE}"
    echo "Updated bundle copied."

    # Back up sources.list and sources.list.d contents
    mkdir -p "${APT_BACKUP_DIR}/"
    if [ -f "${APT_SOURCES_LIST}" ]; then
        mv "${APT_SOURCES_LIST}" "${APT_BACKUP_DIR}/"
    fi
    for sources_file in "${APT_SOURCES_LIST_D_DIR}"/*; do
        if [ -f "$sources_file" ]; then
            mv "$sources_file" "${APT_BACKUP_DIR}/"
        fi
    done

    # Set location of sources file
    . "${OS_RELEASE_FILE}"
    aptSourceFile="${APT_SOURCES_LIST_D_DIR}/ubuntu.sources"

    # Create main sources file
    cat <<EOF > "${aptSourceFile}"

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

check_url() {
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

write_to_sources_file() {
    local sources_list_d_file=$1
    local source_uri=$2
    shift 2
    local key_paths=("$@")

    sources_file_path="${APT_SOURCES_LIST_D_DIR}/${sources_list_d_file}.sources"
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

add_key_ubuntu() {
    local key_name=$1

    key_url="${repodepot_endpoint}/keys/${key_name}"
    check_url $key_url
    echo "Adding $key_name key to keyring..."
    key_data=$(wget -O - $key_url)
    key_path=$(derive_key_paths $key_name)
    echo "$key_data" | gpg --dearmor | tee $key_path > /dev/null
    echo "$key_name key added to keyring."
}

derive_key_paths() {
    local key_names=("$@")
    local key_paths=()

    for key_name in "${key_names[@]}"; do
        key_paths+=("${APT_KEYRINGS_DIR}/${key_name}.gpg")
    done

    echo "${key_paths[*]}"
}

add_ms_keys() {
    # Add the Microsoft package server keys to keyring.
    echo "Adding Microsoft keys to keyring..."

    add_key_ubuntu microsoft.asc
    add_key_ubuntu msopentech.asc
}

aptget_update() {
    echo "apt-get updating..."
    echo "note: depending on how many sources have been added this may take a couple minutes..."
    if apt-get update | grep -q "404 Not Found"; then
        echo "ERROR: apt-get update failed to find all sources. Please validate the sources or remove bad sources from your sources and try again."
        exit 1
    else
        echo "apt-get update complete!"
    fi
}

init_ubuntu_pmc_repo_depot() {
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

setup_ubuntu_ca_refresh_cron() {
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
}

setup_flatcar_or_acl_ca_refresh_timer() {
    script_path="$(readlink -f "$0")"
    svc="${SYSTEMD_SYSTEM_DIR}/azure-ca-refresh.service"
    tmr="${SYSTEMD_SYSTEM_DIR}/azure-ca-refresh.timer"

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
}

init_ubuntu_repo_depot() {
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
}

write_chrony_config() {
    # Disable systemd-timesyncd and install chrony and uses local time source
    # ACL has PTP clock config compiled into chronyd with no config file or sourcedir directives,
    # so it uses only the local PTP clock and has no DHCP-injectable NTP sources.
    if [ "$IS_ACL" -eq 1 ]; then
        echo "Skipping chrony configuration for ACL (PTP clock baked into chronyd, no external NTP sources)"
        return
    fi

    if [ "$IS_UBUNTU" -eq 1 ]; then
        systemctl stop systemd-timesyncd
        systemctl disable systemd-timesyncd

        if [ ! -e "${CHRONY_CONF_FILE}" ]; then
            apt-get update
            apt-get install chrony -y
        fi
    elif [ "$IS_FLATCAR" -eq 1 ]; then
        rm -f "${CHRONY_CONF_FILE}"
    fi

    cat > "${CHRONY_CONF_FILE}" <<EOF
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
}

main() {
    detect_distro
    fetch_and_install_ca_certs

    # This section creates a cron job to poll for refreshed CA certs daily
    # It can be removed if not needed or desired
    action=${1:-init}
    if [ "$action" = "ca-refresh" ]; then
        exit
    fi

    if [ "$IS_UBUNTU" -eq 1 ]; then
        setup_ubuntu_ca_refresh_cron
        init_ubuntu_repo_depot
    elif [ "$IS_FLATCAR" -eq 1 ] || [ "$IS_ACL" -eq 1 ]; then
        setup_flatcar_or_acl_ca_refresh_timer
    fi

    write_chrony_config
}

${__SOURCED__:+return}
main "$@"

#EOF
