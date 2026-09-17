#!/usr/bin/env bash

set -o nounset
set -e

# Global constants used in this file.
# -------------------------------------------------------------------------------------------------
OS_RELEASE_FILE="/etc/os-release"
SECURITY_PATCH_REPO_DIR="/etc/yum.repos.d"
KUBECONFIG="/var/lib/kubelet/kubeconfig"
KUBECTL="/opt/bin/kubectl --kubeconfig ${KUBECONFIG}"
KUBELET_EXECUTABLE="/opt/bin/kubelet"
SECURITY_PATCH_TMP_DIR="/tmp/security-patch"
CLUSTER_CA_CERT="/etc/kubernetes/certs/ca.crt"
: "${LIVE_PATCHING_CONFIG_NAMESPACE:=kube-system}"
: "${LIVE_PATCHING_CONFIGMAP:=live-patching-config}"
: "${LIVE_PATCHING_CONFIG_KEY_JSONPATH:=live-patching-config\.json}"
: "${LIVE_PATCHING_GOAL_ANNOTATION:=kubernetes.azure.com/live-patching-config-goal-hash}"
: "${LIVE_PATCHING_STATUS_ANNOTATION:=kubernetes.azure.com/live-patching-status}"
: "${LIVE_PATCHING_STATE_FILE:=/var/lib/aks/live-patching/current.json}"

LIVE_PATCHING_COMPONENT_RESULTS='{}'

# Function definitions used in this file.
# functions defined until "${__SOURCED__:+return}" are sourced and tested in -
# spec/parts/linux/cloud-init/artifacts/mariner-package-update_spec.sh.
# -------------------------------------------------------------------------------------------------
dnf_update() {
    local selected_golden_timestamp="${1:-${golden_timestamp:-}}"
    retries=10
    dnf_update_output=/tmp/dnf-update.out
    versionID=$(grep '^VERSION_ID=' ${OS_RELEASE_FILE} | cut -d'=' -f2 | tr -d '"')
    if [ "${versionID}" = "3.0" ]; then
        # Convert the golden timestamp (format: YYYYMMDDTHHMMSSZ) to a timestamp in seconds
        # e.g. 20250623T000000Z -> 2025-06-23 00:00:00 -> 1750636800
        snapshottime=$(date -d "$(printf '%s' "${selected_golden_timestamp}" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)T\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)Z/\1-\2-\3 \4:\5:\6/')" +%s)
        echo "using snapshottime ${snapshottime} for azurelinux 3.0 snapshot-based update"
        update_cmd="tdnf --snapshottime ${snapshottime}"
        repo_list=(--repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia)
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

    versionID=$(grep '^VERSION_ID=' ${OS_RELEASE_FILE} | cut -d'=' -f2 | tr -d '"')
    if [ "${versionID}" != "3.0" ]; then
        echo "kubelet patch is only supported on azurelinux 3.0, skipping kubelet update"
        return 0
    fi

    target_kubelet_version=""
    kubelet_url=""
    kubelet_url_with_token=""
    custom_patching=$($KUBECTL get node "${target_node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-custom-patching']}")
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
    mv ${target_kubelet_path} ${KUBELET_EXECUTABLE}
    if ! systemctl restart kubelet.service; then
        echo "failed to restart kubelet.service"
        return 1
    fi
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
        live_patching_repo_service=$(get_node_annotation "${node_json}" "kubernetes.azure.com/live-patching-repo-service")
    else
        live_patching_repo_service=$($KUBECTL get node "${target_node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-repo-service']}")
    fi
    rewrite_repos "${live_patching_repo_service}"

    if ! dnf_update "${target_golden_timestamp}"; then
        echo "dnf_update failed"
        return 1
    fi
    if ! kubelet_update "${target_node_name}" "${target_kubelet_version}" "${target_source}"; then
        echo "kubelet_update failed"
        return 1
    fi
    # shellcheck disable=SC2086
    if ! $KUBECTL annotate --overwrite node "${target_node_name}" "kubernetes.azure.com/live-patching-current-timestamp=${target_golden_timestamp}"; then
        echo "failed to update legacy securityPatch status annotation"
        return 1
    fi
    echo "package update completed successfully"
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
            old_repo=$(cat ${repo_path})
            if [ -z "${live_patching_repo_service}" ]; then
                echo "live patching repo service is not set, use PMC repo"
                original_endpoint=$(sed -nE 's|^#[[:space:]]original_baseurl=(https?://.*packages\.microsoft\.com).*|\1|p' ${repo_path} | head -1)
                if [ -z "${original_endpoint}" ]; then
                    original_endpoint="https://packages.microsoft.com"
                fi
                sed -i 's|http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+|'"${original_endpoint}"'|g' ${repo_path}
                sed -i '/^#[[:space:]]original_baseurl=/d' ${repo_path}
            else
                echo "live patching repo service is: ${live_patching_repo_service}, use it to replace PMC repo"
                original_endpoint=$(sed -nE 's|^baseurl=(https?://.*packages.microsoft.com).*|\1|p' ${repo_path} | head -1)
                sed -Ei 's/^baseurl=https?:\/\/.*packages.microsoft.com/baseurl=http:\/\/'"${live_patching_repo_service}"'/g' ${repo_path}
                sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+/http:\/\/'"${live_patching_repo_service}"'/g' ${repo_path}
                if [ -n "${original_endpoint}" ]; then
                    if grep -q 'original_baseurl=' ${repo_path}; then
                        sed -i 's|^#[[:space:]]original_baseurl=.*$|#\ original_baseurl='"${original_endpoint}"'|g' ${repo_path}
                    else
                        sed -i '1i#\ original_baseurl='"${original_endpoint}"'' ${repo_path}
                    fi
                fi
            fi
            new_repo=$(cat ${repo_path})
            if [ "${old_repo}" != "${new_repo}" ]; then
                echo "${repo_path} is updated"
            fi
        fi
    done
}

