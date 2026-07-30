#!/bin/bash

Describe 'init-aks-custom-cloud-certs.sh'
    setup() {
        TEST_DIR="$(mktemp -d)"
        # shellcheck disable=SC1091
        . "./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-certs.sh"
    }

    cleanup() {
        rm -rf "$TEST_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'determine_cert_endpoint_mode'
        It 'returns legacy for ussec region'
            When call determine_cert_endpoint_mode "ussec"
            The output should eq "legacy"
        End

        It 'returns legacy for usnat region'
            When call determine_cert_endpoint_mode "usnat"
            The output should eq "legacy"
        End

        It 'returns legacy for ussec with suffix'
            When call determine_cert_endpoint_mode "USSecWest"
            The output should eq "legacy"
        End

        It 'returns legacy for usnat with suffix'
            When call determine_cert_endpoint_mode "USNatEast"
            The output should eq "legacy"
        End

        It 'returns rcv1p for US Gov'
            When call determine_cert_endpoint_mode "usgovvirginia"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for China'
            When call determine_cert_endpoint_mode "chinaeast2"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for France'
            When call determine_cert_endpoint_mode "francesouth"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for empty location'
            When call determine_cert_endpoint_mode ""
            The output should eq "rcv1p"
        End
    End

    run_top_level_ca_refresh() {
        local test_bin="${TEST_DIR}/bin"
        local event_capture="${TEST_DIR}/emitted-events"
        local test_script="${TEST_DIR}/init-aks-custom-cloud-certs.sh"
        local real_jq
        real_jq=$(command -v jq)
        mkdir -p "$test_bin"
        sed \
            -e "s|EVENTS_LOGGING_DIR=\"/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/\"|EVENTS_LOGGING_DIR=\"${TEST_DIR}/events/\"|" \
            -e "s|/root/AzureCACertificates|${TEST_DIR}/certificates|g" \
            ./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-certs.sh > "$test_script"

        cat > "${test_bin}/curl" <<'EOF'
#!/bin/bash
printf '{"Name": "test.cer", "CertBody": "test"}\n200\n'
EOF
        cat > "${test_bin}/jq" <<EOF
#!/bin/bash
printf '%s\n' "\$*" >> "\$EVENT_CAPTURE_FILE"
exec "$real_jq" "\$@"
EOF
        cat > "${test_bin}/cp" <<'EOF'
#!/bin/bash
exit 0
EOF
        cat > "${test_bin}/update-ca-certificates" <<'EOF'
#!/bin/bash
exit 0
EOF
        chmod +x "${test_bin}/curl" "${test_bin}/jq" "${test_bin}/cp" "${test_bin}/update-ca-certificates"

        PATH="${test_bin}:$PATH" \
            LOCATION=usseceast \
            EVENT_CAPTURE_FILE="$event_capture" \
            bash "$test_script" ca-refresh 2>/dev/null || return $?

        grep -F "AKS.CSE.rcv1p.certEndpointMode" "$event_capture" | grep -F "mode=legacy, location=usseceast"
    }

    It 'hands LOCATION to mode selection for a top-level ca-refresh run'
        When call run_top_level_ca_refresh
        The status should be success
        The output should include "AKS.CSE.rcv1p.certEndpointMode"
        The output should include "mode=legacy, location=usseceast"
    End

End
