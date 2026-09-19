#!/bin/bash

# Validate the dedicated AMD image on each real node, including PIS nodes. There
# is deliberately no apt/DKMS installation fallback during node provisioning.
ensureAmdGpuDrivers() {
    local vm_sku
    vm_sku=$(get_compute_sku) || return $ERR_AMD_GPU_VALIDATE_FAIL
    if [ "${OS}" != "${UBUNTU_OS_NAME}" ] || [ "${OS_VERSION}" != "24.04" ] || [ "$(uname -m)" != "x86_64" ] || [ "${GPU_NODE:-false}" = "true" ]; then
        echo "AMD GPU requires the dedicated Ubuntu 24.04 amd64 image and exclusive AMD configuration"
        return $ERR_AMD_GPU_UNSUPPORTED
    fi
    case "${vm_sku,,}" in
        standard_nd96isr_mi300x_v5|standard_nd96is_mi300x_v5) ;;
        *) echo "Unsupported AMD GPU VM SKU: ${vm_sku}"; return $ERR_AMD_GPU_UNSUPPORTED ;;
    esac

    if ! validateAmdGpuDriver; then
        echo "Baked AMD GPU driver validation failed; use a qualified AMD GPU VHD"
        return $ERR_AMD_GPU_VALIDATE_FAIL
    fi
}

validateAmdGpuDriver() {
    local marker=/opt/azure/amd-gpu/driver.json
    local package_version firmware_version module_version module_path kernel_version loaded_version
    [ -s "${marker}" ] || return 1
    jq -e '.schema_version == 1' "${marker}" >/dev/null || return 1
    package_version=$(jq -er '.package_version | strings | select(length > 0)' "${marker}") || return 1
    firmware_version=$(jq -er '.firmware_package_version | strings | select(length > 0)' "${marker}") || return 1
    module_version=$(jq -er '.module_version | strings | select(length > 0)' "${marker}") || return 1
    [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms)" = "install ok installed ${package_version}" ] || return 1
    [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms-firmware)" = "install ok installed ${firmware_version}" ] || return 1

    kernel_version=$(uname -r)
    module_path=$(modinfo -k "${kernel_version}" -F filename amdgpu) || return 1
    # Reject the inbox driver even if modprobe succeeds. DKMS must have built the
    # pinned driver for the running kernel, which may differ from the bake kernel.
    case "${module_path}" in
        /lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*|/usr/lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*) ;;
        *) return 1 ;;
    esac
    [ "$(modinfo -k "${kernel_version}" -F version amdgpu)" = "${module_version}" ] || return 1
    retrycmd_if_failure 12 5 30 modprobe amdgpu || return 1
    loaded_version=$(cat /sys/module/amdgpu/version) || return 1
    [ "${loaded_version}" = "${module_version}" ] || return 1
    retrycmd_if_failure 12 5 5 test -c /dev/kfd || return 1
    export -f validateAmdGpuDevices
    retrycmd_if_failure 12 5 5 bash -c validateAmdGpuDevices || return 1
    echo "AMD GPU driver ${module_version} ready on ${kernel_version}; eight GPUs detected"
}

validateAmdGpuDevices() {
    local gpu_id_file gpu_id count=0
    # Count active KFD agents, not DRM render nodes: MI300X VFs can preallocate
    # many more render nodes than the eight GPUs assigned to the VM.
    for gpu_id_file in /sys/class/kfd/kfd/topology/nodes/*/gpu_id; do
        [ -r "${gpu_id_file}" ] || continue
        gpu_id=$(cat "${gpu_id_file}") || return 1
        case "${gpu_id}" in
            ''|*[!0-9]*) return 1 ;;
            0) continue ;;
        esac
        count=$((count + 1))
    done
    [ "${count}" -eq 8 ]
}
