#!/bin/bash

Describe 'measure-tls-bootstrapping-latency.sh'
    Include "./parts/linux/cloud-init/artifacts/measure-tls-bootstrapping-latency.sh"

    KUBECONFIG_PATH="spec-test/kubeconfig"
    KUBECONFIG_DIR="$(dirname "$KUBECONFIG_PATH")"
    TLS_BOOTSTRAPPING_START_TIME_FILEPATH="spec-test/tls-bootstrap-start-time"
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
    writeStartTimeFile() {
        mkdir -p "$(dirname "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH")"
        echo "2026-03-17 00:00:00.000" > "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH"
    }
    cleanup() {
        rm -rf $KUBECONFIG_DIR "$(dirname "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH")"
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

    It 'should exit 0 if TLS bootstrapping start time file does not exist'
        When run waitForTLSBootstrapping
        The stdout should include 'TLS bootstrapping start time file not found at: spec-test/tls-bootstrap-start-time'
        The status should be success
    End

    It 'should exit 0 if KUBECONFIG_PATH already exists'
        writeStartTimeFile
        mkdir -p "$(dirname "$KUBECONFIG_PATH")"
        touch $KUBECONFIG_PATH

        When run waitForTLSBootstrapping
        The stdout should include 'kubeconfig already exists at: spec-test/kubeconfig'
        The stdout should include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrapping 2026-03-17 00:00:00.000'
        The status should be success
    End

    It 'should create a guest agent event when a kubeconfig is created'
        writeStartTimeFile
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
        writeStartTimeFile
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
        writeStartTimeFile
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

    It 'should create a guest agent event if kubeconfig creation was never observed, but did occur due to race condition'
        writeStartTimeFile
        inotifywait() {
            touch $KUBECONFIG_PATH
            echo "$KUBECONFIG_DIR/ CREATE other-file"
            return 0
        }

        When run waitForTLSBootstrapping
        The stdout should include 'watching for kubeconfig to be created at spec-test/kubeconfig with 3s timeout...'
        The stdout should include 'kubeconfig now exists at: spec-test/kubeconfig'
        The stdout should include 'createGuestAgentEvent AKS.Runtime.waitForTLSBootstrapping 2026-03-17 00:00:00.000'
        The stdout should not include 'kill -- -'
        The status should be success
    End
End
