#!/bin/bash
# shellcheck disable=SC2329 # helper functions are invoked by ShellSpec hooks and examples.

Describe 'gpu-uuid-node-labels.sh'
    SCRIPT_PATH="./parts/linux/cloud-init/artifacts/gpu-uuid-node-labels.sh"
    UUID_A="GPU-117510f6-910b-4afe-a0bd-38fccc8e94a7"
    UUID_B="GPU-ffde1b53-f964-8b8e-4f88-04e7a2f09822"
    UUID_C="GPU-aaaaaaaa-1111-2222-3333-444444444444"
    UUID_D="GPU-bbbbbbbb-1111-2222-3333-444444444444"
    LABEL_PREFIX="kubernetes.azure.com/gpu"
    NODE_NAME="aks-gpu-12345678-vmss000000"

    # nvidia-smi output lines, one argument per GPU.
    gpus() {
        printf '%s\n' "$@" > "${FAKE_DIR}/smi_output"
    }

    # Current GPU UUID labels on the Node, as "key=value" lines.
    node_labels() {
        printf '%s\n' "$@" > "${FAKE_DIR}/node_labels"
    }

    kubectl_calls() {
        cat "${FAKE_DIR}/kubectl.log" 2>/dev/null
    }

    nvidia_smi_calls() {
        cat "${FAKE_DIR}/nvidia-smi.log" 2>/dev/null
    }

    kubelet_node_labels() {
        sed -n 's/^KUBELET_NODE_LABELS=//p' "${KUBELET_DEFAULT_FILE}"
    }

    kubelet_defaults_mode() {
        stat -c %a "${KUBELET_DEFAULT_FILE}"
    }

    setup() {
        FAKE_DIR="$(mktemp -d)"
        mkdir -p "${FAKE_DIR}/bin" "${FAKE_DIR}/gpu"

        cat > "${FAKE_DIR}/bin/nvidia-smi" <<'EOF'
#!/bin/sh
echo "$*" >> "${FAKE_DIR}/nvidia-smi.log"
failures=$(cat "${FAKE_DIR}/smi_failures" 2>/dev/null || echo 0)
if [ "${failures}" -gt 0 ]; then
    echo $((failures - 1)) > "${FAKE_DIR}/smi_failures"
    exit 6
fi
cat "${FAKE_DIR}/smi_output"
exit "$(cat "${FAKE_DIR}/smi_status" 2>/dev/null || echo 0)"
EOF
        cat > "${FAKE_DIR}/kubectl" <<'EOF'
#!/bin/sh
echo "$*" >> "${FAKE_DIR}/kubectl.log"
case "$*" in
    *" get node "*)
        [ -f "${FAKE_DIR}/node_missing" ] && exit 1
        cat "${FAKE_DIR}/node_labels"
        echo "kubernetes.azure.com/agentpool=gpu"
        ;;
    *" label node "*)
        exit "$(cat "${FAKE_DIR}/label_status" 2>/dev/null || echo 0)"
        ;;
esac
EOF
        cat > "${FAKE_DIR}/bin/hostname" <<'EOF'
#!/bin/sh
echo "AKS-GPU-12345678-VMSS000000"
EOF
        chmod +x "${FAKE_DIR}/bin/nvidia-smi" "${FAKE_DIR}/kubectl" "${FAKE_DIR}/bin/hostname"
        touch "${FAKE_DIR}/version" "${FAKE_DIR}/kubeconfig"
        cat > "${FAKE_DIR}/kubelet" <<'EOF'
