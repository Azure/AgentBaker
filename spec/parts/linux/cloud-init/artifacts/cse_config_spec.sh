#!/bin/bash

Describe 'cse_config.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'getPrimaryNicIP()'
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

        It 'should only generate the self-signed serving cert when EnableKubeletServingCertificateRotation is false'
            retrycmd_if_failure_no_stats() { # for mocking IMDS calls
                echo "false"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="false"
            When run configureKubeletServing
            The stdout should include 'kubelet serving certificate rotation is disabled, generating self-signed serving certificate with openssl'
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags to disable kubelet serving certificate rotation if opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags to disable kubelet serving certificate rotation if opt-out tag is set and kubelet config file is enabled'
            retrycmd_if_failure_no_stats() {
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
            The variable KUBELET_CONFIG_FILE_CONTENT should satisfy kubelet_config_file
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags and node labels to disable kubelet serving certificate rotation if opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should no-op if kubelet flags and node labels are already correct when the opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When run configureKubeletServing
            The stdout should include 'genrsa -out /etc/kubernetes/certs/kubeletserver.key 2048'
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should no-op if kubelet flags and node labels are already correct when the opt-out tag is set and kubelet config file is enabled'
            retrycmd_if_failure_no_stats() {
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
            The stdout should include 'req -new -x509 -days 7300 -key /etc/kubernetes/certs/kubeletserver.key -out /etc/kubernetes/certs/kubeletserver.crt'
            The variable KUBELET_CONFIG_FILE_CONTENT should satisfy kubelet_config_file
            The variable KUBELET_FLAGS should equal '--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt,--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key,--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet flags node labels to enable kubelet serving certificate rotation if opt-out tag is not set'
            retrycmd_if_failure_no_stats() {
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
            retrycmd_if_failure_no_stats() {
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
            retrycmd_if_failure_no_stats() {
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
End