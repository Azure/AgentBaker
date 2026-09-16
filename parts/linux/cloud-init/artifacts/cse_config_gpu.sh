#!/bin/bash

ensureMigPartition(){
    mkdir -p /etc/systemd/system/mig-partition.service.d/
    touch /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf
    tee /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf > /dev/null <<EOF
[Service]
Environment="GPU_INSTANCE_PROFILE=${GPU_INSTANCE_PROFILE}"
Environment="NVIDIA_MIG_PROFILE_LAYOUT=${NVIDIA_MIG_PROFILE_LAYOUT}"
EOF
    # this is expected to fail and work only on next reboot
    # it MAY succeed, only due to unreliability of systemd
    # service type=Simple, which does not exit non-zero
    # on failure if ExecStart failed to invoke.
    systemctlEnableAndStart mig-partition 300
}

pullGPUDriverImage() {
    # Cache-miss path only. Retry to ride out a transient blip, but stay tight: a truly missing image
    # should fail fast rather than eat the shared CSE window the driver install needs next. retrycmd
    # also self-caps to the CSE budget, so this can't overrun provisioning.
    retrycmd_if_failure 3 5 120 ctr -n k8s.io image pull $NVIDIA_DRIVER_IMAGE_PULL_REF:$NVIDIA_DRIVER_IMAGE_TAG || return 1
    if [ "$NVIDIA_DRIVER_IMAGE_PULL_REF" != "$NVIDIA_DRIVER_IMAGE" ]; then
        # Converge onto the canonical ref that install and cleanup use. Removing the pull ref drops
        # only the name; the canonical tag still holds the manifest, so no blobs are collected.
        ctr -n k8s.io image tag $NVIDIA_DRIVER_IMAGE_PULL_REF:$NVIDIA_DRIVER_IMAGE_TAG $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG || return 1
        ctr -n k8s.io image rm $NVIDIA_DRIVER_IMAGE_PULL_REF:$NVIDIA_DRIVER_IMAGE_TAG
    fi
    return 0 # the rm above is best effort; don't let it set the caller's exit code
}

installGPUDriverImage() {
    retrycmd_if_failure 5 10 600 bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install"
}

# nvidia-cdi-refresh.service (nvidia-container-toolkit-base) is Type=oneshot with Restart=on-failure
# and StartLimitBurst=5/10s, so one nvidia-ctk failure burns the start limit in ~5s and leaves it and
# its .path unit permanently failed and unstartable -- which fails install.sh's `systemctl restart`
# (CSE 84) and kills the .path trigger that would otherwise regenerate the spec later. Tolerate the
# three known codes: 1 (MIG mode on with no MIG instances, which never succeeds), 2 (panic before the
# driver userspace is ready), 127 (nvidia-smi missing on pre-reorder aks-gpu images). Anything else
# still fails the unit. GPU pods reach the device via containerd's nvidia-container-runtime, not CDI.
NVIDIA_CDI_REFRESH_DROP_IN="/etc/systemd/system/nvidia-cdi-refresh.service.d/10-aks-tolerate-generate-failure.conf"

configureNvidiaCDIRefresh() {
    mkdir -p "$(dirname "$NVIDIA_CDI_REFRESH_DROP_IN")" || return 1
    printf '[Service]\nSuccessExitStatus=1 2 127\n' > "$NVIDIA_CDI_REFRESH_DROP_IN" || return 1
    # Drop-ins are read when systemd loads a unit, so writing this before the toolkit is installed
    # means the unit picks it up on its very first start.
    systemctl daemon-reload || true
}

