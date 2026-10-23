#!/bin/bash

Describe 'GPU cleanup across PIS phases'
    CSE_MAIN="./parts/linux/cloud-init/artifacts/cse_main.sh"
    fixture="${PWD}/.shellspec-cse-main-gpu-pis-$$"

    setup_driver() {
        driver_root="${fixture}/driver"
        marker="${fixture}/base_prep.complete"
        mkdir -p "${driver_root}/usr/local/nvidia"
        GPU_NODE=false
        PRE_PROVISION_ONLY=false
        skip_nvidia_driver_install=false
        BASE_PREP_RAN=false
    }

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
            rm -rf "${driver_root}/usr/local/nvidia"
            touch "${fixture}/cleanup-called"
        }
        eval "$dispatch"
    }

    run_cse_marker() {
        local marker_script
        PRE_PROVISION_ONLY=true
        marker_script=$(awk '
            /^if \[ "\$\{PRE_PROVISION_ONLY\}" = "true" \]; then/ { reading = 1 }
            reading && /^if \[ "\$EXIT_CODE" -ne 0 \]; then/ { exit }
            reading { print }
        ' ./parts/linux/cloud-init/artifacts/cse_start.sh)
        marker_script=${marker_script//\/opt\/azure\/containers/$fixture}
        marker_script=${marker_script//\/var\/log\/azure\/cluster-provision.log/$fixture/provision.log}
        eval "$marker_script"
    }

    BeforeEach 'setup_driver'
    AfterEach 'rm -rf "$fixture"'

    It 'preserves customer drivers on a PIS-derived opt-out node'
        touch "$marker"
        When call run_phases
        The status should be success
        The output should include "Skipping basePrep"
        The path "$driver_root/usr/local/nvidia" should be exist
        The path "$fixture/cleanup-called" should not be exist
    End

    It 'cleans CUDA drivers only after basePrep ran in this execution'
        When call run_phases
        The status should be success
        The output should include "basePrep"
        The path "$driver_root/usr/local/nvidia" should not be exist
        The path "$fixture/cleanup-called" should be exist
    End

    It 'does not write the bake marker after failed basePrep'
        EXIT_CODE=224
        When call run_cse_marker
        The status should be success
        The path "$fixture/base_prep.complete" should not be exist
    End

    It 'writes the bake marker after successful basePrep'
        EXIT_CODE=0
        When call run_cse_marker
        The status should be success
        The path "$fixture/base_prep.complete" should be exist
    End
End
