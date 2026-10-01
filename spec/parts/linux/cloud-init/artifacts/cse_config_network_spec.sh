#!/bin/bash

Describe 'cse_config_network.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'getPrimaryNicIP'
        It 'should return the correct IP when a single network interface is attached to the VM'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/single_nic.json"
            When call getPrimaryNicIP
            The output should equal "0.0.0.0"
        End

        It 'should return the correct IP when multiple network interfaces are attached to the VM'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            When call getPrimaryNicIP
            The output should equal "0.0.0.0"
        End
    End
    Describe 'configureSecondaryNICs'
        cleanup() {
            # Only remove files written by the tests, not the entire directory
            rm -f /etc/netplan/60-secondary-nic-*.yaml 2>/dev/null || true
            rm -f /etc/systemd/network/10-secondary-nic-*.network 2>/dev/null || true
        }

        AfterEach 'cleanup'

        # Stub commands that configureSecondaryNICs calls
        chmod() {
            echo "chmod $@"
        }
        netplan() {
            echo "netplan $@"
        }
        networkctl() {
            echo "networkctl $@"
        }
        systemctl() {
            echo "systemctl $@"
        }
        retrycmd_if_failure() {
            echo "retrycmd_if_failure $@"
            shift 3
            "$@"
        }

        It 'should skip when only a single NIC is present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/single_nic.json"
            When run configureSecondaryNICs
            The output should include "No secondary NICs detected, skipping"
            The status should be success
        End

        It 'should configure netplan on Ubuntu when two NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="UBUNTU"
            mkdir -p /etc/netplan
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "could not find interface for MAC"
            The output should include "Configured secondary NIC eth1"
            The output should include "mac=7c:1e:52:5a:aa:aa"
            The output should include "metric=200"
            The output should include "chmod 600"
            The output should include "netplan apply"
            The status should be success
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'secondary-nic-1:'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'macaddress: "7c:1e:52:5a:aa:aa"'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp4: true'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'route-metric: 200'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'use-dns: false'
        End

        It 'should configure networkd on AzureLinux/Mariner when two NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURELINUX"
            # Create the networkd directory so the heredoc write succeeds
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "metric=2100"
            The output should include "retrycmd_if_failure 5 3 10 networkctl reload"
            # networkctl up should NOT be called because the fallback interface
            # (eth1) does not exist in /sys/class/net — the .network files match
            # by MAC and will auto-activate when the real interface appears.
            The output should not include "networkctl up"
            The status should be success
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[Match]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'MACAddress=7c:1e:52:5a:aa:aa'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'DHCP=ipv4'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'UseDNS=false'
        End

        It 'should configure networkd on ACL with systemd restart when two NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURECONTAINERLINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "metric=2100"
            The output should include "retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd"
            The output should not include "networkctl reload"
            The status should be success
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[Match]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'MACAddress=7c:1e:52:5a:aa:aa'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'DHCP=ipv4'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
        End

        It 'should configure netplan for both secondary NICs when three NICs are present on Ubuntu'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/three_nic.json"
            OS="UBUNTU"
            mkdir -p /etc/netplan
            When run configureSecondaryNICs
            The output should include "Detected 3 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "Configured secondary NIC eth2"
            The status should be success
            # First secondary NIC
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'secondary-nic-1:'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'macaddress: "7c:ed:8d:8a:4d:ce"'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'route-metric: 200'
            # Second secondary NIC
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'secondary-nic-2:'
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'macaddress: "bb:cc:11:dd:22:ee"'
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'route-metric: 300'
        End

        It 'should configure networkd for both secondary NICs when three NICs are present on AzureLinux/Mariner'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/three_nic.json"
            OS="AZURELINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 3 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "Configured secondary NIC eth2"
            The output should include "retrycmd_if_failure 5 3 10 networkctl reload"
            # networkctl up should NOT be called because the fallback interfaces
            # don't exist in /sys/class/net
            The output should not include "networkctl up"
            The status should be success
            # First secondary NIC
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'MACAddress=7c:ed:8d:8a:4d:ce'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
            # Second secondary NIC
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'MACAddress=bb:cc:11:dd:22:ee'
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'RouteMetric=2200'
        End

        It 'should configure networkd for both secondary NICs when three NICs are present on ACL with systemd restart'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/three_nic.json"
            OS="AZURECONTAINERLINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 3 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "Configured secondary NIC eth2"
            The output should include "retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd"
            The output should not include "networkctl reload"
            The status should be success
            # First secondary NIC
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'MACAddress=7c:ed:8d:8a:4d:ce'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
            # Second secondary NIC
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'MACAddress=bb:cc:11:dd:22:ee'
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'RouteMetric=2200'
        End

        It 'should configure netplan with IPv6 on Ubuntu when dual-stack NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic_dualstack.json"
            OS="UBUNTU"
            mkdir -p /etc/netplan
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "metric=200"
            The output should include "chmod 600"
            The output should include "netplan apply"
            The status should be success
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp4: true'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'route-metric: 200'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp6: true'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp6-overrides:'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'use-dns: false'
        End

        It 'should configure networkd with IPv6 on AzureLinux/Mariner when dual-stack NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic_dualstack.json"
            OS="AZURELINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "metric=2100"
            The status should be success
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'DHCP=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'IPv6AcceptRA=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[DHCPv4]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[DHCPv6]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'UseDNS=false'
        End

        It 'should configure networkd with IPv6 on ACL when dual-stack NICs are present'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic_dualstack.json"
            OS="AZURECONTAINERLINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 2 NICs, configuring secondary interfaces..."
            The output should include "Configured secondary NIC eth1"
            The output should include "metric=2100"
            The output should include "retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd"
            The status should be success
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'DHCP=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'IPv6AcceptRA=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[DHCPv6]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
        End

        It 'should configure netplan with IPv6 for both secondary NICs when three dual-stack NICs are present on Ubuntu'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/three_nic_dualstack.json"
            OS="UBUNTU"
            mkdir -p /etc/netplan
            When run configureSecondaryNICs
            The output should include "Detected 3 NICs, configuring secondary interfaces..."
            The status should be success
            # First secondary NIC
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp4: true'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'route-metric: 200'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp6: true'
            The contents of file "/etc/netplan/60-secondary-nic-1.yaml" should include 'dhcp6-overrides:'
            # Second secondary NIC
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'dhcp4: true'
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'route-metric: 300'
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'dhcp6: true'
            The contents of file "/etc/netplan/60-secondary-nic-2.yaml" should include 'dhcp6-overrides:'
        End

        It 'should configure networkd with IPv6 for both secondary NICs when three dual-stack NICs are present on AzureLinux'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/three_nic_dualstack.json"
            OS="AZURELINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Detected 3 NICs, configuring secondary interfaces..."
            The status should be success
            # First secondary NIC — dual-stack
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'DHCP=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'IPv6AcceptRA=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include '[DHCPv6]'
            The contents of file "/etc/systemd/network/10-secondary-nic-1.network" should include 'RouteMetric=2100'
            # Second secondary NIC — dual-stack
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'DHCP=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'IPv6AcceptRA=yes'
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include '[DHCPv6]'
            The contents of file "/etc/systemd/network/10-secondary-nic-2.network" should include 'RouteMetric=2200'
        End

        It 'should return error when IMDS cache file is missing'
            IMDS_INSTANCE_METADATA_CACHE_FILE="/nonexistent/path.json"
            When run configureSecondaryNICs
            The stderr should include "Failed to parse NIC count from IMDS cache file"
            The status should equal 243
        End

        It 'should return error when IMDS cache file contains invalid JSON'
            tmpfile=$(mktemp)
            echo "not valid json" > "$tmpfile"
            IMDS_INSTANCE_METADATA_CACHE_FILE="$tmpfile"
            When run configureSecondaryNICs
            The stderr should include "Failed to parse NIC count from IMDS cache file"
            The status should equal 243
        End

        It 'should return error when netplan apply fails on Ubuntu'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="UBUNTU"
            mkdir -p /etc/netplan
            netplan() {
                return 1
            }
            When run configureSecondaryNICs
            The output should include "Configured secondary NIC eth1"
            The stderr should include "Failed to apply netplan config for secondary NICs"
            The status should equal 243
        End

        It 'should return error when networkctl reload fails on AzureLinux/Mariner'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURELINUX"
            mkdir -p /etc/systemd/network
            networkctl() {
                return 1
            }
            When run configureSecondaryNICs
            The output should include "Configured secondary NIC eth1"
            The stderr should include "Failed to reload networkd for secondary NICs"
            The status should equal 243
        End

        It 'should return error when systemd-networkd restart fails on ACL'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURECONTAINERLINUX"
            mkdir -p /etc/systemd/network
            systemctl() {
                return 1
            }
            When run configureSecondaryNICs
            The output should include "Configured secondary NIC eth1"
            The stderr should include "Failed to restart systemd-networkd for secondary NICs"
            The status should equal 243
        End

        It 'should not call networkctl up for fallback interfaces that do not exist in sysfs'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURELINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "could not find interface for MAC"
            The output should include "Configured secondary NIC eth1"
            # networkctl up should NOT be called because the fallback eth1 is not
            # a real interface — it won't exist in /sys/class/net and therefore
            # is excluded from the secondary_ifaces list.
            The output should not include "networkctl up"
            The output should include "retrycmd_if_failure 5 3 10 networkctl reload"
            The status should be success
        End

        It 'should not call networkctl up on ACL after systemd restart'
            IMDS_INSTANCE_METADATA_CACHE_FILE="spec/parts/linux/cloud-init/artifacts/imds_mocks/network/multi_nic.json"
            OS="AZURECONTAINERLINUX"
            mkdir -p /etc/systemd/network
            When run configureSecondaryNICs
            The output should include "Configured secondary NIC eth1"
            The output should include "retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd"
            The output should not include "networkctl up"
            The output should not include "networkctl reload"
            The status should be success
        End
    End
End