configGPUDrivers() {
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        waitForContainerdReady || exit $ERR_GPU_DRIVERS_START_FAIL
        mkdir -p /opt/{actions,gpu}
        # Must precede the install: the toolkit's post-install starts nvidia-cdi-refresh from
        # inside the install container, and on newer aks-gpu images its failure aborts the install.
        logs_to_events "AKS.CSE.configGPUDrivers.configureNvidiaCDIRefresh" configureNvidiaCDIRefresh || exit $ERR_GPU_DRIVERS_START_FAIL
        # Normally the image is baked into the VHD; only pull on a cache miss (expected under VHD/CSE
        # version skew). ctr run does not auto-pull, so a failed pull must exit here rather than
        # resurface as an opaque install error. name== is an exact match, not an `images ls` substring.
        if [ -z "$(ctr -n k8s.io images ls -q "name==${NVIDIA_DRIVER_IMAGE}:${NVIDIA_DRIVER_IMAGE_TAG}")" ]; then
            logs_to_events "AKS.CSE.configGPUDrivers.pullGPUDriverImage" pullGPUDriverImage || exit $ERR_GPU_DRIVERS_START_FAIL
        fi
        logs_to_events "AKS.CSE.configGPUDrivers.installGPUDriverImage" installGPUDriverImage
        ret=$?
        if [ "$ret" -ne 0 ]; then
            echo "Failed to install GPU driver, exiting..."
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
        # Drop the driver image reference so containerd can reclaim its space, but skip --sync so
        # garbage collection runs asynchronously instead of blocking node provisioning.
        ctr -n k8s.io images rm $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
    elif isMarinerOrAzureLinux "$OS" && ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.configGPUDrivers.downloadGPUDrivers" downloadGPUDrivers
        logs_to_events "AKS.CSE.configGPUDrivers.installNvidiaContainerToolkit" installNvidiaContainerToolkit
        enableNvidiaPersistenceMode
    elif isACL "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.configGPUDrivers.installNvidiaContainerToolkitSysext" installNvidiaContainerToolkitSysext
        logs_to_events "AKS.CSE.configGPUDrivers.installGPUDriverSysext" installGPUDriverSysext
        enableNvidiaPersistenceMode
    else
        echo "os $OS $OS_VARIANT not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    logs_to_events "AKS.CSE.configGPUDrivers.waitForNvidiaModprobe" "retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0" || exit $ERR_GPU_DRIVERS_START_FAIL
    logs_to_events "AKS.CSE.configGPUDrivers.waitForNvidiaSmi" "retrycmd_if_failure 120 5 30 nvidia-smi" || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL

    # Fix the NVIDIA /dev/char link issue (Mariner/AzureLinux only)
    if isMarinerOrAzureLinux "$OS"; then
        createNvidiaSymlinkToAllDeviceNodes
    fi

    # GRID vGPU licensing: start nvidia-gridd service to ensure license configuration
    if (isMarinerOrAzureLinux "$OS" || isACL "$OS" "$OS_VARIANT") && [ "$NVIDIA_GPU_DRIVER_TYPE" = "grid" ]; then
        systemctlEnableAndStart nvidia-gridd 300 || exit $ERR_SYSTEMCTL_START_FAIL
    fi

    systemctlEnableAndStart containerd 30 || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT

    # NPD is installed as a VM extension, which might happen before/after/during CSE, so this
    # line may fail. This will need to be updated when NPD is shipped in the VHD - we can control
    # the startup ordering in that case.
    systemctl restart node-problem-detector || true
}

validateGPUDrivers() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL

    if which nvidia-smi; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 30 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 30 $GPU_DEST/bin/nvidia-smi)
    fi
    SMI_STATUS=$?
    if [ "$SMI_STATUS" -ne 0 ]; then
        # shellcheck disable=SC3010
        if [[ $SMI_RESULT == *"infoROM is corrupted"* ]]; then
            exit $ERR_GPU_INFO_ROM_CORRUPTED
        else
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
    else
        echo "gpu driver working fine"
    fi
}

