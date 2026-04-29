#!/usr/bin/env shellspec

# Unit tests for select_localdns_corefile() function
# select_localdns_corefile() reads globals from the environment:
#   LOCALDNS_COREFILE_BASE         — base corefile (no experimental plugins)
#   LOCALDNS_COREFILE_EXPERIMENTAL   — corefile with experimental plugins (e.g. hosts)
#   SHOULD_ENABLE_HOSTS_PLUGIN       — whether hosts plugin is enabled
# Selection is purely based on the SHOULD_ENABLE_HOSTS_PLUGIN feature flag.
# The EXPERIMENTAL corefile uses `reload 5s` so CoreDNS hot-reloads the hosts file
# when it gets populated — no polling/waiting is done in this function.

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
        # Use a temp file for hosts file path so tests don't need root
        _TEST_HOSTS_FILE="$(mktemp)"
        export LOCALDNS_HOSTS_FILE="${_TEST_HOSTS_FILE}"
    }

    cleanup() {
        unset LOCALDNS_COREFILE_BASE
        unset LOCALDNS_COREFILE_EXPERIMENTAL
        unset SHOULD_ENABLE_HOSTS_PLUGIN
        rm -f "${_TEST_HOSTS_FILE:-}" 2>/dev/null || true
        unset LOCALDNS_HOSTS_FILE
        unset _TEST_HOSTS_FILE
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Context 'when both corefile variants are available and hosts plugin is enabled'
        It 'returns EXPERIMENTAL when hosts file exists'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            # _TEST_HOSTS_FILE already exists from setup (mktemp)

            When call select_localdns_corefile
            The output should equal "${COREFILE_WITH_HOSTS}"
            The status should be success
            The stderr should include "using corefile with hosts plugin"
            The stderr should include "reload will pick up hosts file when populated"
        End

        It 'falls back to BASE when hosts file is missing (enableAKSLocalDNSHostsSetup bailed early)'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            # Remove the hosts file to simulate enableAKSLocalDNSHostsSetup bailing early
            # (e.g. empty LOCALDNS_CRITICAL_FQDNS) without creating the hosts file
            rm -f "${_TEST_HOSTS_FILE}"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "falling back to BASE corefile"
        End
    End

    Context 'when both corefile variants are available and hosts plugin is disabled'
        It 'returns BASE when SHOULD_ENABLE_HOSTS_PLUGIN=false'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="false"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns BASE when SHOULD_ENABLE_HOSTS_PLUGIN is empty'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN=""

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns BASE when SHOULD_ENABLE_HOSTS_PLUGIN is any value other than "true"'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_EXPERIMENTAL="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="yes"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End
    End

    Context 'when only BASE is available (no dynamic selection)'
        It 'returns BASE when EXPERIMENTAL is not set'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            unset LOCALDNS_COREFILE_EXPERIMENTAL

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Using LOCALDNS_COREFILE_BASE (no dynamic selection)"
        End
    End

    Context 'when no corefile variants are available'
        It 'returns failure when neither variant is set'
            unset LOCALDNS_COREFILE_BASE
            unset LOCALDNS_COREFILE_EXPERIMENTAL

            When call select_localdns_corefile
            The output should equal ""
            The status should be failure
            The stderr should include "No corefile variants available in environment"
        End
    End
End
