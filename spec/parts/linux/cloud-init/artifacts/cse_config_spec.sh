#!/bin/bash

Describe 'cse_config.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
    Include "./parts/linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"

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

    Describe 'shouldEnableLocalDns'
        setup() {
            TMP_DIR=$(mktemp -d)
            LOCALDNS_COREFILE="$TMP_DIR/localdns.corefile"
            LOCALDNS_SLICEFILE="$TMP_DIR/localdns.slice"
            LOCALDNS_GENERATED_COREFILE=$(echo "bG9jYWxkbnMgY29yZWZpbGU=") # "localdns corefile" base64
            LOCALDNS_MEMORY_LIMIT="512M"
            LOCALDNS_CPU_LIMIT="250%"

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

        # Success case.
        It 'should enable localdns successfully'
            When call shouldEnableLocalDns
            The status should be success
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Corefile file creation.
        It 'should create localdns.corefile with correct data'
            When call shouldEnableLocalDns
            The status should be success
            The output should include "localdns should be enabled."
            The path "$LOCALDNS_COREFILE" should be file
            The contents of file "$LOCALDNS_COREFILE" should include "localdns corefile"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Corefile already exists (idempotency).
        It 'should overwrite existing localdns.corefile'
            echo "wrong data" > "$LOCALDNS_COREFILE"
            When call shouldEnableLocalDns
            The status should be success
            The path "$LOCALDNS_COREFILE" should be file
            The contents of file "$LOCALDNS_COREFILE" should include "localdns corefile"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Slice file creation.
        It 'should create localdns.slice with correct CPU and Memory limits'
            When call shouldEnableLocalDns
            The status should be success
            The output should include "localdns should be enabled."
            The path "$LOCALDNS_SLICEFILE" should be file
            The contents of file "$LOCALDNS_SLICEFILE" should include "MemoryMax=${LOCALDNS_MEMORY_LIMIT}"
            The contents of file "$LOCALDNS_SLICEFILE" should include "CPUQuota=${LOCALDNS_CPU_LIMIT}"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
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
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include 'Environment="BOOTSTRAP_FLAGS=--deadline=2m0s --aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630 --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json"'
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Install]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "WantedBy=kubelet.service"
            The status should be success
        End

        It 'should configure and start secure TLS bootstrapping using provided overrides'
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
            }
            SECURE_TLS_BOOTSTRAPPING_DEADLINE="custom-deadline"
            SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE="custom-resource"
            SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID="custom-identity-id"
            When call configureAndStartSecureTLSBootstrapping
            The output should include "chmod 0600 secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
            The output should include "systemctlEnableAndStartNoBlock secure-tls-bootstrap 30"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Unit]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "Before=kubelet.service"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Service]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include 'Environment="BOOTSTRAP_FLAGS=--deadline=custom-deadline --aad-resource=custom-resource --apiserver-fqdn=fqdn --cloud-provider-config=/etc/kubernetes/azure.json --user-assigned-identity-id=custom-identity-id"'
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "[Install]"
            The contents of file "secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf" should include "WantedBy=kubelet.service"
            The status should be success
        End
    End

    Describe 'configureKubeletAndKubectl'
        # Mock required functions and variables
        logs_to_events() {
            echo "logs_to_events $1 $2"
            # Execute the actual function that was passed
            eval "$2"
        }

        installKubeletKubectlPkgFromPMC() {
            echo "installKubeletKubectlPkgFromPMC $1"
        }

        installKubeletKubectlFromURL() {
            echo "installKubeletKubectlFromURL"
        }

        installKubeletKubectlFromBootstrapProfileRegistry() {
            echo "installKubeletKubectlFromBootstrapProfileRegistry $1 $2"
        }

        # Set default values for common variables
        BeforeEach() {
            OS="UBUNTU"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
            CUSTOM_KUBE_BINARY_DOWNLOAD_URL=""
            PRIVATE_KUBE_BINARY_DOWNLOAD_URL=""
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
            OS_VERSION=""
            KUBERNETES_VERSION=""
        }

        # Test cases for URL installation (first condition)
        It 'should install from URL if CUSTOM_KUBE_BINARY_DOWNLOAD_URL is set'
            CUSTOM_KUBE_BINARY_DOWNLOAD_URL="https://custom-kube-url.com/kube.tar.gz"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should install from URL if PRIVATE_KUBE_BINARY_DOWNLOAD_URL is set'
            PRIVATE_KUBE_BINARY_DOWNLOAD_URL="https://private-kube-url.com/kube.tar.gz"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should not install from PMC if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set'
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        # Test cases for version-based logic (second condition)
        It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is not true and k8s version < 1.34'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
            KUBERNETES_VERSION="1.33.5"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is false and k8s version < 1.34'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="false"
            KUBERNETES_VERSION="1.33.5"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        # Test cases for PMC installation with OS-specific logic
        It 'should install from PMC if k8s version >= 1.34 and OS is Ubuntu'
            OS="UBUNTU"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from PMC if k8s version >= 1.34 and OS is CBLMariner with OS_VERSION != 2.0'
            OS="MARINER"
            OS_VERSION="3.0"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from PMC if k8s version >= 1.34 and OS is AzureLinux with OS_VERSION != 2.0'
            OS="AZURELINUX"
            OS_VERSION="3.0"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from URL if OS is CBLMariner/AzureLinux with OS_VERSION = 2.0'
            OS="AZURELINUX"
            OS_VERSION="2.0"
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        # Test cases for enforce PMC install flag
        It 'should install from PMC if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and k8s version < 1.34'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            OS="UBUNTU"
            KUBERNETES_VERSION="1.32.5"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from PMC if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and OS is CBLMariner with OS_VERSION != 2.0'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            OS="MARINER"
            OS_VERSION="3.0"
            KUBERNETES_VERSION="1.32.5"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        It 'should install from URL if SHOULD_ENFORCE_KUBE_PMC_INSTALL is true but OS is CBLMariner/AzureLinux with OS_VERSION = 2.0'
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            OS="MARINER"
            OS_VERSION="2.0"
            KUBERNETES_VERSION="1.32.5"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        # Test edge cases
        It 'should prioritize custom URL over version-based logic'
            CUSTOM_KUBE_BINARY_DOWNLOAD_URL="https://custom-kube-url.com/kube.tar.gz"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            KUBERNETES_VERSION="1.34.0"
            OS="UBUNTU"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should handle version exactly at boundary (1.34.0)'
            OS="UBUNTU"
            KUBERNETES_VERSION="1.34.0"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL=""
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlPkgFromPMC"
            The output should not include "installKubeletKubectlFromURL"
        End

        # Test unsupported OS scenarios (should fallback to no action)
        It 'should not call any install function for unsupported OS'
            OS="Windows"  # Unsupported OS
            KUBERNETES_VERSION="1.34.0"
            When call configureKubeletAndKubectl
            The output should not include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
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

        It 'should call installKubeletKubectlFromBootstrapProfileRegistry when SHOULD_ENFORCE_KUBE_PMC_INSTALL is true and k8s < 1.34.0' and BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
            KUBERNETES_VERSION="1.33.5"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="true"
            When call configureKubeletAndKubectl
            The output should include "installKubeletKubectlFromBootstrapProfileRegistry myregistry.azurecr.io 1.33.5"
            The output should not include "installKubeletKubectlFromURL"
            The output should not include "installKubeletKubectlPkgFromPMC"
        End

        It 'should not call installKubeletKubectlFromBootstrapProfileRegistry when SHOULD_ENFORCE_KUBE_PMC_INSTALL is false and k8s < 1.34.0' and BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="myregistry.azurecr.io"
            KUBERNETES_VERSION="1.33.5"
            SHOULD_ENFORCE_KUBE_PMC_INSTALL="false"
            When call configureKubeletAndKubectl
            The output should not include "installKubeletKubectlFromBootstrapProfileRegistry"
            The output should include "installKubeletKubectlFromURL"
        End
    End
End
