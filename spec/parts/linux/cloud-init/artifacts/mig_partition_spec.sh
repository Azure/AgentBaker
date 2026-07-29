#!/bin/bash

Describe 'mig-partition.sh'
    SCRIPT_PATH='./parts/linux/cloud-init/artifacts/mig-partition.sh'

    nvidia-smi() {
        echo "nvidia-smi $*"
    }
    export -f nvidia-smi

    Describe 'default seven-slice capacity'
        Parameters
            'MIG1g' '19,19,19,19,19,19,19'
            'MIG2g' '14,14,14'
            'MIG3g' '9,9'
            'MIG4g' '5'
            'MIG7g' '0'
        End

        Example "maps $1 to its existing layout"
            When run bash "$SCRIPT_PATH" "$1"

            The status should be success
            The output should include "nvidia-smi mig -cgi $2"
            The output should include 'nvidia-smi mig -cci'
        End
    End

    Describe 'capacity-aware layouts'
        Parameters
            'MIG1g' '3' '19,19,19'
            'MIG1g' '4' '19,19,19,19'
            'MIG2g' '3' '14'
            'MIG2g' '4' '14,14'
            'MIG3g' '4' '9'
        End

        Example "uses floor division for $1 on a $2-slice VM"
            NVIDIA_MIG_TOTAL_SLICES="$2"
            export NVIDIA_MIG_TOTAL_SLICES

            When run bash "$SCRIPT_PATH" "$1"

            The status should be success
            The output should include "nvidia-smi mig -cgi $3"
            The output should include 'nvidia-smi mig -cci'
        End
    End

    It 'rejects an invalid profile before invoking nvidia-smi'
        NVIDIA_MIG_TOTAL_SLICES='7'
        export NVIDIA_MIG_TOTAL_SLICES

        When run bash "$SCRIPT_PATH" 'MIG6g'

        The status should be failure
        The error should include 'not a valid GPU instance profile: MIG6g'
        The output should not include 'nvidia-smi'
    End

    It 'rejects a profile wider than the VM capacity before invoking nvidia-smi'
        NVIDIA_MIG_TOTAL_SLICES='3'
        export NVIDIA_MIG_TOTAL_SLICES

        When run bash "$SCRIPT_PATH" 'MIG4g'

        The status should be failure
        The error should include 'GPU instance profile MIG4g requires more than 3 slices'
        The output should not include 'nvidia-smi'
    End

    It 'rejects zero capacity before invoking nvidia-smi'
        NVIDIA_MIG_TOTAL_SLICES='0'
        export NVIDIA_MIG_TOTAL_SLICES

        When run bash "$SCRIPT_PATH" 'MIG1g'

        The status should be failure
        The error should include 'total GPU instance slices must be a positive integer: 0'
        The output should not include 'nvidia-smi'
    End

    It 'rejects negative capacity before invoking nvidia-smi'
        NVIDIA_MIG_TOTAL_SLICES='-1'
        export NVIDIA_MIG_TOTAL_SLICES

        When run bash "$SCRIPT_PATH" 'MIG1g'

        The status should be failure
        The error should include 'total GPU instance slices must be a positive integer: -1'
        The output should not include 'nvidia-smi'
    End
End
