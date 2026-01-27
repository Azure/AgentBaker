#!/bin/bash

Describe 'validate-kubelet-credentials.sh'
    Include "./parts/linux/cloud-init/artifacts/validate-kubelet-credentials.sh"

    MAX_RETRIES=2
    MOCK_BOOTSTRAP_KUBECONFIG_PATH="mock-bootstrap-kubeconfig"
    MOCK_KUBECONFIG_PATH="mock-kubeconfig"
    
    setup() {
        KUBECONFIG_PATH=$MOCK_KUBECONFIG_PATH
        BOOTSTRAP_KUBECONFIG_PATH=$MOCK_BOOTSTRAP_KUBECONFIG_PATH
        CREDENTIAL_VALIDATION_KUBE_CA_FILE="ca"
        CREDENTIAL_VALIDATION_APISERVER_URL="url"

        touch $MOCK_BOOTSTRAP_KUBECONFIG_PATH
        tee $MOCK_BOOTSTRAP_KUBECONFIG_PATH > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: ca.crt
    server: url
users:
- name: kubelet-bootstrap
  user:
    token: token
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    }
    cleanup() {
        rm -f $MOCK_KUBECONFIG_PATH
        rm -f $MOCK_BOOTSTRAP_KUBECONFIG_PATH
    }
    sleep() {
        echo "sleep $@"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'
    
    It 'should exit 0 if CREDENTIAL_VALIDATION_KUBE_CA_FILE is not set'
        CREDENTIAL_VALIDATION_KUBE_CA_FILE=""
        When call validateKubeletCredentials
        The stdout should include 'CREDENTIAL_VALIDATION_KUBE_CA_FILE is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if CREDENTIAL_VALIDATION_APISERVER_URL is not set'
        CREDENTIAL_VALIDATION_APISERVER_URL=""
        When call validateKubeletCredentials
        The stdout should include 'CREDENTIAL_VALIDATION_APISERVER_URL is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if BOOTSTRAP_KUBECONFIG_PATH is not set'
        BOOTSTRAP_KUBECONFIG_PATH=""
        When call validateKubeletCredentials
        The stdout should include 'BOOTSTRAP_KUBECONFIG_PATH is not set, skipping kubelet credential validation'
        The status should be success
    End

    It 'should exit 0 if the bootstrap kubeconfig does not exist'
        BOOTSTRAP_KUBECONFIG_PATH="does/not/exist"
        When call validateKubeletCredentials
        The stdout should include 'no bootstrap-kubeconfig found at does/not/exist, no bootstrap credentials to validate'
        The status should be success
    End

    It 'should exit 0 if a kubeconfig already exists'
        touch $KUBECONFIG_PATH

        When call validateKubeletCredentials
        The stdout should include 'client credential already exists within kubeconfig: mock-kubeconfig, no need to validate bootstrap credentials'
        The status should be success
    End

    It 'should exit 0 if the curl command is unavailable'
        command() {
            return 1
        }

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