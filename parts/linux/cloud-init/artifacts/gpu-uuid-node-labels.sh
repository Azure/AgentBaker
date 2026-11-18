#!/bin/bash
# Keeps the kubernetes.azure.com/gpu-<index>-uuid node labels in sync with the physical GPUs on this VM.
# The labels tie node-level GPU health signals (NPD conditions) to physical GPUs, so failures can be
# attributed to a GPU and tracked across the VMs it is used by.
#
#   gpu-uuid-node-labels.sh labels     Print one "<label>=<GPU UUID>" line per physical GPU and exit 0.
#                                      Exits 2 without output when nvidia-smi isn't on the host. Exits 1 when
#                                      the inventory is incomplete, printing only the GPUs it could read.
#   gpu-uuid-node-labels.sh reconcile  Run on every boot by gpu-uuid-node-labels.service. kubelet only applies
#                                      --node-labels when it creates the Node object, so after the VM moves to
#                                      different GPUs (service healing, deallocate and start) this updates the
#                                      Node's labels and /etc/default/kubelet.
#
# vGPU UUIDs are skipped: NVIDIA assigns them per VM, so they don't identify physical hardware.
# Best effort: never fails provisioning or boot.

GPU_UUID_LABEL_PATTERN='^kubernetes\.azure\.com/gpu-[0-9]+-uuid='

# Prints the UUIDs of the vGPUs in "nvidia-smi -q" output. Each GPU section lists "GPU UUID" before its
# "Virtualization Mode", which is VGPU (or Host VGPU) on vGPU guests and None or Pass-Through otherwise.
vgpu_uuids() {
    awk '
        /^[[:space:]]*GPU UUID[[:space:]]*:/ {
            uuid = $0; sub(/^[^:]*:[[:space:]]*/, "", uuid); sub(/[[:space:]]+$/, "", uuid)
        }
        /^[[:space:]]*Virtualization Mode[[:space:]]*:/ {
            mode = $0; sub(/^[^:]*:[[:space:]]*/, "", mode); sub(/[[:space:]]+$/, "", mode)
            if (mode == "VGPU" || mode == "Host VGPU") print uuid
        }'
}

# Prints the label lines for the GPUs on this VM. Returns 2 without output when nvidia-smi isn't on the host
# (not an NVIDIA node, or drivers that live in a container). Returns 1 when the driver isn't loaded yet,
# nvidia-smi times out, or nvidia-smi fails or reports a GPU it can't identify; in the last two cases the
# GPUs it could read are still printed.
gpu_uuid_labels() {
    local nvidia_smi="nvidia-smi"
    local driver_version_file="${NVIDIA_DRIVER_VERSION_FILE:-/proc/driver/nvidia/version}"
    local query_timeout="${GPU_UUID_QUERY_TIMEOUT_SECONDS:-30}"
    local gpu_list=""
    local gpu_details=""
    local vgpus=""
    local smi_status=0
    local incomplete=0
    local labels=""
    local index=""
    local uuid=""
    local extra=""
    local uuid_hex=""

    if ! command -v "${nvidia_smi}" >/dev/null 2>&1; then
        nvidia_smi="${GPU_DEST:-/usr/local/nvidia}/bin/nvidia-smi"
        if [ ! -x "${nvidia_smi}" ]; then
            echo "nvidia-smi not found" >&2
            return 2
        fi
    fi
    # Only query a loaded driver: nvidia-smi can otherwise load kernel modules, which must not happen on
    # nodes whose drivers are managed outside AKS.
    if [ ! -e "${driver_version_file}" ]; then
        echo "NVIDIA driver is not loaded" >&2
        return 1
    fi

    # -k sends SIGKILL if nvidia-smi ignores SIGTERM, so a wedged GPU cannot stall provisioning or boot.
    gpu_list=$(timeout -k 5 "${query_timeout}" "${nvidia_smi}" --query-gpu=index,uuid --format=csv,noheader 2>/dev/null) || smi_status=$?
    case "${smi_status}" in
        0) ;;
        124|137)
            echo "nvidia-smi GPU UUID query timed out" >&2
            return 1
            ;;
        *)
            echo "nvidia-smi GPU UUID query exited with ${smi_status}" >&2
            incomplete=1
            ;;
    esac

    # --query-gpu has no virtualization mode field, so read it from the full report.
    smi_status=0
    gpu_details=$(timeout -k 5 "${query_timeout}" "${nvidia_smi}" -q 2>/dev/null) || smi_status=$?
    case "${smi_status}" in
        0) ;;
        124|137)
            echo "nvidia-smi GPU report timed out" >&2
            return 1
            ;;
        *)
            echo "nvidia-smi GPU report exited with ${smi_status}" >&2
            incomplete=1
            ;;
    esac
    vgpus=$(printf '%s\n' "${gpu_details}" | vgpu_uuids)

    # Expect "<index>, GPU-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx". A 40 character UUID also keeps the value within
    # the label value character set and length limit. Any other line, such as nvidia-smi's "Unable to determine
    # the device handle for GPU..." for a GPU in a bad state, makes the inventory incomplete.
    while IFS=', ' read -r index uuid extra; do
        [ -z "${index}${uuid}${extra}" ] && continue
        uuid_hex="${uuid#GPU-}"
        case "${index}" in
            ''|*[!0-9]*)
                echo "unexpected nvidia-smi GPU line: ${index} ${uuid} ${extra}" >&2
                incomplete=1
                continue
                ;;
        esac
        case "${uuid_hex}" in
            "${uuid}"|''|*[!0-9a-fA-F-]*)
                echo "unexpected nvidia-smi GPU line: ${index} ${uuid} ${extra}" >&2
                incomplete=1
                continue
                ;;
        esac
        if [ "${#uuid}" -ne 40 ] || [ -n "${extra}" ]; then
            echo "unexpected nvidia-smi GPU line: ${index} ${uuid} ${extra}" >&2
            incomplete=1
            continue
        fi
        if printf '%s\n' "${vgpus}" | grep -qxF "${uuid}"; then
            echo "skipping vGPU ${index}: vGPU UUIDs don't identify physical GPUs" >&2
            continue
        fi
        labels="${labels}kubernetes.azure.com/gpu-${index}-uuid=${uuid}
