#!/usr/bin/env bash

# OS-specific securityPatch handler, sourced by mariner-package-update.sh.
# Owns RPM repositories, package/kubelet updates, and legacy security patching.
: "${OS_RELEASE_FILE:=/etc/os-release}"
SECURITY_PATCH_REPO_DIR="/etc/yum.repos.d"
: "${KUBECONFIG:=/var/lib/kubelet/kubeconfig}"
: "${KUBECTL:=/opt/bin/kubectl --kubeconfig ${KUBECONFIG}}"
KUBELET_EXECUTABLE="/opt/bin/kubelet"
SECURITY_PATCH_TMP_DIR="/tmp/security-patch"
CLUSTER_CA_CERT="/etc/kubernetes/certs/ca.crt"

knead_emit_security_patch_event() {
    local outcome="$1"
    local message="$2"
    local level="${3:-Informational}"
    if declare -F knead_emit_event > /dev/null; then
        knead_emit_event "AKS.LivePatching.securityPatch.${outcome}" "${message}" "${level}" || true
    fi
}

knead_emit_security_patch_failure_event() {
    knead_emit_security_patch_event Failed "reason=$1" Error
}

reconcileDualKernelBoot() {
    local boot_dir="${BOOT_DIR:-/boot}"
    local module_source="${GRUB_MODULE_SOURCE:-/usr/lib/grub/arm64-efi}"
    local grub_config="${boot_dir}/grub2/grub.cfg"
    local kernel_package kernel_version grub_versions
    local kernel_versions=()

    for kernel_package in kernel kernel-hwe; do
        kernel_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}\n' "$kernel_package" 2>/dev/null | sort -V | tail -n1)
        if [ -z "$kernel_version" ] || [ ! -s "${boot_dir}/vmlinuz-${kernel_version}" ] || [ ! -s "${boot_dir}/initramfs-${kernel_version}.img" ]; then
            echo "Dual-kernel image is missing boot files for ${kernel_package}" >&2
            return 1
        fi
        kernel_versions+=("$kernel_version")
    done

    if ! grub_versions=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}\n' grub2 grub2-efi-binary grub2-efi 2>/dev/null) || [ "$(printf '%s\n' "$grub_versions" | sort -u | wc -l)" -ne 1 ]; then
        echo "Dual-kernel image has mismatched GRUB packages" >&2
        return 1
    fi
    if [ ! -s "${module_source}/smbios.mod" ]; then
        echo "Missing GRUB module file: ${module_source}/smbios.mod" >&2
        return 1
    fi

    grub2-mkconfig -o "$grub_config" || return 1
    grub2-script-check "$grub_config" || return 1
    for kernel_version in "${kernel_versions[@]}"; do
        grep -Fq "vmlinuz-${kernel_version}" "$grub_config" || { echo "GRUB config is missing kernel ${kernel_version}" >&2; return 1; }
    done
}

dnf_update() {
    local selected_golden_timestamp="${1:-${golden_timestamp:-}}"
    retries=10
    dnf_update_output=/tmp/dnf-update.out
    versionID=$(grep '^VERSION_ID=' "${OS_RELEASE_FILE}" | cut -d'=' -f2 | tr -d '"')
    local should_reconcile_dual_kernel=false
    if [ "${versionID}" = "3.0" ]; then
        # Convert the golden timestamp (format: YYYYMMDDTHHMMSSZ) to a timestamp in seconds
        # e.g. 20250623T000000Z -> 2025-06-23 00:00:00 -> 1750636800
        snapshottime=$(date -d "$(printf '%s' "${selected_golden_timestamp}" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)T\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)Z/\1-\2-\3 \4:\5:\6/')" +%s)
        echo "using snapshottime ${snapshottime} for azurelinux 3.0 snapshot-based update"
        update_cmd="tdnf --snapshottime ${snapshottime}"
        repo_list=(--repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia)
        if [ "$(uname -m)" = "aarch64" ] && rpm -q kernel kernel-hwe &>/dev/null; then
            should_reconcile_dual_kernel=true
        fi
    else
        update_cmd="dnf"
        repo_list=(--repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia)
    fi
    for i in $(seq 1 $retries); do
        set +e
        $update_cmd update \
            --exclude mshv-linuxloader \
            --exclude kernel-mshv \
            "${repo_list[@]}" \
            -y --refresh > "$dnf_update_output" 2>&1
        local update_status=$?
        set -e
        cat "$dnf_update_output"
        if [ "${update_status}" -eq 0 ] && ! grep -Eq "^([WE]:.*)|([eE]rr.*)$" "$dnf_update_output"; then
            break
        fi

        if [ "$i" -eq "$retries" ]; then
        return 1
        else sleep 5
        fi
    done
    if $should_reconcile_dual_kernel; then
        reconcileDualKernelBoot || return 1
    fi
    echo Executed dnf update -y --refresh "$i" times
}

