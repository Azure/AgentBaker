#!/bin/bash

# this spec is meant to ensure that the behavior of helper functions that are used in long running operations keeps returning the expected exit codes

Describe 'long running cse helper functions'
    # unsetting this to test the behavior of check_cse_timeout
    cse_retry_helpers_precheck() {
        unset CSE_STARTTIME_FORMATTED
        unset CSE_STARTTIME_SECONDS
    }
    BeforeEach cse_retry_helpers_precheck

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
            "retrycmd_get_tarball_from_registry_with_oras"   1 3 1 "/tmp/nonexistent.tar" "dummy.registry/binary:v1"
            "systemctl_restart" 1 1 1 1 "nonexistent.service"
            "systemctl_stop"    1 1 1 1 "nonexistent.service"
            "systemctl_disable" 1 1 1 1 "nonexistent.service"
            "_systemctl_retry_svc_operation" 1 1 1 1 "nonexistent.service" "restart"
            "sysctl_reload" 1 1 1 1
            "retrycmd_get_aad_access_token" $ERR_ORAS_IMDS_TIMEOUT 1 1 "http://nonexistent.local/token"
            "retrycmd_get_refresh_token_for_oras" $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry" "tenant-id" "fake-token"
            "retrycmd_curl_file"    1 1 1 1 "/tmp/nonexistent" "https://dummy.url/file"
            "retrycmd_can_oras_ls_acr_anonymously"  $ERR_ORAS_PULL_NETWORK_TIMEOUT 1 1 "dummy.registry"
            "retrycmd_cp_oci_layout_with_oras"  $ERR_PULL_POD_INFRA_CONTAINER_IMAGE 1 1 "/tmp/nonexistent" "tag" "dummy.registry/binary:v1"
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
        Describe '_systemctl_retry_svc_operation logging'

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
                When call _systemctl_retry_svc_operation 2 1 1 "nonexistent.service" "restart" "true"
                The status should eq 1
                The stdout should include "mock systemctl call"
                The stdout should include "mock journalctl call"
            End
            It "won't check systemctl status and journalctl if operation failed and shouldLogRetryInfo is false"
                When call _systemctl_retry_svc_operation 2 1 1 "nonexistent.service" "restart" "false"
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
                The stdout should be defined
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

        Describe 'file curl'
            Describe 'retrycmd_get_tarball'
                It "get_tarball returns 1 if tar curl fails and retries are exhausted"
                    timeout() {
                        echo "curl mock failure"
                        return 1
                    }
                    When call retrycmd_get_tarball 2 1 "/tmp/test_tarball/test_tarball.tar.gz" "https://dummy.url/file.tar"
                    The status should eq 1
                    The stdout should include "2 file curl retries"
                    The stdout should include "curl mock failure"
                End
                It "get_tarball returns 0 if curl tar succeeds"
                    mkdir -p /tmp/test_tarball
                    echo "test content" > /tmp/test_tarball/testfile
                    tar -czf /tmp/test_tarball/test_tarball.tar.gz -C /tmp/test_tarball testfile
                    When call retrycmd_get_tarball 1 1 "/tmp/test_tarball/test_tarball.tar.gz" "https://dummy.url/file.tar"
                    rm -r /tmp/test_tarball
                    The status should eq 0
                    The stdout should include "1 file curl retries"
                End
                It "get_tarball returns 2 if global cse timeout is reached"
                    CSE_STARTTIME_FORMATTED=$(date -d "-781 seconds" +"%F %T.%3N")
                    CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                    mkdir -p /tmp/test_tarball
                    When call retrycmd_get_tarball 2 1 "/tmp/test_tarball/test_tarball.tar.gz" "https://dummy.url/file.tar"
                    rm -r /tmp/test_tarball
                    The status should eq 2
                    The stdout should include "2 file curl retries"
                    The stderr should include "CSE timeout approaching, exiting early"
                End
            End
            Describe 'retrycmd_curl_file'
                It "curl_file returns 1 if curl fails and retries are exhausted"
                    timeout() {
                        echo "curl mock failure"
                        return 1
                    }
                    When call retrycmd_curl_file 2 1 1 "/tmp/testFile" "https://dummy.url/file"
                    The status should eq 1
                    The stdout should include "2 file curl retries"
                    The stdout should include "curl mock failure"
                End
                It "curl_file returns 0 if curl succeeds"
                    touch /tmp/testFile
                    When call retrycmd_curl_file 1 1 1 "/tmp/testFile" "https://dummy.url/file"
                    rm /tmp/testFile
                    The status should eq 0
                    The stdout should eq "1 file curl retries"
                End
                It "curl_file returns 2 if global cse timeout is reached"
                    CSE_STARTTIME_FORMATTED=$(date -d "-781 seconds" +"%F %T.%3N")
                    CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                    When call retrycmd_curl_file 2 1 1 "/tmp/testFile" "https://dummy.url/file"
                    The status should eq 2
                    The stdout should include "2 file curl retries"
                    The stderr should include "CSE timeout approaching, exiting early"
                End
            End
            Describe 'retry_file_curl_internal'
                It "returns 1 if checksToRun fail and retries are exhausted"
                    timeout() {
                        echo "curl mock timeout" >> $CURL_OUTPUT
                        return 124
                    }
                    When call _retry_file_curl_internal 2 1 1 "/tmp/nonexistent" "https://dummy.url/file" "return 2"
                    The status should eq 1
                    The stdout should include "2 file curl retries"
                End
                It "returns 0 if checksToRun succeed"
                    When call _retry_file_curl_internal 1 1 1 "/tmp/nonexistent" "https://dummy.url/file" "return 0 && echo working"
                    The status should eq 0
                    The stdout should eq "1 file curl retries"
                End
                It "returns 0 if checksToRun is unset"
                    # checksToRun arg is unset
                    When call _retry_file_curl_internal 1 1 1 "/tmp/nonexistent" "https://dummy.url/file"
                    The status should eq 0
                    The stdout should eq "1 file curl retries"
                End
                It "returns 2 if checksToRun fail and global cse timeout is reached"
                    CSE_STARTTIME_FORMATTED=$(date -d "-781 seconds" +"%F %T.%3N")
                    CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                    When call _retry_file_curl_internal 2 1 1 "/tmp/nonexistent" "https://dummy.url/file" "return 3"
                    The status should eq 2
                    The stdout should be defined
                    The stderr should include "Error: CSE has been running for"
                    The stderr should include "CSE timeout approaching, exiting early."
                End
                It "prints curl output if curl operation times out"
                    CSE_STARTTIME_FORMATTED=$(date +"%F %T.%3N")
                    CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                    timeout() {
                        echo "curl mock timeout" >> $CURL_OUTPUT
                        return 124
                    }
                    When call _retry_file_curl_internal 2 1 1 "/tmp/nonexistent" "https://dummy.url/file" "return 2"
                    The status should eq 1
                    The stdout should include "curl mock timeout"
                End
            End
        End

        Describe 'retrycmd_get_tarball_from_registry_with_oras'
            It "calls retrycmd_pull_from_registry_with_oras when tarball exists but tar validation fails (returns 1)"
                # Create a temporary directory and invalid tarball
                mkdir -p /tmp/test_oras_tarball
                echo "invalid tarball content" > /tmp/test_oras_tarball/test.tar

                # Mock retrycmd_pull_from_registry_with_oras to track if it's called
                retrycmd_pull_from_registry_with_oras() {
                    echo "retrycmd_pull_from_registry_with_oras called with: $@"
                    return 1
                }

                # When tar -tzf returns 1 (failure/invalid tarball),
                # retrycmd_pull_from_registry_with_oras should be called to re-download
                When call retrycmd_get_tarball_from_registry_with_oras 2 1 "/tmp/test_oras_tarball/test.tar" "dummy.registry/binary:v1"

                The status should eq 1
                The stdout should include "retrycmd_pull_from_registry_with_oras called with: 2 1 /tmp/test_oras_tarball dummy.registry/binary:v1"

                # Cleanup after assertions
                rm -rf /tmp/test_oras_tarball
            End

            It "calls retrycmd_pull_from_registry_with_oras when tarball does not exist"
                # Mock retrycmd_pull_from_registry_with_oras to track if it's called
                retrycmd_pull_from_registry_with_oras() {
                    echo "retrycmd_pull_from_registry_with_oras called with: $@"
                    return 1
                }

                When call retrycmd_get_tarball_from_registry_with_oras 2 1 "/tmp/nonexistent_oras_tarball/test.tar" "dummy.registry/binary:v1"

                The status should eq 1
                The stdout should include "retrycmd_pull_from_registry_with_oras called with: 2 1 /tmp/nonexistent_oras_tarball dummy.registry/binary:v1"
            End

            It "skips download when tarball exists and is valid"
                # Create a valid tarball
                mkdir -p /tmp/test_valid_oras_tarball
                echo "test content" > /tmp/test_valid_oras_tarball/testfile
                tar -czf /tmp/test_valid_oras_tarball/valid.tar.gz -C /tmp/test_valid_oras_tarball testfile

                # Mock retrycmd_pull_from_registry_with_oras - should NOT be called
                retrycmd_pull_from_registry_with_oras() {
                    echo "retrycmd_pull_from_registry_with_oras should not be called"
                    return 1
                }

                When call retrycmd_get_tarball_from_registry_with_oras 2 1 "/tmp/test_valid_oras_tarball/valid.tar.gz" "dummy.registry/binary:v1"

                The status should eq 0
                The stdout should not include "retrycmd_pull_from_registry_with_oras"

                # Cleanup after assertions
                rm -rf /tmp/test_valid_oras_tarball
            End
        End


        Describe 'retrycmd_pull_from_registry_with_oras'
            It "passes exact flags correctly to oras command"
                # Test that the function handles extra arguments and passes them to oras
                # We can't easily mock oras in shellspec context, but we can verify the command construction
                # by checking that the function attempts to call oras with the expected argument check
                When call retrycmd_pull_from_registry_with_oras 2 1 "/tmp/test_dir" "dummy.registry/binary:v1"

                # The function should fail due to network issues (expected behavior with dummy registry)
                The status should eq 1
                # Should show retry attempts
                The stdout should include "2 retries"
                # Covers failure case that no extra args were not parsed to ''
                The stdout should not include "requires exactly 1 argument but got"
            End

            It "passes extra flags correctly to oras command"
                # Test that the function handles extra arguments and passes them to oras
                # We can't easily mock oras in shellspec context, but we can verify the command construction
                # by checking that the function attempts to call oras with the expected argument check
                # which indicates the arguments were passed through correctly

                When call retrycmd_pull_from_registry_with_oras 2 1 "/tmp/test_dir" "dummy.registry/binary:v1" "--platform=test platform a b c d e"

                # The function should fail due to network issues (expected behavior with dummy registry)
                The status should eq 1
                # Should show retry attempts
                The stdout should include "2 retries"
                # Covers failure case that extra args were not wrongly split by spaces
                The stdout should not include "requires exactly 1 argument but got"
            End
        End

        Describe 'retrycmd_internal cse global timeout'
            It "returns 2 and times out when retrycmd_internal exceeds the CSE timeout"
                timeout() {
                    return 124
                }
                CSE_STARTTIME_FORMATTED=$(date -d "-781 seconds" +"%F %T.%3N")
                CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                When call _retrycmd_internal 2 1 5 "true" echo "Failing Command"
                The status should eq 2
                The stdout should eq ""
                The stderr should include "Error: CSE has been running for"
                The stderr should include "CSE timeout approaching, exiting early."
            End
            It "returns 0 and does not time out when retrycmd_internal is within the CSE timeout"
                timeout() {
                    return 124
                }
                CSE_STARTTIME_FORMATTED=$(date -d "-5 minutes" +"%F %T.%3N")
                CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                When call _retrycmd_internal 2 1 5 "true" echo "Failing Command"
                The status should eq 1
                The stdout should eq ""
                The stderr should include "Executed \"echo Failing Command\" 2 times; giving up (last exit status: 124)."
            End
        End
    End

    Describe 'check_cse_timeout'
        Describe 'when CSE_STARTTIME_SECONDS is incorrect'
            It 'returns 0 and prints error to stderr when CSE_STARTTIME_SECONDS is not set'
                When call check_cse_timeout
                The status should eq 0
                The stdout should include "Warning: CSE_STARTTIME_SECONDS environment variable is not set."
                The stderr should eq ""
            End
        End
        Describe 'when CSE_STARTTIME_SECONDS is set'
            It 'returns 0 and prints no output when CSE_STARTTIME_SECONDS is less than the timeout'
                CSE_STARTTIME_FORMATTED=$(date -d "-5 minutes" +"%F %T.%3N")
                CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                When call check_cse_timeout
                The status should eq 0
                The stderr should eq ""
                The stdout should eq ""
            End
            It 'returns 1 and prints error to stderr when CSE_STARTTIME_SECONDS is past the timeout'
                CSE_STARTTIME_FORMATTED=$(date -d "-781 seconds" +"%F %T.%3N")
                CSE_STARTTIME_SECONDS=$(date -d "$CSE_STARTTIME_FORMATTED" +%s)
                When call check_cse_timeout
                The status should eq 1
                The stderr should include "Error: CSE has been running for 781 seconds"
                The stdout should eq ""
            End
        End
    End
End
