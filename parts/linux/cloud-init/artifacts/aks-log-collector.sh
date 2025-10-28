#! /bin/bash
#
# AKS Log Collector
#
# This script collects information and logs that are useful to AKS engineering
# for support and uploads them to the Azure host via a private API. These log
# bundles are available to engineering when customers open a support case and
# are especially useful for troubleshooting failures of networking or
# kubernetes daemons.
#
# This script runs via a systemd unit and slice that limits it to low CPU
# priority and 128MB RAM, to avoid impacting other system functions.

# Shell options - remove non-matching globs, don't care about case, and use
# extended pattern matching
shopt -s nullglob nocaseglob extglob

# Fetch configuration from IMDS - expected is a JSON object in the aks-log-collector tag
# JSON body:
# {
#   "disable": false,
#   "files": [ "/etc/skel/.bashrc", "/etc/skel/.bash_profile" ],
#   "pod_log_namespaces": [ "default", "pahealy" ],
#   "iptables": false,
#   "nftables": false,
#   "netns": true
# }
CONFIG=$(curl -s -H Metadata:true --noproxy '*' 'http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01' | jq '.tagsList[] | select(.name=="aks-log-collector") | .value | fromjson')

# If the JSON object has "disable": true, then quit.
<<<"$CONFIG" jq -esRr 'try fromjson catch null | .disable? // false' >/dev/null && {
  printf "IMDS tag aks-log-collector disable==true, quitting.\n"
  exit 0
}

COLLECT_IPTABLES=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .iptables? // false')
COLLECT_NFTABLES=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .nftables? // false')
COLLECT_NETNS=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .netns? // false')

### START CONFIGURATION
ZIP="aks_logs.zip"

# Log bundle upload max size is limited to 100MB
MAX_SIZE=104857600

# File globs to include
# Smaller and more critical files are closer to the top so that we can be certain they're included.
declare -a GLOBS

# Add explicitly included files at the top to make sure they're included
for FILE in $(<<<"$CONFIG" jq -esRr 'try fromjson catch ("" | halt_error) | .files[]'); do
  GLOBS+=("${FILE}")
done

# Add extra_namespaces to the top to make sure those pod logs are included
for NAMESPACE in $(<<<"$CONFIG" jq -esRr 'try fromjson catch ("" | halt_error) | .pod_log_namespaces[]'); do
  GLOBS+=("/var/log/pods/${NAMESPACE}_*/**/*")
done