tdnf_download() {
    package_name=$1
    repo=$2
    download_dir=$3
    retries=5
    dnf_download_output=/tmp/dnf-update.out
    for i in $(seq 1 $retries); do
        # Try install first; if the package is already installed with the same version,
        # tdnf install will say "already installed" and skip the download, so we fall back to reinstall.
        tdnf install --repo "$repo" "$package_name" -y --downloadonly --downloaddir "$download_dir" 2>&1 | tee $dnf_download_output
        if grep -qi "already installed" $dnf_download_output; then
            echo "package already installed, falling back to tdnf reinstall"
            tdnf reinstall --repo "$repo" "$package_name" -y --downloadonly --downloaddir "$download_dir" 2>&1 | tee $dnf_download_output
        fi
        if ! grep -E "^([WE]:.*)|([eE]rr.*)$" $dnf_download_output; then
            cat $dnf_download_output && break
        fi
        cat $dnf_download_output

        if [ "$i" -eq "$retries" ]; then
        return 1
        else sleep 5
        fi
    done
    echo Executed tdnf download "$package_name" -y "$i" times
}

redact_token() {
    local s="$1"
    # shellcheck disable=SC3010
    if [[ "$s" == *"sig="* ]]; then
        printf '%s\n' "$s" | sed 's/sig=[^&]*/sig=***REDACTED***/g'
    else
        echo "$s"
    fi
}

