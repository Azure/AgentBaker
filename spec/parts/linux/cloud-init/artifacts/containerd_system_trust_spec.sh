# shellcheck shell=bash

Describe 'containerd dynamic system trust'
    # Scheduled refresh must work without the provisioning shell environment.
    Parameters
        init-aks-cloud.sh configure_containerd_system_trust
    End

    Describe 'CA installer error propagation'
        setup() {
            __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
            # No real host trust files are touched in these unit tests.
            mkdir() { :; }
            compgen() { return 0; }
            cp() { :; }
            debug_print_trust_store() { :; }
            update-ca-trust() { return "$UPDATE_STATUS"; }
            configure_containerd_system_trust() {
                echo reconciled
                return "$TRUST_STATUS"
            }
            IS_AZURELINUX=1
            UPDATE_STATUS=0
            TRUST_STATUS=0
        }
        BeforeEach 'setup'

        It 'reconciles runtime trust after the OS update succeeds'
            When call install_certs_to_trust_store
            The status should be success
            The output should eq 'reconciled'
        End

        It 'does not hide trust integration failures with diagnostic output'
            TRUST_STATUS=7
            When call install_certs_to_trust_store
            The status should eq 7
            The output should eq 'reconciled'
        End

        It 'does not reconcile after the OS update failed'
            UPDATE_STATUS=9
            When call install_certs_to_trust_store
            The status should eq 9
            The output should be blank
        End
    End

    setup() {
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
        TEST_DIR=$(mktemp -d)
        ROOT="$TEST_DIR/certs.d"
        BUNDLE="$TEST_DIR/system.crt"
        printf 'CA-A\n' > "$BUNDLE"
    }
    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'reads the current bundle after replacement, without copying roots or restarting'
        "$2" "$ROOT" "$BUNDLE"
        printf 'CA-B\n' > "$TEST_DIR/new.crt"
        mv "$TEST_DIR/new.crt" "$BUNDLE"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The path "$ROOT/_default/aks-system-ca.crt" should be symlink
        The contents of file "$ROOT/_default/aks-system-ca.crt" should eq 'CA-B'
        The path "$ROOT/_default/hosts.toml" should not be exist
    End

    It 'preserves explicit host policies, custom roots, and client keys'
        mkdir -p "$ROOT/custom.example" "$ROOT/docker.example"
        printf 'ca = "custom.pem"\n[host]\n' > "$ROOT/custom.example/hosts.toml"
        printf 'private-client-key\n' > "$ROOT/docker.example/client.key"
        printf 'custom-root\n' > "$ROOT/docker.example/custom.crt"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The contents of file "$ROOT/custom.example/hosts.toml" should include 'ca = "custom.pem"'
        The path "$ROOT/custom.example/aks-system-ca.crt" should not be exist
        The contents of file "$ROOT/docker.example/custom.crt" should eq 'custom-root'
        The contents of file "$ROOT/docker.example/client.key" should eq 'private-client-key'
        The path "$ROOT/docker.example/aks-system-ca.crt" should be symlink
    End

    It 'does not replace an existing default TOML policy'
        mkdir -p "$ROOT/_default"
        printf 'server = "https://mirror.example"\n[host]\n' > "$ROOT/_default/hosts.toml"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The contents of file "$ROOT/_default/hosts.toml" should include 'server = "https://mirror.example"'
    End

    It 'upgrades an unchanged older AKS bootstrap mirror and is idempotent'
        mkdir -p "$ROOT/mcr.example"
        printf '%s\n' '[host."https://mirror.example/v2/cache"]' \
            '  capabilities = ["pull", "resolve"]' '  override_path = true' > "$ROOT/mcr.example/hosts.toml"
        "$2" "$ROOT" "$BUNDLE"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The line 1 of contents of file "$ROOT/mcr.example/hosts.toml" should eq "ca = \"$ROOT/_default/aks-system-ca.crt\""
        The line 3 of contents of file "$ROOT/mcr.example/hosts.toml" should eq "  ca = \"$ROOT/_default/aks-system-ca.crt\""
        The contents of file "$ROOT/mcr.example/hosts.toml" should include 'override_path = true'
    End

    It 'upgrades an unchanged older legacy mirror without losing its header'
        mkdir -p "$ROOT/mcr.azk8s.cn"
        printf '%s\n' '[host."https://mcr.azure.cn"]' \
            '  capabilities = ["pull", "resolve"]' \
            '[host."https://mcr.azure.cn".header]' \
            '    X-Forwarded-For = ["mcr.azk8s.cn"]' > "$ROOT/mcr.azk8s.cn/hosts.toml"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The line 1 of contents of file "$ROOT/mcr.azk8s.cn/hosts.toml" should eq "ca = \"$ROOT/_default/aks-system-ca.crt\""
        The contents of file "$ROOT/mcr.azk8s.cn/hosts.toml" should include 'X-Forwarded-For = ["mcr.azk8s.cn"]'
    End

    It 'does not rewrite a customized AKS mirror'
        mkdir -p "$ROOT/mcr.example"
        printf '%s\n' '[host."https://mirror.example/v2/cache"]' \
            '  capabilities = ["pull", "resolve"]' '  override_path = true' \
            '  ca = "/custom/root.pem"' > "$ROOT/mcr.example/hosts.toml"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be success
        The contents of file "$ROOT/mcr.example/hosts.toml" should include 'ca = "/custom/root.pem"'
        The contents of file "$ROOT/mcr.example/hosts.toml" should not include 'aks-system-ca'
    End

    It 'refuses a conflicting AKS-owned filename without overwriting it'
        mkdir -p "$ROOT/_default"
        printf 'existing-root\n' > "$ROOT/_default/aks-system-ca.crt"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be failure
        The stderr should include 'refusing to replace'
        The contents of file "$ROOT/_default/aks-system-ca.crt" should eq 'existing-root'
    End

    It 'propagates a missing or empty bundle error'
        rm "$BUNDLE"
        When call "$2" "$ROOT" "$BUNDLE"
        The status should be failure
        The stderr should include 'missing system CA bundle'
    End
