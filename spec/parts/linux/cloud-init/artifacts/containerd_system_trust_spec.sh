# shellcheck shell=bash

Describe 'scheduled containerd trust refresh'
    setup() {
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
        TEST_DIR=$(mktemp -d)
        STATE="$TEST_DIR/state"
        BUNDLE="$TEST_DIR/system.crt"
        CALLS="$TEST_DIR/calls"
        OLD_ID=11111111111111111111111111111111
        NEW_ID=22222222222222222222222222222222
        printf 'CA-A\n' > "$BUNDLE"
        printf '%s\n' "$OLD_ID" > "$TEST_DIR/instance"
        echo active > "$TEST_DIR/active"
        : > "$CALLS"
        CA_AFTER=CA-B
        ACQUIRE_STATUS=0 INSTALL_STATUS=0 RESTART_STATUS=0 READY_STATUS=0 LOCK_STATUS=0 STOP_BEFORE_RESTART=0
        emit_event() { :; }
        system_ca_bundle() { echo "$BUNDLE"; }
        sleep() {
            [ "$1" -ge 0 ] && [ "$1" -le 300 ] || return 8
            echo "delay:$1" >> "$CALLS"
        }
        timeout() { shift; "$@"; }
        flock() { echo lock >> "$CALLS"; return "$LOCK_STATUS"; }
        refresh_certs() {
            echo "acquire:$1" >> "$CALLS"
            [ "$ACQUIRE_STATUS" -eq 0 ] || return "$ACQUIRE_STATUS"
            printf '%s\n' "$CA_AFTER" > "$BUNDLE"
            return "$INSTALL_STATUS"
        }
        systemctl() {
            case "$*" in
                'show containerd --property=ActiveState --value') cat "$TEST_DIR/active" ;;
                'show containerd --property=InvocationID --value') cat "$TEST_DIR/instance" ;;
                'try-restart containerd'|'restart containerd')
                    if [ "$STOP_BEFORE_RESTART" -eq 1 ]; then
                        [ "$1" = try-restart ] || return 8
                        echo inactive > "$TEST_DIR/active"
                        : > "$TEST_DIR/instance"
                        return 0
                    fi
                    if [ "$(cat "$TEST_DIR/active")" = failed ]; then
                        [ "$1" = restart ] || return 8
                    else
                        [ "$1" = try-restart ] || return 8
                    fi
                    echo restart >> "$CALLS"
                    if [ "$RESTART_STATUS" -ne 0 ]; then
                        echo failed > "$TEST_DIR/active"
                        : > "$TEST_DIR/instance"
                        return "$RESTART_STATUS"
                    fi
                    echo active > "$TEST_DIR/active"
                    printf '%s\n' "$NEW_ID" > "$TEST_DIR/instance"
                    ;;
                *) echo "unexpected systemctl: $*" >&2; return 8 ;;
            esac
        }
        containerd_cri_ready() { echo ready >> "$CALLS"; return "$READY_STATUS"; }
    }
    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    run_refresh() { refresh_certs_and_containerd eastus "$STATE"; }

    It 'restarts changed trust, checks CRI, and acknowledges recovery'
        When call run_refresh
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
        The contents of file "$CALLS" should include restart
        The contents of file "$CALLS" should include ready
        The contents of file "$TEST_DIR/instance" should eq "$NEW_ID"
        The path "$STATE/pending" should not be exist
    End

    It 'does not restart on identical regenerated bundle content'
        CA_AFTER=CA-A
        When call run_refresh
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=unchanged'
        The contents of file "$CALLS" should not include restart
        The contents of file "$TEST_DIR/instance" should eq "$OLD_ID"
        The path "$STATE/pending" should not be exist
    End

    It 'supports a caller with nounset enabled'
        run_with_nounset() (set -u; run_refresh)
        When call run_with_nounset
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
    End

    It 'does not start a runtime stopped after its active-state check'
        STOP_BEFORE_RESTART=1
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$TEST_DIR/active" should eq inactive
        The contents of file "$CALLS" should not include restart
        The path "$STATE/pending" should be exist
    End

    It 'is idempotent on the next identical refresh'
        twice() {
            run_refresh >/dev/null || return $?
            run_refresh || return $?
            grep -c '^restart$' "$CALLS"
        }
        When call twice
        The status should be success
        The line 1 should eq 'CA_REFRESH_RESULT=unchanged'
        The line 2 should eq 1
    End

    Describe 'inactive runtime without owned recovery'
        Parameters
            inactive
            failed
        End
        It 'installs trust without starting containerd'
            echo "$1" > "$TEST_DIR/active"
            When call run_refresh
            The status should be success
            The output should eq 'CA_REFRESH_RESULT=inactive'
            The contents of file "$BUNDLE" should eq CA-B
            The contents of file "$CALLS" should not include restart
            The path "$STATE/pending" should not be exist
        End
    End

    It 'does not install or restart on opt-out'
        ACQUIRE_STATUS=3
        When call run_refresh
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=unchanged'
        The contents of file "$BUNDLE" should eq CA-A
        The contents of file "$CALLS" should not include restart
    End

    It 'reports failed acquisition without restarting'
        ACQUIRE_STATUS=9
        When call run_refresh
        The status should eq 9
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$BUNDLE" should eq CA-A
        The contents of file "$CALLS" should not include restart
        The path "$STATE/pending" should be exist
    End

    It 'keeps the original baseline after partial installation failure'
        INSTALL_STATUS=7
        When call run_refresh
        The status should eq 7
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$BUNDLE" should eq CA-B
        The contents of file "$CALLS" should not include restart
        The contents of file "$STATE/pending" should eq "$(printf 'CA-A\n' | sha256sum | cut -d' ' -f1)"
    End

    It 'recovers partial installation even when the next acquisition is identical'
        retry_install() {
            INSTALL_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            INSTALL_STATUS=0
            run_refresh
        }
        When call retry_install
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
        The path "$STATE/pending" should not be exist
    End

    It 'retains recovery state when restart fails'
        RESTART_STATUS=7
        When call run_refresh
        The status should eq 7
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$TEST_DIR/active" should eq failed
        The path "$STATE/pending" should be exist
    End

    It 'retries its own failed runtime with identical certificates'
        retry_restart() {
            RESTART_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            RESTART_STATUS=0
            run_refresh
        }
        When call retry_restart
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
        The path "$STATE/pending" should not be exist
    End

    It 'does not treat daemon activation alone as recovery'
        READY_STATUS=7
        When call run_refresh
        The status should eq 7
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The path "$STATE/pending" should be exist
    End

    It 'acknowledges a previous successful restart without restarting again'
        retry_readiness() {
            READY_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            READY_STATUS=0
            run_refresh || return $?
            grep -c '^restart$' "$CALLS"
        }
        When call retry_readiness
        The status should be success
        The line 1 should eq 'CA_REFRESH_RESULT=recovered'
        The line 2 should eq 1
        The path "$STATE/pending" should not be exist
    End

    It 'does not start a runtime subsequently stopped by its owner'
        stopped_after_failure() {
            RESTART_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            echo inactive > "$TEST_DIR/active"
            RESTART_STATUS=0
            run_refresh
        }
        When call stopped_after_failure
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$TEST_DIR/active" should eq inactive
        The path "$STATE/pending" should be exist
    End

    It 'restarts again if trust changed after a prior unacknowledged restart'
        changed_again() {
            READY_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            READY_STATUS=0
            CA_AFTER=CA-A
            NEW_ID=33333333333333333333333333333333
            run_refresh
        }
        When call changed_again
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
        The path "$STATE/pending" should not be exist
    End

    It 'fails if systemctl did not actually replace the runtime instance'
        NEW_ID="$OLD_ID"
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The path "$STATE/pending" should be exist
    End

    It 'does not replay pending recovery after boot-local state is gone'
        rebooted() {
            READY_STATUS=7
            if run_refresh >/dev/null 2>&1; then return 8; fi
            rm "$STATE/pending"
            READY_STATUS=0
            run_refresh || return $?
            grep -c '^restart$' "$CALLS"
        }
        When call rebooted
        The status should be success
        The line 1 should eq 'CA_REFRESH_RESULT=unchanged'
        The line 2 should eq 1
    End

    It 'fails before acquisition when the refresh lock cannot be obtained'
        LOCK_STATUS=1
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$CALLS" should not include acquire
    End

    It 'applies one bounded delay before the lock and acquisition'
        delay_and_order() {
            run_refresh >/dev/null || return $?
            sed -n '1,3p' "$CALLS"
        }
        When call delay_and_order
        The status should be success
        The line 1 should start with 'delay:'
        The line 2 should eq lock
        The line 3 should eq 'acquire:eastus'
    End

    It 'rejects corrupt pending state without executing its contents'
        mkdir -p "$STATE"
        printf 'touch %s/unsafe\n' "$TEST_DIR" > "$STATE/pending"
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$CALLS" should not include acquire
        The path "$TEST_DIR/unsafe" should not be exist
    End

    It 'preserves custom registry policies including symlinks'
        mkdir -p "$TEST_DIR/certs.d/custom"
        printf 'ca = "/custom/root.pem"\n[host]\n' > "$TEST_DIR/policy"
        ln -s "$TEST_DIR/policy" "$TEST_DIR/certs.d/custom/hosts.toml"
        printf 'private-key\n' > "$TEST_DIR/certs.d/custom/client.key"
        When call run_refresh
        The status should be success
        The output should eq 'CA_REFRESH_RESULT=restarted'
        The path "$TEST_DIR/certs.d/custom/hosts.toml" should be symlink
        The contents of file "$TEST_DIR/policy" should include 'ca = "/custom/root.pem"'
        The contents of file "$TEST_DIR/certs.d/custom/client.key" should eq private-key
    End

    It 'rejects an emptied bundle without restarting'
        refresh_certs() { : > "$BUNDLE"; }
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$CALLS" should not include restart
        The path "$STATE/pending" should be exist
    End

    It 'does not mutate trust if the baseline cannot be read'
        rm "$BUNDLE"
        When call run_refresh
        The status should be failure
        The stderr should include 'scheduled CA refresh or containerd recovery failed'
        The output should be blank
        The contents of file "$CALLS" should not include acquire
    End

    It 'serializes actual concurrent acquisition and restarts only once'
        unset -f flock
        flock_missing() { ! command -v flock >/dev/null; }
        Skip if 'flock is not installed' flock_missing
        refresh_certs() {
            mkdir "$TEST_DIR/acquiring" || return $?
            command sleep 0.2
            printf '%s\n' "$CA_AFTER" > "$BUNDLE"
            rmdir "$TEST_DIR/acquiring"
        }
        concurrent() {
            local first second first_status second_status
            run_refresh > "$TEST_DIR/first" & first=$!
            run_refresh > "$TEST_DIR/second" & second=$!
            wait "$first"; first_status=$?
            wait "$second"; second_status=$?
            [ "$first_status" -eq 0 ] && [ "$second_status" -eq 0 ] || return 8
            grep -c '^restart$' "$CALLS"
            cat "$TEST_DIR/first" "$TEST_DIR/second" | sort
        }
        When call concurrent
        The status should be success
        The line 1 should eq 1
        The line 2 should eq 'CA_REFRESH_RESULT=restarted'
        The line 3 should eq 'CA_REFRESH_RESULT=unchanged'
        The path "$STATE/pending" should not be exist
    End
