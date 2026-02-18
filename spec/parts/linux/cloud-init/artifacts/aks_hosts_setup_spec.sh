#shellcheck shell=bash
#shellcheck disable=SC2148

Describe 'aks-hosts-setup.sh'
    SCRIPT_PATH="parts/linux/cloud-init/artifacts/aks-hosts-setup.sh"

    # Helper to build a test script that uses the real system nslookup.
    # Overrides only HOSTS_FILE and TARGET_CLOUD, preserving everything else
    # (cloud selection, resolution loop, atomic write) from the real script.
    # Lines 1-9 of the real script are: shebang, set, blank, comments, and HOSTS_FILE=.
    build_test_script() {
        local test_dir="$1"
        local hosts_file="$2"
        local target_cloud="${3:-AzurePublicCloud}"
        local test_script="${test_dir}/aks-hosts-setup-test.sh"

        cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
HOSTS_FILE="${hosts_file}"
export TARGET_CLOUD="${target_cloud}"
EOF
        tail -n +10 "${SCRIPT_PATH}" >> "${test_script}"
        chmod +x "${test_script}"
        echo "${test_script}"
    }

    # Helper to build a test script with a mock nslookup prepended to PATH.
    # Used only for edge-case tests that need controlled DNS output
    # (failure handling, invalid response filtering).
    build_mock_test_script() {
        local test_dir="$1"
        local hosts_file="$2"
        local mock_bin_dir="$3"
        local target_cloud="${4:-AzurePublicCloud}"
        local test_script="${test_dir}/aks-hosts-setup-test.sh"

        cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
export PATH="${mock_bin_dir}:\$PATH"
HOSTS_FILE="${hosts_file}"
export TARGET_CLOUD="${target_cloud}"
EOF
        tail -n +10 "${SCRIPT_PATH}" >> "${test_script}"
        chmod +x "${test_script}"
        echo "${test_script}"
    }

    # Creates a mock nslookup executable that simulates DNS failure (NXDOMAIN).
    create_failure_mock() {
        local mock_bin_dir="$1"
        mkdir -p "${mock_bin_dir}"
        cat > "${mock_bin_dir}/nslookup" << 'MOCK_EOF'
#!/usr/bin/env bash
echo "Server:		127.0.0.53"
echo "Address:	127.0.0.53#53"
echo ""
echo "** server can't find domain: NXDOMAIN"
MOCK_EOF
        chmod +x "${mock_bin_dir}/nslookup"
    }

    # -----------------------------------------------------------------------
    # Tests using real nslookup (no mocks)
    # -----------------------------------------------------------------------

    Describe 'DNS resolution and hosts file creation (AzurePublicCloud)'
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "AzurePublicCloud")
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

        It 'detects AzurePublicCloud environment'
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: AzurePublicCloud"
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

    Describe 'Cloud-specific FQDN selection'
        # These tests use real nslookup. Sovereign cloud domains may not resolve
        # from CI, so we assert on which FQDNs the script *attempts* to resolve
        # (visible in stdout) rather than checking hosts file contents.
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
        }

        cleanup() {
            rm -rf "$TEST_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'selects AzureChinaCloud FQDNs'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "AzureChinaCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: AzureChinaCloud"
            # Should resolve China-specific endpoints
            The output should include "Resolving addresses for mcr.azure.cn"
            The output should include "Resolving addresses for login.partner.microsoftonline.cn"
            The output should include "Resolving addresses for management.chinacloudapi.cn"
            The output should include "Resolving addresses for packages.microsoft.com"
            # Should NOT attempt public cloud endpoints
            The output should not include "Resolving addresses for login.microsoftonline.com"
            The output should not include "Resolving addresses for management.azure.com"
        End

        It 'selects AzureUSGovernmentCloud FQDNs'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "AzureUSGovernmentCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: AzureUSGovernmentCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.us"
            The output should include "Resolving addresses for management.usgovcloudapi.net"
            The output should include "Resolving addresses for packages.aks.azure.com"
            The output should not include "Resolving addresses for login.microsoftonline.com"
            The output should not include "Resolving addresses for management.azure.com"
        End

        It 'selects USNatCloud FQDNs'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "USNatCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: USNatCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.eaglex.ic.gov"
            The output should include "Resolving addresses for management.azure.eaglex.ic.gov"
            The output should not include "Resolving addresses for login.microsoftonline.com"
        End

        It 'selects USSecCloud FQDNs'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "USSecCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: USSecCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.microsoft.scloud"
            The output should include "Resolving addresses for management.azure.microsoft.scloud"
            The output should not include "Resolving addresses for login.microsoftonline.com"
        End

        It 'selects AzureStackCloud FQDNs'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "AzureStackCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: AzureStackCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for packages.microsoft.com"
            The output should not include "Resolving addresses for management.azure.com"
            The output should not include "Resolving addresses for login.microsoftonline.com"
        End

        It 'falls back to AzurePublicCloud for unknown cloud values'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "SomeUnknownCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Detected cloud environment: SomeUnknownCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
            The output should include "Resolving addresses for login.microsoftonline.com"
            The output should include "Resolving addresses for management.azure.com"
        End

        It 'falls back to AzurePublicCloud when TARGET_CLOUD is unset'
            local test_script="${TEST_DIR}/aks-hosts-setup-test-nocloud.sh"
            cat > "${test_script}" << EOF
#!/usr/bin/env bash
set -uo pipefail
HOSTS_FILE="${HOSTS_FILE}"
unset TARGET_CLOUD
EOF
            tail -n +10 "${SCRIPT_PATH}" >> "${test_script}"
            chmod +x "${test_script}"

            When run command bash "${test_script}"
            The status should be success
            The output should include "Detected cloud environment: AzurePublicCloud"
            The output should include "Resolving addresses for mcr.microsoft.com"
        End

        It 'includes packages.microsoft.com for all clouds (common FQDN)'
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "USNatCloud")
            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Resolving addresses for packages.microsoft.com"
        End
    End

    Describe 'Atomic file write'
        setup() {
            TEST_DIR=$(mktemp -d)
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            TEST_SCRIPT=$(build_test_script "${TEST_DIR}" "${HOSTS_FILE}" "AzurePublicCloud")
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
    # These require controlled nslookup output to verify error handling
    # and response filtering logic that cannot be triggered with real DNS.
    # -----------------------------------------------------------------------

    Describe 'DNS resolution failure handling (mock)'
        setup() {
            TEST_DIR=$(mktemp -d)
            MOCK_BIN="${TEST_DIR}/mock_bin"
            export HOSTS_FILE="${TEST_DIR}/hosts.testing"
            create_failure_mock "${MOCK_BIN}"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}" "AzurePublicCloud")
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
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}" "AzurePublicCloud")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The file "$HOSTS_FILE" should not be exist
        End

        It 'filters out SERVFAIL responses from hosts file'
            cat > "${MOCK_BIN}/nslookup" << 'MOCK_EOF'
