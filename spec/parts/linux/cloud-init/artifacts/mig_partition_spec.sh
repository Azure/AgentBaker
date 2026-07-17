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
            When run env GPU_INSTANCE_PROFILE="$1" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

            The status should be success
            The output should include "nvidia-smi mig -cgi $2"
            The output should include "nvidia-smi mig -cci"
        End
    End

    It 'uses the first plural profile for the Single strategy and trims whitespace'
        When run env NVIDIA_MIG_PROFILES="  MIG2g , MIG1g " NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 14,14,14"
        The output should include "nvidia-smi mig -cci"
    End

    It 'maps every supported mixed profile and ignores empty CSV elements'
        When run env NVIDIA_MIG_PROFILES="MIG1g,, MIG2g,MIG3g,MIG4g,MIG7g" NVIDIA_MIG_STRATEGY="Mixed" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 19,14,9,5,0"
        The output should include "nvidia-smi mig -cci"
    End

    It 'rejects an invalid legacy profile'
        When run env GPU_INSTANCE_PROFILE="MIG6g" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile: MIG6g"
    End

    It 'rejects an invalid mixed profile'
        When run env NVIDIA_MIG_PROFILES="MIG1g,MIG6g" NVIDIA_MIG_STRATEGY="Mixed" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid MIG profile: MIG6g"
    End

    It 'rejects an empty mixed profile list'
        When run env NVIDIA_MIG_PROFILES="," NVIDIA_MIG_STRATEGY="Mixed" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "MIG profiles cannot be empty"
    End

    It 'rejects both profile inputs being set'
        When run env GPU_INSTANCE_PROFILE="MIG2g" NVIDIA_MIG_PROFILES="MIG2g" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "GPU_INSTANCE_PROFILE and NVIDIA_MIG_PROFILES are mutually exclusive"
    End

    It 'rejects neither profile input being set'
        When run env NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "exactly one of GPU_INSTANCE_PROFILE or NVIDIA_MIG_PROFILES must be set"
    End

    It 'does not accept the legacy positional argument without an environment input'
        When run bash "$SCRIPT_PATH" MIG2g

        The status should be failure
        The error should include "exactly one of GPU_INSTANCE_PROFILE or NVIDIA_MIG_PROFILES must be set"
    End
End
