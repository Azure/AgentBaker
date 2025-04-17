#!/bin/bash

# this spec is meant to ensure that the behavior of helper functions that are used in long running operations keeps returning the expected exit codes
Describe 'timeout behavior of helper functions'
    Include parts/linux/cloud-init/artifacts/cse_helpers.sh
    
    # mock timeout to always return 124 - default exit code for timeout command
    timeout() {
        return 124
    }

    Parameters
        "retrycmd_if_failure"           1 1 1 1 sleep 5
        "retrycmd_if_failure_no_stats"  1 1 1 1 sleep 5
        "retrycmd_if_failure_silent"    1 1 1 1 sleep 5
        "retrycmd_get_tarball"          1 1 1 "/tmp/nonexistent.tar" "https://dummy.url/file.tar"
        "retrycmd_get_binary_from_registry_with_oras" 1 3 1 "/tmp/nonexistent" "dummy.registry/binary:v1"
        "systemctl_restart"             1 1 1 1 "nonexistent.service"
        "systemctl_stop"                1 1 1 1 "nonexistent.service"
        "systemctl_disable"             1 1 1 1 "nonexistent.service"
        "sysctl_reload"                 1 1 1 1
        "retrycmd_if_failure_silent"    1 1 1 1 sleep 5
        "retrycmd_get_access_token_for_oras" $ERR_ORAS_IMDS_TIMEOUT 1 1 "http://nonexistent.local/token"
        "retrycmd_get_refresh_token_for_oras" $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry" "tenant-id" "fake-token"
        "retrycmd_curl_file"            1 1 1 1 "/tmp/nonexistent" "https://dummy.url/file"
        "retrycmd_can_oras_ls_acr"      $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry"
    End

    It "returns 1 and times out when calling ($1)"
        func=$1
        err_code=$2
        shift 2
        When call "$func" "$@"
        The status should eq $err_code
        # we don't really care about stdout/stderr here, adding this to suppress warnings
        The stdout should be defined
        The stderr should be defined
    End
End
