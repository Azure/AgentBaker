#!/bin/bash
# shellcheck disable=SC2317

Describe 'remove-bootstrap-kubeconfig.sh'
    Include "./parts/linux/cloud-init/artifacts/remove-bootstrap-kubeconfig.sh"

    setup() {
        TEST_DIR=$(mktemp -d)
        KUBECONFIG_PATH="${TEST_DIR}/kubeconfig with spaces"
        BOOTSTRAP_KUBECONFIG_PATH="${TEST_DIR}/bootstrap kubeconfig"
        KUBELET_CLIENT_CURRENT_PEM_PATH="${TEST_DIR}/kubelet-client-current.pem"
        CLIENT_CERT_PATH="${TEST_DIR}/client certificate.pem"
        SLEEP_CALLS=0
    }

    cleanup() {
        if [ ! -L "$KUBELET_CLIENT_CURRENT_PEM_PATH" ] && [ -d "$KUBELET_CLIENT_CURRENT_PEM_PATH" ]; then
            rmdir "$KUBELET_CLIENT_CURRENT_PEM_PATH"
        fi
        command rm -f "$KUBECONFIG_PATH" "$BOOTSTRAP_KUBECONFIG_PATH" "$KUBELET_CLIENT_CURRENT_PEM_PATH" "$CLIENT_CERT_PATH"
        rmdir "$TEST_DIR"
    }

    create_client_certificate() {
        printf '%s\n' 'client credentials' > "$CLIENT_CERT_PATH"
        ln -s "$CLIENT_CERT_PATH" "$KUBELET_CLIENT_CURRENT_PEM_PATH"
    }

    sleep() {
        echo "unexpected sleep: $*" >&2
        exit 99
    }

    missing_bootstrap_message() {
        echo "$BOOTSTRAP_KUBECONFIG_PATH does not exist, nothing to remove"
    }

    waiting_message() {
        echo "$KUBELET_CLIENT_CURRENT_PEM_PATH does not yet exist a file or valid symlink, waiting 1s..."
    }

    removing_message() {
        echo "$KUBELET_CLIENT_CURRENT_PEM_PATH exists, removing $BOOTSTRAP_KUBECONFIG_PATH..."
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'removeBootstrapKubeconfig'
        It 'returns immediately when bootstrap kubeconfig and the certificate symlink are absent'
            When run removeBootstrapKubeconfig
            The status should be success
            The output should equal "$(missing_bootstrap_message)"
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should not be exist
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
        End

        It 'preserves the certificate and symlink when the bootstrap kubeconfig is absent'
            create_client_certificate

            When run removeBootstrapKubeconfig
            The status should be success
            The output should equal "$(missing_bootstrap_message)"
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
            The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
            The contents of file "$CLIENT_CERT_PATH" should equal 'client credentials'
        End

        It 'removes only the bootstrap kubeconfig without sleeping when the certificate symlink resolves to a file'
            create_client_certificate
            printf '%s\n' 'kubelet configuration' > "$KUBECONFIG_PATH"
            printf '%s\n' 'bootstrap credentials' > "$BOOTSTRAP_KUBECONFIG_PATH"

            When run removeBootstrapKubeconfig
            The status should be success
            The output should equal "$(removing_message)"
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
            The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
            The contents of file "$CLIENT_CERT_PATH" should equal 'client credentials'
            The contents of file "$KUBECONFIG_PATH" should equal 'kubelet configuration'
        End

        It 'waits for the certificate symlink even when kubeconfig already exists'
            printf '%s\n' 'kubelet configuration' > "$KUBECONFIG_PATH"
            printf '%s\n' 'bootstrap credentials' > "$BOOTSTRAP_KUBECONFIG_PATH"
            sleep() {
                [ "$#" -eq 1 ] && [ "$1" = 1 ] || exit 98
                [ "$(cat "$BOOTSTRAP_KUBECONFIG_PATH")" = 'bootstrap credentials' ] || exit 97
                SLEEP_CALLS=$((SLEEP_CALLS + 1))
                [ "$SLEEP_CALLS" -le 3 ] || exit 96
                if [ "$SLEEP_CALLS" -eq 3 ]; then
                    create_client_certificate
                fi
            }

            When call removeBootstrapKubeconfig
            The status should be success
            The output should equal "$(printf '%s\n%s\n%s\n%s' "$(waiting_message)" "$(waiting_message)" "$(waiting_message)" "$(removing_message)")"
            The variable SLEEP_CALLS should equal 3
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
            The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
            The contents of file "$KUBECONFIG_PATH" should equal 'kubelet configuration'
        End

        Context 'when the certificate path is not a symlink to an existing file'
            Parameters
                'regular file'
                'directory'
                'dangling symlink'
                'symlink to directory'
            End

            It "keeps bootstrap credentials while the certificate path is a $1"
                printf '%s\n' 'bootstrap credentials' > "$BOOTSTRAP_KUBECONFIG_PATH"
                case "$1" in
                    'regular file')
                        printf '%s\n' 'client credentials' > "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        ;;
                    'directory')
                        mkdir "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        ;;
                    'dangling symlink')
                        ln -s "$CLIENT_CERT_PATH" "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        ;;
                    'symlink to directory')
                        ln -s "$TEST_DIR" "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        ;;
                esac
                sleep() {
                    [ "$#" -eq 1 ] && [ "$1" = 1 ] || exit 98
                    [ "$(cat "$BOOTSTRAP_KUBECONFIG_PATH")" = 'bootstrap credentials' ] || exit 97
                    SLEEP_CALLS=$((SLEEP_CALLS + 1))
                    [ "$SLEEP_CALLS" -eq 1 ] || exit 96
                    if [ -L "$KUBELET_CLIENT_CURRENT_PEM_PATH" ]; then
                        if [ -d "$KUBELET_CLIENT_CURRENT_PEM_PATH" ]; then
                            command rm -f "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                            ln -s "$CLIENT_CERT_PATH" "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        fi
                        printf '%s\n' 'client credentials' > "$CLIENT_CERT_PATH"
                    else
                        if [ -d "$KUBELET_CLIENT_CURRENT_PEM_PATH" ]; then
                            rmdir "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        else
                            command rm -f "$KUBELET_CLIENT_CURRENT_PEM_PATH"
                        fi
                        create_client_certificate
                    fi
                }

                When call removeBootstrapKubeconfig
                The status should be success
                The output should equal "$(printf '%s\n%s' "$(waiting_message)" "$(removing_message)")"
                The variable SLEEP_CALLS should equal 1
                The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
                The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
                The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
                The contents of file "$CLIENT_CERT_PATH" should equal 'client credentials'
            End
        End

        It 'propagates deletion errors and leaves credentials intact'
            create_client_certificate
            touch "$BOOTSTRAP_KUBECONFIG_PATH"
            rm() {
                [ "$#" -eq 2 ] && [ "$1" = -f ] && [ "$2" = "$BOOTSTRAP_KUBECONFIG_PATH" ] || return 98
                echo 'cannot remove bootstrap kubeconfig' >&2
                return 13
            }

            When run removeBootstrapKubeconfig
            The status should equal 13
            The stdout should equal "$(removing_message)"
            The stderr should equal 'cannot remove bootstrap kubeconfig'
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should be file
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
            The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
            The contents of file "$CLIENT_CERT_PATH" should equal 'client credentials'
        End

        It 'succeeds when called again after cleanup'
            create_client_certificate
            touch "$BOOTSTRAP_KUBECONFIG_PATH"
            remove_twice() {
                removeBootstrapKubeconfig && removeBootstrapKubeconfig
            }

            When run remove_twice
            The status should be success
            The output should equal "$(printf '%s\n%s' "$(removing_message)" "$(missing_bootstrap_message)")"
            The path "$BOOTSTRAP_KUBECONFIG_PATH" should not be exist
            The path "$KUBELET_CLIENT_CURRENT_PEM_PATH" should be symlink
            The contents of file "$KUBELET_CLIENT_CURRENT_PEM_PATH" should equal 'client credentials'
            The contents of file "$CLIENT_CERT_PATH" should equal 'client credentials'
        End
    End
End
