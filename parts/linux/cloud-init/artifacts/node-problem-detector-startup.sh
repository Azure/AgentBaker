#!/usr/bin/env bash

set -euo pipefail

# Add more VM SKUs by updating the list in config/node-problem-detector/plugin/gpu_vms_capabilities.json
readonly GPU_VMS_CAPABILITIES="/etc/node-problem-detector.d/plugin/gpu_vms_capabilities.json"
readonly GPU_CUSTOM_PLUGINS_PATH="/etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks"

readonly PUBLIC_SETTINGS_PATH="/etc/node-problem-detector.d/public-settings.json"
SYSTEM_LOG_MONITOR_FILES=$(find /etc/node-problem-detector.d/system-log-monitor/ -path '*json' | paste -s -d ',' -)
readonly SYSTEM_LOG_MONITOR_FILES
SYSTEM_STATS_MONITOR_FILES=$(find /etc/node-problem-detector.d/system-stats-monitor/ -path '*json' | paste -s -d ',' -)
readonly SYSTEM_STATS_MONITOR_FILES

CUSTOM_PLUGIN_MONITOR_FILES=$(find /etc/node-problem-detector.d/custom-plugin-monitor/ -type f -name '*.json' ! -path "*/gpu_checks/*" | paste -s -d ',' -)

# check_custom_plugin_monitor_files ensures that the custom plugin monitor files
# are available, if they are not it indicates a major issue with the setup.
function check_custom_plugin_monitor_files() {
    if [ -z "$CUSTOM_PLUGIN_MONITOR_FILES" ]; then
        echo "Error: No custom plugin monitor configurations found."
        exit 1
    fi
}

# Function to check if VM has specific capability
function has_capability() {
    local vm_sku="$1"
    local capability="$2"

    # If the VM SKU is not in our list of supported GPU VMs, return false
    if ! jq -e --arg sku "$vm_sku" 'has($sku)' "${GPU_VMS_CAPABILITIES}" >/dev/null; then
        echo "Error: Unsupported VM SKU '${vm_sku}'"
        return 1 # false
    fi

    case "${capability}" in
    gpu)
        local has_gpu
        has_gpu=$(jq -r --arg vm_sku "${vm_sku}" '
          .[$vm_sku] and .[$vm_sku].GPUs and (.[$vm_sku].GPUs | tonumber) > 0
        ' "${GPU_VMS_CAPABILITIES}")

        [[ "${has_gpu}" == "true" ]] && return 0
        ;;
    nvlink)
        local has_nvlink
        has_nvlink=$(jq -r --arg vm_sku "${vm_sku}" '
          .[$vm_sku] and .[$vm_sku].NVLinkEnabled and (.[$vm_sku].NVLinkEnabled == "True")
        ' "${GPU_VMS_CAPABILITIES}")

        [[ "${has_nvlink}" == "true" ]] && return 0
        ;;
    infiniband)
        local has_infiniband
        has_infiniband=$(jq -r --arg vm_sku "${vm_sku}" '
          .[$vm_sku] and .[$vm_sku].RdmaEnabled and (.[$vm_sku].RdmaEnabled == "True")
        ' "${GPU_VMS_CAPABILITIES}")

        [[ "${has_infiniband}" == "true" ]] && return 0
        ;;
    *)
        echo "Error: Unsupported capability '${capability}'."
        return 1 # false
        ;;
    esac

    echo "VM SKU ${vm_sku} does not have ${capability}"
    return 1 # false
}

function get_kube_apiserver_addr() {
    # This script exists to be able to fetch the kube-apiserver address at runtime.
    # Note: kubeconfig can live in different places depending on whether the node is a master node or a worker node
    if [ -f "/home/azureuser/.kube/config" ]; then
        KUBECONFIG_FILE="/home/azureuser/.kube/config"
    elif [ -f "/var/lib/kubelet/kubeconfig" ]; then
        KUBECONFIG_FILE="/var/lib/kubelet/kubeconfig"
    fi
    KUBE_APISERVER_ADDR=$(grep server "${KUBECONFIG_FILE}" | awk -F"server: " '{print $2}')
}

