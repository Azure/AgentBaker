#!/bin/bash

Describe 'cse_install_ubuntu.sh'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

    Describe 'cleanUpPrebakedGPUDriver'
        It 'is a no-op when the prebake marker is absent'
            GPU_DKMS_MARKER_FILE="/tmp/aks-gpu-marker-absent-$$"
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should equal ""
        End

        It 'deregisters the nvidia DKMS module and removes baked artifacts when the marker is present'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            # mock dkms so `command -v dkms` succeeds and `dkms status` returns the installed form
            dkms() {
                if [ "$1" = "status" ]; then
                    echo "nvidia/580.126.09, 6.8.0-1029-azure, x86_64: installed"
                else
                    echo "mock dkms $*"
                fi
            }
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            The output should include "mock dkms remove nvidia/580.126.09 --all"
            The output should include "mock rm -rf /var/lib/dkms/nvidia"
            The output should include "mock rm -rf /usr/bin/lib64"
            The output should include "mock ldconfig"
        End

        It 'parses the bare "nvidia/<ver>: added" dkms status form to a clean version'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkms() {
                if [ "$1" = "status" ]; then
                    echo "nvidia/570.86.15: added"
                else
                    echo "mock dkms $*"
                fi
            }
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "mock dkms remove nvidia/570.86.15 --all"
        End
    End
End
