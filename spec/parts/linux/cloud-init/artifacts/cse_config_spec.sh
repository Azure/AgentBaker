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

    Describe 'addKubeletNodeLabel'
        It 'should perform a no-op when the specified label already exists within the label string'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0"
            When call addKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The stdout should include 'kubelet node label kubernetes.azure.com/kubelet-serving-ca=cluster is already present, nothing to add'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should append the label when it does not already exist within the label string'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call addKubeletNodeLabel "kubernetes.azure.com/kubelet-serving-ca=cluster"
            The stdout should not include 'kubelet node label kubernetes.azure.com/kubelet-serving-ca=cluster is already present, nothing to add'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End
    End

    Describe 'removeKubeletNodeLabel'
        It 'should remove the specified label when it exists within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should remove the specified label when it is the first label within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should remove the specified label when it is the last label within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should not alter kubelet node labels if the target label does not exist'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should return an empty string if the only label within kubelet node labels is the target'
            KUBELET_NODE_LABELS="kubernetes.azure.com/kubelet-serving-ca=cluster"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal ''
        End
    End

    # Describe 'configureKubeletServingCertificateRotation'
    #     It 'should no-op when EnableKubeletServingCertificateRotation is false'
    #         retrycmd_if_failure_no_stats() {
    #             echo "false"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="false"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should eq 'kubelet serving certificate rotation is disabled, nothing to configure'
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
    #     End

    #     It 'should reconfigure kubelet args to disable kubelet serving certificate rotation if opt-out tag is set'
    #         retrycmd_if_failure_no_stats() {
    #             echo "true"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should not eq ''
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
    #     End

    #     It 'should reconfigure kubelet args and node labels to disable kubelet serving certificate rotation if opt-out tag is set'
    #         retrycmd_if_failure_no_stats() {
    #             echo "true"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should not eq ''
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
    #     End

    #     It 'should no-op if kubelet args and node labels are already correct when the opt-out tag is set'
    #         retrycmd_if_failure_no_stats() {
    #             echo "true"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should not eq ''
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0'
    #     End

    #     It 'should reconfigure kubelet node labels to enable kubelet serving certificate rotation if opt-out tag is not set'
    #         retrycmd_if_failure_no_stats() {
    #             echo "false"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0"
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should not eq ''
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
    #     End

    #     It 'should no-op if kubelet args and node labels are already correct when the opt-out tag is not set'
    #         retrycmd_if_failure_no_stats() {
    #             echo "false"
    #         }
    #         KUBELET_FLAGS="--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
    #         KUBELET_NODE_LABELS="kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster" 
    #         ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION="true"
    #         When call configureKubeletServingCertificateRotation
    #         The stdout should not eq ''
    #         The variable KUBELET_FLAGS should equal '--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false'
    #         The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
    #     End
    # End
End