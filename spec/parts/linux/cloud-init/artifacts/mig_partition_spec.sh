#!/bin/bash

Describe 'mig-partition.sh'
    SCRIPT_PATH="./parts/linux/cloud-init/artifacts/mig-partition.sh"

    nvidia-smi() {
        echo "nvidia-smi $@"
    }
    export -f nvidia-smi

    It 'keeps the legacy full-layout mapping for a single profile argument'
        When run bash "$SCRIPT_PATH" MIG2g

        The status should be success
        The output should include "nvidia-smi mig -cgi 14,14,14"
        The output should include "nvidia-smi mig -cci"
    End

    It 'keeps the legacy environment variable mapping'
        When run env GPU_INSTANCE_PROFILE="MIG4g" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 5"
        The output should include "nvidia-smi mig -cci"
    End

    It 'keeps the full-layout mapping for a single MIG profile'
        When run env MIG_PROFILES="MIG2g" NVIDIA_MIG_STRATEGY="Single" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 14,14,14"
        The output should include "nvidia-smi mig -cci"
    End

    It 'maps mixed profile lists to a single create-gpu-instance layout'
        When run env MIG_PROFILES="MIG1g,MIG2g,MIG4g" NVIDIA_MIG_STRATEGY="Mixed" bash "$SCRIPT_PATH"

        The status should be success
        The output should include "nvidia-smi mig -cgi 19,14,5"
        The output should include "nvidia-smi mig -cci"
    End

    It 'rejects invalid mixed profile lists'
        When run env MIG_PROFILES="MIG1g,MIG6g" NVIDIA_MIG_STRATEGY="Mixed" bash "$SCRIPT_PATH"

        The status should be failure
        The error should include "not a valid GPU instance profile: MIG6g"
    End
End