KUBELET_FLAGS=--node-ip=10.224.0.4
KUBELET_NODE_LABELS=kubernetes.azure.com/agentpool=gpu,kubernetes.azure.com/gpu-0-uuid=GPU-117510f6-910b-4afe-a0bd-38fccc8e94a7,kubernetes.azure.com/gpu-1-uuid=GPU-ffde1b53-f964-8b8e-4f88-04e7a2f09822,kubernetes.azure.com/mode=user
KUBELET_REGISTER_SCHEDULABLE=true
EOF
        chmod 0600 "${FAKE_DIR}/kubelet"
        node_labels "${LABEL_PREFIX}-0-uuid=${UUID_A}" "${LABEL_PREFIX}-1-uuid=${UUID_B}"

        export FAKE_DIR
        export PATH="${FAKE_DIR}/bin:${PATH}"
        export GPU_DEST="${FAKE_DIR}/gpu"
        export NVIDIA_DRIVER_VERSION_FILE="${FAKE_DIR}/version"
        export KUBECTL_PATH="${FAKE_DIR}/kubectl"
        export KUBECONFIG="${FAKE_DIR}/kubeconfig"
        export KUBELET_DEFAULT_FILE="${FAKE_DIR}/kubelet"
        export GPU_UUID_DEADLINE_SECONDS=0
        export GPU_UUID_WAIT_SECONDS=0
    }

    cleanup() {
        rm -rf "${FAKE_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'labels'
        It 'prints one label per physical GPU'
            gpus "0, ${UUID_A}, Pass-Through" "1, ${UUID_B}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal "${LABEL_PREFIX}-0-uuid=${UUID_A}
${LABEL_PREFIX}-1-uuid=${UUID_B}"
            The result of function nvidia_smi_calls should equal "--query-gpu=index,uuid,virtualization_mode --format=csv,noheader"
        End

        It 'labels GPUs whose virtualization mode is not reported'
            gpus "0, ${UUID_A}, None" "1, ${UUID_B}, N/A"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal "${LABEL_PREFIX}-0-uuid=${UUID_A}
${LABEL_PREFIX}-1-uuid=${UUID_B}"
        End

        It 'skips vGPUs because their UUIDs do not identify physical GPUs'
            gpus "0, ${UUID_A}, VGPU" "1, ${UUID_B}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal "${LABEL_PREFIX}-1-uuid=${UUID_B}"
            The stderr should include "skipping vGPU 0"
        End

        It 'skips host vGPUs'
            gpus "0, ${UUID_A}, Host VGPU" "1, ${UUID_B}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal "${LABEL_PREFIX}-1-uuid=${UUID_B}"
            The stderr should include "skipping vGPU 0"
        End

        It 'prints nothing for a vGPU-only VM'
            gpus "0, ${UUID_A}, VGPU"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal ""
            The stderr should include "skipping vGPU 0"
        End

        It 'uses nvidia-smi from GPU_DEST when it is not on PATH'
            mv "${FAKE_DIR}/bin/nvidia-smi" "${FAKE_DIR}/gpu/nvidia-smi.tmp"
            mkdir -p "${FAKE_DIR}/gpu/bin"
            mv "${FAKE_DIR}/gpu/nvidia-smi.tmp" "${FAKE_DIR}/gpu/bin/nvidia-smi"
            gpus "0, ${UUID_A}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should be success
            The output should equal "${LABEL_PREFIX}-0-uuid=${UUID_A}"
        End

        It 'exits 2 when nvidia-smi is not on the host'
            rm "${FAKE_DIR}/bin/nvidia-smi"

            When run bash "$SCRIPT_PATH" labels

            The status should equal 2
            The output should equal ""
            The stderr should include "nvidia-smi not found"
        End

        It 'does not run nvidia-smi when the driver is not loaded'
            rm "${FAKE_DIR}/version"
            gpus "0, ${UUID_A}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should equal 1
            The output should equal ""
            The stderr should include "NVIDIA driver is not loaded"
            The path "${FAKE_DIR}/nvidia-smi.log" should not be exist
        End

        It 'reports the inventory as incomplete when nvidia-smi fails'
            gpus "0, ${UUID_A}, Pass-Through"
            echo 9 > "${FAKE_DIR}/smi_status"

            When run bash "$SCRIPT_PATH" labels

            The status should equal 1
            The output should equal "${LABEL_PREFIX}-0-uuid=${UUID_A}"
            The stderr should include "nvidia-smi GPU UUID query exited with 9"
        End

        It 'kills an nvidia-smi that hangs and ignores SIGTERM'
            printf '#!/bin/sh\ntrap "" TERM\nwhile :; do sleep 1; done\n' > "${FAKE_DIR}/bin/nvidia-smi"

            When run env GPU_UUID_QUERY_TIMEOUT_SECONDS=1 bash "$SCRIPT_PATH" labels

            The status should equal 1
            The output should equal ""
            The stderr should include "nvidia-smi GPU UUID query exited with 137"
        End

        Parameters
            "a GPU nvidia-smi cannot open" "Unable to determine the device handle for GPU0000:00:06.0: Unknown Error"
            "an error that mentions vGPU" "Unable to determine vGPU state for GPU0000:00:06.0, VGPU"
            "an unreadable UUID" "0, [GPU requires reset], Pass-Through"
            "a UUID without the GPU- prefix" "0, MIG-117510f6-910b-4afe-a0bd-38fccc8e94a7, Pass-Through"
            "a UUID with unexpected characters" "0, GPU-117510f6-910b-4afe-a0bd-38fccc8e94z7, Pass-Through"
            "a truncated UUID" "0, GPU-117510f6-910b-4afe-a0bd-38fccc8e94a, Pass-Through"
            "a non-numeric index" "[Unknown Error], ${UUID_A}, Pass-Through"
        End

        Example "reports the inventory as incomplete when one line has $1"
            gpus "$2" "1, ${UUID_B}, Pass-Through"

            When run bash "$SCRIPT_PATH" labels

            The status should equal 1
            The output should equal "${LABEL_PREFIX}-1-uuid=${UUID_B}"
            The stderr should include "unexpected nvidia-smi GPU line"
        End
    End

    Describe 'reconcile'
        It 'leaves up-to-date labels unchanged'
            gpus "0, ${UUID_A}, Pass-Through" "1, ${UUID_B}, Pass-Through"
            cp "${KUBELET_DEFAULT_FILE}" "${FAKE_DIR}/kubelet.orig"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "GPU UUID labels are up to date"
            The result of function kubectl_calls should not include "label node"
            The contents of file "${KUBELET_DEFAULT_FILE}" should equal "$(cat "${FAKE_DIR}/kubelet.orig")"
        End

        It 'replaces the labels and kubelet defaults after the VM moves to different GPUs'
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The line 1 of output should equal "updated GPU UUID labels in ${KUBELET_DEFAULT_FILE}"
            The line 2 of output should equal "reconciled GPU UUID labels on node ${NODE_NAME}"
            The result of function kubectl_calls should include "get node ${NODE_NAME} "
            The result of function kubectl_calls should include "label node ${NODE_NAME} --overwrite ${LABEL_PREFIX}-0-uuid=${UUID_C} ${LABEL_PREFIX}-1-uuid=${UUID_D}"
            The result of function kubelet_node_labels should equal "kubernetes.azure.com/agentpool=gpu,kubernetes.azure.com/mode=user,${LABEL_PREFIX}-0-uuid=${UUID_C},${LABEL_PREFIX}-1-uuid=${UUID_D}"
            The line 1 of contents of file "${KUBELET_DEFAULT_FILE}" should equal "KUBELET_FLAGS=--node-ip=10.224.0.4"
            The line 3 of contents of file "${KUBELET_DEFAULT_FILE}" should equal "KUBELET_REGISTER_SCHEDULABLE=true"
            The result of function kubelet_defaults_mode should equal "600"
        End

        It 'removes labels for GPU indexes that no longer exist on a different host'
            node_labels "${LABEL_PREFIX}-0-uuid=${UUID_A}" "${LABEL_PREFIX}-1-uuid=${UUID_B}" "${LABEL_PREFIX}-2-uuid=GPU-cccccccc-1111-2222-3333-444444444444"
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The result of function kubectl_calls should include "--overwrite ${LABEL_PREFIX}-0-uuid=${UUID_C} ${LABEL_PREFIX}-1-uuid=${UUID_D} ${LABEL_PREFIX}-2-uuid-"
        End

        It 'relabels GPUs whose indexes changed'
            gpus "0, ${UUID_B}, Pass-Through" "1, ${UUID_A}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The result of function kubectl_calls should include "--overwrite ${LABEL_PREFIX}-0-uuid=${UUID_B} ${LABEL_PREFIX}-1-uuid=${UUID_A}"
        End

        It 'adds labels to a Node that has none'
            node_labels ""
            gpus "0, ${UUID_A}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The result of function kubectl_calls should include "--overwrite ${LABEL_PREFIX}-0-uuid=${UUID_A}"
        End

        It 'keeps the labels when a labeled GPU is missing on the same host'
            gpus "0, ${UUID_B}, Pass-Through"
            cp "${KUBELET_DEFAULT_FILE}" "${FAKE_DIR}/kubelet.orig"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "keeping GPU UUID labels so the missing GPU stays attributed"
            The result of function kubectl_calls should not include "label node"
            The contents of file "${KUBELET_DEFAULT_FILE}" should equal "$(cat "${FAKE_DIR}/kubelet.orig")"
        End

        It 'removes the labels when the VM only has vGPUs'
            gpus "0, ${UUID_C}, VGPU"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The stderr should include "skipping vGPU 0"
            The result of function kubectl_calls should include "--overwrite ${LABEL_PREFIX}-0-uuid- ${LABEL_PREFIX}-1-uuid-"
            The result of function kubelet_node_labels should equal "kubernetes.azure.com/agentpool=gpu,kubernetes.azure.com/mode=user"
        End

        It 'waits for the GPU inventory to become readable'
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"
            echo 2 > "${FAKE_DIR}/smi_failures"

            When run env GPU_UUID_DEADLINE_SECONDS=30 bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The stderr should include "nvidia-smi GPU UUID query exited with 6"
        End

        It 'leaves the labels unchanged when the GPU inventory never becomes readable'
            gpus "0, ${UUID_C}, Pass-Through"
            rm "${FAKE_DIR}/version"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "could not read the GPU inventory, leaving GPU UUID labels unchanged"
            The stderr should include "NVIDIA driver is not loaded"
            The path "${FAKE_DIR}/kubectl.log" should not be exist
        End

        It 'stops at once when nvidia-smi is not on the host'
            rm "${FAKE_DIR}/bin/nvidia-smi"

            When run env GPU_UUID_DEADLINE_SECONDS=120 GPU_UUID_WAIT_SECONDS=60 bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "nvidia-smi is not on the host, nothing to reconcile"
            The stderr should include "nvidia-smi not found"
            The path "${FAKE_DIR}/kubectl.log" should not be exist
        End

        It 'removes the previous host labels when a GPU on the new host cannot be read'
            gpus "0, ${UUID_C}, Pass-Through" "Unable to determine the device handle for GPU0000:00:06.0: Unknown Error"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The line 1 of output should equal "could not read every GPU on a different host, removing GPU UUID labels"
            The output should include "reconciled GPU UUID labels on node ${NODE_NAME}"
            The stderr should include "unexpected nvidia-smi GPU line"
            The result of function kubectl_calls should include "--overwrite ${LABEL_PREFIX}-0-uuid- ${LABEL_PREFIX}-1-uuid-"
            The result of function kubelet_node_labels should equal "kubernetes.azure.com/agentpool=gpu,kubernetes.azure.com/mode=user"
        End

        It 'keeps the labels when a GPU cannot be read on the same host'
            gpus "0, ${UUID_A}, Pass-Through" "Unable to determine the device handle for GPU0000:00:06.0: Unknown Error"
            cp "${KUBELET_DEFAULT_FILE}" "${FAKE_DIR}/kubelet.orig"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "could not read every GPU; the readable GPUs are already labeled, keeping GPU UUID labels"
            The stderr should include "unexpected nvidia-smi GPU line"
            The result of function kubectl_calls should not include "label node"
            The contents of file "${KUBELET_DEFAULT_FILE}" should equal "$(cat "${FAKE_DIR}/kubelet.orig")"
        End

        It 'does not label only the readable GPUs of a node without labels'
            node_labels ""
            printf 'KUBELET_NODE_LABELS=kubernetes.azure.com/agentpool=gpu\n' > "${KUBELET_DEFAULT_FILE}"
            gpus "0, ${UUID_C}, Pass-Through" "Unable to determine the device handle for GPU0000:00:06.0: Unknown Error"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The line 1 of output should equal "could not read every GPU, leaving the node without GPU UUID labels"
            The stderr should include "unexpected nvidia-smi GPU line"
            The result of function kubectl_calls should not include "label node"
            The result of function kubelet_node_labels should equal "kubernetes.azure.com/agentpool=gpu"
        End

        It 'leaves the labels unchanged when kubectl is not available'
            gpus "0, ${UUID_C}, Pass-Through"
            rm "${FAKE_DIR}/kubectl"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "kubectl or ${KUBECONFIG} not available, leaving GPU UUID labels unchanged"
        End

        It 'leaves the labels unchanged when the kubelet kubeconfig is not available'
            gpus "0, ${UUID_C}, Pass-Through"
            rm "${FAKE_DIR}/kubeconfig"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "kubectl or ${KUBECONFIG} not available, leaving GPU UUID labels unchanged"
            The path "${FAKE_DIR}/kubectl.log" should not be exist
        End

        It 'leaves the labels unchanged when the Node is not found'
            gpus "0, ${UUID_C}, Pass-Through"
            touch "${FAKE_DIR}/node_missing"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "node ${NODE_NAME} not found, leaving GPU UUID labels unchanged"
            The result of function kubectl_calls should not include "label node"
        End

        It 'does not fail when kubectl cannot label the Node'
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"
            echo 1 > "${FAKE_DIR}/label_status"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should include "warning: failed to update GPU UUID labels on node ${NODE_NAME}"
        End

        It 'does not create KUBELET_NODE_LABELS when the kubelet defaults have none'
            printf 'KUBELET_FLAGS=--node-ip=10.224.0.4\n' > "${KUBELET_DEFAULT_FILE}"
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "reconciled GPU UUID labels on node ${NODE_NAME}"
            The contents of file "${KUBELET_DEFAULT_FILE}" should equal "KUBELET_FLAGS=--node-ip=10.224.0.4"
        End

        It 'still labels the Node when the kubelet defaults file is missing'
            rm "${KUBELET_DEFAULT_FILE}"
            gpus "0, ${UUID_C}, Pass-Through" "1, ${UUID_D}, Pass-Through"

            When run bash "$SCRIPT_PATH" reconcile

            The status should be success
            The output should equal "reconciled GPU UUID labels on node ${NODE_NAME}"
            The path "${KUBELET_DEFAULT_FILE}" should not be exist
        End
    End

    It 'does nothing without a command'
        When run bash "$SCRIPT_PATH"

        The status should be success
        The output should equal ""
    End
End
