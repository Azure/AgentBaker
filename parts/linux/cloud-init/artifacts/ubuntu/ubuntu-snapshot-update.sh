#!/usr/bin/env bash

set -o nounset
set -e

# Global constants used in this file. 
# -------------------------------------------------------------------------------------------------
SECURITY_PATCH_CONFIG_DIR=/var/lib/security-patch
KUBECONFIG="/var/lib/kubelet/kubeconfig"
KUBECTL="/usr/local/bin/kubectl --kubeconfig ${KUBECONFIG}"
DEFAULT_ENDPOINT="snapshot.ubuntu.com"

# Function definitions used in this file. 
# functions defined until "${__SOURCED__:+return}" are sourced and tested in -
# spec/parts/linux/cloud-init/artifacts/ubuntu-snapshot-update_spec.sh.
# -------------------------------------------------------------------------------------------------
# Execute unattended-upgrade
unattended_upgrade() {
  retries=10
  for i in $(seq 1 $retries); do
    unattended-upgrade -v && break
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed unattended upgrade $i times
}

generate_sources_list() {
    local endpoint="$1"
    local golden_timestamp="$2"
    local code_name="$3"

    mkdir -p "${SECURITY_PATCH_CONFIG_DIR}"
    if [ "${endpoint}" = "${DEFAULT_ENDPOINT}" ]; then
        cat << EOF > "${SECURITY_PATCH_CONFIG_DIR}/sources.list"
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
        cat << EOF > "${SECURITY_PATCH_CONFIG_DIR}/sources.list"
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

    cat << EOF > "${SECURITY_PATCH_CONFIG_DIR}/apt.conf"
Dir::Etc::sourcelist "${SECURITY_PATCH_CONFIG_DIR}/sources.list";
Dir::Etc::sourceparts "";
EOF

    echo "live patching configuration generated successfully"
}

main() {
    # At startup, we need to wait for kubelet to finish TLS bootstrapping to create the kubeconfig file.
    while [ ! -f ${KUBECONFIG} ]; do
        echo 'Waiting for TLS bootstrapping'
        sleep 3
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

    repo_endpoint="${DEFAULT_ENDPOINT}"
    if [ -z "${live_patching_repo_service}" ]; then
        echo "live patching repo service is not set, use ubuntu snapshot repo"
    else
        echo "live patching repo service is: ${live_patching_repo_service}"
        repo_endpoint="${live_patching_repo_service}"
    fi

    code_name=$(lsb_release -cs)
    generate_sources_list "${repo_endpoint}" "${golden_timestamp}" "${code_name}"

    export APT_CONFIG="${SECURITY_PATCH_CONFIG_DIR}/apt.conf"

    if ! apt_get_update; then
        echo "apt_get_update failed"
        exit 1
    fi
    if ! unattended_upgrade; then
        echo "unattended_upgrade failed"
        exit 1
    fi

    # update current timestamp
    $KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

    echo snapshot update completed successfully
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------

# source apt_get_update
source /opt/azure/containers/provision_source_distro.sh

main "$@"