#!/usr/bin/env bash
echo "Server:		127.0.0.53"
echo "Address:	127.0.0.53#53"
echo ""
echo "** server can't find domain: SERVFAIL"
MOCK_EOF
            chmod +x "${MOCK_BIN}/nslookup"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}" "AzurePublicCloud")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "WARNING: No IP addresses resolved for any domain"
            The file "$HOSTS_FILE" should not be exist
        End

        It 'does not write non-IP strings to hosts file'
            cat > "${MOCK_BIN}/nslookup" << 'MOCK_EOF'
#!/usr/bin/env bash
record_type=""
for arg in "$@"; do
    if [[ "$arg" == "-type=A" ]]; then
        record_type="A"
    elif [[ "$arg" == "-type=AAAA" ]]; then
        record_type="AAAA"
    fi
done

echo "Server:		127.0.0.53"
echo "Address:	127.0.0.53#53"
echo ""
if [[ "$record_type" == "A" ]]; then
    echo "Address: 1.2.3.4"
    echo "Address: not-an-ip"
    echo "Address: NXDOMAIN"
fi
MOCK_EOF
            chmod +x "${MOCK_BIN}/nslookup"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}" "AzurePublicCloud")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Writing addresses"
            The file "$HOSTS_FILE" should be exist
            The contents of file "$HOSTS_FILE" should include "1.2.3.4"
            The contents of file "$HOSTS_FILE" should not include "not-an-ip"
            The contents of file "$HOSTS_FILE" should not include "NXDOMAIN"
        End

        It 'does not write invalid IPv6 strings to hosts file'
            cat > "${MOCK_BIN}/nslookup" << 'MOCK_EOF'
#!/usr/bin/env bash
record_type=""
for arg in "$@"; do
    if [[ "$arg" == "-type=A" ]]; then
        record_type="A"
    elif [[ "$arg" == "-type=AAAA" ]]; then
        record_type="AAAA"
    fi
done

echo "Server:		127.0.0.53"
echo "Address:	127.0.0.53#53"
echo ""
if [[ "$record_type" == "AAAA" ]]; then
    echo "Address: 2001:db8::1"
    echo "Address: not-an-ipv6"
    echo "Address: SERVFAIL"
    echo "Address: fe80::1"
fi
MOCK_EOF
            chmod +x "${MOCK_BIN}/nslookup"
            TEST_SCRIPT=$(build_mock_test_script "${TEST_DIR}" "${HOSTS_FILE}" "${MOCK_BIN}" "AzurePublicCloud")

            When run command bash "${TEST_SCRIPT}"
            The status should be success
            The output should include "Writing addresses"
            The file "$HOSTS_FILE" should be exist
            The contents of file "$HOSTS_FILE" should include "2001:db8::1"
            The contents of file "$HOSTS_FILE" should include "fe80::1"
            The contents of file "$HOSTS_FILE" should not include "not-an-ipv6"
            The contents of file "$HOSTS_FILE" should not include "SERVFAIL"
        End
    End
End
