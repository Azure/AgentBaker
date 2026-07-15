#!/usr/bin/env bash

# Applies the securityPatch component selected for this node's agent pool. This
# preserves the legacy Ubuntu snapshot-update behavior while moving desired state into the
# generic live-patching ConfigMap and status contract owned by the updater loop.

: "${SECURITY_PATCH_CONFIG_DIR:=/var/lib/security-patch}"
: "${SECURITY_PATCH_DEFAULT_ENDPOINT:=snapshot.ubuntu.com}"

security_patch_unattended_upgrade() {
    local attempt

    for attempt in $(seq 1 10); do
        if unattended-upgrade -v; then
            echo "executed unattended upgrade ${attempt} times"
            return 0
        fi
        if [ "${attempt}" -lt 10 ]; then
            sleep 5
        fi
    done

    return 1
}

security_patch_generate_sources_list() {
    local endpoint="$1"
    local golden_timestamp="$2"
    local code_name="$3"

    mkdir -p "${SECURITY_PATCH_CONFIG_DIR}" || return 1
    if [ "${endpoint}" = "${SECURITY_PATCH_DEFAULT_ENDPOINT}" ]; then
        cat <<EOF > "${SECURITY_PATCH_CONFIG_DIR}/sources.list" || return 1
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name} main restricted
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-updates main restricted
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name} universe
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-updates universe
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name} multiverse
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-updates multiverse
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-backports main restricted universe multiverse
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-security main restricted
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-security universe
deb https://${endpoint}/ubuntu/${golden_timestamp} ${code_name}-security multiverse
EOF
    else
    cat <<EOF > "${SECURITY_PATCH_CONFIG_DIR}/sources.list" || return 1
deb http://${endpoint}/ubuntu ${code_name} main restricted
deb http://${endpoint}/ubuntu ${code_name}-updates main restricted
deb http://${endpoint}/ubuntu ${code_name} universe
deb http://${endpoint}/ubuntu ${code_name}-updates universe
deb http://${endpoint}/ubuntu ${code_name} multiverse
deb http://${endpoint}/ubuntu ${code_name}-updates multiverse
deb http://${endpoint}/ubuntu ${code_name}-backports main restricted universe multiverse
deb http://${endpoint}/ubuntu ${code_name}-security main restricted
deb http://${endpoint}/ubuntu ${code_name}-security universe
deb http://${endpoint}/ubuntu ${code_name}-security multiverse
EOF
    fi

    cat <<EOF > "${SECURITY_PATCH_CONFIG_DIR}/apt.conf" || return 1
Dir::Etc::sourcelist "${SECURITY_PATCH_CONFIG_DIR}/sources.list";
Dir::Etc::sourceparts "";
EOF
}

security_patch_repo_endpoint() {
    local node_json="$1"
    local repo_service
    local private_ip_regex='^((10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})|(192\.168\.[0-9]{1,3}\.[0-9]{1,3}))$'

    if ! repo_service="$(printf '%s' "${node_json}" | jq -r '.metadata.annotations["kubernetes.azure.com/live-patching-repo-service"] // empty')"; then
        echo "failed to read live patching repo service annotation"
        return 1
    fi

    if [ -z "${repo_service}" ]; then
        echo "${SECURITY_PATCH_DEFAULT_ENDPOINT}"
    elif printf '%s' "${repo_service}" | grep -Eq "${private_ip_regex}"; then
        echo "${repo_service}"
    else
        echo "ignoring invalid live patching repo service: ${repo_service}" >&2
        echo "${SECURITY_PATCH_DEFAULT_ENDPOINT}"
    fi
}

securityPatchIsCurrent() {
    local desired_payload="$1"
    local current_payload="$2"
    local node_json="$3"
    local agent_pool
    local desired_timestamp
    local current_timestamp

    if ! agent_pool="$(printf '%s' "${node_json}" | jq -er '.metadata.labels["kubernetes.azure.com/agentpool"] // empty')"; then
        return 1
    fi
    if ! desired_timestamp="$(printf '%s' "${desired_payload}" | jq -er --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp // empty')"; then
        return 1
    fi
    if ! current_timestamp="$(printf '%s' "${current_payload}" | jq -er --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp // empty')"; then
        return 1
    fi

    [ "${desired_timestamp}" = "${current_timestamp}" ]
}

updateSecurityPatch() {
    local component_payload="${1:-}"
    local node_json="${2:-}"
    local agent_pool
    local golden_timestamp
    local repo_endpoint
    local code_name
    local apt_opts="-o Acquire::http::Timeout=300 -o Acquire::https::Timeout=300 -o Acquire::Retries=3"

    if [ -z "${node_json}" ]; then
        echo "node JSON is required"
        return 1
    fi
    if ! agent_pool="$(printf '%s' "${node_json}" | jq -r '.metadata.labels["kubernetes.azure.com/agentpool"] // empty')"; then
        echo "failed to read node agent pool label"
        return 1
    fi
    if [ -z "${agent_pool}" ]; then
        echo "node agent pool label is not set"
        return 1
    fi

    if ! golden_timestamp="$(printf '%s' "${component_payload}" | jq -er --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp // empty')"; then
        echo "securityPatch profile is missing for agent pool: ${agent_pool}"
        return 1
    fi
    if ! printf '%s' "${golden_timestamp}" | grep -Eq '^[0-9]{8}T[0-9]{6}Z$'; then
        echo "securityPatch goldenTimestamp is invalid: ${golden_timestamp}"
        return 1
    fi

    if ! repo_endpoint="$(security_patch_repo_endpoint "${node_json}")"; then
        return 1
    fi
    if ! code_name="$(lsb_release -cs)" || [ -z "${code_name}" ]; then
        echo "failed to determine Ubuntu codename"
        return 1
    fi
    if ! security_patch_generate_sources_list "${repo_endpoint}" "${golden_timestamp}" "${code_name}"; then
        echo "failed to generate securityPatch apt configuration"
        return 1
    fi

    export APT_CONFIG="${SECURITY_PATCH_CONFIG_DIR}/apt.conf"
    if ! apt_get_update_with_opts "${apt_opts}"; then
        echo "apt_get_update_with_opts failed"
        return 1
    fi
    if ! security_patch_unattended_upgrade; then
        echo "unattended_upgrade failed"
        return 1
    fi

    echo "securityPatch update completed successfully: ${golden_timestamp}"
}

${__SOURCED__:+return}

# shellcheck disable=SC1091
source /opt/azure/containers/provision_source_distro.sh