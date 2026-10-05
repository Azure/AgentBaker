#!/bin/bash

Describe 'GPU cleanup across PIS phases'
    CSE_MAIN="./parts/linux/cloud-init/artifacts/cse_main.sh"

    run_phases() {
        local dispatch cleanup
        dispatch=$(awk '
            /^if \[ ! -f \/opt\/azure\/containers\/base_prep.complete \]; then/ { reading = 1 }
            reading && /^echo "Custom script finished."/ { exit }
            reading { print }
        ' "$CSE_MAIN")
        cleanup=$(sed -n '/    # Clean up GPU drivers if not a GPU node/,/    fi/p' "$CSE_MAIN")
        dispatch=${dispatch//\/opt\/azure\/containers\/base_prep.complete/$marker}
        basePrep() { echo "basePrep"; }
        nodePrep() { eval "$cleanup"; }
        logs_to_events() { shift; "$@"; }
        cleanUpGPUDrivers() {
            rm -rf "$driver_root/usr/local/nvidia" "$driver_root/var/lib/dkms/nvidia" \
                "$driver_root/lib/modules/test/updates/dkms/nvidia.ko" "$driver_root/usr/bin/nvidia-smi"
            rm -f "$driver_root/dkms-marker"
        }
        eval "$dispatch"
    }

    driver_survives() {
        run_phases
        test -d "$driver_root/usr/local/nvidia" &&
            test -d "$driver_root/var/lib/dkms/nvidia" &&
            test -f "$driver_root/lib/modules/test/updates/dkms/nvidia.ko" &&
            test -f "$driver_root/usr/bin/nvidia-smi" &&
            test -f "$driver_root/dkms-marker"
    }

    setup_driver() {
        driver_root=$(mktemp -d)
        marker="$driver_root/base_prep.complete"
        mkdir -p "$driver_root/usr/local/nvidia" "$driver_root/var/lib/dkms/nvidia" \
            "$driver_root/lib/modules/test/updates/dkms" "$driver_root/usr/bin"
        touch "$driver_root/dkms-marker" \
            "$driver_root/lib/modules/test/updates/dkms/nvidia.ko" "$driver_root/usr/bin/nvidia-smi"
        GPU_NODE=false
        PRE_PROVISION_ONLY=false
        skip_nvidia_driver_install=false
    }
    BeforeEach 'setup_driver'
    AfterEach 'rm -rf "$driver_root"'

    It 'preserves customer-installed driver artifacts on a derived opt-out node'
        touch "$marker"
        When call driver_survives
        The status should be success
        The output should include "Skipping basePrep"
    End

    It 'preserves customer drivers on a derived GPU node with the legacy skip tag'
        touch "$marker"
        GPU_NODE=true
        skip_nvidia_driver_install=true
        When call driver_survives
        The status should be success
        The output should include "Skipping basePrep"
    End

    Describe 'PIS basePrep completion marker'
        run_cse_marker() {
            local marker_script
            marker_script=$(awk '
                /^if \[ "\$\{PRE_PROVISION_ONLY\}" = "true" \]; then/ { reading = 1 }
                reading && /^if \[ "\$EXIT_CODE" -ne 0 \]; then/ { exit }
                reading { print }
            ' ./parts/linux/cloud-init/artifacts/cse_start.sh)
            marker_script=${marker_script//\/opt\/azure\/containers/$marker_root}
            marker_script=${marker_script//\/var\/log\/azure\/cluster-provision.log/$marker_root/provision.log}
            eval "$marker_script"
        }

        setup_marker() { marker_root=$(mktemp -d); PRE_PROVISION_ONLY=true; }
        BeforeEach 'setup_marker'
        AfterEach 'rm -rf "$marker_root"'

        It 'does not capture a failed GPU teardown as a completed image'
            EXIT_CODE=224
            When call run_cse_marker
            The status should be success
            The path "$marker_root/base_prep.complete" should not be exist
        End

        It 'marks a successful bake for derived nodes'
            EXIT_CODE=0
            When call run_cse_marker
            The status should be success
            The path "$marker_root/base_prep.complete" should be exist
        End
    End

    It 'still cleans an opt-out node when basePrep ran in this execution'
        When call run_phases
        The status should be success
        The output should include "basePrep"
        The path "$driver_root/usr/local/nvidia" should not be exist
    End

    It 'still cleans a non-PIS node with the legacy skip-driver tag'
        GPU_NODE=true
        skip_nvidia_driver_install=true
        When call run_phases
        The status should be success
        The output should include "basePrep"
        The path "$driver_root/usr/local/nvidia" should not be exist
    End

    It 'does not clean a GPU bake VM'
        PRE_PROVISION_ONLY=true
        When call driver_survives
        The status should be success
        The output should include "basePrep"
    End

    base_cleanup() {
        local cleanup
        cleanup=$(sed -n '/    if \[ "${PRE_PROVISION_ONLY}" = "true" \] && \[ "${GPU_NODE}" != "true" \] &&/,/    fi/p' "$CSE_MAIN")
        logs_to_events() { shift; "$@"; }
        cleanUpGPUDriversForBasePrep() { echo "basePrep driver cleanup"; return "${CLEANUP_STATUS:-0}"; }
        isMarinerOrAzureLinux() { [ "$OS" = azurelinux ]; }
        isAzureLinuxOSGuard() { [ "$OS_VARIANT" = osguard ]; }
        UBUNTU_OS_NAME=ubuntu
        ERR_NVIDIA_DRIVER_INSTALL=224
        ( eval "$cleanup" )
    }

    It 'tears down AKS drivers on a non-GPU bake VM before image capture'
        PRE_PROVISION_ONLY=true
        OS=ubuntu
        When call base_cleanup
        The status should be success
        The output should include "basePrep driver cleanup"
    End

    It 'fails basePrep when bake-time teardown cannot be verified'
        PRE_PROVISION_ONLY=true
        OS=ubuntu
        CLEANUP_STATUS=1
        When call base_cleanup
        The status should equal 224
        The output should include "basePrep driver cleanup"
    End

    It 'does not tear down AKS drivers on a GPU bake VM'
        PRE_PROVISION_ONLY=true
        OS=ubuntu
        GPU_NODE=true
        When call base_cleanup
        The status should be success
        The output should not include "basePrep driver cleanup"
    End

    It 'does not tear down AKS drivers in non-PIS basePrep'
        OS=ubuntu
        When call base_cleanup
        The status should be success
        The output should not include "basePrep driver cleanup"
    End

    It 'tears down AKS caches on an Azure Linux bake VM'
        PRE_PROVISION_ONLY=true
        OS=azurelinux
        When call base_cleanup
        The status should be success
        The output should include "basePrep driver cleanup"
    End

    It 'does not call unavailable cleanup on Azure Container Linux'
        PRE_PROVISION_ONLY=true
        OS=azurelinux
        OS_VARIANT=osguard
        When call base_cleanup
        The status should be success
        The output should not include "basePrep driver cleanup"
    End
End