# logGPUDriverPrebakeReadiness emits a stage-1 observability signal on a managed GPU node: whether
# the aks-gpu prebake marker is present and matches this node's driver kind -- i.e. whether stage-2
# (skip-build) would take the fast path. Lets the rollout confirm managed CUDA GPU nodes are ready
# before enabling consume. Observability only; no behavior change.
logGPUDriverPrebakeReadiness() {
    local marker="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}"
    local marker_present=false driver_kind_match=false m_kind node_kind
    # Map the AgentBaker driver-type to the aks-gpu marker's driver_kind (the container's DRIVER_KIND
    # build arg): image variants "cuda-lts" and "grid-v20" bake markers as "cuda"/"grid" respectively.
    case "${NVIDIA_GPU_DRIVER_TYPE}" in
        cuda*) node_kind=cuda ;;
        grid*) node_kind=grid ;;
        *) node_kind="${NVIDIA_GPU_DRIVER_TYPE}" ;;
    esac
    if [ -f "${marker}" ]; then
        marker_present=true
        m_kind="$(sed -n 's/^driver_kind=//p' "${marker}" | head -n1)"
        # require both sides non-empty so a marker missing driver_kind= (or an unset
        # NVIDIA_GPU_DRIVER_TYPE) does not falsely report a match (empty = empty).
        if [ -n "${m_kind}" ] && [ -n "${node_kind}" ] && [ "${m_kind}" = "${node_kind}" ]; then
            driver_kind_match=true
        fi
    fi
    echo "AKS_GPU_PREBAKE event=managed_gpu driver_type=${NVIDIA_GPU_DRIVER_TYPE:-} marker_present=${marker_present} driver_kind_match=${driver_kind_match}"
}

# cleanUpGridNodeCudaPrebake tears down a cuda-lts driver pre-baked into the shared Ubuntu VHD when
# THIS node installs a GRID driver. The shared VHD bakes only the cuda(-lts) driver + a DKMS marker;
# a GRID/converged (A10, NVv5) node then installs the grid driver on top, and the stale prebaked
# cuda module + its /usr/bin/lib64 userspace libs collide with the grid driver -> nvidia-smi fails
# with "Failed to initialize NVML: Driver/library version mismatch". This is a pure driver-KIND
# mismatch (grid vs cuda), so no version comparison is needed. A legacy marker with no driver_kind=
# line is treated as a cuda prebake (the only kind the VHD bakes today). No-op unless the node is
# GRID and a prebake marker exists. Reuses cleanUpPrebakedGPUDriver (from cse_install_ubuntu.sh) for
# the actual removal. NAP/agentpool cuda nodes are intentionally untouched here.
cleanUpGridNodeCudaPrebake() {
    [ "$OS" = "$UBUNTU_OS_NAME" ] || return 0
    local marker="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}"
    [ -f "${marker}" ] || return 0

    local node_kind m_kind
    case "${NVIDIA_GPU_DRIVER_TYPE:-}" in
        grid*) node_kind=grid ;;
        *) return 0 ;;
    esac
    m_kind="$(sed -n 's/^driver_kind=//p' "${marker}" | head -n1)"

    # Keep only when the prebake is explicitly grid (matches this grid node). An empty marker kind is
    # a legacy cuda prebake; a "cuda" marker is a cuda prebake -- both mismatch a grid node, tear down.
    if [ "${m_kind}" = "grid" ]; then
        return 0
    fi
    echo "AKS_GPU_PREBAKE event=grid_cuda_prebake_teardown driver_type=${NVIDIA_GPU_DRIVER_TYPE:-} marker_kind=${m_kind:-none} node_kind=${node_kind} action=teardown"
    cleanUpPrebakedGPUDriver
}

ensureGPUDrivers() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    # Tear down a mismatched cuda-lts VHD prebake before a GRID node installs its own driver, or the
    # stale module/libs collide with the grid driver (NVML version mismatch). Runs before the dispatch
    # below so it covers both the configGPUDrivers and validateGPUDrivers paths.
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        # Called only by nodePrep for managed GPU nodes. Restore before GRID teardown or driver
        # validation; a loadable .ko alone does not ensure DKMS can handle later kernel updates.
        logs_to_events "AKS.CSE.ensureGPUDrivers.restorePrebakedGPUDriverRegistration" setPrebakedGPUDriverRegistration restore || exit $ERR_GPU_DRIVERS_START_FAIL
        logs_to_events "AKS.CSE.ensureGPUDrivers.cleanUpGridNodeCudaPrebake" cleanUpGridNodeCudaPrebake || exit $ERR_GPU_DRIVERS_START_FAIL
    fi

    if [ "${CONFIG_GPU_DRIVER_IF_NEEDED}" = true ]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.configGPUDrivers" configGPUDrivers
    else
        logs_to_events "AKS.CSE.ensureGPUDrivers.validateGPUDrivers" validateGPUDrivers
    fi
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.nvidia-modprobe" "systemctlEnableAndStart nvidia-modprobe 30" || exit $ERR_GPU_DRIVERS_START_FAIL
        logGPUDriverPrebakeReadiness
    fi
}

