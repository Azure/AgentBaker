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

    Describe 'configureKubeletServingCertificateRotation'
        It 'should no-op when EnableKubeletServingCertificateRotation is false'
            retrycmd_if_failure_no_stats() { # for mocking IMDS calls
                echo "false"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="false"
            When call configureKubeletServingCertificateRotation
            The stdout should eq 'kubelet serving certificate rotation is disabled, nothing to configure'
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet args to disable kubelet serving certificate rotation if opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When call configureKubeletServingCertificateRotation
            The stdout should not eq ''
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet args and node labels to disable kubelet serving certificate rotation if opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When call configureKubeletServingCertificateRotation
            The stdout should not eq ''
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should no-op if kubelet args and node labels are already correct when the opt-out tag is set'
            retrycmd_if_failure_no_stats() {
                echo "true"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When call configureKubeletServingCertificateRotation
            The stdout should not eq ''
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End

        It 'should reconfigure kubelet node labels to enable kubelet serving certificate rotation if opt-out tag is not set'
            retrycmd_if_failure_no_stats() {
                echo "false"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When call configureKubeletServingCertificateRotation
            The stdout should not eq ''
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End

        It 'should no-op if kubelet args and node labels are already correct when the opt-out tag is not set'
            retrycmd_if_failure_no_stats() {
                echo "false"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster" 
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
            When call configureKubeletServingCertificateRotation
            The stdout should not eq ''
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End
    End
    Describe 'configureContainerd'
        It 'should not contain deprecated properties in config.toml'
            retrycmd_if_failure_no_stats() { # for mocking IMDS calls
                echo "false"
            }
            KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
            KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
            ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="false"
            When call ensureContainerd
            The stdout should eq 'kubelet serving certificate rotation is disabled, nothing to configure'
            The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
        End
End