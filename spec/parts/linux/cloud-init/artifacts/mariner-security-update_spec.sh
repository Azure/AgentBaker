#!/bin/bash

Describe 'Azure Linux securityPatch handler'
    Include ./parts/linux/cloud-init/artifacts/mariner/security-update.sh

    It 'compares the selected profile without requiring dispatcher state'
        node='{"metadata":{"labels":{"kubernetes.azure.com/agentpool":"ap1"}}}'
        desired='{"agentPools":{"ap1":{"goldenTimestamp":"20261007T000000Z"},"ap2":{"goldenTimestamp":"20261008T000000Z"}}}'
        current='{"agentPools":{"ap1":{"goldenTimestamp":"20261007T000000Z","kubeletVersion":""}}}'
        When call securityPatchIsCurrent "${desired}" "${current}" "${node}"
        The status should be success
    End

    It 'treats a kubelet version change as new security patch work'
        node='{"metadata":{"labels":{"kubernetes.azure.com/agentpool":"ap1"}}}'
        desired='{"agentPools":{"ap1":{"goldenTimestamp":"20261007T000000Z","kubeletVersion":"1.35.8"}}}'
        current='{"agentPools":{"ap1":{"goldenTimestamp":"20261007T000000Z","kubeletVersion":"1.35.7"}}}'
        When call securityPatchIsCurrent "${desired}" "${current}" "${node}"
        The status should be failure
    End
End