# Install AMD AMA core SW package for MA35D (Supernova GPU SKU)
dnf_install_amd_ama_core_package() {
    ver=$1; retries=$2; wait_sleep=$3; timeout=$4; shift && shift && shift && shift
    # Currently version 1.5.0 is supported.  Add more versions as they become available.
    if [ "$ver" = "1.5.0" ]; then
        AMD_AMA_CORE_PACKAGE="https://download.microsoft.com/download/f030c57a-a582-4bcc-9c7c-593c9a486814/amd-ama-core_1.5.0-20260424092403.x86_64.rpm"
    else
        return 1
    fi
    for i in $(seq 1 $retries); do
        # RPM_FRONTEND env variable needed to disable license agreement prompt
        RPM_FRONTEND=noninteractive dnf install -y $AMD_AMA_CORE_PACKAGE && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            dnf_makecache
        fi
    done
    echo Executed dnf install AMD AMA core package $i times;
}

# Install AMD AMA drivers/SW for MA35D (Supernova GPU SKU)
# Note that this depends on access to download.microsoft.com, so network-isolated clusters are not supported
setupAmdAma() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    if isAzureLinux "$OS"; then
        # Install MA35D packages - currently version 1.5 and above are supported
        # This install script will extract the driver version/build number to find
        # the corresponding core/FW packages

        # Get driver package name from internal AMD repo
        if ! dnf_install 30 1 600 azurelinux-repos-amd; then
          echo "Unable to install Azure Linux AMD package repo, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        KERNEL_VERSION=$(uname -r | sed 's/-/./g')
        AMD_AMA_DRIVER_PACKAGE=$(dnf repoquery -y --available "amd-ama-driver-*" | grep -E "amd-ama-driver-[0-9]+.*_$KERNEL_VERSION" | sort -V | tail -n 1)
        if [ -z "$AMD_AMA_DRIVER_PACKAGE" ]; then
            echo "Unable to find AMD AMA driver package for current kernel version, exiting..."
            exit $ERR_AMDAMA_DRIVER_NOT_FOUND
        fi

        # Install FW package
        AMD_AMA_FIRMWARE_PACKAGE="${AMD_AMA_DRIVER_PACKAGE/driver/firmware}"
        if [ -z "$AMD_AMA_FIRMWARE_PACKAGE" ] || [ "$AMD_AMA_FIRMWARE_PACKAGE" = "$AMD_AMA_DRIVER_PACKAGE" ]; then
            echo "Unable to find AMD AMA firmware package for current kernel version, exiting..."
            exit $ERR_AMDAMA_DRIVER_NOT_FOUND
        fi
        if ! dnf_install 30 1 600 $AMD_AMA_FIRMWARE_PACKAGE; then
          echo "Unable to install AMD AMA FW package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi

        # Install driver package
        if ! dnf_install 30 1 600 $AMD_AMA_DRIVER_PACKAGE; then
          echo "Unable to install AMD AMA driver package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi

        # Install core package
        if ! dnf_install 30 1 600 azurelinux-repos-extended libzip; then
          echo "Unable to install Azure Linux packages required for AMD AMA core package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        TMP="${AMD_AMA_DRIVER_PACKAGE#amd-ama-driver-*:}"; AMD_AMA_DRIVER_VERSION="${TMP%%_*}"
        if ! dnf_install_amd_ama_core_package $AMD_AMA_DRIVER_VERSION 30 1 600; then
          echo "Unable to install AMD AMA core package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi

        # Install AKS device plugin
        if ! dnf_install 30 1 600 amdama-device-plugin.x86_64; then
          echo "Unable to install AMD AMA AKS device plugin package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        # Configure huge pages
        sh -c "echo 'vm.nr_hugepages=4096' > /etc/sysctl.d/99-ama_transcoder.conf"
        sh -c "echo 4096 > /proc/sys/vm/nr_hugepages"
        if [ "$(systemctl is-active kubelet)" = "active" ]; then
            systemctl restart kubelet
        fi
    fi
}