function get_node_name() {
    NODE_NAME=$(hostname | tr '[:upper:]' '[:lower:]')
}

function get_container_runtime_endpoint() {
    # Determine the container runtime early to avoid querying during each iteration
    # Verify that the container runtime endpoint is set before proceeding to avoid crictl errors
    KUBELET_ARGS=$(systemctl cat kubelet | grep "container-runtime-endpoint")
    if [ -z "$KUBELET_ARGS" ]; then
        echo "Error: container-runtime-endpoint is not set"
        exit 1
    fi
    CONTAINER_RUNTIME_ENDPOINT=$(echo "${KUBELET_ARGS##*--container-runtime-endpoint=}" | cut -f1 -d" " | tr -d "\"")
    export CONTAINER_RUNTIME_ENDPOINT
}

function get_sku() {
    # Azure Instance Metadata Service endpoint for compute information.
    IMDS_ENDPOINT="http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01"
    # Fetch IMDS response
    IMDS_TIMEOUT=10
    IMDS_RESPONSE=$(curl -s --max-time $IMDS_TIMEOUT -H "Metadata: true" "${IMDS_ENDPOINT}")
    if [ -z "$IMDS_RESPONSE" ]; then
        echo "Warning: Failed to retrieve instance metadata; GPU SKU detection will be skipped."
        SKU=""
    else
        SKU=$(echo "${IMDS_RESPONSE}" | jq -r '.vmSize')
    fi
}

function is_toggle_enabled() {
    local setting_name="$1"

    if [ ! -f "$PUBLIC_SETTINGS_PATH" ]; then
        echo "Public settings file not found at ${PUBLIC_SETTINGS_PATH}. Skipping toggle check for ${setting_name}."
        return 1
    fi

    setting_value=$(jq -r ".\"${setting_name}\"" "$PUBLIC_SETTINGS_PATH")
    if [ "$setting_value" = "true" ]; then
        return 0
    fi

    return 1
}

function check_nvidia_drivers() {
    if ! has_capability "$SKU" "gpu"; then
        return 1
    fi

    echo "GPU SKU detected: ${SKU}. Checking for NVIDIA driver availability..."

    if ! (command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1); then
        echo "GPU detected but nvidia-smi is not accessible"
        return 1
    fi

    echo "GPU detected with working NVIDIA drivers."
    return 0
}

function setup_gpu_health_checks() {
    if ! check_nvidia_drivers; then
        echo "Skipping GPU health checks."
        return 0
    fi

    # NOTE: Add more GPU custom plugin monitor files as they are created, these
    # plugins don't need additional checks but just a call to
    # `check_nvidia_drivers`. If a plugin needs additional checks, they should
    # be handled separately in their own functions similar to NVLink checks.
    GPU_CUSTOM_PLUGIN_FILE_LIST=(
        "${GPU_CUSTOM_PLUGINS_PATH}/custom-plugin-gpu-count.json"
        "${GPU_CUSTOM_PLUGINS_PATH}/custom-plugin-xid-error.json"
    )

    local gpu_custom_plugin_files=""

    # Check if the GPU custom plugin monitor files exist
    for plugin_file in "${GPU_CUSTOM_PLUGIN_FILE_LIST[@]}"; do
        if [ ! -f "$plugin_file" ]; then
            echo "Warning: GPU custom plugin monitor configuration file ${plugin_file} not found."
            continue
        fi
        gpu_custom_plugin_files="${gpu_custom_plugin_files}${plugin_file},"
    done

    # Remove the trailing comma if it exists.
    gpu_custom_plugin_files="${gpu_custom_plugin_files%,}"

    if [ -z "$gpu_custom_plugin_files" ]; then
        echo "Warning: No GPU custom plugin monitor configurations found."
        return 0
    fi

    echo "Including GPU health check configurations: ${gpu_custom_plugin_files}"
    CUSTOM_PLUGIN_MONITOR_FILES="${CUSTOM_PLUGIN_MONITOR_FILES},${gpu_custom_plugin_files}"
}

