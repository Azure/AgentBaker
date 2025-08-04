#!/bin/bash

Describe 'cse_config.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    Describe 'configureAzureJson'
        AZURE_JSON_PATH="azure.json"
        AKS_CUSTOM_CLOUD_JSON_PATH="customcloud.json"
        CLOUDPROVIDER_BACKOFF_EXPONENT="1"
        CLOUDPROVIDER_BACKOFF_JITTER="0.1"
        TARGET_CLOUD="AzurePublicCloud"
        TENANT_ID="tenant-id"
        SUBSCRIPTION_ID="subscription-id"
        RESOURCE_GROUP="resource-group"
        LOCATION="eastus"

        chmod () {
            echo "chmod $@"
        }
        chown() {
            echo "chown $@"
        }

        cleanup() {
            rm -f "$AZURE_JSON_PATH"
            rm -f "$AKS_CUSTOM_CLOUD_JSON_PATH"
        }

        AfterEach 'cleanup'

        It 'should configure the azure.json file'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v1"
            # using "run" instead of "call" since configureAzureJson modifies shell opts with set +/-x which conflicts with shellspec
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffExponent": 1'
            The contents of file "azure.json" should include '"cloudProviderBackoffJitter": 0.1'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v1"'
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should configure the azure.json file without a service principal secret if no service principal file content is supplied'
            SERVICE_PRINCIPAL_CLIENT_ID=""
            SERVICE_PRINCIPAL_FILE_CONTENT=""
            CLOUDPROVIDER_BACKOFF_MODE="v1"
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": ""'
            The contents of file "azure.json" should include '"aadClientSecret": ""'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffExponent": 1'
            The contents of file "azure.json" should include '"cloudProviderBackoffJitter": 0.1'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v1"'
            The contents of file "azure.json" should not include "sp-secret"
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should reconfigure azure json if cloud provider backoff mode is "v2"'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v2"
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v2"'
            The contents of file "azure.json" should not include "cloudProviderBackoffExponent"
            The contents of file "azure.json" should not include "cloudProviderBackoffJitter"
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should create the AKS custom cloud json file if running in custom cloud environment'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v2"
            IS_CUSTOM_CLOUD="true"
            CUSTOM_ENV_JSON="eyJjdXN0b20iOnRydWV9Cg==" # base64 encoding of '{"custom":true}'
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The output should include "chmod 0600 customcloud.json"
            The output should include "chown root:root customcloud.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v2"'
            The contents of file "azure.json" should not include "cloudProviderBackoffExponent"
            The contents of file "azure.json" should not include "cloudProviderBackoffJitter"
            The contents of file "customcloud.json" should include '"custom":true'
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End
    End

    Describe 'getPrimaryNicIP'
        It 'should return the correct IP when a single network interface is attached to the VM'
            curl() {
                cat spec/parts/linux/cloud-init/artifacts/imds_mocks/network/single_nic.json
            }
            When call getPrimaryNicIP
            The output should equal "0.0.0.0"
        End

        It 'should return the correct IP when multiple network interfaces are attached to the VM'
            curl() {
                cat spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json
            }
            When call getPrimaryNicIP
            The output should equal "0.0.0.0"
        End
    End

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
            retrycmd_silent() { # for mocking IMDS calls
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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
            retrycmd_silent() {
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

    Describe 'configureContainerdRegistryHost'
        It 'should configure registry host correctly if MCR_REPOSITORY_BASE is unset'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if MCR_REPOSITORY_BASE is set'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            MCR_REPOSITORY_BASE="fake.test.com"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/fake.test.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/fake.test.com"
            The output should include "touch /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if MCR_REPOSITORY_BASE has the suffic "/"'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            MCR_REPOSITORY_BASE="fake.test.com/"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/fake.test.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/fake.test.com"
            The output should include "touch /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is abc.azurecr.io'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="abc.azurecr.io"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The variable CONTAINER_REGISTRY_URL should equal 'abc.azurecr.io/v2/'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is abc.azurecr.io/def'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="abc.azurecr.io/def"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The variable CONTAINER_REGISTRY_URL should equal 'abc.azurecr.io/v2/def/'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End
    End

    Describe 'configCredentialProvider'
        Mock mkdir
            echo "mkdir $@"
        End

        Mock touch
            echo "touch $@"
        End

        Mock tee
            echo "tee $@"
        End

        It 'should configure credential provider for network isolated cluster'
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="test.azurecr.io"
            When call configCredentialProvider
            The variable CREDENTIAL_PROVIDER_CONFIG_FILE should equal '/var/lib/kubelet/credential-provider-config.yaml'
            The output should include "mkdir -p /var/lib/kubelet"
            The output should include "touch /var/lib/kubelet/credential-provider-config.yaml"
            The output should include "configure credential provider for network isolated cluster"
            The output should not include "tee"
        End
    End

    Describe 'enableLocalDNS'
        setup() {
            TMP_DIR=$(mktemp -d)
            LOCALDNS_CORE_FILE="$TMP_DIR/localdns.corefile"

            systemctlEnableAndStart() {
                echo "systemctlEnableAndStart $@"
                return 0
            }
        }
        cleanup() {
            rm -rf "$TMP_DIR"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should enable localdns successfully'
            echo 'localdns corefile' > "$LOCALDNS_CORE_FILE"
            When call enableLocalDNS
            The status should be success
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        It 'should skip enabling localdns if corefile is not created'
            rm -rf "$LOCALDNS_CORE_FILE"
            When call enableLocalDNS
            The status should be success
            The output should include "localdns should not be enabled."
        End

        It 'should return error when systemctl fails to start localdns'
            echo 'localdns corefile' > "$LOCALDNS_CORE_FILE"
            systemctlEnableAndStart() {
                echo "systemctlEnableAndStart $@"
                return 1
            }
            When call enableLocalDNS
            The status should equal 216
            The output should include "localdns should be enabled."
            The output should include "Enable localdns failed."
        End
    End

    Describe 'configureAndStartSecureTLSBootstrapping'
        SECURE_TLS_BOOTSTRAPPING_DROP_IN="secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
        API_SERVER_NAME="fqdn"
        AZURE_JSON_PATH="/etc/kubernetes/azure.json"

        chmod() {
            echo "chmod $@"
        }

        cleanup() {
            rm -rf "$SECURE_TLS_BOOTSTRAPPING_DROP_IN"
        }

        AfterEach 'cleanup'

        It 'should configure and start secure TLS bootstrapping'
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
            }
            When call configureAndStartSecureTLSBootstrapping
            The output should include "chmod 0600 secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
            The output should include "systemctlEnableAndStartNoBlock secure-tls-bootstrap 30"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Unit]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "Before=kubelet.service"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Service]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include 'Environment="BOOTSTRAP_FLAGS=--aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630 --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json"'
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Install]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "WantedBy=kubelet.service"
            The status should be success
        End
    End

    Describe 'configKubeletAndKubectl'
        installKubeletKubectlFromURL() {
            echo "installKubeletKubectlFromURL"
        }
        installKubeletKubectlPkgFromPMC() {
            echo "installKubeletKubectlPkgFromPMC"
        }

        It 'should install from URL if custom URL specified'
            CUSTOM_KUBE_BINARY_DOWNLOAD_URL="https://custom-kube-url.com/kube.tar.gz"
            When call configKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should install using URL if k8s version < 1.34'
            KUBERNETES_VERSION="1.32.5"
            When call configKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should install from PMC if k8s version >= 1.34'
            KUBERNETES_VERSION="1.34.0"
            When call configKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from PMC with nodepool tag enforce_pmc_kube_pkg_install'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            When call configKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End
    End
End