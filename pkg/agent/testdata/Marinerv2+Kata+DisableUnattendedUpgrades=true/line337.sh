#! /bin/bash
#
#
#

MAX_SIZE=104857600

shopt -s nullglob nocaseglob extglob

WORKDIR="$(mktemp -d)"
if [[ ! "$WORKDIR" || "$WORKDIR" == "/" || "$WORKDIR" == "/tmp" || ! -d "$WORKDIR" ]]; then
  echo "ERROR: Could not create temporary working directory."
  exit 1
fi
cd $WORKDIR
echo "Created temporary directory: $WORKDIR"

function cleanup {
  if [[ $WORKDIR =~ ^/tmp/tmp\.[a-zA-Z0-9]+$ ]]; then
    if [[ "$DEBUG" != "1" ]]; then
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

trap "exit 1"  HUP INT PIPE QUIT TERM
trap "cleanup" EXIT

echo "Collecting system information..."
mkdir collect collect/proc collect/proc/net

find /dev /etc /var/lib/waagent /var/log -ls >collect/file_listings.txt 2>&1

dpkg -l >collect/dpkg.txt 2>&1
lsblk >collect/diskinfo.txt 2>&1
blkid >>collect/diskinfo.txt 2>&1
lscpu >collect/lscpu.txt 2>&1
lscpu -J >collect/lscpu.json 2>&1
lshw >collect/lshw.txt 2>&1
lshw -json >collect/lshw.json 2>&1
lsipc >collect/lsipc.txt 2>&1
lsns -J --output-all >collect/lsns.json 2>&1
lspci -vkPP >collect/lspci.txt 2>&1
lsscsi -vv >collect/lsscsi.txt 2>&1
lsvmbus -vv >collect/lsvmbus.txt 2>&1
sysctl -a >collect/sysctl.txt 2>&1
systemctl status --all -fr >collect/systemctl-status.txt 2>&1

crictl version >collect/crictl_version.txt 2>&1
crictl info -o json >collect/crictl_info.json 2>&1
crictl images -o json >collect/crictl_images.json 2>&1
crictl imagefsinfo -o json >collect/crictl_imagefsinfo.json 2>&1
crictl pods -o json >collect/crictl_pods.json 2>&1
crictl ps -o json >collect/crictl_ps.json 2>&1
crictl stats -o json >collect/crictl_stats.json 2>&1
crictl statsp -o json >collect/crictl_statsp.json 2>&1

conntrack -L >collect/conntrack.txt 2>&1
conntrack -S >>collect/conntrack.txt 2>&1
ip -4 -d -j addr show >collect/ip_4_addr.json 2>&1
ip -4 -d -j neighbor show >collect/ip_4_neighbor.json 2>&1
ip -4 -d -j route show >collect/ip_4_route.json 2>&1
ip -4 -d -j tcpmetrics show >collect/ip_4_tcpmetrics.json 2>&1
ip -6 -d -j addr show table all >collect/ip_6_addr.json 2>&1
ip -6 -d -j neighbor show >collect/ip_6_neighbor.json 2>&1
ip -6 -d -j route show table all >collect/ip_6_route.json 2>&1
ip -6 -d -j tcpmetrics show >collect/ip_6_tcpmetrics.json 2>&1
ip -d -j link show >collect/ip_link.json 2>&1
ip -d -j netconf show >collect/ip_netconf.json 2>&1
ip -d -j netns show >collect/ip_netns.json 2>&1
ip -d -j rule show >collect/ip_rule.json 2>&1
iptables -L -vn --line-numbers >collect/iptables.txt 2>&1
ip6tables -L -vn --line-numbers >collect/ip6tables.txt 2>&1
nft -jn list ruleset >collect/nftables.json 2>&1
ss -anoempiO --cgroup >collect/ss.txt 2>&1
ss -s >>collect/ss.txt 2>&1

ip -all netns exec /bin/bash -x -c "
	conntrack -L 2>&1;
	conntrack -S 2>&1;
	ip -4 -d -j addr show 2>&1;
	ip -4 -d -j neighbor show 2>&1;
	ip -4 -d -j route show 2>&1;
	ip -4 -d -j tcpmetrics show 2>&1;
	ip -6 -d -j addr show table all 2>&1;
	ip -6 -d -j neighbor show 2>&1;
	ip -6 -d -j route show table all 2>&1;
	ip -6 -d -j tcpmetrics show 2>&1;
	ip -d -j link show 2>&1;
	ip -d -j netconf show 2>&1;
	ip -d -j netns show 2>&1;
	ip -d -j rule show 2>&1;
	iptables -L -vn --line-numbers 2>&1;
	ip6tables -L -vn --line-numbers 2>&1;
	nft -jn list ruleset 2>&1;
	ss -anoempiO --cgroup 2>&1;
	ss -s 2>&1;
" >collect/ip_netns_commands.txt 2>&1

cp /proc/@(cmdline|cpuinfo|filesystems|interrupts|loadavg|meminfo|modules|mounts|slabinfo|stat|uptime|version*|vmstat) collect/proc/
cp -r /proc/net/* collect/proc/net/

zip -DZ deflate -r aks_logs.zip collect/*

declare -a GLOBS

GLOBS+=(/etc/cni/net.d/*)
GLOBS+=(/etc/containerd/*)
GLOBS+=(/etc/default/kubelet)
GLOBS+=(/etc/kubernetes/manifests/*)
GLOBS+=(/var/lib/kubelet/kubeconfig)

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

echo "Adding log files to zip archive..."
for file in ${GLOBS[*]}; do
  if test -e $file; then
    zip -DZ deflate -u aks_logs.zip $file

    FILE_SIZE=$(stat --printf "%s" aks_logs.zip)
    if [ $FILE_SIZE -ge $MAX_SIZE ]; then
      echo "WARNING: ZIP file size $FILE_SIZE >= $MAX_SIZE; removing last log file and terminating adding more files."
      zip -d aks_logs.zip $file
      break
    fi
  fi
done

echo "Log bundle size: $(du -hs aks_logs.zip)"
mkdir -p /var/lib/waagent/logcollector
cp aks_logs.zip /var/lib/waagent/logcollector/logs.zip
echo -n "Uploading log bundle: "
/usr/bin/env python3 /opt/azure/containers/aks-log-collector-send.py
