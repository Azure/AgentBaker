// Code generated for package templates by go-bindata DO NOT EDIT. (@generated)
// sources:
// linux/cloud-init/artifacts/10-bindmount.conf
// linux/cloud-init/artifacts/10-cgroupv2.conf
// linux/cloud-init/artifacts/10-componentconfig.conf
// linux/cloud-init/artifacts/10-containerd.conf
// linux/cloud-init/artifacts/10-httpproxy.conf
// linux/cloud-init/artifacts/10-tlsbootstrap.conf
// linux/cloud-init/artifacts/aks-logrotate-override.conf
// linux/cloud-init/artifacts/aks-logrotate.service
// linux/cloud-init/artifacts/aks-logrotate.sh
// linux/cloud-init/artifacts/aks-logrotate.timer
// linux/cloud-init/artifacts/aks-rsyslog
// linux/cloud-init/artifacts/apt-preferences
// linux/cloud-init/artifacts/bind-mount.service
// linux/cloud-init/artifacts/bind-mount.sh
// linux/cloud-init/artifacts/block_wireserver.sh
// linux/cloud-init/artifacts/cgroup-memory-telemetry.service
// linux/cloud-init/artifacts/cgroup-memory-telemetry.sh
// linux/cloud-init/artifacts/cgroup-memory-telemetry.timer
// linux/cloud-init/artifacts/cgroup-pressure-telemetry.service
// linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh
// linux/cloud-init/artifacts/cgroup-pressure-telemetry.timer
// linux/cloud-init/artifacts/ci-syslog-watcher.path
// linux/cloud-init/artifacts/ci-syslog-watcher.service
// linux/cloud-init/artifacts/ci-syslog-watcher.sh
// linux/cloud-init/artifacts/cis.sh
// linux/cloud-init/artifacts/containerd-monitor.service
// linux/cloud-init/artifacts/containerd-monitor.timer
// linux/cloud-init/artifacts/containerd.service
// linux/cloud-init/artifacts/containerd_exec_start.conf
// linux/cloud-init/artifacts/crictl.yaml
// linux/cloud-init/artifacts/cse_cmd.sh
// linux/cloud-init/artifacts/cse_config.sh
// linux/cloud-init/artifacts/cse_helpers.sh
// linux/cloud-init/artifacts/cse_install.sh
// linux/cloud-init/artifacts/cse_main.sh
// linux/cloud-init/artifacts/cse_redact_cloud_config.py
// linux/cloud-init/artifacts/cse_send_logs.py
// linux/cloud-init/artifacts/cse_start.sh
// linux/cloud-init/artifacts/dhcpv6.service
// linux/cloud-init/artifacts/disk_queue.service
// linux/cloud-init/artifacts/docker-monitor.service
// linux/cloud-init/artifacts/docker-monitor.timer
// linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf
// linux/cloud-init/artifacts/enable-dhcpv6.sh
// linux/cloud-init/artifacts/ensure-no-dup.service
// linux/cloud-init/artifacts/ensure-no-dup.sh
// linux/cloud-init/artifacts/etc-issue
// linux/cloud-init/artifacts/etc-issue.net
// linux/cloud-init/artifacts/health-monitor.sh
// linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh
// linux/cloud-init/artifacts/init-aks-custom-cloud.sh
// linux/cloud-init/artifacts/ipv6_nftables
// linux/cloud-init/artifacts/ipv6_nftables.service
// linux/cloud-init/artifacts/ipv6_nftables.sh
// linux/cloud-init/artifacts/kms.service
// linux/cloud-init/artifacts/kubelet-monitor.service
// linux/cloud-init/artifacts/kubelet-monitor.timer
// linux/cloud-init/artifacts/kubelet.service
// linux/cloud-init/artifacts/manifest.json
// linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh
// linux/cloud-init/artifacts/mariner/cse_install_mariner.sh
// linux/cloud-init/artifacts/mariner/pam-d-system-auth
// linux/cloud-init/artifacts/mariner/pam-d-system-password
// linux/cloud-init/artifacts/mariner/update_certs_mariner.service
// linux/cloud-init/artifacts/mig-partition.service
// linux/cloud-init/artifacts/mig-partition.sh
// linux/cloud-init/artifacts/modprobe-CIS.conf
// linux/cloud-init/artifacts/nvidia-device-plugin.service
// linux/cloud-init/artifacts/nvidia-docker-daemon.json
// linux/cloud-init/artifacts/nvidia-modprobe.service
// linux/cloud-init/artifacts/pam-d-common-auth
// linux/cloud-init/artifacts/pam-d-common-auth-2204
// linux/cloud-init/artifacts/pam-d-common-password
// linux/cloud-init/artifacts/pam-d-su
// linux/cloud-init/artifacts/profile-d-cis.sh
// linux/cloud-init/artifacts/pwquality-CIS.conf
// linux/cloud-init/artifacts/reconcile-private-hosts.service
// linux/cloud-init/artifacts/reconcile-private-hosts.sh
// linux/cloud-init/artifacts/rsyslog-d-60-CIS.conf
// linux/cloud-init/artifacts/setup-custom-search-domains.sh
// linux/cloud-init/artifacts/sshd_config
// linux/cloud-init/artifacts/sshd_config_1604
// linux/cloud-init/artifacts/sshd_config_1804_fips
// linux/cloud-init/artifacts/sync-container-logs.service
// linux/cloud-init/artifacts/sync-container-logs.sh
// linux/cloud-init/artifacts/sysctl-d-60-CIS.conf
// linux/cloud-init/artifacts/teleportd.service
// linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh
// linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh
// linux/cloud-init/artifacts/update_certs.path
// linux/cloud-init/artifacts/update_certs.service
// linux/cloud-init/artifacts/update_certs.sh
// linux/cloud-init/nodecustomdata.yml
// windows/csecmd.ps1
// windows/kuberneteswindowssetup.ps1
// windows/sendlogs.ps1
// windows/windowscsehelper.ps1
package templates

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _linuxCloudInitArtifacts10BindmountConf = []byte(`[Unit]
Requires=bind-mount.service
After=bind-mount.service
`)

func linuxCloudInitArtifacts10BindmountConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10BindmountConf, nil
}

func linuxCloudInitArtifacts10BindmountConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10BindmountConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-bindmount.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifacts10Cgroupv2Conf = []byte(`[Service]
Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"
`)

func linuxCloudInitArtifacts10Cgroupv2ConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10Cgroupv2Conf, nil
}

func linuxCloudInitArtifacts10Cgroupv2Conf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10Cgroupv2ConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-cgroupv2.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifacts10ComponentconfigConf = []byte(`[Service]
Environment="KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json"
`)

func linuxCloudInitArtifacts10ComponentconfigConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10ComponentconfigConf, nil
}

func linuxCloudInitArtifacts10ComponentconfigConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10ComponentconfigConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-componentconfig.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifacts10ContainerdConf = []byte(`[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--container-runtime=remote --runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service"
`)

func linuxCloudInitArtifacts10ContainerdConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10ContainerdConf, nil
}

func linuxCloudInitArtifacts10ContainerdConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10ContainerdConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-containerd.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifacts10HttpproxyConf = []byte(`[Service]
EnvironmentFile=/etc/environment
`)

func linuxCloudInitArtifacts10HttpproxyConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10HttpproxyConf, nil
}

func linuxCloudInitArtifacts10HttpproxyConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10HttpproxyConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-httpproxy.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifacts10TlsbootstrapConf = []byte(`[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
`)

func linuxCloudInitArtifacts10TlsbootstrapConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifacts10TlsbootstrapConf, nil
}

func linuxCloudInitArtifacts10TlsbootstrapConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifacts10TlsbootstrapConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/10-tlsbootstrap.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAksLogrotateOverrideConf = []byte(`[Timer]
OnCalendar=
OnCalendar=*-*-* *:00:00`)

func linuxCloudInitArtifactsAksLogrotateOverrideConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAksLogrotateOverrideConf, nil
}

func linuxCloudInitArtifactsAksLogrotateOverrideConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAksLogrotateOverrideConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/aks-logrotate-override.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAksLogrotateService = []byte(`[Unit]
Description=runs the logrotate utility for log rotation with a custom configuration
[Service]
ExecStart=/usr/local/bin/logrotate.sh`)

func linuxCloudInitArtifactsAksLogrotateServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAksLogrotateService, nil
}

func linuxCloudInitArtifactsAksLogrotateService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAksLogrotateServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/aks-logrotate.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAksLogrotateSh = []byte(`#!/bin/sh
# This script was originally generated by logrotate automatically and placed in /etc/cron.daily/logrotate
# This will be saved on the target VM within /usr/local/bin/logrotate.sh and invoked by logrotate.service

# Clean non existent log file entries from status file
cd /var/lib/logrotate
test -e status || touch status
head -1 status > status.clean
sed 's/"//g' status | while read logfile date
do
    [ -e "$logfile" ] && echo "\"$logfile\" $date"
done >> status.clean
mv status.clean status

test -x /usr/sbin/logrotate || exit 0
/usr/sbin/logrotate --verbose /etc/logrotate.conf`)

func linuxCloudInitArtifactsAksLogrotateShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAksLogrotateSh, nil
}

func linuxCloudInitArtifactsAksLogrotateSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAksLogrotateShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/aks-logrotate.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAksLogrotateTimer = []byte(`[Unit]
Description=a timer that runs the logrotate-aks service on the top of every hour with a random delay
[Timer]
OnCalendar=*-*-* *:00:00
RandomizedDelaySec=3m
[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsAksLogrotateTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAksLogrotateTimer, nil
}

func linuxCloudInitArtifactsAksLogrotateTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAksLogrotateTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/aks-logrotate.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAksRsyslog = []byte(`/var/log/syslog
/var/log/messages
/var/log/secure
/var/log/kern.log
{
  rotate 5
  daily
  maxsize 300M
  missingok
  notifempty
  compress
  delaycompress
  sharedscripts
  postrotate
      systemctl kill -s HUP rsyslog.service
  endscript
}

/var/log/mail.info
/var/log/mail.warn
/var/log/mail.err
/var/log/mail.log
/var/log/daemon.log
/var/log/auth.log
/var/log/user.log
/var/log/lpr.log
/var/log/cron.log
/var/log/debug
/var/log/warn
{
  rotate 7
  daily
  maxsize 50M
  missingok
  notifempty
  compress
  delaycompress
  sharedscripts
  postrotate
      systemctl kill -s HUP rsyslog.service
  endscript
}`)

func linuxCloudInitArtifactsAksRsyslogBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAksRsyslog, nil
}

func linuxCloudInitArtifactsAksRsyslog() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAksRsyslogBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/aks-rsyslog", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsAptPreferences = []byte(``)

func linuxCloudInitArtifactsAptPreferencesBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAptPreferences, nil
}

func linuxCloudInitArtifactsAptPreferences() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAptPreferencesBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/apt-preferences", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsBindMountService = []byte(`[Unit]
Description=Bind mount kubelet data
[Service]
Restart=on-failure
RemainAfterExit=yes
ExecStart=/bin/bash /opt/azure/containers/bind-mount.sh

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsBindMountServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsBindMountService, nil
}

func linuxCloudInitArtifactsBindMountService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsBindMountServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/bind-mount.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsBindMountSh = []byte(`#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
set -x

# Bind mount kubelet to ephemeral storage on startup, as necessary.
#
# This fixes an issue with kubelet's ability to detect allocatable
# capacity for Node ephemeral-storage. On Azure, ephemeral-storage
# should correspond to the temp disk if a VM has one. This script makes
# that true by bind mounting the temp disk to /var/lib/kubelet, so
# kubelet thinks it's located on the temp disk (/dev/sdb). This results
# in correct calculation of ephemeral-storage capacity.

# if aks ever supports alternatives besides temp disk
# this mount point will need to be updated
MOUNT_POINT="/mnt/aks"

KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet"

mkdir -p "${MOUNT_POINT}"

# only move the kubelet directory to alternate location on first boot.
SENTINEL_FILE="/opt/azure/containers/bind-sentinel"
if [ ! -e "$SENTINEL_FILE" ]; then
    mv "$KUBELET_DIR" "$MOUNT_POINT"
    touch "$SENTINEL_FILE"
fi

# on every boot, bind mount the kubelet directory back to the expected
# location before kubelet itself may start.
mkdir -p "${KUBELET_DIR}"
mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}" 
chmod a+w "${KUBELET_DIR}"`)

func linuxCloudInitArtifactsBindMountShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsBindMountSh, nil
}

func linuxCloudInitArtifactsBindMountSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsBindMountShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/bind-mount.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsBlock_wireserverSh = []byte(`#!/bin/bash
# Disallow container from reaching out to the special IP address 168.63.129.16
# for TCP protocol (which http uses)
#
# 168.63.129.16 contains protected settings that have priviledged info.
#
# The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
#
# Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
# for DNS.
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
`)

func linuxCloudInitArtifactsBlock_wireserverShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsBlock_wireserverSh, nil
}

func linuxCloudInitArtifactsBlock_wireserverSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsBlock_wireserverShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/block_wireserver.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupMemoryTelemetryService = []byte(`[Unit]
Description=Emit system cgroup memory telemetry

[Service]
Type=oneshot
ExecStart=/bin/bash /opt/scripts/cgroup-memory-telemetry.sh`)

func linuxCloudInitArtifactsCgroupMemoryTelemetryServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupMemoryTelemetryService, nil
}

func linuxCloudInitArtifactsCgroupMemoryTelemetryService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupMemoryTelemetryServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-memory-telemetry.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupMemoryTelemetrySh = []byte(`#!/bin/bash

set -o nounset
set -o pipefail

find /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/ -mtime +5 -type f -delete

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME=$(date)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
eventlevel="Microsoft.Azure.Extensions.CustomScript-1.23"

CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then

    VERSION="cgroupv2"
    TASK_NAME="AKS.Runtime.memory_telemetry_cgroupv2"
    CGROUP="/sys/fs/cgroup"

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(if [ -f "${CGROUP}/system.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/system.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg AZURE_SLICE_MEMORY "$(if [ -f "${CGROUP}/azure.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg KUBEPODS_SLICE_MEMORY "$(if [ -f "${CGROUP}/kubepods.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg USER_SLICE_MEMORY "$(if [ -f "${CGROUP}/user.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/user.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg CONTAINERD_MEMORY "$(if [ -f "${CGROUP}/${CSLICE}/containerd.service/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg KUBELET_MEMORY "$(if [ -f "${CGROUP}/${KSLICE}/kubelet.service/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg EMPLOYED_MEMORY "$(if [ -f "${CGROUP}/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}' | awk '{print $1 * 1000}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(if [ -f "${CGROUP}/kubepods.slice/memory.max" ]; then cat ${CGROUP}/kubepods.slice/memory.max; else echo "Not Found"; fi)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )
    
elif [ "$CGROUP_VERSION" = "tmpfs" ]; then

    VERSION="cgroupv1"
    TASK_NAME="AKS.Runtime.memory_telemetry_cgroupv1"
    CGROUP="/sys/fs/cgroup/memory"

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(if [ -f ${CGROUP}/system.slice/memory.stat ]; then expr $(cat ${CGROUP}/system.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg AZURE_SLICE_MEMORY "$(if [ -f ${CGROUP}/azure.slice/memory.stat ]; then expr $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg KUBEPODS_SLICE_MEMORY "$(if [ -f ${CGROUP}/kubepods/memory.stat ]; then expr $(cat ${CGROUP}/kubepods/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/kubepods/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg USER_SLICE_MEMORY "$(if [ -f ${CGROUP}/user.slice/memory.stat ]; then expr $(cat ${CGROUP}/user.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg CONTAINERD_MEMORY "$(if [ -f ${CGROUP}/${CSLICE}/containerd.service/memory.stat ]; then expr $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg KUBELET_MEMORY "$(if [ -f ${CGROUP}/${KSLICE}/kubelet.service/memory.stat ]; then expr $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg EMPLOYED_MEMORY "$(if [ -f ${CGROUP}/memory.stat ]; then expr $(cat ${CGROUP}/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}' | awk '{print $1 * 1000}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(if [ -f ${CGROUP}/kubepods/memory.limit_in_bytes ]; then cat ${CGROUP}/kubepods/memory.limit_in_bytes; else echo "Not Found"; fi)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )

else
    echo "Unexpected cgroup type. Exiting"
    exit 1
fi

memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

message_string=$( jq -n \
    --arg CGROUPV "${VERSION}" \
    --argjson MEMORY "$(echo $memory_string)" \
    '{ CgroupVersion: $CGROUPV, Memory: $MEMORY } | tostring'
)

message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${STARTTIME_FORMATTED}" \
    --arg OperationId   "${ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "${TASK_NAME}" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message    "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json`)

func linuxCloudInitArtifactsCgroupMemoryTelemetryShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupMemoryTelemetrySh, nil
}

