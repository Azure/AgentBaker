#!/bin/bash

lsb_release() {
    echo "mock lsb_release"
}

Describe 'localdns.sh'

    Describe 'verify_localdns_files'
        setup() {
            LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
            LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"
            mkdir -p "$LOCALDNS_SCRIPT_PATH"
            echo "forward . 168.63.129.16" >> "$LOCALDNS_CORE_FILE"

            LOCALDNS_SLICE_PATH="/etc/systemd/system"
            LOCALDNS_SLICE_FILE="${LOCALDNS_SLICE_PATH}/localdns.slice"
            mkdir -p "$LOCALDNS_SLICE_PATH"
            echo "localdns slice file" >> "$LOCALDNS_SLICE_FILE"

            COREDNS_BINARY_PATH="/opt/azure/containers/localdns/binary/coredns"
            mkdir -p "$(dirname "$COREDNS_BINARY_PATH")"

    cat <<EOF > "$COREDNS_BINARY_PATH"
#!/bin/bash
if [[ "\$1" == "--version" ]]; then
    echo "mock v1.12.0  linux/amd64"
    exit 0
fi
EOF
            chmod +x "$COREDNS_BINARY_PATH"

            RESOLV_CONF="/run/systemd/resolve/resolv.conf"
            mkdir -p "$(dirname "$RESOLV_CONF")"

cat <<EOF > "$RESOLV_CONF"
nameserver 10.0.0.1
nameserver 10.0.0.2
EOF
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }
        cleanup() {
            rm -rf "$LOCALDNS_SCRIPT_PATH"
            rm -rf "$LOCALDNS_SLICE_PATH"
            rm -rf "$COREDNS_BINARY_PATH"
            rm -rf "$RESOLV_CONF"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        #------------------------ Test localdns corefile -------------------------------------------------
        It 'should return failure if localdns corefile is missing'
            rm -f "$LOCALDNS_CORE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        It 'should return failure if localdns corefile is empty'
            > "$LOCALDNS_CORE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        #------------------------ Test localdns slice file ------------------------------------------------
        It 'should return failure if localdns slicefile is missing'
            rm -f "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        It 'should return failure if localdns slicefile is empty'
            > "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        #------------------------- Test coredns binary -----------------------------------------------------
        It 'should return failure if coredns binary is missing'
            rm -f "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns binary is not executable'
            chmod -x "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns --version command fails'
            echo '#!/bin/bash' > "$COREDNS_BINARY_PATH"
            echo 'exit 1' >> "$COREDNS_BINARY_PATH"
            chmod +x "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Failed to execute '$COREDNS_BINARY_PATH --version'."
        End

        #------------------------- Test systemd resolv -----------------------------------------------------
        It 'should fail if /run/systemd/resolve/resolv.conf not found'
            rm -f "$RESOLV_CONF"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "/run/systemd/resolve/resolv.conf not found."
        End

        #------------------------- Test AzureDNS replace in corefile ---------------------------------------
        It 'should replace 168.63.129.16 with UpstreamDNS IPs if upstream VNET DNS server is not same as AzureDNS'
            When run verify_localdns_files
            The status should be success
            The stdout should include "forward . 10.0.0.1 10.0.0.2"
        End

        #------------------------- All success path in verify_localdns_files -------------------------------
        It 'should return success - all successful path'
            When run verify_localdns_files
            The status should be success
            The stdout should include "forward . 10.0.0.1 10.0.0.2"
        End
    End
End
