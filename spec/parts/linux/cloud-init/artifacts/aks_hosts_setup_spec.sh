#shellcheck shell=bash
#shellcheck disable=SC2148

Describe 'aks-hosts-setup.sh'
    SCRIPT_PATH="parts/linux/cloud-init/artifacts/aks-hosts-setup.sh"

    # Helper to build a test script that uses the real system dig.
    # Overrides only HOSTS_FILE and LOCALDNS_CRITICAL_FQDNS, preserving everything else
    # (resolution loop, atomic write) from the real script.
    # Uses sed to strip the shebang, set -euo pipefail, and HOSTS_FILE= lines
    # so the test is not brittle to comment changes at the top of the script.
    build_test_script() {
        local test_dir="$1"
        local hosts_file="$2"
        local fqdns="${3:-mcr.microsoft.com,packages.microsoft.com,management.azure.com,login.microsoftonline.com,acs-mirror.azureedge.net,packages.aks.azure.com}"
        local test_script="${test_dir}/aks-hosts-setup-test.sh"

        cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
HOSTS_FILE="${hosts_file}"
export LOCALDNS_CRITICAL_FQDNS="${fqdns}"
EOF
        sed -e '/^#!\/bin\/bash/d' -e '/^set -euo pipefail/d' -e '/^HOSTS_FILE=/d' "${SCRIPT_PATH}" >> "${test_script}"
        chmod +x "${test_script}"
        echo "${test_script}"
    }

    # Helper to build a test script with a mock dig prepended to PATH.
    # Used only for edge-case tests that need controlled DNS output
    # (failure handling, invalid response filtering).
    build_mock_test_script() {
        local test_dir="$1"
        local hosts_file="$2"
        local mock_bin_dir="$3"
        local fqdns="${4:-mcr.microsoft.com,packages.microsoft.com,management.azure.com,login.microsoftonline.com,acs-mirror.azureedge.net,packages.aks.azure.com}"
        local test_script="${test_dir}/aks-hosts-setup-test.sh"

        cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
export PATH="${mock_bin_dir}:\$PATH"
HOSTS_FILE="${hosts_file}"
export LOCALDNS_CRITICAL_FQDNS="${fqdns}"
EOF
        sed -e '/^#!\/bin\/bash/d' -e '/^set -euo pipefail/d' -e '/^HOSTS_FILE=/d' "${SCRIPT_PATH}" >> "${test_script}"
        chmod +x "${test_script}"
        echo "${test_script}"
    }

    # Creates a mock dig executable that simulates DNS failure (empty output).
    create_failure_mock() {
        local mock_bin_dir="$1"
        mkdir -p "${mock_bin_dir}"
        cat > "${mock_bin_dir}/dig" << 'MOCK_EOF'
#!/usr/bin/env bash
# Simulate DNS failure: dig +short returns empty output
exit 0
MOCK_EOF
        chmod +x "${mock_bin_dir}/dig"
    }

    # -----------------------------------------------------------------------
    # Tests using real dig (no mocks)
    # -----------------------------------------------------------------------

    Describe 'DNS resolution and hosts file creation (public cloud FQDNs)'
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}")
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'creates hosts file with resolved addresses for all critical FQDNs'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The file "$HOSTS_FILE" should be exist
            The output should include "Starting AKS critical FQDN hosts resolution"
            The output should include "AKS critical FQDN hosts resolution completed"
        End

        It 'reports the number of FQDNs received from RP'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Received 6 critical FQDNs from RP"
        End

        It 'resolves all public cloud FQDNs'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            # Verify the script attempts to resolve all expected public cloud FQDNs
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for packages.microsoft.com"
            The output should include "Resolving addresses for management.azure.com"
            The output should include "Resolving addresses for login.microsoftonline.com"
            The output should include "Resolving addresses for acs-mirror.azureedge.net"
            The output should include "Resolving addresses for packages.aks.azure.com"
            # Verify hosts file contains real resolved entries
            The contents of file "$HOSTS_FILE" should include "mcr.microsoft.com"
            The contents of file "$HOSTS_FILE" should include "packages.microsoft.com"
        End

        It 'writes valid hosts file format'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The file "$HOSTS_FILE" should be exist
            The output should include "Writing addresses"
        End

        It 'includes header comments in hosts file'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "AKS critical FQDN hosts resolution"
            The contents of file "$HOSTS_FILE" should include "# AKS critical FQDN addresses resolved at"
            The contents of file "$HOSTS_FILE" should include "# This file is automatically generated by aks-hosts-setup.service"
        End
    End

    Describe 'FQDN list parsing'
        # These tests verify the script resolves whatever FQDNs the RP passes.
        # No cloud-specific logic in the script — the RP controls the FQDN list.
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'resolves China cloud FQDNs when passed by RP'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "mcr.azure.cn,mcr.azk8s.cn,login.partner.microsoftonline.cn,management.chinacloudapi.cn,packages.microsoft.com")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Received 5 critical FQDNs from RP"
            # Should resolve China-specific endpoints
            The output should include "Resolving addresses for mcr.azure.cn"
            The output should include "Resolving addresses for mcr.azk8s.cn"
            The output should include "Resolving addresses for login.partner.microsoftonline.cn"
            The output should include "Resolving addresses for management.chinacloudapi.cn"
            The output should include "Resolving addresses for packages.microsoft.com"
            # Should NOT attempt public cloud endpoints (they weren't passed)
            The output should not include "Resolving addresses for login.microsoftonline.com"
            The output should not include "Resolving addresses for management.azure.com"
        End

        It 'resolves US Gov cloud FQDNs when passed by RP'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "mcr.microsoft.com,login.microsoftonline.us,management.usgovcloudapi.net,packages.aks.azure.com,acs-mirror.azureedge.net,packages.microsoft.com")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Received 6 critical FQDNs from RP"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.us"
            The output should include "Resolving addresses for management.usgovcloudapi.net"
            The output should include "Resolving addresses for packages.aks.azure.com"
            The output should not include "Resolving addresses for login.microsoftonline.com"
            The output should not include "Resolving addresses for management.azure.com"
        End

        It 'resolves arbitrary sovereign cloud FQDNs when passed by RP'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "mcr.microsoft.com,login.microsoftonline.com")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Received 2 critical FQDNs from RP"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.com"
        End

        It 'exits gracefully when LOCALDNS_CRITICAL_FQDNS is unset'
            local test_script="${TEST_DIR}/aks-hosts-setup-test-nofqdns.sh"
            cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
