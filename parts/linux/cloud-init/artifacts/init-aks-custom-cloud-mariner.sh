#!/bin/bash
[ -n "${__SOURCED__:-}" ] || set -x

# Path constants — overridable for testing via env vars; defaults preserve
# production behavior. functions defined until "${__SOURCED__:+return}" are
# sourced and tested in spec/parts/linux/cloud-init/artifacts/init_aks_custom_cloud_mariner_spec.sh.
: "${OS_RELEASE_FILE:=/etc/os-release}"
: "${AZURE_CA_CERTS_DIR:=/root/AzureCACertificates}"
: "${CA_TRUST_ANCHORS_DIR:=/etc/pki/ca-trust/source/anchors}"
: "${YUM_REPOS_D_DIR:=/etc/yum.repos.d}"
: "${CHRONY_CONF_FILE:=/etc/chrony.conf}"
: "${WIRESERVER_ENDPOINT:=http://168.63.129.16}"

detect_distro() {
    IS_MARINER=0
    IS_AZURELINUX=0
    # shellcheck disable=SC3010
    if [[ -f "${OS_RELEASE_FILE}" ]]; then
            . "${OS_RELEASE_FILE}"
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

    echo "Running on $NAME"
}

fetch_and_install_ca_certs() {
    mkdir -p "${AZURE_CA_CERTS_DIR}"
    # http://168.63.129.16 is a constant for the host's wireserver endpoint
    certs=$(curl "${WIRESERVER_ENDPOINT}/machine?comp=acmspackage&type=cacertificates&ext=json")
    IFS_backup=$IFS
    IFS=$'\r\n'
    certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
    certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
    for i in ${!certBodies[@]}; do
        echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "${AZURE_CA_CERTS_DIR}/$(echo ${certNames[$i]} | sed 's/.cer/.crt/g')"
    done
    IFS=$IFS_backup

    cp "${AZURE_CA_CERTS_DIR}"/*.crt "${CA_TRUST_ANCHORS_DIR}/"
    /usr/bin/update-ca-trust
}

setup_ca_refresh_cron() {
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

init_mariner_repo_depot() {
    local repodepot_endpoint=$1
    echo "Adding [extended] repo"
    cp "${YUM_REPOS_D_DIR}/mariner-extras.repo" "${YUM_REPOS_D_DIR}/mariner-extended.repo"
    sed -i -e "s|extras|extended|" "${YUM_REPOS_D_DIR}/mariner-extended.repo"
    sed -i -e "s|Extras|Extended|" "${YUM_REPOS_D_DIR}/mariner-extended.repo"

    echo "Adding [nvidia] repo"
    cp "${YUM_REPOS_D_DIR}/mariner-extras.repo" "${YUM_REPOS_D_DIR}/mariner-nvidia.repo"
    sed -i -e "s|extras|nvidia|" "${YUM_REPOS_D_DIR}/mariner-nvidia.repo"
    sed -i -e "s|Extras|Nvidia|" "${YUM_REPOS_D_DIR}/mariner-nvidia.repo"

    echo "Adding [cloud-native] repo"
    cp "${YUM_REPOS_D_DIR}/mariner-extras.repo" "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo"
    sed -i -e "s|extras|cloud-native|" "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo"
    sed -i -e "s|Extras|Cloud-Native|" "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo"

    echo "Pointing Mariner repos at RepoDepot..."
    for f in "${YUM_REPOS_D_DIR}"/*.repo
    do
        sed -i -e "s|https://packages.microsoft.com|${repodepot_endpoint}/mariner/packages.microsoft.com|" $f
        echo "$f modified."
    done
    echo "Mariner repo setup complete."
}

init_azurelinux_repo_depot() {
    local repodepot_endpoint=$1
    repos=("amd" "base" "cloud-native" "extended" "ms-non-oss" "ms-oss" "nvidia")

    # tbd maybe we do this a bit nicer
    rm -f "${YUM_REPOS_D_DIR}"/azurelinux*

    for repo in "${repos[@]}"; do
        output_file="${YUM_REPOS_D_DIR}/azurelinux-${repo}.repo"
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

dnf_makecache() {
    local retries=10
    local dnf_makecache_output=/tmp/dnf-makecache.out
    local i
    for i in $(seq 1 $retries); do
        ! (dnf makecache -y 2>&1 | tee $dnf_makecache_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_makecache_output && break || \
        cat $dnf_makecache_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo "Executed dnf makecache -y $i times"
}

init_repo_depot() {
    marinerRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
    if [ -z "$marinerRepoDepotEndpoint" ]; then
      >&2 echo "repo depot endpoint empty while running custom-cloud init script"
    else
      # logic taken from https://repodepot.azure.com/scripts/cloud-init/setup_repodepot.sh
      if [ "$IS_MARINER" -eq 1 ]; then
          echo "Initializing Mariner repo depot settings..."
          init_mariner_repo_depot ${marinerRepoDepotEndpoint}
          dnf_makecache || exit 1
      elif [ "$IS_AZURELINUX" -eq 1 ]; then
          echo "Initializing Azure Linux repo depot settings..."
          init_azurelinux_repo_depot ${marinerRepoDepotEndpoint}
          dnf_makecache || exit 1
      else
          echo "No customizations for distribution: $NAME"
      fi
    fi
}

write_chrony_config() {
    # Set the chrony config to use the PHC /dev/ptp0 clock
    cat > "${CHRONY_CONF_FILE}" <<EOF
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

    setup_ca_refresh_cron
    cloud-init status --wait
    init_repo_depot
    write_chrony_config
}

${__SOURCED__:+return}
main "$@"

#EOF
