#!/bin/bash

Describe 'mig-partition.sh'
    SCRIPT_PATH="./parts/linux/cloud-init/artifacts/mig-partition.sh"

    nvidia-smi() {
        echo "nvidia-smi $*"
    }
    export -f nvidia-smi

    Describe 'legacy GPU_INSTANCE_PROFILE'
        Parameters
            "MIG1g" "19,19,19,19,19,19,19"
            "MIG2g" "14,14,14"
            "MIG3g" "9,9"
            "MIG4g" "5"
            "MIG7g" "0"
        End

        Example "maps $1 to its uniform layout"
            When run env GPU_INSTANCE_PROFILE="$1" bash "$SCRIPT_PATH"

            The status should be success
            The output should equal "nvidia-smi mig -cgi $2
nvidia-smi mig -cci"
        End
    End

    It 'maps the ordered layout without sorting or expanding profiles'
        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG3g,MIG2g,MIG1g,MIG1g" bash "$SCRIPT_PATH"

        The status should be success
        The output should equal "nvidia-smi mig -cgi 9,14,19,19
nvidia-smi mig -cci"
    End

    It 'prioritizes the layout when the legacy scalar is also populated'
        When run env GPU_INSTANCE_PROFILE="MIG7g" NVIDIA_MIG_PROFILE_LAYOUT="MIG2g,MIG1g" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 14,19"
    End

    It 'does not use MIG strategy to calculate the layout'
        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG2g,MIG1g" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 14,19"
    End

    It 'rejects an invalid legacy profile'
        When run env GPU_INSTANCE_PROFILE="MIG6g" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile: MIG6g"
    End

    It 'rejects an invalid layout profile'
        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG1g,MIG6g" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile: MIG6g"
    End

    It 'rejects empty layout elements instead of repairing the plan'
        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG1g,,MIG2g" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile: "
    End

    It 'rejects whitespace around a layout element'
        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG1g, MIG2g" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile:  MIG2g"
    End

    It 'rejects missing layout inputs'
        When run bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "neither NVIDIA_MIG_PROFILE_LAYOUT nor GPU_INSTANCE_PROFILE is set"
    End

    It 'propagates failure from creating GPU instances without running -cci'
        nvidia-smi() {
            echo "nvidia-smi $*"
            [ "$2" != "-cgi" ]
        }
        export -f nvidia-smi

        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG2g" bash "$SCRIPT_PATH"

        The status should be failure
        The output should equal "nvidia-smi mig -cgi 14"
    End

    It 'propagates failure from creating compute instances'
        nvidia-smi() {
            echo "nvidia-smi $*"
            [ "$2" != "-cci" ]
        }
        export -f nvidia-smi

        When run env NVIDIA_MIG_PROFILE_LAYOUT="MIG2g" bash "$SCRIPT_PATH"

        The status should be failure
        The output should equal "nvidia-smi mig -cgi 14
nvidia-smi mig -cci"
    End
End