End

Describe 'generated containerd mirror trust'
    setup() {
        . ./parts/linux/cloud-init/artifacts/cse_config.sh
        TEST_DIR=$(mktemp -d)
        EXPECTED_BUNDLE=/etc/ssl/certs/ca-certificates.crt
        [ -s "$EXPECTED_BUNDLE" ] || EXPECTED_BUNDLE=/etc/pki/tls/certs/ca-bundle.crt
        mkdir() { :; }
        touch() { :; }
        chmod() { :; }
        tee() { command cat > "$TEST_DIR/hosts.toml"; }
    }
    cleanup() { command rm -rf "$TEST_DIR"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'configures both the bootstrap mirror and implicit server without losing path semantics'
        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="mirror.example/cache"
        When call configureContainerdRegistryHost
        The status should be success
        The line 1 of contents of file "$TEST_DIR/hosts.toml" should eq "ca = \"$EXPECTED_BUNDLE\""
        The contents of file "$TEST_DIR/hosts.toml" should include '[host."https://mirror.example/v2/cache"]'
        The contents of file "$TEST_DIR/hosts.toml" should include "  ca = \"$EXPECTED_BUNDLE\""
        The contents of file "$TEST_DIR/hosts.toml" should include '  override_path = true'
        The contents of file "$TEST_DIR/hosts.toml" should include '  capabilities = ["pull", "resolve"]'
    End

    It 'preserves the legacy mirror header and implicit fallback'
        When call configureContainerdLegacyMooncakeMcrHost
        The status should be success
        The line 1 of contents of file "$TEST_DIR/hosts.toml" should eq "ca = \"$EXPECTED_BUNDLE\""
        The contents of file "$TEST_DIR/hosts.toml" should include '[host."https://mcr.azure.cn"]'
        The contents of file "$TEST_DIR/hosts.toml" should include "  ca = \"$EXPECTED_BUNDLE\""
        The contents of file "$TEST_DIR/hosts.toml" should include 'X-Forwarded-For = ["mcr.azk8s.cn"]'
        The contents of file "$TEST_DIR/hosts.toml" should not include 'skip_verify'
    End
End