# Function to check if NVLink is supported on this system
function check_nvlink_support() {
    if ! has_capability "$SKU" "nvlink"; then
        return 1 # NVLink is not supported (failure)
    fi

    echo "NVLink SKU detected: ${SKU}. Checking for NVIDIA driver availability..."

    if ! (command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi nvlink --status >/dev/null 2>&1); then
        echo "NVLink detected but nvidia-smi is not accessible"
        return 1
    fi

    echo "NVLink detected with working NVIDIA drivers."
    return 0 # NVLink is supported (success)
}

function setup_nvlink_health_checks() {
    if ! check_nvlink_support; then
        echo "Skipping NVLink health checks."
        return 0
    fi

    echo "NVLink hardware support detected, including NVLink health checks."

    NVLINK_PLUGIN_FILE="${GPU_CUSTOM_PLUGINS_PATH}/custom-plugin-nvlink-status.json"
    # Check if the file exists before adding it
    if [ ! -f "$NVLINK_PLUGIN_FILE" ]; then
        echo "Warning: NVLink custom plugin monitor configuration not found."
        return 0
    fi

    echo "Including NVLink health check configurations: ${NVLINK_PLUGIN_FILE}"
    CUSTOM_PLUGIN_MONITOR_FILES="${CUSTOM_PLUGIN_MONITOR_FILES},${NVLINK_PLUGIN_FILE}"
}

function check_ib_support() {
    if ! has_capability "$SKU" "infiniband"; then
        return 1 # Infiniband is not supported (failure)
    fi

    echo "Infiniband SKU detected: ${SKU}. Checking for Infiniband driver availability..."

    if ! (command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi nvlink --status >/dev/null 2>&1); then
        echo "Infiniband detected but nvidia-smi is not accessible"
        return 1
    fi

    echo "Infiniband detected with working NVIDIA drivers."
    return 0 # Infiniband is supported (success)
}

function setup_ib_health_checks() {
    if ! check_ib_support; then
        echo "Skipping InfiniBand health checks."
        return 0
    fi

    echo "Infiniband hardware support detected, including Infiniband health checks."
    local ib_custom_plugin_file="${GPU_CUSTOM_PLUGINS_PATH}/custom-plugin-ib-link-flapping.json"
    # Check if the file exists before adding it
    if [ ! -f "$ib_custom_plugin_file" ]; then
        echo "Warning: Infiniband custom plugin monitor configuration not found."
        return 0
    fi

    echo "Including Infiniband health check configurations: ${ib_custom_plugin_file}"
    CUSTOM_PLUGIN_MONITOR_FILES="${CUSTOM_PLUGIN_MONITOR_FILES},${ib_custom_plugin_file}"
}

# Function to check public settings for a specific toggle and remove the custom plugin monitor file if it's not set
function check_toggles() {
    local setting_name="$1"
    local plugin_file="$2"
    local full_plugin_path="/etc/node-problem-detector.d/custom-plugin-monitor/${plugin_file}"

    if [ -f "$PUBLIC_SETTINGS_PATH" ]; then
        setting_value=$(jq -r ".\"${setting_name}\"" "$PUBLIC_SETTINGS_PATH")
        if [ "$setting_value" = "true" ]; then
            echo "${setting_name} is enabled. Keeping ${plugin_file}"
        else
            echo "${setting_name} is disabled. Removing ${plugin_file} from custom-plugin-monitor/"
            # Remove the plugin file from the CUSTOM_PLUGIN_MONITOR_FILES variable
            # sed commands at the end cleanup any extra commas that might be left after removing the file from the comma-delimited list
            # (,, in the middle of the list, ^, at the beginning, ,$ at the end)
            CUSTOM_PLUGIN_MONITOR_FILES=$(echo "$CUSTOM_PLUGIN_MONITOR_FILES" | sed "s|${full_plugin_path}||g" | sed 's/,,/,/g' | sed 's/^,//' | sed 's/,$//')
        fi
    fi
}

