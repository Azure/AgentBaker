#!/usr/bin/env shellspec

# Unit tests for select_localdns_corefile() function
# select_localdns_corefile() reads globals from the environment:
#   LOCALDNS_COREFILE_ACTIVE         — base corefile (no experimental plugins)
#   LOCALDNS_COREFILE_EXPERIMENTAL   — corefile with experimental plugins (e.g. hosts)
#   SHOULD_ENABLE_HOSTS_PLUGIN       — whether hosts plugin is enabled
# It checks /etc/localdns/hosts for valid IP mappings to decide which variant to use.

Describe 'select_localdns_corefile()'
    LOCALDNS_PATH="parts/linux/cloud-init/artifacts/localdns.sh"

    # Mock base64-encoded corefiles for testing
    COREFILE_WITH_HOSTS="aG9zdHMgL2V0Yy9sb2NhbGRucy9ob3N0cw=="  # "hosts /etc/localdns/hosts"
    COREFILE_NO_HOSTS="bm8gaG9zdHMgcGx1Z2lu"  # "no hosts plugin"

    setup() {
        # Source localdns.sh to get select_localdns_corefile function
        # We set __SOURCED__=1 to only source the functions, not run main execution
        # shellcheck disable=SC1090
        __SOURCED__=1 . "${LOCALDNS_PATH}"

        # Create temp directory for test hosts file
        TEST_DIR=$(mktemp -d)
        HOSTS_FILE="${TEST_DIR}/hosts"
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
        unset LOCALDNS_COREFILE_ACTIVE
        unset LOCALDNS_COREFILE_EXPERIMENTAL
        unset SHOULD_ENABLE_HOSTS_PLUGIN
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Context 'when both corefile variants are available and hosts plugin is enabled'
        It 'returns EXPERIMENTAL when hosts file has valid IP mappings'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            # Create hosts file with valid IP mappings at the path the function checks
            mkdir -p /etc/localdns
            echo "10.0.0.1 mcr.microsoft.com" > /etc/localdns/hosts

            When call select_localdns_corefile
            The output should equal "${COREFILE_WITH_HOSTS}"
            The status should be success
            The stderr should include "Hosts file has IP mappings"
            The stderr should include "using corefile with hosts plugin"
        End

        It 'returns ACTIVE when hosts file exists but has no IP mappings'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            mkdir -p /etc/localdns
            echo "# comment only" > /etc/localdns/hosts

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "not ready yet, falling back to corefile without hosts plugin"
        End

        It 'returns ACTIVE when hosts file does not exist'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            rm -f /etc/localdns/hosts

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "not ready yet, falling back to corefile without hosts plugin"
        End

        It 'handles IPv6 addresses in hosts file'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            mkdir -p /etc/localdns
            echo "2001:db8::1 mcr.microsoft.com" > /etc/localdns/hosts

            When call select_localdns_corefile
            The output should equal "${COREFILE_WITH_HOSTS}"
            The status should be success
            The stderr should include "using corefile with hosts plugin"
        End
    End

    Context 'when both corefile variants are available and hosts plugin is disabled'
        It 'returns ACTIVE when SHOULD_ENABLE_HOSTS_PLUGIN=false'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="false"
            # Create hosts file with valid IP mappings (should be ignored)
            mkdir -p /etc/localdns
            echo "10.0.0.1 mcr.microsoft.com" > /etc/localdns/hosts

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns ACTIVE when SHOULD_ENABLE_HOSTS_PLUGIN is empty'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN=""

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns ACTIVE when SHOULD_ENABLE_HOSTS_PLUGIN is any value other than "true"'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="yes"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End
    End

    Context 'when only ACTIVE is available (no dynamic selection)'
        It 'returns ACTIVE when EXPERIMENTAL is not set'
            LOCALDNS_COREFILE_ACTIVE="${COREFILE_NO_HOSTS}"
            unset LOCALDNS_COREFILE_EXPERIMENTAL

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Using LOCALDNS_COREFILE_ACTIVE (no dynamic selection)"
        End
    End

    Context 'when no corefile variants are available'
        It 'returns empty string when neither variant is set'
            unset LOCALDNS_COREFILE_ACTIVE
            unset LOCALDNS_COREFILE_EXPERIMENTAL

            When call select_localdns_corefile
            The output should equal ""
            The status should be success
            The stderr should include "No corefile variants available in environment"
        End
    End
End
