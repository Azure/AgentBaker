#!/bin/bash

Describe 'cse_config_kubelet.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'configureKubeletServing'
        preserve_vars() {
            %preserve KUBELET_FLAGS
            %preserve KUBELET_NODE_LABELS
            %preserve KUBELET_CONFIG_FILE_CONTENT
        }
        # preserve contents of variables on which to assert since we need to run configureKubeletServing
        # in a subshell due to it modfiying shell opts (set +/-x), which would otherwise conflict with shellspec
        AfterRun preserve_vars

        Mock openssl
            echo "$@"
        End
        Mock mkdir
            echo "mkdir $@"
        End

        It 'should only generate the self-signed serving cert when EnableKubeletServingCertificateRotation is false'
            should_disable_kubelet_serving_certificate_rotation() { # for mocking IMDS calls
                echo "false"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="false"
            When run configureKubeletServing
            The stdout should include 'kubelet serving certificate rotation is disabled, generating self-signed serving certificate with openssl'
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags to disable kubelet serving certificate rotation if opt-out tag is set'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags to disable kubelet serving certificate rotation if opt-out tag is set and kubelet config file is enabled'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "true"
            }
            kubelet_config_file() {
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.serverTLSBootstrap')" == "false" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.tlsCertFile')" == "/etc/kubernetes/certs/kubeletserver.crt" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.tlsPrivateKeyFile')" == "/etc/kubernetes/certs/kubeletserver.key" ]
            }
            KUBELET_CONFIG_FILE_ENABLED="true"
            KUBELET_CONFIG_FILE_CONTENT=$(cat spec/parts/linux/cloud-init/artifacts/kubelet_mocks/config_file/server_tls_bootstrap_enabled.json | base64)
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stderr should not eq ''
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The variable KUBELET_CONFIG_FILE_CONTENT should satisfy kubelet_config_file
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags and node labels to disable kubelet serving certificate rotation if opt-out tag is set'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should no-op if kubelet flags and node labels are already correct when the opt-out tag is set'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should no-op if kubelet flags and node labels are already correct when the opt-out tag is set and kubelet config file is enabled'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "true"
            }
            kubelet_config_file() {
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r 'has("serverTLSBootstrap")')" == "false" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.tlsCertFile')" == "/etc/kubernetes/certs/kubeletserver.crt" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.tlsPrivateKeyFile')" == "/etc/kubernetes/certs/kubeletserver.key" ]
            }
            KUBELET_CONFIG_FILE_ENABLED="true"
            KUBELET_CONFIG_FILE_CONTENT=$(cat spec/parts/linux/cloud-init/artifacts/kubelet_mocks/config_file/server_tls_bootstrap_disabled.json | base64)
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stderr should not eq ''
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'mkdir -p /etc/kubernetes/certs'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_CONFIG_FILE_CONTENT should satisfy kubelet_config_file
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags node labels to enable kubelet serving certificate rotation if opt-out tag is not set'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "false"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'kubelet serving certificate rotation is enabled'
            The stdout should include 'removing --tls-cert-file and --tls-private-key-file from kubelet flags'
            The stdout should include 'adding node label'
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End

        It 'should reconfigure kubelet flags and node labels to enable kubelet serving certificate rotation if opt-out tag is not set and kubelet config file is enabled'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "false"
            }
            kubelet_config_file() {
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r '.serverTLSBootstrap')" == "true" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r 'has("tlsCertFile")')" == "false" ] && \
                [ "$(echo "${kubelet_config_file:?}" | base64 -d | jq -r 'has("tlsPrivateKeyFile")')" == "false" ]
            }
            KUBELET_CONFIG_FILE_ENABLED="true"
            KUBELET_CONFIG_FILE_CONTENT=$(cat spec/parts/linux/cloud-init/artifacts/kubelet_mocks/config_file/server_tls_bootstrap_enabled.json | base64)
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stderr should not eq ''
            The stdout should include 'kubelet serving certificate rotation is enabled'
            The stdout should include 'removing --tls-cert-file and --tls-private-key-file from kubelet flags'
            The stdout should include 'adding node label'
            The variable KUBELET_CONFIG_FILE_CONTENT should satisfy kubelet_config_file
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End

        It 'should no-op if kubelet flags and node labels are already correct when the opt-out tag is not set'
            should_disable_kubelet_serving_certificate_rotation() {
                echo "false"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'kubelet serving certificate rotation is enabled'
            The stdout should include 'removing --tls-cert-file and --tls-private-key-file from kubelet flags'
            The stdout should include 'adding node label'
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End
    End
    Describe 'writeCredentialProviderConfig'
        setup() {
            TMP_DIR=$(mktemp -d)
            # Reset all related variables before each test
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED=""
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI=""
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID=""
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID=""
            API_SERVER_NAME=""
            AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX=""
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
            MCR_REPOSITORY_BASE=""
        }
        cleanup() {
            rm -rf "$TMP_DIR"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should configure credential provider with default settings when no special flags are set'
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should not include dedicated data endpoint patterns in matchImages (data endpoints are not login servers)'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should not include "data.azurecr"
        End

        It 'should configure credential provider for network isolated cluster'
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="test.azurecr.io"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "mcr.microsoft.com"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json
      - --registry-mirror=mcr.microsoft.com:test.azurecr.io'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider for custom cloud'
            AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX=".custom.registry.io"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "*.custom.registry.io"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider for custom cloud"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider for custom cloud network isolated cluster'
            AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX=".custom.registry.io"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="test.azurecr.io"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "*.custom.registry.io"
      - "mcr.microsoft.com"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json
      - --registry-mirror=mcr.microsoft.com:test.azurecr.io'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider for custom cloud"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider with identity binding enabled and all args'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID="my-tenant-id"
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --ib-sni-name=test.sni.local
      - --ib-default-client-id=my-client-id
      - --ib-default-tenant-id=my-tenant-id
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider with identity binding enabled without optional client-id'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID=""
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID="my-tenant-id"
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --ib-sni-name=test.sni.local
      - --ib-default-tenant-id=my-tenant-id
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider with identity binding enabled without optional tenant-id'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID=""
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --ib-sni-name=test.sni.local
      - --ib-default-client-id=my-client-id
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider with identity binding enabled with only required args'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID=""
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID=""
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --ib-sni-name=test.sni.local
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider for network isolated cluster with identity binding enabled'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID="my-tenant-id"
            API_SERVER_NAME="apiserver.example.com"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="test.azurecr.io"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "mcr.microsoft.com"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --registry-mirror=mcr.microsoft.com:test.azurecr.io
      - --ib-sni-name=test.sni.local
      - --ib-default-client-id=my-client-id
      - --ib-default-tenant-id=my-tenant-id
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should configure credential provider for custom cloud with identity binding enabled'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="true"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID="my-tenant-id"
            API_SERVER_NAME="apiserver.example.com"
            AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX=".custom.registry.io"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "*.custom.registry.io"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id
    args:
      - /etc/kubernetes/azure.json
      - --ib-sni-name=test.sni.local
      - --ib-default-client-id=my-client-id
      - --ib-default-tenant-id=my-tenant-id
      - --ib-apiserver-ip=apiserver.example.com'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider for custom cloud"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should not add identity binding config when SERVICE_ACCOUNT_IMAGE_PULL_ENABLED is false'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED="false"
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End

        It 'should not add identity binding config when SERVICE_ACCOUNT_IMAGE_PULL_ENABLED is empty'
            SERVICE_ACCOUNT_IMAGE_PULL_ENABLED=""
            IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI="test.sni.local"
            SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID="my-client-id"
            API_SERVER_NAME="apiserver.example.com"
            expected_config='apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json'
            When call writeCredentialProviderConfig "$TMP_DIR/credential-provider-config.yaml"
            The output should include "configure credential provider with default settings"
            The contents of file "$TMP_DIR/credential-provider-config.yaml" should equal "$expected_config"
        End
    End
    Describe 'configureAndEnableSecureTLSBootstrapping'
        SECURE_TLS_BOOTSTRAPPING_DROP_IN_DIR="secure-tls-bootstrap.service.d"
        SECURE_TLS_BOOTSTRAPPING_DROP_IN="${SECURE_TLS_BOOTSTRAPPING_DROP_IN_DIR}/10-securetlsbootstrap.conf"
        SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE_DIR="default"
        SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE="${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE_DIR}/secure-tls-bootstrap"
        API_SERVER_NAME="fqdn"
        AZURE_JSON_PATH="/etc/kubernetes/azure.json"

        chmod() {
            echo "chmod $@"
        }

        retrycmd_if_failure() {
            shift 3
            echo "$@"
        }

        cleanup() {
            rm -rf "$SECURE_TLS_BOOTSTRAPPING_DROP_IN_DIR"
            rm -rf "$SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE_DIR"
        }

        AfterEach 'cleanup'

        It 'should configure and enable secure TLS bootstrapping'
            When call configureAndEnableSecureTLSBootstrapping
            The output should include "chmod 0600 secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
            The output should include "chmod 0600 default/secure-tls-bootstrap"
            The output should include "systemctl enable secure-tls-bootstrap"
            The output should not include "systemctlEnableAndStartNoBlock"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Unit]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "Before=kubelet.service"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Service]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "EnvironmentFile=default/secure-tls-bootstrap"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Install]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "WantedBy=kubelet.service"
            The contents of file "default/secure-tls-bootstrap" should include 'BOOTSTRAP_FLAGS=--aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630 --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json'
            The contents of file "default/secure-tls-bootstrap" should not include 'AZURE_ENVIRONMENT_FILEPATH'
            The status should be success
        End

        It 'should include AZURE_ENVIRONMENT_FILEPATH in the default file when set'
            AZURE_ENVIRONMENT_FILEPATH="/etc/kubernetes/akscustom.json"
            When call configureAndEnableSecureTLSBootstrapping
            The output should include "systemctl enable secure-tls-bootstrap"
            The contents of file "default/secure-tls-bootstrap" should include 'BOOTSTRAP_FLAGS=--aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630 --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json'
            The contents of file "default/secure-tls-bootstrap" should include 'AZURE_ENVIRONMENT_FILEPATH=/etc/kubernetes/akscustom.json'
            The status should be success
        End

        It 'should configure and enable secure TLS bootstrapping using provided overrides'
            SECURE_TLS_BOOTSTRAPPING_VALIDATE_KUBECONFIG_TIMEOUT="custom-validate-kubeconfig-timeout"
            SECURE_TLS_BOOTSTRAPPING_GET_ACCESS_TOKEN_TIMEOUT="custom-get-access-token-timeout"
            SECURE_TLS_BOOTSTRAPPING_GET_INSTANCE_DATA_TIMEOUT="custom-get-instance-data-timeout"
            SECURE_TLS_BOOTSTRAPPING_GET_NONCE_TIMEOUT="custom-get-nonce-timeout"
            SECURE_TLS_BOOTSTRAPPING_GET_ATTESTED_DATA_TIMEOUT="custom-get-attested-data-timeout"
            SECURE_TLS_BOOTSTRAPPING_GET_CREDENTIAL_TIMEOUT="custom-get-credential-timeout"
            SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE="custom-resource"
            SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID="custom-identity-id"
            When call configureAndEnableSecureTLSBootstrapping
            The output should include "chmod 0600 secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
            The output should include "chmod 0600 default/secure-tls-bootstrap"
            The output should include "systemctl enable secure-tls-bootstrap"
            The output should not include "systemctlEnableAndStartNoBlock"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Unit]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "Before=kubelet.service"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Service]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "EnvironmentFile=default/secure-tls-bootstrap"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Install]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "WantedBy=kubelet.service"
            The contents of file "default/secure-tls-bootstrap" should include 'BOOTSTRAP_FLAGS=--aad-resource=custom-resource --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json --user-assigned-identity-id=custom-identity-id --validate-kubeconfig-timeout=custom-validate-kubeconfig-timeout --get-access-token-timeout=custom-get-access-token-timeout --get-instance-data-timeout=custom-get-instance-data-timeout --get-nonce-timeout=custom-get-nonce-timeout --get-attested-data-timeout=custom-get-attested-data-timeout --get-credential-timeout=custom-get-credential-timeout'
            The status should be success
        End
    End
    Describe 'ensureKubelet credential provider installation gate'
        logs_to_events() {
            echo "logs_to_events $1 $2"
            eval "$2"
        }

        BeforeEach 'setup_ensure_kubelet'
        AfterEach 'cleanup_ensure_kubelet'
        setup_ensure_kubelet() {
            mkdir -p /opt/azure/containers
            KUBELET_FLAGS="--image-credential-provider-config=/etc/kubernetes/credential-provider-config.yaml --image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider"
            NETWORK_POLICY=""
            KUBELET_IMAGE=""
            KUBELET_NODE_LABELS=""
            AZURE_ENVIRONMENT_FILEPATH=""
            API_SERVER_NAME="example.invalid"
            ENABLE_IMDS_RESTRICTION="false"
            INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE="false"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
            KUBE_RESERVED_CGROUP=""
            SYSTEM_RESERVED_CGROUP=""
        }

        cleanup_ensure_kubelet() {
            rm -f /etc/default/kubelet /var/lib/kubelet/kubeconfig /var/lib/kubelet/bootstrap-kubeconfig \
                /opt/azure/containers/kubelet.sh /opt/azure/containers/tls-bootstrap-start-time \
                /etc/systemd/system/kubelet.service.d/10-{watchdog,credential-validation,tlsbootstrap,ensure-imds-restriction,containerd-base-flag}.conf
        }

        setKubeletNodeIPFlag() { :; }
        getPrimaryNicIP() { echo "10.0.0.4"; }
        configCredentialProvider() { :; }
        resolveKubeletReservedCgroups() { :; }
        systemctl() { :; }
        systemctlEnableAndStartNoBlock() { :; }
        tee() { cat > /dev/null; }

        # ensureKubelet enables xtrace mid-function and never restores it, so the
        # trace would leak onto ShellSpec's stderr and fail the example. Wrap the
        # call to discard that trace and restore set +x before returning.
        invoke_ensure_kubelet() {
            ensureKubelet 2>/dev/null
            { local rc=$?; set +x; } 2>/dev/null
            return "$rc"
        }

        Describe 'on Ubuntu'
            OS="UBUNTU"
            Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
            Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

            It 'installs the credential provider from URL below Kubernetes 1.33'
                installCredentialProviderFromUrl() { echo "installCredentialProviderFromUrl"; }
                installCredentialProviderFromPkg() { echo "installCredentialProviderFromPkg $1"; }
                KUBERNETES_VERSION="1.32.99"

                When call invoke_ensure_kubelet

                The output should include "installCredentialProviderFromUrl"
                The output should not include "installCredentialProviderFromPkg"
                The status should be success
            End

            It 'installs the credential provider from PMC at Kubernetes 1.33'
                installCredentialProviderFromUrl() { echo "installCredentialProviderFromUrl"; }
                installCredentialProviderFromPkg() { echo "installCredentialProviderFromPkg $1"; }
                KUBERNETES_VERSION="1.33.0"

                When call invoke_ensure_kubelet

                The output should include "installCredentialProviderFromPkg 1.33.0"
                The output should not include "installCredentialProviderFromUrl"
                The status should be success
            End
        End

        Describe 'on Azure Linux'
            OS="AZURELINUX"
            OS_VERSION="3.0"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"

            It 'installs the credential provider from PMC at Kubernetes 1.33'
                installCredentialProviderFromUrl() { echo "installCredentialProviderFromUrl"; }
                installCredentialProviderFromPkg() { echo "installCredentialProviderFromPkg $1"; }
                KUBERNETES_VERSION="1.33.0"

                When call invoke_ensure_kubelet

                The output should include "installCredentialProviderFromPkg 1.33.0"
                The output should not include "installCredentialProviderFromUrl"
                The status should be success
            End
        End
    End
    Describe 'configureKubeletAndKubectl'
        # Mock required functions and variables
        logs_to_events() {
            echo "logs_to_events $1 $2"
            # Execute the actual function that was passed
            eval "$2"
        }

        installKubeletKubectlFromURL() {
            echo "installKubeletKubectlFromURL"
        }

        installKubeletKubectlFromBootstrapProfileRegistry() {
            echo "installKubeletKubectlFromBootstrapProfileRegistry $1 $2"
        }

        # Set default values for common variables
        BeforeEach 'setup'
        setup() {
            SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
            CUSTOM_KUBE_BINARY_DOWNLOAD_URL=""
            PRIVATE_KUBE_BINARY_DOWNLOAD_URL=""
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
            OS_VERSION=""
            KUBERNETES_VERSION=""
        }

        Describe 'on Ubuntu'
            OS="UBUNTU"
            Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
            Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

            # Test cases for URL installation (first condition)
            It 'should install from URL if CUSTOM_KUBE_BINARY_DOWNLOAD_URL is set'
                CUSTOM_KUBE_BINARY_DOWNLOAD_URL="https://custom-kube-url.com/kube.tar.gz"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should install from URL if PRIVATE_KUBE_BINARY_DOWNLOAD_URL is set'
                PRIVATE_KUBE_BINARY_DOWNLOAD_URL="https://private-kube-url.com/kube.tar.gz"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should not install from PMC if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set'
                BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should not include "installKubeletKubectlFromPkg"
            End

            # Test cases for version-based logic (second condition)
            It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is not true and k8s version < 1.34'
                SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
                KUBERNETES_VERSION="1.33.5"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is false and k8s version < 1.34'
                SHOULD_ENFORCE_KUBE_PMC_INSTALL="false"
                KUBERNETES_VERSION="1.33.5"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should install from PMC if k8s version >= 1.34'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            It 'should install from PMC if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and k8s version < 1.34'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
                KUBERNETES_VERSION="1.32.5"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            # Test edge cases
            It 'should prioritize custom URL over version-based logic'
                CUSTOM_KUBE_BINARY_DOWNLOAD_URL="https://custom-kube-url.com/kube.tar.gz"
                SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should handle version exactly at boundary (1.34.0)'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                KUBERNETES_VERSION="1.34.0"
                SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            # Test BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER scenarios
            It 'should call installKubeletKubectlFromBootstrapProfileRegistry when BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set and k8s >= 1.34.0 and succeeds'
                BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromBootstrapProfileRegistry myregistry.azurecr.io 1.34.0"
                The output should not include "installKubeletKubectlFromURL"
            End

            It 'should call installKubeletKubectlFromURL when BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set and k8s < 1.34.0'
                BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
                KUBERNETES_VERSION="1.33.5"
                When call configureKubeletAndKubectl
                The output should not include "installKubeletKubectlFromBootstrapProfileRegistry"
                The output should include "installKubeletKubectlFromURL"
            End

            It 'should call installKubeletKubectlFromBootstrapProfileRegistry when SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and k8s < 1.34.0 and BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set'
                BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
                KUBERNETES_VERSION="1.33.5"
                SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromBootstrapProfileRegistry myregistry.azurecr.io 1.33.5"
                The output should not include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End

            It 'should not call installKubeletKubectlFromBootstrapProfileRegistry when SHOULD_ENFORCE_KUBE_PMC_INSTALL is false and k8s < 1.34.0 and BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set'
                BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
                KUBERNETES_VERSION="1.33.5"
                SHOULD_ENFORCE_KUBE_PMC_INSTALL="false"
                When call configureKubeletAndKubectl
                The output should not include "installKubeletKubectlFromBootstrapProfileRegistry"
                The output should include "installKubeletKubectlFromURL"
            End

            It 'should fallback to kube binary install when version uncached'
                ls() {
                    echo ""
                }
                fallbackToKubeBinaryInstall() {
                    echo "fallbackToKubeBinaryInstall $1 $2"
                }
                updatePMCRepository() {
                    echo "updatePMCRepository"
                }

                KUBERNETES_VERSION="1.34.0"
                SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
                When call configureKubeletAndKubectl
                The output should include "fallbackToKubeBinaryInstall"
                The output should not include "updatePMCRepository"
            End
        End

        Describe 'on Flatcar'
            OS="FLATCAR"
            Include "./parts/linux/cloud-init/artifacts/flatcar/cse_helpers_flatcar.sh"
            Include "./parts/linux/cloud-init/artifacts/flatcar/cse_install_flatcar.sh"

            installKubeletKubectlFromPkg() {
                echo "installKubeletKubectlFromPkg $@"
            }

            It 'should install from MAR if k8s version >= 1.34'
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End
        End

        Describe 'on Mariner'
            OS="MARINER"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"

            It 'should install from PMC if k8s version >= 1.34 and OS_VERSION != 2.0'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                OS_VERSION="3.0"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            It 'should install from PMC if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and OS_VERSION != 2.0'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
                OS_VERSION="3.0"
                KUBERNETES_VERSION="1.32.5"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and OS_VERSION = 2.0'
                SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
                OS_VERSION="2.0"
                KUBERNETES_VERSION="1.32.5"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End
        End

        Describe 'on Azure Linux'
            OS="AZURELINUX"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"
            Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"

            It 'should install from PMC if k8s version >= 1.34 and OS_VERSION != 2.0'
                installKubeletKubectlFromPkg() {
                    echo "installKubeletKubectlFromPkg $1"
                }

                OS_VERSION="3.0"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromPkg"
                The output should not include "installKubeletKubectlFromURL"
            End

            It 'should install from URL if OS_VERSION = 2.0'
                OS_VERSION="2.0"
                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should include "installKubeletKubectlFromURL"
                The output should not include "installKubeletKubectlFromPkg"
            End
        End

        Describe 'on Windows'
            OS="Windows"  # Unsupported OS

            It 'should not call any install function for unsupported OS'
                exit() {
                    echo "mock exit $1"
                }

                KUBERNETES_VERSION="1.34.0"
                When call configureKubeletAndKubectl
                The output should not include "installKubeletKubectlFromURL"
                The output should include "installKubeletKubectlFromPkg is not defined"
            End
        End
    End
    Describe 'ensurePodInfraContainerImage'
        waitForContainerdReady() { return 0; }
        ctr() { echo ""; return 0; }
        mkdir() { echo "mkdir $@"; }
        tar() { echo "tar $@"; return 0; }
        rm() { echo "rm $@"; }
        labelContainerImage() { echo "labelContainerImage $@"; }
        retrycmd_cp_oci_layout_with_oras() { echo "retrycmd_cp_oci_layout_with_oras $@"; return 0; }
        ERR_PULL_POD_INFRA_CONTAINER_IMAGE=1

        It 'should use MCR_REPOSITORY_BASE for image replacement when set'
            get_sandbox_image() { echo "mcr.microsoft.us/oss/v2/kubernetes/pause:3.10.2"; }
            MCR_REPOSITORY_BASE="mcr.microsoft.us"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myacr.azurecr.io/aks-managed-repository"

            When call ensurePodInfraContainerImage

            The status should be success
            The output should include "Pulling with authentication for myacr.azurecr.io/aks-managed-repository/oss/v2/kubernetes/pause:3.10.2"
        End

        It 'should fall back to mcr.microsoft.com when MCR_REPOSITORY_BASE is unset'
            get_sandbox_image() { echo "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2"; }
            MCR_REPOSITORY_BASE=""
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myacr.azurecr.io/aks-managed-repository"

            When call ensurePodInfraContainerImage

            The status should be success
            The output should include "Pulling with authentication for myacr.azurecr.io/aks-managed-repository/oss/v2/kubernetes/pause:3.10.2"
        End

        It 'should handle MCR_REPOSITORY_BASE with trailing slash'
            get_sandbox_image() { echo "mcr.microsoft.us/oss/v2/kubernetes/pause:3.10.2"; }
            MCR_REPOSITORY_BASE="mcr.microsoft.us/"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myacr.azurecr.io/aks-managed-repository"

            When call ensurePodInfraContainerImage

            The status should be success
            The output should include "Pulling with authentication for myacr.azurecr.io/aks-managed-repository/oss/v2/kubernetes/pause:3.10.2"
        End
    End
End
