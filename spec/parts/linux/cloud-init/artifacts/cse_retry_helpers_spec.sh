#!/bin/bash

# this spec is meant to ensure that the behavior of helper functions that are used in long running operations keeps returning the expected exit codes
Describe 'long running cse helper functions'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    Describe 'timeout behavior of helper functions'
        # mock timeout to always return 124 - default exit code for timeout command
        timeout() {
            return 124
        }

        Parameters
            "retrycmd_if_failure" 1 1 1 1 sleep 5
            "retrycmd_silent"   1 1 1 1 sleep 5
            "_retrycmd_internal"    1 1 1 1 false sleep 
            "retrycmd_get_tarball"  1 1 1 "/tmp/nonexistent.tar" "https://dummy.url/file.tar"
            "retrycmd_get_binary_from_registry_with_oras"   1 3 1 "/tmp/nonexistent" "dummy.registry/binary:v1"
            "systemctl_restart" 1 1 1 1 "nonexistent.service"
            "systemctl_stop"    1 1 1 1 "nonexistent.service"
            "systemctl_disable" 1 1 1 1 "nonexistent.service"
            systemctl_retry_svc_operation 1 1 1 1 "nonexistent.service" "restart" 
            "sysctl_reload" 1 1 1 1
            "retrycmd_get_access_token_for_oras" $ERR_ORAS_IMDS_TIMEOUT 1 1 "http://nonexistent.local/token"
            "retrycmd_get_refresh_token_for_oras" $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry" "tenant-id" "fake-token"
            "retrycmd_curl_file"    1 1 1 1 "/tmp/nonexistent" "https://dummy.url/file"
            "retrycmd_can_oras_ls_acr"  $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry"
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

    Describe 'systemctl svc retry'
        Describe 'systemctl_retry_svc_operation logging'
        
            timeout() {
                return 124
            }
            systemctl() {
                echo "mock systemctl call"
            }
            journalctl() {
                echo "mock journalctl call"
            }
            
            It "checks systemctl status and journalctl if operation failed and shouldLogRetryInfo is true"
                When call systemctl_retry_svc_operation 2 1 1 "nonexistent.service" "restart" "true"
                The status should eq 1
                The stdout should include "mock systemctl call"
                The stdout should include "mock journalctl call"
            End
            It "won't check systemctl status and journalctl if operation failed and shouldLogRetryInfo is false"
                When call systemctl_retry_svc_operation 2 1 1 "nonexistent.service" "restart" "false"
                The status should eq 1
                The stdout should not include "mock systemctl call"
                The stdout should not include "mock journalctl call"
            End
        End
    End

    Describe 'retrycmd_internal'
        Describe 'retrycmd_internal logging'
            It "logs output when shouldLog is true and command succeeds"
                When call _retrycmd_internal 3 1 5 "true" echo "Success Command"
                The status should eq 0
                The stdout should include "Executed \"echo Success Command\" 1 times."
            End

            It "logs output when shouldLog is true and command fails"
                timeout() {
                    return 124
                }
                When call _retrycmd_internal 2 1 5 "true" echo "Failing Command"
                The status should eq 1
                The stderr should include "Executed \"echo Failing Command\" 2 times; giving up (last exit status: 124)."
            End

            It "does not log output when shouldLog is false and command succeeds"
                When call _retrycmd_internal 3 1 5 "false" echo "Success Command"
                The status should eq 0
                # we only expect the output from the test command
                The stdout should eq "Success Command"
                The stderr should eq "" 
            End

            It "does not log output when shouldLog is false and command fails"
                # Mock timeout to always fail for this test
                timeout() {
                    return 124
                }
                When call _retrycmd_internal 2 1 5 "false" echo "Failing Command"
                The status should eq 1
                # Ensure stdout/stderr are empty
                The stdout should eq ""
                The stderr should eq ""
            End
        End
    End
End