End

Describe 'bounded CRI runtime readiness'
    setup() {
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
        TEST_DIR=$(mktemp -d)
        CALLS="$TEST_DIR/calls"
        : > "$CALLS"
        READY=true
        CRI_STATUS=0
        sleep() { :; }
        systemctl() {
            [ "$*" = 'is-active --quiet containerd' ] || return 8
        }
        timeout() {
            [ "$1" = 5 ] || return 8
            shift
            "$@"
        }
        crictl() {
            [ "$*" = '--runtime-endpoint unix:///run/containerd/containerd.sock info' ] || return 8
            echo cri >> "$CALLS"
            [ "$CRI_STATUS" -eq 0 ] || return "$CRI_STATUS"
            printf '{"status":{"conditions":[{"type":"RuntimeReady","status":%s}]}}\n' "$READY"
        }
    }
    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'requires CRI RuntimeReady on the containerd socket'
        When call containerd_cri_ready
        The status should be success
        The output should be blank
        The contents of file "$CALLS" should eq cri
    End

    It 'stops polling after twelve unsuccessful probes'
        READY=false
        not_ready() {
            containerd_cri_ready
            local rc=$?
            grep -c '^cri$' "$CALLS"
            return "$rc"
        }
        When call not_ready
        The status should be failure
        The stderr should include 'containerd CRI did not become ready'
        The output should eq 12
    End

    It 'does not treat a failed CRI command as healthy'
        CRI_STATUS=7
        When call containerd_cri_ready
        The status should be failure
        The stderr should include 'containerd CRI did not become ready'
        The output should be blank
    End
