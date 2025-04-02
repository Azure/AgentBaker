#!/bin/bash

# Mock the lsb_release function for testing
lsb_release() {
    echo "mock lsb_release"
}

# # Verify if the localdns corefile exists and is not empty
# verify_localdns_corefile() {
#     if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
#         printf "Localdns corefile either does not exist or is empty at %s.\n" "${LOCALDNS_CORE_FILE}"
#         exit $ERR_LOCALDNS_COREFILE_NOTFOUND
#     fi
# }

# Shellspec Test for cse_helpers.sh
Describe 'cse_helpers.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Include "./parts/linux/cloud-init/artifacts/localdns.sh"

    # Describe the 'shouldEnableLocaldns' function
    Describe 'shouldEnableLocaldns'
        # Setup and cleanup functions
        setup() {
            TMP_DIR=$(mktemp -d)
            LOCALDNS_CORE_FILE="$TMP_DIR/localdns.corefile"
        }
        cleanup() {
            rm -rf "$TMP_DIR"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should return 217 if LOCALDNS_CORE_FILE does not exist'
            rm -f "$LOCALDNS_CORE_FILE"
            When run shouldEnableLocaldns
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE"
        End

        It 'should return 217 if LOCALDNS_CORE_FILE is empty'
            > "$LOCALDNS_CORE_FILE"
            When run shouldEnableLocaldns
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE"
        End

        It 'should return 0 if LOCALDNS_CORE_FILE exists and is not empty'
            echo 'localdns corefile' > "$LOCALDNS_CORE_FILE"
            When run shouldEnableLocaldns
            The status should be success
            The stdout should include "Localdns should be enabled."
        End

        # Verify function for checking the corefile directly
        It 'should return success when LOCALDNS_CORE_FILE exists and is not empty'
            echo 'localdns corefile' > "$LOCALDNS_CORE_FILE"
            When run verify_localdns_corefile
            The status should be success
        End

        It 'should return failure if LOCALDNS_CORE_FILE is missing'
            rm -f "$LOCALDNS_CORE_FILE"
            When run verify_localdns_corefile
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE"
        End

        It 'should return failure if LOCALDNS_CORE_FILE is empty'
            > "$LOCALDNS_CORE_FILE"
            When run verify_localdns_corefile
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE"
        End
    End
End
