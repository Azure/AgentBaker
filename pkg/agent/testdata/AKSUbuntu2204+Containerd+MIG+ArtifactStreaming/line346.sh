#! /bin/bash

function tsecho { echo "$(date -Iseconds) $*"; }
function tsechon { echo -n "$(date -Iseconds) $*"; }

INITDIR="$(pwd)"
CDIR="$(mktemp -d)"
pushd $CDIR >/dev/null

tsecho "Created temporary directory: $CDIR"

function cleanup {
  popd >/dev/null
  if [ "$DEBUG" != "1" ]; then
    tsecho "Cleaning up $CDIR..."
    rm -rf $CDIR
  fi
  tsecho "Log collection finished."
}
trap cleanup EXIT

tsecho "Collecting system information..."
mkdir collect collect/proc collect/proc/net
find /var/log /var/lib/waagent /etc -ls > $CDIR/collect/log_files.txt
lsblk > $CDIR/collect/diskinfo.txt
blkid >> $CDIR/collect/diskinfo.txt
conntrack -S > $CDIR/collect/conntrack.txt
ip -4 -j addr show > $CDIR/collect/ip4_addr.txt
ip -6 -j addr show > $CDIR/collect/ip6_addr.txt
iptables -L -v --line-numbers > $CDIR/collect/iptables.txt
ip6tables -L -v --line-numbers > $CDIR/collect/ip6tables.txt
cp /proc/cpuinfo /proc/meminfo /proc/mounts /proc/vmstat collect/proc/
cp /proc/net/* collect/proc/net/

zip -Z deflate -r $CDIR/aks_logs.zip collect/*

declare -a GLOBS
GLOBS+=(/var/lib/azure/provisioned)
GLOBS+=(/etc/fstab)
GLOBS+=(/etc/ssh/sshd_config)
GLOBS+=(/boot/grub*/grub.c*)
GLOBS+=(/boot/grub*/menu.lst)
GLOBS+=(/etc/*-release)
GLOBS+=(/etc/HOSTNAME)
GLOBS+=(/etc/hostname)
GLOBS+=(/etc/network/interfaces)
GLOBS+=(/etc/network/interfaces.d/*.cfg)
GLOBS+=(/etc/netplan/50-cloud-init.yaml)
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
GLOBS+=(/etc/sysconfig/SuSEfirewall2)
GLOBS+=(/etc/ufw/ufw.conf)
GLOBS+=(/etc/waagent.conf)
GLOBS+=(/var/lib/dhcp/dhclient.eth0.leases)
GLOBS+=(/var/lib/dhclient/dhclient-eth0.leases)
GLOBS+=(/var/lib/wicked/lease-eth0-dhcp-ipv4.xml)
GLOBS+=(/var/log/azure/custom-script/handler.log)
GLOBS+=(/var/log/azure/run-command/handler.log)
GLOBS+=(/var/lib/azure/ovf-env.xml)
GLOBS+=(/var/lib/azure/*/status/*.status)
GLOBS+=(/var/lib/azure/*/config/*.settings)
GLOBS+=(/var/lib/azure/*/config/HandlerState)
GLOBS+=(/var/lib/azure/*/config/HandlerStatus)
GLOBS+=(/var/lib/azure/SharedConfig.xml)
GLOBS+=(/var/lib/azure/ManagedIdentity-*.json)
GLOBS+=(/var/lib/azure/waagent_status.json)
GLOBS+=(/var/lib/azure/*/error.json)
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
GLOBS+=(/var/lib/azure/history/*.zip)

tsecho "Adding log files to zip archive..."
for file in ${GLOBS[*]}; do
  if test -e $file; then
    zip -Z deflate -u $CDIR/aks_logs.zip $file
    if [ $(stat --printf "%s" $CDIR/aks_logs.zip) -ge 104857600 ]; then
      echo "ZIP file size >= 100MB; removing last log file and terminating adding more files."
      zip -Z deflate -d $CDIR/aks_logs.zip $file
      break
    fi
  fi
done

tsecho "Log bundle size: $(du -hs $CDIR/aks_logs.zip)"

tsecho "Copying log bundle to WALA location..."
mkdir -p /var/lib/waagent/logcollector
cp $CDIR/aks_logs.zip /var/lib/waagent/logcollector/logs.zip

tsechon "Uploading log bundle: "
/opt/azure/containers/provision_send_logs.py
