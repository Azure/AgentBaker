#!/bin/bash
# ShellSpec tests for node-exporter-startup.sh
#
# HIGH-RISK REGRESSION #1 (PR #7704): TLS must be OFF by default.
#
# The original implementation in PR #7704 always activated TLS when kubelet
# serving certs were present on the node (i.e., on every production node).
# The VHD-resident web-config.yml further hardcoded RequireAndVerifyClientCert,
# which mandates mutual TLS.  AKS control-plane Prometheus scrapes node-exporter
# through the API-server proxy over plain HTTP — enabling mTLS silently breaks
# scraping on every node the moment a new VHD is rolled out.
#
# Correct behaviour: TLS is strictly opt-in via NODE_EXPORTER_TLS_ENABLED=true
# in /etc/default/node-exporter.  When that flag is absent or "false",
# node-exporter MUST start without --web.config.file regardless of whether
# kubelet certs exist on the node.

Describe 'node-exporter-startup.sh'
    # All paths used by the script (TLS config, binary, certs) are redirected
    # to a temp directory so tests are hermetically isolated from the host.
    setup() {
        TMP_DIR=$(mktemp -d)

        # Fake node-exporter binary: echoes its argv so assertions can inspect args.
        cat > "$TMP_DIR/fake-node-exporter" <<'EOF'
#!/bin/bash
echo "EXEC_ARGS: $@"
EOF
        chmod +x "$TMP_DIR/fake-node-exporter"

        # Testability overrides: the five vars below were added to the production
        # script specifically to enable isolated unit testing; production defaults
        # are used when the vars are unset.
        export NODE_EXPORTER_BIN="$TMP_DIR/fake-node-exporter"
        export NODE_EXPORTER_TLS_CONFIG_PATH="$TMP_DIR/web-config.yml"
        export NODE_EXPORTER_WAIT_TIMEOUT=0
        export NODE_EXPORTER_ROTATION_CERT="$TMP_DIR/pki/kubelet-server-current.pem"
        export NODE_EXPORTER_STATIC_CERT_CRT="$TMP_DIR/certs/kubeletserver.crt"
        export NODE_EXPORTER_STATIC_CERT_KEY="$TMP_DIR/certs/kubeletserver.key"
    }

    cleanup() {
        rm -rf "$TMP_DIR"
        unset NODE_EXPORTER_BIN NODE_EXPORTER_TLS_CONFIG_PATH \
              NODE_EXPORTER_TLS_ENABLED NODE_EXPORTER_TLS_CLIENT_AUTH \
              NODE_EXPORTER_EXTRA_ARGS NODE_EXPORTER_WAIT_TIMEOUT \
              NODE_EXPORTER_ROTATION_CERT NODE_EXPORTER_STATIC_CERT_CRT \
              NODE_EXPORTER_STATIC_CERT_KEY
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    # -----------------------------------------------------------------------
    # Regression guard: TLS must be OFF by default (high-risk regression #1).
    #
    # If any of these tests fail it means someone re-introduced always-on TLS,
    # which breaks AKS Prometheus scraping cluster-wide on next VHD rollout.
    # -----------------------------------------------------------------------
    Describe 'TLS disabled by default'
        It 'does not pass --web.config.file when NODE_EXPORTER_TLS_ENABLED is unset'
            unset NODE_EXPORTER_TLS_ENABLED
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should not include '--web.config.file'
        End

        It 'does not pass --web.config.file when NODE_EXPORTER_TLS_ENABLED is false'
            export NODE_EXPORTER_TLS_ENABLED="false"
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should not include '--web.config.file'
        End

        It 'does not write a web-config.yml when TLS is disabled'
            unset NODE_EXPORTER_TLS_ENABLED
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The path "$TMP_DIR/web-config.yml" should not be exist
        End

        It 'does not activate TLS even when the rotation cert file exists on the node'
            # Regression guard: in PR #7704 the mere presence of cert files was
            # sufficient to activate TLS (no opt-in flag existed).  Ensure that
            # the opt-in flag is now the sole gate.
            unset NODE_EXPORTER_TLS_ENABLED
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should not include '--web.config.file'
        End
    End

    # -----------------------------------------------------------------------
    # TLS opt-in: correct behaviour when NODE_EXPORTER_TLS_ENABLED=true.
    # -----------------------------------------------------------------------
    Describe 'TLS explicitly enabled'
        It 'passes --web.config.file when TLS is enabled and the rotation cert exists'
            export NODE_EXPORTER_TLS_ENABLED="true"
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include "--web.config.file="
            The path "$TMP_DIR/web-config.yml" should be exist
            The contents of file "$TMP_DIR/web-config.yml" should include 'tls_server_config'
        End

        It 'sets cert_file and key_file to the rotation cert path'
            export NODE_EXPORTER_TLS_ENABLED="true"
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            # kubelet-server-current.pem is a combined PEM file (cert + key in one file),
            # so both cert_file and key_file intentionally reference the same path.
            The contents of file "$TMP_DIR/web-config.yml" should include "cert_file: \"$TMP_DIR/pki/kubelet-server-current.pem\""
            The contents of file "$TMP_DIR/web-config.yml" should include "key_file: \"$TMP_DIR/pki/kubelet-server-current.pem\""
        End

        It 'falls back to static certs when the rotation cert is absent'
            export NODE_EXPORTER_TLS_ENABLED="true"
            # Do NOT create the rotation cert; create the static pair instead.
            mkdir -p "$TMP_DIR/certs"
            touch "$TMP_DIR/certs/kubeletserver.crt"
            touch "$TMP_DIR/certs/kubeletserver.key"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include "--web.config.file="
            The contents of file "$TMP_DIR/web-config.yml" should include "cert_file: \"$TMP_DIR/certs/kubeletserver.crt\""
            The contents of file "$TMP_DIR/web-config.yml" should include "key_file: \"$TMP_DIR/certs/kubeletserver.key\""
        End

        It 'starts without TLS and emits a warning when no certs are available'
            # WAIT_TIMEOUT=0 (set in setup) ensures the cert-wait loop exits
            # immediately without blocking the test.
            export NODE_EXPORTER_TLS_ENABLED="true"
            # No cert files created.

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include 'WARNING'
            The output should not include '--web.config.file'
        End

        It 'defaults client_auth_type to NoClientCert when NODE_EXPORTER_TLS_CLIENT_AUTH is unset'
            # PR #7704 used the VHD baseline web-config.yml which hardcoded
            # RequireAndVerifyClientCert, breaking unauthenticated scrapers.
            # The correct default is NoClientCert so plain-HTTP scrapers still work
            # when an operator enables TLS without specifying client auth.
            export NODE_EXPORTER_TLS_ENABLED="true"
            unset NODE_EXPORTER_TLS_CLIENT_AUTH
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "NoClientCert"'
        End

        It 'honours the requested client_auth_type when NODE_EXPORTER_TLS_CLIENT_AUTH is set'
            export NODE_EXPORTER_TLS_ENABLED="true"
            export NODE_EXPORTER_TLS_CLIENT_AUTH="RequireAndVerifyClientCert"
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "RequireAndVerifyClientCert"'
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_ca_file: "/etc/kubernetes/certs/ca.crt"'
        End

        It 'emits a warning and falls back to NoClientCert for an unsupported client_auth_type'
            export NODE_EXPORTER_TLS_ENABLED="true"
            export NODE_EXPORTER_TLS_CLIENT_AUTH="BadValue"
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"

            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include 'WARNING'
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "NoClientCert"'
        End
    End

    # -----------------------------------------------------------------------
    # Base arguments: always present regardless of TLS setting.
    # -----------------------------------------------------------------------
    Describe 'base arguments'
        It 'disables the wifi collector'
            unset NODE_EXPORTER_TLS_ENABLED
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include '--no-collector.wifi'
        End

        It 'disables the hwmon collector'
            unset NODE_EXPORTER_TLS_ENABLED
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include '--no-collector.hwmon'
        End

        It 'enables the cpu.info collector'
            unset NODE_EXPORTER_TLS_ENABLED
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include '--collector.cpu.info'
        End

        It 'appends NODE_EXPORTER_EXTRA_ARGS verbatim to the invocation'
            unset NODE_EXPORTER_TLS_ENABLED
            export NODE_EXPORTER_EXTRA_ARGS="--collector.systemd --no-collector.bonding"
            When run ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
            The status should be success
            The output should include '--collector.systemd'
            The output should include '--no-collector.bonding'
        End
    End
End
