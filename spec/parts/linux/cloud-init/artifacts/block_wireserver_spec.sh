#!/bin/bash

Describe 'block_wireserver.sh'
    SCRIPT="./parts/linux/cloud-init/artifacts/block_wireserver.sh"
    CSE_CONFIG="./parts/linux/cloud-init/artifacts/cse_config.sh"

    Mock iptables
        echo "iptables $*"
        case "$1" in
            -C)
                if [ "${RULE_EXISTS:-false}" = "true" ]; then
                    exit 0
                fi
                exit 1
                ;;
            -I) exit "${INSERT_EXIT_CODE:-0}" ;;
        esac
    End

    It 'inserts both rules when they are absent'
        export RULE_EXISTS=false
        export INSERT_EXIT_CODE=0
        When run command bash "${SCRIPT}"
        The status should be success
        The lines of output should eq 4
        The output should include "iptables -C FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP"
        The output should include "iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP"
        The output should include "iptables -C FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP"
        The output should include "iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP"
    End

    It 'does not insert duplicates when both rules exist'
        export RULE_EXISTS=true
        export INSERT_EXIT_CODE=0
        When run command bash "${SCRIPT}"
        The status should be success
        The lines of output should eq 2
        The output should include "iptables -C FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP"
        The output should include "iptables -C FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP"
        The output should not include "iptables -I"
    End

    It 'propagates failure when rule insertion fails'
        export RULE_EXISTS=false
        export INSERT_EXIT_CODE=4
        When run command bash "${SCRIPT}"
        The status should eq 4
        The lines of output should eq 4
    End

    renderKubeletRuntimeScript() {
        awk '
            /tee "\$\{KUBELET_RUNTIME_CONFIG_SCRIPT_FILE\}".*EOF/ { printing = 1; next }
            printing && /^EOF$/ { exit }
            printing { print }
        ' "${CSE_CONFIG}"
    }

    It 'keeps the provisioning copy identical to the VHD copy'
        When call renderKubeletRuntimeScript
        The status should be success
        The output should eq "$(cat "${SCRIPT}")"
    End
End