wait_for_kubeconfig() {
    n=0
    while [ ! -f ${KUBECONFIG} ]; do
        echo 'Waiting for TLS bootstrapping'
        if [ "$n" -lt 100 ]; then
            n=$((n+1))
            sleep 3
        else
            echo "timeout waiting for kubeconfig to be present"
            return 1
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

get_node_annotation() {
    local node_json="$1"
    local annotation="$2"

    printf '%s' "${node_json}" | jq -r --arg annotation "${annotation}" '(.metadata.annotations // {})[$annotation] // empty'
}

read_generic_config() {
    local goal="$1"
    local payload
    local payload_with_sentinel
    local payload_hash

    # shellcheck disable=SC2086
    if ! payload_with_sentinel="$($KUBECTL get cm -n "${LIVE_PATCHING_CONFIG_NAMESPACE}" "${LIVE_PATCHING_CONFIGMAP}" -o "jsonpath={.data.${LIVE_PATCHING_CONFIG_KEY_JSONPATH}}" && printf '.')"; then
        echo "failed to read live-patching-config ConfigMap" >&2
        return 1
    fi
    payload="${payload_with_sentinel%.}"
    if ! printf '%s' "${payload}" | jq -e '
        (.components | type == "array") and
        (.components | all((.name | type == "string") and (.name | length > 0) and (.nodeConfig | type == "string"))) and
        ([.components[].name] | length) == ([.components[].name] | unique | length)
    ' > /dev/null; then
        echo "live-patching-config payload has invalid envelope" >&2
        return 1
    fi
    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        return 1
    fi
    payload_hash=$(printf '%s' "${payload}" | sha256sum | awk '{print $1}')
    if [ "${payload_hash}" != "${goal}" ]; then
        echo "live-patching goal hash does not match ConfigMap payload: goal=${goal}, payload=${payload_hash}" >&2
        return 1
    fi
    printf '%s' "${payload}"
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

component_is_current() {
    local component_payload="$1"
    local agent_pool="$2"
    local current_payload
    local desired_profile
    local current_profile

    [ -f "${LIVE_PATCHING_STATE_FILE}" ] || return 1
    current_payload=$(jq -er '.components.securityPatch.nodeConfig' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || return 1
    desired_profile=$(selected_security_patch_profile "${component_payload}" "${agent_pool}") || return 1
    current_profile=$(selected_security_patch_profile "${current_payload}" "${agent_pool}") || return 1
    [ "${desired_profile}" = "${current_profile}" ]
}

write_component_checkpoint() {
    local component_payload="$1"
    local state_tmp="${LIVE_PATCHING_STATE_FILE}.tmp"
    local state='{"components":{}}'

    mkdir -p "$(dirname "${LIVE_PATCHING_STATE_FILE}")" || return 1
    if [ -f "${LIVE_PATCHING_STATE_FILE}" ]; then
        state=$(jq -c 'select(.components | type == "object") | {components:.components}' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || state='{"components":{}}'
        [ -n "${state}" ] || state='{"components":{}}'
    fi
    printf '%s' "${state}" | jq --arg nodeConfig "${component_payload}" \
        '.components.securityPatch = {nodeConfig:$nodeConfig}' > "${state_tmp}" || return 1
    mv "${state_tmp}" "${LIVE_PATCHING_STATE_FILE}"
}

apply_security_patch() {
    local component_payload="$1"
    local node_json="$2"
    local agent_pool="$3"
    local node_name="$4"
    local agent_pools_type
    local golden_timestamp
    local kubelet_version
    local timestamp_date

    agent_pools_type=$(printf '%s' "${component_payload}" | jq -er 'if has("agentPools") then (.agentPools | type) else "missing" end' 2> /dev/null) || {
        echo "securityPatch configuration is invalid"
        return 1
    }
    if [ "${agent_pools_type}" = "missing" ]; then
        echo "securityPatch has no profile for agent pool ${agent_pool}; no action needed"
        return 0
    fi
    if [ "${agent_pools_type}" != "object" ]; then
        echo "securityPatch agentPools must be an object"
        return 1
    fi
    if ! printf '%s' "${component_payload}" | jq -e --arg agentPool "${agent_pool}" '.agentPools | has($agentPool)' > /dev/null; then
        echo "securityPatch has no profile for agent pool ${agent_pool}; no action needed"
        return 0
    fi
    if ! printf '%s' "${component_payload}" | jq -e --arg agentPool "${agent_pool}" '
        (.agentPools[$agentPool] | type == "object") and
        (.agentPools[$agentPool].goldenTimestamp | type == "string") and
        ((.agentPools[$agentPool] | has("kubeletVersion") | not) or (.agentPools[$agentPool].kubeletVersion | type == "string"))
    ' > /dev/null 2>&1; then
        echo "securityPatch profile is invalid for agent pool: ${agent_pool}"
        return 1
    fi
    golden_timestamp=$(printf '%s' "${component_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp')
    kubelet_version=$(printf '%s' "${component_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].kubeletVersion // empty')
    if ! printf '%s' "${golden_timestamp}" | grep -Eq '^[0-9]{8}T[0-9]{6}Z$'; then
        echo "securityPatch goldenTimestamp is invalid: ${golden_timestamp}"
        return 1
    fi
    timestamp_date=$(printf '%s' "${golden_timestamp}" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)T\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)Z/\1-\2-\3 \4:\5:\6/')
    if ! date -d "${timestamp_date}" > /dev/null 2>&1; then
        echo "securityPatch goldenTimestamp is invalid: ${golden_timestamp}"
        return 1
    fi
    apply_updates "${node_name}" "${golden_timestamp}" "${kubelet_version}" generic "${node_json}"
}

write_generic_status() {
    local node_name="$1"
    local goal="$2"
    local status

    status=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -c --arg currentHash "${goal}" '{currentHash:$currentHash,components:.}') || return 1
    # shellcheck disable=SC2086
    $KUBECTL annotate --overwrite node "${node_name}" "${LIVE_PATCHING_STATUS_ANNOTATION}=${status}"
}

generic_main() {
    local node_json="$1"
    local goal="$2"
    local node_name agent_pool status payload component_count component_name component_payload
    local component_index=0
    local failed=false

    node_name=$(printf '%s' "${node_json}" | jq -er '.metadata.name // empty') || return 1
    agent_pool=$(printf '%s' "${node_json}" | jq -er '.metadata.labels["kubernetes.azure.com/agentpool"] // empty') || {
        echo "node agent pool label is not set"
        return 1
    }
    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        return 1
    fi
    status=$(get_node_annotation "${node_json}" "${LIVE_PATCHING_STATUS_ANNOTATION}") || return 1
    if [ -n "${status}" ] && printf '%s' "${status}" | jq -e --arg goal "${goal}" \
        '.currentHash == $goal and (.components | type == "object") and (.components | all(.code == "Succeeded"))' > /dev/null 2>&1; then
        echo "live-patching goal is already converged, nothing to apply"
        return 0
    fi
    payload=$(read_generic_config "${goal}") || return 1
    component_count=$(printf '%s' "${payload}" | jq -r '.components | length') || return 1
    LIVE_PATCHING_COMPONENT_RESULTS='{}'

    while [ "${component_index}" -lt "${component_count}" ]; do
        component_name=$(printf '%s' "${payload}" | jq -r --argjson index "${component_index}" '.components[$index].name') || return 1
        component_payload=$(printf '%s' "${payload}" | jq -r --argjson index "${component_index}" '.components[$index].nodeConfig') || return 1
        case "${component_name}" in
            securityPatch)
                echo "applying component: securityPatch"
                if component_is_current "${component_payload}" "${agent_pool}"; then
                    echo "component is already current: securityPatch"
                    LIVE_PATCHING_COMPONENT_RESULTS=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -c '.securityPatch={code:"Succeeded"}')
                elif apply_security_patch "${component_payload}" "${node_json}" "${agent_pool}" "${node_name}" && write_component_checkpoint "${component_payload}"; then
                    LIVE_PATCHING_COMPONENT_RESULTS=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -c '.securityPatch={code:"Succeeded"}')
                else
                    echo "component failed: securityPatch"
                    LIVE_PATCHING_COMPONENT_RESULTS=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -c '.securityPatch={code:"Failed"}')
                    failed=true
                fi
                ;;
            *) echo "unsupported component: ${component_name}" ;;
        esac
        component_index=$((component_index + 1))
    done

    if ! write_generic_status "${node_name}" "${goal}"; then
        echo "failed to update live-patching status annotation"
        return 1
    fi
    if [ "${failed}" = true ]; then
        return 1
    fi
    echo "generic live-patching completed successfully"
}

main() {
    local version_id node_name node_json goal

    if ! wait_for_kubeconfig; then
        return 1
    fi
    version_id=$(grep '^VERSION_ID=' "${OS_RELEASE_FILE}" | cut -d'=' -f2 | tr -d '"')
    if [ "${version_id}" != "3.0" ]; then
        legacy_main
        return
    fi
    node_name=$(hostname)
    if [ -z "${node_name}" ]; then
        echo "cannot get node name"
        return 1
    fi
    node_name=$(printf '%s' "${node_name}" | tr '[:upper:]' '[:lower:]')
    # Read the goal first so legacy clusters retain the exact annotation-driven path.
    # shellcheck disable=SC2086
    goal=$($KUBECTL get node "${node_name}" -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-config-goal-hash']}") || return 1
    if [ -z "${goal}" ]; then
        legacy_main
        return
    fi
    # shellcheck disable=SC2086
    node_json=$($KUBECTL get node "${node_name}" -o json) || return 1
    echo "live-patching goal is: ${goal}"
    generic_main "${node_json}" "${goal}"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------
main "$@"