"
    done <<EOF
${gpu_list}
EOF

    printf '%s' "${labels}"
    [ "${incomplete}" -eq 0 ] || return 1
    return 0
}

# Returns 0 when every GPU UUID in the first set of label lines is also a label value in the second set.
all_uuids_labeled() {
    local label

    while read -r label; do
        [ -n "${label}" ] || continue
        printf '%s\n' "$2" | grep -qF "=${label#*=}" || return 1
    done <<EOF
$1
EOF
    return 0
}

# Prints "yes" when the GPUs now present are a strict subset of the labeled ones: a GPU dropped off the
# bus on the same host. The labels are then kept, so the missing GPU's UUID stays attributed to the node.
gpus_missing_from_labels() {
    local desired="$1"
    local current="$2"
    local desired_count
    local current_count

    [ -n "${desired}" ] || return 0
    desired_count=$(printf '%s\n' "${desired}" | sed '/^$/d' | wc -l)
    current_count=$(printf '%s\n' "${current}" | sed '/^$/d' | wc -l)
    [ "${desired_count}" -lt "${current_count}" ] || return 0
    all_uuids_labeled "${desired}" "${current}" && echo "yes"
    return 0
}

# Rewrites the GPU UUID entries of KUBELET_NODE_LABELS so a later Node registration uses the current GPUs.
update_kubelet_defaults() {
    local desired="$1"
    local defaults_file="$2"
    local current_value
    local current_gpu_labels
    local new_value
    local tmp_file

    [ -f "${defaults_file}" ] || return 0
    grep -q '^KUBELET_NODE_LABELS=' "${defaults_file}" || return 0
    current_value=$(sed -n 's/^KUBELET_NODE_LABELS=//p' "${defaults_file}" | head -n 1)
    current_gpu_labels=$(printf '%s\n' "${current_value}" | tr ',' '\n' | grep -E "${GPU_UUID_LABEL_PATTERN}" | sort)
    [ "${current_gpu_labels}" != "${desired}" ] || return 0
    new_value=$( { printf '%s\n' "${current_value}" | tr ',' '\n' | grep -vE "${GPU_UUID_LABEL_PATTERN}"; printf '%s\n' "${desired}"; } | sed '/^$/d' | paste -s -d , -)

    tmp_file=$(mktemp "${defaults_file}.XXXXXX") || return 0
    if awk -v value="${new_value}" '/^KUBELET_NODE_LABELS=/ && !done { print "KUBELET_NODE_LABELS=" value; done = 1; next } { print }' "${defaults_file}" > "${tmp_file}" \
        && chmod 0600 "${tmp_file}" && mv -f "${tmp_file}" "${defaults_file}"; then
        echo "updated GPU UUID labels in ${defaults_file}"
    else
        rm -f "${tmp_file}"
        echo "warning: could not update ${defaults_file}"
    fi
}