HOSTS_FILE="${HOSTS_FILE}"
unset LOCALDNS_CRITICAL_FQDNS
EOF
            tail -n +10 "${SCRIPT_PATH}" >> "${test_script}"
            chmod +x "${test_script}"

            When run command bash "${test_script}"
            The status should be success
            The output should include "LOCALDNS_CRITICAL_FQDNS is not set or empty"
            The output should include "Exiting without modifying hosts file"
        End

        It 'exits gracefully when LOCALDNS_CRITICAL_FQDNS is empty string'
            local test_script="${TEST_DIR}/aks-hosts-setup-test-empty.sh"
            cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
HOSTS_FILE="${HOSTS_FILE}"
export LOCALDNS_CRITICAL_FQDNS=""
EOF
            tail -n +10 "${SCRIPT_PATH}" >> "${test_script}"
            chmod +x "${test_script}"

            When run command bash "${test_script}"
            The status should be success
            The output should include "LOCALDNS_CRITICAL_FQDNS is not set or empty"
            The output should include "Exiting without modifying hosts file"
        End

        It 'handles single FQDN correctly'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "mcr.microsoft.com")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Received 1 critical FQDNs from RP"
            The output should include "Resolving addresses for mcr.microsoft.com"
        End
    End

    Describe 'Atomic file write'
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}")
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'does not leave a temp file behind after successful write'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "AKS critical FQDN hosts resolution"
            The file "$HOSTS_FILE" should be exist
        End

        It 'verifies no leftover temp files exist'
            bash "${TEST_SCRIPT}" >/dev/null 2>&1
            # The temp file (hosts.testing.tmp.<pid>) should have been renamed away
            When run command find "${TEST_DIR}" -name 'hosts.testing.tmp.*'
            The output should equal ""
        End

        It 'sets correct permissions on the hosts file'
            bash "${TEST_SCRIPT}" >/dev/null 2>&1
            When run command stat -c '%a' "${HOSTS_FILE}"
            The output should equal "644"
        End
    End

    # -----------------------------------------------------------------------
    # Mock-based tests below
    # These require controlled dig output to verify error handling
    # and response filtering logic that cannot be triggered with real DNS.
    # -----------------------------------------------------------------------

    Describe 'DNS resolution failure handling (mock)'
        setup() {
            TEST_DIR=$(mktemp -d)
            MOCK_BIN="${TEST_DIR}/mock_bin"
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            create_failure_mock "${MOCK_BIN}"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'exits gracefully when no DNS records are resolved'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The output should include "This is likely a temporary DNS issue"
        End

        It 'does not create hosts file when no DNS records are resolved'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The file "$HOSTS_FILE" should not be exist
        End

        It 'preserves existing hosts file when no DNS records are resolved'
            echo "# old hosts content" > "${HOSTS_FILE}"
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            # Original hosts file should still be intact
            The contents of file "$HOSTS_FILE" should include "# old hosts content"
        End
    End

    Describe 'Invalid DNS response filtering (mock)'
        setup() {
            TEST_DIR=$(mktemp -d)
            MOCK_BIN="${TEST_DIR}/mock_bin"
            mkdir -p "${MOCK_BIN}"
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'filters out NXDOMAIN responses from hosts file'
            create_failure_mock "${MOCK_BIN}"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The file "$HOSTS_FILE" should not be exist
        End

        It 'filters out SERVFAIL responses from hosts file'
            cat > "${MOCK_BIN}/dig" << 'MOCK_EOF'
#!/usr/bin/env bash
# Simulate SERVFAIL: dig +short returns empty output
exit 0
MOCK_EOF
            chmod +x "${MOCK_BIN}/dig"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The file "$HOSTS_FILE" should not be exist
        End

        It 'does not write non-IP strings to hosts file'
            cat > "${MOCK_BIN}/dig" << 'MOCK_EOF'
#!/usr/bin/env bash
record_type=""
for arg in "$@"; do
    if [[ "$arg" == "A" ]]; then
        record_type="A"
    elif [[ "$arg" == "AAAA" ]]; then
        record_type="AAAA"
    fi
done

# dig +short outputs one result per line, no prefix
if [[ "$record_type" == "A" ]]; then
    echo "1.2.3.4"
    echo "not-an-ip"
    echo "NXDOMAIN"
fi
MOCK_EOF
            chmod +x "${MOCK_BIN}/dig"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Writing addresses"
            The file "$HOSTS_FILE" should be exist
            The contents of file "$HOSTS_FILE" should include "1.2.3.4"
            The contents of file "$HOSTS_FILE" should not include "not-an-ip"
            The contents of file "$HOSTS_FILE" should not include "NXDOMAIN"
        End

        It 'does not write invalid IPv6 strings to hosts file'
            cat > "${MOCK_BIN}/dig" << 'MOCK_EOF'
#!/usr/bin/env bash
record_type=""
for arg in "$@"; do
    if [[ "$arg" == "A" ]]; then
        record_type="A"
    elif [[ "$arg" == "AAAA" ]]; then
        record_type="AAAA"
    fi
done

# dig +short outputs one result per line, no prefix
if [[ "$record_type" == "AAAA" ]]; then
    echo "2001:db8::1"
    echo "not-an-ipv6"
    echo "SERVFAIL"
    echo "fe80::1"
    echo "1:2"
    echo ":ff"
    echo ":::::::"
fi
MOCK_EOF
            chmod +x "${MOCK_BIN}/dig"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Writing addresses"
            The file "$HOSTS_FILE" should be exist
            The contents of file "$HOSTS_FILE" should include "2001:db8::1"
            The contents of file "$HOSTS_FILE" should include "fe80::1"
            The contents of file "$HOSTS_FILE" should not include "not-an-ipv6"
            The contents of file "$HOSTS_FILE" should not include "SERVFAIL"
            # Tightened IPv6 validation rejects too-short strings with fewer than 2 colons
            The contents of file "$HOSTS_FILE" should not include "1:2"
            The contents of file "$HOSTS_FILE" should not include ":ff"
            # Rejects all-colon strings with no hex digits
            The contents of file "$HOSTS_FILE" should not include ":::::::"
        End

        It 'rejects IPv4 addresses with out-of-range octets'
            cat > "${MOCK_BIN}/dig" << 'MOCK_EOF'
#!/usr/bin/env bash
record_type=""
for arg in "$@"; do
    if [[ "$arg" == "A" ]]; then
        record_type="A"
    elif [[ "$arg" == "AAAA" ]]; then
        record_type="AAAA"
    fi
done

# dig +short outputs one result per line, no prefix
if [[ "$record_type" == "A" ]]; then
    echo "10.0.0.1"
    echo "999.999.999.999"
    echo "256.1.1.1"
    echo "1.2.3.400"
    echo "255.255.255.255"
fi
MOCK_EOF
            chmod +x "${MOCK_BIN}/dig"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Writing addresses"
            The file "$HOSTS_FILE" should be exist
            The contents of file "$HOSTS_FILE" should include "10.0.0.1"
            The contents of file "$HOSTS_FILE" should include "255.255.255.255"
            The contents of file "$HOSTS_FILE" should not include "999.999.999.999"
            The contents of file "$HOSTS_FILE" should not include "256.1.1.1"
            The contents of file "$HOSTS_FILE" should not include "1.2.3.400"
        End
    End
End