End

Describe 'CA installation and acquisition'
    setup() {
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
        mkdir() { :; }
        rm() { :; }
        cp() { return "$COPY_STATUS"; }
        compgen() { return "$GLOB_STATUS"; }
        debug_print_trust_store() { :; }
        update-ca-trust() { return "$UPDATE_STATUS"; }
        systemctl() { echo 'unexpected runtime operation' >&2; return 8; }
        emit_event() { :; }
        logs_to_events() { shift; "$@"; }
        IS_AZURELINUX=1
        COPY_STATUS=0 UPDATE_STATUS=0 GLOB_STATUS=0
    }
    BeforeEach 'setup'

    It 'installs OS trust without a runtime operation during provisioning'
        When call install_certs_to_trust_store
        The status should be success
        The output should be blank
    End

    It 'propagates copy failures'
        COPY_STATUS=7
        When call install_certs_to_trust_store
        The status should eq 7
        The output should be blank
    End

    It 'propagates OS update failures rather than diagnostic status'
        UPDATE_STATUS=9
        When call install_certs_to_trust_store
        The status should eq 9
        The output should be blank
    End

    It 'rejects an empty download'
        GLOB_STATUS=1
        When call install_certs_to_trust_store
        The status should be failure
        The stderr should include 'no *.crt files'
    End

    Describe 'real acquisition function'
        BeforeEach 'stub_acquisition'
        stub_acquisition() {
            OPT_STATUS=0 RETRIEVE_STATUS=0 INSTALL_STATUS=0
            find() { :; }
            is_opted_in_for_root_certs() { return "$OPT_STATUS"; }
            retrieve_legacy_certs() { return "$RETRIEVE_STATUS"; }
            retrieve_rcv1p_certs() { return "$RETRIEVE_STATUS"; }
            install_certs_to_trust_store() { echo installed; return "$INSTALL_STATUS"; }
        }
        Parameters
            ussec-test
            eastus
        End

        It 'installs after acquisition succeeds'
            When call refresh_certs "$1"
            The status should be success
            The output should include installed
        End

        It 'does not install when acquisition fails'
            RETRIEVE_STATUS=9
            When call refresh_certs "$1"
            The status should be failure
            The output should include 'failed to retrieve'
            The output should not include installed
        End

        It 'does not hide installation failure'
            INSTALL_STATUS=7
            When call refresh_certs "$1"
            The status should be failure
            The stderr should include 'failed to install'
            The output should include installed
        End

        It 'returns the explicit opt-out status without installation'
            OPT_STATUS=1
            When call refresh_certs eastus
            The status should eq 3
            The output should not include installed
        End

        It 'fails when wireserver opt-in cannot be determined'
            OPT_STATUS=2
            When call refresh_certs eastus
            The status should be failure
            The output should include 'wireserver unreachable'
            The output should not include installed
        End
    End
