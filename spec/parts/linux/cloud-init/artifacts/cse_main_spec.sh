#!/usr/bin/env shellspec

# Unit tests for cse_main.sh helper functions (via cse_helpers.sh)
# Tests the select_localdns_corefile() function for localdns corefile selection logic

Describe 'cse_main.sh corefile selection'
    CSE_HELPERS_PATH="parts/linux/cloud-init/artifacts/cse_helpers.sh"

    # Mock base64-encoded corefiles for testing
    COREFILE_WITH_HOSTS="aG9zdHMgL2V0Yy9sb2NhbGRucy9ob3N0cw=="  # "hosts /etc/localdns/hosts"
    COREFILE_NO_HOSTS="bm8gaG9zdHMgcGx1Z2lu"  # "no hosts plugin"

    setup() {
        # Source the helpers to get select_localdns_corefile function
        # shellcheck disable=SC1090
        . "${CSE_HELPERS_PATH}"

        # Create temp directory for test files
        TEST_DIR=$(mktemp -d)
        HOSTS_FILE="${TEST_DIR}/hosts"
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'select_localdns_corefile()'
        Context 'when hosts plugin is enabled (SHOULD_ENABLE_HOSTS_PLUGIN=true)'
            It 'returns corefile WITH hosts plugin when hosts file exists with valid IP mappings'
                # Create hosts file with valid IP mappings
                echo "10.0.0.1 mcr.microsoft.com" > "${HOSTS_FILE}"
                echo "192.168.1.1 login.microsoftonline.com" >> "${HOSTS_FILE}"

                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_WITH_HOSTS}"
                The status should be success
                The stderr should include "Hosts plugin is enabled"
                The stderr should include "checking ${HOSTS_FILE} for content"
                The stderr should include "using corefile with hosts plugin"
            End

            It 'returns corefile WITHOUT hosts plugin when hosts file exists but has no IP mappings'
                # Create empty hosts file
                touch "${HOSTS_FILE}"

                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "exists but has no IP mappings"
                The stderr should include "falling back to corefile without hosts plugin"
            End

            It 'returns corefile WITHOUT hosts plugin when hosts file exists with only comments'
                # Create hosts file with only comments (no valid IP mappings)
                echo "# This is a comment" > "${HOSTS_FILE}"
                echo "# Another comment line" >> "${HOSTS_FILE}"

                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "exists but has no IP mappings"
            End

            It 'returns corefile WITHOUT hosts plugin when hosts file does not exist'
                # Don't create hosts file
                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "does not exist"
                The stderr should include "falling back to corefile without hosts plugin"
            End

            It 'handles IPv6 addresses in hosts file'
                # Create hosts file with IPv6 addresses
                echo "2001:db8::1 mcr.microsoft.com" > "${HOSTS_FILE}"
                echo "fe80::1 login.microsoftonline.com" >> "${HOSTS_FILE}"

                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_WITH_HOSTS}"
                The status should be success
                The stderr should include "using corefile with hosts plugin"
            End
        End

        Context 'when hosts plugin is disabled'
            It 'returns corefile WITHOUT hosts plugin when SHOULD_ENABLE_HOSTS_PLUGIN=false'
                # Create hosts file with valid IP mappings (should be ignored)
                echo "10.0.0.1 mcr.microsoft.com" > "${HOSTS_FILE}"

                When call select_localdns_corefile "false" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "Hosts plugin is not enabled"
                The stderr should include "using corefile without hosts plugin"
            End

            It 'returns corefile WITHOUT hosts plugin when SHOULD_ENABLE_HOSTS_PLUGIN is empty'
                # Create hosts file with valid IP mappings (should be ignored)
                echo "10.0.0.1 mcr.microsoft.com" > "${HOSTS_FILE}"

                When call select_localdns_corefile "" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "Hosts plugin is not enabled"
            End

            It 'returns corefile WITHOUT hosts plugin when SHOULD_ENABLE_HOSTS_PLUGIN is any value other than "true"'
                # Create hosts file with valid IP mappings (should be ignored)
                echo "10.0.0.1 mcr.microsoft.com" > "${HOSTS_FILE}"

                When call select_localdns_corefile "yes" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "Hosts plugin is not enabled"
            End
        End

        Context 'unknown cloud scenario (no hosts file created by aks-hosts-setup.sh)'
            It 'returns corefile WITHOUT hosts plugin when hosts plugin enabled but file does not exist (unknown cloud)'
                # Simulate unknown cloud: SHOULD_ENABLE_HOSTS_PLUGIN=true but aks-hosts-setup.sh
                # exited before creating the file

                When call select_localdns_corefile "true" "${COREFILE_WITH_HOSTS}" "${COREFILE_NO_HOSTS}" "${HOSTS_FILE}"
                The output should equal "${COREFILE_NO_HOSTS}"
                The status should be success
                The stderr should include "does not exist"
                The stderr should include "falling back to corefile without hosts plugin"
            End
        End
    End
End
