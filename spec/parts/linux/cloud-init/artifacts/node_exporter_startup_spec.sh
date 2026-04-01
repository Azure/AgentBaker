#!/bin/bash
# ShellSpec tests for node-exporter-startup.sh
#
# High-risk regression #1 covered here:
#   TLS must be OFF by default. The original implementation (PR #7704) always enabled
#   TLS with RequireAndVerifyClientCert, which immediately broke AKS control-plane
#   Prometheus scraping because it connects over plain HTTP via the API-server proxy.
#   These tests enforce that TLS stays opt-in via NODE_EXPORTER_TLS_ENABLED=true.

STARTUP_SCRIPT="./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh"

Describe 'node-exporter-startup.sh'
    setup() {
        TMP_DIR=$(mktemp -d)

        # Fake node-exporter binary: records the args it receives so we can assert on them.
        cat > "$TMP_DIR/fake-node-exporter" <<'EOF'
#!/bin/bash
echo "EXEC_ARGS: $@"
exit 0
EOF
        chmod +x "$TMP_DIR/fake-node-exporter"

        export NODE_EXPORTER_BIN="$TMP_DIR/fake-node-exporter"
        # Redirect TLS config writes to a temp path so tests are isolated from /etc.
        export NODE_EXPORTER_TLS_CONFIG_PATH="$TMP_DIR/web-config.yml"
    }

    cleanup() {
        unset NODE_EXPORTER_BIN NODE_EXPORTER_TLS_CONFIG_PATH \
              NODE_EXPORTER_TLS_ENABLED NODE_EXPORTER_TLS_CLIENT_AUTH \
              NODE_EXPORTER_EXTRA_ARGS
        rm -rf "$TMP_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    # -----------------------------------------------------------------------
    # Regression: TLS must be disabled by default
    # -----------------------------------------------------------------------
    Describe 'TLS default-off (regression guard for PR #7704 TLS-always-on)'

        # The AKS Prometheus scraper connects to node-exporter over plain HTTP via
        # the API-server proxy. If TLS is enabled by default, scraping breaks
        # immediately on every new node. NODE_EXPORTER_TLS_ENABLED must default to
        # false so existing scrapers are never broken by a VHD update.
        It 'does not pass --web.config.file when NODE_EXPORTER_TLS_ENABLED is unset'
            unset NODE_EXPORTER_TLS_ENABLED
            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should not include '--web.config.file'
        End

        It 'does not pass --web.config.file when NODE_EXPORTER_TLS_ENABLED is false'
            export NODE_EXPORTER_TLS_ENABLED="false"
            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should not include '--web.config.file'
        End

        It 'does not write a web-config.yml when TLS is disabled'
            unset NODE_EXPORTER_TLS_ENABLED
            When run "$STARTUP_SCRIPT"
            The status should be success
            # The TLS config file must NOT be created when TLS is off.
            The path "$TMP_DIR/web-config.yml" should not be exist
        End
    End

    # -----------------------------------------------------------------------
    # TLS opt-in: verify it works correctly when explicitly enabled
    # -----------------------------------------------------------------------
    Describe 'TLS opt-in behavior'

        It 'passes --web.config.file when NODE_EXPORTER_TLS_ENABLED is true and rotation cert exists'
            export NODE_EXPORTER_TLS_ENABLED="true"
            # Create fake rotation cert (single combined PEM used for both cert and key).
            mkdir -p "$TMP_DIR/pki"
            touch "$TMP_DIR/pki/kubelet-server-current.pem"
            # Patch the cert paths used by the startup script via env substitution is not
            # available, so we rely on the script's own detection logic. Override the real
            # paths by bind-mounting is not feasible here; instead we create the real
            # /var/lib/kubelet/pki path and rely on cleanup to remove it.
            mkdir -p /var/lib/kubelet/pki
            touch /var/lib/kubelet/pki/kubelet-server-current.pem

            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include "--web.config.file=${NODE_EXPORTER_TLS_CONFIG_PATH}"
            The path "$TMP_DIR/web-config.yml" should be exist
            The contents of file "$TMP_DIR/web-config.yml" should include 'tls_server_config'
            The contents of file "$TMP_DIR/web-config.yml" should include 'kubelet-server-current.pem'
        End

        It 'writes web-config.yml with correct cert_file and key_file for rotation cert'
            export NODE_EXPORTER_TLS_ENABLED="true"
            mkdir -p /var/lib/kubelet/pki
            touch /var/lib/kubelet/pki/kubelet-server-current.pem

            When run "$STARTUP_SCRIPT"
            The status should be success
            The contents of file "$TMP_DIR/web-config.yml" should include 'cert_file: "/var/lib/kubelet/pki/kubelet-server-current.pem"'
            The contents of file "$TMP_DIR/web-config.yml" should include 'key_file: "/var/lib/kubelet/pki/kubelet-server-current.pem"'
        End

        It 'falls back to static certs when rotation cert is absent'
            export NODE_EXPORTER_TLS_ENABLED="true"
            rm -f /var/lib/kubelet/pki/kubelet-server-current.pem
            mkdir -p /etc/kubernetes/certs
            touch /etc/kubernetes/certs/kubeletserver.crt
            touch /etc/kubernetes/certs/kubeletserver.key

            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include "--web.config.file=${NODE_EXPORTER_TLS_CONFIG_PATH}"
            The contents of file "$TMP_DIR/web-config.yml" should include 'cert_file: "/etc/kubernetes/certs/kubeletserver.crt"'
            The contents of file "$TMP_DIR/web-config.yml" should include 'key_file: "/etc/kubernetes/certs/kubeletserver.key"'
        End

        It 'runs without TLS when enabled but no certs are present after timeout (graceful fallback)'
            export NODE_EXPORTER_TLS_ENABLED="true"
            # No certs created. Override WAIT_TIMEOUT so the test completes quickly.
            export WAIT_TIMEOUT=0
            rm -f /var/lib/kubelet/pki/kubelet-server-current.pem \
                  /etc/kubernetes/certs/kubeletserver.crt \
                  /etc/kubernetes/certs/kubeletserver.key

            When run "$STARTUP_SCRIPT"
            The status should be success
            # Graceful fallback: node-exporter starts without TLS rather than failing.
            The output should include 'WARNING'
            The output should not include '--web.config.file'
        End

        It 'writes client_auth_type from NODE_EXPORTER_TLS_CLIENT_AUTH when set'
            export NODE_EXPORTER_TLS_ENABLED="true"
            export NODE_EXPORTER_TLS_CLIENT_AUTH="RequireAndVerifyClientCert"
            mkdir -p /var/lib/kubelet/pki
            touch /var/lib/kubelet/pki/kubelet-server-current.pem

            When run "$STARTUP_SCRIPT"
            The status should be success
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "RequireAndVerifyClientCert"'
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_ca_file: "/etc/kubernetes/certs/ca.crt"'
        End

        It 'defaults client_auth_type to NoClientCert when NODE_EXPORTER_TLS_CLIENT_AUTH is unset'
            export NODE_EXPORTER_TLS_ENABLED="true"
            unset NODE_EXPORTER_TLS_CLIENT_AUTH
            mkdir -p /var/lib/kubelet/pki
            touch /var/lib/kubelet/pki/kubelet-server-current.pem

            When run "$STARTUP_SCRIPT"
            The status should be success
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "NoClientCert"'
        End

        It 'warns and defaults to NoClientCert when NODE_EXPORTER_TLS_CLIENT_AUTH is invalid'
            export NODE_EXPORTER_TLS_ENABLED="true"
            export NODE_EXPORTER_TLS_CLIENT_AUTH="InvalidValue"
            mkdir -p /var/lib/kubelet/pki
            touch /var/lib/kubelet/pki/kubelet-server-current.pem

            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include 'WARNING'
            The output should include 'NoClientCert'
            The contents of file "$TMP_DIR/web-config.yml" should include 'client_auth_type: "NoClientCert"'
        End
    End

    # -----------------------------------------------------------------------
    # Core args: always present regardless of TLS setting
    # -----------------------------------------------------------------------
    Describe 'base arguments'
        It 'always passes --no-collector.wifi and --no-collector.hwmon'
            unset NODE_EXPORTER_TLS_ENABLED
            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include '--no-collector.wifi'
            The output should include '--no-collector.hwmon'
        End

        It 'always includes --collector.cpu.info'
            unset NODE_EXPORTER_TLS_ENABLED
            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include '--collector.cpu.info'
        End

        It 'appends NODE_EXPORTER_EXTRA_ARGS to the invocation'
            unset NODE_EXPORTER_TLS_ENABLED
            export NODE_EXPORTER_EXTRA_ARGS="--collector.systemd --no-collector.bonding"
            When run "$STARTUP_SCRIPT"
            The status should be success
            The output should include '--collector.systemd'
            The output should include '--no-collector.bonding'
        End
    End
End