past_deadline() {
    [ "$(date +%s)" -ge "$1" ]
}

reconcile_gpu_uuid_labels() {
    local kubectl="${KUBECTL_PATH:-/opt/bin/kubectl}"
    local kubeconfig="${KUBECONFIG:-/var/lib/kubelet/kubeconfig}"
    local defaults_file="${KUBELET_DEFAULT_FILE:-/etc/default/kubelet}"
    local wait_seconds="${GPU_UUID_WAIT_SECONDS:-6}"
    local deadline
    local status=0
    local desired=""
    local current=""
    local node_name
    local key

    # The driver and kubelet credentials can take a while after boot. All waits share one deadline, so the
    # script ends well within the unit's RuntimeMaxSec even when nvidia-smi or the API server hangs.
    deadline=$(( $(date +%s) + ${GPU_UUID_DEADLINE_SECONDS:-900} ))
    while :; do
        status=0
        desired=$(gpu_uuid_labels) || status=$?
        [ "${status}" -eq 0 ] && break
        if [ "${status}" -eq 2 ]; then
            echo "nvidia-smi is not on the host, nothing to reconcile"
            return 0
        fi
        past_deadline "${deadline}" && break
        sleep "${wait_seconds}"
    done
    desired=$(printf '%s\n' "${desired}" | sed '/^$/d' | sort)
    if [ "${status}" -ne 0 ] && [ -z "${desired}" ]; then
        echo "could not read the GPU inventory, leaving GPU UUID labels unchanged"
        return 0
    fi

    until [ -x "${kubectl}" ] && [ -f "${kubeconfig}" ]; do
        if past_deadline "${deadline}"; then
            echo "kubectl or ${kubeconfig} not available, leaving GPU UUID labels unchanged"
            return 0
        fi
        sleep "${wait_seconds}"
    done

    # Azure cloud provider names the node after the lower-cased hostname.
    node_name=$(hostname | tr '[:upper:]' '[:lower:]')
    # shellcheck disable=SC2016 # $k and $v are go-template variables, not shell expansions.
    until current=$("${kubectl}" --kubeconfig "${kubeconfig}" --request-timeout=30s get node "${node_name}" \
        -o go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{$v}}{{"\n"}}{{end}}' 2>/dev/null); do
        if past_deadline "${deadline}"; then
            echo "node ${node_name} not found, leaving GPU UUID labels unchanged"
            return 0
        fi
        sleep "${wait_seconds}"
    done
    current=$(printf '%s\n' "${current}" | grep -E "${GPU_UUID_LABEL_PATTERN}" | sort)

    if [ "${status}" -ne 0 ]; then
        # Some GPUs could not be read. If the readable GPUs are already labeled this is the same host, so keep
        # the labels. Otherwise the VM moved: drop the previous host's UUIDs rather than attribute this host's
        # GPU faults to them. The readable GPUs are not labeled alone, which could blame them for the unreadable one.
        if all_uuids_labeled "${desired}" "${current}"; then
            echo "could not read every GPU; the readable GPUs are already labeled, keeping GPU UUID labels"
            return 0
        fi
        if [ -n "${current}" ]; then
            echo "could not read every GPU on a different host, removing GPU UUID labels"
        else
            echo "could not read every GPU, leaving the node without GPU UUID labels"
        fi
        desired=""
    elif [ "$(gpus_missing_from_labels "${desired}" "${current}")" = "yes" ]; then
        echo "fewer GPUs than labeled on the same host, keeping GPU UUID labels so the missing GPU stays attributed"
        return 0
    fi

    update_kubelet_defaults "${desired}" "${defaults_file}"

    if [ "${desired}" = "${current}" ]; then
        echo "GPU UUID labels are up to date"
        return 0
    fi

    set --
    while read -r key; do
        [ -n "${key}" ] && set -- "$@" "${key}"
    done <<EOF
${desired}
EOF
    while IFS='=' read -r key _; do
        [ -n "${key}" ] || continue
        printf '%s\n' "${desired}" | grep -q "^${key}=" || set -- "$@" "${key}-"
    done <<EOF
${current}
EOF
    if "${kubectl}" --kubeconfig "${kubeconfig}" --request-timeout=30s label node "${node_name}" --overwrite "$@"; then
        echo "reconciled GPU UUID labels on node ${node_name}"
    else
        echo "warning: failed to update GPU UUID labels on node ${node_name}"
    fi
    return 0
}

case "${1:-}" in
    labels)
        gpu_uuid_labels
        ;;
    reconcile)
        reconcile_gpu_uuid_labels
        ;;
esac