# AKS specific entries
GLOBS+=(/etc/cni/net.d/*)
GLOBS+=(/etc/containerd/*)
GLOBS+=(/etc/default/kubelet)
GLOBS+=(/etc/kubernetes/manifests/*)
GLOBS+=(/var/log/azure-cni*)
GLOBS+=(/var/log/azure-cns*)
GLOBS+=(/var/log/azure-ipam*)
GLOBS+=(/var/log/azure-vnet*)
GLOBS+=(/var/log/cilium-cni*)
GLOBS+=(/var/run/azure-vnet*)
GLOBS+=(/var/run/azure-cns*)

# GPU specific entries
GLOBS+=(/var/log/nvidia*.log)
GLOBS+=(/var/log/azure/nvidia*.log)
GLOBS+=(/var/log/fabricmanager*.log)

# based on MANIFEST_FULL from Azure Linux Agent's log collector
# https://github.com/Azure/WALinuxAgent/blob/master/azurelinuxagent/common/logcollector_manifests.py
GLOBS+=(/var/lib/waagent/provisioned)
GLOBS+=(/etc/fstab)
GLOBS+=(/etc/ssh/sshd_config)
GLOBS+=(/boot/grub*/grub.c*)
GLOBS+=(/boot/grub*/menu.lst)
GLOBS+=(/etc/*-release)
GLOBS+=(/etc/HOSTNAME)
GLOBS+=(/etc/hostname)
GLOBS+=(/etc/apt/sources.list)
GLOBS+=(/etc/apt/sources.list.d/*)
GLOBS+=(/etc/network/interfaces)
GLOBS+=(/etc/network/interfaces.d/*.cfg)
GLOBS+=(/etc/netplan/*.yaml)
GLOBS+=(/etc/nsswitch.conf)
GLOBS+=(/etc/resolv.conf)
GLOBS+=(/run/systemd/resolve/stub-resolv.conf)
GLOBS+=(/run/resolvconf/resolv.conf)
GLOBS+=(/etc/sysconfig/iptables)
GLOBS+=(/etc/sysconfig/network)
GLOBS+=(/etc/sysconfig/network/ifcfg-eth*)
GLOBS+=(/etc/sysconfig/network/routes)
GLOBS+=(/etc/sysconfig/network-scripts/ifcfg-eth*)
GLOBS+=(/etc/sysconfig/network-scripts/route-eth*)
GLOBS+=(/etc/ufw/ufw.conf)
GLOBS+=(/etc/waagent.conf)
GLOBS+=(/var/lib/hyperv/.kvp_pool_*)
GLOBS+=(/var/lib/dhcp/dhclient.eth0.leases)
GLOBS+=(/var/lib/dhclient/dhclient-eth0.leases)
GLOBS+=(/var/lib/wicked/lease-eth0-dhcp-ipv4.xml)
GLOBS+=(/var/log/azure/custom-script/handler.log)
GLOBS+=(/var/log/azure/run-command/handler.log)
GLOBS+=(/var/lib/waagent/ovf-env.xml)
GLOBS+=(/var/lib/waagent/*/status/*.status)
GLOBS+=(/var/lib/waagent/*/config/*.settings)
GLOBS+=(/var/lib/waagent/*/config/HandlerState)
GLOBS+=(/var/lib/waagent/*/config/HandlerStatus)
GLOBS+=(/var/lib/waagent/SharedConfig.xml)
GLOBS+=(/var/lib/waagent/ManagedIdentity-*.json)
GLOBS+=(/var/lib/waagent/waagent_status.json)
GLOBS+=(/var/lib/waagent/*/error.json)
GLOBS+=(/var/log/cloud-init*)
GLOBS+=(/var/log/azure/*/*)
GLOBS+=(/var/log/azure/*/*/*)
GLOBS+=(/var/log/syslog*)
GLOBS+=(/var/log/rsyslog*)
GLOBS+=(/var/log/messages*)
GLOBS+=(/var/log/kern*)
GLOBS+=(/var/log/dmesg*)
GLOBS+=(/var/log/dpkg*)
GLOBS+=(/var/log/yum*)
GLOBS+=(/var/log/boot*)
GLOBS+=(/var/log/auth*)
GLOBS+=(/var/log/secure*)
GLOBS+=(/var/log/journal*)

### END CONFIGURATION

command -v zip >/dev/null || {
  echo "Error: zip utility not found. Please install zip."
  exit 255
}

# Create a temporary directory to store results in
WORKDIR="$(mktemp -d)"
# check if tmp dir was created
if [ -z "$WORKDIR" ] || [ "$WORKDIR" = "/" ] || [ "$WORKDIR" = "/tmp" ] || [ ! -d "$WORKDIR" ]; then
  echo "ERROR: Could not create temporary working directory."
  exit 1
fi
cd $WORKDIR || { echo "Failed to change directory to $WORKDIR. Exiting."; exit 1; }
echo "Created temporary directory: $WORKDIR"

# Function to clean up the output directory and log termination
function cleanup {
  # Make sure WORKDIR is a proper temp directory so we don't rm something we shouldn't
  # shellcheck disable=SC3010
  if [[ $WORKDIR =~ ^/tmp/tmp\.[a-zA-Z0-9]+$ ]]; then
    if [ "$DEBUG" != "1" ]; then
      echo "Cleaning up $WORKDIR..."
      rm -rf "$WORKDIR"
    else
      echo "DEBUG active or $WORKDIR looks wrong; leaving $WORKDIR behind."
    fi
  else
    echo "ERROR: WORKDIR ($WORKDIR) doesn't look like a proper mktemp directory; not removing it for safety reasons!"
    exit 255
  fi
  echo "Log collection finished."
}

# Execute the cleanup function if the script terminates
trap "exit 1" HUP INT PIPE QUIT TERM
trap "cleanup" EXIT

# This function runs a command and dumps its output to a named pipe, then includes that named
# pipe into a zip file. It's used to include command output in the ZIP file without taking up
# any disk space aside from the ZIP file itself.
# USAGE: collectToZip FILENAME CMDTORUN
function collectToZip {
  command -v "${2}" >/dev/null || { printf "%s not found, skipping.\n" "${2}"; return; }
  mkfifo "${1}"
  "${@:2}" >"${1}" 2>&1 &
  zip -gumDZ deflate --fifo "${ZIP}" "${1}"
}

# Collect general system information
echo "Collecting system information..."
mkdir collect

# Collect general information and create the ZIP in the first place
zip -DZ deflate "${ZIP}" /proc/@(cmdline|cpuinfo|filesystems|interrupts|loadavg|meminfo|modules|mounts|slabinfo|stat|uptime|version*|vmstat) /proc/net/*

# Include some disk listings
collectToZip collect/file_listings.txt find /dev /etc /var/lib/waagent /var/log -ls

# Collect system information
collectToZip collect/blkid.txt blkid $(find /dev -type b ! -name 'sr*')
collectToZip collect/du_bytes.txt df -al
collectToZip collect/du_inodes.txt df -ail
collectToZip collect/diskinfo.txt lsblk
collectToZip collect/lscpu.txt lscpu
collectToZip collect/lscpu.json lscpu -J
collectToZip collect/lshw.txt lshw
collectToZip collect/lshw.json lshw -json
collectToZip collect/lsipc.txt lsipc
collectToZip collect/lsns.json lsns -J --output-all
collectToZip collect/lspci.txt lspci -vkPP
collectToZip collect/lsscsi.txt lsscsi -vv
collectToZip collect/lsvmbus.txt lsvmbus -vv
collectToZip collect/sysctl.txt sysctl -a
collectToZip collect/systemctl-status.txt systemctl status --all -fr

# Collect logs of the Nvidia services if present
collectToZip collect/journalctl_nvidia-dcgm.txt journalctl -u nvidia-dcgm --no-pager
collectToZip collect/journalctl_nvidia-dcgm-exporter.txt journalctl -u nvidia-dcgm-exporter --no-pager
collectToZip collect/journalctl_nvidia-device-plugin.txt journalctl -u nvidia-device-plugin --no-pager

# Collect container runtime information
collectToZip collect/crictl_version.txt crictl version
collectToZip collect/crictl_info.json crictl info -o json
collectToZip collect/crictl_images.json crictl images -o json
collectToZip collect/crictl_imagefsinfo.json crictl imagefsinfo -o json
collectToZip collect/crictl_pods.json crictl pods -o json
collectToZip collect/crictl_ps.json crictl ps -o json
collectToZip collect/crictl_stats.json crictl stats -o json
collectToZip collect/crictl_statsp.json crictl statsp -o json

# Collect network information
collectToZip collect/conntrack.txt conntrack -L
collectToZip collect/conntrack_stats.txt conntrack -S
collectToZip collect/ip_4_addr.json ip -4 -d -j addr show
collectToZip collect/ip_4_neighbor.json ip -4 -d -j neighbor show
collectToZip collect/ip_4_route.json ip -4 -d -j route show
collectToZip collect/ip_4_tcpmetrics.json ip -4 -d -j tcpmetrics show
collectToZip collect/ip_6_addr.json ip -6 -d -j addr show table all
collectToZip collect/ip_6_neighbor.json ip -6 -d -j neighbor show
collectToZip collect/ip_6_route.json ip -6 -d -j route show table all
collectToZip collect/ip_6_tcpmetrics.json ip -6 -d -j tcpmetrics show
collectToZip collect/ip_link.json ip -d -j link show
collectToZip collect/ip_netconf.json ip -d -j netconf show
collectToZip collect/ip_netns.json ip -d -j netns show
collectToZip collect/ip_rule.json ip -d -j rule show

if [ "${COLLECT_IPTABLES}" = "true" ]; then
  collectToZip collect/iptables.txt iptables -L -vn --line-numbers
  collectToZip collect/ip6tables.txt ip6tables -L -vn --line-numbers
fi

if [ "${COLLECT_NFTABLES}" = "true" ]; then
  collectToZip collect/nftables.txt nft -n list ruleset 2>&1
fi

collectToZip collect/ss.txt ss -anoempiO --cgroup
collectToZip collect/ss_stats.txt ss -s

# Collect network information from network namespaces
if [ "${COLLECT_NETNS}" = "true" ]; then
  for NETNS in $(ip -j netns list | jq -r '.[].name'); do
    mkdir -p "collect/ip_netns_${NETNS}/"
    collectToZip collect/ip_netns_${NETNS}/conntrack.txt ip netns exec "${NETNS}" conntrack -L
    collectToZip collect/ip_netns_${NETNS}/conntrack_stats.txt ip netns exec "${NETNS}" conntrack -S
    collectToZip collect/ip_netns_${NETNS}/ip_4_addr.json ip -n "${NETNS}" -4 -d -j addr show
    collectToZip collect/ip_netns_${NETNS}/ip_4_neighbor.json ip -n "${NETNS}" -4 -d -j neighbor show
    collectToZip collect/ip_netns_${NETNS}/ip_4_route.json ip -n "${NETNS}" -4 -d -j route show
    collectToZip collect/ip_netns_${NETNS}/ip_4_tcpmetrics.json ip -n "${NETNS}" -4 -d -j tcpmetrics show
    collectToZip collect/ip_netns_${NETNS}/ip_6_addr.json ip -n "${NETNS}" -6 -d -j addr show table all
    collectToZip collect/ip_netns_${NETNS}/ip_6_neighbor.json ip -n "${NETNS}" -6 -d -j neighbor show
    collectToZip collect/ip_netns_${NETNS}/ip_6_route.json ip -n "${NETNS}" -6 -d -j route show table all
    collectToZip collect/ip_netns_${NETNS}/ip_6_tcpmetrics.json ip -n "${NETNS}" -6 -d -j tcpmetrics show
    collectToZip collect/ip_netns_${NETNS}/ip_link.json ip -n "${NETNS}" -d -j link show
    collectToZip collect/ip_netns_${NETNS}/ip_netconf.json ip -n "${NETNS}" -d -j netconf show
    collectToZip collect/ip_netns_${NETNS}/ip_netns.json ip -n "${NETNS}" -d -j netns show
    collectToZip collect/ip_netns_${NETNS}/ip_rule.json ip -n "${NETNS}" -d -j rule show
    if [ "${COLLECT_IPTABLES}" = "true" ]; then
      collectToZip collect/ip_netns_${NETNS}/iptables.txt ip netns exec "${NETNS}" iptables -L -vn --line-numbers
      collectToZip collect/ip_netns_${NETNS}/ip6tables.txt ip netns exec "${NETNS}" ip6tables -L -vn --line-numbers
    fi
    if [ "${COLLECT_NFTABLES}" = "true" ]; then
      collectToZip collect/ip_netns_${NETNS}/nftables.txt nft -n list ruleset
    fi
    collectToZip collect/ip_netns_${NETNS}/ss.txt ip netns exec "${NETNS}" ss -anoempiO --cgroup
    collectToZip collect/ip_netns_${NETNS}/ss_stats.txt ip netns exec "${NETNS}" ss -s
  done
fi

# Add each file sequentially to the zip archive. This is slightly less efficient then adding them
# all at once, but allows us to easily check when we've exceeded the maximum file size and stop
# adding things to the archive.
echo "Adding log files to zip archive..."
for file in ${GLOBS[*]}; do
  if test -e $file; then
    zip -g -DZ deflate -u "${ZIP}" $file -x '*.sock'

    # The API for the log bundle has a max file size (defined above, usually 100MB), so if
    # adding this last file made the zip go over that size, remove that file and stop processing.
    FILE_SIZE=$(stat --printf "%s" ${ZIP})
    if [ "$FILE_SIZE" -ge "$MAX_SIZE" ]; then
      echo "WARNING: ZIP file size $FILE_SIZE >= $MAX_SIZE; removing last log file and terminating adding more files."
      zip -d "${ZIP}" $file
      break
    fi
  fi
done

# Copy the log bundle to the expected path for uploading, then trigger
# the upload script to push it to the host storage location.
echo "Log bundle size: $(du -hs ${ZIP})"
mkdir -p /var/lib/waagent/logcollector
cp ${ZIP} /var/lib/waagent/logcollector/logs.zip
echo -n "Uploading log bundle: "
/usr/bin/env python3 /opt/azure/containers/aks-log-collector-send.py
