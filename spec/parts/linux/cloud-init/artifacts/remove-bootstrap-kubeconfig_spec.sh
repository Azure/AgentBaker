#!/bin/bash
# shellcheck disable=SC2317

Describe 'remove-bootstrap-kubeconfig.sh'
    Include "./parts/linux/cloud-init/artifacts/remove-bootstrap-kubeconfig.sh"

    setup() {
        TEST_DIR=$(mktemp -d)
        KUBECONFIG_PATH="${TEST_DIR}/kubeconfig with spaces"
        BOOTSTRAP_KUBECONFIG_PATH="${TEST_DIR}/bootstrap kubeconfig"
        EVENTS_LOGGING_DIR="${TEST_DIR}/events/"
        mkdir -p "$EVENTS_LOGGING_DIR"
        SLEEP_CALLS=0
    }

    cleanup() {
        if [ -d "$KUBECONFIG_PATH" ]; then
            rmdir "$KUBECONFIG_PATH"
        fi
        if [ -d "$BOOTSTRAP_KUBECONFIG_PATH" ]; then
            rmdir "$BOOTSTRAP_KUBECONFIG_PATH"
        fi
        command rm -f "$KUBECONFIG_PATH" "$BOOTSTRAP_KUBECONFIG_PATH" "${EVENTS_LOGGING_DIR}"*.json
        rmdir "$EVENTS_LOGGING_DIR" "$TEST_DIR"
    }

    sleep() {
        echo "unexpected sleep: $*" >&2
        exit 99
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'removeBootstrapKubeconfig'
        It 'returns immediately when neither kubeconfig exists'
            When run removeBootstrapKubeconfig
            The status should be success
            The output should be blank
            The path "$KUBECONFIG_PATH" should not be exist
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
        End

        It 'preserves the kubeconfig when the bootstrap kubeconfig is absent'
            printf '%s\n' 'client credentials' > "$KUBECONFIG_PATH"

            When run removeBootstrapKubeconfig
            The status should be success
            The output should be blank
            The contents of file "$KUBECONFIG_PATH" should equal 'client credentials'
        End

        It 'removes only the bootstrap kubeconfig without sleeping when kubeconfig exists'
            printf '%s\n' 'client credentials' > "$KUBECONFIG_PATH"
            printf '%s\n' 'bootstrap credentials' > "$BOOTSTRAP_KUBECONFIG_PATH"

            When run removeBootstrapKubeconfig
            The status should be success
            The output should be blank
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The contents of file "$KUBECONFIG_PATH" should equal 'client credentials'
        End

        It 'preserves bootstrap credentials while polling every second until kubeconfig exists'
            printf '%s\n' 'bootstrap credentials' > "$BOOTSTRAP_KUBECONFIG_PATH"
            sleep() {
                [ "$#" -eq 1 ] && [ "$1" = 1 ] || exit 98
                [ "$(cat "$BOOTSTRAP_KUBECONFIG_PATH")" = 'bootstrap credentials' ] || exit 97
                SLEEP_CALLS=$((SLEEP_CALLS + 1))
                [ "$SLEEP_CALLS" -le 3 ] || exit 96
                if [ "$SLEEP_CALLS" -eq 3 ]; then
                    printf '%s\n' 'client credentials' > "$KUBECONFIG_PATH"
                fi
            }

            When call removeBootstrapKubeconfig
            The status should be success
            The output should be blank
            The variable SLEEP_CALLS should equal 3
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The contents of file "$KUBECONFIG_PATH" should equal 'client credentials'
        End

        It 'propagates deletion errors and leaves credentials intact'
            touch "$KUBECONFIG_PATH" "$BOOTSTRAP_KUBECONFIG_PATH"
            rm() {
                [ "$#" -eq 2 ] && [ "$1" = -f ] && [ "$2" = "$BOOTSTRAP_KUBECONFIG_PATH" ] || return 98
                echo 'cannot remove bootstrap kubeconfig' >&2
                return 13
            }

            When run removeBootstrapKubeconfig
            The status should equal 13
            The stdout should be blank
            The stderr should equal 'cannot remove bootstrap kubeconfig'
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should be file
            The path "$KUBECONFIG_PATH" should be file
        End

        It 'succeeds when called again after cleanup'
            touch "$KUBECONFIG_PATH" "$BOOTSTRAP_KUBECONFIG_PATH"
            remove_twice() {
                removeBootstrapKubeconfig && removeBootstrapKubeconfig
            }

            When run remove_twice
            The status should be success
            The output should be blank
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The path "$KUBECONFIG_PATH" should be file
        End
    End
End
