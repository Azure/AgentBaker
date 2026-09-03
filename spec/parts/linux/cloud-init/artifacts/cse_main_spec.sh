#!/usr/bin/env shellspec

# Unit tests for select_localdns_corefile() function
# select_localdns_corefile() reads globals from the environment:
#   LOCALDNS_COREFILE_BASE         — base corefile (no hosts plugin)
#   LOCALDNS_COREFILE_WITH_HOSTS   — corefile with hosts plugin
#   SHOULD_ENABLE_HOSTS_PLUGIN       — whether hosts plugin is enabled
# Selection is purely based on the SHOULD_ENABLE_HOSTS_PLUGIN feature flag.
# The WITH_HOSTS corefile uses `reload 5s` so CoreDNS hot-reloads the hosts file
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
        unset LOCALDNS_COREFILE_WITH_HOSTS
        unset SHOULD_ENABLE_HOSTS_PLUGIN
        rm -f "${_TEST_HOSTS_FILE:-}" 2>/dev/null || true
        unset LOCALDNS_HOSTS_FILE
        unset _TEST_HOSTS_FILE
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Context 'when both corefile variants are available and hosts plugin is enabled'
        It 'returns WITH_HOSTS when hosts file exists'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_WITH_HOSTS="${COREFILE_WITH_HOSTS}"
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
            LOCALDNS_COREFILE_WITH_HOSTS="${COREFILE_WITH_HOSTS}"
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
            LOCALDNS_COREFILE_WITH_HOSTS="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="false"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns BASE when SHOULD_ENABLE_HOSTS_PLUGIN is empty'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_WITH_HOSTS="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN=""

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End

        It 'returns BASE when SHOULD_ENABLE_HOSTS_PLUGIN is any value other than "true"'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            LOCALDNS_COREFILE_WITH_HOSTS="${COREFILE_WITH_HOSTS}"
            SHOULD_ENABLE_HOSTS_PLUGIN="yes"

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Hosts plugin is not enabled"
        End
    End

    Context 'when only BASE is available (no dynamic selection)'
        It 'returns BASE when WITH_HOSTS is not set'
            LOCALDNS_COREFILE_BASE="${COREFILE_NO_HOSTS}"
            unset LOCALDNS_COREFILE_WITH_HOSTS

            When call select_localdns_corefile
            The output should equal "${COREFILE_NO_HOSTS}"
            The status should be success
            The stderr should include "Using LOCALDNS_COREFILE_BASE (no dynamic selection)"
        End
    End

    Context 'when no corefile variants are available'
        It 'returns failure when neither variant is set'
            unset LOCALDNS_COREFILE_BASE
            unset LOCALDNS_COREFILE_WITH_HOSTS

            When call select_localdns_corefile
            The output should equal ""
            The status should be failure
            The stderr should include "No corefile variants available in environment"
        End
    End
End

