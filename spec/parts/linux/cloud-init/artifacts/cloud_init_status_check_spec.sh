#!/bin/bash

Describe 'cloudInitStatusCheck'
    Include "./parts/linux/cloud-init/artifacts/cloud-init-status-check.sh"
    cleanUpLoggingDirs() {
        # Clean up the logging directories to avoid conflicts in tests
        rm -rf /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
        rm -f /tmp/var/test-log.txt
        touch /tmp/var/test-log.txt
    }
    setEventsDir() {
        export EVENTS_LOGGING_DIR=/tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
    }   
    unsetEventsDir() {
        unset EVENTS_LOGGING_DIR
    }
    testLongCloudInitStatus="status: test status extended_status: extended_test_status boot_status_code: test_boot_status_code detail: test_detail errors: [] recoverable_errors: {}"
    mkdir -p /tmp/var
    Describe 'cloud-init failure error handling'

        BeforeAll setEventsDir
        AfterAll unsetEventsDir
        BeforeEach cleanUpLoggingDirs
        It "should correctly handle cloud-init returning code 1 and log the error"
            cloud-init() {
                echo "$testLongCloudInitStatus"
                return 1
            } 
            When call handleCloudInitStatus "/tmp/var/test-log.txt"
            The status should be failure
            The status should eq 223
            The contents of file /tmp/var/test-log.txt should include "ERROR: cloud-init finished with fatal error (exit code 1)"
            eventsFilePath=$(ls -t /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*.json | head -n 1)
            The contents of file ${eventsFilePath} should include "ERROR: cloud-init finished with fatal error (exit code 1)"
            The contents of file ${eventsFilePath} should include "recoverable_errors"
        End
        It "should correctly handle cloud-init returning code 2 and log extra information"
            cloud-init() {
                echo "$testLongCloudInitStatus"
                return 2
            } 
            When call handleCloudInitStatus "/tmp/var/test-log.txt"
            The status should be success
            The status should eq 0 
            The contents of file /tmp/var/test-log.txt should include "WARNING: cloud-init finished with recoverable errors (exit code 2)"
            eventsFilePath=$(ls -t /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*.json | head -n 1)
            The contents of file ${eventsFilePath} should include "WARNING: cloud-init finished with recoverable errors (exit code 2)"
            The contents of file ${eventsFilePath} should include "recoverable_errors:"
        End
        It "should correctly handle cloud-init returning code 0 and log success"
            cloud-init() {
                echo "$testLongCloudInitStatus"
                return 0
            } 
            When call handleCloudInitStatus "/tmp/var/test-log.txt"
            The status should be success
            The status should eq 0 
            The contents of file /tmp/var/test-log.txt should include "cloud-init succeeded"
            # Check that the events directory is empty (no JSON files) - we don't log events for cloud-init status == 0
            eventsFileCount=$(ls /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*.json 2>/dev/null | wc -l)
            The variable eventsFileCount should eq 0
        End
        It "should correctly handle cloud-init returning an unexpected code and log information"
            cloud-init() {
                echo "$testLongCloudInitStatus"
                # return an unexpected code, e.g. 123
                return 123
            } 
            When call handleCloudInitStatus "/tmp/var/test-log.txt"
            The status should be success
            The status should eq 0 
            The contents of file /tmp/var/test-log.txt should include "WARNING: cloud-init exited with unexpected code: 123"
            eventsFilePath=$(ls -t /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*.json | head -n 1)
            The contents of file ${eventsFilePath} should include "WARNING: cloud-init exited with unexpected code: 123"
            The contents of file ${eventsFilePath} should include "recoverable_errors:"
        End
        It "should truncate JSON output when cloudInitLongStatus is too long"
            # Create a very long cloud-init status that exceeds the truncation threshold
            longStatus="status: test status extended_status: extended_test_status boot_status_code: test_boot_status_code detail: test_detail errors: [] recoverable_errors: {}"
            padding=$(printf 'A%.0s' $(seq 1 4000))
            veryLongStatus="${longStatus}${padding}"
            
            cloud-init() {
                if [[ "$1" == "status" && "$2" == "--long" && "$3" == "--format" && "$4" == "json" ]]; then
                    echo "$veryLongStatus"
                else
                    echo "$testLongCloudInitStatus"
                    return 1
                fi
            }
            When call handleCloudInitStatus "/tmp/var/test-log.txt"
            The status should be failure
            The status should eq 223
            # Verify that truncated suffix is added
            eventsFilePath=$(ls -t /tmp/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/*.json | head -n 1)
            The contents of file ${eventsFilePath} should include "...TRUNCATED"
            # Verify the original long status is logged in the provision output
            The contents of file /tmp/var/test-log.txt should include "Cloud-init detailed status:"
            # Verify that message is now shorter
            messageLength=$(jq -r '.Message | length' < "${eventsFilePath}")
            originalLength=${#veryLongStatus}
            messageLengthCheck=$((messageLength < originalLength ? 0 : 1))
            The variable messageLengthCheck should eq 0
        End 
    End
End