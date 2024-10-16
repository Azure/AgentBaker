#!/bin/bash

Describe 'cse_config.sh'
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
End