Describe 'proxy environment exports'
    setup() {
        unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
        HTTP_PROXY_URLS=""
        HTTPS_PROXY_URLS=""
        NO_PROXY_URLS=""
        proxy_exports="$(sed -n '/^# configureEtcEnvironment persists/,/^# Disable a single kernel module/p' parts/linux/cloud-init/artifacts/cse_main.sh)"
    }

    cleanup() {
        unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
        unset HTTP_PROXY_URLS HTTPS_PROXY_URLS NO_PROXY_URLS
    }

    exported_proxy_environment() {
        eval "${proxy_exports}"
        /bin/bash -c 'printf "%s\n" "${HTTP_PROXY-<unset>}" "${http_proxy-<unset>}" "${HTTPS_PROXY-<unset>}" "${https_proxy-<unset>}" "${NO_PROXY-<unset>}" "${no_proxy-<unset>}"'
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'exports HTTP proxy values only'
        HTTP_PROXY_URLS="http://proxy.example.com:8080"

        When call exported_proxy_environment
        The line 1 of output should equal "http://proxy.example.com:8080"
        The line 2 of output should equal "http://proxy.example.com:8080"
        The line 3 of output should equal "<unset>"
        The line 4 of output should equal "<unset>"
        The line 5 of output should equal "<unset>"
        The line 6 of output should equal "<unset>"
    End

    It 'exports HTTPS proxy values only'
        HTTPS_PROXY_URLS="https://proxy.example.com:8443"

        When call exported_proxy_environment
        The line 1 of output should equal "<unset>"
        The line 2 of output should equal "<unset>"
        The line 3 of output should equal "https://proxy.example.com:8443"
        The line 4 of output should equal "https://proxy.example.com:8443"
        The line 5 of output should equal "<unset>"
        The line 6 of output should equal "<unset>"
    End

    It 'exports no-proxy values only'
        NO_PROXY_URLS="127.0.0.1,localhost,.svc"

        When call exported_proxy_environment
        The line 1 of output should equal "<unset>"
        The line 2 of output should equal "<unset>"
        The line 3 of output should equal "<unset>"
        The line 4 of output should equal "<unset>"
        The line 5 of output should equal "127.0.0.1,localhost,.svc"
        The line 6 of output should equal "127.0.0.1,localhost,.svc"
    End

    It 'exports all proxy values simultaneously'
        HTTP_PROXY_URLS="http://proxy.example.com:8080"
        HTTPS_PROXY_URLS="https://proxy.example.com:8443"
        NO_PROXY_URLS="127.0.0.1,localhost,.svc"

        When call exported_proxy_environment
        The line 1 of output should equal "http://proxy.example.com:8080"
        The line 2 of output should equal "http://proxy.example.com:8080"
        The line 3 of output should equal "https://proxy.example.com:8443"
        The line 4 of output should equal "https://proxy.example.com:8443"
        The line 5 of output should equal "127.0.0.1,localhost,.svc"
        The line 6 of output should equal "127.0.0.1,localhost,.svc"
    End

    It 'leaves existing values unchanged when proxy URLs are empty'
        export HTTP_PROXY="existing-http-upper"
        export http_proxy="existing-http-lower"
        export HTTPS_PROXY="existing-https-upper"
        export https_proxy="existing-https-lower"
        export NO_PROXY="existing-no-proxy-upper"
        export no_proxy="existing-no-proxy-lower"

        When call exported_proxy_environment
        The line 1 of output should equal "existing-http-upper"
        The line 2 of output should equal "existing-http-lower"
        The line 3 of output should equal "existing-https-upper"
        The line 4 of output should equal "existing-https-lower"
        The line 5 of output should equal "existing-no-proxy-upper"
        The line 6 of output should equal "existing-no-proxy-lower"
    End
End

Describe 'connectivity preflight timeouts'
    It 'exports proxy values before persistent configuration and the outbound check'
        proxy_consumer_order() {
            awk '
                /export HTTP_PROXY=/ && !proxy_exports { proxy_exports = NR }
                /^[[:space:]]*configureEtcEnvironment$/ { configure_etc_environment = NR }
                /retrycmd_if_failure 20 1 15 \$OUTBOUND_COMMAND/ { outbound_check = NR }
                END {
                    print proxy_exports < configure_etc_environment
                    print proxy_exports < outbound_check
                }
            ' parts/linux/cloud-init/artifacts/cse_main.sh
        }

        When call proxy_consumer_order
        The line 1 of output should equal "1"
        The line 2 of output should equal "1"
    End

    It 'allows DNS failover during the outbound check'
        When run awk '/retrycmd_if_failure [0-9]+ [0-9]+ [0-9]+ \$OUTBOUND_COMMAND/ { print $2, $3, $4 }' parts/linux/cloud-init/artifacts/cse_main.sh
        The output should eq "20 1 15"
        The status should be success
    End

    It 'allows DNS failover during the API server check'
        When run grep -F 'retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 15 curl' parts/linux/cloud-init/artifacts/cse_main.sh
        The output should include 'retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 15 curl'
        The status should be success
    End
End
