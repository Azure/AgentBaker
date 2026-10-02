#!/bin/bash

Describe 'NVIDIA prebake producer opt-in'
    setup() {
        OS=UBUNTU UBUNTU_OS_NAME=UBUNTU
        NVIDIA_DRIVER_IMAGE=image NVIDIA_DRIVER_IMAGE_TAG=580.159.04-test
        VHD_LOGS_FILEPATH="${SHELLSPEC_TMPBASE}/vhd.log"
        marker="${SHELLSPEC_TMPBASE}/marker"
        # Exercise the actual producer function with just its fixed marker path redirected.
        sed -n '/^buildNVIDIAKernelModule() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh |
            sed "s|/opt/azure/aks-gpu/dkms-marker|${marker}|g" > "${SHELLSPEC_TMPBASE}/build-function.sh"
        source "${SHELLSPEC_TMPBASE}/build-function.sh"
    }
    BeforeEach 'setup'
    isARM64() { echo 0; }
    apt_get_install() { echo TOOLCHAIN; }
    retrycmd_if_failure() { touch "$marker"; echo "$*"; }
    prepareStagedGPUDriver() { echo PREPARE; }
    stageGPUDriver() { echo STAGE; }

    Context 'existing behavior'
        Parameters
            NVIDIA_CUDA_PREBAKE
            NVIDIA_CUDA_PREBAKE,UNRELATED_FLAG
        End
        It "still prebakes without staging for $1"
            FEATURE_FLAGS="$1"
            When run buildNVIDIAKernelModule
            The status should be success
            The output should include '/entrypoint.sh build-only'
            The output should not include PREPARE
            The output should not include STAGE
            The contents of file "$VHD_LOGS_FILEPATH" should include nvidia-cuda-driver-prebaked
        End
    End

    It 'stages only with the separate comma-separated opt-in'
        FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE,NVIDIA_PREBAKE_STAGE
        When run buildNVIDIAKernelModule
        The status should be success
        The output should include '/entrypoint.sh build-only'
        The output should include PREPARE
        The output should include STAGE
    End

    It 'preserves the no-prebake default'
        FEATURE_FLAGS=None
        When run buildNVIDIAKernelModule
        The status should be success
        The output should be blank
    End

    It 'transports the combined flag as one sudo argument in the actual Packer templates'
        transport() {
            local flags=NVIDIA_CUDA_PREBAKE,NVIDIA_PREBAKE_STAGE template command args baseline combined spaced checked=0 value
            while IFS= read -r template; do
                baseline=0 combined=0 spaced=0
                for value in NVIDIA_CUDA_PREBAKE "$flags" 'NVIDIA_CUDA_PREBAKE NVIDIA_PREBAKE_STAGE'; do
                    # shellcheck disable=SC2016 # Packer's backticks are literal template syntax.
                    command=$(printf '%s\n' "$template" | sed "s/{{user \`feature_flags\`}}/${value}/g" |
                        sed 's/{{user `[^`]*`}}/test/g')
                    # Execute the real command template, replacing sudo with an argv recorder.
                    args=$(/bin/sh -c 'sudo() { printf "%s\n" "$@"; }; '"$command") || return
                    case "$value" in
                        NVIDIA_CUDA_PREBAKE) baseline=$(printf '%s\n' "$args" | wc -l) ;;
                        "$flags")
                            [ "$(printf '%s\n' "$args" | head -n1)" = "FEATURE_FLAGS=$flags" ] || return 1
                            combined=$(printf '%s\n' "$args" | wc -l) ;;
                        *) spaced=$(printf '%s\n' "$args" | wc -l) ;;
                    esac
                done
                [ "$baseline" -eq "$combined" ] && [ "$spaced" -eq "$((baseline + 1))" ] || return 1
                checked=$((checked + 1))
            done < <(jq -r '.provisioners[].inline[]? | select(startswith("sudo FEATURE_FLAGS="))' \
                vhdbuilder/packer/vhd-image-builder-base.json vhdbuilder/packer/vhd-image-builder-cvm.json)
            [ "$checked" -eq 6 ] || return 1
            echo '6 Packer sudo templates preserve the comma-separated flag argument'
        }
        When call transport
        The status should be success
        The output should equal '6 Packer sudo templates preserve the comma-separated flag argument'
    End
End
