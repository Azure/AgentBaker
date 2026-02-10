#!/bin/bash

echo "Sourcing cse_install_distro.sh for ACL"

stub() {
    echo "${FUNCNAME[1]} stub"
}

downloadSysextFromVersion() {
    local seName=$1
    local seURL=$2
    local downloadDir=${3:-"/opt/${seName}/downloads"}

    if ! retrycmd_if_failure 120 5 60 oras pull --registry-config "${ORAS_REGISTRY_CONFIG_FILE}" --output "${downloadDir}" "${seURL}"; then
        echo "Failed to download ${seName} system extension from ${seURL}"
        return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
    fi

    echo "Succeeded to download ${seName} system extension from ${seURL}"
}

matchLocalSysext() {
    local seName=$1 desiredVer=$2 seArch=$3
<<<<<<< HEAD
    local downloadDir="/opt/${seName}/downloads"
    # Try arch-specific versioned filename first (kubelet-style: name-vVER.X-arch.raw)
    local match
    match=$(find "${downloadDir}" -maxdepth 2 -name "${seName}-v${desiredVer}*-${seArch}.raw" -type f 2>/dev/null | sort -V | tail -n1)
=======
    # Try arch-specific versioned filename first (kubelet-style: name-vVER.X-arch.raw)
    local match
    match=$(printf "%s\n" "/opt/${seName}/downloads/${seName}-v${desiredVer}"[.~-]*"-${seArch}.raw" | sort -V | tail -n1)
>>>>>>> 5fac68b53c (feat: onboard ACL GPU provisioning support)
    if [ -f "${match}" ]; then
        echo "${match}"
        return
    fi
<<<<<<< HEAD
    # Fallback: GPU sysexts are downloaded as simple name.raw (e.g. nvidia-driver-vgpu.raw).
    # MCR artifacts may place files in an arch subdirectory (e.g. amd64/name.raw),
    # so search up to 2 levels deep.
    match=$(find "${downloadDir}" -maxdepth 2 -name "${seName}.raw" -type f 2>/dev/null | head -n1)
=======
    # Fallback: GPU sysexts are downloaded as simple name.raw (e.g. nvidia-driver-vgpu.raw)
    match=$(find "/opt/${seName}/downloads" -maxdepth 1 -name "${seName}.raw" -type f 2>/dev/null | head -n1)
>>>>>>> 5fac68b53c (feat: onboard ACL GPU provisioning support)
    echo "${match}"
}

matchRemoteSysext() {
    local seURL=$1 desiredVer=$2 seArch=$3
    # Match either arch-specific tags (v{ver}[.~-]*-azlinux3-{arch}) or exact version tags ({ver})
    retrycmd_silent 120 5 20 oras repo tags --registry-config "${ORAS_REGISTRY_CONFIG_FILE}" "${seURL}" | grep -Ex "(v${desiredVer//./\\.}[.~-].*-azlinux3-${seArch}|${desiredVer//./\\.})" | sort -V | tail -n1
    test ${PIPESTATUS[0]} -eq 0
}

mergeSysexts() {
    local seArch
    seArch=$(getSystemdArch)

    while [ "${1-}" ]; do
        local seName=$1 seURL=$2 desiredVer=$3 seMatch

        seMatch=$(matchLocalSysext "${seName}" "${desiredVer}" "${seArch}")
        if ! test -f "${seMatch}"; then
            echo "Failed to find valid ${seName} system extension for ${desiredVer} locally"

            seMatch=$(matchRemoteSysext "${seURL}" "${desiredVer}" "${seArch}")
            if [ -z "${seMatch}" ]; then
                echo "Failed to find valid ${seName} system extension for ${desiredVer} remotely"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            if ! downloadSysextFromVersion "${seName}" "${seURL}:${seMatch}"; then
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            seMatch=$(matchLocalSysext "${seName}" "${desiredVer}" "${seArch}")
            if ! test -f "${seMatch}"; then
                echo "Failed to find valid ${seName} system extension for ${desiredVer} after downloading"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi
        fi

        ln -snf "${seMatch}" "/etc/extensions/${seName}.raw"
        shift 3
    done

    systemd-sysext --no-reload refresh
}

installDeps() {
    stub
}

installCriCtlPackage() {
    stub
}

installKubeletKubectlFromPkg() {
    if mergeSysexts kubelet "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/kubelet-sysext "$1" \
                    kubectl "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/kubectl-sysext "$1"; then
        ln -snf /usr/bin/{kubelet,kubectl} /opt/bin/
    else
        installKubeletKubectlFromURL
    fi
}