End

Describe 'generated containerd mirrors retain implicit system trust'
    setup() {
        . ./parts/linux/cloud-init/artifacts/cse_config.sh
        TEST_DIR=$(mktemp -d)
        mkdir() { :; }
        touch() { :; }
        chmod() { :; }
        tee() { command cat > "$TEST_DIR/hosts.toml"; }
    }
    cleanup() { command rm -rf "$TEST_DIR"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'preserves bootstrap routing without overriding root selection'
        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="mirror.example/cache"
        When call configureContainerdRegistryHost
        The status should be success
        The contents of file "$TEST_DIR/hosts.toml" should include '[host."https://mirror.example/v2/cache"]'
        The contents of file "$TEST_DIR/hosts.toml" should include '  override_path = true'
        The contents of file "$TEST_DIR/hosts.toml" should include '  capabilities = ["pull", "resolve"]'
        The contents of file "$TEST_DIR/hosts.toml" should not include 'ca ='
        The contents of file "$TEST_DIR/hosts.toml" should not include skip_verify
    End

    It 'preserves the legacy mirror header and implicit fallback'
        When call configureContainerdLegacyMooncakeMcrHost
        The status should be success
        The contents of file "$TEST_DIR/hosts.toml" should include '[host."https://mcr.azure.cn"]'
        The contents of file "$TEST_DIR/hosts.toml" should include 'X-Forwarded-For = ["mcr.azk8s.cn"]'
        The contents of file "$TEST_DIR/hosts.toml" should not include 'ca ='
        The contents of file "$TEST_DIR/hosts.toml" should not include skip_verify
    End
End