configureManagedGPUExperience() {
    if [ "${GPU_NODE}" != "true" ] || [ "${skip_nvidia_driver_install}" = "true" ]; then
        return
    fi
    if [ "${ENABLE_MANAGED_GPU_EXPERIENCE}" = "true" ] && [ "${ENABLE_MANAGED_GPU_EXPERIENCE_DRA}" = "true" ]; then
        echo "Error: ENABLE_MANAGED_GPU_EXPERIENCE and ENABLE_MANAGED_GPU_EXPERIENCE_DRA cannot both be true"
        exit $ERR_ENABLE_MANAGED_GPU_EXPERIENCE
    fi
    local managed_gpu_marker="/opt/azure/containers/managed-gpu-experience.enabled"
    if [ "${ENABLE_MANAGED_GPU_EXPERIENCE}" = "true" ]; then
        logs_to_events "AKS.CSE.installNvidiaManagedExpPkgFromCache" "installNvidiaManagedExpPkgFromCache" || exit $ERR_NVIDIA_DCGM_INSTALL
        logs_to_events "AKS.CSE.startNvidiaManagedExpServices" "startNvidiaManagedExpServices" || exit $ERR_NVIDIA_DCGM_EXPORTER_FAIL
        addKubeletNodeLabel "kubernetes.azure.com/dcgm-exporter=enabled"
        mkdir -p "$(dirname "${managed_gpu_marker}")"
        touch "${managed_gpu_marker}"
    elif [ "${ENABLE_MANAGED_GPU_EXPERIENCE_DRA}" = "true" ]; then
        logs_to_events "AKS.CSE.installNvidiaManagedExpPkgFromCache" "installNvidiaManagedExpPkgFromCache" || exit $ERR_NVIDIA_DCGM_INSTALL
        # defer startNvidiaManagedExpServices() after kubelet starts
        addKubeletNodeLabel "kubernetes.azure.com/dcgm-exporter=enabled"
        mkdir -p "$(dirname "${managed_gpu_marker}")"
        touch "${managed_gpu_marker}"
    else
        # EnableManagedGPUExperience is mutable, so services may have been
        # installed on a previous CSE run. Stop them if they exist.
        # systemctlDisableAndStop check if the service exists before attempting to stop it,
        # so this is safe to call even if the services were never installed.
        logs_to_events "AKS.CSE.stop.nvidia-device-plugin" "systemctlDisableAndStop nvidia-device-plugin"
        logs_to_events "AKS.CSE.stop.dra-driver-nvidia-gpu" "systemctlDisableAndStop dra-driver-nvidia-gpu"
        logs_to_events "AKS.CSE.stop.nvidia-dcgm" "systemctlDisableAndStop nvidia-dcgm"
        logs_to_events "AKS.CSE.stop.nvidia-dcgm-exporter" "systemctlDisableAndStop nvidia-dcgm-exporter"
        rm -f "${managed_gpu_marker}"
    fi
}

