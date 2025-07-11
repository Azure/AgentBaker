#!/bin/bash

Describe 'measure-tls-bootstrapping-latency.sh'
    Include "./parts/linux/cloud-init/artifacts/measure-tls-bootstrapping-latency.sh"

    KUBECONFIG_PATH="spec-test/kubeconfig"
    KUBECONFIG_DIR="$(dirname "$KUBECONFIG_PATH")"
    WATCH_TIMEOUT_SECONDS=3

    createGuestAgentEvent() {
        echo "createGuestAgentEvent $@"
    }
    command() {
        return 0
    }
    kill() {
        echo "kill $@"
    }
    cleanup() {
        rm -rf $KUBECONFIG_DIR
    }

    AfterEach 'cleanup'

    It 'should exit 0 if inotifywait command is unavailable'
        command() {
            return 1
        }

        When run waitForTLSBootstrapping
        The stdout should include 'notifywait is not available, unable to wait for TLS bootstrapping'
        The status should be success
    End

    It 'should exit 0 if KUBECONFIG_PATH already exists'
        mkdir -p "$(dirname "$KUBECONFIG_PATH")"
        touch $KUBECONFIG_PATH

        When run waitForTLSBootstrapping
        The stdout should include 'kubeconfig already exists at: spec-test/kubeconfig'
        The status should be success
    End

    It 'should create a guest agent event when a kubeconfig is created'
        inotifywait() {
            echo "$KUBECONFIG_DIR/ CREATE kubeconfig"
            return 0
        }

        When run waitForTLSBootstrapping
        The stdout should include 'watching for kubeconfig to be created at spec-test/kubeconfig with 3s timeout...'
        The stdout should include 'new kubeconfig created at: spec-test/kubeconfig'
        The stdout should include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrapping'
        The stdout should include 'kill -- -'
        The status should be success
    End

    It 'should not create a guest agent event if a kubeconfig is never created'
        inotifywait() {
            echo "$KUBECONFIG_DIR/ CREATE other-file"
            return 0
        }

        When run waitForTLSBootstrapping
        The stdout should include 'watching for kubeconfig to be created at spec-test/kubeconfig with 3s timeout...'
        The stdout should include 'kubeconfig was not created after 3s'
        The stdout should include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrappingTimeout'
        The stdout should not include 'kill -- -'
        The status should be success
    End

    It 'should not create a guest agent event if inotifywait times out without observing anything'
        inotifywait() {
            sleep $WATCH_TIMEOUT_SECONDS
            return 0
        }

        When run waitForTLSBootstrapping
        The stdout should include 'watching for kubeconfig to be created at spec-test/kubeconfig with 3s timeout...'
        The stdout should include 'kubeconfig was not created after 3s'
        The stdout should include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrappingTimeout'
        The stdout should not include 'kill -- -'
        The status should be success
    End

    It 'should not create a guest agent event if kubeconfig creation was never observed, but did occur due to race condition'
        inotifywait() {
            touch $KUBECONFIG_PATH
            echo "$KUBECONFIG_DIR/ CREATE other-file"
            return 0
        }

        When run waitForTLSBootstrapping
        The stdout should include 'watching for kubeconfig to be created at spec-test/kubeconfig with 3s timeout...'
        The stdout should include 'kubeconfig now exists at: spec-test/kubeconfig'
        The stdout should not include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrapping'
        The stdout should not include 'kill -- -'
        The status should be success
    End
End