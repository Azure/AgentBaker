# shellcheck shell=bash

Describe 'containerd dynamic system trust'
    # Scheduled refresh must work without the provisioning shell environment.
    Describe 'CA installer error propagation'
        setup() {
            __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
            # No real host trust files are touched in these unit tests.
            mkdir() { :; }
            compgen() { return 0; }
            cp() { :; }
            debug_print_trust_store() { :; }
            update-ca-trust() { return "$UPDATE_STATUS"; }
            update_containerd_ca() {
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
        update_containerd_ca "$ROOT" "$BUNDLE"
        printf 'CA-B\n' > "$TEST_DIR/new.crt"
        mv "$TEST_DIR/new.crt" "$BUNDLE"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
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
        When call update_containerd_ca "$ROOT" "$BUNDLE"
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
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be success
        The contents of file "$ROOT/_default/hosts.toml" should include 'server = "https://mirror.example"'
    End

    It 'upgrades an unchanged older AKS bootstrap mirror and is idempotent'
        mkdir -p "$ROOT/mcr.example"
        printf '%s\n' '[host."https://mirror.example/v2/cache"]' \
            '  capabilities = ["pull", "resolve"]' '  override_path = true' > "$ROOT/mcr.example/hosts.toml"
        update_containerd_ca "$ROOT" "$BUNDLE"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be success
        The line 1 of contents of file "$ROOT/mcr.example/hosts.toml" should eq "ca = \"$BUNDLE\""
        The line 3 of contents of file "$ROOT/mcr.example/hosts.toml" should eq "ca = \"$BUNDLE\""
        The contents of file "$ROOT/mcr.example/hosts.toml" should include 'override_path = true'
    End

    It 'upgrades an unchanged older legacy mirror without losing its header'
        mkdir -p "$ROOT/mcr.azk8s.cn"
        printf '%s\n' '[host."https://mcr.azure.cn"]' \
            '  capabilities = ["pull", "resolve"]' \
            '[host."https://mcr.azure.cn".header]' \
            '    X-Forwarded-For = ["mcr.azk8s.cn"]' > "$ROOT/mcr.azk8s.cn/hosts.toml"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be success
        The line 1 of contents of file "$ROOT/mcr.azk8s.cn/hosts.toml" should eq "ca = \"$BUNDLE\""
        The contents of file "$ROOT/mcr.azk8s.cn/hosts.toml" should include 'X-Forwarded-For = ["mcr.azk8s.cn"]'
    End

    It 'does not rewrite a customized AKS mirror'
        mkdir -p "$ROOT/mcr.example"
        printf '%s\n' '[host."https://mirror.example/v2/cache"]' \
            '  capabilities = ["pull", "resolve"]' '  override_path = true' \
            '  ca = "/custom/root.pem"' > "$ROOT/mcr.example/hosts.toml"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be success
        The contents of file "$ROOT/mcr.example/hosts.toml" should include 'ca = "/custom/root.pem"'
        The contents of file "$ROOT/mcr.example/hosts.toml" should not include 'aks-system-ca'
    End

    It 'refuses a conflicting AKS-owned filename without overwriting it'
        mkdir -p "$ROOT/_default"
        printf 'existing-root\n' > "$ROOT/_default/aks-system-ca.crt"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be failure
        The stderr should include 'aks-system-ca.crt'
        The contents of file "$ROOT/_default/aks-system-ca.crt" should eq 'existing-root'
    End

    Describe 'non-file CA link collisions'
        Parameters
            directory
            dangling
            directory-link
        End

        It 'refuses the collision without following or replacing it'
            mkdir -p "$ROOT/_default" "$TEST_DIR/target"
            case "$1" in
                directory) mkdir "$ROOT/_default/aks-system-ca.crt" ;;
                dangling) ln -s "$TEST_DIR/missing" "$ROOT/_default/aks-system-ca.crt" ;;
                directory-link) ln -s "$TEST_DIR/target" "$ROOT/_default/aks-system-ca.crt" ;;
            esac
            When call update_containerd_ca "$ROOT" "$BUNDLE"
            The status should be failure
            The stderr should include 'aks-system-ca.crt'
            The path "$ROOT/_default/aks-system-ca.crt/system.crt" should not be exist
            The path "$TEST_DIR/target/system.crt" should not be exist
        End
    End

    prepare_mirror() {
        mkdir -p "$ROOT/mcr.example"
        printf '%s\n' '[host."https://mirror.example/v2/cache"]' \
            '  capabilities = ["pull", "resolve"]' '  override_path = true' > "$ROOT/mcr.example/hosts.toml"
        cp "$ROOT/mcr.example/hosts.toml" "$TEST_DIR/original"
    }

    It 'publishes a new inode without mutating old readers or dropping permissions'
        prepare_mirror
        chmod 640 "$ROOT/mcr.example/hosts.toml"
        ln "$ROOT/mcr.example/hosts.toml" "$TEST_DIR/old-reader"
        migrate_and_stat() {
            update_containerd_ca "$ROOT" "$BUNDLE" && stat -c '%a' "$ROOT/mcr.example/hosts.toml"
        }
        When call migrate_and_stat
        The status should be success
        The contents of file "$TEST_DIR/old-reader" should eq "$(cat "$TEST_DIR/original")"
        The contents of file "$ROOT/mcr.example/hosts.toml" should include "ca = \"$BUNDLE\""
        The output should eq 640
    End

    It 'propagates a failed atomic edit and leaves the original template intact'
        prepare_mirror
        sed() { return 7; }
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should eq 7
        The contents of file "$ROOT/mcr.example/hosts.toml" should eq "$(cat "$TEST_DIR/original")"
    End

    It 'propagates a missing or empty bundle error'
        rm "$BUNDLE"
        When call update_containerd_ca "$ROOT" "$BUNDLE"
        The status should be failure
        The stderr should include 'missing system CA bundle'
    End

    Describe 'CA acquisition and refresh control flow'
        # Execute the actual entry-point refresh block, independent of host distro.
        # All acquisition/filesystem/installation commands are stubs. The marker
        # after the block represents continuing into scheduling/full initialization.
        run_refresh() (
            __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
            local location="$1" action="$2"
            OPT_STATUS="$3" RETRIEVE_STATUS="$4" INSTALL_STATUS="$5"
            mkdir() { :; }
            rm() { :; }
            find() { :; }
            emit_event() { :; }
            logs_to_events() { shift; "$@"; }
            is_opted_in_for_root_certs() { return "$OPT_STATUS"; }
            retrieve_legacy_certs() { return "$RETRIEVE_STATUS"; }
            retrieve_rcv1p_certs() { return "$RETRIEVE_STATUS"; }
            install_certs_to_trust_store() { echo installed; return "$INSTALL_STATUS"; }
            # shellcheck disable=SC1090
            . <(awk '/^refresh_location=/{copy=1}
                /^if \[ "\$IS_UBUNTU" -eq 1 \] \|\|/{exit}
                copy' ./parts/linux/cloud-init/artifacts/init-aks-cloud.sh) "$action" "$location"
            echo full-init
        )

        Parameters
            ussec-test
            eastus
        End

        It 'installs for either successful acquisition mode, then exits ca-refresh'
            When run run_refresh "$1" ca-refresh 0 0 0
            The status should be success
            The output should include installed
            The output should not include full-init
        End

        It 'continues to full initialization only after successful installation'
            When run run_refresh "$1" init 0 0 0
            The status should be success
            The output should include installed
            The output should include full-init
        End

        It 'fails either mode when installation fails'
            When run run_refresh "$1" init 0 0 7
            The status should eq 1
            The stderr should include 'failed to install'
            The output should include installed
            The output should not include full-init
        End

        It 'does not install after failed acquisition'
            When run run_refresh "$1" init 0 9 0
            The status should eq 1
            The output should include 'failed to retrieve'
            The output should not include installed
            The output should not include full-init
        End

        It 'does not install or schedule on RCV1P opt-out'
            When run run_refresh eastus init 1 0 0
            The status should be success
            The output should not include installed
            The output should not include full-init
        End

        It 'fails without installing when the opt-in check cannot reach wireserver'
            When run run_refresh eastus init 2 0 0
            The status should eq 1
            The output should include 'wireserver unreachable'
            The output should not include installed
        End
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