startNvidiaManagedExpServices() {
    # 1. Start the nvidia-device-plugin service or dra-driver-nvidia-gpu service.
    if [ "${ENABLE_MANAGED_GPU_EXPERIENCE}" = "true" ]; then
        # Create systemd override directory to configure device plugin
        NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR="/etc/systemd/system/nvidia-device-plugin.service.d"
        mkdir -p "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}"

        if [ "${MIG_NODE}" = "true" ]; then
            # Configure with MIG strategy for MIG nodes.
            # MIG strategy controls how nvidia-device-plugin exposes MIG instances to Kubernetes:
            #   - "single": All MIG devices exposed as generic nvidia.com/gpu resources
            #   - "mixed": MIG devices exposed with specific types like nvidia.com/mig-1g.5gb
            #
            # We only use "mixed" when explicitly specified via NVIDIA_MIG_STRATEGY.
            # Otherwise, we default to "single" which is the safer/simpler option.
            # Note: NVIDIA_MIG_STRATEGY values from RP are "None", "Single", "Mixed".
            # "None" and "Single" both result in using the "single" strategy.
            if [ "${NVIDIA_MIG_STRATEGY}" = "Mixed" ]; then
                MIG_STRATEGY_FLAG="--mig-strategy mixed"
            else
                # Default to "single" for "Single", "None", empty, or any other value
                MIG_STRATEGY_FLAG="--mig-strategy single"
            fi

            tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin ${MIG_STRATEGY_FLAG} --pass-device-specs
EOF
        else
            # Configure with pass-device-specs for non-MIG nodes
            tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin --pass-device-specs
EOF
        fi

        # Reload systemd to pick up the override
        systemctl daemon-reload

        logs_to_events "AKS.CSE.start.nvidia-device-plugin" "systemctlEnableAndStart nvidia-device-plugin 30" || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL
    elif [ "${ENABLE_MANAGED_GPU_EXPERIENCE_DRA}" = "true" ]; then
        DRA_DRIVER_NVIDIA_GPU_OVERRIDE_DIR="/etc/systemd/system/dra-driver-nvidia-gpu.service.d"
        mkdir -p "${DRA_DRIVER_NVIDIA_GPU_OVERRIDE_DIR}"

        tee "${DRA_DRIVER_NVIDIA_GPU_OVERRIDE_DIR}/10-dra-driver-nvidia-gpu.conf" > /dev/null <<EOF
[Unit]
Requires=kubelet.service
After=kubelet.service
[Service]
ExecStart=
ExecStart=/usr/bin/gpu-kubelet-plugin --kubeconfig /var/lib/kubelet/kubeconfig --container-driver-root / --image-name "" --node-name=${NODE_NAME}
EOF

        # Reload systemd to pick up the override
        systemctl daemon-reload

        logs_to_events "AKS.CSE.start.dra-driver-nvidia-gpu" "systemctlEnableAndStart dra-driver-nvidia-gpu 30" || exit $ERR_DRA_DRIVER_START_FAIL
    fi

    # 2. Start the nvidia-dcgm service.
    # DCGM is monitoring/telemetry and does not gate GPU workload scheduling, so start it without
    # blocking node provisioning and treat a slow/failed start as non-fatal.
    logs_to_events "AKS.CSE.start.nvidia-dcgm" "systemctlEnableAndStartNoBlock nvidia-dcgm 30" || echo "warning: nvidia-dcgm could not be enqueued; GPU monitoring will start asynchronously"

    # 3. Start the nvidia-dcgm-exporter service.
    # Create systemd drop-in directory for nvidia-dcgm-exporter service
    DCGM_EXPORTER_OVERRIDE_DIR="/etc/systemd/system/nvidia-dcgm-exporter.service.d"
    mkdir -p "${DCGM_EXPORTER_OVERRIDE_DIR}"

    # Create drop-in file to override service configuration
    tee "${DCGM_EXPORTER_OVERRIDE_DIR}/10-aks-override.conf" > /dev/null <<EOF
[Service]
# Remove file-based logging - let systemd handle logs
StandardOutput=journal
StandardError=journal
# Change default port from 9400 to 19400 so that it does not conflict with user installed dcgm-exporter
ExecStart=
ExecStart=/usr/bin/dcgm-exporter -f /etc/dcgm-exporter/default-counters.csv --address ":19400"
EOF

    # Reload systemd to apply the override configuration
    systemctl daemon-reload

    # Start the nvidia-dcgm-exporter service.
    # The exporter is telemetry only and does not gate scheduling, so start it off the critical
    # path and treat a slow/failed start as non-fatal.
    logs_to_events "AKS.CSE.start.nvidia-dcgm-exporter" "systemctlEnableAndStartNoBlock nvidia-dcgm-exporter 30" || echo "warning: nvidia-dcgm-exporter could not be enqueued; GPU metrics will start asynchronously"
}

#EOF