kubelet_update() {
    local target_node_name="${1:-${node_name:-}}"
    local configured_target_version="${2:-}"
    local target_source="${3:-legacy}"

    versionID=$(grep '^VERSION_ID=' "${OS_RELEASE_FILE}" | cut -d'=' -f2 | tr -d '"')
    if [ "${versionID}" != "3.0" ]; then
        echo "kubelet patch is only supported on azurelinux 3.0, skipping kubelet update"
        return 0
    fi

    target_kubelet_version=""
    kubelet_url=""
    kubelet_url_with_token=""
    if ! custom_patching=$($KUBECTL get node "${target_node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-custom-patching']}"); then
        echo "failed to read custom patching annotation"
        knead_emit_security_patch_failure_event CustomPatchingAnnotationReadFailed
        return 1
    fi
    if [ "${custom_patching}" = "true" ]; then
        echo "custom patching is enabled, retrieving target kubelet version from custom patching service"
        cluster_ip=$($KUBECTL get svc kubernetes -o jsonpath="{.spec.clusterIP}")
        if [ -z "${cluster_ip}" ]; then
            echo "cannot get kubernetes cluster IP"
            return 1
        fi
        imds_token=$(curl --max-time 30 -s -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/attested/document?api-version=2025-04-07" | jq -r .signature)
        if [ -z "${imds_token}" ] || [ "${imds_token}" = "null" ]; then
            echo "cannot get IMDS token"
            return 1
        fi
        endpoint="aks-security-patch.data.mcr.microsoft.com"
        response=$(curl --max-time 30 -s -w "%{http_code}" -H "Authorization: ${imds_token}" --cacert "${CLUSTER_CA_CERT}" --resolve "${endpoint}:443:${cluster_ip}" "https://${endpoint}/v1/packages")
        packages="${response::-3}"
        status="${response: -3}"
        if [ "${status}" != "200" ]; then
            echo "failed to get custom patching info, status code: ${status}"
            return 1
        fi
        while IFS= read -r package; do
            name=$(echo "${package}" | jq -r '.name')
            if [ "${name}" = "kubelet" ]; then
                target_kubelet_version=$(echo "${package}" | jq -r '.version')
                kubelet_url=$(echo "${package}" | jq -r '.url')
                token=$(echo "${package}" | jq -r '.token')
                kubelet_url_with_token="${kubelet_url}?${token}"
                echo "retrieved target kubelet version: ${target_kubelet_version} from custom patching service"
                break
            fi
        done < <(echo "${packages}" | jq -c '.packages[]')
    elif [ "${target_source}" = "generic" ]; then
        target_kubelet_version="${configured_target_version}"
        echo "custom patching is not enabled, retrieved target kubelet version from securityPatch profile: ${target_kubelet_version}"
    else
        target_kubelet_version=$($KUBECTL get node "${target_node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-kubelet-version']}")
        echo "custom patching is not enabled, retrieved target kubelet version from node annotation: ${target_kubelet_version}"
    fi

    if [ -z "${target_kubelet_version}" ]; then
        echo "target kubelet version is not set, skip kubelet update"
        return 0
    fi

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

    if [ "${custom_patching}" = "true" ]; then
        package_name=$(basename "${kubelet_url}")
        curl_output=$(curl --max-time 300 -fsSL -o "${SECURITY_PATCH_TMP_DIR}/${package_name}" "${kubelet_url_with_token}" 2>&1)
        curl_exit_code=$?
        if [ "${curl_exit_code}" -ne 0 ]; then
            echo "failed to download kubelet package from custom patching service: $(redact_token "${curl_output}")" >&2
            return 1
        fi
    else
        tdnf_download "kubelet-${target_kubelet_version}" azurelinux-official-cloud-native "${SECURITY_PATCH_TMP_DIR}"
    fi

    rpm2cpio ${SECURITY_PATCH_TMP_DIR}/*kubelet*.rpm | cpio -idmv -D ${SECURITY_PATCH_TMP_DIR}
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
    local kubelet_backup="${SECURITY_PATCH_TMP_DIR}/kubelet.backup"
    if ! cp --preserve=mode,ownership,timestamps "${KUBELET_EXECUTABLE}" "${kubelet_backup}"; then
        echo "failed to back up kubelet executable"
        return 1
    fi
    if ! mv "${target_kubelet_path}" "${KUBELET_EXECUTABLE}"; then
        echo "failed to replace kubelet executable"
        knead_emit_security_patch_failure_event KubeletReplacementFailed
        return 1
    fi
    if ! systemctl restart kubelet.service; then
        echo "failed to restart kubelet.service, restoring previous kubelet"
        knead_emit_security_patch_failure_event KubeletRestartFailed
        if ! mv "${kubelet_backup}" "${KUBELET_EXECUTABLE}" || ! systemctl restart kubelet.service; then
            echo "failed to restore previous kubelet"
            knead_emit_security_patch_failure_event KubeletRestoreFailed
        fi
        return 1
    fi
    rm -f "${kubelet_backup}"
    echo "kubelet update completed successfully"

    rm -rf ${SECURITY_PATCH_TMP_DIR}
}

apply_updates() {
    local target_node_name="$1"
    local target_golden_timestamp="$2"
    local target_kubelet_version="${3:-}"
    local target_source="${4:-legacy}"
    local node_json="${5:-}"
    local live_patching_repo_service

    if [ -n "${node_json}" ]; then
        live_patching_repo_service=$(printf '%s' "${node_json}" | jq -r '(.metadata.annotations // {})["kubernetes.azure.com/live-patching-repo-service"] // empty') || {
            knead_emit_security_patch_failure_event RepositoryEndpointReadFailed
            return 1
        }
    else
        live_patching_repo_service=$($KUBECTL get node "${target_node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-repo-service']}") || {
            knead_emit_security_patch_failure_event RepositoryEndpointReadFailed
            return 1
        }
    fi
    if ! rewrite_repos "${live_patching_repo_service}"; then
        echo "failed to rewrite security patch repositories"
        knead_emit_security_patch_failure_event RepositoryRewriteFailed
        return 1
    fi

    if ! dnf_update "${target_golden_timestamp}"; then
        echo "dnf_update failed"
        knead_emit_security_patch_failure_event PackageUpdateFailed
        return 1
    fi
    if ! kubelet_update "${target_node_name}" "${target_kubelet_version}" "${target_source}"; then
        echo "kubelet_update failed"
        knead_emit_security_patch_failure_event KubeletUpdateFailed
        return 1
    fi
    # shellcheck disable=SC2086
    if ! $KUBECTL annotate --overwrite node "${target_node_name}" "kubernetes.azure.com/live-patching-current-timestamp=${target_golden_timestamp}"; then
        echo "failed to update legacy securityPatch status annotation"
        knead_emit_security_patch_failure_event LegacyStatusAnnotationFailed
        return 1
    fi
    echo "package update completed successfully"
    knead_emit_security_patch_event Applied "goldenTimestamp=${target_golden_timestamp}"
}

rewrite_repos() {
    local live_patching_repo_service="$1"
    local private_ip_regex="^((10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})|(192\.168\.[0-9]{1,3}\.[0-9]{1,3}))$"
    local repo repo_path old_repo new_repo original_endpoint

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
            old_repo=$(cat "${repo_path}") || return 1
            if [ -z "${live_patching_repo_service}" ]; then
                echo "live patching repo service is not set, use PMC repo"
                original_endpoint=$(printf '%s\n' "${old_repo}" | sed -nE 's|^#[[:space:]]original_baseurl=(https?://.*packages\.microsoft\.com).*|\1|p') || return 1
                original_endpoint="${original_endpoint%%$'\n'*}"
                if [ -z "${original_endpoint}" ]; then
                    original_endpoint="https://packages.microsoft.com"
                fi
                sed -i 's|http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+|'"${original_endpoint}"'|g' "${repo_path}" || return 1
                sed -i '/^#[[:space:]]original_baseurl=/d' "${repo_path}" || return 1
            else
                echo "live patching repo service is: ${live_patching_repo_service}, use it to replace PMC repo"
                original_endpoint=$(printf '%s\n' "${old_repo}" | sed -nE 's|^baseurl=(https?://.*packages.microsoft.com).*|\1|p') || return 1
                original_endpoint="${original_endpoint%%$'\n'*}"
                sed -Ei 's/^baseurl=https?:\/\/.*packages.microsoft.com/baseurl=http:\/\/'"${live_patching_repo_service}"'/g' "${repo_path}" || return 1
                sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+/http:\/\/'"${live_patching_repo_service}"'/g' "${repo_path}" || return 1
                if [ -n "${original_endpoint}" ]; then
                    if [ "${old_repo#*original_baseurl=}" != "${old_repo}" ]; then
                        sed -i 's|^#[[:space:]]original_baseurl=.*$|#\ original_baseurl='"${original_endpoint}"'|g' "${repo_path}" || return 1
                    else
                        sed -i '1i#\ original_baseurl='"${original_endpoint}"'' "${repo_path}" || return 1
                    fi
                fi
            fi
            new_repo=$(cat "${repo_path}") || return 1
            if [ "${old_repo}" != "${new_repo}" ]; then
                echo "${repo_path} is updated"
            fi
        fi
    done
}

legacy_main() {
    node_name=$(hostname)
    if [ -z "${node_name}" ]; then
        echo "cannot get node name"
        exit 1
    fi

    # Azure cloud provider assigns node name as the lowner case of the hostname
    node_name=$(echo "$node_name" | tr '[:upper:]' '[:lower:]')

    # retrieve golden timestamp from node annotation
    golden_timestamp=$($KUBECTL get node "${node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-golden-timestamp']}")
    if [ -z "${golden_timestamp}" ]; then
        echo "golden timestamp is not set, skip live patching"
        exit 0
    fi
    echo "golden timestamp is: ${golden_timestamp}"

    current_timestamp=$($KUBECTL get node "${node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-current-timestamp']}")
    if [ -n "${current_timestamp}" ]; then
        echo "current timestamp is: ${current_timestamp}"

        if [ "${golden_timestamp}" = "${current_timestamp}" ]; then
            echo "golden and current timestamp is the same, nothing to patch"
            exit 0
        fi
    fi

    apply_updates "${node_name}" "${golden_timestamp}"
}

selected_security_patch_profile() {
    local component_payload="$1"
    local agent_pool="$2"

    printf '%s' "${component_payload}" | jq -ce --arg agentPool "${agent_pool}" '
        if has("agentPools") then
            if (.agentPools | type) != "object" then error("agentPools must be an object")
            elif (.agentPools | has($agentPool)) then
                .agentPools[$agentPool] as $profile |
                if ($profile | type) != "object" or ($profile.goldenTimestamp | type) != "string" or
                   (($profile | has("kubeletVersion")) and ($profile.kubeletVersion | type) != "string")
                then error("invalid selected profile")
                else $profile | {goldenTimestamp: .goldenTimestamp, kubeletVersion: (.kubeletVersion // "")} end
            else null end
        else null end
    ' 2> /dev/null
}

securityPatchIsCurrent() {
    local component_payload="$1"
    local current_payload="$2"
    local node_json="$3"
    local agent_pool
    local desired_profile
    local current_profile

    agent_pool=$(printf '%s' "${node_json}" | jq -er '.metadata.labels["kubernetes.azure.com/agentpool"] // empty') || return 1
    desired_profile=$(selected_security_patch_profile "${component_payload}" "${agent_pool}") || return 1
    current_profile=$(selected_security_patch_profile "${current_payload}" "${agent_pool}") || return 1
    [ "${desired_profile}" = "${current_profile}" ]
}

updateSecurityPatch() {
    local component_payload="$1"
    local node_json="$2"
    local agent_pool
    local node_name
    local agent_pools_type
    local golden_timestamp
    local kubelet_version
    local timestamp_date

    node_name=$(printf '%s' "${node_json}" | jq -er '.metadata.name // empty') || {
        knead_emit_security_patch_failure_event NodeNameMissing
        return 1
    }
    agent_pool=$(printf '%s' "${node_json}" | jq -er '.metadata.labels["kubernetes.azure.com/agentpool"] // empty') || {
        echo "node agent pool label is not set"
        knead_emit_security_patch_failure_event AgentPoolLabelMissing
        return 1
    }
    agent_pools_type=$(printf '%s' "${component_payload}" | jq -er 'if has("agentPools") then (.agentPools | type) else "missing" end' 2> /dev/null) || {
        echo "securityPatch configuration is invalid"
        knead_emit_security_patch_failure_event ConfigurationInvalid
        return 1
    }
    if [ "${agent_pools_type}" = "missing" ]; then
        echo "securityPatch has no profile for agent pool ${agent_pool}; no action needed"
        knead_emit_security_patch_event NoAction "No profile for agent pool ${agent_pool}"
        return 0
    fi
    if [ "${agent_pools_type}" != "object" ]; then
        echo "securityPatch agentPools must be an object"
        knead_emit_security_patch_failure_event AgentPoolsInvalid
        return 1
    fi
    if ! printf '%s' "${component_payload}" | jq -e --arg agentPool "${agent_pool}" '.agentPools | has($agentPool)' > /dev/null; then
        echo "securityPatch has no profile for agent pool ${agent_pool}; no action needed"
        knead_emit_security_patch_event NoAction "No profile for agent pool ${agent_pool}"
        return 0
    fi
    if ! printf '%s' "${component_payload}" | jq -e --arg agentPool "${agent_pool}" '
        (.agentPools[$agentPool] | type == "object") and
        (.agentPools[$agentPool].goldenTimestamp | type == "string") and
        ((.agentPools[$agentPool] | has("kubeletVersion") | not) or (.agentPools[$agentPool].kubeletVersion | type == "string"))
    ' > /dev/null 2>&1; then
        echo "securityPatch profile is invalid for agent pool: ${agent_pool}"
        knead_emit_security_patch_failure_event ProfileInvalid
        return 1
    fi
    # Validate the complete JSON string before command substitution can trim newlines.
    if ! printf '%s' "${component_payload}" | jq -e --arg agentPool "${agent_pool}" '
        .agentPools[$agentPool].goldenTimestamp | (length == 16) and test("^[0-9]{8}T[0-9]{6}Z$")
    ' > /dev/null; then
        echo "securityPatch goldenTimestamp is invalid"
        knead_emit_security_patch_failure_event GoldenTimestampInvalid
        return 1
    fi
    golden_timestamp=$(printf '%s' "${component_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp') || return 1
    kubelet_version=$(printf '%s' "${component_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].kubeletVersion // empty') || return 1
    timestamp_date=$(printf '%s' "${golden_timestamp}" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)T\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)Z/\1-\2-\3 \4:\5:\6/')
    if ! date -d "${timestamp_date}" > /dev/null 2>&1; then
        echo "securityPatch goldenTimestamp is invalid: ${golden_timestamp}"
        knead_emit_security_patch_failure_event GoldenTimestampInvalid
        return 1
    fi
    apply_updates "${node_name}" "${golden_timestamp}" "${kubelet_version}" generic "${node_json}"
}
