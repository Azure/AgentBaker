#!/bin/bash

Describe 'validate-kubelet-credentials.sh'
    Include "./parts/linux/cloud-init/artifacts/validate-kubelet-credentials.sh"

    KUBECONFIG_PATH="mock-kubeconfig"

    sleep() {
        echo "sleep $@"
    }
    cleanup() {
        rm -f $KUBECONFIG_PATH
    }

    AfterEach 'cleanup'
    
    It 'should exit 0 if CREDENTIAL_VALIDATION_KUBE_CA_FILE is not set'
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'CREDENTIAL_VALIDATION_KUBE_CA_FILE is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN is not set'
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if CREDENTIAL_VALIDATION_APISERVER_URL is not set'
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        When call validateKubeletCredentials
        The stdout should include 'CREDENTIAL_VALIDATION_APISERVER_URL is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if a kubeconfig already exists'
        KUBECONFIG_PATH="mock-kubeconfig"
        touch $KUBECONFIG_PATH
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'client credential already exists within kubeconfig: mock-kubeconfig, no need to validate bootstrap credentials'
        The status should be success
    End

    It 'should exit 0 if the curl command is unavailable'
        command() {
            return 1
        }
        KUBECONFIG_PATH="mock-kubeconfig"
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'curl is not available, unable to validate bootstrap credentials'
        The status should be success
    End

    It 'should exit 0 immediately if the bootstrap token is valid'
        command() {
            return 0
        }
        curl() {
            echo "200"
        }
        KUBECONFIG_PATH="mock-kubeconfig"
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'will validate kubelet bootstrap credentials'
        The stdout should include 'will check credential validity against apiserver url: url'
        The stdout should include '(retry=0) received valid HTTP status code from apiserver: 200'
        The stdout should include 'kubelet bootstrap token credential is valid'
        The status should be success
    End

    It 'should exit 0 even if all retires are exhausted'
        command() {
            return 0
        }
        curl() {
            echo "401"
        }

        KUBECONFIG_PATH="mock-kubeconfig"
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        MAX_RETRIES=2
        When call validateKubeletCredentials
        The stdout should include 'will validate kubelet bootstrap credentials'
        The stdout should include 'will check credential validity against apiserver url: url'
        The stdout should include '(retry=0) received invalid HTTP status code from apiserver: 401'
        The stdout should include 'sleep 2'
        The stdout should include '(retry=0) received invalid HTTP status code from apiserver: 401'
        The stdout should include 'unable to validate bootstrap credentials after 2 attempts'
        The stdout should include 'proceeding to start kubelet...'
        The status should be success
    End

    It 'should exit 0 after retrying curl once if the initial status is 000'
        command() {
            return 0
        }
        curl() {
            echo "000"
            return 6
        }

        KUBECONFIG_PATH="mock-kubeconfig"
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_TLS_BOOTSTRAP_TOKEN="token"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"
        When call validateKubeletCredentials
        The stdout should include 'will validate kubelet bootstrap credentials'
        The stdout should include 'will check credential validity against apiserver url: url'
        The stdout should include '(retry=0) curl response code is 000, curl exited with code: 6'
        The stdout should include 'retrying once more to get a more detailed error response...'
        The stdout should include '000'
        The stdout should include 'proceeding to start kubelet...'
        The stdout should not include 'sleep'
        The status should be success
    End
End
