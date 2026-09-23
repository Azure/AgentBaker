#!/bin/bash

Describe 'init-aks-cloud.sh certificate logging'
    setup() {
        TEST_DIR="$(mktemp -d)"
        unset AKS_CLOUD_LOG_CERTIFICATES JOURNAL_STREAM
        # shellcheck disable=SC1091
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    logger() {
        printf '%s\n' "$@" > "${TEST_DIR}/logger-args"
        cat > "${TEST_DIR}/diagnostics"
    }

    make_certificate() {
        openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
            -subj '/CN=CA refresh logging test' \
            -keyout "${TEST_DIR}/key.pem" -out "${TEST_DIR}/cert.pem" 2>/dev/null
    }

    Describe 'opt-in boundaries'
        Parameters
            ''
            false
        End

        It "does not read certificate files with certificates='$1'"
            AKS_CLOUD_LOG_CERTIFICATES=$1
            openssl() { echo 'Unexpected certificate read' >&2; return 1; }
            When call log_certificates "${TEST_DIR}/missing.pem"
            The status should be success
            The stdout should be blank
            The stderr should be blank
            The path "${TEST_DIR}/diagnostics" should not be exist
        End
    End

    It 'disables inherited tracing without exposing expanded variables'
        When run bash -xc '__SOURCED__=1 . ./parts/linux/cloud-init/artifacts/init-aks-cloud.sh; response=TRACE_PAYLOAD_SENTINEL; [[ $- != *x* ]]'
        The status should be success
        The stdout should be blank
        The stderr should include 'set +x'
        The stderr should not include TRACE_PAYLOAD_SENTINEL
    End

    Describe 'certificate log transport'
        Parameters
            '' cron
            '8:1234' systemd
        End

        It "sends the complete public PEM with the single opt-in under $2"
            make_certificate
            AKS_CLOUD_LOG_CERTIFICATES=true
            JOURNAL_STREAM=$1
            When call log_certificates "${TEST_DIR}/cert.pem"
            The status should be success
            The stdout should be blank
            The stderr should be blank
            The contents of file "${TEST_DIR}/logger-args" should eq $'-p\nuser.debug\n-t\nazure-ca-refresh'
            The contents of file "${TEST_DIR}/diagnostics" should include "$(cat "${TEST_DIR}/cert.pem")"
        End
    End

    log_bundle() {
        log_certificates "${TEST_DIR}/bundle.pem"
        grep -c '^-----BEGIN CERTIFICATE-----$' "${TEST_DIR}/diagnostics"
    }

    It 'logs every certificate in a bundle but no private key or unrelated data'
        make_certificate
        {
            cat "${TEST_DIR}/cert.pem" "${TEST_DIR}/key.pem"
            printf '%s\n' RAW_RESPONSE_SENTINEL
            cat "${TEST_DIR}/cert.pem"
        } > "${TEST_DIR}/bundle.pem"
        AKS_CLOUD_LOG_CERTIFICATES=true
        When call log_bundle
        The status should be success
        The stdout should eq 2
        The stderr should be blank
        The contents of file "${TEST_DIR}/diagnostics" should include "$(cat "${TEST_DIR}/cert.pem")"
        The contents of file "${TEST_DIR}/diagnostics" should not include 'PRIVATE KEY'
        The contents of file "${TEST_DIR}/diagnostics" should not include "$(sed -n '2p' "${TEST_DIR}/key.pem")"
        The contents of file "${TEST_DIR}/diagnostics" should not include RAW_RESPONSE_SENTINEL
    End

    It 'reports invalid certificates without dumping raw content'
        printf '%s\n' RAW_RESPONSE_SENTINEL > "${TEST_DIR}/invalid.pem"
        AKS_CLOUD_LOG_CERTIFICATES=true
        When call log_certificates "${TEST_DIR}/invalid.pem"
        The status should be success
        The stdout should be blank
        The stderr should eq "Warning: could not decode public certificates: ${TEST_DIR}/invalid.pem"
        The path "${TEST_DIR}/diagnostics" should not be exist
    End

    It 'reports logging failures without falling back to a console PEM dump'
        make_certificate
        AKS_CLOUD_LOG_CERTIFICATES=true
        logger() { cat >/dev/null; return 1; }
        When call log_certificates "${TEST_DIR}/cert.pem"
        The status should be success
        The stdout should be blank
        The stderr should eq 'Warning: failed to log certificates'
    End

    Describe 'request output'
        curl() { printf '%s\n%s' "$RESPONSE_BODY" "$RESPONSE_CODE"; }
        sleep() { :; }

        It 'returns response data unchanged without logging it'
            RESPONSE_BODY=$'-----BEGIN CERTIFICATE-----\nPUBLIC_DATA\n-----END CERTIFICATE-----'
            RESPONSE_CODE=200
            When call make_request_with_retry http://wireserver.test/cert
            The status should be success
            The stdout should eq "$RESPONSE_BODY"
            The stderr should be blank
            The path "${TEST_DIR}/diagnostics" should not be exist
        End

        It 'retains retry and failure messages but never raw error responses'
            RESPONSE_BODY=RAW_RESPONSE_SENTINEL
            RESPONSE_CODE=500
            AKS_CLOUD_LOG_CERTIFICATES=true
            When call make_request_with_retry http://wireserver.test/cert
            The status should eq 1
            The stdout should be blank
            The stderr should include 'wireserver request failed (HTTP 500) on attempt 1/10:'
            The stderr should include 'exhausted all retries'
            The stderr should not include RAW_RESPONSE_SENTINEL
            The path "${TEST_DIR}/diagnostics" should not be exist
        End

        It 'retains opt-in status without the raw opt-in response'
            RESPONSE_BODY='{"IsOptedInForRootCerts":true,"extra":"RAW_RESPONSE_SENTINEL"}'
            RESPONSE_CODE=200
            When call is_opted_in_for_root_certs
            The status should be success
            The stdout should eq IsOptedInForRootCerts=true
            The stderr should be blank
        End

        It 'preserves the expected opt-out return code'
            RESPONSE_BODY='{"IsOptedInForRootCerts":false}'
            RESPONSE_CODE=200
            When call is_opted_in_for_root_certs
            The status should eq 1
            The stdout should include 'Skipping custom cloud root cert installation'
            The stderr should be blank
        End
    End

    Describe 'trust-store installation'
        install_with_diagnostics() {
            # Relocate staging only; mock every trust-store write and update.
            # shellcheck disable=SC1090
            __SOURCED__=1 . <(sed "s|/root/AzureCACertificates|${TEST_DIR}/certs|g" \
                ./parts/linux/cloud-init/artifacts/init-aks-cloud.sh)
            IS_AZURELINUX=1
            cp() { :; }
            update-ca-trust() { return "${INSTALL_STATUS:-0}"; }
            install_certs_to_trust_store
        }

        It 'logs public PEMs through the actual installation path when opted in'
            make_certificate
            mkdir "${TEST_DIR}/certs"
            cp "${TEST_DIR}/cert.pem" "${TEST_DIR}/certs/test.crt"
            AKS_CLOUD_LOG_CERTIFICATES=true
            When call install_with_diagnostics
            The status should be success
            The stdout should eq 'Refreshing CA trust store'
            The stderr should be blank
            The contents of file "${TEST_DIR}/diagnostics" should include "$(cat "${TEST_DIR}/cert.pem")"
        End

        It 'does not read or log certificate bodies during default installation'
            mkdir "${TEST_DIR}/certs"
            printf '%s\n' RAW_RESPONSE_SENTINEL > "${TEST_DIR}/certs/test.crt"
            openssl() { echo 'Unexpected certificate read' >&2; return 1; }
            When call install_with_diagnostics
            The status should be success
            The stdout should eq 'Refreshing CA trust store'
            The stderr should be blank
            The path "${TEST_DIR}/diagnostics" should not be exist
        End

        It 'preserves installation failures'
            mkdir "${TEST_DIR}/certs"
            : > "${TEST_DIR}/certs/test.crt"
            INSTALL_STATUS=9
            When call install_with_diagnostics
            The status should eq 9
            The stdout should eq 'Refreshing CA trust store'
            The stderr should be blank
        End
    End
End