installKubeletKubectlFromBootstrapProfileRegistry() {
    installKubeletKubectlFromPkg "$2" "$1"
}

installCredentialProviderFromPkg() {
    if mergeSysexts azure-acr-credential-provider "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/azure-acr-credential-provider-sysext "$1"; then
        mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
        chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
        ln -snf /usr/bin/azure-acr-credential-provider "$CREDENTIAL_PROVIDER_BIN_DIR/acr-credential-provider"
    else
        installCredentialProviderFromUrl
    fi
}

installCredentialProviderPackageFromBootstrapProfileRegistry() {
    installCredentialProviderFromPkg "$2" "$1"
}

# Reads VERSION_ID from /etc/os-release for use as the sysext version tag.
# GPU sysexts are tagged by the OS image version, not the driver version.
getACLVersionID() {
    # shellcheck disable=SC1091
    source /etc/os-release
    if [ -z "${VERSION_ID}" ]; then
        echo "ERROR: VERSION_ID not found in /etc/os-release" >&2
        return "${ERR_SYSEXT_VERSION_ID_NOT_FOUND}"
    fi
    echo "${VERSION_ID}"
}

# Pulls a GPU-related sysext by name using the ACL MCR registry.
# Registry path uses major.minor (e.g. 3.0), tag uses full VERSION_ID (e.g. 3.0.20260304).
# Example: mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-cuda:3.0.20260304
installACLGPUSysext() {
    local sysext_name=$1
    local version_id
    version_id=$(getACLVersionID) || exit $ERR_SYSEXT_VERSION_ID_NOT_FOUND
    local mcr_base="${MCR_REPOSITORY_BASE:-mcr.microsoft.com}"
    local registry_base="${mcr_base%/}/azurelinux/${version_id%.*}/azure-container-linux"
    mergeSysexts "${sysext_name}" "${registry_base}/${sysext_name}" "${version_id}" \
        || exit $ERR_ORAS_PULL_SYSEXT_FAIL
}

installGPUDriverSysext() {
    # ACL NVIDIA GPU driver sysext registry paths:
    # Registry path uses major.minor (e.g. 3.0), tag uses full VERSION_ID (e.g. 3.0.20260304).
    #
    # 1. NVIDIA proprietary driver:
    # mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-cuda:v${VERSION_ID}...
    #
    # 2. NVIDIA OpenRM driver:
    # mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-cuda-open:v${VERSION_ID}...
    #
    # 3. NVIDIA GRID (vGPU guest) driver for converged GPU sizes:
    # mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-vgpu:v${VERSION_ID}...
    #
    # NVIDIA_GPU_DRIVER_TYPE is set by AgentBaker based on ConvergedGPUDriverSizes map
    # in gpu_components.go. Converged sizes get "grid"; all others get "cuda".
    # Legacy GPUs (T4, V100) require proprietary CUDA drivers; A100+ use NVIDIA open drivers.
    local vm_sku
    vm_sku=$(get_compute_sku)
    local sysext_name

    # Converged GPU sizes (NVads_A10_v5, NCads_A10_v4) use GRID drivers
    if [ "$NVIDIA_GPU_DRIVER_TYPE" = "grid" ]; then
        echo "VM SKU ${vm_sku} uses NVIDIA GRID driver (converged)"
        sysext_name="nvidia-driver-vgpu"
    else
        local driver_ret
        should_use_nvidia_open_drivers
        driver_ret=$?
        if [ "$driver_ret" -eq 2 ]; then
            echo "Failed to determine GPU driver type"
            exit $ERR_MISSING_CUDA_PACKAGE
        elif [ "$driver_ret" -eq 0 ]; then
            echo "VM SKU ${vm_sku} uses NVIDIA OpenRM driver (cuda-open)"
            sysext_name="nvidia-driver-cuda-open"
        else
            echo "VM SKU ${vm_sku} uses NVIDIA proprietary driver (cuda)"
            sysext_name="nvidia-driver-cuda"
        fi
    fi

    installACLGPUSysext "${sysext_name}"

    # Process tmpfiles.d rules shipped inside the GPU sysexts (e.g. symlink
    # /etc/nvidia/gridd.conf -> /usr/share/nvidia/gridd.conf). The sysext
    # overlay only covers /usr; files under /etc must be created on the
    # writable root via tmpfiles.d rules.
    systemd-tmpfiles --create
}

installNvidiaContainerToolkitSysext() {
    installACLGPUSysext nvidia-container-toolkit
}

installNvidiaFabricManagerSysext() {
    installACLGPUSysext nvidia-fabric-manager
}

ensureRunc() {
    stub
}

removeNvidiaRepos() {
    stub
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
