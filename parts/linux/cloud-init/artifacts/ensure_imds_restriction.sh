#!/bin/bash
# Ensure IMDS restriction rule is correctly set on the node, this script will be registered as kubelet service's ExecStart
# to ensure desired iptables rule will be ensured after the node is restarted.
# Below 3 variables are passed from kubelet service's drop-in file.
primaryNicIP="${PRIMARY_NIC_IP}"
insertRuleToMangleTable="${INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE}"
enableIMDSRestriction="${ENABLE_IMDS_RESTRICTION}"

echo "Primary NIC IP: $primaryNicIP"
echo "Insert IMDS restriction rule to mangle table: $insertRuleToMangleTable"
echo "Enable IMDS restriction: $enableIMDSRestriction"

# If any required variable is not set, exit with error
if [[ -z "$primaryNicIP" || -z "$insertRuleToMangleTable" || -z "$enableIMDSRestriction" ]]; then
    echo "One or more required variables are not set, exiting..."
    exit 1
fi

ensureIMDSRestrictionRule() {
    if [[ "${insertRuleToMangleTable,,}" == "true" ]]; then
        echo "Before inserting IMDS restriction rule to mangle table, checking whether the rule already exists..."
        iptables -t mangle -S | grep -- '-d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP'
        if [[ $? -eq 0 ]]; then
            echo "IMDS restriction rule already exists in mangle table, returning..."
            return 0
        fi
        echo "Inserting IMDS restriction rule to mangle table..."
        iptables -t mangle -I FORWARD 1 ! -s "$primaryNicIP" -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP || exit 1
    else
        echo "Before inserting IMDS restriction rule to filter table, checking whether the rule already exists..."
        iptables -t filter -S | grep -- '-d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP'
        if [[ $? -eq 0 ]]; then
            echo "IMDS restriction rule already exists in filter table, returning..."
            return 0
        fi
        echo "Inserting IMDS restriction rule to filter table..."
        iptables -t filter -I FORWARD 1 ! -s "$primaryNicIP" -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP || exit 1
    fi
}

disableIMDSRestriction() {
    echo "Checking whether IMDS restriction rule exists in mangle table..."
    iptables -t mangle -S | grep -- '-d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP'
    if [[ $? -ne 0 ]]; then
        echo "IMDS restriction rule does not exist in mangle table, no need to delete"
    else
        echo "Deleting IMDS restriction rule from mangle table..."
        iptables -t mangle -D FORWARD ! -s $primaryNicIP -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP || exit 1
    fi

    echo "Checking whether IMDS restriction rule exists in filter table..."
    iptables -t filter -S | grep -- '-d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP'
    if [[ $? -ne 0 ]]; then
         echo "IMDS restriction rule does not exist in filter table, no need to delete"
    else
        echo "Deleting IMDS restriction rule from filter table..."
        iptables -t filter -D FORWARD ! -s $primaryNicIP -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP || exit 1
    fi
}

if [[ "${enableIMDSRestriction,,}" == "true" ]]; then
    ensureIMDSRestrictionRule
else
    disableIMDSRestriction
fi