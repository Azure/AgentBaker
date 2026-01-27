#!/usr/bin/env bash

set -o nounset
set -e

# Global constants used in this file. 
# -------------------------------------------------------------------------------------------------
OS_RELEASE_FILE="/etc/os-release"
SECURITY_PATCH_REPO_DIR="/etc/yum.repos.d"
KUBECONFIG="/var/lib/kubelet/kubeconfig"
KUBECTL="/usr/local/bin/kubectl --kubeconfig ${KUBECONFIG}"

# Function definitions used in this file. 
# functions defined until "${__SOURCED__:+return}" are sourced and tested in -
# spec/parts/linux/cloud-init/artifacts/mariner-package-update_spec.sh.
# -------------------------------------------------------------------------------------------------
dnf_update() {
    retries=10
    dnf_update_output=/tmp/dnf-update.out
    versionID=$(grep '^VERSION_ID=' ${OS_RELEASE_FILE} | cut -d'=' -f2 | tr -d '"')
    if [ "${versionID}" = "3.0" ]; then
        # Convert the golden timestamp (format: YYYYMMDDTHHMMSSZ) to a timestamp in seconds
        # e.g. 20250623T000000Z -> 2025-06-23 00:00:00 -> 1750636800
        snapshottime=$(date -d "$(echo ${golden_timestamp} | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)T\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)Z/\1-\2-\3 \4:\5:\6/')" +%s)
        echo "using snapshottime ${snapshottime} for azurelinux 3.0 snapshot-based update"
        update_cmd="tdnf --snapshottime ${snapshottime}"
        repo_list=(--repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia) 
    else
        update_cmd="dnf"
        repo_list=(--repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia) 
    fi
    for i in $(seq 1 $retries); do
        ! ($update_cmd update \
            --exclude mshv-linuxloader \
            --exclude kernel-mshv \
            "${repo_list[@]}" \
            -y --refresh 2>&1 | tee $dnf_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_update_output && break || \
        cat $dnf_update_output

        if [ $i -eq $retries ]; then
        return 1
        else sleep 5
        fi
    done
    echo Executed dnf update -y --refresh $i times
}

main() {
    # At startup, we need to wait for kubelet to finish TLS bootstrapping to create the kubeconfig file.
    n=0
    while [ ! -f ${KUBECONFIG} ]; do
        echo 'Waiting for TLS bootstrapping'
        if [ "$n" -lt 100 ]; then
            n=$((n+1))
            sleep 3
        else
            echo "timeout waiting for kubeconfig to be present"
            exit 1
        fi
    done

    node_name=$(hostname)
    if [ -z "${node_name}" ]; then
        echo "cannot get node name"
        exit 1
    fi

    # Azure cloud provider assigns node name as the lowner case of the hostname
    node_name=$(echo "$node_name" | tr '[:upper:]' '[:lower:]')

    # retrieve golden timestamp from node annotation
    golden_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-golden-timestamp']}")
    if [ -z "${golden_timestamp}" ]; then
        echo "golden timestamp is not set, skip live patching"
        exit 0
    fi
    echo "golden timestamp is: ${golden_timestamp}"

    current_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-current-timestamp']}")
    if [ -n "${current_timestamp}" ]; then
        echo "current timestamp is: ${current_timestamp}"

        if [ "${golden_timestamp}" = "${current_timestamp}" ]; then
            echo "golden and current timestamp is the same, nothing to patch"
            exit 0
        fi
    fi

    # Network isolated cluster can't access the internet, so we deploy a live patching repo service in the cluster
    # The node will use the live patching repo service to download the repo metadata and packages
    # If the annotation is not set, we will use the ubuntu snapshot repo
    live_patching_repo_service=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-repo-service']}")
    # Limit the live patching repo service to private IPs in the range of 10.x.x.x, 172.16.x.x - 172.31.x.x, and 192.168.x.x
    private_ip_regex="^((10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})|(192\.168\.[0-9]{1,3}\.[0-9]{1,3}))$"
    # shellcheck disable=SC3010
    if [ -n "${live_patching_repo_service}" ] && [[ ! "${live_patching_repo_service}" =~ $private_ip_regex ]]; then
        echo "Ignore invalid live patching repo service: ${live_patching_repo_service}"
        live_patching_repo_service=""
    fi
    for repo in mariner-official-base.repo \
                mariner-microsoft.repo \
                mariner-extras.repo \
                mariner-nvidia.repo \
                azurelinux-official-base.repo \
                azurelinux-ms-non-oss.repo \
                azurelinux-ms-oss.repo \
                azurelinux-nvidia.repo; do
        repo_path="${SECURITY_PATCH_REPO_DIR}/${repo}"
        if [ -f ${repo_path} ]; then
            old_repo=$(cat ${repo_path})
            if [ -z "${live_patching_repo_service}" ]; then
                echo "live patching repo service is not set, use PMC repo"
                # upgrade from live patching repo service to PMC repo
                # e.g. replace http://10.224.0.5 with https://packages.microsoft.com
                # add a trailing comments to avoid the sed command (contains #...) being considered as trailing comment and removed
                original_endpoint=$(sed -nE 's|^# original_baseurl=(https?://.*packages\.microsoft\.com).*|\1|p' ${repo_path} | head -1) # extract original endpoint
                # if original_endpoint is not set, use https://packages.microsoft.com as the default endpoint
                if [ -z "${original_endpoint}" ]; then
                    original_endpoint="https://packages.microsoft.com"
                fi
                sed -i 's|http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+|'"${original_endpoint}"'|g' ${repo_path}
                sed -i '/^# original_baseurl=/d' ${repo_path} # remove original_baseurl comment
            else
                echo "live patching repo service is: ${live_patching_repo_service}, use it to replace PMC repo" 
                original_endpoint=$(sed -nE 's|^baseurl=(https?://.*packages\.microsoft\.com).*|\1|p' ${repo_path} | head -1)
                # upgrade from PMC to live patching repo service
                # e.g. replace https://packages.microsoft.com with http://10.224.0.5
                sed -Ei 's/^baseurl=https?:\/\/.*packages.microsoft.com/baseurl=http:\/\/'"${live_patching_repo_service}"'/g' ${repo_path}
                # upgrade the old live patching repo service to the new one
                # e.g. replace http://10.224.0.5 with http://10.224.0.6
                sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+/http:\/\/'"${live_patching_repo_service}"'/g' ${repo_path}
                # save the original PMC endpoint in the repo file so that we can revert back to it if needed 
                if [ -n "${original_endpoint}" ]; then
                    if grep -q 'original_baseurl=' ${repo_path}; then
                        sed -i 's|^# original_baseurl=.*$|# original_baseurl='"${original_endpoint}"'|g' ${repo_path} # update existing original_baseurl comment
                    else
                        sed -i '1i# original_baseurl='"${original_endpoint}"'' ${repo_path} # add original_baseurl comment
                    fi
                fi
            fi
            new_repo=$(cat ${repo_path})
            if [ "${old_repo}" != "${new_repo}" ]; then
                # No diff command in mariner, so just log if the repo is changed 
                echo "${repo_path} is updated"
            fi
        fi
    done

    if ! dnf_update; then
        echo "dnf_update failed"
        exit 1
    fi

    # update current timestamp
    $KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

    echo "package update completed successfully"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------
main "$@"