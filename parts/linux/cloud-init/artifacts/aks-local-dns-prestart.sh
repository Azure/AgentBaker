#! /bin/bash

# In --check-tag mode, we check if the EnableAKSLocalDNS tag exists and is set to true
if [[ $* == *--check-tag* ]]; then
    INSTANCE_METADATA="$(curl -fsSL -H 'Metadata: true' --noproxy '*' 'http://169.254.169.254/metadata/instance?api-version=2021-02-01')"
    RESULT=$(jq -e '.compute.tagsList | map(select(.name | test("EnableAKSLocalDNS"; "i")))[0].value // "false" | test("true"; "i")' 2>&1 <<<"${INSTANCE_METADATA}")
    JQ_RC=$?
    printf "EnableAKSLocalDNS node tag value: ${RESULT} (${JQ_RC}).\n"
    exit $JQ_RC
fi

# This is a startup script for a node that's not preconfigured. It will get the cluster
# DNS service IP from the kubelet configuration and make sure the kubelet configuration 
# points at the aks-local-dns node IP.

set -euo pipefail
. /etc/default/aks-local-dns

# This is the IP that the local DNS service should bind to for node traffic; usually an APIPA address
LOCAL_NODE_DNS_IP="${LOCAL_NODE_DNS_IP:-169.254.10.10}"
# This is the IP that the local DNS service should bind to for pod traffic; usually an APIPA address
LOCAL_POD_DNS_IP="${LOCAL_POD_DNS_IP:-169.254.10.11}"

# Read the --cluster-dns IP from /etc/default/kubelet
KUBELET_CLUSTER_DNS_IP="$(gawk 'BEGIN {rc=1} match($0, /--cluster-dns=(\S+)/, a) {print a[1]; rc=0} END {exit rc}' </etc/default/kubelet)"

# Config file not initialized with DNS service IP; assume that we need to set up the node.
if [[ ! -z "${DNS_SERVICE_IP:-}" && "${KUBELET_CLUSTER_DNS_IP}" == "${LOCAL_POD_DNS_IP}" ]]; then
    printf "DNS_SERVICE_IP is set and kubelet is using aks-local-dns.\n"
else
    if [[ "${KUBELET_CLUSTER_DNS_IP}" == "${LOCAL_POD_DNS_IP}" ]]; then
        printf "ERROR: kubelet DNS is already configured for aks-local-dns but our DNS_SERVICE_IP isn't set!\n"
        exit 1
    fi
    
    # Write the DNS service IP to the environment file
    printf "Configuring aks-local-dns to use DNS_SERVICE_IP=${KUBELET_CLUSTER_DNS_IP}\n"
    printf "\n\n#Added by aks-local-dns-startup.sh\nDNS_SERVICE_IP=${KUBELET_CLUSTER_DNS_IP}\n" >> /etc/default/aks-local-dns

    # Replace the kubelet --cluster-dns IP with our pod IP.
    printf "Configuring kubelet to use --cluster-dns=${LOCAL_POD_DNS_IP}\n"
    sed -ie "s/--cluster-dns=[^ \n]\+/--cluster-dns=${LOCAL_POD_DNS_IP}/" /etc/default/kubelet
fi

# Check if kubelet is using aks-local-dns, if it's already running
if systemctl is-active kubelet.service >/dev/null; then
    printf "Kubelet is running; checking if it needs to be restarted.\n"

    # Check kubelet's environment to see what --cluster-dns it was started with
    if grep -q -- "--cluster-dns=${LOCAL_POD_DNS_IP}" /proc/$(pgrep -f /usr/local/bin/kubelet)/environ; then
        printf "kubelet is pointing at aks-local-dns; not restarting.\n"
    else
        printf "Restarting kubelet to update --cluster-dns.\n"
        systemctl restart kubelet
    fi
else
    printf "kubelet not running yet; no restart needed.\n"
fi