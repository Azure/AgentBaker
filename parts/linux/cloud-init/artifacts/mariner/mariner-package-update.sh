#!/usr/bin/env bash

set -o nounset
set -e

# Global constants used in this file.
# -------------------------------------------------------------------------------------------------
OS_RELEASE_FILE="/etc/os-release"
SECURITY_PATCH_REPO_DIR="/etc/yum.repos.d"
KUBECONFIG="/var/lib/kubelet/kubeconfig"
KUBECTL="/usr/local/bin/kubectl --kubeconfig ${KUBECONFIG}"
KUBELET_EXECUTABLE="/usr/local/bin/kubelet"
SECURITY_PATCH_TMP_DIR="/tmp/security-patch"

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

tdnf_download() {
    package_name=$1
    repo=$2
    download_dir=$3
    retries=5
    dnf_download_output=/tmp/dnf-update.out
    for i in $(seq 1 $retries); do
        ! (tdnf install --repo "$repo" "$package_name" -y --downloadonly --downloaddir "$download_dir" 2>&1 | tee $dnf_download_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_download_output && break || \
        cat $dnf_download_output

        if [ $i -eq $retries ]; then
        return 1
        else sleep 5
        fi
    done
    echo Executed dnf install $package_name -y $i times
}

kubelet_update() {
    versionID=$(grep '^VERSION_ID=' ${OS_RELEASE_FILE} | cut -d'=' -f2 | tr -d '"')
    if [ "${versionID}" != "3.0" ]; then
        echo "kubelet patch is only supported on azurelinux 3.0, skipping kubelet update"
        return 0
    fi

    target_kubelet_version=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-kubelet-version']}")
    if [ -z "${target_kubelet_version}" ]; then
        echo "target kubelet version is not set, skip kubelet update"
        return 0
    fi
    echo "target kubelet version to update to is: ${target_kubelet_version}"

    if [ ! -f ${KUBELET_EXECUTABLE} ]; then
        echo "kubelet executable not found at ${KUBELET_EXECUTABLE}"
        return 1
    fi
    current_kubelet_version=$(${KUBELET_EXECUTABLE} --version | awk '{print $2}')
    current_kubelet_version=${current_kubelet_version#v}
    echo "current kubelet version is: ${current_kubelet_version}"

    current_major_minor=$(echo "$current_kubelet_version" | cut -d. -f1,2)
    target_major_minor=$(echo "$target_kubelet_version" | cut -d. -f1,2)

    if [ "$current_major_minor" != "$target_major_minor" ]; then
        echo "kubelet major.minor version mismatch: current ${current_kubelet_version}, target ${target_kubelet_version}"
        return 1
    fi

    # We may still need to patch even if the target version is the same as the current version because their release version may be different
    if [ "$(printf "%s\n%s\n" "$current_kubelet_version" "$target_kubelet_version" | sort -V | head -n1)" != "$current_kubelet_version" ]; then
        echo "Skip kubelet update since target_kubelet_version ($target_kubelet_version) is older than current_kubelet_version ($current_kubelet_version)"
        return 0
    fi

    rm -rf ${SECURITY_PATCH_TMP_DIR} && mkdir -p ${SECURITY_PATCH_TMP_DIR}

    tdnf_download kubelet-${target_kubelet_version} azurelinux-official-cloud-native ${SECURITY_PATCH_TMP_DIR}
    rpm2cpio ${SECURITY_PATCH_TMP_DIR}/kubelet-${target_kubelet_version}*.rpm | cpio -idmv -D ${SECURITY_PATCH_TMP_DIR}
    target_kubelet_path="${SECURITY_PATCH_TMP_DIR}/usr/bin/kubelet"
    if [ ! -f ${target_kubelet_path} ]; then
        echo "kubelet binary not found in the downloaded package"
        return 1
    fi
    chmod +x ${target_kubelet_path}

    target_kubelet_sha256=$(sha256sum ${target_kubelet_path} | awk '{print $1}')
    current_kubelet_sha256=$(sha256sum ${KUBELET_EXECUTABLE}| awk '{print $1}')
    if [ "${target_kubelet_sha256}" = "${current_kubelet_sha256}" ]; then
        echo "kubelet binary is the same, no need to update"
        return 0
    fi
    echo "updating kubelet from ${current_kubelet_version} (sha256: ${current_kubelet_sha256}) to version ${target_kubelet_version} (sha256: ${target_kubelet_sha256})"
    echo "current kubelet raw version: $(${KUBELET_EXECUTABLE} --version=raw)"
    echo "target kubelet raw version: $(${target_kubelet_path} --version=raw)"
    mv ${target_kubelet_path} ${KUBELET_EXECUTABLE}
    systemctl restart kubelet.service
    echo "kubelet update completed successfully"

    rm -rf ${SECURITY_PATCH_TMP_DIR}
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
                azurelinux-cloud-native.repo \
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

    if ! kubelet_update; then
        echo "kubelet_update failed"
        exit 1
    fi

    # update current timestamp
    $KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

    echo "package update completed successfully"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------
main "$@"
