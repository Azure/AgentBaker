#!/bin/bash

Describe 'NPD package selection'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh

    select_npd_packages() {
        local os="$1" release="$2" package
        while IFS= read -r package; do
            updatePackageVersions "${package}" "${os}" "${release}" ''
            printf '%s: %s\n' "$(jq -r .name <<< "${package}")" "${PACKAGE_VERSIONS[*]}"
        done < <(jq -c '.Packages[] | select(.name | startswith("node-problem-detector"))' parts/common/components.json)
    }

    It 'selects both packages for Ubuntu 24.04'
        When call select_npd_packages UBUNTU 24.04
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25-ubuntu24.04u16'
        The output should include 'node-problem-detector-aks-config: 1.0.0-ubuntu24.04u1'
    End

    It 'leaves unpublished Ubuntu 26.04 packages empty'
        When call select_npd_packages UBUNTU 26.04
        The status should be success
        The output should not include 'ubuntu26.04u'
    End

    It 'leaves Azure Linux extension-managed'
        When call select_npd_packages AZURELINUX 3.0
        The status should be success
        The output should not include 'ubuntu'
    End
End