func linuxCloudInitArtifactsCgroupMemoryTelemetrySh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupMemoryTelemetryShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-memory-telemetry.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupMemoryTelemetryTimer = []byte(`[Unit]
Description=emit memory telemetry

[Timer]
OnBootSec=0min
OnCalendar=*-*-* *:0/5:0
Unit=cgroup-memory-telemetry.service

[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsCgroupMemoryTelemetryTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupMemoryTelemetryTimer, nil
}

func linuxCloudInitArtifactsCgroupMemoryTelemetryTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupMemoryTelemetryTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-memory-telemetry.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupPressureTelemetryService = []byte(`[Unit]
Description=Emit system cgroup pressure telemetry

[Service]
Type=oneshot
ExecStart=/bin/bash /opt/scripts/cgroup-pressure-telemetry.sh`)

func linuxCloudInitArtifactsCgroupPressureTelemetryServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupPressureTelemetryService, nil
}

func linuxCloudInitArtifactsCgroupPressureTelemetryService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupPressureTelemetryServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-pressure-telemetry.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupPressureTelemetrySh = []byte(`#!/bin/bash

set -o nounset
set -o pipefail

find /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/ -mtime +5 -type f -delete

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME=$(date)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
eventlevel="Microsoft.Azure.Extensions.CustomScript-1.23"

CGROUP="/sys/fs/cgroup"
CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then

    VERSION="cgroupv2"
    TASK_NAME="AKS.Runtime.pressure_telemetry_cgroupv2"

    cgroup_cpu_pressure=$(cat ${CGROUP}/cpu.pressure)
    cgroup_memory_pressure=$(cat ${CGROUP}/memory.pressure)
    cgroup_io_pressure=$(cat ${CGROUP}/io.pressure)

    cgroup_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    cgroup_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $cgroup_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $cgroup_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    cgroup_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $cgroup_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $cgroup_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $cgroup_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $cgroup_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $cgroup_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $cgroup_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $cgroup_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $cgroup_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    cgroup_cpu_pressures=$(echo $cgroup_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    cgroup_memory_pressures=$(echo $cgroup_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    cgroup_io_pressures=$(echo $cgroup_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    cgroup_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $cgroup_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $cgroup_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $cgroup_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    SYSTEMSLICE="${CGROUP}/system.slice"
    system_slice_cpu_pressure=$(cat $SYSTEMSLICE/cpu.pressure)
    system_slice_memory_pressure=$(cat $SYSTEMSLICE/memory.pressure)
    system_slice_io_pressure=$(cat $SYSTEMSLICE/io.pressure)

    system_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    system_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $system_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $system_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    system_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $system_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $system_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $system_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $system_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $system_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $system_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $system_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $system_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    system_slice_cpu_pressures=$(echo $system_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_memory_pressures=$(echo $system_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_io_pressures=$(echo $system_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    system_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $system_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $system_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $system_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    AZURESLICE="${CGROUP}/azure.slice"
    azure_slice_cpu_pressure=$(cat $AZURESLICE/cpu.pressure)
    azure_slice_memory_pressure=$(cat $AZURESLICE/memory.pressure)
    azure_slice_io_pressure=$(cat $AZURESLICE/io.pressure)

    azure_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    azure_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $azure_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    azure_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $azure_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $azure_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $azure_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    azure_slice_cpu_pressures=$(echo $azure_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_memory_pressures=$(echo $azure_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_io_pressures=$(echo $azure_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    azure_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $azure_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $azure_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $azure_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    KUBEPODSSLICE="${CGROUP}/kubepods.slice"
    kubepods_slice_cpu_pressure=$(cat $KUBEPODSSLICE/cpu.pressure)
    kubepods_slice_memory_pressure=$(cat $KUBEPODSSLICE/memory.pressure)
    kubepods_slice_io_pressure=$(cat $KUBEPODSSLICE/io.pressure)

    kubepods_slice_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    kubepods_slice_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubepods_slice_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubepods_slice_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubepods_slice_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubepods_slice_cpu_pressures=$(echo $kubepods_slice_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_memory_pressures=$(echo $kubepods_slice_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_io_pressures=$(echo $kubepods_slice_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    kubepods_slice_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $kubepods_slice_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $kubepods_slice_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $kubepods_slice_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    KUBELETSERVICE="${CGROUP}/${KSLICE}/kubelet.service"
    kubelet_service_cpu_pressure=$(cat $KUBELETSERVICE/cpu.pressure)
    kubelet_service_memory_pressure=$(cat $KUBELETSERVICE/memory.pressure)
    kubelet_service_io_pressure=$(cat $KUBELETSERVICE/io.pressure)

    kubelet_service_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    kubelet_service_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubelet_service_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubelet_service_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $kubelet_service_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    kubelet_service_cpu_pressures=$(echo $kubelet_service_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_memory_pressures=$(echo $kubelet_service_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_io_pressures=$(echo $kubelet_service_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    kubelet_service_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $kubelet_service_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $kubelet_service_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $kubelet_service_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    CONTAINERDSERVICE="${CGROUP}/${CSLICE}/containerd.service"
    containerd_service_cpu_pressure=$(cat $CONTAINERDSERVICE/cpu.pressure)
    containerd_service_memory_pressure=$(cat $CONTAINERDSERVICE/memory.pressure)
    containerd_service_io_pressure=$(cat $CONTAINERDSERVICE/io.pressure)

    containerd_service_cpu_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_cpu_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL } | tostring'
    )

    containerd_service_memory_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $containerd_service_memory_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    containerd_service_io_pressures=$( jq -n \
    --arg SOME_AVG10 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $2}' | awk '{print $1}')" \
    --arg SOME_AVG60 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $3}' | awk '{print $1}')" \
    --arg SOME_AVG300 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $4}' | awk '{print $1}')" \
    --arg SOME_TOTAL "$(echo $containerd_service_io_pressure | awk -F "=" '{print $5}' | awk '{print $1}')" \
    --arg FULL_AVG10 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $6}' | awk '{print $1}')" \
    --arg FULL_AVG60 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $7}' | awk '{print $1}')" \
    --arg FULL_AVG300 "$(echo $containerd_service_io_pressure | awk -F "=" '{print $8}' | awk '{print $1}')" \
    --arg FULL_TOTAL "$(echo $containerd_service_io_pressure | awk -F "=" '{print $9}' | awk '{print $1}')" \
    '{ some_avg10: $SOME_AVG10, some_avg60: $SOME_AVG60, some_avg300: $SOME_AVG300, some_total: $SOME_TOTAL, full_avg10: $FULL_AVG10, full_avg60: $FULL_AVG60, full_avg300: $FULL_AVG300, full_total: $FULL_TOTAL } | tostring'
    )

    containerd_service_cpu_pressures=$(echo $containerd_service_cpu_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_memory_pressures=$(echo $containerd_service_memory_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_io_pressures=$(echo $containerd_service_io_pressures | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    containerd_service_pressure=$( jq -n \
    --argjson CPU_PRESSURE "$(echo $containerd_service_cpu_pressures)" \
    --argjson MEMORY_PRESSURE "$(echo $containerd_service_memory_pressures)" \
    --argjson IO_PRESSURE "$(echo $containerd_service_io_pressures)" \
    '{ CPUPressure: $CPU_PRESSURE, MemoryPressure: $MEMORY_PRESSURE, IOPressure: $IO_PRESSURE } | tostring'
    )

    cgroup_pressure=$(echo $system_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    system_slice_pressure=$(echo $system_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    azure_slice_pressure=$(echo $azure_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubepods_slice_pressure=$(echo $kubepods_slice_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    kubelet_service_pressure=$(echo $kubelet_service_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
    containerd_service_pressure=$(echo $containerd_service_pressure | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    pressure_string=$( jq -n \
    --argjson CGROUP "$(echo $cgroup_pressure)" \
    --argjson SYSTEMSLICE "$(echo $system_slice_pressure)" \
    --argjson AZURESLICE "$(echo $azure_slice_pressure)" \
    --argjson KUBEPODSSLICE "$(echo $kubepods_slice_pressure)" \
    --argjson KUBELETSERVICE "$(echo $kubelet_service_pressure)" \
    --argjson CONTAINERDSERVICE "$(echo $containerd_service_pressure)" \
    '{ cgroup_pressure: $CGROUP, system_slice_pressure: $SYSTEMSLICE, azure_slice_pressure: $AZURESLICE, kubepods_slice_pressure: $KUBEPODSSLICE, kubelet_service_pressure: $KUBELETSERVICE, containerd_service_pressure: $CONTAINERDSERVICE } | tostring'
    )

    pressure_string=$(echo $pressure_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

    message_string=$( jq -n \
        --arg CGROUPV "${VERSION}" \
        --argjson PRESSURE "$(echo $pressure_string)" \
        '{ CgroupVersion: $CGROUPV, Pressure: $PRESSURE } | tostring'
    )

else
    echo "Unexpected cgroup type. Exiting"
    exit 1
fi


message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${STARTTIME_FORMATTED}" \
    --arg OperationId   "${ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "${TASK_NAME}" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json`)

func linuxCloudInitArtifactsCgroupPressureTelemetryShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupPressureTelemetrySh, nil
}

func linuxCloudInitArtifactsCgroupPressureTelemetrySh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupPressureTelemetryShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCgroupPressureTelemetryTimer = []byte(`[Unit]
Description=emit pressure telemetry

[Timer]
OnBootSec=0min
OnCalendar=*-*-* *:0/5:0
Unit=cgroup-pressure-telemetry.service

[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsCgroupPressureTelemetryTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCgroupPressureTelemetryTimer, nil
}

func linuxCloudInitArtifactsCgroupPressureTelemetryTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCgroupPressureTelemetryTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cgroup-pressure-telemetry.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCiSyslogWatcherPath = []byte(`[Unit]
Description=Monitor the ContainerInsights syslog status file for changes

[Path]
PathModified=/var/run/mdsd-ci/update.status
Unit=ci-syslog-watcher.service

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsCiSyslogWatcherPathBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCiSyslogWatcherPath, nil
}

func linuxCloudInitArtifactsCiSyslogWatcherPath() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCiSyslogWatcherPathBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ci-syslog-watcher.path", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCiSyslogWatcherService = []byte(`[Unit]
Description=Update syslog config based on ContainerInsights syslog status change

[Service]
Type=oneshot
ExecStart=/usr/local/bin/ci-syslog-watcher.sh

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsCiSyslogWatcherServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCiSyslogWatcherService, nil
}

func linuxCloudInitArtifactsCiSyslogWatcherService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCiSyslogWatcherServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ci-syslog-watcher.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCiSyslogWatcherSh = []byte(`#!/usr/bin/env bash

set -o nounset
set -o pipefail

[ ! -f "/var/run/mdsd-ci/update.status" ] && exit 0
status=$(cat /var/run/mdsd-ci/update.status)

if [[ "$status" == "add" ]]; then
        echo "Status changed to $status."
        [ -f "/var/run/mdsd-ci/70-rsyslog-forward-mdsd-ci.conf" ] && cp /var/run/mdsd-ci/70-rsyslog-forward-mdsd-ci.conf /etc/rsyslog.d
elif [[ "$status" == "remove" ]]; then
        echo "Status changed to $status."
        [ -f "/etc/rsyslog.d/70-rsyslog-forward-mdsd-ci.conf" ] && rm /etc/rsyslog.d/70-rsyslog-forward-mdsd-ci.conf
else
        echo "Unexpected status change to $status. Exiting"
        exit 1
fi

echo "Restarting rsyslog"
systemctl restart rsyslog

exit 0
`)

func linuxCloudInitArtifactsCiSyslogWatcherShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCiSyslogWatcherSh, nil
}

func linuxCloudInitArtifactsCiSyslogWatcherSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCiSyslogWatcherShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ci-syslog-watcher.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCisSh = []byte(`#!/bin/bash
# This gets us the error codes we use and the os and such.
source /home/packer/provision_source.sh

assignRootPW() {
    if grep '^root:[!*]:' /etc/shadow; then
        VERSION=$(grep DISTRIB_RELEASE /etc/*-release | cut -f 2 -d "=")
        SALT=$(openssl rand -base64 5)
        SECRET=$(openssl rand -base64 37)
        CMD="import crypt, getpass, pwd; print(crypt.crypt('$SECRET', '\$6\$$SALT\$'))"
        if [[ "${VERSION}" == "22.04" ]]; then
            HASH=$(python3 -c "$CMD")
        else
            HASH=$(python -c "$CMD")
        fi

        echo 'root:'$HASH | /usr/sbin/chpasswd -e || exit $ERR_CIS_ASSIGN_ROOT_PW
    fi
}

assignFilePermissions() {
    FILES="
    auth.log
    alternatives.log
    cloud-init.log
    cloud-init-output.log
    daemon.log
    dpkg.log
    kern.log
    lastlog
    waagent.log
    syslog
    unattended-upgrades/unattended-upgrades.log
    unattended-upgrades/unattended-upgrades-dpkg.log
    azure-vnet-ipam.log
    azure-vnet-telemetry.log
    azure-cnimonitor.log
    azure-vnet.log
    kv-driver.log
    blobfuse-driver.log
    blobfuse-flexvol-installer.log
    landscape/sysinfo.log
    "
    for FILE in ${FILES}; do
        FILEPATH="/var/log/${FILE}"
        DIR=$(dirname "${FILEPATH}")
        mkdir -p ${DIR} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        touch ${FILEPATH} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        chmod 640 ${FILEPATH} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    done
    find /var/log -type f -perm '/o+r' -exec chmod 'g-wx,o-rwx' {} \;
    chmod 600 /etc/passwd- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 600 /etc/shadow- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 600 /etc/group- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION

    if [[ -f /etc/default/grub ]]; then
        chmod 644 /etc/default/grub || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi

    if [[ -f /etc/crontab ]]; then
        chmod 0600 /etc/crontab || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi
    for filepath in /etc/cron.hourly /etc/cron.daily /etc/cron.weekly /etc/cron.monthly /etc/cron.d; do
        if [[ -e $filepath ]]; then
            chmod 0600 $filepath || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        fi
    done

    # Docs: https://www.man7.org/linux/man-pages/man1/crontab.1.html
    # If cron.allow exists, then cron.deny is ignored. To minimize who can use cron, we
    # always want cron.allow and will default it to empty if it doesn't exist.
    # We also need to set appropriate permissions on it.
    # Since it will be ignored anyway, we delete cron.deny.
    touch /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 640 /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    rm -rf /etc/cron.deny || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
}

# Helper function to replace or append settings to a setting file.
# This abstracts the general logic of:
#   1. Search for the setting (via a pattern passed in).
#   2. If it's there, replace it with desired setting line; otherwise append it to the end of the file.
#   3. Validate that there is now exactly one instance of the setting, and that it is the one we want.
replaceOrAppendSetting() {
    local SEARCH_PATTERN=$1
    local SETTING_LINE=$2
    local FILE=$3

    # Search and replace/append.
    if grep -E "$SEARCH_PATTERN" "$FILE" >/dev/null; then
        sed -E -i "s|${SEARCH_PATTERN}|${SETTING_LINE}|g" "$FILE" || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
    else
        echo -e "\n${SETTING_LINE}" >>"$FILE"
    fi

    # After replacement/append, there should be exactly one line that sets the setting,
    # and it must have the value we want.
    # If not, then there's something wrong with this script.
    if [[ $(grep -E "$SEARCH_PATTERN" "$FILE") != "$SETTING_LINE" ]]; then
        echo "replacement was wrong"
        exit $ERR_CIS_APPLY_PASSWORD_CONFIG
    fi
}

# Creates the search pattern and setting lines for login.defs settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#   '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#   '${1}\s+'       -- Then the setting name followed by one or more whitespace characters
#   '[0-9]+$'       -- Then one more more number, which is the setting value, which is the end of the line.
#
# This is based on a combination of the syntax for the file and real examples we've found.
replaceOrAppendLoginDefs() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}\s+[0-9]+$" "${1} ${2}" /etc/login.defs
}

# Creates the search pattern and setting lines for useradd default settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#   '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#   '${1}='         -- Then the setting name followed by '='
#   '.*$'           -- Then 0 or nore of any character which is the end of the line.
#                      Note that this allows for a setting value to be there or not.
#
# This is based on a combination of the syntax for the file and real examples we've found.
replaceOrAppendUserAdd() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}=.*$" "${1}=${2}" /etc/default/useradd
}

setPWExpiration() {
    replaceOrAppendLoginDefs PASS_MAX_DAYS 90
    replaceOrAppendLoginDefs PASS_MIN_DAYS 7
    replaceOrAppendUserAdd INACTIVE 30
}

# Creates the search pattern and setting lines for the core dump settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#  '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#  '${1}='         -- Then the setting name followed by '='
#  '.*$'           -- Then 0 or nore of any character which is the end of the line.
#
# This is based on a combination of the syntax for the file (https://www.man7.org/linux/man-pages/man5/coredump.conf.5.html)
# and real examples we've found.
replaceOrAppendCoreDump() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}=.*$" "${1}=${2}" /etc/systemd/coredump.conf
}

configureCoreDump() {
    replaceOrAppendCoreDump Storage none
    replaceOrAppendCoreDump ProcessSizeMax 0
}

fixUmaskSettings() {
    # CIS requires the default UMASK for account creation to be set to 027, so change that in /etc/login.defs.
    replaceOrAppendLoginDefs UMASK 027

    # It also requires that nothing in etc/profile.d sets umask to anything less restrictive than that.
    # Mariner sets umask directly in /etc/profile after sourcing everything in /etc/profile.d. But it also has /etc/profile.d/umask.sh
    # which sets umask (but is then ignored). We don't want to simply delete /etc/profile.d/umask.sh, because if we take an update to
    # the package that supplies it, it would just be copied over again.
    # This is complicated by an oddity/bug in the auditing script cis uses, which will flag line in a file with the work umask in the file name
    # that doesn't set umask correctly. So we can't just comment out all the lines or have any comments that explain what we're doing.
    # So since we can't delete the file, we just overwrite it with the correct umask setting. This duplicates what /etc/profile does, but
    # it does no harm and works with the tools.
    # Note that we use printf to avoid a trailing newline.
    local umask_sh="/etc/profile.d/umask.sh"
    if [[ "${OS}" == "${MARINER_OS_NAME}" && "${OS_VERSION}" == "2.0" && -f "${umask_sh}" ]]; then
        printf "umask 027" >${umask_sh}
    fi
}

function maskNfsServer() {
    # If nfs-server.service exists, we need to mask it per CIS requirement.
    # Note that on ubuntu systems, it isn't installed but on mariner we need it
    # due to a dependency, but disable it by default.
    if systemctl list-unit-files nfs-server.service >/dev/null; then
        systemctl --now mask nfs-server || $ERR_SYSTEMCTL_MASK_FAIL
    fi
}

function addFailLockDir() {
    # Mariner V2 uses pamd faillocking, which requires a directory to store the faillock files.
    # Default is /var/run/faillock, but that's a tmpfs, so we need to use /var/log/faillock instead.
    # But we need to leave settings alone for other skus.
    if [[ "${OS}" == "${MARINER_OS_NAME}" && "${OS_VERSION}" == "2.0" ]]; then
        # Replace or append the dir setting in /etc/security/faillock.conf
        # Docs: https://www.man7.org/linux/man-pages/man5/faillock.conf.5.html
        #
        # Search pattern is:
        # '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
        # 'dir\s+'        -- Then the setting name followed by one or more whitespace characters
        # '.*$'           -- Then 0 or nore of any character which is the end of the line.
        #
        # This is based on a combination of the syntax for the file and real examples we've found.
        local fail_lock_dir="/var/log/faillock"
        mkdir -p ${fail_lock_dir}
        replaceOrAppendSetting "^#{0,1} {0,1}dir\s+.*$" "dir = ${fail_lock_dir}" /etc/security/faillock.conf
    fi
}

applyCIS() {
    setPWExpiration
    assignRootPW
    assignFilePermissions
    configureCoreDump
    fixUmaskSettings
    maskNfsServer
    addFailLockDir
}

applyCIS

#EOF
`)

func linuxCloudInitArtifactsCisShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCisSh, nil
}

func linuxCloudInitArtifactsCisSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCisShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cis.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsContainerdMonitorService = []byte(`[Unit]
Description=a script that checks containerd health and restarts if needed
After=containerd.service
[Service]
Restart=always
RestartSec=10
RemainAfterExit=yes
ExecStart=/usr/local/bin/health-monitor.sh container-runtime containerd
#EOF
`)

func linuxCloudInitArtifactsContainerdMonitorServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsContainerdMonitorService, nil
}

func linuxCloudInitArtifactsContainerdMonitorService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsContainerdMonitorServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/containerd-monitor.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsContainerdMonitorTimer = []byte(`[Unit]
Description=a timer that delays containerd-monitor from starting too soon after boot
[Timer]
Unit=containerd-monitor.service
OnBootSec=10min
[Install]
WantedBy=multi-user.target
#EOF
`)

func linuxCloudInitArtifactsContainerdMonitorTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsContainerdMonitorTimer, nil
}

func linuxCloudInitArtifactsContainerdMonitorTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsContainerdMonitorTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/containerd-monitor.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsContainerdService = []byte(`# Explicitly configure containerd systemd service on Mariner AKS to maintain consistent
# settings with the containerd.service file previously deployed during cloud-init.
# Additionally set LimitNOFILE to the exact value "infinity" means on Ubuntu, eg "1048576".
[Unit]
Description=containerd daemon
After=network.target
[Service]
ExecStartPre=/sbin/modprobe overlay
ExecStart=/usr/bin/containerd
Delegate=yes
KillMode=process
Restart=always
# Explicitly set OOMScoreAdjust to make containerd unlikely to be oom killed
OOMScoreAdjust=-999
# Explicitly set LimitNOFILE to match what infinity means on Ubuntu AKS
LimitNOFILE=1048576
# Explicitly set LimitCORE, LimitNPROC, and TasksMax to infinity to match Ubuntu AKS
LimitCORE=infinity
TasksMax=infinity
LimitNPROC=infinity
[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsContainerdServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsContainerdService, nil
}

func linuxCloudInitArtifactsContainerdService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsContainerdServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/containerd.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsContainerd_exec_startConf = []byte(`[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
`)

func linuxCloudInitArtifactsContainerd_exec_startConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsContainerd_exec_startConf, nil
}

func linuxCloudInitArtifactsContainerd_exec_startConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsContainerd_exec_startConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/containerd_exec_start.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCrictlYaml = []byte(`runtime-endpoint: unix:///run/containerd/containerd.sock
`)

func linuxCloudInitArtifactsCrictlYamlBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCrictlYaml, nil
}

func linuxCloudInitArtifactsCrictlYaml() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCrictlYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/crictl.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_cmdSh = []byte(`PROVISION_OUTPUT="/var/log/azure/cluster-provision-cse-output.log";
echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if ShouldEnableCustomData}}
cloud-init status --wait > /dev/null 2>&1;
[ $? -ne 0 ] && echo 'cloud-init failed' >> ${PROVISION_OUTPUT} && exit 1;
echo "cloud-init succeeded" >> ${PROVISION_OUTPUT};
{{end}}
{{if IsAKSCustomCloud}}
REPO_DEPOT_ENDPOINT="{{AKSCustomCloudRepoDepotEndpoint}}"
{{GetInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
ADMINUSER={{GetParameter "linuxAdminUsername"}}
MOBY_VERSION={{GetParameter "mobyVersion"}}
TENANT_ID={{GetVariable "tenantID"}}
KUBERNETES_VERSION={{GetParameter "kubernetesVersion"}}
HYPERKUBE_URL={{GetParameter "kubernetesHyperkubeSpec"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
CUSTOM_KUBE_BINARY_URL={{GetParameter "customKubeBinaryURL"}}
KUBEPROXY_URL={{GetParameter "kubeProxySpec"}}
APISERVER_PUBLIC_KEY={{GetParameter "apiServerCertificate"}}
SUBSCRIPTION_ID={{GetVariable "subscriptionId"}}
RESOURCE_GROUP={{GetVariable "resourceGroup"}}
LOCATION={{GetVariable "location"}}
VM_TYPE={{GetVariable "vmType"}}
SUBNET={{GetVariable "subnetName"}}
NETWORK_SECURITY_GROUP={{GetVariable "nsgName"}}
VIRTUAL_NETWORK={{GetVariable "virtualNetworkName"}}
VIRTUAL_NETWORK_RESOURCE_GROUP={{GetVariable "virtualNetworkResourceGroupName"}}
ROUTE_TABLE={{GetVariable "routeTableName"}}
PRIMARY_AVAILABILITY_SET={{GetVariable "primaryAvailabilitySetName"}}
PRIMARY_SCALE_SET={{GetVariable "primaryScaleSetName"}}
SERVICE_PRINCIPAL_CLIENT_ID={{GetParameter "servicePrincipalClientId"}}
NETWORK_PLUGIN={{GetParameter "networkPlugin"}}
NETWORK_POLICY={{GetParameter "networkPolicy"}}
VNET_CNI_PLUGINS_URL={{GetParameter "vnetCniLinuxPluginsURL"}}
CNI_PLUGINS_URL={{GetParameter "cniPluginsURL"}}
CLOUDPROVIDER_BACKOFF={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoff"}}
CLOUDPROVIDER_BACKOFF_MODE={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffMode"}}
CLOUDPROVIDER_BACKOFF_RETRIES={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffRetries"}}
CLOUDPROVIDER_BACKOFF_EXPONENT={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffExponent"}}
CLOUDPROVIDER_BACKOFF_DURATION={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffDuration"}}
CLOUDPROVIDER_BACKOFF_JITTER={{GetParameterProperty "cloudproviderConfig" "cloudProviderBackoffJitter"}}
CLOUDPROVIDER_RATELIMIT={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimit"}}
CLOUDPROVIDER_RATELIMIT_QPS={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitQPS"}}
CLOUDPROVIDER_RATELIMIT_QPS_WRITE={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitQPSWrite"}}
CLOUDPROVIDER_RATELIMIT_BUCKET={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitBucket"}}
CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE={{GetParameterProperty "cloudproviderConfig" "cloudProviderRateLimitBucketWrite"}}
LOAD_BALANCER_DISABLE_OUTBOUND_SNAT={{GetParameterProperty "cloudproviderConfig" "cloudProviderDisableOutboundSNAT"}}
USE_MANAGED_IDENTITY_EXTENSION={{GetVariable "useManagedIdentityExtension"}}
USE_INSTANCE_METADATA={{GetVariable "useInstanceMetadata"}}
LOAD_BALANCER_SKU={{GetVariable "loadBalancerSku"}}
EXCLUDE_MASTER_FROM_STANDARD_LB={{GetVariable "excludeMasterFromStandardLB"}}
MAXIMUM_LOADBALANCER_RULE_COUNT={{GetVariable "maximumLoadBalancerRuleCount"}}
CONTAINER_RUNTIME={{GetParameter "containerRuntime"}}
CLI_TOOL={{GetParameter "cliTool"}}
CONTAINERD_DOWNLOAD_URL_BASE={{GetParameter "containerdDownloadURLBase"}}
NETWORK_MODE={{GetParameter "networkMode"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
USER_ASSIGNED_IDENTITY_ID={{GetVariable "userAssignedIdentityID"}}
API_SERVER_NAME={{GetKubernetesEndpoint}}
IS_VHD={{GetVariable "isVHD"}}
GPU_NODE={{GetVariable "gpuNode"}}
SGX_NODE={{GetVariable "sgxNode"}}
MIG_NODE={{GetVariable "migNode"}}
CONFIG_GPU_DRIVER_IF_NEEDED={{GetVariable "configGPUDriverIfNeeded"}}
ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED={{GetVariable "enableGPUDevicePluginIfNeeded"}}
TELEPORTD_PLUGIN_DOWNLOAD_URL={{GetParameter "teleportdPluginURL"}}
CONTAINERD_VERSION={{GetParameter "containerdVersion"}}
CONTAINERD_PACKAGE_URL={{GetParameter "containerdPackageURL"}}
RUNC_VERSION={{GetParameter "runcVersion"}}
RUNC_PACKAGE_URL={{GetParameter "runcPackageURL"}}
ENABLE_HOSTS_CONFIG_AGENT="{{EnableHostsConfigAgent}}"
DISABLE_SSH="{{ShouldDisableSSH}}"
NEEDS_CONTAINERD="{{NeedsContainerd}}"
TELEPORT_ENABLED="{{TeleportEnabled}}"
SHOULD_CONFIGURE_HTTP_PROXY="{{ShouldConfigureHTTPProxy}}"
SHOULD_CONFIGURE_HTTP_PROXY_CA="{{ShouldConfigureHTTPProxyCA}}"
HTTP_PROXY_TRUSTED_CA="{{GetHTTPProxyCA}}"
SHOULD_CONFIGURE_CUSTOM_CA_TRUST="{{ShouldConfigureCustomCATrust}}"
CUSTOM_CA_TRUST_COUNT="{{len GetCustomCATrustConfigCerts}}"
{{range $i, $cert := GetCustomCATrustConfigCerts}}
CUSTOM_CA_CERT_{{$i}}="{{$cert}}"
{{end}}
IS_KRUSTLET="{{IsKrustlet}}"
GPU_NEEDS_FABRIC_MANAGER="{{GPUNeedsFabricManager}}"
NEEDS_DOCKER_LOGIN="{{and IsDockerContainerRuntime HasPrivateAzureRegistryServer}}"
IPV6_DUAL_STACK_ENABLED="{{IsIPv6DualStackFeatureEnabled}}"
OUTBOUND_COMMAND="{{GetOutboundCommand}}"
ENABLE_UNATTENDED_UPGRADES="{{EnableUnattendedUpgrade}}"
ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE="{{ and NeedsContainerd IsKubenet (not HasCalicoNetworkPolicy) }}"
SHOULD_CONFIG_SWAP_FILE="{{ShouldConfigSwapFile}}"
SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE="{{ShouldConfigTransparentHugePage}}"
SHOULD_CONFIG_CONTAINERD_ULIMITS="{{ShouldConfigContainerdUlimits}}"
CONTAINERD_ULIMITS="{{GetContainerdUlimitString}}"
{{/* both CLOUD and ENVIRONMENT have special values when IsAKSCustomCloud == true */}}
{{/* CLOUD uses AzureStackCloud and seems to be used by kubelet, k8s cloud provider */}}
{{/* target environment seems to go to ARM SDK config */}}
{{/* not sure why separate/inconsistent? */}}
{{/* see GetCustomEnvironmentJSON for more weirdness. */}}
TARGET_CLOUD="{{- if IsAKSCustomCloud -}} AzureStackCloud {{- else -}} {{GetTargetEnvironment}} {{- end -}}"
TARGET_ENVIRONMENT="{{GetTargetEnvironment}}"
CUSTOM_ENV_JSON="{{GetBase64EncodedEnvironmentJSON}}"
IS_CUSTOM_CLOUD="{{IsAKSCustomCloud}}"
CSE_HELPERS_FILEPATH="{{GetCSEHelpersScriptFilepath}}"
CSE_DISTRO_HELPERS_FILEPATH="{{GetCSEHelpersScriptDistroFilepath}}"
CSE_INSTALL_FILEPATH="{{GetCSEInstallScriptFilepath}}"
CSE_DISTRO_INSTALL_FILEPATH="{{GetCSEInstallScriptDistroFilepath}}"
CSE_CONFIG_FILEPATH="{{GetCSEConfigScriptFilepath}}"
AZURE_PRIVATE_REGISTRY_SERVER="{{GetPrivateAzureRegistryServer}}"
HAS_CUSTOM_SEARCH_DOMAIN="{{HasCustomSearchDomain}}"
CUSTOM_SEARCH_DOMAIN_FILEPATH="{{GetCustomSearchDomainsCSEScriptFilepath}}"
HTTP_PROXY_URLS="{{GetHTTPProxy}}"
HTTPS_PROXY_URLS="{{GetHTTPSProxy}}"
NO_PROXY_URLS="{{GetNoProxy}}"
PROXY_VARS="{{GetProxyVariables}}"
CLIENT_TLS_BOOTSTRAPPING_ENABLED="{{IsKubeletClientTLSBootstrappingEnabled}}"
DHCPV6_SERVICE_FILEPATH="{{GetDHCPv6ServiceCSEScriptFilepath}}"
DHCPV6_CONFIG_FILEPATH="{{GetDHCPv6ConfigCSEScriptFilepath}}"
THP_ENABLED="{{GetTransparentHugePageEnabled}}"
THP_DEFRAG="{{GetTransparentHugePageDefrag}}"
SERVICE_PRINCIPAL_FILE_CONTENT="{{GetServicePrincipalSecret}}"
KUBELET_CLIENT_CONTENT="{{GetKubeletClientKey}}"
KUBELET_CLIENT_CERT_CONTENT="{{GetKubeletClientCert}}"
KUBELET_CONFIG_FILE_ENABLED="{{IsKubeletConfigFileEnabled}}"
KUBELET_CONFIG_FILE_CONTENT="{{GetKubeletConfigFileContentBase64}}"
SWAP_FILE_SIZE_MB="{{GetSwapFileSizeMB}}"
GPU_DRIVER_VERSION="{{GPUDriverVersion}}"
GPU_INSTANCE_PROFILE="{{GetGPUInstanceProfile}}"
CUSTOM_SEARCH_DOMAIN_NAME="{{GetSearchDomainName}}"
CUSTOM_SEARCH_REALM_USER="{{GetSearchDomainRealmUser}}"
CUSTOM_SEARCH_REALM_PASSWORD="{{GetSearchDomainRealmPassword}}"
MESSAGE_OF_THE_DAY="{{GetMessageOfTheDay}}"
HAS_KUBELET_DISK_TYPE="{{HasKubeletDiskType}}"
NEEDS_CGROUPV2="{{Is2204VHD}}"
TLS_BOOTSTRAP_TOKEN="{{GetTLSBootstrapTokenForKubeConfig}}"
KUBELET_FLAGS="{{GetKubeletConfigKeyVals}}"
NETWORK_POLICY="{{GetParameter "networkPolicy"}}"
{{- if not (IsKubernetesVersionGe "1.17.0")}}
KUBELET_IMAGE="{{GetHyperkubeImageReference}}"
{{end}}
{{if IsKubernetesVersionGe "1.16.0"}}
KUBELET_NODE_LABELS="{{GetAgentKubernetesLabels . }}"
{{else}}
KUBELET_NODE_LABELS="{{GetAgentKubernetesLabelsDeprecated . }}"
{{end}}
AZURE_ENVIRONMENT_FILEPATH="{{- if IsAKSCustomCloud}}/etc/kubernetes/{{GetTargetEnvironment}}.json{{end}}"
KUBE_CA_CRT="{{GetParameter "caCertificate"}}"
KUBENET_TEMPLATE="{{GetKubenetTemplate}}"
CONTAINERD_CONFIG_CONTENT="{{GetContainerdConfigContent}}"
CONTAINERD_CONFIG_NO_GPU_CONTENT="{{GetContainerdConfigNoGPUContent}}"
IS_KATA="{{IsKata}}"
SYSCTL_CONTENT="{{GetSysctlContent}}"
PRIVATE_EGRESS_PROXY_ADDRESS="{{GetPrivateEgressProxyAddress}}"
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"`)

func linuxCloudInitArtifactsCse_cmdShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_cmdSh, nil
}

func linuxCloudInitArtifactsCse_cmdSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_cmdShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_cmd.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_configSh = []byte(`#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
}

configPrivateClusterHosts() {
    mkdir -p /etc/systemd/system/reconcile-private-hosts.service.d/
    touch /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf
    tee /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf > /dev/null <<EOF
[Service]
Environment="KUBE_API_SERVER_NAME=${API_SERVER_NAME}"
EOF
  systemctlEnableAndStart reconcile-private-hosts || exit $ERR_SYSTEMCTL_START_FAIL
}
configureTransparentHugePage() {
    ETC_SYSFS_CONF="/etc/sysfs.conf"
    if [[ "${THP_ENABLED}" != "" ]]; then
        echo "${THP_ENABLED}" > /sys/kernel/mm/transparent_hugepage/enabled
        echo "kernel/mm/transparent_hugepage/enabled=${THP_ENABLED}" >> ${ETC_SYSFS_CONF}
    fi
    if [[ "${THP_DEFRAG}" != "" ]]; then
        echo "${THP_DEFRAG}" > /sys/kernel/mm/transparent_hugepage/defrag
        echo "kernel/mm/transparent_hugepage/defrag=${THP_DEFRAG}" >> ${ETC_SYSFS_CONF}
    fi
}

configureSwapFile() {
    # https://learn.microsoft.com/en-us/troubleshoot/azure/virtual-machines/troubleshoot-device-names-problems#identify-disk-luns
    swap_size_kb=$(expr ${SWAP_FILE_SIZE_MB} \* 1000)
    swap_location=""
    
    # Attempt to use the resource disk
    if [[ -L /dev/disk/azure/resource-part1 ]]; then
        resource_disk_path=$(findmnt -nr -o target -S $(readlink -f /dev/disk/azure/resource-part1))
        disk_free_kb=$(df ${resource_disk_path} | sed 1d | awk '{print $4}')
        if [[ ${disk_free_kb} -gt ${swap_size_kb} ]]; then
            echo "Will use resource disk for swap file"
            swap_location=${resource_disk_path}/swapfile
        else
            echo "Insufficient disk space on resource disk to create swap file: request ${swap_size_kb} free ${disk_free_kb}, attempting to fall back to OS disk..."
        fi
    fi

    # If we couldn't use the resource disk, attempt to use the OS disk
    if [[ -z "${swap_location}" ]]; then
        # Directly check size on the root directory since we can't rely on 'root-part1' always being the correct label
        os_device=$(readlink -f /dev/disk/azure/root)
        disk_free_kb=$(df -P / | sed 1d | awk '{print $4}')
        if [[ ${disk_free_kb} -gt ${swap_size_kb} ]]; then
            echo "Will use OS disk for swap file"
            swap_location=/swapfile
        else
            echo "Insufficient disk space on OS device ${os_device} to create swap file: request ${swap_size_kb} free ${disk_free_kb}"
            exit $ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE
        fi
    fi

    echo "Swap file will be saved to: ${swap_location}"
    retrycmd_if_failure 24 5 25 fallocate -l ${swap_size_kb}K ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    chmod 600 ${swap_location}
    retrycmd_if_failure 24 5 25 mkswap ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    retrycmd_if_failure 24 5 25 swapon ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    retrycmd_if_failure 24 5 25 swapon --show | grep ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    echo "${swap_location} none swap sw 0 0" >> /etc/fstab
}

configureEtcEnvironment() {
    mkdir -p /etc/systemd/system.conf.d/
    touch /etc/systemd/system.conf.d/proxy.conf
    chmod 0644 /etc/systemd/system.conf.d/proxy.conf

    mkdir -p  /etc/apt/apt.conf.d
    chmod 0644 /etc/apt/apt.conf.d/95proxy
    touch /etc/apt/apt.conf.d/95proxy

    # TODO(ace): this pains me but quick and dirty refactor
    echo "[Manager]" >> /etc/systemd/system.conf.d/proxy.conf
    if [ "${HTTP_PROXY_URLS}" != "" ]; then
        echo "HTTP_PROXY=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "http_proxy=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "Acquire::http::proxy \"${HTTP_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTP_PROXY=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"http_proxy=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ "${HTTPS_PROXY_URLS}" != "" ]; then
        echo "HTTPS_PROXY=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "https_proxy=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "Acquire::https::proxy \"${HTTPS_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTPS_PROXY=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"https_proxy=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ "${NO_PROXY_URLS}" != "" ]; then
        echo "NO_PROXY=${NO_PROXY_URLS}" >> /etc/environment
        echo "no_proxy=${NO_PROXY_URLS}" >> /etc/environment
        echo "DefaultEnvironment=\"NO_PROXY=${NO_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"no_proxy=${NO_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi

    # for kubelet to pick up the proxy
    mkdir -p "/etc/systemd/system/kubelet.service.d"
    tee "/etc/systemd/system/kubelet.service.d/10-httpproxy.conf" > /dev/null <<'EOF'
[Service]
EnvironmentFile=/etc/environment
EOF
}

configureHTTPProxyCA() {
    if [[ $OS == $MARINER_OS_NAME ]]; then
        cert_dest="/usr/share/pki/ca-trust-source/anchors"
        update_cmd="update-ca-trust"
    else
        cert_dest="/usr/local/share/ca-certificates"
        update_cmd="update-ca-certificates"
    fi
    echo "${HTTP_PROXY_TRUSTED_CA}" | base64 -d > "${cert_dest}/proxyCA.crt" || exit $ERR_UPDATE_CA_CERTS
    $update_cmd || exit $ERR_UPDATE_CA_CERTS
}

configureCustomCaCertificate() {
    mkdir -p /opt/certs
    for i in $(seq 0 $((${CUSTOM_CA_TRUST_COUNT} - 1))); do
        # directly referring to the variable as "${CUSTOM_CA_CERT_${i}}"
        # causes bad substitution errors in bash
        # dynamically declare and use `+"`"+`!`+"`"+` to add a layer of indirection
        declare varname=CUSTOM_CA_CERT_${i} 
        echo "${!varname}" | base64 -d > /opt/certs/00000000000000cert${i}.crt
    done
    # This will block until the service is considered active.
    # Update_certs.service is a oneshot type of unit that
    # is considered active when the ExecStart= command terminates with a zero status code.
    systemctl restart update_certs.service || exit $ERR_UPDATE_CA_CERTS
    # after new certs are added to trust store, containerd will not pick them up properly before restart.
    # aim here is to have this working straight away for a freshly provisioned node
    # so we force a restart after the certs are updated
    # custom CA daemonset copies certs passed by the user to the node, what then triggers update_certs.path unit
    # path unit then triggers the script that copies over cert files to correct location on the node and updates the trust store
    # as a part of this flow we could restart containerd everytime a new cert is added to the trust store using custom CA
    systemctl restart containerd
}

configureContainerdUlimits() {
  CONTAINERD_ULIMIT_DROP_IN_FILE_PATH="/etc/systemd/system/containerd.service.d/set_ulimits.conf"
  touch "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  chmod 0600 "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  tee "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}" > /dev/null <<EOF
$(echo "$CONTAINERD_ULIMITS" | tr ' ' '\n')
EOF

  systemctl daemon-reload
  systemctl restart containerd
}


configureKubeletServerCert() {
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}" -addext "subjectAltName=DNS:${NODE_NAME}"
}

configureK8s() {
    APISERVER_PUBLIC_KEY_PATH="/etc/kubernetes/certs/apiserver.crt"
    touch "${APISERVER_PUBLIC_KEY_PATH}"
    chmod 0644 "${APISERVER_PUBLIC_KEY_PATH}"
    chown root:root "${APISERVER_PUBLIC_KEY_PATH}"

    AZURE_JSON_PATH="/etc/kubernetes/azure.json"
    touch "${AZURE_JSON_PATH}"
    chmod 0600 "${AZURE_JSON_PATH}"
    chown root:root "${AZURE_JSON_PATH}"

    mkdir -p "/etc/kubernetes/certs"
    if [ -n "${KUBELET_CLIENT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.key
    fi
    if [ -n "${KUBELET_CLIENT_CERT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CERT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.crt
    fi
    if [ -n "${SERVICE_PRINCIPAL_FILE_CONTENT}" ]; then
        echo "${SERVICE_PRINCIPAL_FILE_CONTENT}" | base64 -d > /etc/kubernetes/sp.txt
    fi

    set +x
    echo "${APISERVER_PUBLIC_KEY}" | base64 --decode > "${APISERVER_PUBLIC_KEY_PATH}"
    # Perform the required JSON escaping
    SP_FILE="/etc/kubernetes/sp.txt"
    SERVICE_PRINCIPAL_CLIENT_SECRET="$(cat "$SP_FILE")"
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\\/\\\\}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\"/\\\"}
    rm "$SP_FILE" # unneeded after reading from disk.
    cat << EOF > "${AZURE_JSON_PATH}"
{
    "cloud": "${TARGET_CLOUD}",
    "tenantId": "${TENANT_ID}",
    "subscriptionId": "${SUBSCRIPTION_ID}",
    "aadClientId": "${SERVICE_PRINCIPAL_CLIENT_ID}",
    "aadClientSecret": "${SERVICE_PRINCIPAL_CLIENT_SECRET}",
    "resourceGroup": "${RESOURCE_GROUP}",
    "location": "${LOCATION}",
    "vmType": "${VM_TYPE}",
    "subnetName": "${SUBNET}",
    "securityGroupName": "${NETWORK_SECURITY_GROUP}",
    "vnetName": "${VIRTUAL_NETWORK}",
    "vnetResourceGroup": "${VIRTUAL_NETWORK_RESOURCE_GROUP}",
    "routeTableName": "${ROUTE_TABLE}",
    "primaryAvailabilitySetName": "${PRIMARY_AVAILABILITY_SET}",
    "primaryScaleSetName": "${PRIMARY_SCALE_SET}",
    "cloudProviderBackoffMode": "${CLOUDPROVIDER_BACKOFF_MODE}",
    "cloudProviderBackoff": ${CLOUDPROVIDER_BACKOFF},
    "cloudProviderBackoffRetries": ${CLOUDPROVIDER_BACKOFF_RETRIES},
    "cloudProviderBackoffExponent": ${CLOUDPROVIDER_BACKOFF_EXPONENT},
    "cloudProviderBackoffDuration": ${CLOUDPROVIDER_BACKOFF_DURATION},
    "cloudProviderBackoffJitter": ${CLOUDPROVIDER_BACKOFF_JITTER},
    "cloudProviderRateLimit": ${CLOUDPROVIDER_RATELIMIT},
    "cloudProviderRateLimitQPS": ${CLOUDPROVIDER_RATELIMIT_QPS},
    "cloudProviderRateLimitBucket": ${CLOUDPROVIDER_RATELIMIT_BUCKET},
    "cloudProviderRateLimitQPSWrite": ${CLOUDPROVIDER_RATELIMIT_QPS_WRITE},
    "cloudProviderRateLimitBucketWrite": ${CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE},
    "useManagedIdentityExtension": ${USE_MANAGED_IDENTITY_EXTENSION},
    "userAssignedIdentityID": "${USER_ASSIGNED_IDENTITY_ID}",
    "useInstanceMetadata": ${USE_INSTANCE_METADATA},
    "loadBalancerSku": "${LOAD_BALANCER_SKU}",
    "disableOutboundSNAT": ${LOAD_BALANCER_DISABLE_OUTBOUND_SNAT},
    "excludeMasterFromStandardLB": ${EXCLUDE_MASTER_FROM_STANDARD_LB},
    "providerVaultName": "${KMS_PROVIDER_VAULT_NAME}",
    "maximumLoadBalancerRuleCount": ${MAXIMUM_LOADBALANCER_RULE_COUNT},
    "providerKeyName": "k8s",
    "providerKeyVersion": ""
}
EOF
    set -x
    if [[ "${CLOUDPROVIDER_BACKOFF_MODE}" = "v2" ]]; then
        sed -i "/cloudProviderBackoffExponent/d" /etc/kubernetes/azure.json
        sed -i "/cloudProviderBackoffJitter/d" /etc/kubernetes/azure.json
    fi

    configureKubeletServerCert
    if [ "${IS_CUSTOM_CLOUD}" == "true" ]; then
        set +x
        AKS_CUSTOM_CLOUD_JSON_PATH="/etc/kubernetes/${TARGET_ENVIRONMENT}.json"
        touch "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chmod 0600 "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chown root:root "${AKS_CUSTOM_CLOUD_JSON_PATH}"

        echo "${CUSTOM_ENV_JSON}" | base64 -d > "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        set -x
    fi

    if [ "${KUBELET_CONFIG_FILE_ENABLED}" == "true" ]; then
        set +x
        KUBELET_CONFIG_JSON_PATH="/etc/default/kubeletconfig.json"
        touch "${KUBELET_CONFIG_JSON_PATH}"
        chmod 0600 "${KUBELET_CONFIG_JSON_PATH}"
        chown root:root "${KUBELET_CONFIG_JSON_PATH}"
        echo "${KUBELET_CONFIG_FILE_CONTENT}" | base64 -d > "${KUBELET_CONFIG_JSON_PATH}"
        set -x
        KUBELET_CONFIG_DROP_IN="/etc/systemd/system/kubelet.service.d/10-componentconfig.conf"
        touch "${KUBELET_CONFIG_DROP_IN}"
        chmod 0600 "${KUBELET_CONFIG_DROP_IN}"
        tee "${KUBELET_CONFIG_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json"
EOF
    fi
}

configureCNI() {
    # needed for the iptables rules to work on bridges
    retrycmd_if_failure 120 5 25 modprobe br_netfilter || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
    configureCNIIPTables
}

configureCNIIPTables() {
    if [[ "${NETWORK_PLUGIN}" = "azure" ]]; then
        mv $CNI_BIN_DIR/10-azure.conflist $CNI_CONFIG_DIR/
        chmod 600 $CNI_CONFIG_DIR/10-azure.conflist
        if [[ "${NETWORK_POLICY}" == "calico" ]]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        elif [[ "${NETWORK_POLICY}" == "" || "${NETWORK_POLICY}" == "none" ]] && [[ "${NETWORK_MODE}" == "transparent" ]]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        fi
        /sbin/ebtables -t nat --list
    fi
}

disableSystemdResolved() {
    ls -ltr /etc/resolv.conf
    cat /etc/resolv.conf
    UBUNTU_RELEASE=$(lsb_release -r -s)
    if [[ "${UBUNTU_RELEASE}" == "18.04" || "${UBUNTU_RELEASE}" == "20.04" || "${UBUNTU_RELEASE}" == "22.04" ]]; then
        echo "Ingorings systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
    fi
}

ensureContainerd() {
  if [ "${TELEPORT_ENABLED}" == "true" ]; then
    ensureTeleportd
  fi
  mkdir -p "/etc/systemd/system/containerd.service.d" 
  tee "/etc/systemd/system/containerd.service.d/exec_start.conf" > /dev/null <<EOF
[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
EOF

  mkdir -p /etc/containerd
  if [[ "${GPU_NODE}" = true ]] && [[ "${skip_nvidia_driver_install}" == "true" ]]; then
    echo "Generating non-GPU containerd config for GPU node due to VM tags"
    echo "${CONTAINERD_CONFIG_NO_GPU_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  else
    echo "Generating containerd config..."
    echo "${CONTAINERD_CONFIG_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  fi

  tee "/etc/sysctl.d/99-force-bridge-forward.conf" > /dev/null <<EOF 
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD
  systemctl is-active --quiet docker && (systemctl_disable 20 30 120 docker || exit $ERR_SYSTEMD_DOCKER_STOP_FAIL)
  systemctlEnableAndStart containerd || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureNoDupOnPromiscuBridge() {
    systemctlEnableAndStart ensure-no-dup || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureTeleportd() {
    systemctlEnableAndStart teleportd || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureDocker() {
    DOCKER_SERVICE_EXEC_START_FILE=/etc/systemd/system/docker.service.d/exec_start.conf
    usermod -aG docker ${ADMINUSER}
    DOCKER_MOUNT_FLAGS_SYSTEMD_FILE=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
    DOCKER_JSON_FILE=/etc/docker/daemon.json
    for i in $(seq 1 1200); do
        if [ -s $DOCKER_JSON_FILE ]; then
            jq '.' < $DOCKER_JSON_FILE && break
        fi
        if [ $i -eq 1200 ]; then
            exit $ERR_FILE_WATCH_TIMEOUT
        else
            sleep 1
        fi
    done
    systemctl is-active --quiet containerd && (systemctl_disable 20 30 120 containerd || exit $ERR_SYSTEMD_CONTAINERD_STOP_FAIL)
    systemctlEnableAndStart docker || exit $ERR_DOCKER_START_FAIL

}

ensureDHCPv6() {
    systemctlEnableAndStart dhcpv6 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}

ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    mkdir -p /etc/default
    echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_REGISTER_SCHEDULABLE=true" >> "${KUBELET_DEFAULT_FILE}"
    echo "NETWORK_POLICY=${NETWORK_POLICY}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_IMAGE=${KUBELET_IMAGE}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_NODE_LABELS=${KUBELET_NODE_LABELS}" >> "${KUBELET_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${KUBELET_DEFAULT_FILE}"
    fi
    
    KUBE_CA_FILE="/etc/kubernetes/certs/ca.crt"
    mkdir -p "$(dirname "${KUBE_CA_FILE}")"
    echo "${KUBE_CA_CRT}" | base64 -d > "${KUBE_CA_FILE}"
    chmod 0600 "${KUBE_CA_FILE}"
    
    if [ "${CLIENT_TLS_BOOTSTRAPPING_ENABLED}" == "true" ]; then
        KUBELET_TLS_DROP_IN="/etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf"
        mkdir -p "$(dirname "${KUBELET_TLS_DROP_IN}")"
        touch "${KUBELET_TLS_DROP_IN}"
        chmod 0600 "${KUBELET_TLS_DROP_IN}"
        tee "${KUBELET_TLS_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
EOF
        BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
        mkdir -p "$(dirname "${BOOTSTRAP_KUBECONFIG_FILE}")"
        touch "${BOOTSTRAP_KUBECONFIG_FILE}"
        chmod 0644 "${BOOTSTRAP_KUBECONFIG_FILE}"
        tee "${BOOTSTRAP_KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: kubelet-bootstrap
  user:
    token: "${TLS_BOOTSTRAP_TOKEN}"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    else
        KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
        mkdir -p "$(dirname "${KUBECONFIG_FILE}")"
        touch "${KUBECONFIG_FILE}"
        chmod 0644 "${KUBECONFIG_FILE}"
        tee "${KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
EOF
    fi
    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    tee "${KUBELET_RUNTIME_CONFIG_SCRIPT_FILE}" > /dev/null <<EOF
#!/bin/bash
# Disallow container from reaching out to the special IP address 168.63.129.16
# for TCP protocol (which http uses)
#
# 168.63.129.16 contains protected settings that have priviledged info.
#
# The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
#
# Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
# for DNS.
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
EOF
    systemctlEnableAndStart kubelet || exit $ERR_KUBELET_START_FAIL
}

ensureMigPartition(){
    mkdir -p /etc/systemd/system/mig-partition.service.d/
    touch /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf
    tee /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf > /dev/null <<EOF
[Service]
Environment="GPU_INSTANCE_PROFILE=${GPU_INSTANCE_PROFILE}"
EOF
    # this is expected to fail and work only on next reboot
    # it MAY succeed, only due to unreliability of systemd
    # service type=Simple, which does not exit non-zero
    # on failure if ExecStart failed to invoke.
    systemctlEnableAndStart mig-partition
}

ensureSysctl() {
    SYSCTL_CONFIG_FILE=/etc/sysctl.d/999-sysctl-aks.conf
    mkdir -p "$(dirname "${SYSCTL_CONFIG_FILE}")"
    touch "${SYSCTL_CONFIG_FILE}"
    chmod 0644 "${SYSCTL_CONFIG_FILE}"
    echo "${SYSCTL_CONTENT}" | base64 -d > "${SYSCTL_CONFIG_FILE}"
    retrycmd_if_failure 24 5 25 sysctl --system
}

ensureK8sControlPlane() {
    if $REBOOTREQUIRED || [ "$NO_OUTBOUND" = "true" ]; then
        return
    fi
    retrycmd_if_failure 120 5 25 $KUBECTL 2>/dev/null cluster-info || exit $ERR_K8S_RUNNING_TIMEOUT
}

createKubeManifestDir() {
    KUBEMANIFESTDIR=/etc/kubernetes/manifests
    mkdir -p $KUBEMANIFESTDIR
}

writeKubeConfig() {
    KUBECONFIGDIR=/home/$ADMINUSER/.kube
    KUBECONFIGFILE=$KUBECONFIGDIR/config
    mkdir -p $KUBECONFIGDIR
    touch $KUBECONFIGFILE
    chown $ADMINUSER:$ADMINUSER $KUBECONFIGDIR
    chown $ADMINUSER:$ADMINUSER $KUBECONFIGFILE
    chmod 700 $KUBECONFIGDIR
    chmod 600 $KUBECONFIGFILE
    set +x
    echo "
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: \"$CA_CERTIFICATE\"
    server: $KUBECONFIG_SERVER
  name: \"$MASTER_FQDN\"
contexts:
- context:
    cluster: \"$MASTER_FQDN\"
    user: \"$MASTER_FQDN-admin\"
  name: \"$MASTER_FQDN\"
current-context: \"$MASTER_FQDN\"
kind: Config
users:
- name: \"$MASTER_FQDN-admin\"
  user:
    client-certificate-data: \"$KUBECONFIG_CERTIFICATE\"
    client-key-data: \"$KUBECONFIG_KEY\"
" > $KUBECONFIGFILE
    set -x
}

configClusterAutoscalerAddon() {
    CLUSTER_AUTOSCALER_ADDON_FILE=/etc/kubernetes/addons/cluster-autoscaler-deployment.yaml
    sed -i "s|<clientID>|$(echo $SERVICE_PRINCIPAL_CLIENT_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<clientSec>|$(echo $SERVICE_PRINCIPAL_CLIENT_SECRET | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<subID>|$(echo $SUBSCRIPTION_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<tenantID>|$(echo $TENANT_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<rg>|$(echo $RESOURCE_GROUP | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
}

configACIConnectorAddon() {
    ACI_CONNECTOR_CREDENTIALS=$(printf "{\"clientId\": \"%s\", \"clientSecret\": \"%s\", \"tenantId\": \"%s\", \"subscriptionId\": \"%s\", \"activeDirectoryEndpointUrl\": \"https://login.microsoftonline.com\",\"resourceManagerEndpointUrl\": \"https://management.azure.com/\", \"activeDirectoryGraphResourceId\": \"https://graph.windows.net/\", \"sqlManagementEndpointUrl\": \"https://management.core.windows.net:8443/\", \"galleryEndpointUrl\": \"https://gallery.azure.com/\", \"managementEndpointUrl\": \"https://management.core.windows.net/\"}" "$SERVICE_PRINCIPAL_CLIENT_ID" "$SERVICE_PRINCIPAL_CLIENT_SECRET" "$TENANT_ID" "$SUBSCRIPTION_ID" | base64 -w 0)

    openssl req -newkey rsa:4096 -new -nodes -x509 -days 3650 -keyout /etc/kubernetes/certs/aci-connector-key.pem -out /etc/kubernetes/certs/aci-connector-cert.pem -subj "/C=US/ST=CA/L=virtualkubelet/O=virtualkubelet/OU=virtualkubelet/CN=virtualkubelet"
    ACI_CONNECTOR_KEY=$(base64 /etc/kubernetes/certs/aci-connector-key.pem -w0)
    ACI_CONNECTOR_CERT=$(base64 /etc/kubernetes/certs/aci-connector-cert.pem -w0)

    ACI_CONNECTOR_ADDON_FILE=/etc/kubernetes/addons/aci-connector-deployment.yaml
    sed -i "s|<creds>|$ACI_CONNECTOR_CREDENTIALS|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<rgName>|$RESOURCE_GROUP|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<cert>|$ACI_CONNECTOR_CERT|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<key>|$ACI_CONNECTOR_KEY|g" $ACI_CONNECTOR_ADDON_FILE
}

configAzurePolicyAddon() {
    AZURE_POLICY_ADDON_FILE=/etc/kubernetes/addons/azure-policy-deployment.yaml
    sed -i "s|<resourceId>|/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP|g" $AZURE_POLICY_ADDON_FILE
}

configGPUDrivers() {
    # install gpu driver
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        mkdir -p /opt/{actions,gpu}
        if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
            ctr image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
            retrycmd_if_failure 5 10 600 bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install"
            ret=$?
            if [[ "$ret" != "0" ]]; then
                echo "Failed to install GPU driver, exiting..."
                exit $ERR_GPU_DRIVERS_START_FAIL
            fi
            ctr images rm --sync $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
        else
            bash -c "$DOCKER_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG install" 
            ret=$?
            if [[ "$ret" != "0" ]]; then
                echo "Failed to install GPU driver, exiting..."
                exit $ERR_GPU_DRIVERS_START_FAIL
            fi
            docker rmi $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
        fi
    elif [[ $OS == $MARINER_OS_NAME ]]; then
        downloadGPUDrivers
        installNvidiaContainerRuntime
        enableNvidiaPersistenceMode
    else 
        echo "os $OS not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    # validate on host, already done inside container.
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    fi

    retrycmd_if_failure 120 5 300 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
    
    # reload containerd/dockerd
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    else
        retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
}

validateGPUDrivers() {
    if [[ $(isARM64) == 1 ]]; then
        # no GPU on ARM64
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL
    which nvidia-smi
    if [[ $? == 0 ]]; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 $GPU_DEST/bin/nvidia-smi)
    fi
    SMI_STATUS=$?
    if [[ $SMI_STATUS != 0 ]]; then
        if [[ $SMI_RESULT == *"infoROM is corrupted"* ]]; then
            exit $ERR_GPU_INFO_ROM_CORRUPTED
        else
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
    else
        echo "gpu driver working fine"
    fi
}

ensureGPUDrivers() {
    if [[ $(isARM64) == 1 ]]; then
        # no GPU on ARM64
        return
    fi

    if [[ "${CONFIG_GPU_DRIVER_IF_NEEDED}" = true ]]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.configGPUDrivers" configGPUDrivers
    else
        logs_to_events "AKS.CSE.ensureGPUDrivers.validateGPUDrivers" validateGPUDrivers
    fi
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.nvidia-modprobe" "systemctlEnableAndStart nvidia-modprobe" || exit $ERR_GPU_DRIVERS_START_FAIL
    fi
}

disableSSH() {
    systemctlDisableAndStop ssh || exit $ERR_DISABLE_SSH
}

#EOF
`)

func linuxCloudInitArtifactsCse_configShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_configSh, nil
}

func linuxCloudInitArtifactsCse_configSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_configShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_config.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_helpersSh = []byte(`#!/bin/bash
# ERR_SYSTEMCTL_ENABLE_FAIL=3 Service could not be enabled by systemctl -- DEPRECATED 
ERR_SYSTEMCTL_START_FAIL=4 # Service could not be started or enabled by systemctl
ERR_CLOUD_INIT_TIMEOUT=5 # Timeout waiting for cloud-init runcmd to complete
ERR_FILE_WATCH_TIMEOUT=6 # Timeout waiting for a file
ERR_HOLD_WALINUXAGENT=7 # Unable to place walinuxagent apt package on hold during install
ERR_RELEASE_HOLD_WALINUXAGENT=8 # Unable to release hold on walinuxagent apt package after install
ERR_APT_INSTALL_TIMEOUT=9 # Timeout installing required apt packages
ERR_DOCKER_INSTALL_TIMEOUT=20 # Timeout waiting for docker install
ERR_DOCKER_DOWNLOAD_TIMEOUT=21 # Timout waiting for docker downloads
ERR_DOCKER_KEY_DOWNLOAD_TIMEOUT=22 # Timeout waiting to download docker repo key
ERR_DOCKER_APT_KEY_TIMEOUT=23 # Timeout waiting for docker apt-key
ERR_DOCKER_START_FAIL=24 # Docker could not be started by systemctl
ERR_MOBY_APT_LIST_TIMEOUT=25 # Timeout waiting for moby apt sources
ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT=26 # Timeout waiting for MS GPG key download
ERR_MOBY_INSTALL_TIMEOUT=27 # Timeout waiting for moby-docker install
ERR_CONTAINERD_INSTALL_TIMEOUT=28 # Timeout waiting for moby-containerd install
ERR_RUNC_INSTALL_TIMEOUT=29 # Timeout waiting for moby-runc install
ERR_K8S_RUNNING_TIMEOUT=30 # Timeout waiting for k8s cluster to be healthy
ERR_K8S_DOWNLOAD_TIMEOUT=31 # Timeout waiting for Kubernetes downloads
ERR_KUBECTL_NOT_FOUND=32 # kubectl client binary not found on local disk
ERR_IMG_DOWNLOAD_TIMEOUT=33 # Timeout waiting for img download
ERR_KUBELET_START_FAIL=34 # kubelet could not be started by systemctl
ERR_DOCKER_IMG_PULL_TIMEOUT=35 # Timeout trying to pull a Docker image
ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT=36 # Timeout trying to pull a containerd image via cli tool ctr
ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT=37 # Timeout trying to pull a containerd image via cli tool crictl
ERR_CONTAINERD_INSTALL_FILE_NOT_FOUND=38 # Unable to locate containerd debian pkg file
ERR_CNI_DOWNLOAD_TIMEOUT=41 # Timeout waiting for CNI downloads
ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT=42 # Timeout waiting for https://packages.microsoft.com/config/ubuntu/16.04/packages-microsoft-prod.deb
ERR_MS_PROD_DEB_PKG_ADD_FAIL=43 # Failed to add repo pkg file
# ERR_FLEXVOLUME_DOWNLOAD_TIMEOUT=44 Failed to add repo pkg file -- DEPRECATED
ERR_SYSTEMD_INSTALL_FAIL=48 # Unable to install required systemd version
ERR_MODPROBE_FAIL=49 # Unable to load a kernel module using modprobe
ERR_OUTBOUND_CONN_FAIL=50 # Unable to establish outbound connection
ERR_K8S_API_SERVER_CONN_FAIL=51 # Unable to establish connection to k8s api serve
ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL=52 # Unable to resolve k8s api server name
ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL=53 # Unable to resolve k8s api server name due to Azure DNS issue
ERR_KATA_KEY_DOWNLOAD_TIMEOUT=60 # Timeout waiting to download kata repo key
ERR_KATA_APT_KEY_TIMEOUT=61 # Timeout waiting for kata apt-key
ERR_KATA_INSTALL_TIMEOUT=62 # Timeout waiting for kata install
ERR_VHD_FILE_NOT_FOUND=65 # VHD log file not found on VM built from VHD distro (previously classified as exit code 124)
ERR_CONTAINERD_DOWNLOAD_TIMEOUT=70 # Timeout waiting for containerd downloads
ERR_RUNC_DOWNLOAD_TIMEOUT=71 # Timeout waiting for runc downloads
ERR_CUSTOM_SEARCH_DOMAINS_FAIL=80 # Unable to configure custom search domains
ERR_GPU_DOWNLOAD_TIMEOUT=83 # Timeout waiting for GPU driver download
ERR_GPU_DRIVERS_START_FAIL=84 # nvidia-modprobe could not be started by systemctl
ERR_GPU_DRIVERS_INSTALL_TIMEOUT=85 # Timeout waiting for GPU drivers install
ERR_GPU_DEVICE_PLUGIN_START_FAIL=86 # nvidia device plugin could not be started by systemctl
ERR_GPU_INFO_ROM_CORRUPTED=87 # info ROM corrupted error when executing nvidia-smi
ERR_SGX_DRIVERS_INSTALL_TIMEOUT=90 # Timeout waiting for SGX prereqs to download
ERR_SGX_DRIVERS_START_FAIL=91 # Failed to execute SGX driver binary
ERR_APT_DAILY_TIMEOUT=98 # Timeout waiting for apt daily updates
ERR_APT_UPDATE_TIMEOUT=99 # Timeout waiting for apt-get update to complete
ERR_CSE_PROVISION_SCRIPT_NOT_READY_TIMEOUT=100 # Timeout waiting for cloud-init to place this script on the vm
ERR_APT_DIST_UPGRADE_TIMEOUT=101 # Timeout waiting for apt-get dist-upgrade to complete
ERR_APT_PURGE_FAIL=102 # Error purging distro packages
ERR_SYSCTL_RELOAD=103 # Error reloading sysctl config
ERR_CIS_ASSIGN_ROOT_PW=111 # Error assigning root password in CIS enforcement
ERR_CIS_ASSIGN_FILE_PERMISSION=112 # Error assigning permission to a file in CIS enforcement
ERR_PACKER_COPY_FILE=113 # Error writing a file to disk during VHD CI
ERR_CIS_APPLY_PASSWORD_CONFIG=115 # Error applying CIS-recommended passwd configuration
ERR_SYSTEMD_DOCKER_STOP_FAIL=116 # Error stopping dockerd
ERR_CRICTL_DOWNLOAD_TIMEOUT=117 # Timeout waiting for crictl downloads
ERR_CRICTL_OPERATION_ERROR=118 # Error executing a crictl operation
ERR_CTR_OPERATION_ERROR=119 # Error executing a ctr containerd cli operation

# Azure Stack specific errors
ERR_AZURE_STACK_GET_ARM_TOKEN=120 # Error generating a token to use with Azure Resource Manager
ERR_AZURE_STACK_GET_NETWORK_CONFIGURATION=121 # Error fetching the network configuration for the node
ERR_AZURE_STACK_GET_SUBNET_PREFIX=122 # Error fetching the subnet address prefix for a subnet ID

# Error code 124 is returned when a `+"`"+`timeout`+"`"+` command times out, and --preserve-status is not specified: https://man7.org/linux/man-pages/man1/timeout.1.html
ERR_VHD_BUILD_ERROR=125 # Reserved for VHD CI exit conditions

ERR_SWAP_CREATE_FAIL=130 # Error allocating swap file
ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE=131 # Error insufficient disk space for swap file creation

ERR_TELEPORTD_DOWNLOAD_ERR=150 # Error downloading teleportd binary
ERR_TELEPORTD_INSTALL_ERR=151 # Error installing teleportd binary
ERR_ARTIFACT_STREAMING_DOWNLOAD_INSTALL=152 # Error downloading or installing mirror proxy and overlaybd components

ERR_HTTP_PROXY_CA_CONVERT=160 # Error converting http proxy ca cert from pem to crt format
ERR_UPDATE_CA_CERTS=161 # Error updating ca certs to include user-provided certificates

ERR_DISBALE_IPTABLES=170 # Error disabling iptables service

ERR_KRUSTLET_DOWNLOAD_TIMEOUT=171 # Timeout waiting for krustlet downloads
ERR_DISABLE_SSH=172 # Error disabling ssh service

ERR_VHD_REBOOT_REQUIRED=200 # Reserved for VHD reboot required exit condition
ERR_NO_PACKAGES_FOUND=201 # Reserved for no security packages found exit condition

ERR_SYSTEMCTL_MASK_FAIL=2 # Service could not be masked by systemctl

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
KUBECTL=/usr/local/bin/kubectl
DOCKER=/usr/bin/docker
# this will be empty during VHD build
# but vhd build runs with `+"`"+`set -o nounset`+"`"+`
# so needs a default value
# prefer empty string to avoid potential "it works but did something weird" scenarios
export GPU_DV="${GPU_DRIVER_VERSION:=}"
export GPU_DEST=/usr/local/nvidia
NVIDIA_DOCKER_VERSION=2.8.0-1
DOCKER_VERSION=1.13.1-1
NVIDIA_CONTAINER_RUNTIME_VERSION="3.6.0"
export NVIDIA_DRIVER_IMAGE_SHA="sha-e8873b"
export NVIDIA_DRIVER_IMAGE_TAG="${GPU_DV}-${NVIDIA_DRIVER_IMAGE_SHA}"
export NVIDIA_DRIVER_IMAGE="mcr.microsoft.com/aks/aks-gpu"
export CTR_GPU_INSTALL_CMD="ctr run --privileged --rm --net-host --with-ns pid:/proc/1/ns/pid --mount type=bind,src=/opt/gpu,dst=/mnt/gpu,options=rbind --mount type=bind,src=/opt/actions,dst=/mnt/actions,options=rbind"
export DOCKER_GPU_INSTALL_CMD="docker run --privileged --net=host --pid=host -v /opt/gpu:/mnt/gpu -v /opt/actions:/mnt/actions --rm"
APT_CACHE_DIR=/var/cache/apt/archives/
PERMANENT_CACHE_DIR=/root/aptcache/
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
CURL_OUTPUT=/tmp/curl_verbose.out

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout "${@}" && break || \
        if [ $i -eq $retries ]; then
            echo Executed \"$@\" $i times;
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}
retrycmd_if_failure_no_stats() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
retrycmd_get_tarball() {
    tar_retries=$1; wait_sleep=$2; tarball=$3; url=$4
    echo "${tar_retries} retries"
    for i in $(seq 1 $tar_retries); do
        tar -tzf $tarball && break || \
        if [ $i -eq $tar_retries ]; then
            return 1
        else
            timeout 60 curl -fsSLv $url -o $tarball 2>&1 | tee $CURL_OUTPUT >/dev/null
            if [[ $? != 0 ]]; then
                cat $CURL_OUTPUT
            fi
            sleep $wait_sleep
        fi
    done
}
retrycmd_curl_file() {
    curl_retries=$1; wait_sleep=$2; timeout=$3; filepath=$4; url=$5
    echo "${curl_retries} retries"
    for i in $(seq 1 $curl_retries); do
        [[ -f $filepath ]] && break
        if [ $i -eq $curl_retries ]; then
            return 1
        else
            timeout $timeout curl -fsSLv $url -o $filepath 2>&1 | tee $CURL_OUTPUT >/dev/null
            if [[ $? != 0 ]]; then
                cat $CURL_OUTPUT
            fi
            sleep $wait_sleep
        fi
    done
}
wait_for_file() {
    retries=$1; wait_sleep=$2; filepath=$3
    paved=/opt/azure/cloud-init-files.paved
    grep -Fq "${filepath}" $paved && return 0
    for i in $(seq 1 $retries); do
        grep -Fq '#EOF' $filepath && break
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    sed -i "/#EOF/d" $filepath
    echo $filepath >> $paved
}
systemctl_restart() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl restart $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            systemctl status $svcname --no-pager -l
            journalctl -u $svcname
            sleep $wait_sleep
        fi
    done
}
systemctl_stop() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl stop $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
systemctl_disable() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl disable $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
sysctl_reload() {
    retries=$1; wait_sleep=$2; timeout=$3
    for i in $(seq 1 $retries); do
        timeout $timeout sysctl --system && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
version_gte() {
  test "$(printf '%s\n' "$@" | sort -rV | head -n 1)" == "$1"
}

systemctlEnableAndStart() {
    systemctl_restart 100 5 30 $1
    RESTART_STATUS=$?
    systemctl status $1 --no-pager -l > /var/log/azure/$1-status.log
    if [ $RESTART_STATUS -ne 0 ]; then
        echo "$1 could not be started"
        return 1
    fi
    if ! retrycmd_if_failure 120 5 25 systemctl enable $1; then
        echo "$1 could not be enabled by systemctl"
        return 1
    fi
}

systemctlDisableAndStop() {
    if systemctl list-units --full --all | grep -q "$1.service"; then
        systemctl_stop 20 5 25 $1 || echo "$1 could not be stopped"
        systemctl_disable 20 5 25 $1 || echo "$1 could not be disabled"
    fi
}

# return true if a >= b 
semverCompare() {
    VERSION_A=$(echo $1 | cut -d "+" -f 1)
    VERSION_B=$(echo $2 | cut -d "+" -f 1)
    [[ "${VERSION_A}" == "${VERSION_B}" ]] && return 0
    sorted=$(echo ${VERSION_A} ${VERSION_B} | tr ' ' '\n' | sort -V )
    highestVersion=$(IFS= echo "${sorted}" | cut -d$'\n' -f2)
    [[ "${VERSION_A}" == ${highestVersion} ]] && return 0
    return 1
}
downloadDebPkgToFile() {
    PKG_NAME=$1
    PKG_VERSION=$2
    PKG_DIRECTORY=$3
    mkdir -p $PKG_DIRECTORY
    # shellcheck disable=SC2164
    pushd ${PKG_DIRECTORY}
    retrycmd_if_failure 10 5 600 apt-get download ${PKG_NAME}=${PKG_VERSION}*
    # shellcheck disable=SC2164
    popd
}
apt_get_download() {
  retries=$1; wait_sleep=$2; shift && shift;
  local ret=0
  pushd $APT_CACHE_DIR || return 1
  for i in $(seq 1 $retries); do
    dpkg --configure -a --force-confdef
    wait_for_apt_locks
    apt-get -o Dpkg::Options::=--force-confold download -y "${@}" && break
    if [ $i -eq $retries ]; then ret=1; else sleep $wait_sleep; fi
  done
  popd || return 1
  return $ret
}
getCPUArch() {
    arch=$(uname -m)
    if [[ ${arch,,} == "aarch64" || ${arch,,} == "arm64"  ]]; then
        echo "arm64"
    else
        echo "amd64"
    fi
}
isARM64() {
    if [[ $(getCPUArch) == "arm64" ]]; then
        echo 1
    else
        echo 0
    fi
}

logs_to_events() {
    # local vars here allow for nested function tracking
    # installContainerRuntime for example
    local task=$1; shift
    local eventsFileName=$(date +%s%3N)

    local startTime=$(date +"%F %T.%3N")
    ${@}
    ret=$?
    local endTime=$(date +"%F %T.%3N")

    # arg names are defined by GA and all these are required to be correctly read by GA
    # EventPid, EventTid are required to be int. No use case for them at this point.
    json_string=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "Completed: ${@}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo ${json_string} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json

    # this allows an error from the command at ${@} to be returned and correct code assigned in cse_main
    if [ "$ret" != "0" ]; then
      return $ret
    fi
}

should_skip_nvidia_drivers() {
    set -x
    body=$(curl -fsSL -H "Metadata: true" --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01")
    ret=$?
    if [ "$ret" != "0" ]; then
      return $ret
    fi
    should_skip=$(echo "$body" | jq -e '.compute.tagsList | map(select(.name | test("SkipGpuDriverInstall"; "i")))[0].value // "false" | test("true"; "i")')
    echo "$should_skip" # true or false
}
#HELPERSEOF
`)

func linuxCloudInitArtifactsCse_helpersShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_helpersSh, nil
}

func linuxCloudInitArtifactsCse_helpersSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_helpersShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_helpers.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_installSh = []byte(`#!/bin/bash

CC_SERVICE_IN_TMP=/opt/azure/containers/cc-proxy.service.in
CC_SOCKET_IN_TMP=/opt/azure/containers/cc-proxy.socket.in
CNI_CONFIG_DIR="/etc/cni/net.d"
CNI_BIN_DIR="/opt/cni/bin"
CNI_DOWNLOADS_DIR="/opt/cni/downloads"
CRICTL_DOWNLOAD_DIR="/opt/crictl/downloads"
CRICTL_BIN_DIR="/usr/local/bin"
CONTAINERD_DOWNLOADS_DIR="/opt/containerd/downloads"
RUNC_DOWNLOADS_DIR="/opt/runc/downloads"
K8S_DOWNLOADS_DIR="/opt/kubernetes/downloads"
UBUNTU_RELEASE=$(lsb_release -r -s)
TELEPORTD_PLUGIN_DOWNLOAD_DIR="/opt/teleportd/downloads"
TELEPORTD_PLUGIN_BIN_DIR="/usr/local/bin"
CONTAINERD_WASM_VERSIONS="v0.3.0 v0.5.1 v0.8.0"
MANIFEST_FILEPATH="/opt/azure/manifest.json"
MAN_DB_AUTO_UPDATE_FLAG_FILEPATH="/var/lib/man-db/auto-update"
CURL_OUTPUT=/tmp/curl_verbose.out

removeManDbAutoUpdateFlagFile() {
    rm -f $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

createManDbAutoUpdateFlagFile() {
    touch $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

cleanupContainerdDlFiles() {
    rm -rf $CONTAINERD_DOWNLOADS_DIR
}

installContainerRuntime() {
    if [ "${NEEDS_CONTAINERD}" == "true" ]; then
        echo "in installContainerRuntime - KUBERNETES_VERSION = ${KUBERNETES_VERSION}"
        local containerd_version
        if [ -f "$MANIFEST_FILEPATH" ]; then
            containerd_version="$(jq -r .containerd.edge "$MANIFEST_FILEPATH")"
        else
            echo "WARNING: containerd version not found in manifest, defaulting to hardcoded."
        fi

        containerd_patch_version="$(echo "$containerd_version" | cut -d- -f1)"
        containerd_revision="$(echo "$containerd_version" | cut -d- -f2)"
        if [ -z "$containerd_patch_version" ] || [ "$containerd_patch_version" == "null" ] || [ "$containerd_revision" == "null" ]; then
            echo "invalid container version: $containerd_version"
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi

        logs_to_events "AKS.CSE.installContainerRuntime.installStandaloneContainerd" "installStandaloneContainerd ${containerd_patch_version} ${containerd_revision}"
        echo "in installContainerRuntime - CONTAINERD_VERION = ${containerd_patch_version}"
    else
        installMoby
    fi
}

installNetworkPlugin() {
    if [[ "${NETWORK_PLUGIN}" = "azure" ]]; then
        installAzureCNI
    fi
    installCNI
    rm -rf $CNI_DOWNLOADS_DIR &
}

downloadCNI() {
    mkdir -p $CNI_DOWNLOADS_DIR
    CNI_TGZ_TMP=${CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadContainerdWasmShims() {
    for shim_version in $CONTAINERD_WASM_VERSIONS; do
        binary_version="$(echo "${shim_version}" | tr . -)"
        local containerd_wasm_url="https://acs-mirror.azureedge.net/containerd-wasm-shims/${shim_version}/linux/amd64"
        local containerd_wasm_filepath="/usr/local/bin"
        if [[ $(isARM64) == 1 ]]; then
            containerd_wasm_url="https://acs-mirror.azureedge.net/containerd-wasm-shims/${shim_version}/linux/arm64"
        fi

        if [ ! -f "$containerd_wasm_filepath/containerd-shim-spin-${shim_version}" ] || [ ! -f "$containerd_wasm_filepath/containerd-shim-slight-${shim_version}" ]; then
            retrycmd_if_failure 30 5 60 curl -fSLv -o "$containerd_wasm_filepath/containerd-shim-spin-${binary_version}-v1" "$containerd_wasm_url/containerd-shim-spin-v1" 2>&1 | tee $CURL_OUTPUT >/dev/null | grep -E "^(curl:.*)|([eE]rr.*)$" && (cat $CURL_OUTPUT && exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT)
            retrycmd_if_failure 30 5 60 curl -fSLv -o "$containerd_wasm_filepath/containerd-shim-slight-${binary_version}-v1" "$containerd_wasm_url/containerd-shim-slight-v1" 2>&1 | tee $CURL_OUTPUT >/dev/null | grep -E "^(curl:.*)|([eE]rr.*)$" && (cat $CURL_OUTPUT && exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT)
            retrycmd_if_failure 30 5 60 curl -fSLv -o "$containerd_wasm_filepath/containerd-shim-wws-${binary_version}-v1" "$containerd_wasm_url/containerd-shim-wws-v1" 2>&1 | tee $CURL_OUTPUT >/dev/null | grep -E "^(curl:.*)|([eE]rr.*)$" && (cat $CURL_OUTPUT && exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT)
            chmod 755 "$containerd_wasm_filepath/containerd-shim-spin-${binary_version}-v1"
            chmod 755 "$containerd_wasm_filepath/containerd-shim-slight-${binary_version}-v1"
            chmod 755 "$containerd_wasm_filepath/containerd-shim-wws-${binary_version}-v1"
        fi
    done
}

downloadAzureCNI() {
    mkdir -p $CNI_DOWNLOADS_DIR
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadCrictl() {
    CRICTL_VERSION=$1
    CPU_ARCH=$(getCPUArch) #amd64 or arm64
    mkdir -p $CRICTL_DOWNLOAD_DIR
    CRICTL_DOWNLOAD_URL="https://acs-mirror.azureedge.net/cri-tools/v${CRICTL_VERSION}/binaries/crictl-v${CRICTL_VERSION}-linux-${CPU_ARCH}.tar.gz"
    CRICTL_TGZ_TEMP=${CRICTL_DOWNLOAD_URL##*/}
    retrycmd_curl_file 10 5 60 "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ${CRICTL_DOWNLOAD_URL}
}

installCrictl() {
    CPU_ARCH=$(getCPUArch) #amd64 or arm64
    currentVersion=$(crictl --version 2>/dev/null | sed 's/crictl version //g')
    if [[ "${currentVersion}" != "" ]]; then
        echo "version ${currentVersion} of crictl already installed. skipping installCrictl of target version ${KUBERNETES_VERSION%.*}.0"
    else
        # this is only called during cse. VHDs should have crictl binaries pre-cached so no need to download.
        # if the vhd does not have crictl pre-baked, return early
        CRICTL_TGZ_TEMP="crictl-v${CRICTL_VERSION}-linux-${CPU_ARCH}.tar.gz"
        if [[ ! -f "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ]]; then
            rm -rf ${CRICTL_DOWNLOAD_DIR}
            echo "pre-cached crictl not found: skipping installCrictl"
            return 1
        fi
        echo "Unpacking crictl into ${CRICTL_BIN_DIR}"
        tar zxvf "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" -C ${CRICTL_BIN_DIR}
        chown root:root $CRICTL_BIN_DIR/crictl
        chmod 755 $CRICTL_BIN_DIR/crictl
    fi
}

downloadTeleportdPlugin() {
    DOWNLOAD_URL=$1
    TELEPORTD_VERSION=$2
    if [[ $(isARM64) == 1 ]]; then
        # no arm64 teleport binaries according to owner
        return
    fi

    if [[ -z ${DOWNLOAD_URL} ]]; then
        echo "download url parameter for downloadTeleportdPlugin was not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    if [[ -z ${TELEPORTD_VERSION} ]]; then
        echo "teleportd version not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    mkdir -p $TELEPORTD_PLUGIN_DOWNLOAD_DIR
    retrycmd_curl_file 10 5 60 "${TELEPORTD_PLUGIN_DOWNLOAD_DIR}/teleportd-v${TELEPORTD_VERSION}" "${DOWNLOAD_URL}/v${TELEPORTD_VERSION}/teleportd" || exit ${ERR_TELEPORTD_DOWNLOAD_ERR}
}

installTeleportdPlugin() {
    if [[ $(isARM64) == 1 ]]; then
        # no arm64 teleport binaries according to owner
        return
    fi

    CURRENT_VERSION=$(teleportd --version 2>/dev/null | sed 's/teleportd version v//g')
    local TARGET_VERSION="0.8.0"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${TARGET_VERSION}; then
        echo "currently installed teleportd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${TARGET_VERSION}. skipping installTeleportdPlugin."
    else
        downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} ${TARGET_VERSION}
        mv "${TELEPORTD_PLUGIN_DOWNLOAD_DIR}/teleportd-v${TELEPORTD_VERSION}" "${TELEPORTD_PLUGIN_BIN_DIR}/teleportd" || exit ${ERR_TELEPORTD_INSTALL_ERR}
        chmod 755 "${TELEPORTD_PLUGIN_BIN_DIR}/teleportd" || exit ${ERR_TELEPORTD_INSTALL_ERR}
    fi
    rm -rf ${TELEPORTD_PLUGIN_DOWNLOAD_DIR}
}

setupCNIDirs() {
    mkdir -p $CNI_BIN_DIR
    chown -R root:root $CNI_BIN_DIR
    chmod -R 755 $CNI_BIN_DIR

    mkdir -p $CNI_CONFIG_DIR
    chown -R root:root $CNI_CONFIG_DIR
    chmod 755 $CNI_CONFIG_DIR
}

installCNI() {
    CNI_TGZ_TMP=${CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}    # Use bash builtin % to remove the .tgz to look for a folder rather than tgz

    # We want to use the untar cni reference first. And if that doesn't exist on the vhd does the tgz?
    # And if tgz is already on the vhd then just untar into CNI_BIN_DIR
    # Latest VHD should have the untar, older should have the tgz. And who knows will have neither.
    if [[ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]]; then
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR
    else
        if [[ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]]; then
            downloadCNI
        fi

        tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_BIN_DIR
    fi

    chown -R root:root $CNI_BIN_DIR
}

installAzureCNI() {
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}         # Use bash builtin % to remove the .tgz to look for a folder rather than tgz

    # We want to use the untar azurecni reference first. And if that doesn't exist on the vhd does the tgz?
    # And if tgz is already on the vhd then just untar into CNI_BIN_DIR
    # Latest VHD should have the untar, older should have the tgz. And who knows will have neither.
    if [[ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]]; then
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR
    else
        if [[ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]]; then
            downloadAzureCNI
        fi

        tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_BIN_DIR
    fi

    chown -R root:root $CNI_BIN_DIR
}

extractKubeBinaries() {
    K8S_VERSION=$1
    KUBE_BINARY_URL=$2

    mkdir -p ${K8S_DOWNLOADS_DIR}
    K8S_TGZ_TMP=${KUBE_BINARY_URL##*/}
    retrycmd_get_tarball 120 5 "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" ${KUBE_BINARY_URL} || exit $ERR_K8S_DOWNLOAD_TIMEOUT
    tar --transform="s|.*|&-${K8S_VERSION}|" --show-transformed-names -xzvf "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" \
        --strip-components=3 -C /usr/local/bin kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl
    rm -f "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}"
}

installKubeletKubectlAndKubeProxy() {

    CUSTOM_KUBE_BINARY_DOWNLOAD_URL="${CUSTOM_KUBE_BINARY_URL:=}"
    if [[ ! -z ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL} ]]; then
        # remove the kubelet binaries to make sure the only binary left is from the CUSTOM_KUBE_BINARY_DOWNLOAD_URL
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*

        # NOTE(mainred): we expect kubelet binary to be under `+"`"+`kubernetes/node/bin`+"`"+`. This suits the current setting of
        # kube binaries used by AKS and Kubernetes upstream.
        # TODO(mainred): let's see if necessary to auto-detect the path of kubelet
        logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}

    else
        if [[ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ]]; then
            #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
            if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
                logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL}
            fi
        fi
    fi
    mv "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" "/usr/local/bin/kubelet"
    mv "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" "/usr/local/bin/kubectl"

    chmod a+x /usr/local/bin/kubelet /usr/local/bin/kubectl
    rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-* /home/hyperkube-downloads &
}

pullContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    echo "pulling the image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        logs_to_events "AKS.CSE.imagepullctr.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 60 1 1200 ctr --namespace k8s.io image pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via ctr" && exit $ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT)
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        logs_to_events "AKS.CSE.imagepullcrictl.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 60 1 1200 crictl pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via crictl" && exit $ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT)
    else
        logs_to_events "AKS.CSE.imagepull.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 60 1 1200 docker pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via docker" && exit $ERR_DOCKER_IMG_PULL_TIMEOUT)
    fi
}

retagContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    RETAG_IMAGE_URL=$3
    echo "retaging from ${CONTAINER_IMAGE_URL} to ${RETAG_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        ctr --namespace k8s.io image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        crictl image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    else
        docker image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    fi
}

retagMCRImagesForChina() {
    # retag all the mcr for mooncake
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        # shellcheck disable=SC2016
        allMCRImages=($(ctr --namespace k8s.io images list | grep '^mcr.microsoft.com/' | awk '{print $1}'))
    else
        # shellcheck disable=SC2016
        allMCRImages=($(docker images | grep '^mcr.microsoft.com/' | awk '{str = sprintf("%s:%s", $1, $2)} {print str}'))
    fi
    if [[ "${allMCRImages}" == "" ]]; then
        echo "failed to find mcr images for retag"
        return
    fi
    for mcrImage in ${allMCRImages[@]+"${allMCRImages[@]}"}; do
        # in mooncake, the mcr endpoint is: mcr.azk8s.cn
        # shellcheck disable=SC2001
        retagMCRImage=$(echo ${mcrImage} | sed -e 's/^mcr.microsoft.com/mcr.azk8s.cn/g')
        # can't use CLI_TOOL because crictl doesn't support retagging.
        if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
            retagContainerImage "ctr" ${mcrImage} ${retagMCRImage}
        else
            retagContainerImage "docker" ${mcrImage} ${retagMCRImage}
        fi
    done
}

removeContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    if [[ "${CLI_TOOL}" == "docker" ]]; then
        docker image rm $CONTAINER_IMAGE_URL
    else
        # crictl should always be present
        crictl rmi $CONTAINER_IMAGE_URL
    fi
}

cleanUpImages() {
    local targetImage=$1
    export targetImage
    function cleanupImagesRun() {
        if [ "${NEEDS_CONTAINERD}" == "true" ]; then
            if [[ "${CLI_TOOL}" == "crictl" ]]; then
                images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
            else
                images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
            fi
        else
            images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        fi
        local exit_code=$?
        if [[ $exit_code != 0 ]]; then
            exit $exit_code
        elif [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                if [ "${NEEDS_CONTAINERD}" == "true" ]; then
                    removeContainerImage ${CLI_TOOL} ${image}
                else
                    removeContainerImage "docker" ${image}
                fi
            done
        fi
    }
    export -f cleanupImagesRun
    retrycmd_if_failure 10 5 120 bash -c cleanupImagesRun
}

cleanUpKubeProxyImages() {
    echo $(date),$(hostname), startCleanUpKubeProxyImages
    cleanUpImages "kube-proxy"
    echo $(date),$(hostname), endCleanUpKubeProxyImages
}

cleanupRetaggedImages() {
    if [[ "${TARGET_CLOUD}" != "AzureChinaCloud" ]]; then
        if [ "${NEEDS_CONTAINERD}" == "true" ]; then
            if [[ "${CLI_TOOL}" == "crictl" ]]; then
                images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
            else
                images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
            fi
        else
            images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
        fi
        if [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                if [ "${NEEDS_CONTAINERD}" == "true" ]; then
                    # always use ctr, even if crictl is installed.
                    # crictl will remove *ALL* references to a given imageID (SHA), which removes too much.
                    removeContainerImage "ctr" ${image}
                else
                    removeContainerImage "docker" ${image}
                fi
            done
        fi
    else
        echo "skipping container cleanup for AzureChinaCloud"
    fi
}

cleanUpContainerImages() {
    export KUBERNETES_VERSION
    export CLI_TOOL
    export -f retrycmd_if_failure
    export -f removeContainerImage
    export -f cleanUpImages
    export -f cleanUpKubeProxyImages
    bash -c cleanUpKubeProxyImages &
}

cleanUpContainerd() {
    rm -Rf $CONTAINERD_DOWNLOADS_DIR
}

overrideNetworkConfig() {
    CONFIG_FILEPATH="/etc/cloud/cloud.cfg.d/80_azure_net_config.cfg"
    touch ${CONFIG_FILEPATH}
    cat <<EOF >>${CONFIG_FILEPATH}
datasource:
    Azure:
        apply_network_config: false
EOF
}
#EOF
`)

func linuxCloudInitArtifactsCse_installShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_installSh, nil
}

func linuxCloudInitArtifactsCse_installSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_installShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_install.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_mainSh = []byte(`#!/bin/bash
# Timeout waiting for a file
ERR_FILE_WATCH_TIMEOUT=6 
set -x
if [ -f /opt/azure/containers/provision.complete ]; then
      echo "Already ran to success exiting..."
      exit 0
fi

aptmarkWALinuxAgent hold &

# Setup logs for upload to host
LOG_DIR=/var/log/azure/aks
mkdir -p ${LOG_DIR}
ln -s /var/log/azure/cluster-provision.log \
      /var/log/azure/cluster-provision-cse-output.log \
      /opt/azure/*.json \
      /opt/azure/cloud-init-files.paved \
      /opt/azure/vhd-install.complete \
      ${LOG_DIR}/

# Redact the necessary secrets from cloud-config.txt so we don't expose any sensitive information
# when cloud-config.txt gets included within log bundles
python3 /opt/azure/containers/provision_redact_cloud_config.py \
    --cloud-config-path /var/lib/cloud/instance/cloud-config.txt \
    --output-path ${LOG_DIR}/cloud-config.txt

UBUNTU_RELEASE=$(lsb_release -r -s)
if [[ ${UBUNTU_RELEASE} == "16.04" ]]; then
    sudo apt-get -y autoremove chrony
    echo $?
    sudo systemctl restart systemd-timesyncd
fi

echo $(date),$(hostname), startcustomscript>>/opt/m

for i in $(seq 1 3600); do
    if [ -s "${CSE_HELPERS_FILEPATH}" ]; then
        grep -Fq '#HELPERSEOF' "${CSE_HELPERS_FILEPATH}" && break
    fi
    if [ $i -eq 3600 ]; then
        exit $ERR_FILE_WATCH_TIMEOUT
    else
        sleep 1
    fi
done
sed -i "/#HELPERSEOF/d" "${CSE_HELPERS_FILEPATH}"
source "${CSE_HELPERS_FILEPATH}"

source "${CSE_DISTRO_HELPERS_FILEPATH}"
source "${CSE_INSTALL_FILEPATH}"
source "${CSE_DISTRO_INSTALL_FILEPATH}"
source "${CSE_CONFIG_FILEPATH}"

if [[ "${DISABLE_SSH}" == "true" ]]; then
    disableSSH || exit $ERR_DISABLE_SSH
fi

# This involes using proxy, log the config before fetching packages
echo "private egress proxy address is '${PRIVATE_EGRESS_PROXY_ADDRESS}'"
# TODO update to use proxy

if [[ "${SHOULD_CONFIGURE_HTTP_PROXY}" == "true" ]]; then
    if [[ "${SHOULD_CONFIGURE_HTTP_PROXY_CA}" == "true" ]]; then
        configureHTTPProxyCA || exit $ERR_UPDATE_CA_CERTS
    fi
    configureEtcEnvironment
fi


if [[ "${SHOULD_CONFIGURE_CUSTOM_CA_TRUST}" == "true" ]]; then
    configureCustomCaCertificate || exit $ERR_UPDATE_CA_CERTS
fi

if [[ -n "${OUTBOUND_COMMAND}" ]]; then
    if [[ -n "${PROXY_VARS}" ]]; then
        eval $PROXY_VARS
    fi
    retrycmd_if_failure 50 1 5 $OUTBOUND_COMMAND >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || exit $ERR_OUTBOUND_CONN_FAIL;
fi

# Bring in OS-related vars
source /etc/os-release

# Mandb is not currently available on MarinerV1
if [[ ${ID} != "mariner" ]]; then
    echo "Removing man-db auto-update flag file..."
    logs_to_events "AKS.CSE.removeManDbAutoUpdateFlagFile" removeManDbAutoUpdateFlagFile
fi

export -f should_skip_nvidia_drivers
skip_nvidia_driver_install=$(retrycmd_if_failure_no_stats 10 1 10 bash -cx should_skip_nvidia_drivers)
ret=$?
if [[ "$ret" != "0" ]]; then
    echo "Failed to determine if nvidia driver install should be skipped"
    exit $ERR_NVIDIA_DRIVER_INSTALL
fi

if [[ "${GPU_NODE}" != "true" ]] || [[ "${skip_nvidia_driver_install}" == "true" ]]; then
    logs_to_events "AKS.CSE.cleanUpGPUDrivers" cleanUpGPUDrivers
fi

logs_to_events "AKS.CSE.disableSystemdResolved" disableSystemdResolved

logs_to_events "AKS.CSE.configureAdminUser" configureAdminUser

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
if [ -f $VHD_LOGS_FILEPATH ]; then
    echo "detected golden image pre-install"
    logs_to_events "AKS.CSE.cleanUpContainerImages" cleanUpContainerImages
    FULL_INSTALL_REQUIRED=false
else
    if [[ "${IS_VHD}" = true ]]; then
        echo "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
        exit $ERR_VHD_FILE_NOT_FOUND
    fi
    FULL_INSTALL_REQUIRED=true
fi

if [[ $OS == $UBUNTU_OS_NAME ]] && [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
    logs_to_events "AKS.CSE.installDeps" installDeps
else
    echo "Golden image; skipping dependencies installation"
fi

logs_to_events "AKS.CSE.installContainerRuntime" installContainerRuntime
if [ "${NEEDS_CONTAINERD}" == "true" ] && [ "${TELEPORT_ENABLED}" == "true" ]; then 
    logs_to_events "AKS.CSE.installTeleportdPlugin" installTeleportdPlugin
fi

setupCNIDirs

logs_to_events "AKS.CSE.installNetworkPlugin" installNetworkPlugin

if [ "${IS_KRUSTLET}" == "true" ]; then
    logs_to_events "AKS.CSE.downloadKrustlet" downloadContainerdWasmShims
fi

# By default, never reboot new nodes.
REBOOTREQUIRED=false

echo $(date),$(hostname), "Start configuring GPU drivers"
if [[ "${GPU_NODE}" = true ]] && [[ "${skip_nvidia_driver_install}" != "true" ]]; then
    logs_to_events "AKS.CSE.ensureGPUDrivers" ensureGPUDrivers
    if [[ "${ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED}" = true ]]; then
        if [[ "${MIG_NODE}" == "true" ]] && [[ -f "/etc/systemd/system/nvidia-device-plugin.service" ]]; then
            mkdir -p "/etc/systemd/system/nvidia-device-plugin.service.d"
            tee "/etc/systemd/system/nvidia-device-plugin.service.d/10-mig_strategy.conf" > /dev/null <<'EOF'
[Service]
Environment="MIG_STRATEGY=--mig-strategy single"
ExecStart=
ExecStart=/usr/local/nvidia/bin/nvidia-device-plugin $MIG_STRATEGY    
EOF
        fi
        logs_to_events "AKS.CSE.start.nvidia-device-plugin" "systemctlEnableAndStart nvidia-device-plugin" || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL
    else
        logs_to_events "AKS.CSE.stop.nvidia-device-plugin" "systemctlDisableAndStop nvidia-device-plugin"
    fi

    if [[ "${GPU_NEEDS_FABRIC_MANAGER}" == "true" ]]; then
        # fabric manager trains nvlink connections between multi instance gpus.
        # it appears this is only necessary for systems with *multiple cards*.
        # i.e., an A100 can be partitioned a maximum of 7 ways.
        # An NC24ads_A100_v4 has one A100.
        # An ND96asr_v4 has eight A100, for a maximum of 56 partitions.
        # ND96 seems to require fabric manager *even when not using mig partitions*
        # while it fails to install on NC24.
        if [[ $OS == $MARINER_OS_NAME ]]; then
            logs_to_events "AKS.CSE.installNvidiaFabricManager" installNvidiaFabricManager
        fi
        logs_to_events "AKS.CSE.nvidia-fabricmanager" "systemctlEnableAndStart nvidia-fabricmanager" || exit $ERR_GPU_DRIVERS_START_FAIL
    fi

    # This will only be true for multi-instance capable VM sizes
    # for which the user has specified a partitioning profile.
    # it is valid to use mig-capable gpus without a partitioning profile.
    if [[ "${MIG_NODE}" == "true" ]]; then
        # A100 GPU has a bit in the physical card (infoROM) to enable mig mode.
        # Changing this bit in either direction requires a VM reboot on Azure (hypervisor/plaform stuff).
        # Commands such as `+"`"+`nvidia-smi --gpu-reset`+"`"+` may succeed,
        # while commands such as `+"`"+`nvidia-smi -q`+"`"+` will show mismatched current/pending mig mode.
        # this will not be required per nvidia for next gen H100.
        REBOOTREQUIRED=true
        
        # this service applies the partitioning scheme with nvidia-smi.
        # we should consider moving to mig-parted which is simpler/newer.
        # we couldn't because of old drivers but that has long been fixed.
        logs_to_events "AKS.CSE.ensureMigPartition" ensureMigPartition
    fi
fi

echo $(date),$(hostname), "End configuring GPU drivers"

if [ "${NEEDS_DOCKER_LOGIN}" == "true" ]; then
    set +x
    docker login -u $SERVICE_PRINCIPAL_CLIENT_ID -p $SERVICE_PRINCIPAL_CLIENT_SECRET "${AZURE_PRIVATE_REGISTRY_SERVER}"
    set -x
fi

logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy" installKubeletKubectlAndKubeProxy

createKubeManifestDir

if [ "${HAS_CUSTOM_SEARCH_DOMAIN}" == "true" ]; then
    "${CUSTOM_SEARCH_DOMAIN_FILEPATH}" > /opt/azure/containers/setup-custom-search-domain.log 2>&1 || exit $ERR_CUSTOM_SEARCH_DOMAINS_FAIL
fi


# for drop ins, so they don't all have to check/create the dir
mkdir -p "/etc/systemd/system/kubelet.service.d"

logs_to_events "AKS.CSE.configureK8s" configureK8s

logs_to_events "AKS.CSE.configureCNI" configureCNI

# configure and enable dhcpv6 for dual stack feature
if [ "${IPV6_DUAL_STACK_ENABLED}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureDHCPv6" ensureDHCPv6
fi

if [ "${NEEDS_CONTAINERD}" == "true" ]; then
    # containerd should not be configured until cni has been configured first
    logs_to_events "AKS.CSE.ensureContainerd" ensureContainerd 
else
    logs_to_events "AKS.CSE.ensureDocker" ensureDocker
fi

if [[ "${MESSAGE_OF_THE_DAY}" != "" ]]; then
    echo "${MESSAGE_OF_THE_DAY}" | base64 -d > /etc/motd
fi

# must run before kubelet starts to avoid race in container status using wrong image
# https://github.com/kubernetes/kubernetes/issues/51017
# can remove when fixed
if [[ "${TARGET_CLOUD}" == "AzureChinaCloud" ]]; then
    retagMCRImagesForChina
fi

if [[ "${ENABLE_HOSTS_CONFIG_AGENT}" == "true" ]]; then
    logs_to_events "AKS.CSE.configPrivateClusterHosts" configPrivateClusterHosts
fi

if [ "${SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE}" == "true" ]; then
    logs_to_events "AKS.CSE.configureTransparentHugePage" configureTransparentHugePage
fi

if [ "${SHOULD_CONFIG_SWAP_FILE}" == "true" ]; then
    logs_to_events "AKS.CSE.configureSwapFile" configureSwapFile
fi

if [ "${NEEDS_CGROUPV2}" == "true" ]; then
    tee "/etc/systemd/system/kubelet.service.d/10-cgroupv2.conf" > /dev/null <<EOF
[Service]
Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"
EOF
fi

if [ "${NEEDS_CONTAINERD}" == "true" ]; then
    # gross, but the backticks make it very hard to do in Go
    # TODO: move entirely into vhd.
    # alternatively, can we verify this is safe with docker?
    # or just do it even if not because docker is out of support?
    mkdir -p /etc/containerd
    echo "${KUBENET_TEMPLATE}" | base64 -d > /etc/containerd/kubenet_template.conf

    # In k8s 1.27, the flag --container-runtime was removed.
    # We now have 2 drop-in's, one with the still valid flags that will be applied to all k8s versions,
    # the flags are --runtime-request-timeout, --container-runtime-endpoint, --runtime-cgroups
    # For k8s >= 1.27, the flag --container-runtime will not be passed.
    tee "/etc/systemd/system/kubelet.service.d/10-containerd-base-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service"
EOF
    
    # if k8s version < 1.27.0, add the drop in for --container-runtime flag
    if ! semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.27.0"; then
        tee "/etc/systemd/system/kubelet.service.d/10-container-runtime-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINER_RUNTIME_FLAG=--container-runtime=remote"
EOF
    fi
fi

if [ "${HAS_KUBELET_DISK_TYPE}" == "true" ]; then
    tee "/etc/systemd/system/kubelet.service.d/10-bindmount.conf" > /dev/null <<EOF
[Unit]
Requires=bind-mount.service
After=bind-mount.service
EOF
fi

logs_to_events "AKS.CSE.ensureSysctl" ensureSysctl

if [ "${NEEDS_CONTAINERD}" == "true" ] &&  [ "${SHOULD_CONFIG_CONTAINERD_ULIMITS}" == "true" ]; then
  logs_to_events "AKS.CSE.setContainerdUlimits" configureContainerdUlimits
fi

logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet
if [ "${ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureNoDupOnPromiscuBridge" ensureNoDupOnPromiscuBridge
fi

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # mitigation for bug https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1676635 
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi

VALIDATION_ERR=0

# Edge case scenarios:
# high retry times to wait for new API server DNS record to replicate (e.g. stop and start cluster)
# high timeout to address high latency for private dns server to forward request to Azure DNS
# dns check will be done only if we use FQDN for API_SERVER_NAME
API_SERVER_CONN_RETRIES=50
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
    API_SERVER_CONN_RETRIES=100
fi
if ! [[ ${API_SERVER_NAME} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    API_SERVER_DNS_RETRIES=100
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
       API_SERVER_DNS_RETRIES=200
    fi
    if [[ "${ENABLE_HOSTS_CONFIG_AGENT}" != "true" ]]; then
        RES=$(logs_to_events "AKS.CSE.apiserverNslookup" "retrycmd_if_failure ${API_SERVER_DNS_RETRIES} 1 20 nslookup -timeout=5 -retry=0 ${API_SERVER_NAME}")
        STS=$?
    else
        STS=0
    fi
    if [[ $STS != 0 ]]; then
        time nslookup ${API_SERVER_NAME}
        if [[ $RES == *"168.63.129.16"*  ]]; then
            VALIDATION_ERR=$ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL
        else
            VALIDATION_ERR=$ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL
        fi
    else
        logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
    fi
else
    logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
fi

if [[ ${ID} != "mariner" ]]; then
    echo "Recreating man-db auto-update flag file and kicking off man-db update process at $(date)"
    createManDbAutoUpdateFlagFile
    /usr/bin/mandb && echo "man-db finished updates at $(date)" &
fi

if $REBOOTREQUIRED; then
    echo 'reboot required, rebooting node in 1 minute'
    /bin/bash -c "shutdown -r 1 &"
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # logs_to_events should not be run on & commands
        aptmarkWALinuxAgent unhold &
    fi
else
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # logs_to_events should not be run on & commands
        if [ "${ENABLE_UNATTENDED_UPGRADES}" == "true" ]; then
            UU_CONFIG_DIR="/etc/apt/apt.conf.d/99periodic"
            mkdir -p "$(dirname "${UU_CONFIG_DIR}")"
            touch "${UU_CONFIG_DIR}"
            chmod 0644 "${UU_CONFIG_DIR}"
            echo 'APT::Periodic::Update-Package-Lists "1";' >> "${UU_CONFIG_DIR}"
            echo 'APT::Periodic::Unattended-Upgrade "1";' >> "${UU_CONFIG_DIR}"
            systemctl unmask apt-daily.service apt-daily-upgrade.service
            systemctl enable apt-daily.service apt-daily-upgrade.service
            systemctl enable apt-daily.timer apt-daily-upgrade.timer
            systemctl restart --no-block apt-daily.timer apt-daily-upgrade.timer            
            # this is the DOWNLOAD service
            # meaning we are wasting IO without even triggering an upgrade 
            # -________________-
            systemctl restart --no-block apt-daily.service
            
        fi
        aptmarkWALinuxAgent unhold &
    elif [[ $OS == $MARINER_OS_NAME ]]; then
        if [ "${ENABLE_UNATTENDED_UPGRADES}" == "true" ]; then
            if [ "${IS_KATA}" == "true" ]; then
                # Currently kata packages must be updated as a unit (including the kernel which requires a reboot). This can
                # only be done reliably via image updates as of now so never enable automatic updates.
                echo 'EnableUnattendedUpgrade is not supported by kata images, will not be enabled'
            else
                # By default the dnf-automatic is service is notify only in Mariner.
                # Enable the automatic install timer and the check-restart timer.
                # Stop the notify only dnf timer since we've enabled the auto install one.
                # systemctlDisableAndStop adds .service to the end which doesn't work on timers.
                systemctl disable dnf-automatic-notifyonly.timer
                systemctl stop dnf-automatic-notifyonly.timer
                # At 6:00:00 UTC (1 hour random fuzz) download and install package updates.
                systemctl unmask dnf-automatic-install.service || exit $ERR_SYSTEMCTL_START_FAIL
                systemctl unmask dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
                systemctlEnableAndStart dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
                # The check-restart service which will inform kured of required restarts should already be running
            fi
        fi
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

exit $VALIDATION_ERR


#EOF
`)

func linuxCloudInitArtifactsCse_mainShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_mainSh, nil
}

func linuxCloudInitArtifactsCse_mainSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_mainShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_main.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_redact_cloud_configPy = []byte(`import yaml
import argparse

# String value used to replace secret data
REDACTED = 'REDACTED'

# Redact functions
def redact_bootstrap_kubeconfig_tls_token(bootstrap_kubeconfig_write_file):
    content_yaml = yaml.safe_load(bootstrap_kubeconfig_write_file['content'])
    content_yaml['users'][0]['user']['token'] = REDACTED
    bootstrap_kubeconfig_write_file['content'] = yaml.dump(content_yaml)


def redact_service_principal_secret(sp_secret_write_file):
    sp_secret_write_file['content'] = REDACTED


# Maps write_file's path to the corresponding function used to redact it within cloud-config.txt
# This script will always redact these write_files if they exist within the specified cloud-config.txt
PATH_TO_REDACT_FUNC = {
    '/var/lib/kubelet/bootstrap-kubeconfig': redact_bootstrap_kubeconfig_tls_token,
    '/etc/kubernetes/sp.txt': redact_service_principal_secret
}


def redact_cloud_config(cloud_config_path, output_path):
    target_paths = set(PATH_TO_REDACT_FUNC.keys())

    with open(cloud_config_path, 'r') as f:
        cloud_config_data = f.read()
    cloud_config = yaml.safe_load(cloud_config_data)

    for write_file in cloud_config['write_files']:
        if write_file['path'] in target_paths:
            target_path = write_file['path']
            target_paths.remove(target_path)

            print('Redacting secrets from write_file: ' + target_path)
            PATH_TO_REDACT_FUNC[target_path](write_file)

        if len(target_paths) == 0:
            break


    print('Dumping redacted cloud-config to: ' + output_path)
    with open(output_path, 'w+') as output_file:
        output_file.write(yaml.dump(cloud_config))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Command line utility used to redact secrets from write_file definitions for ' +
            str([", ".join(PATH_TO_REDACT_FUNC)]) + ' within a specified cloud-config.txt. \
            These secrets must be redacted before cloud-config.txt can be collected for logging.')
    parser.add_argument(
        "--cloud-config-path",
        required=True,
        type=str,
        help='Path to cloud-config.txt to redact')
    parser.add_argument(
        "--output-path",
        required=True,
        type=str,
        help='Path to the newly generated cloud-config.txt with redacted secrets')

    args = parser.parse_args()
    redact_cloud_config(args.cloud_config_path, args.output_path)`)

func linuxCloudInitArtifactsCse_redact_cloud_configPyBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_redact_cloud_configPy, nil
}

func linuxCloudInitArtifactsCse_redact_cloud_configPy() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_redact_cloud_configPyBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_redact_cloud_config.py", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_send_logsPy = []byte(`#! /usr/bin/env python3

import urllib3
import uuid
import xml.etree.ElementTree as ET

http = urllib3.PoolManager()

# Get the container_id and deployment_id from the Goal State
goal_state_xml = http.request(
        'GET',
        'http://168.63.129.16/machine/?comp=goalstate',
        headers={
            'x-ms-version': '2012-11-30'
        }
    )
goal_state = ET.fromstring(goal_state_xml.data.decode('utf-8'))
container_id = goal_state.findall('./Container/ContainerId')[0].text
role_config_name = goal_state.findall('./Container/RoleInstanceList/RoleInstance/Configuration/ConfigName')[0].text
deployment_id = role_config_name.split('.')[0]

# Upload the logs
with open('/var/lib/waagent/logcollector/logs.zip', 'rb') as logs:
    logs_data = logs.read()
    upload_logs = http.request(
        'PUT',
        'http://168.63.129.16:32526/vmAgentLog',
        headers={
            'x-ms-version': '2015-09-01',
            'x-ms-client-correlationid': str(uuid.uuid4()),
            'x-ms-client-name': 'AKSCSEPlugin',
            'x-ms-client-version': '0.1.0',
            'x-ms-containerid': container_id,
            'x-ms-vmagentlog-deploymentid': deployment_id,
        },
        body=logs_data,
    )

if upload_logs.status == 200:
    print("Successfully uploaded logs")
    exit(0)
else:
    print('Failed to upload logs')
    print(f'Response status: {upload_logs.status}')
    print(f'Response body:\n{upload_logs.data.decode("utf-8")}')
    exit(1)
`)

func linuxCloudInitArtifactsCse_send_logsPyBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_send_logsPy, nil
}

func linuxCloudInitArtifactsCse_send_logsPy() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_send_logsPyBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_send_logs.py", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsCse_startSh = []byte(`CSE_STARTTIME=$(date)
CSE_STARTTIME_FORMATTED=$(date +"%F %T.%3N")
timeout -k5s 15m /bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(tail -c 3000 "/var/log/azure/cluster-provision.log")
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
KERNEL_STARTTIME_FORMATTED=$(date -d "${KERNEL_STARTTIME}" +"%F %T.%3N" )
CLOUDINITLOCAL_STARTTIME=$(systemctl show cloud-init-local -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITLOCAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITLOCAL_STARTTIME}" +"%F %T.%3N" )
CLOUDINIT_STARTTIME=$(systemctl show cloud-init -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINIT_STARTTIME_FORMATTED=$(date -d "${CLOUDINIT_STARTTIME}" +"%F %T.%3N" )
CLOUDINITFINAL_STARTTIME=$(systemctl show cloud-final -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITFINAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITFINAL_STARTTIME}" +"%F %T.%3N" )
NETWORKD_STARTTIME=$(systemctl show systemd-networkd -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
NETWORKD_STARTTIME_FORMATTED=$(date -d "${NETWORKD_STARTTIME}" +"%F %T.%3N" )
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
GUEST_AGENT_STARTTIME_FORMATTED=$(date -d "${GUEST_AGENT_STARTTIME}" +"%F %T.%3N" )
KUBELET_START_TIME=$(systemctl show kubelet.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
KUBELET_START_TIME_FORMATTED=$(date -d "${KUBELET_START_TIME}" +"%F %T.%3N" )
KUBELET_READY_TIME_FORMATTED="$(date -d "$(journalctl -u kubelet | grep NodeReady | cut -d' ' -f1-3)" +"%F %T.%3N")"
SYSTEMD_SUMMARY=$(systemd-analyze || true)
CSE_ENDTIME_FORMATTED=$(date +"%F %T.%3N")
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
EXECUTION_DURATION=$(echo $(($(date +%s) - $(date -d "$CSE_STARTTIME" +%s))))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg cinitl "$CLOUDINITLOCAL_STARTTIME" \
                  --arg cinit "$CLOUDINIT_STARTTIME" \
                  --arg cf "$CLOUDINITFINAL_STARTTIME" \
                  --arg ns "$NETWORKD_STARTTIME" \
                  --arg cse "$CSE_STARTTIME" \
                  --arg ga "$GUEST_AGENT_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  --arg kubelet "$KUBELET_START_TIME" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CloudInitLocalStartTime: $cinitl, CloudInitStartTime: $cinit, CloudFinalStartTime: $cf, NetworkdStartTime: $ns, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, KubeletStartTime: $kubelet }}' )
mkdir -p /var/log/azure/aks
echo $JSON_STRING | tee /var/log/azure/aks/provision.json

# messsage_string is here because GA only accepts strings in Message.
message_string=$( jq -n \
--arg EXECUTION_DURATION                  "${EXECUTION_DURATION}" \
--arg EXIT_CODE                           "${EXIT_CODE}" \
--arg KERNEL_STARTTIME_FORMATTED          "${KERNEL_STARTTIME_FORMATTED}" \
--arg CLOUDINITLOCAL_STARTTIME_FORMATTED  "${CLOUDINITLOCAL_STARTTIME_FORMATTED}" \
--arg CLOUDINIT_STARTTIME_FORMATTED       "${CLOUDINIT_STARTTIME_FORMATTED}" \
--arg CLOUDINITFINAL_STARTTIME_FORMATTED  "${CLOUDINITFINAL_STARTTIME_FORMATTED}" \
--arg NETWORKD_STARTTIME_FORMATTED        "${NETWORKD_STARTTIME_FORMATTED}" \
--arg GUEST_AGENT_STARTTIME_FORMATTED     "${GUEST_AGENT_STARTTIME_FORMATTED}" \
--arg KUBELET_START_TIME_FORMATTED        "${KUBELET_START_TIME_FORMATTED}" \
--arg KUBELET_READY_TIME_FORMATTED       "${KUBELET_READY_TIME_FORMATTED}" \
'{ExitCode: $EXIT_CODE, E2E: $EXECUTION_DURATION, KernelStartTime: $KERNEL_STARTTIME_FORMATTED, CloudInitLocalStartTime: $CLOUDINITLOCAL_STARTTIME_FORMATTED, CloudInitStartTime: $CLOUDINIT_STARTTIME_FORMATTED, CloudFinalStartTime: $CLOUDINITFINAL_STARTTIME_FORMATTED, NetworkdStartTime: $NETWORKD_STARTTIME_FORMATTED, GuestAgentStartTime: $GUEST_AGENT_STARTTIME_FORMATTED, KubeletStartTime: $KUBELET_START_TIME_FORMATTED, KubeletReadyTime: $KUBELET_READY_TIME_FORMATTED } | tostring'
)
# this clean up brings me no joy, but removing extra "\" and then removing quotes at the end of the string
# allows parsing to happening without additional manipulation
message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

# arg names are defined by GA and all these are required to be correctly read by GA
# EventPid, EventTid are required to be int. No use case for them at this point.
EVENT_JSON=$( jq -n \
    --arg Timestamp     "${CSE_STARTTIME_FORMATTED}" \
    --arg OperationId   "${CSE_ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.CSE.cse_start" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)
echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json

# force a log upload to the host after the provisioning script finishes
# if we failed, wait for the upload to complete so that we don't remove
# the VM before it finishes. if we succeeded, upload in the background
# so that the provisioning script returns success more quickly
upload_logs() {
    # find the most recent version of WALinuxAgent and use it to collect logs per
    # https://supportability.visualstudio.com/AzureIaaSVM/_wiki/wikis/AzureIaaSVM/495009/Log-Collection_AGEX?anchor=manually-collect-logs
    PYTHONPATH=$(find /var/lib/waagent -name WALinuxAgent\*.egg | sort -rV | head -n1)
    python3 $PYTHONPATH -collect-logs -full >/dev/null 2>&1
    python3 /opt/azure/containers/provision_send_logs.py >/dev/null 2>&1
}
if [ $EXIT_CODE -ne 0 ]; then
    upload_logs
else
    upload_logs &
fi

exit $EXIT_CODE`)

func linuxCloudInitArtifactsCse_startShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsCse_startSh, nil
}

func linuxCloudInitArtifactsCse_startSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsCse_startShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/cse_start.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsDhcpv6Service = []byte(`[Unit]
Description=enabledhcpv6
After=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/enable-dhcpv6.sh

[Install]
WantedBy=multi-user.target
#EOF
`)

func linuxCloudInitArtifactsDhcpv6ServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsDhcpv6Service, nil
}

func linuxCloudInitArtifactsDhcpv6Service() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsDhcpv6ServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/dhcpv6.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsDisk_queueService = []byte(`[Unit]
Description=Set nr_requests and queue_depth based on experimental tuning

[Service]
Type=oneshot
ExecStart=/usr/bin/env bash -c 'echo 128 > /sys/block/sda/queue/nr_requests && echo 128 > /sys/block/sda/device/queue_depth'
RemainAfterExit=true
StandardOutput=journal

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsDisk_queueServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsDisk_queueService, nil
}

func linuxCloudInitArtifactsDisk_queueService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsDisk_queueServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/disk_queue.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsDockerMonitorService = []byte(`[Unit]
Description=a script that checks docker health and restarts if needed
After=docker.service
[Service]
Restart=always
RestartSec=10
RemainAfterExit=yes
ExecStart=/usr/local/bin/health-monitor.sh container-runtime docker
#EOF
`)

func linuxCloudInitArtifactsDockerMonitorServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsDockerMonitorService, nil
}

func linuxCloudInitArtifactsDockerMonitorService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsDockerMonitorServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/docker-monitor.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsDockerMonitorTimer = []byte(`[Unit]
Description=a timer that delays docker-monitor from starting too soon after boot
[Timer]
Unit=docker-monitor.service
OnBootSec=10min
[Install]
WantedBy=multi-user.target
#EOF
`)

func linuxCloudInitArtifactsDockerMonitorTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsDockerMonitorTimer, nil
}

func linuxCloudInitArtifactsDockerMonitorTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsDockerMonitorTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/docker-monitor.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf = []byte(`[Service]
MountFlags=shared
#EOF
`)

func linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf, nil
}

func linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsEnableDhcpv6Sh = []byte(`#!/usr/bin/env bash

set -e
set -o pipefail
set -u

DHCLIENT6_CONF_FILE=/etc/dhcp/dhclient6.conf
CLOUD_INIT_CFG=/etc/network/interfaces.d/50-cloud-init.cfg

read -r -d '' NETWORK_CONFIGURATION << EOC || true
iface eth0 inet6 auto
    up sleep 5
    up dhclient -1 -6 -cf /etc/dhcp/dhclient6.conf -lf /var/lib/dhcp/dhclient6.eth0.leases -v eth0 || true
EOC

add_if_not_exists() {
    grep -qxF "${1}" "${2}" || echo "${1}" >> "${2}"
}

echo "Configuring dhcpv6 ..."

touch /etc/dhcp/dhclient6.conf && add_if_not_exists "timeout 10;" ${DHCLIENT6_CONF_FILE} && \
    add_if_not_exists "${NETWORK_CONFIGURATION}" ${CLOUD_INIT_CFG} && \
    sudo ifdown eth0 && sudo ifup eth0

echo "Configuration complete"
#EOF
`)

func linuxCloudInitArtifactsEnableDhcpv6ShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEnableDhcpv6Sh, nil
}

func linuxCloudInitArtifactsEnableDhcpv6Sh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEnableDhcpv6ShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/enable-dhcpv6.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsEnsureNoDupService = []byte(`[Unit]
Description=Add dedup ebtable rules for kubenet bridge in promiscuous mode
After=containerd.service
After=kubelet.service
[Service]
Restart=on-failure
RestartSec=2
ExecStart=/bin/bash /opt/azure/containers/ensure-no-dup.sh
#EOF
`)

func linuxCloudInitArtifactsEnsureNoDupServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEnsureNoDupService, nil
}

func linuxCloudInitArtifactsEnsureNoDupService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEnsureNoDupServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ensure-no-dup.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsEnsureNoDupSh = []byte(`#!/bin/bash

# remove this if we are no longer using promiscuous bridge mode for containerd
# background: we get duplicated packets from pod to serviceIP if both are on the same node (one from the cbr0 bridge and one from the pod ip itself via kernel due to promiscuous mode being on)
# we should filter out the one from pod ip
# this is exactly what kubelet does for dockershim+kubenet
# https://github.com/kubernetes/kubernetes/pull/28717

ebtables -t filter -L AKS-DEDUP-PROMISC 2>/dev/null
if [[ $? -eq 0 ]]; then
    echo "AKS-DEDUP-PROMISC rule already set"
    exit 0
fi
if [[ ! -f /etc/cni/net.d/10-containerd-net.conflist ]]; then
    echo "cni config not up yet...exiting early"
    exit 1
fi

bridgeName=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .bridge")
promiscMode=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .promiscMode")
if [[ "${promiscMode}" != "true" ]]; then
    echo "bridge ${bridgeName} not in promiscuous mode...exiting early"
    exit 0
fi

if [[ ! -f /sys/class/net/${bridgeName}/address ]]; then
    echo "bridge ${bridgeName} not up yet...exiting early"
    exit 1
fi


bridgeIP=$(ip addr show ${bridgeName} | grep -Eo "inet ([0-9]*\.){3}[0-9]*" | grep -Eo "([0-9]*\.){3}[0-9]*")
if [[ -z "${bridgeIP}" ]]; then
    echo "bridge ${bridgeName} does not have an ipv4 address...exiting early"
    exit 1
fi

podSubnetAddr=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .ipam.subnet")
if [[ -z "${podSubnetAddr}" ]]; then
    echo "could not determine this node's pod ipam subnet range from 10-containerd-net.conflist...exiting early"
    exit 1
fi

bridgeMAC=$(cat /sys/class/net/${bridgeName}/address)

echo "adding AKS-DEDUP-PROMISC ebtable chain"
ebtables -t filter -N AKS-DEDUP-PROMISC # add new AKS-DEDUP-PROMISC chain
ebtables -t filter -A AKS-DEDUP-PROMISC -p IPv4 -s ${bridgeMAC} -o veth+ --ip-src ${bridgeIP} -j ACCEPT
ebtables -t filter -A AKS-DEDUP-PROMISC -p IPv4 -s ${bridgeMAC} -o veth+ --ip-src ${podSubnetAddr} -j DROP
ebtables -t filter -A OUTPUT -j AKS-DEDUP-PROMISC # add new rule to OUTPUT chain jump to AKS-DEDUP-PROMISC

echo "outputting newly added AKS-DEDUP-PROMISC rules:"
ebtables -t filter -L OUTPUT 2>/dev/null
ebtables -t filter -L AKS-DEDUP-PROMISC 2>/dev/null
exit 0
#EOF`)

func linuxCloudInitArtifactsEnsureNoDupShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEnsureNoDupSh, nil
}

func linuxCloudInitArtifactsEnsureNoDupSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEnsureNoDupShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ensure-no-dup.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsEtcIssue = []byte(`
Authorized uses only. All activity may be monitored and reported.
`)

func linuxCloudInitArtifactsEtcIssueBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEtcIssue, nil
}

func linuxCloudInitArtifactsEtcIssue() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEtcIssueBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/etc-issue", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsEtcIssueNet = []byte(`
Authorized uses only. All activity may be monitored and reported.
`)

func linuxCloudInitArtifactsEtcIssueNetBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEtcIssueNet, nil
}

func linuxCloudInitArtifactsEtcIssueNet() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEtcIssueNetBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/etc-issue.net", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsHealthMonitorSh = []byte(`#!/usr/bin/env bash

# This script originated at https://github.com/kubernetes/kubernetes/blob/master/cluster/gce/gci/health-monitor.sh
# and has been modified for aks-engine.

set -o nounset
set -o pipefail

container_runtime_monitoring() {
  local -r max_attempts=5
  local attempt=1
  local -r container_runtime_name=$1

  if [[ ${container_runtime_name} == "containerd" ]]; then
    local healthcheck_command="ctr --namespace k8s.io container list"
  else 
    local healthcheck_command="docker ps"
  fi

  until timeout 60 ${healthcheck_command} > /dev/null; do
    if (( attempt == max_attempts )); then
      echo "Max attempt ${max_attempts} reached! Proceeding to monitor container runtime healthiness."
      break
    fi
    echo "$attempt initial attempt \"${healthcheck_command}\"! Trying again in $attempt seconds..."
    sleep "$(( 2 ** attempt++ ))"
  done
  while true; do
    if ! timeout 60 ${healthcheck_command} > /dev/null; then
      echo "Container runtime ${container_runtime_name} failed!"
      if [[ "$container_runtime_name" == "containerd" ]]; then
        pkill -SIGUSR1 containerd
      else 
        pkill -SIGUSR1 dockerd
      fi
      systemctl kill --kill-who=main "${container_runtime_name}"
      sleep 120
    else
      sleep "${SLEEP_SECONDS}"
    fi
  done
}

kubelet_monitoring() {
  echo "Wait for 2 minutes for kubelet to be functional"
  sleep 120
  local -r max_seconds=10
  local output=""
  while true; do
    if ! output=$(curl -m "${max_seconds}" -f -s -S http://127.0.0.1:10255/healthz 2>&1); then
      echo $output
      echo "Kubelet is unhealthy!"
      systemctl kill kubelet
      sleep 60
    else
      sleep "${SLEEP_SECONDS}"
    fi
  done
}

if [[ "$#" -lt 1 ]]; then
  echo "Usage: health-monitor.sh <container-runtime/kubelet>"
  exit 1
fi

component=$1
if [[ "${component}" == "container-runtime" ]]; then
  if [[ -z $2 ]]; then
    echo "Usage: health-monitor.sh container-runtime <docker/containerd>"
    exit 1
  fi
  container_runtime=$2
fi

KUBE_HOME="/usr/local/bin"
KUBE_ENV="/etc/default/kube-env"
if [[  -e "${KUBE_ENV}" ]]; then
  source "${KUBE_ENV}"
fi

SLEEP_SECONDS=10

echo "Start kubernetes health monitoring for ${component}"

if [[ "${component}" == "container-runtime" ]]; then
  container_runtime_monitoring ${container_runtime}
elif [[ "${component}" == "kubelet" ]]; then
  kubelet_monitoring
else
  echo "Health monitoring for component ${component} is not supported!"
fi
`)

func linuxCloudInitArtifactsHealthMonitorShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsHealthMonitorSh, nil
}

func linuxCloudInitArtifactsHealthMonitorSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsHealthMonitorShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/health-monitor.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsInitAksCustomCloudMarinerSh = []byte(`#!/bin/bash
mkdir -p /root/AzureCACertificates
# http://168.63.129.16 is a constant for the host's wireserver endpoint
certs=$(curl "http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json")
IFS_backup=$IFS
IFS=$'\r\n'
certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
for i in ${!certBodies[@]}; do
    echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${certNames[$i]} | sed 's/.cer/.crt/g')"
done
IFS=$IFS_backup

cp /root/AzureCACertificates/*.crt /etc/pki/ca-trust/source/anchors/
/usr/bin/update-ca-trust

cloud-init status --wait

# TODO - Set the repoDepotEndpoint in a .repo file if package update becomes necessary

# Set the chrony config to use the PHC /dev/ptp0 clock
cat > /etc/chrony.conf <<EOF
# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

systemctl restart chronyd

#EOF`)

func linuxCloudInitArtifactsInitAksCustomCloudMarinerShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsInitAksCustomCloudMarinerSh, nil
}

func linuxCloudInitArtifactsInitAksCustomCloudMarinerSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsInitAksCustomCloudMarinerShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsInitAksCustomCloudSh = []byte(`#!/bin/bash
mkdir -p /root/AzureCACertificates
# http://168.63.129.16 is a constant for the host's wireserver endpoint
certs=$(curl "http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json")
IFS_backup=$IFS
IFS=$'\r\n'
certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
for i in ${!certBodies[@]}; do
    echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${certNames[$i]} | sed 's/.cer/.crt/g')"
done
IFS=$IFS_backup

cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/
/usr/sbin/update-ca-certificates

# This copies the updated bundle to the location used by OpenSSL which is commonly used
cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem

# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
action=${1:-init}
if [ $action == "ca-refresh" ]
then
    exit
fi

(crontab -l ; echo "0 19 * * * $0 ca-refresh") | crontab -

cloud-init status --wait
repoDepotEndpoint="${REPO_DEPOT_ENDPOINT}"
sudo sed -i "s,http://.[^ ]*,$repoDepotEndpoint,g" /etc/apt/sources.list

# Disable systemd-timesyncd and install chrony and uses local time source
systemctl stop systemd-timesyncd
systemctl disable systemd-timesyncd

apt-get update
apt-get install chrony -y
cat > /etc/chrony/chrony.conf <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usuable directives.

# This will use (up to):
# - 4 sources from ntp.ubuntu.com which some are ipv6 enabled
# - 2 sources from 2.ubuntu.pool.ntp.org which is ipv6 enabled as well
# - 1 source from [01].ubuntu.pool.ntp.org each (ipv4 only atm)
# This means by default, up to 6 dual-stack and up to 2 additional IPv4-only
# sources will be used.
# At the same time it retains some protection against one of the entries being
# down (compare to just using one of the lines). See (LP: #1754358) for the
# discussion.
#
# About using servers from the NTP Pool Project in general see (LP: #104525).
# Approved by Ubuntu Technical Board on 2011-02-08.
# See http://www.pool.ntp.org/join.html for more information.
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

systemctl restart chrony

#EOF`)

func linuxCloudInitArtifactsInitAksCustomCloudShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsInitAksCustomCloudSh, nil
}

func linuxCloudInitArtifactsInitAksCustomCloudSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsInitAksCustomCloudShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/init-aks-custom-cloud.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsIpv6_nftables = []byte(`define slb_lla = fe80::1234:5678:9abc
define slb_gua = 2603:1062:0:1:fe80:1234:5678:9abc

table ip6 azureSLBProbe
flush table ip6 azureSLBProbe

table ip6 azureSLBProbe {
    chain prerouting {
        type filter hook prerouting priority -300;

        # Add a rule that accepts router discovery packets without mangling or ipv6 breaks after
        # 9000 seconds when the default route times out
        iifname eth0 icmpv6 type { nd-neighbor-solicit, nd-router-advert, nd-neighbor-advert } counter accept

        # Map packets from the LB probe LLA to a SLA IP instead
        iifname eth0 ip6 saddr $slb_lla ip6 saddr set $slb_gua counter
    }
    chain postrouting {
        type filter hook postrouting priority -300;

        # Reverse the modification on the way back out
        oifname eth0 ip6 daddr $slb_gua ip6 daddr set $slb_lla counter
    }
}
`)

func linuxCloudInitArtifactsIpv6_nftablesBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsIpv6_nftables, nil
}

func linuxCloudInitArtifactsIpv6_nftables() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsIpv6_nftablesBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ipv6_nftables", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsIpv6_nftablesService = []byte(`[Unit]
Description=Configure nftables rules for handling Azure SLB IPv6 health probe packets

[Service]
Type=simple
RemainAfterExit=true
ExecStart=/bin/bash /opt/scripts/ipv6_nftables.sh
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsIpv6_nftablesServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsIpv6_nftablesService, nil
}

func linuxCloudInitArtifactsIpv6_nftablesService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsIpv6_nftablesServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ipv6_nftables.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsIpv6_nftablesSh = []byte(`#! /bin/bash

set -uo pipefail
set -x
set -e

NFTABLES_RULESET_FILE=/etc/systemd/system/ipv6_nftables

# query IMDS to check if node has IPv6
# example interface
# [
#   {
#     "ipv4": {
#       "ipAddress": [
#         {
#           "privateIpAddress": "10.224.0.4",
#           "publicIpAddress": ""
#         }
#       ],
#       "subnet": [
#         {
#           "address": "10.224.0.0",
#           "prefix": "16"
#         }
#       ]
#     },
#     "ipv6": {
#       "ipAddress": [
#         {
#           "privateIpAddress": "fd85:534e:4cd6:ab02::5"
#         }
#       ]
#     },
#     "macAddress": "000D3A98DA20"
#   }
# ]

# check the number of IPv6 addresses this instance has from IMDS
IPV6_ADDR_COUNT=$(curl -sSL -H "Metadata: true" "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01" | \
    jq '[.[].ipv6.ipAddress[] | select(.privateIpAddress != "")] | length')

if [[ $IPV6_ADDR_COUNT -eq 0 ]];
then
    echo "instance is not configured with IPv6, skipping nftables rules"
else
    echo "writing nftables from $NFTABLES_RULESET_FILE"
    nft -f $NFTABLES_RULESET_FILE
fi
`)

func linuxCloudInitArtifactsIpv6_nftablesShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsIpv6_nftablesSh, nil
}

func linuxCloudInitArtifactsIpv6_nftablesSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsIpv6_nftablesShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ipv6_nftables.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsKmsService = []byte(`[Unit]
Description=azurekms
Requires=docker.service
After=network-online.target

[Service]
Type=simple
Restart=always
TimeoutStartSec=0
ExecStart=/usr/bin/docker run \
  --net=host \
  --volume=/opt:/opt \
  --volume=/etc/kubernetes:/etc/kubernetes \
  --volume=/etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt \
  --volume=/var/lib/waagent:/var/lib/waagent \
  mcr.microsoft.com/k8s/kms/keyvault:v0.0.9

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsKmsServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsKmsService, nil
}

func linuxCloudInitArtifactsKmsService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsKmsServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/kms.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsKubeletMonitorService = []byte(`[Unit]
Description=a script that checks kubelet health and restarts if needed
After=kubelet.service
[Service]
Restart=always
RestartSec=10
RemainAfterExit=yes
ExecStart=/usr/local/bin/health-monitor.sh kubelet`)

func linuxCloudInitArtifactsKubeletMonitorServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsKubeletMonitorService, nil
}

func linuxCloudInitArtifactsKubeletMonitorService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsKubeletMonitorServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/kubelet-monitor.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsKubeletMonitorTimer = []byte(`[Unit]
Description=a timer that delays kubelet-monitor from starting too soon after boot
[Timer]
OnBootSec=30min
[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsKubeletMonitorTimerBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsKubeletMonitorTimer, nil
}

func linuxCloudInitArtifactsKubeletMonitorTimer() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsKubeletMonitorTimerBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/kubelet-monitor.timer", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsKubeletService = []byte(`[Unit]
Description=Kubelet
ConditionPathExists=/usr/local/bin/kubelet
Wants=network-online.target containerd.service
After=network-online.target containerd.service

[Service]
Restart=always
RestartSec=2
EnvironmentFile=/etc/default/kubelet
# Graceful termination (SIGTERM)
SuccessExitStatus=143
ExecStartPre=/bin/bash /opt/azure/containers/kubelet.sh
ExecStartPre=/bin/mkdir -p /var/lib/kubelet
ExecStartPre=/bin/mkdir -p /var/lib/cni
ExecStartPre=/bin/bash -c "if [ $(mount | grep \"/var/lib/kubelet\" | wc -l) -le 0 ] ; then /bin/mount --bind /var/lib/kubelet /var/lib/kubelet ; fi"
ExecStartPre=/bin/mount --make-shared /var/lib/kubelet

ExecStartPre=-/sbin/ebtables -t nat --list
ExecStartPre=-/sbin/iptables -t nat --numeric --list

ExecStart=/usr/local/bin/kubelet \
        --enable-server \
        --node-labels="${KUBELET_NODE_LABELS}" \
        --v=2 \
        --volume-plugin-dir=/etc/kubernetes/volumeplugins \
        $KUBELET_TLS_BOOTSTRAP_FLAGS \
        $KUBELET_CONFIG_FILE_FLAGS \
        $KUBELET_CONTAINERD_FLAGS \
        $KUBELET_CONTAINER_RUNTIME_FLAG \
        $KUBELET_CGROUP_FLAGS \
        $KUBELET_FLAGS

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsKubeletServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsKubeletService, nil
}

func linuxCloudInitArtifactsKubeletService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsKubeletServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/kubelet.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsManifestJson = []byte(`{
    "containerd": {
        "fileName": "moby-containerd_${CONTAINERD_VERSION}+azure-${CONTAINERD_PATCH_VERSION}.deb",
        "downloadLocation": "/opt/containerd/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/${UBUNTU_CODENAME}/linux_${CPU_ARCH}/moby-containerd_${CONTAINERD_VERSION}+azure-ubuntu${UBUNTU_RELEASE}u${CONTAINERD_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [],
        "edge": "1.7.1-1"
    },
    "runc": {
        "fileName": "moby-runc_${RUNC_VERSION}+azure-ubuntu${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb",
        "downloadLocation": "/opt/runc/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-runc/${RUNC_VERSION}+azure/bionic/linux_${CPU_ARCH}/moby-runc_${RUNC_VERSION}+azure-ubuntu${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [],
        "installed": {
            "default": "1.1.7"
        }
    },
    "nvidia-container-runtime": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": []
    },
    "nvidia-drivers": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": []
    },
    "kubernetes": {
        "fileName": "kubernetes-node-linux-arch.tar.gz",
        "downloadLocation": "",
        "downloadURL": "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz",
        "versions": [
            "1.24.9-hotfix.20230612",
            "1.24.10-hotfix.20230612",
            "1.24.15",
            "1.25.5-hotfix.20230612",
            "1.25.6-hotfix.20230612",
            "1.25.11",
            "1.26.0-hotfix.20230612",
            "1.26.3-hotfix.20230612",
            "1.26.6",
            "1.27.1-hotfix.20230612",
            "1.27.3",
            "1.28.0"
        ]
    },
    "_template": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": []
    }
}
#EOF
`)

func linuxCloudInitArtifactsManifestJsonBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsManifestJson, nil
}

func linuxCloudInitArtifactsManifestJson() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsManifestJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/manifest.json", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMarinerCse_helpers_marinerSh = []byte(`#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Mariner"

dnfversionlockWALinuxAgent() {
    echo "No aptmark equivalent for DNF by default. If this is necessary add support for dnf versionlock plugin"
}

aptmarkWALinuxAgent() {
    echo "No aptmark equivalent for DNF by default. If this is necessary add support for dnf versionlock plugin"
}

dnf_makecache() {
    retries=10
    dnf_makecache_output=/tmp/dnf-makecache.out
    for i in $(seq 1 $retries); do
        ! (dnf makecache -y 2>&1 | tee $dnf_makecache_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_makecache_output && break || \
        cat $dnf_makecache_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed dnf makecache -y $i times
}
dnf_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        dnf install -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            dnf_makecache
        fi
    done
    echo Executed dnf install -y \"$@\" $i times;
}
dnf_remove() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        dnf remove -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed dnf remove  -y \"$@\" $i times;
}
dnf_update() {
  retries=10
  dnf_update_output=/tmp/dnf-update.out
  for i in $(seq 1 $retries); do
    ! (dnf update --exclude mshv-linuxloader --exclude kernel-mshv -y --refresh 2>&1 | tee $dnf_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
    cat $dnf_update_output && break || \
    cat $dnf_update_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed dnf update -y --refresh $i times
}
#EOF
`)

func linuxCloudInitArtifactsMarinerCse_helpers_marinerShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMarinerCse_helpers_marinerSh, nil
}

func linuxCloudInitArtifactsMarinerCse_helpers_marinerSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMarinerCse_helpers_marinerShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMarinerCse_install_marinerSh = []byte(`#!/bin/bash

echo "Sourcing cse_install_distro.sh for Mariner"

removeContainerd() {
    retrycmd_if_failure 10 5 60 dnf remove -y moby-containerd
}

installDeps() {
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in blobfuse ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse git inotify-tools iotop iproute ipset iptables jq kernel-devel logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    # install additional apparmor deps for 2.0;
    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in apparmor-parser libapparmor blobfuse2 nftables; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done
    fi
}

downloadGPUDrivers() {
    # Mariner CUDA rpm name comes in the following format:
    #
    # cuda-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # Before installing cuda, check the active kernel version (uname -r) and use that to determine which cuda to install
    KERNEL_VERSION=$(uname -r | sed 's/-/./g')
    CUDA_VERSION="*_${KERNEL_VERSION}*"

    if ! dnf_install 30 1 600 cuda-${CUDA_VERSION}; then
      exit $ERR_APT_INSTALL_TIMEOUT
    fi
}

installNvidiaFabricManager() {
    # Check the NVIDIA driver version installed and install nvidia-fabric-manager
    NVIDIA_DRIVER_VERSION=$(cut -d - -f 2 <<< "$(rpm -qa cuda)")
    for nvidia_package in nvidia-fabric-manager-${NVIDIA_DRIVER_VERSION} nvidia-fabric-manager-devel-${NVIDIA_DRIVER_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installNvidiaContainerRuntime() {
    MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION="3.11.0"
    MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION="1.11.0"
    
    for nvidia_package in nvidia-container-runtime-${MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION} nvidia-container-toolkit-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-base-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container-tools-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container1-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

enableNvidiaPersistenceMode() {
    PERSISTENCED_SERVICE_FILE_PATH="/etc/systemd/system/nvidia-persistenced.service"
    touch ${PERSISTENCED_SERVICE_FILE_PATH}
    cat << EOF > ${PERSISTENCED_SERVICE_FILE_PATH} 
[Unit]
Description=NVIDIA Persistence Daemon
Wants=syslog.target

[Service]
Type=forking
ExecStart=/usr/bin/nvidia-persistenced --verbose
ExecStopPost=/bin/rm -rf /var/run/nvidia-persistenced
Restart=always

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable nvidia-persistenced.service || exit 1
    systemctl restart nvidia-persistenced.service || exit 1
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    CONTAINERD_VERSION=$1
    #overwrite the passed containerd_version since mariner uses only 1 version now which is different than ubuntu's
    CONTAINERD_VERSION="1.3.4"
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.4.1 is our lowest supported version of containerd
    
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeContainerd
        # TODO: tie runc to r92 once that's possible on Mariner's pkg repo and if we're still using v1.linux shim
        if ! dnf_install 30 1 600 moby-containerd; then
          exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi

    # Workaround to restore the CSE configuration after containerd has been installed from the package server.
    if [[ -f /etc/containerd/config.toml.rpmsave ]]; then
        mv /etc/containerd/config.toml.rpmsave /etc/containerd/config.toml
    fi

}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

downloadContainerdFromVersion() {
    echo "downloadContainerdFromVersion not implemented for mariner"
}

downloadContainerdFromURL() {
    echo "downloadContainerdFromURL not implemented for mariner"
}

#EOF
`)

func linuxCloudInitArtifactsMarinerCse_install_marinerShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMarinerCse_install_marinerSh, nil
}

func linuxCloudInitArtifactsMarinerCse_install_marinerSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMarinerCse_install_marinerShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mariner/cse_install_mariner.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMarinerPamDSystemAuth = []byte(`# Begin /etc/pam.d/system-auth

auth      required      pam_faillock.so preauth silent audit deny=5 unlock_time=900
auth      [success=1 default=ignore]      pam_unix.so use_authtok try_first_pass
auth      [default=die] pam_faillock.so authfail audit deny=5 unlock_time=900
auth      sufficient    pam_faillock.so authsucc audit deny=5 unlock_time=900
auth      required      pam_deny.so

account   required      pam_faillock.so
account   include       system-account

password  requisite     pam_pwquality.so retry=3
password  required      pam_pwhistory.so use_authtok remember=5
password  [success=1 default=ignore]      pam_unix.so use_authtok try_first_pass sha512 audit
# here's the fallback if no module succeeds
password  requisite     pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
password  required      pam_permit.so


# End /etc/pam.d/system-auth
`)

func linuxCloudInitArtifactsMarinerPamDSystemAuthBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMarinerPamDSystemAuth, nil
}

func linuxCloudInitArtifactsMarinerPamDSystemAuth() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMarinerPamDSystemAuthBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mariner/pam-d-system-auth", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMarinerPamDSystemPassword = []byte(`# Begin /etc/pam.d/system-auth

auth      required      pam_faillock.so preauth silent audit deny=5 unlock_time=900
auth      [success=1 default=ignore]      pam_unix.so use_authtok try_first_pass
auth      [default=die] pam_faillock.so authfail audit deny=5 unlock_time=900
auth      sufficient    pam_faillock.so authsucc audit deny=5 unlock_time=900
auth      required      pam_deny.so

account   required      pam_faillock.so
account   include       system-account

password  requisite     pam_pwquality.so retry=3
password  required      pam_pwhistory.so use_authtok remember=5
password  [success=1 default=ignore]      pam_unix.so use_authtok try_first_pass sha512 audit
# here's the fallback if no module succeeds
password  requisite     pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
password  required      pam_permit.so


# End /etc/pam.d/system-auth
`)

func linuxCloudInitArtifactsMarinerPamDSystemPasswordBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMarinerPamDSystemPassword, nil
}

func linuxCloudInitArtifactsMarinerPamDSystemPassword() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMarinerPamDSystemPasswordBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mariner/pam-d-system-password", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMarinerUpdate_certs_marinerService = []byte(`[Unit]
Description=Updates certificates copied from AKS DS

[Service]
Type=oneshot
ExecStart=/opt/scripts/update_certs.sh /usr/share/pki/ca-trust-source/anchors update-ca-trust
RestartSec=5`)

func linuxCloudInitArtifactsMarinerUpdate_certs_marinerServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMarinerUpdate_certs_marinerService, nil
}

func linuxCloudInitArtifactsMarinerUpdate_certs_marinerService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMarinerUpdate_certs_marinerServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mariner/update_certs_mariner.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMigPartitionService = []byte(`[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU

[Service]
Restart=on-failure
ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh ${GPU_INSTANCE_PROFILE}

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsMigPartitionServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMigPartitionService, nil
}

func linuxCloudInitArtifactsMigPartitionService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMigPartitionServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mig-partition.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsMigPartitionSh = []byte(`#!/bin/bash

#NOTE: Currently, Nvidia library mig-parted (https://github.com/NVIDIA/mig-parted) cannot work properly because of the outdated GPU driver version
#TODO: Use mig-parted library to do the partition after the above issue is fixed 
MIG_PROFILE=${1}
case ${MIG_PROFILE} in 
    "MIG1g")
        nvidia-smi mig -cgi 19,19,19,19,19,19,19
        ;;
    "MIG2g")
        nvidia-smi mig -cgi 14,14,14
        ;;
    "MIG3g")
        nvidia-smi mig -cgi 9,9
        ;;
    "MIG4g")
        nvidia-smi mig -cgi 5
        ;;
    "MIG7g")
        nvidia-smi mig -cgi 0
        ;;  
    *)
        echo "not a valid GPU instance profile"
        exit 1
        ;;
esac
nvidia-smi mig -cci`)

func linuxCloudInitArtifactsMigPartitionShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsMigPartitionSh, nil
}

func linuxCloudInitArtifactsMigPartitionSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsMigPartitionShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/mig-partition.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsModprobeCisConf = []byte(`# 3.5.1 Ensure DCCP is disabled
install dccp /bin/true
# 3.5.2 Ensure SCTP is disabled
install sctp /bin/true
# 3.5.3 Ensure RDS is disabled
install rds /bin/true
# 3.5.4 Ensure TIPC is disabled
install tipc /bin/true
# 1.1.1.1 Ensure mounting of cramfs filesystems is disabled
# Mariner AKS CIS Benchmark: Ensure mounting of cramfs filesystems is disabled
install cramfs /bin/true
blacklist cramfs
# 1.1.1.2 Ensure mounting of freevxfs filesystems is disabled
install freevxfs /bin/true
# 1.1.1.3 Ensure mounting of jffs2 filesystems is disabled
install jffs2 /bin/true
# 1.1.1.4 Ensure mounting of hfs filesystems is disabled
install hfs /bin/true
# 1.1.1.5 Ensure mounting of hfsplus filesystems is disabled
install hfsplus /bin/true
`)

func linuxCloudInitArtifactsModprobeCisConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsModprobeCisConf, nil
}

func linuxCloudInitArtifactsModprobeCisConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsModprobeCisConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/modprobe-CIS.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsNvidiaDevicePluginService = []byte(`[Unit]
Description=Run nvidia device plugin
[Service]
RemainAfterExit=true
ExecStart=/usr/local/nvidia/bin/nvidia-device-plugin $MIG_STRATEGY
Restart=on-failure
[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsNvidiaDevicePluginServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsNvidiaDevicePluginService, nil
}

func linuxCloudInitArtifactsNvidiaDevicePluginService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsNvidiaDevicePluginServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/nvidia-device-plugin.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsNvidiaDockerDaemonJson = []byte(`{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts":  {
       "max-size": "50m",
       "max-file": "5"
    },
    "default-runtime": "nvidia",
    "runtimes": {
       "nvidia": {
           "path": "/usr/bin/nvidia-container-runtime",
           "runtimeArgs": []
      }
    }
}`)

func linuxCloudInitArtifactsNvidiaDockerDaemonJsonBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsNvidiaDockerDaemonJson, nil
}

func linuxCloudInitArtifactsNvidiaDockerDaemonJson() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsNvidiaDockerDaemonJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/nvidia-docker-daemon.json", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsNvidiaModprobeService = []byte(`[Unit]
Description=Installs and loads Nvidia GPU kernel module
[Service]
Type=oneshot
RemainAfterExit=true
ExecStartPre=/bin/sh -c "dkms autoinstall --verbose"
ExecStart=/bin/sh -c "nvidia-modprobe -u -c0"
ExecStartPost=/bin/sh -c "sleep 10 && systemctl restart kubelet"
[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsNvidiaModprobeServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsNvidiaModprobeService, nil
}

func linuxCloudInitArtifactsNvidiaModprobeService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsNvidiaModprobeServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/nvidia-modprobe.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsPamDCommonAuth = []byte(`#
# /etc/pam.d/common-auth - authentication settings common to all services
#
# This file is included from other service-specific PAM config files,
# and should contain a list of the authentication modules that define
# the central authentication scheme for use on the system
# (e.g., /etc/shadow, LDAP, Kerberos, etc.).  The default is to use the
# traditional Unix authentication mechanisms.
#
# As of pam 1.0.1-6, this file is managed by pam-auth-update by default.
# To take advantage of this, it is recommended that you configure any
# local modules either before or after the default block, and use
# pam-auth-update to manage selection of other modules.  See
# pam-auth-update(8) for details.

# here are the per-package modules (the "Primary" block)
auth	[success=1 default=ignore]	pam_unix.so nullok_secure
# here's the fallback if no module succeeds
auth	requisite			pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
auth	required			pam_permit.so
# and here are more per-package modules (the "Additional" block)
# end of pam-auth-update config

# 5.3.2 Ensure lockout for failed password attempts is configured
auth required pam_tally2.so onerr=fail audit silent deny=5 unlock_time=900
`)

func linuxCloudInitArtifactsPamDCommonAuthBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsPamDCommonAuth, nil
}

func linuxCloudInitArtifactsPamDCommonAuth() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsPamDCommonAuthBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/pam-d-common-auth", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsPamDCommonAuth2204 = []byte(`#
# /etc/pam.d/common-auth - authentication settings common to all services
#
# This file is included from other service-specific PAM config files,
# and should contain a list of the authentication modules that define
# the central authentication scheme for use on the system
# (e.g., /etc/shadow, LDAP, Kerberos, etc.).  The default is to use the
# traditional Unix authentication mechanisms.
#
# As of pam 1.0.1-6, this file is managed by pam-auth-update by default.
# To take advantage of this, it is recommended that you configure any
# local modules either before or after the default block, and use
# pam-auth-update to manage selection of other modules.  See
# pam-auth-update(8) for details.

# here are the per-package modules (the "Primary" block)
auth	[success=1 default=ignore]	pam_unix.so nullok_secure
# here's the fallback if no module succeeds
auth	requisite			pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
auth	required			pam_permit.so
# and here are more per-package modules (the "Additional" block)
# end of pam-auth-update config

# 5.3.2 Ensure lockout for failed password attempts is configured
auth required pam_faillock.so preauth silent audit deny=5 unlock_time=900
`)

func linuxCloudInitArtifactsPamDCommonAuth2204Bytes() ([]byte, error) {
	return _linuxCloudInitArtifactsPamDCommonAuth2204, nil
}

func linuxCloudInitArtifactsPamDCommonAuth2204() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsPamDCommonAuth2204Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/pam-d-common-auth-2204", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsPamDCommonPassword = []byte(`#
# /etc/pam.d/common-password - password-related modules common to all services
#
# This file is included from other service-specific PAM config files,
# and should contain a list of modules that define the services to be
# used to change user passwords.  The default is pam_unix.

# Explanation of pam_unix options:
#
# The "sha512" option enables salted SHA512 passwords.  Without this option,
# the default is Unix crypt.  Prior releases used the option "md5".
#
# The "obscure" option replaces the old `+"`"+`OBSCURE_CHECKS_ENAB' option in
# login.defs.
#
# See the pam_unix manpage for other options.

# As of pam 1.0.1-6, this file is managed by pam-auth-update by default.
# To take advantage of this, it is recommended that you configure any
# local modules either before or after the default block, and use
# pam-auth-update to manage selection of other modules.  See
# pam-auth-update(8) for details.

# here are the per-package modules (the "Primary" block)
password	requisite			pam_pwquality.so retry=3
password	[success=1 default=ignore]	pam_unix.so obscure use_authtok try_first_pass sha512
# here's the fallback if no module succeeds
password	requisite			pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
password	required			pam_permit.so
# and here are more per-package modules (the "Additional" block)
# end of pam-auth-update config

# 5.3.3 Ensure password reuse is limited
# 5.3.4 Ensure password hashing algorithm is SHA-512
password	[success=1 default=ignore]	pam_unix.so obscure use_authtok try_first_pass sha512 remember=5
`)

func linuxCloudInitArtifactsPamDCommonPasswordBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsPamDCommonPassword, nil
}

func linuxCloudInitArtifactsPamDCommonPassword() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsPamDCommonPasswordBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/pam-d-common-password", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsPamDSu = []byte(`#
# The PAM configuration file for the Shadow `+"`"+`su' service
#

# This allows root to su without passwords (normal operation)
auth       sufficient pam_rootok.so

# Uncomment this to force users to be a member of group root
# before they can use `+"`"+`su'. You can also add "group=foo"
# to the end of this line if you want to use a group other
# than the default "root" (but this may have side effect of
# denying "root" user, unless she's a member of "foo" or explicitly
# permitted earlier by e.g. "sufficient pam_rootok.so").
# (Replaces the `+"`"+`SU_WHEEL_ONLY' option from login.defs)

# 5.6 Ensure access to the su command is restricted
auth required pam_wheel.so use_uid

# Uncomment this if you want wheel members to be able to
# su without a password.
# auth       sufficient pam_wheel.so trust

# Uncomment this if you want members of a specific group to not
# be allowed to use su at all.
# auth       required   pam_wheel.so deny group=nosu

# Uncomment and edit /etc/security/time.conf if you need to set
# time restrainst on su usage.
# (Replaces the `+"`"+`PORTTIME_CHECKS_ENAB' option from login.defs
# as well as /etc/porttime)
# account    requisite  pam_time.so

# This module parses environment configuration file(s)
# and also allows you to use an extended config
# file /etc/security/pam_env.conf.
#
# parsing /etc/environment needs "readenv=1"
session       required   pam_env.so readenv=1
# locale variables are also kept into /etc/default/locale in etch
# reading this file *in addition to /etc/environment* does not hurt
session       required   pam_env.so readenv=1 envfile=/etc/default/locale

# Defines the MAIL environment variable
# However, userdel also needs MAIL_DIR and MAIL_FILE variables
# in /etc/login.defs to make sure that removing a user
# also removes the user's mail spool file.
# See comments in /etc/login.defs
#
# "nopen" stands to avoid reporting new mail when su'ing to another user
session    optional   pam_mail.so nopen

# Sets up user limits according to /etc/security/limits.conf
# (Replaces the use of /etc/limits in old login)
session    required   pam_limits.so

# The standard Unix authentication modules, used with
# NIS (man nsswitch) as well as normal /etc/passwd and
# /etc/shadow entries.
@include common-auth
@include common-account
@include common-session
`)

func linuxCloudInitArtifactsPamDSuBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsPamDSu, nil
}

func linuxCloudInitArtifactsPamDSu() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsPamDSuBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/pam-d-su", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsProfileDCisSh = []byte(`#!/bin/bash

# 5.4.4 Ensure default user umask is 027 or more restrictive
umask 027
`)

func linuxCloudInitArtifactsProfileDCisShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsProfileDCisSh, nil
}

func linuxCloudInitArtifactsProfileDCisSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsProfileDCisShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/profile-d-cis.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsPwqualityCisConf = []byte(`# 5.3.1 Ensure password creation requirements are configured (Scored)

minlen=14
dcredit=-1
ucredit=-1
ocredit=-1
lcredit=-1`)

func linuxCloudInitArtifactsPwqualityCisConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsPwqualityCisConf, nil
}

func linuxCloudInitArtifactsPwqualityCisConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsPwqualityCisConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/pwquality-CIS.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsReconcilePrivateHostsService = []byte(`[Unit]
Description=Reconcile /etc/hosts file for private cluster
[Service]
Type=simple
Restart=on-failure
ExecStart=/bin/bash /opt/azure/containers/reconcilePrivateHosts.sh
[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsReconcilePrivateHostsServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsReconcilePrivateHostsService, nil
}

func linuxCloudInitArtifactsReconcilePrivateHostsService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsReconcilePrivateHostsServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/reconcile-private-hosts.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsReconcilePrivateHostsSh = []byte(`#!/bin/bash

set -o nounset
set -o pipefail

get-apiserver-ip-from-tags() {
  tags=$(curl -sSL -H "Metadata: true" "http://169.254.169.254/metadata/instance/compute/tags?api-version=2019-03-11&format=text")
  if [ "$?" == "0" ]; then
    IFS=";" read -ra tagList <<< "$tags"
    for i in "${tagList[@]}"; do
      tagKey=$(cut -d":" -f1 <<<$i)
      tagValue=$(cut -d":" -f2 <<<$i)
      if echo $tagKey | grep -iq "^aksAPIServerIPAddress$"; then
        echo -n "$tagValue"
        return
      fi
    done
  fi
  echo -n ""
}

SLEEP_SECONDS=15
clusterFQDN="${KUBE_API_SERVER_NAME}"
if [[ $clusterFQDN != *.privatelink.* ]]; then
  echo "skip reconcile hosts for $clusterFQDN since it's not AKS private cluster"
  exit 0
fi
echo "clusterFQDN: $clusterFQDN"

while true; do
  clusterIP=$(get-apiserver-ip-from-tags)
  if [ -z $clusterIP ]; then
    sleep "${SLEEP_SECONDS}"
    continue
  fi
  if grep -q "$clusterIP $clusterFQDN" /etc/hosts; then
    echo -n ""
  else
    sudo sed -i "/$clusterFQDN/d" /etc/hosts
    echo "$clusterIP $clusterFQDN" | sudo tee -a /etc/hosts > /dev/null
    echo "Updated $clusterFQDN to $clusterIP"
  fi
  sleep "${SLEEP_SECONDS}"
done

#EOF
`)

func linuxCloudInitArtifactsReconcilePrivateHostsShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsReconcilePrivateHostsSh, nil
}

func linuxCloudInitArtifactsReconcilePrivateHostsSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsReconcilePrivateHostsShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/reconcile-private-hosts.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsRsyslogD60CisConf = []byte(`# 4.2.1.2 Ensure logging is configured (Not Scored)
*.emerg                            :omusrmsg:*
mail.*                             -/var/log/mail
mail.info                          -/var/log/mail.info
mail.warning                       -/var/log/mail.warn
mail.err                           /var/log/mail.err
news.crit                          -/var/log/news/news.crit
news.err                           -/var/log/news/news.err
news.notice                        -/var/log/news/news.notice
*.=warning;*.=err                  -/var/log/warn
*.crit                             /var/log/warn
*.*;mail.none;news.none            -/var/log/messages
local0,local1.*                    -/var/log/localmessages
local2,local3.*                    -/var/log/localmessages
local4,local5.*                    -/var/log/localmessages
local6,local7.*                    -/var/log/localmessages`)

func linuxCloudInitArtifactsRsyslogD60CisConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsRsyslogD60CisConf, nil
}

func linuxCloudInitArtifactsRsyslogD60CisConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsRsyslogD60CisConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/rsyslog-d-60-CIS.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSetupCustomSearchDomainsSh = []byte(`#!/bin/bash
set -x
source "${CSE_HELPERS_FILEPATH}"
source "${CSE_DISTRO_HELPERS_FILEPATH}"

echo "  dns-search ${CUSTOM_SEARCH_DOMAIN_NAME}" | tee -a /etc/network/interfaces.d/50-cloud-init.cfg
systemctl_restart 20 5 10 networking
wait_for_apt_locks
retrycmd_if_failure 10 5 120 apt-get -y install realmd sssd sssd-tools samba-common samba samba-common python2.7 samba-libs packagekit
wait_for_apt_locks
echo "${CUSTOM_SEARCH_REALM_PASSWORD}" | realm join -U ${CUSTOM_SEARCH_REALM_USER}@$(echo "${CUSTOM_SEARCH_DOMAIN_NAME}" | tr /a-z/ /A-Z/) $(echo "${CUSTOM_SEARCH_DOMAIN_NAME}" | tr /a-z/ /A-Z/)
`)

func linuxCloudInitArtifactsSetupCustomSearchDomainsShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSetupCustomSearchDomainsSh, nil
}

func linuxCloudInitArtifactsSetupCustomSearchDomainsSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSetupCustomSearchDomainsShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/setup-custom-search-domains.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSshd_config = []byte(`# What ports, IPs and protocols we listen for
Port 22
# Use these options to restrict which interfaces/protocols sshd will bind to
#ListenAddress ::
#ListenAddress 0.0.0.0
Protocol 2

# 5.2.11 Ensure only approved MAC algorithms are used
MACs hmac-sha2-512-etm@openssh.com,hmac-sha2-256-etm@openssh.com,umac-128-etm@openssh.com,hmac-sha2-512,hmac-sha2-256,umac-128@openssh.com
KexAlgorithms curve25519-sha256@libssh.org
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com,aes256-ctr,aes192-ctr,aes128-ctr

# 5.2.12 Ensure SSH Idle Timeout Interval is configured
ClientAliveInterval 120
ClientAliveCountMax 3

# HostKeys for protocol version 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_dsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key

# Logging
SyslogFacility AUTH
LogLevel INFO

# Authentication:
LoginGraceTime 60

# 5.2.8 Ensure SSH root login is disabled
PermitRootLogin no
# 5.2.10 Ensure SSH PermitUserEnvironment is disabled
PermitUserEnvironment no

StrictModes yes
PubkeyAuthentication yes
#AuthorizedKeysFile	%h/.ssh/authorized_keys

# Don't read the user's ~/.rhosts and ~/.shosts files
IgnoreRhosts yes
# similar for protocol version 2
HostbasedAuthentication no

# To enable empty passwords, change to yes (NOT RECOMMENDED)
PermitEmptyPasswords no

# Change to yes to enable challenge-response passwords (beware issues with
# some PAM modules and threads)
ChallengeResponseAuthentication no

# Change to no to disable tunnelled clear text passwords
PasswordAuthentication no

# 5.2.4 Ensure SSH X11 forwarding is disabled
X11Forwarding no

# 5.2.5 Ensure SSH MaxAuthTries is set to 4 or less
MaxAuthTries 4

X11DisplayOffset 10
PrintMotd no
PrintLastLog yes
TCPKeepAlive yes
#UseLogin no

#MaxStartups 10:30:60
Banner /etc/issue.net

# Allow client to pass locale environment variables
AcceptEnv LANG LC_*

Subsystem sftp /usr/lib/openssh/sftp-server

# Set this to 'yes' to enable PAM authentication, account processing,
# and session processing. If this is enabled, PAM authentication will
# be allowed through the ChallengeResponseAuthentication and
# PasswordAuthentication.  Depending on your PAM configuration,
# PAM authentication via ChallengeResponseAuthentication may bypass
# the setting of "PermitRootLogin without-password".
# If you just want the PAM account and session checks to run without
# PAM authentication, then enable this but set PasswordAuthentication
# and ChallengeResponseAuthentication to 'no'.
UsePAM yes
UseDNS no
GSSAPIAuthentication no

# Mariner AKS CIS Benchmark: Ensure SSH access is limited
DenyUsers root omsagent nxautomation
`)

func linuxCloudInitArtifactsSshd_configBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSshd_config, nil
}

func linuxCloudInitArtifactsSshd_config() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSshd_configBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sshd_config", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSshd_config_1604 = []byte(`# What ports, IPs and protocols we listen for
Port 22
# Use these options to restrict which interfaces/protocols sshd will bind to
#ListenAddress ::
#ListenAddress 0.0.0.0
Protocol 2

# 5.2.11 Ensure only approved MAC algorithms are used
MACs hmac-sha2-512-etm@openssh.com,hmac-sha2-256-etm@openssh.com,umac-128-etm@openssh.com,hmac-sha2-512,hmac-sha2-256,umac-128@openssh.com
KexAlgorithms curve25519-sha256@libssh.org
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com,aes256-ctr,aes192-ctr,aes128-ctr

# 5.2.12 Ensure SSH Idle Timeout Interval is configured
ClientAliveInterval 120
ClientAliveCountMax 3

# HostKeys for protocol version 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_dsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key

#Privilege Separation is turned on for security
UsePrivilegeSeparation yes

# Lifetime and size of ephemeral version 1 server key
KeyRegenerationInterval 3600
ServerKeyBits 1024

# Logging
SyslogFacility AUTH
LogLevel INFO

# Authentication:
LoginGraceTime 60

# 5.2.8 Ensure SSH root login is disabled
PermitRootLogin no
# 5.2.10 Ensure SSH PermitUserEnvironment is disabled
PermitUserEnvironment no

StrictModes yes
RSAAuthentication yes
PubkeyAuthentication yes
#AuthorizedKeysFile	%h/.ssh/authorized_keys

# Don't read the user's ~/.rhosts and ~/.shosts files
IgnoreRhosts yes
# For this to work you will also need host keys in /etc/ssh_known_hosts
RhostsRSAAuthentication no
# similar for protocol version 2
HostbasedAuthentication no

# To enable empty passwords, change to yes (NOT RECOMMENDED)
PermitEmptyPasswords no

# Change to yes to enable challenge-response passwords (beware issues with
# some PAM modules and threads)
ChallengeResponseAuthentication no

# Change to no to disable tunnelled clear text passwords
PasswordAuthentication no

# 5.2.4 Ensure SSH X11 forwarding is disabled
X11Forwarding no

# 5.2.5 Ensure SSH MaxAuthTries is set to 4 or less
MaxAuthTries 4

X11DisplayOffset 10
PrintMotd no
PrintLastLog yes
TCPKeepAlive yes
#UseLogin no

#MaxStartups 10:30:60
Banner /etc/issue.net

# Allow client to pass locale environment variables
AcceptEnv LANG LC_*

Subsystem sftp /usr/lib/openssh/sftp-server

# Set this to 'yes' to enable PAM authentication, account processing,
# and session processing. If this is enabled, PAM authentication will
# be allowed through the ChallengeResponseAuthentication and
# PasswordAuthentication.  Depending on your PAM configuration,
# PAM authentication via ChallengeResponseAuthentication may bypass
# the setting of "PermitRootLogin without-password".
# If you just want the PAM account and session checks to run without
# PAM authentication, then enable this but set PasswordAuthentication
# and ChallengeResponseAuthentication to 'no'.
UsePAM yes
UseDNS no
GSSAPIAuthentication no

# Mariner AKS CIS Benchmark: Ensure SSH access is limited
DenyUsers root omsagent nxautomation
`)

func linuxCloudInitArtifactsSshd_config_1604Bytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSshd_config_1604, nil
}

func linuxCloudInitArtifactsSshd_config_1604() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSshd_config_1604Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sshd_config_1604", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSshd_config_1804_fips = []byte(`#	$OpenBSD: sshd_config,v 1.101 2017/03/14 07:19:07 djm Exp $

# This is the sshd server system-wide configuration file.  See
# sshd_config(5) for more information.

# This sshd was compiled with PATH=/usr/bin:/bin:/usr/sbin:/sbin

# The strategy used for options in the default sshd_config shipped with
# OpenSSH is to specify options with their default value where
# possible, but leave them commented.  Uncommented options override the
# default value.

#Port 22
#AddressFamily any
#ListenAddress 0.0.0.0
#ListenAddress ::

#HostKey /etc/ssh/ssh_host_rsa_key
#HostKey /etc/ssh/ssh_host_ecdsa_key
#HostKey /etc/ssh/ssh_host_ed25519_key

# Ciphers and keying
#RekeyLimit default none

# Logging
#SyslogFacility AUTH
#LogLevel INFO

# Authentication:

#LoginGraceTime 2m
#PermitRootLogin prohibit-password
#StrictModes yes
#MaxAuthTries 6
#MaxSessions 10

#PubkeyAuthentication yes

# Expect .ssh/authorized_keys2 to be disregarded by default in future.
#AuthorizedKeysFile	.ssh/authorized_keys .ssh/authorized_keys2

#AuthorizedPrincipalsFile none

#AuthorizedKeysCommand none
#AuthorizedKeysCommandUser nobody

# For this to work you will also need host keys in /etc/ssh/ssh_known_hosts
#HostbasedAuthentication no
# Change to yes if you don't trust ~/.ssh/known_hosts for
# HostbasedAuthentication
#IgnoreUserKnownHosts no
# Don't read the user's ~/.rhosts and ~/.shosts files
#IgnoreRhosts yes

# To disable tunneled clear text passwords, change to no here!
PasswordAuthentication yes
#PermitEmptyPasswords no

# Change to yes to enable challenge-response passwords (beware issues with
# some PAM modules and threads)
ChallengeResponseAuthentication no

# Kerberos options
#KerberosAuthentication no
#KerberosOrLocalPasswd yes
#KerberosTicketCleanup yes
#KerberosGetAFSToken no

# GSSAPI options
#GSSAPIAuthentication no
#GSSAPICleanupCredentials yes
#GSSAPIStrictAcceptorCheck yes
#GSSAPIKeyExchange no

# Set this to 'yes' to enable PAM authentication, account processing,
# and session processing. If this is enabled, PAM authentication will
# be allowed through the ChallengeResponseAuthentication and
# PasswordAuthentication.  Depending on your PAM configuration,
# PAM authentication via ChallengeResponseAuthentication may bypass
# the setting of "PermitRootLogin without-password".
# If you just want the PAM account and session checks to run without
# PAM authentication, then enable this but set PasswordAuthentication
# and ChallengeResponseAuthentication to 'no'.
UsePAM yes

#AllowAgentForwarding yes
#AllowTcpForwarding yes
#GatewayPorts no
X11Forwarding yes
#X11DisplayOffset 10
#X11UseLocalhost yes
#PermitTTY yes
PrintMotd no
#PrintLastLog yes
#TCPKeepAlive yes
#UseLogin no
#PermitUserEnvironment no
#Compression delayed
#ClientAliveInterval 0
#ClientAliveCountMax 3
#UseDNS no
#PidFile /var/run/sshd.pid
#MaxStartups 10:30:100
#PermitTunnel no
#ChrootDirectory none
#VersionAddendum none

# no default banner path
#Banner none

# Allow client to pass locale environment variables
AcceptEnv LANG LC_*

# override default of no subsystems
Subsystem sftp	/usr/lib/openssh/sftp-server

# Example of overriding settings on a per-user basis
#Match User anoncvs
#	X11Forwarding no
#	AllowTcpForwarding no
#	PermitTTY no
#	ForceCommand cvs server

# CLOUD_IMG: This file was created/modified by the Cloud Image build process
ClientAliveInterval 120

# Mariner AKS CIS Benchmark: Ensure SSH access is limited
DenyUsers root omsagent nxautomation
`)

func linuxCloudInitArtifactsSshd_config_1804_fipsBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSshd_config_1804_fips, nil
}

func linuxCloudInitArtifactsSshd_config_1804_fips() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSshd_config_1804_fipsBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sshd_config_1804_fips", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSyncContainerLogsService = []byte(`[Unit]
Description=Syncs AKS pod log symlinks so that WALinuxAgent can include kube-system pod logs in the hourly upload.
After=containerd.service

[Service]
ExecStart=/opt/azure/containers/sync-container-logs.sh
Restart=always

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsSyncContainerLogsServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSyncContainerLogsService, nil
}

func linuxCloudInitArtifactsSyncContainerLogsService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSyncContainerLogsServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sync-container-logs.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSyncContainerLogsSh = []byte(`#! /bin/bash

SRC=/var/log/containers
DST=/var/log/azure/aks/pods

# Bring in OS-related bash vars
source /etc/os-release

# Install inotify-tools if they're missing from the image
if [[ ${ID} == "mariner" ]]; then
  command -v inotifywait >/dev/null 2>&1 || dnf install -y inotify-tools
else 
  command -v inotifywait >/dev/null 2>&1 || apt-get -o DPkg::Lock::Timeout=300 -y install inotify-tools
fi

# Set globbing options so that compgen grabs only the logs we want
shopt -s extglob
shopt -s nullglob

# Wait for /var/log/containers to exist
if [ ! -d $SRC ]; then
  echo -n "Waiting for $SRC to exist..."
  while [ ! -d $SRC ]; do
    sleep 15
    echo -n "."
  done
  echo "done."
fi

# Make the destination directory if not already present
mkdir -p $DST

# Start a background process to clean up logs from deleted pods that
# haven't been modified in 2 hours. This allows us to retain pod
# logs after a restart.
while true; do
  find /var/log/azure/aks/pods -type f -links 1 -mmin +120 -delete
  sleep 3600
done &

# Manually sync all matching logs once
for CONTAINER_LOG_FILE in $(compgen -G "$SRC/*_kube-system_*.log"); do
   echo "Linking $CONTAINER_LOG_FILE"
   /bin/ln -Lf $CONTAINER_LOG_FILE $DST/
done
echo "Starting inotifywait..."

# Monitor for changes
inotifywait -q -m -r -e delete,create $SRC | while read DIRECTORY EVENT FILE; do
    case $FILE in
        *_kube-system_*.log)
            case $EVENT in
                CREATE*)
                    echo "Linking $FILE"
                    /bin/ln -Lf "$DIRECTORY/$FILE" "$DST/$FILE"
                    ;;
            esac;;
    esac
done
`)

func linuxCloudInitArtifactsSyncContainerLogsShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSyncContainerLogsSh, nil
}

func linuxCloudInitArtifactsSyncContainerLogsSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSyncContainerLogsShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sync-container-logs.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsSysctlD60CisConf = []byte(`# Ubuntu CIS Benchmark: Ensure packet redirect sending is disabled
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0

# Ubuntu CIS Benchmark: Ensure source routed packets are not accepted
# Mariner AKS CIS Benchmark: Ensure source routed packets are not accepted
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv6.conf.all.accept_source_route = 0
net.ipv6.conf.default.accept_source_route = 0

# Ubuntu CIS Benchmark: Ensure ICMP redirects are not accepted
# Mariner AKS CIS Benchmark: Ensure ICMP redirects are not accepted
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0

# Ubuntu CIS Benchmark: Ensure secure ICMP redirects are not accepted
# Mariner AKS CIS Benchmark: Ensure secure ICMP redirects are not accepted
net.ipv4.conf.all.secure_redirects = 0
net.ipv4.conf.default.secure_redirects = 0

# Ubuntu CIS Benchmark: Ensure suspicious packets are logged
# Mariner AKS CIS Benchmark: Ensure suspicious packets are logged
net.ipv4.conf.all.log_martians = 1
net.ipv4.conf.default.log_martians = 1

# Ubuntu CIS Benchmark: Ensure IPv6 router advertisements are not accepted
# Mariner AKS CIS Benchmark: Ensure IPv6 router advertisements are not accepted
net.ipv6.conf.all.accept_ra = 0
net.ipv6.conf.default.accept_ra = 0

# Mariner AKS CIS Benchmark: Ensure broadcast ICMP requests are ignored
net.ipv4.icmp_echo_ignore_broadcasts = 1

# Mariner AKS CIS Benchmark: Ensure bogus ICMP responses are ignored
net.ipv4.icmp_ignore_bogus_error_responses = 1

# Mariner AKS CIS Benchmark: Ensure Reverse Path Filtering is enabled
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1

# Mariner AKS CIS Benchmark: Ensure TCP SYN Cookies is enabled
net.ipv4.tcp_syncookies = 1

# refer to https://github.com/kubernetes/kubernetes/blob/75d45bdfc9eeda15fb550e00da662c12d7d37985/pkg/kubelet/cm/container_manager_linux.go#L359-L397
vm.overcommit_memory = 1
kernel.panic = 10
kernel.panic_on_oops = 1
# to ensure node stability, we set this to the PID_MAX_LIMIT on 64-bit systems: refer to https://kubernetes.io/docs/concepts/policy/pid-limiting/
kernel.pid_max = 4194304
# https://github.com/Azure/AKS/issues/772
fs.inotify.max_user_watches = 1048576
# Ubuntu 22.04 has inotify_max_user_instances set to 128, where as Ubuntu 18.04 had 1024.
fs.inotify.max_user_instances = 1024
`)

func linuxCloudInitArtifactsSysctlD60CisConfBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSysctlD60CisConf, nil
}

func linuxCloudInitArtifactsSysctlD60CisConf() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSysctlD60CisConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sysctl-d-60-CIS.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsTeleportdService = []byte(`[Unit]
Description=teleportd teleport runtime
After=network.target
[Service]
ExecStart=/usr/local/bin/teleportd --metrics --aksConfig /etc/kubernetes/azure.json
Delegate=yes
KillMode=process
Restart=always
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=1048576
TasksMax=infinity
[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsTeleportdServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsTeleportdService, nil
}

func linuxCloudInitArtifactsTeleportdService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsTeleportdServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/teleportd.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh = []byte(`#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Ubuntu"


aptmarkWALinuxAgent() {
    echo $(date),$(hostname), startAptmarkWALinuxAgent "$1"
    wait_for_apt_locks
    retrycmd_if_failure 120 5 25 apt-mark $1 walinuxagent || \
    if [[ "$1" == "hold" ]]; then
        exit $ERR_HOLD_WALINUXAGENT
    elif [[ "$1" == "unhold" ]]; then
        exit $ERR_RELEASE_HOLD_WALINUXAGENT
    fi
    echo $(date),$(hostname), endAptmarkWALinuxAgent "$1"
}

wait_for_apt_locks() {
    while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock >/dev/null 2>&1; do
        echo 'Waiting for release of apt locks'
        sleep 3
    done
}
apt_get_update() {
    retries=10
    apt_update_output=/tmp/apt-get-update.out
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        apt-get -f -y install
        ! (apt-get update 2>&1 | tee $apt_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $apt_update_output && break || \
        cat $apt_update_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed apt-get update $i times
    wait_for_apt_locks
}
apt_get_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        apt-get install -o Dpkg::Options::="--force-confold" --no-install-recommends -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            apt_get_update
        fi
    done
    echo Executed apt-get install --no-install-recommends -y \"$@\" $i times;
    wait_for_apt_locks
}
apt_get_purge() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        timeout $timeout apt-get purge -o Dpkg::Options::="--force-confold" -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed apt-get purge -y \"$@\" $i times;
    wait_for_apt_locks
}
apt_get_dist_upgrade() {
  retries=10
  apt_dist_upgrade_output=/tmp/apt-get-dist-upgrade.out
  for i in $(seq 1 $retries); do
    wait_for_apt_locks
    export DEBIAN_FRONTEND=noninteractive
    dpkg --configure -a --force-confdef
    apt-get -f -y install
    apt-mark showhold
    ! (apt-get -o Dpkg::Options::="--force-confnew" dist-upgrade -y 2>&1 | tee $apt_dist_upgrade_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
    cat $apt_dist_upgrade_output && break || \
    cat $apt_dist_upgrade_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed apt-get dist-upgrade $i times
  wait_for_apt_locks
}
installDebPackageFromFile() {
    DEB_FILE=$1
    wait_for_apt_locks
    retrycmd_if_failure 10 5 600 apt-get -y -f install ${DEB_FILE} --allow-downgrades
    if [[ $? -ne 0 ]]; then
        return 1
    fi
}
#EOF
`)

func linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh, nil
}

func linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh = []byte(`#!/bin/bash

echo "Sourcing cse_install_distro.sh for Ubuntu"

removeMoby() {
    apt_get_purge 10 5 300 moby-engine moby-cli
}

removeContainerd() {
    apt_get_purge 10 5 300 moby-containerd
}

installDeps() {
    if [[ $(isARM64) == 1 ]]; then
        wait_for_apt_locks
        retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    else
        retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    fi
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL

    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    pkg_list=(apt-transport-https ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool git glusterfs-client htop iftop init-system-helpers inotify-tools iotop iproute2 ipset iptables nftables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat traceroute util-linux xz-utils netcat dnsutils zip rng-tools kmod gcc make dkms initramfs-tools linux-headers-$(uname -r) linux-modules-extra-$(uname -r))

    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    BLOBFUSE_VERSION="1.4.5"
    BLOBFUSE2_VERSION="2.0.5"

    if [ "${OSVERSION}" == "16.04" ]; then
        BLOBFUSE_VERSION="1.3.7"
    fi

    if [[ $(isARM64) != 1 ]]; then
        # blobfuse is not present in ARM64
        # blobfuse2 is installed for all ubuntu versions, it is included in pkg_list
        # for 22.04, fuse3 is installed. for all others, fuse is installed
        # for 16.04, installed blobfuse1.3.7, for all others except 22.04, installed blobfuse1.4.5
        pkg_list+=(blobfuse2=${BLOBFUSE2_VERSION})
        if [[ "${OSVERSION}" == "22.04" ]]; then
            pkg_list+=(fuse3)
        else
            pkg_list+=(blobfuse=${BLOBFUSE_VERSION} fuse)
        fi
    fi

    for apt_package in ${pkg_list[*]}; do
        if ! apt_get_install 30 1 600 $apt_package; then
            journalctl --no-pager -u $apt_package
            exit $ERR_APT_INSTALL_TIMEOUT
        fi
    done
}

updateAptWithMicrosoftPkg() {
    if [[ $(isARM64) == 1 ]]; then
        if [ "${UBUNTU_RELEASE}" == "22.04" ]; then
            retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
        else
            retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/multiarch/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
        fi
    else
        retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    fi

    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/18.04/multiarch/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    elif [[ ${UBUNTU_RELEASE} == "20.04" || ${UBUNTU_RELEASE} == "22.04" ]]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/${UBUNTU_RELEASE}/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    fi
    
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    UBUNTU_RELEASE=$(lsb_release -r -s)
    UBUNTU_CODENAME=$(lsb_release -c -s)
    CONTAINERD_VERSION=$1    
    # we always default to the .1 patch versons
    CONTAINERD_PATCH_VERSION="${2:-1}"

    # runc needs to be installed first or else existing vhd version causes conflict with containerd.
    logs_to_events "AKS.CSE.installContainerRuntime.ensureRunc" "ensureRunc ${RUNC_VERSION:-""}" # RUNC_VERSION is an optional override supplied via NodeBootstrappingConfig api

    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    CURRENT_COMMIT=$(containerd -version | cut -d " " -f 4)
    # v1.4.1 is our lowest supported version of containerd

    if [ -z "$CURRENT_VERSION" ]; then
        CURRENT_VERSION="0.0.0"
    fi

    # the user-defined package URL is always picked first, and the other options won't be tried when this one fails
    CONTAINERD_PACKAGE_URL="${CONTAINERD_PACKAGE_URL:=}"
    if [[ ! -z ${CONTAINERD_PACKAGE_URL} ]]; then
        echo "Installing containerd from user input: ${CONTAINERD_PACKAGE_URL}"
        # we'll use a user-defined containerd package to install containerd even though it's the same version as
        # the one already installed on the node considering the source is built by the user for hotfix or test
        logs_to_events "AKS.CSE.installContainerRuntime.removeMoby" removeMoby
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromURL" downloadContainerdFromURL ${CONTAINERD_PACKAGE_URL}
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        echo "Succeeded to install containerd from user input: ${CONTAINERD_PACKAGE_URL}"
        return 0
    fi

    #if there is no containerd_version input from RP, use hardcoded version
    if [[ -z ${CONTAINERD_VERSION} ]]; then
        CONTAINERD_VERSION="1.7.1"
        CONTAINERD_PATCH_VERSION="1"
        echo "Containerd Version not specified, using default version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    else
        echo "Using specified Containerd Version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    fi

    CURRENT_MAJOR_MINOR="$(echo $CURRENT_VERSION | tr '.' '\n' | head -n 2 | paste -sd.)"
    DESIRED_MAJOR_MINOR="$(echo $CONTAINERD_VERSION | tr '.' '\n' | head -n 2 | paste -sd.)"
    semverCompare "$CURRENT_VERSION" "$CONTAINERD_VERSION"
    HAS_GREATER_VERSION="$?"

    if [[ "$HAS_GREATER_VERSION" == "0" ]] && [[ "$CURRENT_MAJOR_MINOR" == "$DESIRED_MAJOR_MINOR" ]]; then
        echo "currently installed containerd version ${CURRENT_VERSION} matches major.minor with higher patch ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        logs_to_events "AKS.CSE.installContainerRuntime.removeMoby" removeMoby
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
        # if containerd version has been overriden then there should exist a local .deb file for it on aks VHDs (best-effort)
        # if no files found then try fetching from packages.microsoft repo
        CONTAINERD_DEB_FILE="$(ls ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_${CONTAINERD_VERSION}*)"
        if [[ -f "${CONTAINERD_DEB_FILE}" ]]; then
            logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
            return 0
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromVersion" "downloadContainerdFromVersion ${CONTAINERD_VERSION} ${CONTAINERD_PATCH_VERSION}"
        CONTAINERD_DEB_FILE="$(ls ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_${CONTAINERD_VERSION}*)"
        if [[ -z "${CONTAINERD_DEB_FILE}" ]]; then
            echo "Failed to locate cached containerd deb"
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
}

downloadContainerdFromVersion() {
    CONTAINERD_VERSION=$1
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    # Adding updateAptWithMicrosoftPkg since AB e2e uses an older image version with uncached containerd 1.6 so it needs to download from testing repo.
    # And RP no image pull e2e has apt update restrictions that prevent calls to packages.microsoft.com in CSE
    # This won't be called for new VHDs as they have containerd 1.6 cached
    updateAptWithMicrosoftPkg 
    apt_get_download 20 30 moby-containerd=${CONTAINERD_VERSION}* || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    cp -al ${APT_CACHE_DIR}moby-containerd_${CONTAINERD_VERSION}* $CONTAINERD_DOWNLOADS_DIR/ || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
}

downloadContainerdFromURL() {
    CONTAINERD_DOWNLOAD_URL=$1
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}

installMoby() {
    ensureRunc ${RUNC_VERSION:-""} # RUNC_VERSION is an optional override supplied via NodeBootstrappingConfig api
    CURRENT_VERSION=$(dockerd --version | grep "Docker version" | cut -d "," -f 1 | cut -d " " -f 3 | cut -d "+" -f 1)
    local MOBY_VERSION="19.03.14"
    local MOBY_CONTAINERD_VERSION="1.4.13"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${MOBY_VERSION}; then
        echo "currently installed moby-docker version ${CURRENT_VERSION} is greater than (or equal to) target base version ${MOBY_VERSION}. skipping installMoby."
    else
        removeMoby
        updateAptWithMicrosoftPkg
        MOBY_CLI=${MOBY_VERSION}
        if [[ "${MOBY_CLI}" == "3.0.4" ]]; then
            MOBY_CLI="3.0.3"
        fi
        apt_get_install 20 30 120 moby-engine=${MOBY_VERSION}* moby-cli=${MOBY_CLI}* moby-containerd=${MOBY_CONTAINERD_VERSION}* --allow-downgrades || exit $ERR_MOBY_INSTALL_TIMEOUT
    fi
}

ensureRunc() {
    RUNC_PACKAGE_URL="${RUNC_PACKAGE_URL:=}"
    # the user-defined runc package URL is always picked first, and the other options won't be tried when this one fails
    if [[ ! -z ${RUNC_PACKAGE_URL} ]]; then
        echo "Installing runc from user input: ${RUNC_PACKAGE_URL}"
        mkdir -p $RUNC_DOWNLOADS_DIR
        RUNC_DEB_TMP=${RUNC_PACKAGE_URL##*/}
        RUNC_DEB_FILE="$RUNC_DOWNLOADS_DIR/${RUNC_DEB_TMP}"
        retrycmd_curl_file 120 5 60 ${RUNC_DEB_FILE} ${RUNC_PACKAGE_URL} || exit $ERR_RUNC_DOWNLOAD_TIMEOUT
        # we'll use a user-defined containerd package to install containerd even though it's the same version as
        # the one already installed on the node considering the source is built by the user for hotfix or test
        installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
        echo "Succeeded to install runc from user input: ${RUNC_PACKAGE_URL}"
        return 0
    fi

    TARGET_VERSION=${1:-""}
    if [[ -z ${TARGET_VERSION} ]]; then
        TARGET_VERSION="1.1.7+azure-ubuntu${UBUNTU_RELEASE}"
    fi

    if [[ $(isARM64) == 1 ]]; then
        if [[ ${TARGET_VERSION} == "1.0.0-rc92" || ${TARGET_VERSION} == "1.0.0-rc95" ]]; then
            # only moby-runc-1.0.3+azure-1 exists in ARM64 ubuntu repo now, no 1.0.0-rc92 or 1.0.0-rc95
            return
        fi
    fi

    CPU_ARCH=$(getCPUArch)  #amd64 or arm64
    CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
    CLEANED_TARGET_VERSION=${TARGET_VERSION%+*} # removes the +azure-ubuntu18.04u1 (or similar) suffix

    if [ "${CURRENT_VERSION}" == "${CLEANED_TARGET_VERSION}" ]; then
        echo "target moby-runc version ${CLEANED_TARGET_VERSION} is already installed. skipping installRunc."
        return
    fi
    # if on a vhd-built image, first check if we've cached the deb file
    if [ -f $VHD_LOGS_FILEPATH ]; then
        RUNC_DEB_PATTERN="moby-runc_*.deb"
        RUNC_DEB_FILE=$(find ${RUNC_DOWNLOADS_DIR} -type f -iname "${RUNC_DEB_PATTERN}" | sort -V | tail -n1)
        if [[ -f "${RUNC_DEB_FILE}" ]]; then
            installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
            return 0
        fi
    fi
    apt_get_install 20 30 120 moby-runc=${TARGET_VERSION}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
}

#EOF
`)

func linuxCloudInitArtifactsUbuntuCse_install_ubuntuShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh, nil
}

func linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsUbuntuCse_install_ubuntuShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsUpdate_certsPath = []byte(`[Unit]
Description=Monitor the cert directory for changes

[Path]
PathModified=/opt/certs
Unit=update_certs.service

[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsUpdate_certsPathBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsUpdate_certsPath, nil
}

func linuxCloudInitArtifactsUpdate_certsPath() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsUpdate_certsPathBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/update_certs.path", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsUpdate_certsService = []byte(`[Unit]
Description=Updates certificates copied from AKS DS

[Service]
Type=oneshot
ExecStart=/opt/scripts/update_certs.sh
RestartSec=5`)

func linuxCloudInitArtifactsUpdate_certsServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsUpdate_certsService, nil
}

func linuxCloudInitArtifactsUpdate_certsService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsUpdate_certsServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/update_certs.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsUpdate_certsSh = []byte(`#!/usr/bin/env bash
set -uo pipefail

certSource=/opt/certs
certDestination="${1:-/usr/local/share/ca-certificates/certs}"
updateCmd="${2:-update-ca-certificates -f}"
destPrefix="aks-custom-"

[ ! -d "$certDestination" ] && mkdir "$certDestination"
for file in "$certSource"/*; do
  [ -f "$file" ] || continue
  cp -a -- "$file" "$certDestination/$destPrefix${file##*/}"
done

if [[ -z $(ls -A "$certSource") ]]; then
  echo "Source dir "$certSource" was empty, attempting to remove cert files"
  ls "$certDestination" | grep -E '^'$destPrefix'[0-9]{14}' | while read -r line; do
    echo "removing "$line" in "$certDestination""
    rm $certDestination/"$line"
  done
else
  echo "found cert files in "$certSource""
  certsToCopy=(${certSource}/*)
  currIterationCertFile=${certsToCopy[0]##*/}
  currIterationTag=${currIterationCertFile:0:14}
  for file in "$certDestination/$destPrefix"*.crt; do
     currFile=${file##*/}
     if [[ "${currFile:${#destPrefix}:14}" != "${currIterationTag}" && -f "${file}" ]]; then
          echo "removing "$file" in "$certDestination""
          rm "${file}"
     fi
  done
fi

$updateCmd`)

func linuxCloudInitArtifactsUpdate_certsShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsUpdate_certsSh, nil
}

func linuxCloudInitArtifactsUpdate_certsSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsUpdate_certsShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/update_certs.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitNodecustomdataYml = []byte(`#cloud-config

write_files:
- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}


{{if IsMariner}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceMariner"}}
{{- else}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceUbuntu"}}
{{end}}

{{ if not IsCustomImage -}}
- path: /opt/azure/containers/provision_start.sh
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionStartScript"}}
{{- end }}

- path: /opt/azure/containers/provision.sh
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionScript"}}

- path: {{GetCSEInstallScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionInstalls"}}

- path: /opt/azure/containers/provision_redact_cloud_config.py
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionRedactCloudConfig"}}

- path: /opt/azure/containers/provision_send_logs.py
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSendLogs"}}

{{if IsMariner}}
- path: {{GetCSEInstallScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionInstallsMariner"}}
{{- else}}
- path: {{GetCSEInstallScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionInstallsUbuntu"}}
{{end}}

- path: {{GetCSEConfigScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionConfigs"}}

- path: /opt/azure/manifest.json
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "componentManifestFile"}}

- path: {{GetInitAKSCustomCloudFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "initAKSCustomCloud"}}

- path: /opt/azure/containers/reconcilePrivateHosts.sh
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "reconcilePrivateHostsScript"}}

- path: /etc/systemd/system/reconcile-private-hosts.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "reconcilePrivateHostsService"}}

- path: /etc/systemd/system/kubelet.service
  permissions: "0600"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "kubeletSystemdService"}}

- path: /etc/systemd/system/mig-partition.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "migPartitionSystemdService"}}

- path: /opt/azure/containers/mig-partition.sh
  permissions: "0544"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "migPartitionScript"}}

- path: /opt/azure/containers/bind-mount.sh
  permissions: "0544"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "bindMountScript"}}

- path: /etc/systemd/system/bind-mount.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "bindMountSystemdService"}}

- path: {{GetDHCPv6ServiceCSEScriptFilepath}}
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dhcpv6SystemdService"}}

- path: /opt/azure/containers/enable-dhcpv6.sh
  permissions: "0544"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dhcpv6ConfigurationScript"}}

- path: /etc/systemd/system/docker.service.d/exec_start.conf
  permissions: "0644"
  owner: root
  content: |
    [Service]
    ExecStart=
    ExecStart=/usr/bin/dockerd -H fd:// --storage-driver=overlay2 --bip={{GetParameter "dockerBridgeCidr"}}
    ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
    #EOF

- path: /etc/docker/daemon.json
  permissions: "0644"
  owner: root
  content: |
    {
      "live-restore": true,
      "log-driver": "json-file",
      "log-opts":  {
         "max-size": "50m",
         "max-file": "5"
      }{{if IsNSeriesSKU}}
      ,"default-runtime": "nvidia",
      "runtimes": {
         "nvidia": {
             "path": "/usr/bin/nvidia-container-runtime",
             "runtimeArgs": []
        }
      }{{end}}{{if HasDataDir}},
      "data-root": "{{GetDataDir}}"{{- end}}
    }

- path: /etc/systemd/system/containerd.service.d/exec_start.conf
  permissions: "0644"
  owner: root
  content: |
    [Service]
    ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
    #EOF

- path: /etc/crictl.yaml
  permissions: "0644"
  owner: root
  content: |
    runtime-endpoint: unix:///run/containerd/containerd.sock
    #EOF

- path: /etc/systemd/system/ensure-no-dup.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "ensureNoDupEbtablesService"}}

- path: /opt/azure/containers/ensure-no-dup.sh
  permissions: "0755"
  owner: root
  encoding: gzip
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "ensureNoDupEbtablesScript"}}

- path: /etc/systemd/system/teleportd.service
  permissions: "0644"
  owner: root
  content: |
    [Unit]
    Description=teleportd teleport runtime
    After=network.target
    [Service]
    ExecStart=/usr/local/bin/teleportd --metrics --aksConfig /etc/kubernetes/azure.json
    Delegate=yes
    KillMode=process
    Restart=always
    LimitNPROC=infinity
    LimitCORE=infinity
    LimitNOFILE=1048576
    TasksMax=infinity
    [Install]
    WantedBy=multi-user.target
    #EOF

- path: /etc/systemd/system/nvidia-modprobe.service
  permissions: "0644"
  owner: root
  content: |
    [Unit]
    Description=Installs and loads Nvidia GPU kernel module
    [Service]
    Type=oneshot
    RemainAfterExit=true
    ExecStartPre=/bin/sh -c "dkms autoinstall --verbose"
    ExecStart=/bin/sh -c "nvidia-modprobe -u -c0"
    ExecStartPost=/bin/sh -c "sleep 10 && systemctl restart kubelet"
    [Install]
    WantedBy=multi-user.target

- path: /etc/default/kubelet
  permissions: "0644"
  owner: root
  content: |
    KUBELET_FLAGS={{GetKubeletConfigKeyVals}}
    KUBELET_REGISTER_SCHEDULABLE=true
    NETWORK_POLICY={{GetParameter "networkPolicy"}}
{{- if not (IsKubernetesVersionGe "1.17.0")}}
    KUBELET_IMAGE={{GetHyperkubeImageReference}}
{{- end}}
{{- if IsKubernetesVersionGe "1.16.0"}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabels . }}
{{- else}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabelsDeprecated . }}
{{- end}}
{{- if IsAKSCustomCloud}}
    AZURE_ENVIRONMENT_FILEPATH=/etc/kubernetes/{{GetTargetEnvironment}}.json
{{- end}}

{{ if IsKubeletClientTLSBootstrappingEnabled -}}
- path: /var/lib/kubelet/bootstrap-kubeconfig
  permissions: "0644"
  owner: root
  content: |
    apiVersion: v1
    kind: Config
    clusters:
    - name: localcluster
      cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://{{GetKubernetesEndpoint}}:443
    users:
    - name: kubelet-bootstrap
      user:
        token: "{{GetTLSBootstrapTokenForKubeConfig}}"
    contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
    current-context: bootstrap-context
{{else -}}
- path: /var/lib/kubelet/kubeconfig
  permissions: "0644"
  owner: root
  content: |
    apiVersion: v1
    kind: Config
    clusters:
    - name: localcluster
      cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://{{GetKubernetesEndpoint}}:443
    users:
    - name: client
      user:
        client-certificate: /etc/kubernetes/certs/client.crt
        client-key: /etc/kubernetes/certs/client.key
    contexts:
    - context:
        cluster: localcluster
        user: client
      name: localclustercontext
    current-context: localclustercontext
{{- end}}

- path: /opt/azure/containers/kubelet.sh
  permissions: "0755"
  owner: root
  content: |
    #!/bin/bash
    # Disallow container from reaching out to the special IP address 168.63.129.16
    # for TCP protocol (which http uses)
    #
    # 168.63.129.16 contains protected settings that have priviledged info.
    #
    # The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
    #
    # Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
    # for DNS.
    iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP

- path: /etc/kubernetes/certs/ca.crt
  permissions: "0600"
  encoding: base64
  owner: root
  content: |
    {{GetParameter "caCertificate"}}

- path: {{GetCustomSearchDomainsCSEScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "customSearchDomainsScript"}}`)

func linuxCloudInitNodecustomdataYmlBytes() ([]byte, error) {
	return _linuxCloudInitNodecustomdataYml, nil
}

func linuxCloudInitNodecustomdataYml() (*asset, error) {
	bytes, err := linuxCloudInitNodecustomdataYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/nodecustomdata.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsCsecmdPs1 = []byte(`powershell.exe -ExecutionPolicy Unrestricted -command \"
$arguments = '
-MasterIP ''{{ GetKubernetesEndpoint }}''
-KubeDnsServiceIp ''{{ GetParameter "kubeDNSServiceIP" }}''
-MasterFQDNPrefix ''{{ GetParameter "masterEndpointDNSNamePrefix" }}''
-Location ''{{ GetVariable "location" }}''
{{if UserAssignedIDEnabled}}
-UserAssignedClientID ''{{ GetVariable "userAssignedIdentityID" }}''
{{ end }}
-TargetEnvironment ''{{ GetTargetEnvironment }}''
-AgentKey ''{{ GetParameter "clientPrivateKey" }}''
-AADClientId ''{{ GetParameter "servicePrincipalClientId" }}''
-AADClientSecret ''{{ GetParameter "encodedServicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01
-LogFile %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log
-CSEResultFilePath %SYSTEMDRIVE%\AzureData\CSEResult.log';
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
if (!(Test-Path $inputFile)) { echo 49 | Out-File -FilePath '%SYSTEMDRIVE%\AzureData\CSEResult.log' -Encoding utf8; exit; };
Copy-Item $inputFile $outputFile;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\" >> %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1; if (!(Test-Path %SYSTEMDRIVE%\AzureData\CSEResult.log)) { exit 50; }; $code=(Get-Content %SYSTEMDRIVE%\AzureData\CSEResult.log); exit $code`)

func windowsCsecmdPs1Bytes() ([]byte, error) {
	return _windowsCsecmdPs1, nil
}

func windowsCsecmdPs1() (*asset, error) {
	bytes, err := windowsCsecmdPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/csecmd.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsKuberneteswindowssetupPs1 = []byte(`<#
    .SYNOPSIS
        Provisions VM as a Kubernetes agent.

    .DESCRIPTION
        Provisions VM as a Kubernetes agent.

        The parameters passed in are required, and will vary per-deployment.

        Notes on modifying this file:
        - This file extension is PS1, but it is actually used as a template from pkg/engine/template_generator.go
        - All of the lines that have braces in them will be modified. Please do not change them here, change them in the Go sources
        - Single quotes are forbidden, they are reserved to delineate the different members for the ARM template concat() call
        - windowscsehelper.ps1 contains basic util functions. It will be compressed to a zip file and then be converted to base64 encoding
          string and stored in $zippedFiles. Reason: This script is a template and has some limitations.
        - All other scripts will be packaged and published in a single package. It will be downloaded in provisioning VM.
          Reason: CustomData has length limitation 87380.
        - ProvisioningScriptsPackage contains scripts to start kubelet, kubeproxy, etc. The source is https://github.com/Azure/aks-engine/tree/master/staging/provisioning/windows
#>
[CmdletBinding(DefaultParameterSetName="Standard")]
param(
    [string]
    [ValidateNotNullOrEmpty()]
    $MasterIP,

    [parameter()]
    [ValidateNotNullOrEmpty()]
    $KubeDnsServiceIp,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $MasterFQDNPrefix,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $Location,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $AgentKey,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $AADClientId,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $AADClientSecret, # base64

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $NetworkAPIVersion,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $TargetEnvironment,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $LogFile,

    [parameter(Mandatory=$true)]
    [ValidateNotNullOrEmpty()]
    $CSEResultFilePath,

    [string]
    $UserAssignedClientID
)
# Do not parse the start time from $LogFile to simplify the logic
$StartTime=Get-Date
$global:ExitCode=0
$global:ErrorMessage=""

# These globals will not change between nodes in the same cluster, so they are not
# passed as powershell parameters

## SSH public keys to add to authorized_keys
$global:SSHKeys = @( {{ GetSshPublicKeysPowerShell }} )

## Certificates generated by aks-engine
$global:CACertificate = "{{GetParameter "caCertificate"}}"
$global:AgentCertificate = "{{GetParameter "clientCertificate"}}"

## Download sources provided by aks-engine
$global:KubeBinariesPackageSASURL = "{{GetParameter "kubeBinariesSASURL"}}"
$global:WindowsKubeBinariesURL = "{{GetParameter "windowsKubeBinariesURL"}}"
$global:KubeBinariesVersion = "{{GetParameter "kubeBinariesVersion"}}"
$global:ContainerdUrl = "{{GetParameter "windowsContainerdURL"}}"
$global:ContainerdSdnPluginUrl = "{{GetParameter "windowsSdnPluginURL"}}"

## Docker Version
$global:DockerVersion = "{{GetParameter "windowsDockerVersion"}}"

## ContainerD Usage
$global:DefaultContainerdWindowsSandboxIsolation = "{{GetParameter "defaultContainerdWindowsSandboxIsolation"}}"
$global:ContainerdWindowsRuntimeHandlers = "{{GetParameter "containerdWindowsRuntimeHandlers"}}"

## VM configuration passed by Azure
$global:WindowsTelemetryGUID = "{{GetParameter "windowsTelemetryGUID"}}"
{{if eq GetIdentitySystem "adfs"}}
$global:TenantId = "adfs"
{{else}}
$global:TenantId = "{{GetVariable "tenantID"}}"
{{end}}
$global:SubscriptionId = "{{GetVariable "subscriptionId"}}"
$global:ResourceGroup = "{{GetVariable "resourceGroup"}}"
$global:VmType = "{{GetVariable "vmType"}}"
$global:SubnetName = "{{GetVariable "subnetName"}}"
# NOTE: MasterSubnet is still referenced by `+"`"+`kubeletstart.ps1`+"`"+` and `+"`"+`windowsnodereset.ps1`+"`"+`
# for case of Kubenet
$global:MasterSubnet = ""
$global:SecurityGroupName = "{{GetVariable "nsgName"}}"
$global:VNetName = "{{GetVariable "virtualNetworkName"}}"
$global:RouteTableName = "{{GetVariable "routeTableName"}}"
$global:PrimaryAvailabilitySetName = "{{GetVariable "primaryAvailabilitySetName"}}"
$global:PrimaryScaleSetName = "{{GetVariable "primaryScaleSetName"}}"

$global:KubeClusterCIDR = "{{GetParameter "kubeClusterCidr"}}"
$global:KubeServiceCIDR = "{{GetParameter "kubeServiceCidr"}}"
$global:VNetCIDR = "{{GetParameter "vnetCidr"}}"
{{if IsKubernetesVersionGe "1.16.0"}}
$global:KubeletNodeLabels = "{{GetAgentKubernetesLabels . }}"
{{else}}
$global:KubeletNodeLabels = "{{GetAgentKubernetesLabelsDeprecated . }}"
{{end}}
$global:KubeletConfigArgs = @( {{GetKubeletConfigKeyValsPsh}} )
$global:KubeproxyConfigArgs = @( {{GetKubeproxyConfigKeyValsPsh}} )

$global:KubeproxyFeatureGates = @( {{GetKubeProxyFeatureGatesPsh}} )

$global:UseManagedIdentityExtension = "{{GetVariable "useManagedIdentityExtension"}}"
$global:UseInstanceMetadata = "{{GetVariable "useInstanceMetadata"}}"

$global:LoadBalancerSku = "{{GetVariable "loadBalancerSku"}}"
$global:ExcludeMasterFromStandardLB = "{{GetVariable "excludeMasterFromStandardLB"}}"

$global:PrivateEgressProxyAddress = "{{GetPrivateEgressProxyAddress}}"

# Windows defaults, not changed by aks-engine
$global:CacheDir = "c:\akse-cache"
$global:KubeDir = "c:\k"
$global:HNSModule = [Io.path]::Combine("$global:KubeDir", "hns.v2.psm1")

$global:KubeDnsSearchPath = "svc.cluster.local"

$global:CNIPath = [Io.path]::Combine("$global:KubeDir", "cni")
$global:NetworkMode = "L2Bridge"
$global:CNIConfig = [Io.path]::Combine($global:CNIPath, "config", "`+"`"+`$global:NetworkMode.conf")
$global:CNIConfigPath = [Io.path]::Combine("$global:CNIPath", "config")


$global:AzureCNIDir = [Io.path]::Combine("$global:KubeDir", "azurecni")
$global:AzureCNIBinDir = [Io.path]::Combine("$global:AzureCNIDir", "bin")
$global:AzureCNIConfDir = [Io.path]::Combine("$global:AzureCNIDir", "netconf")

# Azure cni configuration
# $global:NetworkPolicy = "{{GetParameter "networkPolicy"}}" # BUG: unused
$global:NetworkPlugin = "{{GetParameter "networkPlugin"}}"
$global:VNetCNIPluginsURL = "{{GetParameter "vnetCniWindowsPluginsURL"}}"
$global:IsDualStackEnabled = {{if IsIPv6DualStackFeatureEnabled}}$true{{else}}$false{{end}}
$global:IsAzureCNIOverlayEnabled = {{if IsAzureCNIOverlayFeatureEnabled}}$true{{else}}$false{{end}}

# CSI Proxy settings
$global:EnableCsiProxy = [System.Convert]::ToBoolean("{{GetVariable "windowsEnableCSIProxy" }}");
$global:CsiProxyUrl = "{{GetVariable "windowsCSIProxyURL" }}";

# Hosts Config Agent settings
$global:EnableHostsConfigAgent = [System.Convert]::ToBoolean("{{ EnableHostsConfigAgent }}");

# These scripts are used by cse
$global:CSEScriptsPackageUrl = "{{GetVariable "windowsCSEScriptsPackageURL" }}";

# PauseImage
$global:WindowsPauseImageURL = "{{GetVariable "windowsPauseImageURL" }}";
$global:AlwaysPullWindowsPauseImage = [System.Convert]::ToBoolean("{{GetVariable "alwaysPullWindowsPauseImage" }}");

# Calico
$global:WindowsCalicoPackageURL = "{{GetVariable "windowsCalicoPackageURL" }}";

# GMSA
$global:WindowsGmsaPackageUrl = "{{GetVariable "windowsGmsaPackageUrl" }}";

# TLS Bootstrap Token
$global:TLSBootstrapToken = "{{GetTLSBootstrapTokenForKubeConfig}}"

# Disable OutBoundNAT in Azure CNI configuration
$global:IsDisableWindowsOutboundNat = [System.Convert]::ToBoolean("{{GetVariable "isDisableWindowsOutboundNat" }}");

# Base64 representation of ZIP archive
$zippedFiles = "{{ GetKubernetesWindowsAgentFunctions }}"

$global:KubeClusterConfigPath = "c:\k\kubeclusterconfig.json"
$fipsEnabled = [System.Convert]::ToBoolean("{{ FIPSEnabled }}")

# HNS remediator
$global:HNSRemediatorIntervalInMinutes = [System.Convert]::ToUInt32("{{GetHnsRemediatorIntervalInMinutes}}");

# Log generator
$global:LogGeneratorIntervalInMinutes = [System.Convert]::ToUInt32("{{GetLogGeneratorIntervalInMinutes}}");

$global:EnableIncreaseDynamicPortRange = $false

# Extract cse helper script from ZIP
[io.file]::WriteAllBytes("scripts.zip", [System.Convert]::FromBase64String($zippedFiles))
Expand-Archive scripts.zip -DestinationPath "C:\\AzureData\\"

# Dot-source windowscsehelper.ps1 with functions that are called in this script
. c:\AzureData\windows\windowscsehelper.ps1
# util functions only can be used after this line, for example, Write-Log

try
{
    Write-Log ".\CustomDataSetupScript.ps1 -MasterIP $MasterIP -KubeDnsServiceIp $KubeDnsServiceIp -MasterFQDNPrefix $MasterFQDNPrefix -Location $Location -AADClientId $AADClientId -NetworkAPIVersion $NetworkAPIVersion -TargetEnvironment $TargetEnvironment"

    # Exit early if the script has been executed
    if (Test-Path -Path $CSEResultFilePath -PathType Leaf) {
        Write-Log "The script has been executed before, will exit without doing anything."
        return
    }

    # This involes using proxy, log the config before fetching packages
    Write-Log "private egress proxy address is '$global:PrivateEgressProxyAddress'"
    # TODO update to use proxy

    $WindowsCSEScriptsPackage = "aks-windows-cse-scripts-v0.0.29.zip"
    Write-Log "CSEScriptsPackageUrl is $global:CSEScriptsPackageUrl"
    Write-Log "WindowsCSEScriptsPackage is $WindowsCSEScriptsPackage"
    # Old AKS RP sets the full URL (https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.11.zip) in CSEScriptsPackageUrl
    # but it is better to set the CSE package version in Windows CSE in AgentBaker
    # since most changes in CSE package also need the change in Windows CSE in AgentBaker
    # In future, AKS RP only sets the endpoint with the pacakge name, for example, https://acs-mirror.azureedge.net/aks/windows/cse/
    if ($global:CSEScriptsPackageUrl.EndsWith("/")) {
        $global:CSEScriptsPackageUrl = $global:CSEScriptsPackageUrl + $WindowsCSEScriptsPackage
        Write-Log "CSEScriptsPackageUrl is set to $global:CSEScriptsPackageUrl"
    }
    # Download CSE function scripts
    Write-Log "Getting CSE scripts"
    $tempfile = 'c:\csescripts.zip'
    DownloadFileOverHttp -Url $global:CSEScriptsPackageUrl -DestinationPath $tempfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CSE_PACKAGE
    Expand-Archive $tempfile -DestinationPath "C:\\AzureData\\windows"
    Remove-Item -Path $tempfile -Force

    # Dot-source cse scripts with functions that are called in this script
    . c:\AzureData\windows\azurecnifunc.ps1
    . c:\AzureData\windows\calicofunc.ps1
    . c:\AzureData\windows\configfunc.ps1
    . c:\AzureData\windows\containerdfunc.ps1
    . c:\AzureData\windows\kubeletfunc.ps1
    . c:\AzureData\windows\kubernetesfunc.ps1

    # Install OpenSSH if SSH enabled
    $sshEnabled = [System.Convert]::ToBoolean("{{ WindowsSSHEnabled }}")

    if ( $sshEnabled ) {
        Write-Log "Install OpenSSH"
        Install-OpenSSH -SSHKeys $SSHKeys
    }

    Write-Log "Apply telemetry data setting"
    Set-TelemetrySetting -WindowsTelemetryGUID $global:WindowsTelemetryGUID

    Write-Log "Resize os drive if possible"
    Resize-OSDrive

    Write-Log "Initialize data disks"
    Initialize-DataDisks

    Write-Log "Create required data directories as needed"
    Initialize-DataDirectories

    Create-Directory -FullPath "c:\k"
    Write-Log "Remove `+"`"+`"NT AUTHORITY\Authenticated Users`+"`"+`" write permissions on files in c:\k"
    icacls.exe "c:\k" /inheritance:r
    icacls.exe "c:\k" /grant:r SYSTEM:`+"`"+`(OI`+"`"+`)`+"`"+`(CI`+"`"+`)`+"`"+`(F`+"`"+`)
    icacls.exe "c:\k" /grant:r BUILTIN\Administrators:`+"`"+`(OI`+"`"+`)`+"`"+`(CI`+"`"+`)`+"`"+`(F`+"`"+`)
    icacls.exe "c:\k" /grant:r BUILTIN\Users:`+"`"+`(OI`+"`"+`)`+"`"+`(CI`+"`"+`)`+"`"+`(RX`+"`"+`)
    Write-Log "c:\k permissions: "
    icacls.exe "c:\k"
    Get-ProvisioningScripts
    Get-LogCollectionScripts

    Write-KubeClusterConfig -MasterIP $MasterIP -KubeDnsServiceIp $KubeDnsServiceIp

    Write-Log "Download kubelet binaries and unzip"
    Get-KubePackage -KubeBinariesSASURL $global:KubeBinariesPackageSASURL

    # This overwrites the binaries that are downloaded from the custom packge with binaries.
    # The custom package has a few files that are necessary for future steps (nssm.exe)
    # this is a temporary work around to get the binaries until we depreciate
    # custom package and nssm.exe as defined in aks-engine#3851.
    if ($global:WindowsKubeBinariesURL){
        Write-Log "Overwriting kube node binaries from $global:WindowsKubeBinariesURL"
        Get-KubeBinaries -KubeBinariesURL $global:WindowsKubeBinariesURL
    }

    Write-Log "Installing ContainerD"
    $cniBinPath = $global:AzureCNIBinDir
    $cniConfigPath = $global:AzureCNIConfDir
    if ($global:NetworkPlugin -eq "kubenet") {
        $cniBinPath = $global:CNIPath
        $cniConfigPath = $global:CNIConfigPath
    }
    Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $global:ContainerdUrl -CNIBinDir $cniBinPath -CNIConfDir $cniConfigPath -KubeDir $global:KubeDir -KubernetesVersion $global:KubeBinariesVersion

    Retag-ImagesForAzureChinaCloud -TargetEnvironment $TargetEnvironment

    # For AKSClustomCloud, TargetEnvironment must be set to AzureStackCloud
    Write-Log "Write Azure cloud provider config"
    Write-AzureConfig `+"`"+`
        -KubeDir $global:KubeDir `+"`"+`
        -AADClientId $AADClientId `+"`"+`
        -AADClientSecret $([System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($AADClientSecret))) `+"`"+`
        -TenantId $global:TenantId `+"`"+`
        -SubscriptionId $global:SubscriptionId `+"`"+`
        -ResourceGroup $global:ResourceGroup `+"`"+`
        -Location $Location `+"`"+`
        -VmType $global:VmType `+"`"+`
        -SubnetName $global:SubnetName `+"`"+`
        -SecurityGroupName $global:SecurityGroupName `+"`"+`
        -VNetName $global:VNetName `+"`"+`
        -RouteTableName $global:RouteTableName `+"`"+`
        -PrimaryAvailabilitySetName $global:PrimaryAvailabilitySetName `+"`"+`
        -PrimaryScaleSetName $global:PrimaryScaleSetName `+"`"+`
        -UseManagedIdentityExtension $global:UseManagedIdentityExtension `+"`"+`
        -UserAssignedClientID $UserAssignedClientID `+"`"+`
        -UseInstanceMetadata $global:UseInstanceMetadata `+"`"+`
        -LoadBalancerSku $global:LoadBalancerSku `+"`"+`
        -ExcludeMasterFromStandardLB $global:ExcludeMasterFromStandardLB `+"`"+`
        -TargetEnvironment {{if IsAKSCustomCloud}}"AzureStackCloud"{{else}}$TargetEnvironment{{end}} 

    # we borrow the logic of AzureStackCloud to achieve AKSCustomCloud. 
    # In case of AKSCustomCloud, customer cloud env will be loaded from azurestackcloud.json 
    {{if IsAKSCustomCloud}}
    $azureStackConfigFile = [io.path]::Combine($global:KubeDir, "azurestackcloud.json")
    $envJSON = "{{ GetBase64EncodedEnvironmentJSON }}"
    [io.file]::WriteAllBytes($azureStackConfigFile, [System.Convert]::FromBase64String($envJSON))

    Get-CACertificates
    {{end}}

    Write-Log "Write ca root"
    Write-CACert -CACertificate $global:CACertificate `+"`"+`
        -KubeDir $global:KubeDir

    if ($global:EnableCsiProxy) {
        New-CsiProxyService -CsiProxyPackageUrl $global:CsiProxyUrl -KubeDir $global:KubeDir
    }

    if ($global:TLSBootstrapToken) {
        Write-Log "Write TLS bootstrap kubeconfig"
        Write-BootstrapKubeConfig -CACertificate $global:CACertificate `+"`"+`
            -KubeDir $global:KubeDir `+"`"+`
            -MasterFQDNPrefix $MasterFQDNPrefix `+"`"+`
            -MasterIP $MasterIP `+"`"+`
            -TLSBootstrapToken $global:TLSBootstrapToken

        # NOTE: we need kubeconfig to setup calico even if TLS bootstrapping is enabled
        #       This kubeconfig will deleted after calico installation.
        # TODO(hbc): once TLS bootstrap is fully enabled, remove this if block
        Write-Log "Write temporary kube config"
    } else {
        Write-Log "Write kube config"
    }

    Write-KubeConfig -CACertificate $global:CACertificate `+"`"+`
        -KubeDir $global:KubeDir `+"`"+`
        -MasterFQDNPrefix $MasterFQDNPrefix `+"`"+`
        -MasterIP $MasterIP `+"`"+`
        -AgentKey $AgentKey `+"`"+`
        -AgentCertificate $global:AgentCertificate

    if ($global:EnableHostsConfigAgent) {
        Write-Log "Starting hosts config agent"
        New-HostsConfigService
    }

    Write-Log "Configuring networking with NetworkPlugin:$global:NetworkPlugin"

    # Configure network policy.
    Get-HnsPsm1 -HNSModule $global:HNSModule
    Import-Module $global:HNSModule

    Write-Log "Installing Azure VNet plugins"
    Install-VnetPlugins -AzureCNIConfDir $global:AzureCNIConfDir `+"`"+`
        -AzureCNIBinDir $global:AzureCNIBinDir `+"`"+`
        -VNetCNIPluginsURL $global:VNetCNIPluginsURL

    Set-AzureCNIConfig -AzureCNIConfDir $global:AzureCNIConfDir `+"`"+`
        -KubeDnsSearchPath $global:KubeDnsSearchPath `+"`"+`
        -KubeClusterCIDR $global:KubeClusterCIDR `+"`"+`
        -KubeServiceCIDR $global:KubeServiceCIDR `+"`"+`
        -VNetCIDR $global:VNetCIDR `+"`"+`
        -IsDualStackEnabled $global:IsDualStackEnabled `+"`"+`
        -IsAzureCNIOverlayEnabled $global:IsAzureCNIOverlayEnabled

    if ($TargetEnvironment -ieq "AzureStackCloud") {
        GenerateAzureStackCNIConfig `+"`"+`
            -TenantId $global:TenantId `+"`"+`
            -SubscriptionId $global:SubscriptionId `+"`"+`
            -ResourceGroup $global:ResourceGroup `+"`"+`
            -AADClientId $AADClientId `+"`"+`
            -KubeDir $global:KubeDir `+"`"+`
            -AADClientSecret $([System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($AADClientSecret))) `+"`"+`
            -NetworkAPIVersion $NetworkAPIVersion `+"`"+`
            -AzureEnvironmentFilePath $([io.path]::Combine($global:KubeDir, "azurestackcloud.json")) `+"`"+`
            -IdentitySystem "{{ GetIdentitySystem }}"
    }

    New-ExternalHnsNetwork -IsDualStackEnabled $global:IsDualStackEnabled

    Install-KubernetesServices `+"`"+`
        -KubeDir $global:KubeDir

    Write-Log "Disable Internet Explorer compat mode and set homepage"
    Set-Explorer

    Write-Log "Adjust pagefile size"
    Adjust-PageFileSize

    Write-Log "Start preProvisioning script"
    PREPROVISION_EXTENSION

    Write-Log "Update service failure actions"
    Update-ServiceFailureActions
    Adjust-DynamicPortRange
    Register-LogsCleanupScriptTask
    Register-NodeResetScriptTask
    Update-DefenderPreferences


    $windowsVersion = Get-WindowsVersion
    if ($windowsVersion -ne "1809") {
        Write-Log "Skip secure TLS protocols for Windows version: $windowsVersion"
    } else {
        Write-Log "Enable secure TLS protocols"
        try {
            . C:\k\windowssecuretls.ps1
            Enable-SecureTls
        }
        catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ENABLE_SECURE_TLS -ErrorMessage $_
        }
    }

    Enable-FIPSMode -FipsEnabled $fipsEnabled
    if ($global:WindowsGmsaPackageUrl) {
        Write-Log "Start to install Windows gmsa package"
        Install-GmsaPlugin -GmsaPackageUrl $global:WindowsGmsaPackageUrl
    }

    Check-APIServerConnectivity -MasterIP $MasterIP

    if ($global:WindowsCalicoPackageURL) {
        Write-Log "Start calico installation"
        Start-InstallCalico -RootDir "c:\" -KubeServiceCIDR $global:KubeServiceCIDR -KubeDnsServiceIp $KubeDnsServiceIp
    }

    if (Test-Path $CacheDir)
    {
        Write-Log "Removing aks-engine bits cache directory"
        Remove-Item $CacheDir -Recurse -Force
    }

    if ($global:TLSBootstrapToken) {
        Write-Log "Removing temporary kube config"
        $kubeConfigFile = [io.path]::Combine($KubeDir, "config")
        Remove-Item $kubeConfigFile
    }

    Enable-GuestVMLogs -IntervalInMinutes $global:LogGeneratorIntervalInMinutes

    Write-Log "Setup Complete, starting NodeResetScriptTask to register Winodws node without reboot"
    Start-ScheduledTask -TaskName "k8s-restart-job"

    $timeout = 180 ##  seconds
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ((Get-ScheduledTask -TaskName 'k8s-restart-job').State -ne 'Ready') {
        # The task `+"`"+`k8s-restart-job`+"`"+` needs ~8 seconds.
        if ($timer.Elapsed.TotalSeconds -gt $timeout) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_START_NODE_RESET_SCRIPT_TASK -ErrorMessage "NodeResetScriptTask is not finished after [$($timer.Elapsed.TotalSeconds)] seconds"
        }

        Write-Log -Message "Waiting on NodeResetScriptTask..."
        Start-Sleep -Seconds 3
    }
    $timer.Stop()
    Write-Log -Message "We waited [$($timer.Elapsed.TotalSeconds)] seconds on NodeResetScriptTask"
}
catch
{
    # Set-ExitCode will exit with the specified ExitCode immediately and not be caught by this catch block
    # Ideally all exceptions will be handled and no exception will be thrown.
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_UNKNOWN -ErrorMessage $_
}
finally
{
    # Generate CSE result so it can be returned as the CSE response in csecmd.ps1
    $ExecutionDuration=$(New-Timespan -Start $StartTime -End $(Get-Date))
    Write-Log "CSE ExecutionDuration: $ExecutionDuration"

    # Windows CSE does not return any error message so we cannot generate below content as the response
    # $JsonString = "ExitCode: `+"`"+`"{0}`+"`"+`", Output: `+"`"+`"{1}`+"`"+`", Error: `+"`"+`"{2}`+"`"+`", ExecDuration: `+"`"+`"{3}`+"`"+`"" -f $global:ExitCode, "", $global:ErrorMessage, $ExecutionDuration.TotalSeconds
    Write-Log "Generate CSE result to $CSEResultFilePath : $global:ExitCode"
    echo $global:ExitCode | Out-File -FilePath $CSEResultFilePath -Encoding utf8

    # Flush stdout to C:\AzureData\CustomDataSetupScript.log
    [Console]::Out.Flush()

    Upload-GuestVMLogs -ExitCode $global:ExitCode
}
`)

func windowsKuberneteswindowssetupPs1Bytes() ([]byte, error) {
	return _windowsKuberneteswindowssetupPs1, nil
}

func windowsKuberneteswindowssetupPs1() (*asset, error) {
	bytes, err := windowsKuberneteswindowssetupPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/kuberneteswindowssetup.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsSendlogsPs1 = []byte(`<#
    .SYNOPSIS
        Uploads a log bundle to the host for retrieval via GuestVMLogs.

    .DESCRIPTION
        Uploads a log bundle to the host for retrieval via GuestVMLogs.

        Takes a parameter of a ZIP file name to upload, which is sent to the HostAgent
        via the /vmAgentLog endpoint.
#>
[CmdletBinding()]
param(
    [string]
    $Path
)

if (!(Test-Path $Path)) {
    return
}

$GoalStateArgs = @{
    "Method"="Get";
    "Uri"="http://168.63.129.16/machine/?comp=goalstate";
    "Headers"=@{"x-ms-version"="2012-11-30"}
}
$GoalState = $(Invoke-RestMethod @GoalStateArgs).GoalState

$UploadArgs = @{
    "Method"="Put";
    "Uri"="http://168.63.129.16:32526/vmAgentLog";
    "InFile"=$Path;
    "Headers"=@{
        "x-ms-version"="2015-09-01";
        "x-ms-client-correlationid"="";
        "x-ms-client-name"="AKSCSEPlugin";
        "x-ms-client-version"="0.1.0";
        "x-ms-containerid"=$GoalState.Container.ContainerId;
        "x-ms-vmagentlog-deploymentid"=($GoalState.Container.RoleInstanceList.RoleInstance.Configuration.ConfigName -split "\.")[0]
    }
}
Invoke-RestMethod @UploadArgs`)

func windowsSendlogsPs1Bytes() ([]byte, error) {
	return _windowsSendlogsPs1, nil
}

func windowsSendlogsPs1() (*asset, error) {
	bytes, err := windowsSendlogsPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/sendlogs.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowscsehelperPs1 = []byte(`# This script is used to define basic util functions
# It is better to define functions in the scripts under staging/cse/windows.

# Define all exit codes in Windows CSE
$global:WINDOWS_CSE_ERROR_UNKNOWN=1 # For unexpected error caught by the catch block in kuberneteswindowssetup.ps1
$global:WINDOWS_CSE_ERROR_DOWNLOAD_FILE_WITH_RETRY=2
$global:WINDOWS_CSE_ERROR_INVOKE_EXECUTABLE=3
$global:WINDOWS_CSE_ERROR_FILE_NOT_EXIST=4
$global:WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY=5
$global:WINDOWS_CSE_ERROR_PAUSE_IMAGE_NOT_EXIST=6
$global:WINDOWS_CSE_ERROR_GET_SUBNET_PREFIX=7
$global:WINDOWS_CSE_ERROR_GENERATE_TOKEN_FOR_ARM=8
$global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST=9
$global:WINDOWS_CSE_ERROR_NETWORK_ADAPTER_NOT_EXIST=10
$global:WINDOWS_CSE_ERROR_MANAGEMENT_IP_NOT_EXIST=11
$global:WINDOWS_CSE_ERROR_CALICO_SERVICE_ACCOUNT_NOT_EXIST=12
$global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED=13
$global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_RUNNING=14
$global:WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED=15
$global:WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED=16
$global:WINDOWS_CSE_ERROR_INVALID_PARAMETER_IN_AZURE_CONFIG=17
$global:WINDOWS_CSE_ERROR_NO_DOCKER_TO_BUILD_PAUSE_CONTAINER=18
$global:WINDOWS_CSE_ERROR_GET_CA_CERTIFICATES=19
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CA_CERTIFICATES=20
$global:WINDOWS_CSE_ERROR_EMPTY_CA_CERTIFICATES=21
$global:WINDOWS_CSE_ERROR_ENABLE_SECURE_TLS=22
$global:WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE=23
$global:WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE=24
$global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION=25
$global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES=26
$global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS=27
$global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGAKVPPLUGINEVENTS=28
$global:WINDOWS_CSE_ERROR_NOT_FOUND_MANAGEMENT_IP=29
$global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER=30
$global:WINDOWS_CSE_ERROR_NOT_FOUND_PROVISIONING_SCRIPTS=31
$global:WINDOWS_CSE_ERROR_START_NODE_RESET_SCRIPT_TASK=32
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CSE_PACKAGE=33
$global:WINDOWS_CSE_ERROR_DOWNLOAD_KUBERNETES_PACKAGE=34
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CNI_PACKAGE=35
$global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE=36
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CALICO_PACKAGE=37
$global:WINDOWS_CSE_ERROR_DOWNLOAD_GMSA_PACKAGE=38
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CSI_PROXY_PACKAGE=39
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE=40
$global:WINDOWS_CSE_ERROR_SET_TCP_DYNAMIC_PORT_RANGE=41
$global:WINDOWS_CSE_ERROR_BUILD_DOCKER_PAUSE_CONTAINER=42
$global:WINDOWS_CSE_ERROR_PULL_PAUSE_IMAGE=43
$global:WINDOWS_CSE_ERROR_BUILD_TAG_PAUSE_IMAGE=44
$global:WINDOWS_CSE_ERROR_CONTAINERD_BINARY_EXIST=45
$global:WINDOWS_CSE_ERROR_SET_TCP_EXCLUDE_PORT_RANGE=46
$global:WINDOWS_CSE_ERROR_SET_UDP_DYNAMIC_PORT_RANGE=47
$global:WINDOWS_CSE_ERROR_SET_UDP_EXCLUDE_PORT_RANGE=48
$global:WINDOWS_CSE_ERROR_NO_CUSTOM_DATA_BIN=49 # Return this error code in csecmd.ps1 when C:\AzureData\CustomData.bin does not exist
$global:WINDOWS_CSE_ERROR_NO_CSE_RESULT_LOG=50 # Return this error code in csecmd.ps1 when C:\AzureData\CSEResult.log does not exist
$global:WINDOWS_CSE_ERROR_COPY_LOG_COLLECTION_SCRIPTS=51
$global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE=52

# NOTE: KubernetesVersion does not contain "v"
$global:MinimalKubernetesVersionWithLatestContainerd = "1.28.0" # Will change it to the correct version when we support new Windows containerd version
$global:StableContainerdPackage = "v1.6.21-azure.1/binaries/containerd-v1.6.21-azure.1-windows-amd64.tar.gz"
# The latest containerd version
$global:LatestContainerdPackage = "v1.7.1-azure.1/binaries/containerd-v1.7.1-azure.1-windows-amd64.tar.gz"

# This filter removes null characters (\0) which are captured in nssm.exe output when logged through powershell
filter RemoveNulls { $_ -replace '\0', '' }

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

function DownloadFileOverHttp {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Url,
        [Parameter(Mandatory = $true)][string]
        $DestinationPath,
        [Parameter(Mandatory = $true)][int]
        $ExitCode
    )

    # First check to see if a file with the same name is already cached on the VHD
    $fileName = [IO.Path]::GetFileName($Url)

    $search = @()
    if (Test-Path $global:CacheDir) {
        $search = [IO.Directory]::GetFiles($global:CacheDir, $fileName, [IO.SearchOption]::AllDirectories)
    }

    if ($search.Count -ne 0) {
        Write-Log "Using cached version of $fileName - Copying file from $($search[0]) to $DestinationPath"
        Copy-Item -Path $search[0] -Destination $DestinationPath -Force
    }
    else {
        $secureProtocols = @()
        $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)

        foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType])) {
            if ($insecureProtocols -notcontains $protocol) {
                $secureProtocols += $protocol
            }
        }
        [System.Net.ServicePointManager]::SecurityProtocol = $secureProtocols

        $oldProgressPreference = $ProgressPreference
        $ProgressPreference = 'SilentlyContinue'

        $downloadTimer = [System.Diagnostics.Stopwatch]::StartNew()
        try {
            $args = @{Uri=$Url; Method="Get"; OutFile=$DestinationPath}
            Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $ExitCode -ErrorMessage "Failed in downloading $Url. Error: $_"
        }
        $downloadTimer.Stop()

        if ($global:AppInsightsClient -ne $null) {
            $event = New-Object "Microsoft.ApplicationInsights.DataContracts.EventTelemetry"
            $event.Name = "FileDownload"
            $event.Properties["FileName"] = $fileName
            $event.Metrics["DurationMs"] = $downloadTimer.ElapsedMilliseconds
            $global:AppInsightsClient.TrackEvent($event)
        }

        $ProgressPreference = $oldProgressPreference
        Write-Log "Downloaded file $Url to $DestinationPath"
    }
}

function Set-ExitCode
{
    Param(
        [Parameter(Mandatory=$true)][int]
        $ExitCode,
        [Parameter(Mandatory=$true)][string]
        $ErrorMessage
    )
    Write-Log "Set ExitCode to $ExitCode and exit. Error: $ErrorMessage"
    $global:ExitCode=$ExitCode
    $global:ErrorMessage=$ErrorMessage
    exit $ExitCode
}

function Create-Directory
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $FullPath,
        [Parameter(Mandatory=$false)][string]
        $DirectoryUsage = "general purpose"
    )
    
    if (-Not (Test-Path $FullPath)) {
        Write-Log "Create directory $FullPath for $DirectoryUsage"
        New-Item -ItemType Directory -Path $FullPath > $null
    } else {
        Write-Log "Directory $FullPath for $DirectoryUsage exists"
    }
}

# https://stackoverflow.com/a/34559554/697126
function New-TemporaryDirectory {
    $parent = [System.IO.Path]::GetTempPath()
    [string] $name = [System.Guid]::NewGuid()
    New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

function Retry-Command {
    Param(
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]
        $Command,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][hashtable]
        $Args,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][int]
        $Retries,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][int]
        $RetryDelaySeconds
    )

    for ($i = 0; ; ) {
        try {
            # Do not log Args since Args may contain sensitive data
            Write-Log "Retry $i : $command"
            return & $Command @Args
        }
        catch {
            $i++
            if ($i -ge $Retries) {
                throw $_
            }
            Start-Sleep $RetryDelaySeconds
        }
    }
}

function Invoke-Executable {
    Param(
        [Parameter(Mandatory=$true)][string]
        $Executable,
        [Parameter(Mandatory=$true)][string[]]
        $ArgList,
        [Parameter(Mandatory=$true)][int]
        $ExitCode,
        [int[]]
        $AllowedExitCodes = @(0),
        [int]
        $Retries = 0,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -le $Retries; $i++) {
        Write-Log "$i - Running $Executable $ArgList ..."
        & $Executable $ArgList
        if ($LASTEXITCODE -notin $AllowedExitCodes) {
            Write-Log "$Executable returned unsuccessfully with exit code $LASTEXITCODE"
            Start-Sleep -Seconds $RetryDelaySeconds
            continue
        }
        else {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    Set-ExitCode -ExitCode $ExitCode -ErrorMessage "Exhausted retries for $Executable $ArgList"
}

function Assert-FileExists {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Filename,
        [Parameter(Mandatory = $true)][int]
        $ExitCode
    )

    if (-Not (Test-Path $Filename)) {
        Set-ExitCode -ExitCode $ExitCode -ErrorMessage "$Filename does not exist"
    }
}

function Get-WindowsVersion {
    $buildNumber = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").CurrentBuild
    switch ($buildNumber) {
        "17763" { return "1809" }
        "20348" { return "ltsc2022" }
        Default {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER -ErrorMessage "Failed to find the windows build number: $buildNumber"
        }
    }
}

function Install-Containerd-Based-On-Kubernetes-Version {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir,
    [Parameter(Mandatory = $true)][string]
    $KubernetesVersion
  )

  # In the past, $global:ContainerdUrl is a full URL to download Windows containerd package.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/v0.0.46/binaries/containerd-v0.0.46-windows-amd64.tar.gz"
  # To support multiple containerd versions, we only set the endpoint in $global:ContainerdUrl.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/"
  # We only set containerd package based on kubernetes version when $global:ContainerdUrl ends with "/" so we support:
  #   1. Current behavior to set the full URL
  #   2. Setting containerd package in toggle for test purpose or hotfix
  if ($ContainerdUrl.EndsWith("/")) {
    Write-Log "ContainerdURL is $ContainerdUrl"
    $containerdPackage=$global:StableContainerdPackage
    if (([version]$KubernetesVersion).CompareTo([version]$global:MinimalKubernetesVersionWithLatestContainerd) -ge 0) {
      $containerdPackage=$global:LatestContainerdPackage
      Write-Log "Kubernetes version $KubernetesVersion is greater than or equal to $global:MinimalKubernetesVersionWithLatestContainerd so the latest containerd version $containerdPackage is used"
    } else {
      Write-Log "Kubernetes version $KubernetesVersion is less than $global:MinimalKubernetesVersionWithLatestContainerd so the stable containerd version $containerdPackage is used"
    }
    $ContainerdUrl = $ContainerdUrl + $containerdPackage
  }
  Install-Containerd -ContainerdUrl $ContainerdUrl -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
}
`)

func windowsWindowscsehelperPs1Bytes() ([]byte, error) {
	return _windowsWindowscsehelperPs1, nil
}

func windowsWindowscsehelperPs1() (*asset, error) {
	bytes, err := windowsWindowscsehelperPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowscsehelper.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"linux/cloud-init/artifacts/10-bindmount.conf":                         linuxCloudInitArtifacts10BindmountConf,
	"linux/cloud-init/artifacts/10-cgroupv2.conf":                          linuxCloudInitArtifacts10Cgroupv2Conf,
	"linux/cloud-init/artifacts/10-componentconfig.conf":                   linuxCloudInitArtifacts10ComponentconfigConf,
	"linux/cloud-init/artifacts/10-containerd.conf":                        linuxCloudInitArtifacts10ContainerdConf,
	"linux/cloud-init/artifacts/10-httpproxy.conf":                         linuxCloudInitArtifacts10HttpproxyConf,
	"linux/cloud-init/artifacts/10-tlsbootstrap.conf":                      linuxCloudInitArtifacts10TlsbootstrapConf,
	"linux/cloud-init/artifacts/aks-logrotate-override.conf":               linuxCloudInitArtifactsAksLogrotateOverrideConf,
	"linux/cloud-init/artifacts/aks-logrotate.service":                     linuxCloudInitArtifactsAksLogrotateService,
	"linux/cloud-init/artifacts/aks-logrotate.sh":                          linuxCloudInitArtifactsAksLogrotateSh,
	"linux/cloud-init/artifacts/aks-logrotate.timer":                       linuxCloudInitArtifactsAksLogrotateTimer,
	"linux/cloud-init/artifacts/aks-rsyslog":                               linuxCloudInitArtifactsAksRsyslog,
	"linux/cloud-init/artifacts/apt-preferences":                           linuxCloudInitArtifactsAptPreferences,
	"linux/cloud-init/artifacts/bind-mount.service":                        linuxCloudInitArtifactsBindMountService,
	"linux/cloud-init/artifacts/bind-mount.sh":                             linuxCloudInitArtifactsBindMountSh,
	"linux/cloud-init/artifacts/block_wireserver.sh":                       linuxCloudInitArtifactsBlock_wireserverSh,
	"linux/cloud-init/artifacts/cgroup-memory-telemetry.service":           linuxCloudInitArtifactsCgroupMemoryTelemetryService,
	"linux/cloud-init/artifacts/cgroup-memory-telemetry.sh":                linuxCloudInitArtifactsCgroupMemoryTelemetrySh,
	"linux/cloud-init/artifacts/cgroup-memory-telemetry.timer":             linuxCloudInitArtifactsCgroupMemoryTelemetryTimer,
	"linux/cloud-init/artifacts/cgroup-pressure-telemetry.service":         linuxCloudInitArtifactsCgroupPressureTelemetryService,
	"linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh":              linuxCloudInitArtifactsCgroupPressureTelemetrySh,
	"linux/cloud-init/artifacts/cgroup-pressure-telemetry.timer":           linuxCloudInitArtifactsCgroupPressureTelemetryTimer,
	"linux/cloud-init/artifacts/ci-syslog-watcher.path":                    linuxCloudInitArtifactsCiSyslogWatcherPath,
	"linux/cloud-init/artifacts/ci-syslog-watcher.service":                 linuxCloudInitArtifactsCiSyslogWatcherService,
	"linux/cloud-init/artifacts/ci-syslog-watcher.sh":                      linuxCloudInitArtifactsCiSyslogWatcherSh,
	"linux/cloud-init/artifacts/cis.sh":                                    linuxCloudInitArtifactsCisSh,
	"linux/cloud-init/artifacts/containerd-monitor.service":                linuxCloudInitArtifactsContainerdMonitorService,
	"linux/cloud-init/artifacts/containerd-monitor.timer":                  linuxCloudInitArtifactsContainerdMonitorTimer,
	"linux/cloud-init/artifacts/containerd.service":                        linuxCloudInitArtifactsContainerdService,
	"linux/cloud-init/artifacts/containerd_exec_start.conf":                linuxCloudInitArtifactsContainerd_exec_startConf,
	"linux/cloud-init/artifacts/crictl.yaml":                               linuxCloudInitArtifactsCrictlYaml,
	"linux/cloud-init/artifacts/cse_cmd.sh":                                linuxCloudInitArtifactsCse_cmdSh,
	"linux/cloud-init/artifacts/cse_config.sh":                             linuxCloudInitArtifactsCse_configSh,
	"linux/cloud-init/artifacts/cse_helpers.sh":                            linuxCloudInitArtifactsCse_helpersSh,
	"linux/cloud-init/artifacts/cse_install.sh":                            linuxCloudInitArtifactsCse_installSh,
	"linux/cloud-init/artifacts/cse_main.sh":                               linuxCloudInitArtifactsCse_mainSh,
	"linux/cloud-init/artifacts/cse_redact_cloud_config.py":                linuxCloudInitArtifactsCse_redact_cloud_configPy,
	"linux/cloud-init/artifacts/cse_send_logs.py":                          linuxCloudInitArtifactsCse_send_logsPy,
	"linux/cloud-init/artifacts/cse_start.sh":                              linuxCloudInitArtifactsCse_startSh,
	"linux/cloud-init/artifacts/dhcpv6.service":                            linuxCloudInitArtifactsDhcpv6Service,
	"linux/cloud-init/artifacts/disk_queue.service":                        linuxCloudInitArtifactsDisk_queueService,
	"linux/cloud-init/artifacts/docker-monitor.service":                    linuxCloudInitArtifactsDockerMonitorService,
	"linux/cloud-init/artifacts/docker-monitor.timer":                      linuxCloudInitArtifactsDockerMonitorTimer,
	"linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf": linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf,
	"linux/cloud-init/artifacts/enable-dhcpv6.sh":                          linuxCloudInitArtifactsEnableDhcpv6Sh,
	"linux/cloud-init/artifacts/ensure-no-dup.service":                     linuxCloudInitArtifactsEnsureNoDupService,
	"linux/cloud-init/artifacts/ensure-no-dup.sh":                          linuxCloudInitArtifactsEnsureNoDupSh,
	"linux/cloud-init/artifacts/etc-issue":                                 linuxCloudInitArtifactsEtcIssue,
	"linux/cloud-init/artifacts/etc-issue.net":                             linuxCloudInitArtifactsEtcIssueNet,
	"linux/cloud-init/artifacts/health-monitor.sh":                         linuxCloudInitArtifactsHealthMonitorSh,
	"linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh":          linuxCloudInitArtifactsInitAksCustomCloudMarinerSh,
	"linux/cloud-init/artifacts/init-aks-custom-cloud.sh":                  linuxCloudInitArtifactsInitAksCustomCloudSh,
	"linux/cloud-init/artifacts/ipv6_nftables":                             linuxCloudInitArtifactsIpv6_nftables,
	"linux/cloud-init/artifacts/ipv6_nftables.service":                     linuxCloudInitArtifactsIpv6_nftablesService,
	"linux/cloud-init/artifacts/ipv6_nftables.sh":                          linuxCloudInitArtifactsIpv6_nftablesSh,
	"linux/cloud-init/artifacts/kms.service":                               linuxCloudInitArtifactsKmsService,
	"linux/cloud-init/artifacts/kubelet-monitor.service":                   linuxCloudInitArtifactsKubeletMonitorService,
	"linux/cloud-init/artifacts/kubelet-monitor.timer":                     linuxCloudInitArtifactsKubeletMonitorTimer,
	"linux/cloud-init/artifacts/kubelet.service":                           linuxCloudInitArtifactsKubeletService,
	"linux/cloud-init/artifacts/manifest.json":                             linuxCloudInitArtifactsManifestJson,
	"linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh":            linuxCloudInitArtifactsMarinerCse_helpers_marinerSh,
	"linux/cloud-init/artifacts/mariner/cse_install_mariner.sh":            linuxCloudInitArtifactsMarinerCse_install_marinerSh,
	"linux/cloud-init/artifacts/mariner/pam-d-system-auth":                 linuxCloudInitArtifactsMarinerPamDSystemAuth,
	"linux/cloud-init/artifacts/mariner/pam-d-system-password":             linuxCloudInitArtifactsMarinerPamDSystemPassword,
	"linux/cloud-init/artifacts/mariner/update_certs_mariner.service":      linuxCloudInitArtifactsMarinerUpdate_certs_marinerService,
	"linux/cloud-init/artifacts/mig-partition.service":                     linuxCloudInitArtifactsMigPartitionService,
	"linux/cloud-init/artifacts/mig-partition.sh":                          linuxCloudInitArtifactsMigPartitionSh,
	"linux/cloud-init/artifacts/modprobe-CIS.conf":                         linuxCloudInitArtifactsModprobeCisConf,
	"linux/cloud-init/artifacts/nvidia-device-plugin.service":              linuxCloudInitArtifactsNvidiaDevicePluginService,
	"linux/cloud-init/artifacts/nvidia-docker-daemon.json":                 linuxCloudInitArtifactsNvidiaDockerDaemonJson,
	"linux/cloud-init/artifacts/nvidia-modprobe.service":                   linuxCloudInitArtifactsNvidiaModprobeService,
	"linux/cloud-init/artifacts/pam-d-common-auth":                         linuxCloudInitArtifactsPamDCommonAuth,
	"linux/cloud-init/artifacts/pam-d-common-auth-2204":                    linuxCloudInitArtifactsPamDCommonAuth2204,
	"linux/cloud-init/artifacts/pam-d-common-password":                     linuxCloudInitArtifactsPamDCommonPassword,
	"linux/cloud-init/artifacts/pam-d-su":                                  linuxCloudInitArtifactsPamDSu,
	"linux/cloud-init/artifacts/profile-d-cis.sh":                          linuxCloudInitArtifactsProfileDCisSh,
	"linux/cloud-init/artifacts/pwquality-CIS.conf":                        linuxCloudInitArtifactsPwqualityCisConf,
	"linux/cloud-init/artifacts/reconcile-private-hosts.service":           linuxCloudInitArtifactsReconcilePrivateHostsService,
	"linux/cloud-init/artifacts/reconcile-private-hosts.sh":                linuxCloudInitArtifactsReconcilePrivateHostsSh,
	"linux/cloud-init/artifacts/rsyslog-d-60-CIS.conf":                     linuxCloudInitArtifactsRsyslogD60CisConf,
	"linux/cloud-init/artifacts/setup-custom-search-domains.sh":            linuxCloudInitArtifactsSetupCustomSearchDomainsSh,
	"linux/cloud-init/artifacts/sshd_config":                               linuxCloudInitArtifactsSshd_config,
	"linux/cloud-init/artifacts/sshd_config_1604":                          linuxCloudInitArtifactsSshd_config_1604,
	"linux/cloud-init/artifacts/sshd_config_1804_fips":                     linuxCloudInitArtifactsSshd_config_1804_fips,
	"linux/cloud-init/artifacts/sync-container-logs.service":               linuxCloudInitArtifactsSyncContainerLogsService,
	"linux/cloud-init/artifacts/sync-container-logs.sh":                    linuxCloudInitArtifactsSyncContainerLogsSh,
	"linux/cloud-init/artifacts/sysctl-d-60-CIS.conf":                      linuxCloudInitArtifactsSysctlD60CisConf,
	"linux/cloud-init/artifacts/teleportd.service":                         linuxCloudInitArtifactsTeleportdService,
	"linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh":              linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh,
	"linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh":              linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh,
	"linux/cloud-init/artifacts/update_certs.path":                         linuxCloudInitArtifactsUpdate_certsPath,
	"linux/cloud-init/artifacts/update_certs.service":                      linuxCloudInitArtifactsUpdate_certsService,
	"linux/cloud-init/artifacts/update_certs.sh":                           linuxCloudInitArtifactsUpdate_certsSh,
	"linux/cloud-init/nodecustomdata.yml":                                  linuxCloudInitNodecustomdataYml,
	"windows/csecmd.ps1":                                                   windowsCsecmdPs1,
	"windows/kuberneteswindowssetup.ps1":                                   windowsKuberneteswindowssetupPs1,
	"windows/sendlogs.ps1":                                                 windowsSendlogsPs1,
	"windows/windowscsehelper.ps1":                                         windowsWindowscsehelperPs1,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"linux": &bintree{nil, map[string]*bintree{
		"cloud-init": &bintree{nil, map[string]*bintree{
			"artifacts": &bintree{nil, map[string]*bintree{
				"10-bindmount.conf":                         &bintree{linuxCloudInitArtifacts10BindmountConf, map[string]*bintree{}},
				"10-cgroupv2.conf":                          &bintree{linuxCloudInitArtifacts10Cgroupv2Conf, map[string]*bintree{}},
				"10-componentconfig.conf":                   &bintree{linuxCloudInitArtifacts10ComponentconfigConf, map[string]*bintree{}},
				"10-containerd.conf":                        &bintree{linuxCloudInitArtifacts10ContainerdConf, map[string]*bintree{}},
				"10-httpproxy.conf":                         &bintree{linuxCloudInitArtifacts10HttpproxyConf, map[string]*bintree{}},
				"10-tlsbootstrap.conf":                      &bintree{linuxCloudInitArtifacts10TlsbootstrapConf, map[string]*bintree{}},
				"aks-logrotate-override.conf":               &bintree{linuxCloudInitArtifactsAksLogrotateOverrideConf, map[string]*bintree{}},
				"aks-logrotate.service":                     &bintree{linuxCloudInitArtifactsAksLogrotateService, map[string]*bintree{}},
				"aks-logrotate.sh":                          &bintree{linuxCloudInitArtifactsAksLogrotateSh, map[string]*bintree{}},
				"aks-logrotate.timer":                       &bintree{linuxCloudInitArtifactsAksLogrotateTimer, map[string]*bintree{}},
				"aks-rsyslog":                               &bintree{linuxCloudInitArtifactsAksRsyslog, map[string]*bintree{}},
				"apt-preferences":                           &bintree{linuxCloudInitArtifactsAptPreferences, map[string]*bintree{}},
				"bind-mount.service":                        &bintree{linuxCloudInitArtifactsBindMountService, map[string]*bintree{}},
				"bind-mount.sh":                             &bintree{linuxCloudInitArtifactsBindMountSh, map[string]*bintree{}},
				"block_wireserver.sh":                       &bintree{linuxCloudInitArtifactsBlock_wireserverSh, map[string]*bintree{}},
				"cgroup-memory-telemetry.service":           &bintree{linuxCloudInitArtifactsCgroupMemoryTelemetryService, map[string]*bintree{}},
				"cgroup-memory-telemetry.sh":                &bintree{linuxCloudInitArtifactsCgroupMemoryTelemetrySh, map[string]*bintree{}},
				"cgroup-memory-telemetry.timer":             &bintree{linuxCloudInitArtifactsCgroupMemoryTelemetryTimer, map[string]*bintree{}},
				"cgroup-pressure-telemetry.service":         &bintree{linuxCloudInitArtifactsCgroupPressureTelemetryService, map[string]*bintree{}},
				"cgroup-pressure-telemetry.sh":              &bintree{linuxCloudInitArtifactsCgroupPressureTelemetrySh, map[string]*bintree{}},
				"cgroup-pressure-telemetry.timer":           &bintree{linuxCloudInitArtifactsCgroupPressureTelemetryTimer, map[string]*bintree{}},
				"ci-syslog-watcher.path":                    &bintree{linuxCloudInitArtifactsCiSyslogWatcherPath, map[string]*bintree{}},
				"ci-syslog-watcher.service":                 &bintree{linuxCloudInitArtifactsCiSyslogWatcherService, map[string]*bintree{}},
				"ci-syslog-watcher.sh":                      &bintree{linuxCloudInitArtifactsCiSyslogWatcherSh, map[string]*bintree{}},
				"cis.sh":                                    &bintree{linuxCloudInitArtifactsCisSh, map[string]*bintree{}},
				"containerd-monitor.service":                &bintree{linuxCloudInitArtifactsContainerdMonitorService, map[string]*bintree{}},
				"containerd-monitor.timer":                  &bintree{linuxCloudInitArtifactsContainerdMonitorTimer, map[string]*bintree{}},
				"containerd.service":                        &bintree{linuxCloudInitArtifactsContainerdService, map[string]*bintree{}},
				"containerd_exec_start.conf":                &bintree{linuxCloudInitArtifactsContainerd_exec_startConf, map[string]*bintree{}},
				"crictl.yaml":                               &bintree{linuxCloudInitArtifactsCrictlYaml, map[string]*bintree{}},
				"cse_cmd.sh":                                &bintree{linuxCloudInitArtifactsCse_cmdSh, map[string]*bintree{}},
				"cse_config.sh":                             &bintree{linuxCloudInitArtifactsCse_configSh, map[string]*bintree{}},
				"cse_helpers.sh":                            &bintree{linuxCloudInitArtifactsCse_helpersSh, map[string]*bintree{}},
				"cse_install.sh":                            &bintree{linuxCloudInitArtifactsCse_installSh, map[string]*bintree{}},
				"cse_main.sh":                               &bintree{linuxCloudInitArtifactsCse_mainSh, map[string]*bintree{}},
				"cse_redact_cloud_config.py":                &bintree{linuxCloudInitArtifactsCse_redact_cloud_configPy, map[string]*bintree{}},
				"cse_send_logs.py":                          &bintree{linuxCloudInitArtifactsCse_send_logsPy, map[string]*bintree{}},
				"cse_start.sh":                              &bintree{linuxCloudInitArtifactsCse_startSh, map[string]*bintree{}},
				"dhcpv6.service":                            &bintree{linuxCloudInitArtifactsDhcpv6Service, map[string]*bintree{}},
				"disk_queue.service":                        &bintree{linuxCloudInitArtifactsDisk_queueService, map[string]*bintree{}},
				"docker-monitor.service":                    &bintree{linuxCloudInitArtifactsDockerMonitorService, map[string]*bintree{}},
				"docker-monitor.timer":                      &bintree{linuxCloudInitArtifactsDockerMonitorTimer, map[string]*bintree{}},
				"docker_clear_mount_propagation_flags.conf": &bintree{linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf, map[string]*bintree{}},
				"enable-dhcpv6.sh":                          &bintree{linuxCloudInitArtifactsEnableDhcpv6Sh, map[string]*bintree{}},
				"ensure-no-dup.service":                     &bintree{linuxCloudInitArtifactsEnsureNoDupService, map[string]*bintree{}},
				"ensure-no-dup.sh":                          &bintree{linuxCloudInitArtifactsEnsureNoDupSh, map[string]*bintree{}},
				"etc-issue":                                 &bintree{linuxCloudInitArtifactsEtcIssue, map[string]*bintree{}},
				"etc-issue.net":                             &bintree{linuxCloudInitArtifactsEtcIssueNet, map[string]*bintree{}},
				"health-monitor.sh":                         &bintree{linuxCloudInitArtifactsHealthMonitorSh, map[string]*bintree{}},
				"init-aks-custom-cloud-mariner.sh":          &bintree{linuxCloudInitArtifactsInitAksCustomCloudMarinerSh, map[string]*bintree{}},
				"init-aks-custom-cloud.sh":                  &bintree{linuxCloudInitArtifactsInitAksCustomCloudSh, map[string]*bintree{}},
				"ipv6_nftables":                             &bintree{linuxCloudInitArtifactsIpv6_nftables, map[string]*bintree{}},
				"ipv6_nftables.service":                     &bintree{linuxCloudInitArtifactsIpv6_nftablesService, map[string]*bintree{}},
				"ipv6_nftables.sh":                          &bintree{linuxCloudInitArtifactsIpv6_nftablesSh, map[string]*bintree{}},
				"kms.service":                               &bintree{linuxCloudInitArtifactsKmsService, map[string]*bintree{}},
				"kubelet-monitor.service":                   &bintree{linuxCloudInitArtifactsKubeletMonitorService, map[string]*bintree{}},
				"kubelet-monitor.timer":                     &bintree{linuxCloudInitArtifactsKubeletMonitorTimer, map[string]*bintree{}},
				"kubelet.service":                           &bintree{linuxCloudInitArtifactsKubeletService, map[string]*bintree{}},
				"manifest.json":                             &bintree{linuxCloudInitArtifactsManifestJson, map[string]*bintree{}},
				"mariner": &bintree{nil, map[string]*bintree{
					"cse_helpers_mariner.sh":       &bintree{linuxCloudInitArtifactsMarinerCse_helpers_marinerSh, map[string]*bintree{}},
					"cse_install_mariner.sh":       &bintree{linuxCloudInitArtifactsMarinerCse_install_marinerSh, map[string]*bintree{}},
					"pam-d-system-auth":            &bintree{linuxCloudInitArtifactsMarinerPamDSystemAuth, map[string]*bintree{}},
					"pam-d-system-password":        &bintree{linuxCloudInitArtifactsMarinerPamDSystemPassword, map[string]*bintree{}},
					"update_certs_mariner.service": &bintree{linuxCloudInitArtifactsMarinerUpdate_certs_marinerService, map[string]*bintree{}},
				}},
				"mig-partition.service":           &bintree{linuxCloudInitArtifactsMigPartitionService, map[string]*bintree{}},
				"mig-partition.sh":                &bintree{linuxCloudInitArtifactsMigPartitionSh, map[string]*bintree{}},
				"modprobe-CIS.conf":               &bintree{linuxCloudInitArtifactsModprobeCisConf, map[string]*bintree{}},
				"nvidia-device-plugin.service":    &bintree{linuxCloudInitArtifactsNvidiaDevicePluginService, map[string]*bintree{}},
				"nvidia-docker-daemon.json":       &bintree{linuxCloudInitArtifactsNvidiaDockerDaemonJson, map[string]*bintree{}},
				"nvidia-modprobe.service":         &bintree{linuxCloudInitArtifactsNvidiaModprobeService, map[string]*bintree{}},
				"pam-d-common-auth":               &bintree{linuxCloudInitArtifactsPamDCommonAuth, map[string]*bintree{}},
				"pam-d-common-auth-2204":          &bintree{linuxCloudInitArtifactsPamDCommonAuth2204, map[string]*bintree{}},
				"pam-d-common-password":           &bintree{linuxCloudInitArtifactsPamDCommonPassword, map[string]*bintree{}},
				"pam-d-su":                        &bintree{linuxCloudInitArtifactsPamDSu, map[string]*bintree{}},
				"profile-d-cis.sh":                &bintree{linuxCloudInitArtifactsProfileDCisSh, map[string]*bintree{}},
				"pwquality-CIS.conf":              &bintree{linuxCloudInitArtifactsPwqualityCisConf, map[string]*bintree{}},
				"reconcile-private-hosts.service": &bintree{linuxCloudInitArtifactsReconcilePrivateHostsService, map[string]*bintree{}},
				"reconcile-private-hosts.sh":      &bintree{linuxCloudInitArtifactsReconcilePrivateHostsSh, map[string]*bintree{}},
				"rsyslog-d-60-CIS.conf":           &bintree{linuxCloudInitArtifactsRsyslogD60CisConf, map[string]*bintree{}},
				"setup-custom-search-domains.sh":  &bintree{linuxCloudInitArtifactsSetupCustomSearchDomainsSh, map[string]*bintree{}},
				"sshd_config":                     &bintree{linuxCloudInitArtifactsSshd_config, map[string]*bintree{}},
				"sshd_config_1604":                &bintree{linuxCloudInitArtifactsSshd_config_1604, map[string]*bintree{}},
				"sshd_config_1804_fips":           &bintree{linuxCloudInitArtifactsSshd_config_1804_fips, map[string]*bintree{}},
				"sync-container-logs.service":     &bintree{linuxCloudInitArtifactsSyncContainerLogsService, map[string]*bintree{}},
				"sync-container-logs.sh":          &bintree{linuxCloudInitArtifactsSyncContainerLogsSh, map[string]*bintree{}},
				"sysctl-d-60-CIS.conf":            &bintree{linuxCloudInitArtifactsSysctlD60CisConf, map[string]*bintree{}},
				"teleportd.service":               &bintree{linuxCloudInitArtifactsTeleportdService, map[string]*bintree{}},
				"ubuntu": &bintree{nil, map[string]*bintree{
					"cse_helpers_ubuntu.sh": &bintree{linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh, map[string]*bintree{}},
					"cse_install_ubuntu.sh": &bintree{linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh, map[string]*bintree{}},
				}},
				"update_certs.path":    &bintree{linuxCloudInitArtifactsUpdate_certsPath, map[string]*bintree{}},
				"update_certs.service": &bintree{linuxCloudInitArtifactsUpdate_certsService, map[string]*bintree{}},
				"update_certs.sh":      &bintree{linuxCloudInitArtifactsUpdate_certsSh, map[string]*bintree{}},
			}},
			"nodecustomdata.yml": &bintree{linuxCloudInitNodecustomdataYml, map[string]*bintree{}},
		}},
	}},
	"windows": &bintree{nil, map[string]*bintree{
		"csecmd.ps1":                 &bintree{windowsCsecmdPs1, map[string]*bintree{}},
		"kuberneteswindowssetup.ps1": &bintree{windowsKuberneteswindowssetupPs1, map[string]*bintree{}},
		"sendlogs.ps1":               &bintree{windowsSendlogsPs1, map[string]*bintree{}},
		"windowscsehelper.ps1":       &bintree{windowsWindowscsehelperPs1, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