function start_npd() {
    get_kube_apiserver_addr
    get_node_name

    # You can review the preconfigured systemd defaults here at:
    #   https://github.com/kubernetes/node-problem-detector/blob/master/config/systemd/node-problem-detector-metric-only.service
    #
    #   Here is a list of the configurable runtime flags for node-problem-detector
    #
    # root@ubuntu-bionic:~# /usr/local/bin/node-problem-detector -h
    # Usage of /usr/local/bin/node-problem-detector:
    #      --address string                           The address to bind the node problem detector server. (default "127.0.0.1")
    #      --alsologtostderr                          log to standard error as well as files
    #      --apiserver-override string                Custom URI used to connect to Kubernetes ApiServer. This is ignored if --enable-k8s-exporter is false.
    #      --apiserver-wait-interval duration         The interval between the checks on the readiness of kube-apiserver. This is ignored if --enable-k8s-exporter is false. (default 5s)
    #      --apiserver-wait-timeout duration          The timeout on waiting for kube-apiserver to be ready. This is ignored if --enable-k8s-exporter is false. (default 5m0s)
    #      --config.custom-plugin-monitor strings     Comma separated configurations for custom-plugin-monitor monitor. Set to config file paths.
    #      --config.system-log-monitor strings        Comma separated configurations for system-log-monitor monitor. Set to config file paths.
    #      --config.system-stats-monitor strings      Comma separated configurations for system-stats-monitor monitor. Set to config file paths.
    #      --enable-k8s-exporter                      Enables reporting to Kubernetes API server. (default true)
    #      --exporter.stackdriver string              Configuration for Stackdriver exporter. Set to config file path.
    #      --hostname-override string                 Custom node name used to override hostname
    #      --k8s-exporter-heartbeat-period duration   The period at which k8s-exporter does forcibly sync with apiserver. (default 5m0s)
    #      --log_backtrace_at traceLocation           when logging hits line file:N, emit a stack trace (default :0)
    #      --log_dir string                           If non-empty, write log files in this directory
    #      --logtostderr                              log to standard error instead of files
    #      --port int                                 The port to bind the node problem detector server. Use 0 to disable. (default 20256)
    #      --prometheus-address string                The address to bind the Prometheus scrape endpoint. (default "127.0.0.1")
    #      --prometheus-port int                      The port to bind the Prometheus scrape endpoint. Prometheus exporter is enabled by default at port 20257. Use 0 to disable. (default 20257)
    #      --stderrthreshold severity                 logs at or above this threshold go to stderr (default 2)
    #  -v, --v Level                                  log level for V logs
    #      --version                                  Print version information and quit
    #      --vmodule moduleSpec                       comma-separated list of pattern=N settings for file-filtered logging
    #  pflag: help requested
    # In order to enable reporting to kubernetes, set KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT
    #   or use the --apiserver-override flag
    exec node-problem-detector \
        --config.system-log-monitor="${SYSTEM_LOG_MONITOR_FILES}" \
        --config.custom-plugin-monitor="${CUSTOM_PLUGIN_MONITOR_FILES}" \
        --config.system-stats-monitor="${SYSTEM_STATS_MONITOR_FILES}" \
        --prometheus-address 0.0.0.0 \
        --apiserver-override "${KUBE_APISERVER_ADDR}?inClusterConfig=false&auth=${KUBECONFIG_FILE}" \
        --hostname-override "${NODE_NAME}" \
        --logtostderr

}

check_custom_plugin_monitor_files
get_container_runtime_endpoint
get_sku

setup_gpu_health_checks
setup_nvlink_health_checks
setup_ib_health_checks

# Use existing toggle for new dns issue monitor
check_toggles "npd-validate-in-prod" "custom-dns-issue-monitor.json"
# Use existing toggle for new rx out of buffer errors monitor
check_toggles "npd-validate-in-prod" "custom-rx-buffer-errors-monitor.json"

start_npd
