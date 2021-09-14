// Code generated for package templates by go-bindata DO NOT EDIT. (@generated)
// sources:
// linux/cloud-init/artifacts/10-bindmount.conf
// linux/cloud-init/artifacts/10-componentconfig.conf
// linux/cloud-init/artifacts/10-containerd.conf
// linux/cloud-init/artifacts/10-httpproxy.conf
// linux/cloud-init/artifacts/10-tlsbootstrap.conf
// linux/cloud-init/artifacts/apt-preferences
// linux/cloud-init/artifacts/bind-mount.service
// linux/cloud-init/artifacts/bind-mount.sh
// linux/cloud-init/artifacts/cis.sh
// linux/cloud-init/artifacts/containerd-monitor.service
// linux/cloud-init/artifacts/containerd-monitor.timer
// linux/cloud-init/artifacts/cse_cmd.sh
// linux/cloud-init/artifacts/cse_config.sh
// linux/cloud-init/artifacts/cse_helpers.sh
// linux/cloud-init/artifacts/cse_install.sh
// linux/cloud-init/artifacts/cse_main.sh
// linux/cloud-init/artifacts/cse_start.sh
// linux/cloud-init/artifacts/dhcpv6.service
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
// linux/cloud-init/artifacts/kms.service
// linux/cloud-init/artifacts/krustlet.service
// linux/cloud-init/artifacts/kubelet-monitor.service
// linux/cloud-init/artifacts/kubelet-monitor.timer
// linux/cloud-init/artifacts/kubelet.service
// linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh
// linux/cloud-init/artifacts/mariner/cse_install_mariner.sh
// linux/cloud-init/artifacts/mig-partition.service
// linux/cloud-init/artifacts/mig-partition.sh
// linux/cloud-init/artifacts/modprobe-CIS.conf
// linux/cloud-init/artifacts/nvidia-device-plugin.service
// linux/cloud-init/artifacts/nvidia-docker-daemon.json
// linux/cloud-init/artifacts/nvidia-modprobe.service
// linux/cloud-init/artifacts/pam-d-common-auth
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
// linux/cloud-init/artifacts/sysctl-d-60-CIS.conf
// linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh
// linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh
// linux/cloud-init/nodecustomdata.yml
// windows/containerdtemplate.toml
// windows/csecmd.ps1
// windows/kuberneteswindowsfunctions.ps1
// windows/kuberneteswindowssetup.ps1
// windows/windowsazurecnifunc.ps1
// windows/windowsazurecnifunc.tests.ps1
// windows/windowscalicofunc.ps1
// windows/windowscnifunc.ps1
// windows/windowsconfigfunc.ps1
// windows/windowscontainerdfunc.ps1
// windows/windowscsehelper.ps1
// windows/windowscsiproxyfunc.ps1
// windows/windowshostsconfigagentfunc.ps1
// windows/windowsinstallopensshfunc.ps1
// windows/windowskubeletfunc.ps1
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

var _linuxCloudInitArtifacts10ComponentconfigConf = []byte(`[Service]
Environment=KUBELET_CONFIG_FILE_FLAGS="--config /etc/default/kubeletconfig.json"
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
Environment=KUBELET_CONTAINERD_FLAGS="--container-runtime=remote --runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock"
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
Environment=KUBELET_TLS_BOOTSTRAP_FLAGS="--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
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

{{if eq GetKubeletDiskType "Temporary"}}
MOUNT_POINT="/mnt/aks"
{{end}}

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

var _linuxCloudInitArtifactsCisSh = []byte(`#!/bin/bash

assignRootPW() {
  if grep '^root:[!*]:' /etc/shadow; then
    SALT=$(openssl rand -base64 5)
    SECRET=$(openssl rand -base64 37)
    CMD="import crypt, getpass, pwd; print crypt.crypt('$SECRET', '\$6\$$SALT\$')"
    HASH=$(python -c "$CMD")

    echo 'root:'$HASH | /usr/sbin/chpasswd -e || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
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
      chmod 0600 $filepath || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    done
}

setPWExpiration() {
  sed -i "s|PASS_MAX_DAYS||g" /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'PASS_MAX_DAYS' /etc/login.defs && exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  sed -i "s|PASS_MIN_DAYS||g" /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'PASS_MIN_DAYS' /etc/login.defs && exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  sed -i "s|INACTIVE=||g" /etc/default/useradd || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'INACTIVE=' /etc/default/useradd && exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  echo 'PASS_MAX_DAYS 90' >> /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'PASS_MAX_DAYS 90' /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  echo 'PASS_MIN_DAYS 7' >> /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'PASS_MIN_DAYS 7' /etc/login.defs || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  echo 'INACTIVE=30' >> /etc/default/useradd || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
  grep 'INACTIVE=30' /etc/default/useradd || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
}

applyCIS() {
  setPWExpiration
  assignRootPW
  assignFilePermissions
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

var _linuxCloudInitArtifactsCse_cmdSh = []byte(`echo $(date),$(hostname) > /var/log/azure/cluster-provision-cse-output.log;
{{GetVariable "outBoundCmd"}}
for i in $(seq 1 1200); do
grep -Fq "EOF" /opt/azure/containers/provision.sh && break;
if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi;
done;
{{if IsAKSCustomCloud}}
for i in $(seq 1 1200); do
grep -Fq "EOF" {{GetInitAKSCustomCloudFilepath}} && break;
if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi;
done;
{{GetInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
ADMINUSER={{GetParameter "linuxAdminUsername"}}
MOBY_VERSION={{GetParameter "mobyVersion"}}
TENANT_ID={{GetVariable "tenantID"}}
KUBERNETES_VERSION={{GetParameter "kubernetesVersion"}}
HYPERKUBE_URL={{GetParameter "kubernetesHyperkubeSpec"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
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
SERVICE_PRINCIPAL_CLIENT_SECRET='{{GetParameter "servicePrincipalClientSecret"}}'
KUBELET_PRIVATE_KEY={{GetParameter "clientPrivateKey"}}
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
RUNC_VERSION={{GetParameter "runcVersion"}}
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

{{- if EnableHostsConfigAgent}}
configPrivateClusterHosts() {
  systemctlEnableAndStart reconcile-private-hosts || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- end}}

ensureRPC() {
    systemctlEnableAndStart rpcbind || exit $ERR_SYSTEMCTL_START_FAIL
    systemctlEnableAndStart rpc-statd || exit $ERR_SYSTEMCTL_START_FAIL
}

{{- if ShouldConfigTransparentHugePage}}
configureTransparentHugePage() {
    ETC_SYSFS_CONF="/etc/sysfs.conf"
    THP_ENABLED={{GetTransparentHugePageEnabled}}
    if [[ "${THP_ENABLED}" != "" ]]; then
        echo "${THP_ENABLED}" > /sys/kernel/mm/transparent_hugepage/enabled
        echo "kernel/mm/transparent_hugepage/enabled=${THP_ENABLED}" >> ${ETC_SYSFS_CONF}
    fi
    THP_DEFRAG={{GetTransparentHugePageDefrag}}
    if [[ "${THP_DEFRAG}" != "" ]]; then
        echo "${THP_DEFRAG}" > /sys/kernel/mm/transparent_hugepage/defrag
        echo "kernel/mm/transparent_hugepage/defrag=${THP_DEFRAG}" >> ${ETC_SYSFS_CONF}
    fi
}
{{- end}}

{{- if ShouldConfigSwapFile}}
configureSwapFile() {
    SWAP_SIZE_KB=$(expr {{GetSwapFileSizeMB}} \* 1000)
    DISK_FREE_KB=$(df /dev/sdb1 | sed 1d | awk '{print $4}')
    if [[ ${DISK_FREE_KB} -gt ${SWAP_SIZE_KB} ]]; then
        SWAP_LOCATION=/mnt/swapfile
        retrycmd_if_failure 24 5 25 fallocate -l ${SWAP_SIZE_KB}K ${SWAP_LOCATION} || exit $ERR_SWAP_CREAT_FAIL
        chmod 600 ${SWAP_LOCATION}
        retrycmd_if_failure 24 5 25 mkswap ${SWAP_LOCATION} || exit $ERR_SWAP_CREAT_FAIL
        retrycmd_if_failure 24 5 25 swapon ${SWAP_LOCATION} || exit $ERR_SWAP_CREAT_FAIL
        retrycmd_if_failure 24 5 25 swapon --show | grep ${SWAP_LOCATION} || exit $ERR_SWAP_CREAT_FAIL
        echo "${SWAP_LOCATION} none swap sw 0 0" >> /etc/fstab
    else
        echo "Insufficient disk space creating swap file: request ${SWAP_SIZE_KB} free ${DISK_FREE_KB}"
        exit $ERR_SWAP_CREAT_INSUFFICIENT_DISK_SPACE
    fi
}
{{- end}}

{{- if ShouldConfigureHTTPProxy}}
configureEtcEnvironment() {
    {{- if HasHTTPProxy }}
    echo 'HTTP_PROXY="{{GetHTTPProxy}}"' >> /etc/environment
    echo 'http_proxy="{{GetHTTPProxy}}"' >> /etc/environment
    {{- end}}
    {{- if HasHTTPSProxy }}
    echo 'HTTPS_PROXY="{{GetHTTPSProxy}}"' >> /etc/environment
    echo 'https_proxy="{{GetHTTPSProxy}}"' >> /etc/environment
    {{- end}}
    {{- if HasNoProxy }}
    echo 'NO_PROXY="{{GetNoProxy}}"' >> /etc/environment
    echo 'no_proxy="{{GetNoProxy}}"' >> /etc/environment
    {{- end}}
}
{{- end}}

{{- if ShouldConfigureHTTPProxyCA}}
configureHTTPProxyCA() {
    openssl x509 -outform pem -in /usr/local/share/ca-certificates/proxyCA.pem -out /usr/local/share/ca-certificates/proxyCA.crt || exit $ERR_HTTP_PROXY_CA_CONVERT
    rm -f /usr/local/share/ca-certificates/proxyCA.pem
    update-ca-certificates || exit $ERR_HTTP_PROXY_CA_UPDATE
}
{{- end}}

configureKubeletServerCert() {
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}"
}

configureK8s() {
    KUBELET_PRIVATE_KEY_PATH="/etc/kubernetes/certs/client.key"
    touch "${KUBELET_PRIVATE_KEY_PATH}"
    chmod 0600 "${KUBELET_PRIVATE_KEY_PATH}"
    chown root:root "${KUBELET_PRIVATE_KEY_PATH}"

    APISERVER_PUBLIC_KEY_PATH="/etc/kubernetes/certs/apiserver.crt"
    touch "${APISERVER_PUBLIC_KEY_PATH}"
    chmod 0644 "${APISERVER_PUBLIC_KEY_PATH}"
    chown root:root "${APISERVER_PUBLIC_KEY_PATH}"

    AZURE_JSON_PATH="/etc/kubernetes/azure.json"
    touch "${AZURE_JSON_PATH}"
    chmod 0600 "${AZURE_JSON_PATH}"
    chown root:root "${AZURE_JSON_PATH}"

    set +x
    echo "${KUBELET_PRIVATE_KEY}" | base64 --decode > "${KUBELET_PRIVATE_KEY_PATH}"
    echo "${APISERVER_PUBLIC_KEY}" | base64 --decode > "${APISERVER_PUBLIC_KEY_PATH}"
    {{/* Perform the required JSON escaping */}}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\\/\\\\}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\"/\\\"}
    cat << EOF > "${AZURE_JSON_PATH}"
{
    {{- if IsAKSCustomCloud}}
    "cloud": "AzureStackCloud",
    {{- else}}
    "cloud": "{{GetTargetEnvironment}}",
    {{- end}}
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
{{- if IsAKSCustomCloud}}
    set +x
    AKS_CUSTOM_CLOUD_JSON_PATH="/etc/kubernetes/{{GetTargetEnvironment}}.json"
    touch "${AKS_CUSTOM_CLOUD_JSON_PATH}"
    chmod 0600 "${AKS_CUSTOM_CLOUD_JSON_PATH}"
    chown root:root "${AKS_CUSTOM_CLOUD_JSON_PATH}"

    cat << EOF > "${AKS_CUSTOM_CLOUD_JSON_PATH}"
{
    "name": "{{GetTargetEnvironment}}",
    "managementPortalURL": "{{AKSCustomCloudManagementPortalURL}}",
    "publishSettingsURL": "{{AKSCustomCloudPublishSettingsURL}}",
    "serviceManagementEndpoint": "{{AKSCustomCloudServiceManagementEndpoint}}",
    "resourceManagerEndpoint": "{{AKSCustomCloudResourceManagerEndpoint}}",
    "activeDirectoryEndpoint": "{{AKSCustomCloudActiveDirectoryEndpoint}}",
    "galleryEndpoint": "{{AKSCustomCloudGalleryEndpoint}}",
    "keyVaultEndpoint": "{{AKSCustomCloudKeyVaultEndpoint}}",
    "graphEndpoint": "{{AKSCustomCloudGraphEndpoint}}",
    "serviceBusEndpoint": "{{AKSCustomCloudServiceBusEndpoint}}",
    "batchManagementEndpoint": "{{AKSCustomCloudBatchManagementEndpoint}}",
    "storageEndpointSuffix": "{{AKSCustomCloudStorageEndpointSuffix}}",
    "sqlDatabaseDNSSuffix": "{{AKSCustomCloudSqlDatabaseDNSSuffix}}",
    "trafficManagerDNSSuffix": "{{AKSCustomCloudTrafficManagerDNSSuffix}}",
    "keyVaultDNSSuffix": "{{AKSCustomCloudKeyVaultDNSSuffix}}",
    "serviceBusEndpointSuffix": "{{AKSCustomCloudServiceBusEndpointSuffix}}",
    "serviceManagementVMDNSSuffix": "{{AKSCustomCloudServiceManagementVMDNSSuffix}}",
    "resourceManagerVMDNSSuffix": "{{AKSCustomCloudResourceManagerVMDNSSuffix}}",
    "containerRegistryDNSSuffix": "{{AKSCustomCloudContainerRegistryDNSSuffix}}",
    "cosmosDBDNSSuffix": "{{AKSCustomCloudCosmosDBDNSSuffix}}",
    "tokenAudience": "{{AKSCustomCloudTokenAudience}}",
    "resourceIdentifiers": {
        "graph": "{{AKSCustomCloudResourceIdentifiersGraph}}",
        "keyVault": "{{AKSCustomCloudResourceIdentifiersKeyVault}}",
        "datalake": "{{AKSCustomCloudResourceIdentifiersDatalake}}",
        "batch": "{{AKSCustomCloudResourceIdentifiersBatch}}",
        "operationalInsights": "{{AKSCustomCloudResourceIdentifiersOperationalInsights}}",
        "storage": "{{AKSCustomCloudResourceIdentifiersStorage}}"
    }
}
EOF
    set -x
{{end}}

{{- if IsKubeletConfigFileEnabled}}
    set +x
    KUBELET_CONFIG_JSON_PATH="/etc/default/kubeletconfig.json"
    touch "${KUBELET_CONFIG_JSON_PATH}"
    chmod 0644 "${KUBELET_CONFIG_JSON_PATH}"
    chown root:root "${KUBELET_CONFIG_JSON_PATH}"
    cat << EOF > "${KUBELET_CONFIG_JSON_PATH}"
{{GetKubeletConfigFileContent}}
EOF
    set -x
{{- end}}
}

configureCNI() {
    {{/* needed for the iptables rules to work on bridges */}}
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

disable1804SystemdResolved() {
    ls -ltr /etc/resolv.conf
    cat /etc/resolv.conf
    {{- if Disable1804SystemdResolved}}
    UBUNTU_RELEASE=$(lsb_release -r -s)
    if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
        echo "Ingorings systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
    fi
    {{- else}}
    echo "Disable1804SystemdResolved is false. Skipping."
    {{- end}}
}

{{- if NeedsContainerd}}
ensureContainerd() {
  {{- if TeleportEnabled}}
  ensureTeleportd
  {{- end}}
  wait_for_file 1200 1 /etc/systemd/system/containerd.service.d/exec_start.conf || exit $ERR_FILE_WATCH_TIMEOUT
  wait_for_file 1200 1 /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  wait_for_file 1200 1 /etc/sysctl.d/11-containerd.conf || exit $ERR_FILE_WATCH_TIMEOUT
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD
  systemctl is-active --quiet docker && (systemctl_disable 20 30 120 docker || exit $ERR_SYSTEMD_DOCKER_STOP_FAIL)
  systemctlEnableAndStart containerd || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- if and IsKubenet (not HasCalicoNetworkPolicy)}}
ensureNoDupOnPromiscuBridge() {
    wait_for_file 1200 1 /opt/azure/containers/ensure-no-dup.sh || exit $ERR_FILE_WATCH_TIMEOUT
    wait_for_file 1200 1 /etc/systemd/system/ensure-no-dup.service || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart ensure-no-dup || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- end}}
{{- if TeleportEnabled}}
ensureTeleportd() {
    wait_for_file 1200 1 /etc/systemd/system/teleportd.service || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart teleportd || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- end}}
{{- else}}
ensureDocker() {
    DOCKER_SERVICE_EXEC_START_FILE=/etc/systemd/system/docker.service.d/exec_start.conf
    wait_for_file 1200 1 $DOCKER_SERVICE_EXEC_START_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    usermod -aG docker ${ADMINUSER}
    DOCKER_MOUNT_FLAGS_SYSTEMD_FILE=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
    wait_for_file 1200 1 $DOCKER_MOUNT_FLAGS_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
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
{{- end}}
{{- if NeedsContainerd}}
ensureMonitorService() {
    {{/* Delay start of containerd-monitor for 30 mins after booting */}}
    CONTAINERD_MONITOR_SYSTEMD_TIMER_FILE=/etc/systemd/system/containerd-monitor.timer
    wait_for_file 1200 1 $CONTAINERD_MONITOR_SYSTEMD_TIMER_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    CONTAINERD_MONITOR_SYSTEMD_FILE=/etc/systemd/system/containerd-monitor.service
    wait_for_file 1200 1 $CONTAINERD_MONITOR_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart containerd-monitor.timer || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- else}}
ensureMonitorService() {
    {{/* Delay start of docker-monitor for 30 mins after booting */}}
    DOCKER_MONITOR_SYSTEMD_TIMER_FILE=/etc/systemd/system/docker-monitor.timer
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_TIMER_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    DOCKER_MONITOR_SYSTEMD_FILE=/etc/systemd/system/docker-monitor.service
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart docker-monitor.timer || exit $ERR_SYSTEMCTL_START_FAIL
}
{{- end}}

{{if IsIPv6DualStackFeatureEnabled}}
ensureDHCPv6() {
    wait_for_file 3600 1 {{GetDHCPv6ServiceCSEScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
    wait_for_file 3600 1 {{GetDHCPv6ConfigCSEScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart dhcpv6 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}
{{end}}

ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    wait_for_file 1200 1 $KUBELET_DEFAULT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    {{if IsKubeletClientTLSBootstrappingEnabled -}}
    BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
    wait_for_file 1200 1 $BOOTSTRAP_KUBECONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    {{- else -}}
    KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
    wait_for_file 1200 1 $KUBECONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    {{- end}}
    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    wait_for_file 1200 1 $KUBELET_RUNTIME_CONFIG_SCRIPT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    {{- if ShouldConfigureHTTPProxy}}
    configureEtcEnvironment
    {{- end}}
    systemctlEnableAndStart kubelet || exit $ERR_KUBELET_START_FAIL
    {{if HasAntreaNetworkPolicy}}
    while [ ! -f /etc/cni/net.d/10-antrea.conf ]; do
        sleep 3
    done
    {{end}}
    {{if HasFlannelNetworkPlugin}}
    while [ ! -f /etc/cni/net.d/10-flannel.conf ]; do
        sleep 3
    done
    {{end}}
}

ensureMigPartition(){
    systemctlEnableAndStart mig-partition || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureSysctl() {
    SYSCTL_CONFIG_FILE=/etc/sysctl.d/999-sysctl-aks.conf
    wait_for_file 1200 1 $SYSCTL_CONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    retrycmd_if_failure 24 5 25 sysctl --system
}

ensureJournal() {
    {
        echo "Storage=persistent"
        echo "SystemMaxUse=1G"
        echo "RuntimeMaxUse=1G"
        echo "ForwardToSyslog=yes"
    } >> /etc/systemd/journald.conf
    systemctlEnableAndStart systemd-journald || exit $ERR_SYSTEMCTL_START_FAIL
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
    wait_for_file 1200 1 $CLUSTER_AUTOSCALER_ADDON_FILE || exit $ERR_FILE_WATCH_TIMEOUT
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
    wait_for_file 1200 1 $ACI_CONNECTOR_ADDON_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    sed -i "s|<creds>|$ACI_CONNECTOR_CREDENTIALS|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<rgName>|$RESOURCE_GROUP|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<cert>|$ACI_CONNECTOR_CERT|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<key>|$ACI_CONNECTOR_KEY|g" $ACI_CONNECTOR_ADDON_FILE
}

configAzurePolicyAddon() {
    AZURE_POLICY_ADDON_FILE=/etc/kubernetes/addons/azure-policy-deployment.yaml
    sed -i "s|<resourceId>|/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP|g" $AZURE_POLICY_ADDON_FILE
}

{{if IsNSeriesSKU}}
installGPUDriversRun() {
    {{- /* there is no file under the module folder, the installation failed, so clean up the dirty directory
    when you upgrade the GPU driver version, please help check whether the retry installation issue is gone,
    if yes please help remove the clean up logic here too */}}
    set -x
    MODULE_NAME="nvidia"
    NVIDIA_DKMS_DIR="/var/lib/dkms/${MODULE_NAME}/${GPU_DV}"
    KERNEL_NAME=$(uname -r)
    if [ -d "${NVIDIA_DKMS_DIR}" ]; then
        if [ -x "$(command -v dkms)" ]; then
          dkms remove -m ${MODULE_NAME} -v ${GPU_DV} -k ${KERNEL_NAME}
        else
          rm -rf "${NVIDIA_DKMS_DIR}"
        fi
    fi
    {{- /* we need to append the date to the end of the file because the retry will override the log file */}}
    local log_file_name="/var/log/nvidia-installer-$(date +%s).log"
    if [ ! -f "${GPU_DEST}/nvidia-drivers-${GPU_DV}" ]; then
        installGPUDrivers
    fi
    sh $GPU_DEST/nvidia-drivers-$GPU_DV -s \
        -k=$KERNEL_NAME \
        --log-file-name=${log_file_name} \
        -a --no-drm --dkms --utility-prefix="${GPU_DEST}" --opengl-prefix="${GPU_DEST}"
    exit $?
}

configGPUDrivers() {
    {{/* only install the runtime since nvidia-docker2 has a hard dep on docker CE packages. */}}
    {{/* we will manually install nvidia-docker2 */}}
    rmmod nouveau
    echo blacklist nouveau >> /etc/modprobe.d/blacklist.conf
    retrycmd_if_failure_no_stats 120 5 25 update-initramfs -u || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    {{/* if the unattened upgrade is turned on, and it may takes 10 min to finish the installation, and we use the 1 second just to try to get the lock more aggressively */}}
    retrycmd_if_failure 600 1 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+${NVIDIA_DOCKER_SUFFIX}" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    tmpDir=$GPU_DEST/tmp
    (
      set -e -o pipefail
      cd "${tmpDir}"
      wait_for_apt_locks
      dpkg-deb -R ./nvidia-docker2*.deb "${tmpDir}/pkg" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
      cp -r ${tmpDir}/pkg/usr/* /usr/ || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    )
    rm -rf $GPU_DEST/tmp
    {{if NeedsContainerd}}
    retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    {{else}}
    retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    {{end}}
    mkdir -p $GPU_DEST/lib64 $GPU_DEST/overlay-workdir
    retrycmd_if_failure 120 5 25 mount -t overlay -o lowerdir=/usr/lib/x86_64-linux-gnu,upperdir=${GPU_DEST}/lib64,workdir=${GPU_DEST}/overlay-workdir none /usr/lib/x86_64-linux-gnu || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    export -f installGPUDriversRun
    retrycmd_if_failure 3 1 600 bash -c installGPUDriversRun || exit $ERR_GPU_DRIVERS_START_FAIL
    mv ${GPU_DEST}/bin/* /usr/bin
    echo "${GPU_DEST}/lib64" > /etc/ld.so.conf.d/nvidia.conf
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
    umount -l /usr/lib/x86_64-linux-gnu
    retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
}

validateGPUDrivers() {
    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL
    which nvidia-smi
    if [[ $? == 0 ]]; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 25 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 25 $GPU_DEST/bin/nvidia-smi)
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
    if [[ "${CONFIG_GPU_DRIVER_IF_NEEDED}" = true ]]; then
        configGPUDrivers
    else
        validateGPUDrivers
    fi
    systemctlEnableAndStart nvidia-modprobe || exit $ERR_GPU_DRIVERS_START_FAIL
}
{{end}}
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
{{/* ERR_SYSTEMCTL_ENABLE_FAIL=3 Service could not be enabled by systemctl -- DEPRECATED */}}
ERR_SYSTEMCTL_START_FAIL=4 {{/* Service could not be started or enabled by systemctl */}}
ERR_CLOUD_INIT_TIMEOUT=5 {{/* Timeout waiting for cloud-init runcmd to complete */}}
ERR_FILE_WATCH_TIMEOUT=6 {{/* Timeout waiting for a file */}}
ERR_HOLD_WALINUXAGENT=7 {{/* Unable to place walinuxagent apt package on hold during install */}}
ERR_RELEASE_HOLD_WALINUXAGENT=8 {{/* Unable to release hold on walinuxagent apt package after install */}}
ERR_APT_INSTALL_TIMEOUT=9 {{/* Timeout installing required apt packages */}}
ERR_DOCKER_INSTALL_TIMEOUT=20 {{/* Timeout waiting for docker install */}}
ERR_DOCKER_DOWNLOAD_TIMEOUT=21 {{/* Timout waiting for docker downloads */}}
ERR_DOCKER_KEY_DOWNLOAD_TIMEOUT=22 {{/* Timeout waiting to download docker repo key */}}
ERR_DOCKER_APT_KEY_TIMEOUT=23 {{/* Timeout waiting for docker apt-key */}}
ERR_DOCKER_START_FAIL=24 {{/* Docker could not be started by systemctl */}}
ERR_MOBY_APT_LIST_TIMEOUT=25 {{/* Timeout waiting for moby apt sources */}}
ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT=26 {{/* Timeout waiting for MS GPG key download */}}
ERR_MOBY_INSTALL_TIMEOUT=27 {{/* Timeout waiting for moby-docker install */}}
ERR_CONTAINERD_INSTALL_TIMEOUT=28 {{/* Timeout waiting for moby-containerd install */}}
ERR_CONTAINERD_INSTALL_FILE_NOT_FOUND=38 {{/* Unable to locate containerd debian pkg file */}}
ERR_RUNC_INSTALL_TIMEOUT=29 {{/* Timeout waiting for moby-runc install */}}
ERR_K8S_RUNNING_TIMEOUT=30 {{/* Timeout waiting for k8s cluster to be healthy */}}
ERR_K8S_DOWNLOAD_TIMEOUT=31 {{/* Timeout waiting for Kubernetes downloads */}}
ERR_KUBECTL_NOT_FOUND=32 {{/* kubectl client binary not found on local disk */}}
ERR_IMG_DOWNLOAD_TIMEOUT=33 {{/* Timeout waiting for img download */}}
ERR_KUBELET_START_FAIL=34 {{/* kubelet could not be started by systemctl */}}
ERR_DOCKER_IMG_PULL_TIMEOUT=35 {{/* Timeout trying to pull a Docker image */}}
ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT=36 {{/* Timeout trying to pull a containerd image via cli tool ctr */}}
ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT=37 {{/* Timeout trying to pull a containerd image via cli tool crictl */}}
ERR_CNI_DOWNLOAD_TIMEOUT=41 {{/* Timeout waiting for CNI downloads */}}
ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT=42 {{/* Timeout waiting for https://packages.microsoft.com/config/ubuntu/16.04/packages-microsoft-prod.deb */}}
ERR_MS_PROD_DEB_PKG_ADD_FAIL=43 {{/* Failed to add repo pkg file */}}
{{/* ERR_FLEXVOLUME_DOWNLOAD_TIMEOUT=44 Failed to add repo pkg file -- DEPRECATED */}}
ERR_SYSTEMD_INSTALL_FAIL=48 {{/* Unable to install required systemd version */}}
ERR_MODPROBE_FAIL=49 {{/* Unable to load a kernel module using modprobe */}}
ERR_OUTBOUND_CONN_FAIL=50 {{/* Unable to establish outbound connection */}}
ERR_K8S_API_SERVER_CONN_FAIL=51 {{/* Unable to establish connection to k8s api server*/}}
ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL=52 {{/* Unable to resolve k8s api server name */}}
ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL=53 {{/* Unable to resolve k8s api server name due to Azure DNS issue */}}
ERR_KATA_KEY_DOWNLOAD_TIMEOUT=60 {{/* Timeout waiting to download kata repo key */}}
ERR_KATA_APT_KEY_TIMEOUT=61 {{/* Timeout waiting for kata apt-key */}}
ERR_KATA_INSTALL_TIMEOUT=62 {{/* Timeout waiting for kata install */}}
ERR_CONTAINERD_DOWNLOAD_TIMEOUT=70 {{/* Timeout waiting for containerd downloads */}}
ERR_CUSTOM_SEARCH_DOMAINS_FAIL=80 {{/* Unable to configure custom search domains */}}
ERR_GPU_DRIVERS_START_FAIL=84 {{/* nvidia-modprobe could not be started by systemctl */}}
ERR_GPU_DRIVERS_INSTALL_TIMEOUT=85 {{/* Timeout waiting for GPU drivers install */}}
ERR_GPU_DEVICE_PLUGIN_START_FAIL=86 {{/* nvidia device plugin could not be started by systemctl */}}
ERR_GPU_INFO_ROM_CORRUPTED=87 {{/* info ROM corrupted error when executing nvidia-smi */}}
ERR_SGX_DRIVERS_INSTALL_TIMEOUT=90 {{/* Timeout waiting for SGX prereqs to download */}}
ERR_SGX_DRIVERS_START_FAIL=91 {{/* Failed to execute SGX driver binary */}}
ERR_APT_DAILY_TIMEOUT=98 {{/* Timeout waiting for apt daily updates */}}
ERR_APT_UPDATE_TIMEOUT=99 {{/* Timeout waiting for apt-get update to complete */}}
ERR_CSE_PROVISION_SCRIPT_NOT_READY_TIMEOUT=100 {{/* Timeout waiting for cloud-init to place this script on the vm */}}
ERR_APT_DIST_UPGRADE_TIMEOUT=101 {{/* Timeout waiting for apt-get dist-upgrade to complete */}}
ERR_APT_PURGE_FAIL=102 {{/* Error purging distro packages */}}
ERR_SYSCTL_RELOAD=103 {{/* Error reloading sysctl config */}}
ERR_CIS_ASSIGN_ROOT_PW=111 {{/* Error assigning root password in CIS enforcement */}}
ERR_CIS_ASSIGN_FILE_PERMISSION=112 {{/* Error assigning permission to a file in CIS enforcement */}}
ERR_PACKER_COPY_FILE=113 {{/* Error writing a file to disk during VHD CI */}}
ERR_CIS_APPLY_PASSWORD_CONFIG=115 {{/* Error applying CIS-recommended passwd configuration */}}
ERR_SYSTEMD_DOCKER_STOP_FAIL=116 {{/* Error stopping dockerd */}}
ERR_CRICTL_DOWNLOAD_TIMEOUT=117 {{/* Timeout waiting for crictl downloads */}}
ERR_CRICTL_OPERATION_ERROR=118 {{/* Error executing a crictl operation */}}
ERR_CTR_OPERATION_ERROR=119 {{/* Error executing a ctr containerd cli operation */}}

ERR_VHD_FILE_NOT_FOUND=124 {{/* VHD log file not found on VM built from VHD distro */}}
ERR_VHD_BUILD_ERROR=125 {{/* Reserved for VHD CI exit conditions */}}

{{/* Azure Stack specific errors */}}
ERR_AZURE_STACK_GET_ARM_TOKEN=120 {{/* Error generating a token to use with Azure Resource Manager */}}
ERR_AZURE_STACK_GET_NETWORK_CONFIGURATION=121 {{/* Error fetching the network configuration for the node */}}
ERR_AZURE_STACK_GET_SUBNET_PREFIX=122 {{/* Error fetching the subnet address prefix for a subnet ID */}}

ERR_SWAP_CREAT_FAIL=130 {{/* Error allocating swap file */}}
ERR_SWAP_CREAT_INSUFFICIENT_DISK_SPACE=131 {{/* Error insufficient disk space for swap file creation */}}

ERR_TELEPORTD_DOWNLOAD_ERR=150 {{/* Error downloading teleportd binary */}}
ERR_TELEPORTD_INSTALL_ERR=151 {{/* Error installing teleportd binary */}}

ERR_HTTP_PROXY_CA_CONVERT=160 {{/* Error converting http proxy ca cert from pem to crt format */}}
ERR_HTTP_PROXY_CA_UPDATE=161 {{/* Error updating ca certs to include http proxy ca */}}

ERR_DISBALE_IPTABLES=170 {{/* Error disabling iptables service */}}

ERR_MIG_PARTITION_FAILURE=180 {{/* Error creating MIG instances on MIG node */}}
ERR_KRUSTLET_DOWNLOAD_TIMEOUT=171 {{/* Timeout waiting for krustlet downloads */}}


OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
KUBECTL=/usr/local/bin/kubectl
DOCKER=/usr/bin/docker
export GPU_DV=470.57.02
export GPU_DEST=/usr/local/nvidia
NVIDIA_DOCKER_VERSION=2.0.3
DOCKER_VERSION=1.13.1-1
NVIDIA_CONTAINER_RUNTIME_VERSION=2.0.0
NVIDIA_DOCKER_SUFFIX=docker18.09.2-1

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout ${@} && break || \
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
            timeout 60 curl -fsSL $url -o $tarball
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
            timeout $timeout curl -fsSL $url -o $filepath
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
KRUSTLET_VERSION="v1.0.0-alpha.1"

cleanupContainerdDlFiles() {
    rm -rf $CONTAINERD_DOWNLOADS_DIR
}

installContainerRuntime() {
    {{if NeedsContainerd}}
        installStandaloneContainerd ${CONTAINERD_VERSION}
    {{else}}
        installMoby
    {{end}}
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

downloadKrustlet() {
    local krustlet_url="https://acs-mirror.azureedge.net/krustlet/${KRUSTLET_VERSION}/linux/amd64/krustlet-wasi"
    local krustlet_filepath="/usr/local/bin/krustlet-wasi"
    if [[ -f "$krustlet_filepath" ]]; then
        installed_version="$("$krustlet_filepath" --version | cut -d' ' -f2)"
        if [[ "${KRUSTLET_VERSION}" == "$installed_version" ]]; then
            echo "desired krustlet version exists on disk, skipping download."
            return
        fi
        rm -rf "$krustlet_filepath"
    fi
    retrycmd_if_failure 30 5 60 curl -fSL -o "$krustlet_filepath" "$krustlet_url" || exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT
    chmod 755 "$krustlet_filepath"
}

downloadAzureCNI() {
    mkdir -p $CNI_DOWNLOADS_DIR
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}
{{- if NeedsContainerd}}

downloadCrictl() {
    CRICTL_VERSION=$1
    mkdir -p $CRICTL_DOWNLOAD_DIR
    CRICTL_DOWNLOAD_URL="https://github.com/kubernetes-sigs/cri-tools/releases/download/v${CRICTL_VERSION}/crictl-v${CRICTL_VERSION}-linux-amd64.tar.gz"
    CRICTL_TGZ_TEMP=${CRICTL_DOWNLOAD_URL##*/}
    retrycmd_curl_file 10 5 60 "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ${CRICTL_DOWNLOAD_URL}
}

installCrictl() {
    currentVersion=$(crictl --version 2>/dev/null | sed 's/crictl version //g')
    local CRICTL_VERSION=${KUBERNETES_VERSION%.*}.0
    if [[ ${currentVersion} =~ ${CRICTL_VERSION} ]]; then
        echo "version ${currentVersion} of crictl already installed. skipping installCrictl of target version ${CRICTL_VERSION}"
    else
        # this is only called during cse. VHDs should have crictl binaries pre-cached so no need to download.
        # if the vhd does not have crictl pre-baked, return early
        CRICTL_TGZ_TEMP="crictl-v${CRICTL_VERSION}-linux-amd64.tar.gz"
        if [[ ! -f "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ]]; then
            rm -rf ${CRICTL_DOWNLOAD_DIR}
            echo "pre-cached crictl not found: skipping installCrictl"
            return 1
        fi
        echo "Unpacking crictl into ${CRICTL_BIN_DIR}"
        tar zxvf "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" -C ${CRICTL_BIN_DIR}
        chmod 755 $CRICTL_BIN_DIR/crictl
    fi
    rm -rf ${CRICTL_DOWNLOAD_DIR}
}
{{- if TeleportEnabled}}
downloadTeleportdPlugin() {
    DOWNLOAD_URL=$1
    TELEPORTD_VERSION=$2
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
{{- end}}
{{- end}}

installCNI() {
    CNI_TGZ_TMP=${CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    if [[ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]]; then
        downloadCNI
    fi
    mkdir -p $CNI_BIN_DIR
    tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_BIN_DIR
    chown -R root:root $CNI_BIN_DIR
    chmod -R 755 $CNI_BIN_DIR
}

installAzureCNI() {
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    if [[ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]]; then
        downloadAzureCNI
    fi
    mkdir -p $CNI_CONFIG_DIR
    chown -R root:root $CNI_CONFIG_DIR
    chmod 755 $CNI_CONFIG_DIR
    mkdir -p $CNI_BIN_DIR
    tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_BIN_DIR
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

extractHyperkube() {
    CLI_TOOL=$1
    path="/home/hyperkube-downloads/${KUBERNETES_VERSION}"
    pullContainerImage $CLI_TOOL ${HYPERKUBE_URL}
    mkdir -p "$path"
    if [[ "$CLI_TOOL" == "ctr" ]]; then
        if ctr --namespace k8s.io run --rm --mount type=bind,src=$path,dst=$path,options=bind:rw ${HYPERKUBE_URL} extractTask /bin/bash -c "cp /usr/local/bin/{kubelet,kubectl} $path"; then
            mv "$path/kubelet" "/usr/local/bin/kubelet-${KUBERNETES_VERSION}"
            mv "$path/kubectl" "/usr/local/bin/kubectl-${KUBERNETES_VERSION}"
        else
            ctr --namespace k8s.io run --rm --mount type=bind,src=$path,dst=$path,options=bind:rw ${HYPERKUBE_URL} extractTask /bin/bash -c "cp /hyperkube $path"
        fi

    else
        if docker run --rm --entrypoint "" -v $path:$path ${HYPERKUBE_URL} /bin/bash -c "cp /usr/local/bin/{kubelet,kubectl} $path"; then
            mv "$path/kubelet" "/usr/local/bin/kubelet-${KUBERNETES_VERSION}"
            mv "$path/kubectl" "/usr/local/bin/kubectl-${KUBERNETES_VERSION}"
        else
            docker run --rm -v $path:$path ${HYPERKUBE_URL} /bin/bash -c "cp /hyperkube $path"
        fi
    fi

    cp "$path/hyperkube" "/usr/local/bin/kubelet-${KUBERNETES_VERSION}"
    mv "$path/hyperkube" "/usr/local/bin/kubectl-${KUBERNETES_VERSION}"
}

installKubeletKubectlAndKubeProxy() {
    if [[ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ]]; then
        #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
        if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
            extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL}
        else
            if [[ "$CONTAINER_RUNTIME" == "containerd" ]]; then
                extractHyperkube "ctr"
            else
                extractHyperkube "docker"
            fi
        fi
    fi

    mv "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" "/usr/local/bin/kubelet"
    mv "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" "/usr/local/bin/kubectl"
    chmod a+x /usr/local/bin/kubelet /usr/local/bin/kubectl
    rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-* /home/hyperkube-downloads &

    if [ -n "${KUBEPROXY_URL}" ]; then
        #kubeproxy is a system addon that is dictated by control plane so it shouldn't block node provisioning
        pullContainerImage ${CLI_TOOL} ${KUBEPROXY_URL} &
    fi
}

pullContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    echo "pulling the image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        retrycmd_if_failure 60 1 1200 ctr --namespace k8s.io image pull $CONTAINER_IMAGE_URL || ( echo "timed out pulling image ${CONTAINER_IMAGE_URL} via ctr" && exit $ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT )
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        retrycmd_if_failure 60 1 1200 crictl pull $CONTAINER_IMAGE_URL || ( echo "timed out pulling image ${CONTAINER_IMAGE_URL} via crictl" && exit $ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT )
    else
        retrycmd_if_failure 60 1 1200 docker pull $CONTAINER_IMAGE_URL || ( echo "timed out pulling image ${CONTAINER_IMAGE_URL} via docker" && exit $ERR_DOCKER_IMG_PULL_TIMEOUT )
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

removeContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        ctr --namespace k8s.io image rm $CONTAINER_IMAGE_URL
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        crictl rmi $CONTAINER_IMAGE_URL
    else
        docker image rm $CONTAINER_IMAGE_URL
    fi
}

cleanUpImages() {
    local targetImage=$1
    export targetImage
    function cleanupImagesRun() {
        {{if NeedsContainerd}}
        if [[ "${CLI_TOOL}" == "crictl" ]]; then
            images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        else
            images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        fi
        {{else}}
        images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        {{end}}
        local exit_code=$?
        if [[ $exit_code != 0 ]]; then
            exit $exit_code
        elif [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                {{if NeedsContainerd}}
                removeContainerImage ${CLI_TOOL} ${image}
                {{else}}
                removeContainerImage "docker" ${image}
                {{end}}
            done
        fi
    }
    export -f cleanupImagesRun
    retrycmd_if_failure 10 5 120 bash -c cleanupImagesRun
}

cleanUpHyperkubeImages() {
    echo $(date),$(hostname), cleanUpHyperkubeImages
    cleanUpImages "hyperkube"
    echo $(date),$(hostname), endCleanUpHyperkubeImages
}

cleanUpKubeProxyImages() {
    echo $(date),$(hostname), startCleanUpKubeProxyImages
    cleanUpImages "kube-proxy"
    echo $(date),$(hostname), endCleanUpKubeProxyImages
}

cleanUpContainerImages() {
    # run cleanUpHyperkubeImages and cleanUpKubeProxyImages concurrently
    export KUBERNETES_VERSION
    export CLI_TOOL
    export -f retrycmd_if_failure
    export -f removeContainerImage
    export -f cleanUpImages
    export -f cleanUpHyperkubeImages
    export -f cleanUpKubeProxyImages
    bash -c cleanUpHyperkubeImages &
    bash -c cleanUpKubeProxyImages &
}

cleanUpContainerd() {
    rm -Rf $CONTAINERD_DOWNLOADS_DIR
}

overrideNetworkConfig() {
    CONFIG_FILEPATH="/etc/cloud/cloud.cfg.d/80_azure_net_config.cfg"
    touch ${CONFIG_FILEPATH}
    cat << EOF >> ${CONFIG_FILEPATH}
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
ERR_FILE_WATCH_TIMEOUT=6 {{/* Timeout waiting for a file */}}
set -x
if [ -f /opt/azure/containers/provision.complete ]; then
      echo "Already ran to success exiting..."
      exit 0
fi

UBUNTU_RELEASE=$(lsb_release -r -s)
if [[ ${UBUNTU_RELEASE} == "16.04" ]]; then
    sudo apt-get -y autoremove chrony
    echo $?
    sudo systemctl restart systemd-timesyncd
fi

echo $(date),$(hostname), startcustomscript>>/opt/m

for i in $(seq 1 3600); do
    if [ -s {{GetCSEHelpersScriptFilepath}} ]; then
        grep -Fq '#HELPERSEOF' {{GetCSEHelpersScriptFilepath}} && break
    fi
    if [ $i -eq 3600 ]; then
        exit $ERR_FILE_WATCH_TIMEOUT
    else
        sleep 1
    fi
done
sed -i "/#HELPERSEOF/d" {{GetCSEHelpersScriptFilepath}}
source {{GetCSEHelpersScriptFilepath}}

wait_for_file 3600 1 {{GetCSEHelpersScriptDistroFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEHelpersScriptDistroFilepath}}

wait_for_file 3600 1 {{GetCSEInstallScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEInstallScriptFilepath}}

wait_for_file 3600 1 {{GetCSEInstallScriptDistroFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEInstallScriptDistroFilepath}}

wait_for_file 3600 1 {{GetCSEConfigScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEConfigScriptFilepath}}

{{- if not NeedsContainerd}}
cleanUpContainerd
{{- end}}

if [[ "${GPU_NODE}" != "true" ]]; then
    cleanUpGPUDrivers
fi

{{- if ShouldConfigureHTTPProxyCA}}
configureHTTPProxyCA
{{- end}}

disable1804SystemdResolved

if [ -f /var/run/reboot-required ]; then
    REBOOTREQUIRED=true
else
    REBOOTREQUIRED=false
fi

configureAdminUser

{{- if NeedsContainerd}}
# If crictl gets installed then use it as the cri cli instead of ctr
# crictl is not a critical component so continue with boostrapping if the install fails
# CLI_TOOL is by default set to "ctr"
installCrictl && CLI_TOOL="crictl"
{{- end}}

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
if [ -f $VHD_LOGS_FILEPATH ]; then
    echo "detected golden image pre-install"
    cleanUpContainerImages
    FULL_INSTALL_REQUIRED=false
else
    if [[ "${IS_VHD}" = true ]]; then
        echo "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
        exit $ERR_VHD_FILE_NOT_FOUND
    fi
    FULL_INSTALL_REQUIRED=true
fi

if [[ $OS == $UBUNTU_OS_NAME ]] && [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
    installDeps
else
    echo "Golden image; skipping dependencies installation"
fi

installContainerRuntime
{{- if and NeedsContainerd TeleportEnabled}}
installTeleportdPlugin
{{- end}}

installNetworkPlugin

{{- if IsKrustlet }}
    downloadKrustlet
{{- end }}

{{- if IsNSeriesSKU}}
echo $(date),$(hostname), "Start configuring GPU drivers"
if [[ "${GPU_NODE}" = true ]]; then
    if $FULL_INSTALL_REQUIRED; then
        installGPUDrivers
    fi
    ensureGPUDrivers
    if [[ "${ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED}" = true ]]; then
        systemctlEnableAndStart nvidia-device-plugin || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL
    else
        systemctlDisableAndStop nvidia-device-plugin
    fi
fi
echo $(date),$(hostname), "End configuring GPU drivers"
{{end}}

{{- if and IsDockerContainerRuntime HasPrivateAzureRegistryServer}}
set +x
docker login -u $SERVICE_PRINCIPAL_CLIENT_ID -p $SERVICE_PRINCIPAL_CLIENT_SECRET {{GetPrivateAzureRegistryServer}}
set -x
{{end}}

installKubeletKubectlAndKubeProxy

ensureRPC

createKubeManifestDir

{{- if HasDCSeriesSKU}}
if [[ ${SGX_NODE} == true && ! -e "/dev/sgx" ]]; then
    installSGXDrivers
fi
{{end}}

{{- if HasCustomSearchDomain}}
wait_for_file 3600 1 {{GetCustomSearchDomainsCSEScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
{{GetCustomSearchDomainsCSEScriptFilepath}} > /opt/azure/containers/setup-custom-search-domain.log 2>&1 || exit $ERR_CUSTOM_SEARCH_DOMAINS_FAIL
{{end}}

configureK8s

configureCNI

{{/* configure and enable dhcpv6 for dual stack feature */}}
{{- if IsIPv6DualStackFeatureEnabled}}
ensureDHCPv6
{{- end}}

{{- if NeedsContainerd}}
ensureContainerd {{/* containerd should not be configured until cni has been configured first */}}
{{- else}}
ensureDocker
{{- end}}

ensureMonitorService

{{- if EnableHostsConfigAgent}}
configPrivateClusterHosts
{{- end}}

{{- if ShouldConfigTransparentHugePage}}
configureTransparentHugePage
{{- end}}

{{- if ShouldConfigSwapFile}}
configureSwapFile
{{- end}}

ensureSysctl
ensureJournal
{{- if IsKrustlet}}
systemctlEnableAndStart krustlet
{{- else}}
ensureKubelet
{{- if NeedsContainerd}} {{- if and IsKubenet (not HasCalicoNetworkPolicy)}}
ensureNoDupOnPromiscuBridge
{{- end}} {{- end}}
{{- end}}

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        {{/* mitigation for bug https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1676635 */}}
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi

{{- /* re-enable unattended upgrades */}}
rm -f /etc/apt/apt.conf.d/99periodic

if [[ $OS == $UBUNTU_OS_NAME ]]; then
    apt_get_purge 20 30 120 apache2-utils &
fi

VALIDATION_ERR=0

{{- /* Edge case scenarios: */}}
{{- /* high retry times to wait for new API server DNS record to replicate (e.g. stop and start cluster) */}}
{{- /* high timeout to address high latency for private dns server to forward request to Azure DNS */}}
API_SERVER_DNS_RETRIES=100
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
  API_SERVER_DNS_RETRIES=200
fi
{{- if not EnableHostsConfigAgent}}
RES=$(retrycmd_if_failure ${API_SERVER_DNS_RETRIES} 1 10 nslookup ${API_SERVER_NAME})
STS=$?
{{- else}}
STS=0
{{- end}}
if [[ $STS != 0 ]]; then
    time nslookup ${API_SERVER_NAME}
    if [[ $RES == *"168.63.129.16"*  ]]; then
        VALIDATION_ERR=$ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL
    else
        VALIDATION_ERR=$ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL
    fi
else
    API_SERVER_CONN_RETRIES=50
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
        API_SERVER_CONN_RETRIES=100
    fi
    retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443 || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
fi

# If it is a MIG Node, enable mig-partition systemd service to create MIG instances
if [[ "${MIG_NODE}" == "true" ]]; then
    REBOOTREQUIRED=true
    ensureMigPartition
fi

if $REBOOTREQUIRED; then
    echo 'reboot required, rebooting node in 1 minute'
    /bin/bash -c "shutdown -r 1 &"
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        aptmarkWALinuxAgent unhold &
    fi
else
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        /usr/lib/apt/apt.systemd.daily &
        aptmarkWALinuxAgent unhold &
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete
ps auxfww > /opt/azure/provision-ps.log &

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

var _linuxCloudInitArtifactsCse_startSh = []byte(`CSE_STARTTIME=$(date)
/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(head -c 3000 "/var/log/azure/cluster-provision-cse-output.log")
KUBELET_START_TIME=$(echo "$OUTPUT" | cut -d ',' -f -1 | head -1)
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
SYSTEMD_SUMMARY=$(systemd-analyze || true)
EXECUTION_DURATION=$(echo $(($(date +%s) - $(date -d "$CSE_STARTTIME" +%s))))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg cse "$CSE_STARTTIME" \
                  --arg ga "$GUEST_AGENT_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  --arg kubelet "$KUBELET_START_TIME" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, KubeletStartTime: $kubelet }}' )
echo $JSON_STRING
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
ExecStart={{GetDHCPv6ConfigCSEScriptFilepath}}

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
repoDepotEndpoint="{{AKSCustomCloudRepoDepotEndpoint}}"
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

var _linuxCloudInitArtifactsKrustletService = []byte(`[Unit]
Description=Krustlet

[Service]
Restart=on-failure
RestartSec=5s
EnvironmentFile=/etc/default/kubelet
Environment=KUBECONFIG=/var/lib/kubelet/kubeconfig
Environment=KRUSTLET_CERT_FILE=/etc/kubernetes/certs/kubeletserver.crt
Environment=KRUSTLET_PRIVATE_KEY_FILE=/etc/kubernetes/certs/kubeletserver.key
Environment=KRUSTLET_DATA_DIR=/etc/krustlet
Environment=RUST_LOG=wasi_provider=info,main=info
Environment=KRUSTLET_BOOTSTRAP_FILE=/var/lib/kubelet/bootstrap-kubeconfig
ExecStart=/usr/local/bin/krustlet-wasi --node-labels="${KUBELET_NODE_LABELS}" {{GetKrustletFlags}}

[Install]
WantedBy=multi-user.target
`)

func linuxCloudInitArtifactsKrustletServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsKrustletService, nil
}

func linuxCloudInitArtifactsKrustletService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsKrustletServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/krustlet.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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

[Service]
Restart=always
EnvironmentFile=/etc/default/kubelet
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
    ! (dnf update -y --refresh 2>&1 | tee $dnf_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
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
    for dnf_package in blobfuse ca-certificates cifs-utils conntrack-tools cracklib ebtables ethtool fuse git iotop iproute ipset iptables jq logrotate lsof nmap-ncat nfs-utils pam pigz psmisc socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installGPUDrivers() {
    echo "GPU drivers not yet supported for Mariner"
    exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
}

installSGXDrivers() {
    echo "SGX drivers not yet supported for Mariner"
    exit $ERR_SGX_DRIVERS_START_FAIL
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
    rm -Rf $GPU_DEST
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

var _linuxCloudInitArtifactsMigPartitionService = []byte(`[Unit]
Description=Apply MIG configuration on Nvidia A100 GPU

[Service]
Restart=on-failure
ExecStartPre=/usr/bin/nvidia-smi -mig 1
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh {{GetGPUInstanceProfile}}

[Install]
WantedBy=multi-user.target`)

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
        exit ${ERR_MIG_PARTITION_FAILURE}
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
install cramfs /bin/true
# 1.1.1.2 Ensure mounting of freevxfs filesystems is disabled
install freevxfs /bin/true
# 1.1.1.3 Ensure mounting of jffs2 filesystems is disabled
install jffs2 /bin/true
# 1.1.1.4 Ensure mounting of hfs filesystems is disabled
install hfs /bin/true
# 1.1.1.5 Ensure mounting of hfsplus filesystems is disabled
install hfsplus /bin/true`)

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
ExecStart=/usr/local/nvidia/bin/nvidia-device-plugin
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
[Install]
WantedBy=multi-user.target`)

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
clusterFQDN="{{GetKubernetesEndpoint}}"
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
source {{GetCSEHelpersScriptFilepath}}
source {{GetCSEHelpersScriptDistroFilepath}}


echo "  dns-search {{GetSearchDomainName}}" | tee -a /etc/network/interfaces.d/50-cloud-init.cfg
systemctl_restart 20 5 10 networking
wait_for_apt_locks
retrycmd_if_failure 10 5 120 apt-get -y install realmd sssd sssd-tools samba-common samba samba-common python2.7 samba-libs packagekit
wait_for_apt_locks
echo "{{GetSearchDomainRealmPassword}}" | realm join -U {{GetSearchDomainRealmUser}}@$(echo "{{GetSearchDomainName}}" | tr /a-z/ /A-Z/) $(echo "{{GetSearchDomainName}}" | tr /a-z/ /A-Z/)
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

var _linuxCloudInitArtifactsSysctlD60CisConf = []byte(`# 3.1.2 Ensure packet redirect sending is disabled
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
# 3.2.1 Ensure source routed packets are not accepted 
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
# 3.2.2 Ensure ICMP redirects are not accepted
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
# 3.2.3 Ensure secure ICMP redirects are not accepted
net.ipv4.conf.all.secure_redirects = 0
net.ipv4.conf.default.secure_redirects = 0
# 3.2.4 Ensure suspicious packets are logged
net.ipv4.conf.all.log_martians = 1
net.ipv4.conf.default.log_martians = 1
# 3.3.1 Ensure IPv6 router advertisements are not accepted
net.ipv6.conf.all.accept_ra = 0
net.ipv6.conf.default.accept_ra = 0
# 3.3.2 Ensure IPv6 redirects are not accepted
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
# refer to https://github.com/kubernetes/kubernetes/blob/75d45bdfc9eeda15fb550e00da662c12d7d37985/pkg/kubelet/cm/container_manager_linux.go#L359-L397
vm.overcommit_memory = 1
kernel.panic = 10
kernel.panic_on_oops = 1
# https://github.com/Azure/AKS/issues/772
fs.inotify.max_user_watches = 1048576
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
        apt-get purge -o Dpkg::Options::="--force-confold" -y ${@} && break || \
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
    ! (apt-get dist-upgrade -y 2>&1 | tee $apt_dist_upgrade_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
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
    wait_for_apt_locks
    retrycmd_if_failure 10 5 60 apt-get purge -y moby-engine moby-cli
}

removeContainerd() {
    wait_for_apt_locks
    retrycmd_if_failure 10 5 60 apt-get purge -y moby-containerd
}

installDeps() {
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL
    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for apt_package in apache2-utils apt-transport-https blobfuse=1.3.7 ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool fuse git glusterfs-client htop iftop init-system-helpers iotop iproute2 ipset iptables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat traceroute util-linux xz-utils zip; do
      if ! apt_get_install 30 1 600 $apt_package; then
        journalctl --no-pager -u $apt_package
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installGPUDrivers() {
    mkdir -p $GPU_DEST/tmp
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://nvidia.github.io/nvidia-docker/gpgkey > $GPU_DEST/tmp/aptnvidia.gpg || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure 120 5 25 apt-key add $GPU_DEST/tmp/aptnvidia.gpg || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://nvidia.github.io/nvidia-docker/ubuntu${UBUNTU_RELEASE}/nvidia-docker.list > $GPU_DEST/tmp/nvidia-docker.list || exit  $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure_no_stats 120 5 25 cat $GPU_DEST/tmp/nvidia-docker.list > /etc/apt/sources.list.d/nvidia-docker.list || exit  $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    apt_get_update
    retrycmd_if_failure 30 5 3600 apt-get install -y linux-headers-$(uname -r) gcc make dkms || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    retrycmd_if_failure 30 5 60 curl -fLS https://us.download.nvidia.com/tesla/$GPU_DV/NVIDIA-Linux-x86_64-${GPU_DV}.run -o ${GPU_DEST}/nvidia-drivers-${GPU_DV} || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    tmpDir=$GPU_DEST/tmp
    if ! (
      set -e -o pipefail
      cd "${tmpDir}"
      retrycmd_if_failure 30 5 3600 apt-get download nvidia-docker2="${NVIDIA_DOCKER_VERSION}+${NVIDIA_DOCKER_SUFFIX}" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    ); then
      exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
}

installSGXDrivers() {
    echo "Installing SGX driver"
    local VERSION
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    case $VERSION in
    "18.04")
        SGX_DRIVER_URL="https://download.01.org/intel-sgx/dcap-1.2/linux/dcap_installers/ubuntuServer18.04/sgx_linux_x64_driver_1.12_c110012.bin"
        ;;
    "16.04")
        SGX_DRIVER_URL="https://download.01.org/intel-sgx/dcap-1.2/linux/dcap_installers/ubuntuServer16.04/sgx_linux_x64_driver_1.12_c110012.bin"
        ;;
    "*")
        echo "Version $VERSION is not supported"
        exit 1
        ;;
    esac

    local PACKAGES="make gcc dkms"
    wait_for_apt_locks
    retrycmd_if_failure 30 5 3600 apt-get -y install $PACKAGES  || exit $ERR_SGX_DRIVERS_INSTALL_TIMEOUT

    local SGX_DRIVER
    SGX_DRIVER=$(basename $SGX_DRIVER_URL)
    local OE_DIR=/opt/azure/containers/oe
    mkdir -p ${OE_DIR}

    retrycmd_if_failure 120 5 25 curl -fsSL ${SGX_DRIVER_URL} -o ${OE_DIR}/${SGX_DRIVER} || exit $ERR_SGX_DRIVERS_INSTALL_TIMEOUT
    chmod a+x ${OE_DIR}/${SGX_DRIVER}
    ${OE_DIR}/${SGX_DRIVER} || exit $ERR_SGX_DRIVERS_START_FAIL
}

updateAptWithMicrosoftPkg() {
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

{{- if NeedsContainerd}}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    CONTAINERD_VERSION=$1
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.4.1 is our lowest supported version of containerd
    
    #if there is no containerd_version input from RP, use hardcoded version
    if [[ -z ${CONTAINERD_VERSION} ]]; then
        CONTAINERD_VERSION="1.4.8"
        echo "Containerd Version not specified, using default version: ${CONTAINERD_VERSION}"
    else
        echo "Using specified Containerd Version: ${CONTAINERD_VERSION}"
    fi

    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeMoby
        removeContainerd
        # if containerd version has been overriden then there should exist a local .deb file for it on aks VHDs (best-effort)
        # if no files found then try fetching from packages.microsoft repo
        if [ -f $VHD_LOGS_FILEPATH ]; then
            CONTAINERD_DEB_TMP="moby-containerd_${CONTAINERD_VERSION/-/\~}+azure-1_amd64.deb"
            CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
            if [[ -f "${CONTAINERD_DEB_FILE}" ]]; then
                installDebPackageFromFile ${CONTAINERD_DEB_FILE} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
                return 0
            fi
        fi
        updateAptWithMicrosoftPkg
        apt_get_install 20 30 120 moby-containerd=${CONTAINERD_VERSION}* --allow-downgrades || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    fi
    ensureRunc ${RUNC_VERSION:-""} # RUNC_VERSION is an optional override supplied via NodeBootstrappingConfig api
}

downloadContainerd() {
    CONTAINERD_VERSION=$1
    # currently upstream maintains the package on a storage endpoint rather than an actual apt repo
    CONTAINERD_DOWNLOAD_URL="https://mobyartifacts.azureedge.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/bionic/linux_amd64/moby-containerd_${CONTAINERD_VERSION/-/\~}+azure-1_amd64.deb"
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}
{{- end}}

installMoby() {
    CURRENT_VERSION=$(dockerd --version | grep "Docker version" | cut -d "," -f 1 | cut -d " " -f 3 | cut -d "+" -f 1)
    local MOBY_VERSION="19.03.14"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${MOBY_VERSION}; then
        echo "currently installed moby-docker version ${CURRENT_VERSION} is greater than (or equal to) target base version ${MOBY_VERSION}. skipping installMoby."
    else
        removeMoby
        updateAptWithMicrosoftPkg
        MOBY_CLI=${MOBY_VERSION}
        if [[ "${MOBY_CLI}" == "3.0.4" ]]; then
            MOBY_CLI="3.0.3"
        fi
        apt_get_install 20 30 120 moby-engine=${MOBY_VERSION}* moby-cli=${MOBY_CLI}* --allow-downgrades || exit $ERR_MOBY_INSTALL_TIMEOUT
    fi
    ensureRunc ${RUNC_VERSION:-""} # RUNC_VERSION is an optional override supplied via NodeBootstrappingConfig api
}

ensureRunc() {
    TARGET_VERSION=$1
    if [[ -z ${TARGET_VERSION} ]]; then
        TARGET_VERSION="1.0.0-rc95"
    fi
    CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
    if [ "${CURRENT_VERSION}" == "${TARGET_VERSION}" ]; then
        echo "target moby-runc version ${TARGET_VERSION} is already installed. skipping installRunc."
    fi
    # if on a vhd-built image, first check if we've cached the deb file
    if [ -f $VHD_LOGS_FILEPATH ]; then
        RUNC_DEB_PATTERN="moby-runc_${TARGET_VERSION/-/\~}+azure-*_amd64.deb"
        RUNC_DEB_FILE=$(find ${RUNC_DOWNLOADS_DIR} -type f -iname "${RUNC_DEB_PATTERN}" | sort -V | tail -n1)
        if [[ -f "${RUNC_DEB_FILE}" ]]; then
            installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
            return 0
        fi
    fi
    apt_get_install 20 30 120 moby-runc=${TARGET_VERSION/-/\~}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST
    rm -f /etc/apt/sources.list.d/nvidia-docker.list
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

- path: /opt/azure/containers/provision_start.sh
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionStartScript"}}

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

{{if not .IsVHDDistro}}
- path: /opt/azure/containers/provision_cis.sh
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionCIS"}}
{{end}}

{{if IsAKSCustomCloud}}
- path: {{GetInitAKSCustomCloudFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "initAKSCustomCloud"}}
{{end}}

{{- if EnableHostsConfigAgent}}
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
{{- end}}

 {{- if IsKrustlet}}
- path: /etc/systemd/system/krustlet.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "krustletSystemdService"}}
{{ else }}
- path: /etc/systemd/system/kubelet.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "kubeletSystemdService"}}
{{- end}}

{{- if IsMIGEnabledNode}}
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
{{end}}

{{- if HasKubeletDiskType}}
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

- path: /etc/systemd/system/kubelet.service.d/10-bindmount.conf
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "bindMountDropin"}}
{{end}}

{{if not .IsVHDDistro}}
- path: /usr/local/bin/health-monitor.sh
  permissions: "0544"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "healthMonitorScript"}}

- path: /etc/systemd/system/kubelet-monitor.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "kubeletMonitorSystemdService"}}

{{if NeedsContainerd}}
- path: /etc/systemd/system/containerd-monitor.timer
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "containerdMonitorSystemdTimer"}}

- path: /etc/systemd/system/containerd-monitor.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "containerdMonitorSystemdService"}}
{{- else}}
- path: /etc/systemd/system/docker-monitor.timer
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dockerMonitorSystemdTimer"}}

- path: /etc/systemd/system/docker-monitor.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dockerMonitorSystemdService"}}
{{- end}}

- path: /etc/apt/preferences
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "aptPreferences"}}
{{end}}

{{if not IsMariner}}
- path: /etc/apt/apt.conf.d/99periodic
  permissions: "0644"
  owner: root
  content: |
    APT::Periodic::Update-Package-Lists "0";
    APT::Periodic::Download-Upgradeable-Packages "0";
    APT::Periodic::AutocleanInterval "0";
    APT::Periodic::Unattended-Upgrade "0";
{{end}}

{{- if ShouldConfigureHTTPProxy}}
- path: /etc/apt/apt.conf.d/95proxy
  permissions: "0644"
  owner: root
  content: |
    {{- if HasHTTPProxy }}
    Acquire::http::proxy "{{GetHTTPProxy}}";
    {{- end}}
    {{- if HasHTTPSProxy }}
    Acquire::https::proxy "{{GetHTTPSProxy}}";
    {{- end}}

- path: /etc/systemd/system.conf.d/proxy.conf
  permissions: "0644"
  owner: root
  content: |
    [Manager]
    {{- if HasHTTPProxy }}
    DefaultEnvironment="HTTP_PROXY={{GetHTTPProxy}}"
    DefaultEnvironment="http_proxy={{GetHTTPProxy}}"
    {{- end}}
    {{- if HasHTTPSProxy }}
    DefaultEnvironment="HTTPS_PROXY={{GetHTTPSProxy}}"
    DefaultEnvironment="https_proxy={{GetHTTPSProxy}}"
    {{- end}}
    {{- if HasNoProxy }}
    DefaultEnvironment="NO_PROXY={{GetNoProxy}}"
    DefaultEnvironment="no_proxy={{GetNoProxy}}"
    {{- end}}

- path: /etc/systemd/system/kubelet.service.d/10-httpproxy.conf
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "httpProxyDropin"}}
{{- end}}

{{- if ShouldConfigureHTTPProxyCA}}
- path: /usr/local/share/ca-certificates/proxyCA.pem
  permissions: "0644"
  owner: root
  content: |
{{GetHTTPProxyCA}}
{{- end}}

{{if IsIPv6DualStackFeatureEnabled}}
- path: {{GetDHCPv6ServiceCSEScriptFilepath}}
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dhcpv6SystemdService"}}

- path: {{GetDHCPv6ConfigCSEScriptFilepath}}
  permissions: "0544"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dhcpv6ConfigurationScript"}}
{{end}}

{{if RequiresDocker}}
    {{if not .IsVHDDistro}}
- path: /etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
  permissions: "0644"
  encoding: gzip
  owner: "root"
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "dockerClearMountPropagationFlags"}}
    {{end}}

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
{{end}}

{{if NeedsContainerd}}
- path: /etc/systemd/system/kubelet.service.d/10-containerd.conf
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "containerdKubeletDropin"}}
{{if UseRuncShimV2}}
- path: /etc/containerd/config.toml
  permissions: "0644"
  owner: root
  content: |
    version = 2
    oom_score = 0{{if HasDataDir }}
    root = "{{GetDataDir}}"{{- end}}
    [plugins."io.containerd.grpc.v1.cri"]
      sandbox_image = "{{GetPodInfraContainerSpec}}"
      [plugins."io.containerd.grpc.v1.cri".containerd]
        {{- if TeleportEnabled }}
        snapshotter = "teleportd"
        disable_snapshot_annotations = false
        {{- end}}
        {{- if IsNSeriesSKU }}
        default_runtime_name = "nvidia-container-runtime"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime.options]
          BinaryName = "/usr/bin/nvidia-container-runtime"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
          BinaryName = "/usr/bin/nvidia-container-runtime"
        {{- else}}
        default_runtime_name = "runc"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
          BinaryName = "/usr/bin/runc"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
          BinaryName = "/usr/bin/runc"
        {{- end}}
      {{- if and (IsKubenet) (not HasCalicoNetworkPolicy) }}
      [plugins."io.containerd.grpc.v1.cri".cni]
        bin_dir = "/opt/cni/bin"
        conf_dir = "/etc/cni/net.d"
        conf_template = "/etc/containerd/kubenet_template.conf"
      {{- end}}
      [plugins."io.containerd.grpc.v1.cri".registry.headers]
        X-Meta-Source-Client = ["azure/aks"]
    [metrics]
      address = "0.0.0.0:10257"
    {{- if TeleportEnabled }}
    [proxy_plugins]
      [proxy_plugins.teleportd]
        type = "snapshot"
        address = "/run/teleportd/snapshotter.sock"
    {{- end}}
    #EOF
{{else}}
- path: /etc/containerd/config.toml
  permissions: "0644"
  owner: root
  content: |
    version = 2
    subreaper = false
    oom_score = 0{{if HasDataDir }}
    root = "{{GetDataDir}}"{{- end}}
    [plugins."io.containerd.grpc.v1.cri"]
      sandbox_image = "{{GetPodInfraContainerSpec}}"
      [plugins."io.containerd.grpc.v1.cri".containerd]
        {{ if TeleportEnabled }}
        snapshotter = "teleportd"
        disable_snapshot_annotations = false
        {{ end}}
        [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
          runtime_type = "io.containerd.runtime.v1.linux"
          {{- if IsNSeriesSKU}}
          runtime_engine = "/usr/bin/nvidia-container-runtime"
          {{- else}}
          runtime_engine = "/usr/bin/runc"
          {{- end}}
        [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
          runtime_type = "io.containerd.runtime.v1.linux"
          {{- if IsNSeriesSKU}}
          runtime_engine = "/usr/bin/nvidia-container-runtime"
          {{- else}}
          runtime_engine = "/usr/bin/runc"
          {{- end}}
      {{ if and (IsKubenet) (not HasCalicoNetworkPolicy) }}
      [plugins."io.containerd.grpc.v1.cri".cni]
        bin_dir = "/opt/cni/bin"
        conf_dir = "/etc/cni/net.d"
        conf_template = "/etc/containerd/kubenet_template.conf"
      {{ end}}
      [plugins."io.containerd.grpc.v1.cri".registry.headers]
        X-Meta-Source-Client = ["azure/aks"]
    [metrics]
      address = "0.0.0.0:10257"
    {{ if TeleportEnabled }}
    [proxy_plugins]
      [proxy_plugins.teleportd]
        type = "snapshot"
        address = "/run/teleportd/snapshotter.sock"
    {{ end}}
    #EOF
{{end}}
- path: /etc/containerd/kubenet_template.conf
  permissions: "0644"
  owner: root
  content: |
      {
          "cniVersion": "0.3.1",
          "name": "kubenet",
          "plugins": [{
            "type": "bridge",
            "bridge": "cbr0",
            "mtu": 1500,
            "addIf": "eth0",
            "isGateway": true,
            {{- if IsIPMasqAgentEnabled}}
            "ipMasq": false,
            {{- else}}
            "ipMasq": true,
            {{- end}}
            "promiscMode": true,
            "hairpinMode": false,
            "ipam": {
                "type": "host-local",
                "subnet": "{{`+"`"+`{{.PodCIDR}}`+"`"+`}}",
                "routes": [{ "dst": "0.0.0.0/0" }]
            }
          },
          {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "externalSetMarkChain": "KUBE-MARK-MASQ"
          }]
      }

- path: /etc/systemd/system/containerd.service
  permissions: "0644"
  owner: root
  content: |
    [Unit]
    Description=containerd daemon
    After=network.target

    [Service]
    ExecStartPre=/sbin/modprobe overlay
    ExecStart=/usr/bin/containerd
    Delegate=yes
    KillMode=process
    Restart=always
    OOMScoreAdjust=-999
    LimitNOFILE=1048576
    LimitNPROC=infinity
    LimitCORE=infinity

    [Install]
    WantedBy=multi-user.target

    #EOF

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

- path: /etc/sysctl.d/11-containerd.conf
  permissions: "0644"
  owner: root
  content: |
    net.ipv4.ip_forward = 1
    net.ipv4.conf.all.forwarding = 1
    net.bridge.bridge-nf-call-iptables = 1
    #EOF

{{if TeleportEnabled}}
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
{{end}}
{{- if and IsKubenet (not HasCalicoNetworkPolicy)}}
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
{{- end}}
{{end}}

{{if IsNSeriesSKU}}
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
{{end}}

- path: /etc/kubernetes/certs/ca.crt
  permissions: "0644"
  encoding: base64
  owner: root
  content: |
    {{GetParameter "caCertificate"}}

{{if not IsKubeletClientTLSBootstrappingEnabled -}}
- path: /etc/kubernetes/certs/client.crt
  permissions: "0644"
  encoding: base64
  owner: root
  content: |
    {{GetParameter "clientCertificate"}}
{{- end}}

{{if HasCustomSearchDomain}}
- path: {{GetCustomSearchDomainsCSEScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "customSearchDomainsScript"}}
{{end}}

{{- if IsKubeletConfigFileEnabled}}
- path: /etc/systemd/system/kubelet.service.d/10-componentconfig.conf
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "componentConfigDropin"}}
{{ end }}

{{if IsKubeletClientTLSBootstrappingEnabled -}}
- path: /var/lib/kubelet/bootstrap-kubeconfig
  permissions: "0644"
  owner: root
  content: |
    apiVersion: v1
    kind: Config
    clusters:
    - name: localcluster
      cluster:
        {{ if IsKrustlet -}}
        certificate-authority-data: "{{GetBase64CertificateAuthorityData}}"
        {{- else -}}
        certificate-authority: /etc/kubernetes/certs/ca.crt
        {{- end }}
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
    #EOF
- path: /etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "tlsBootstrapDropin"}}
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
    #EOF
{{- end}}

- path: /etc/default/kubelet
  permissions: "0644"
  owner: root
  content: |
    KUBELET_FLAGS={{GetKubeletConfigKeyVals}}
    KUBELET_REGISTER_SCHEDULABLE=true
    NETWORK_POLICY={{GetParameter "networkPolicy"}}
{{- if not (IsKubernetesVersionGe "1.17.0")}}
    KUBELET_IMAGE={{GetHyperkubeImageReference}}
{{end}}
{{if IsKubernetesVersionGe "1.16.0"}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabels . }}
{{else}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabelsDeprecated . }}
{{end}}
{{- if IsAKSCustomCloud}}
    AZURE_ENVIRONMENT_FILEPATH=/etc/kubernetes/{{GetTargetEnvironment}}.json
{{end}}
    #EOF

- path: /opt/azure/containers/kubelet.sh
  permissions: "0755"
  owner: root
  content: |
    #!/bin/bash
{{if not IsIPMasqAgentEnabled}}
    {{if IsAzureCNI}}
    iptables -t nat -A POSTROUTING -m iprange ! --dst-range 168.63.129.16 -m addrtype ! --dst-type local ! -d {{GetParameter "vnetCidr"}} -j MASQUERADE
    {{end}}
{{end}}

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
    #EOF

- path: /etc/sysctl.d/999-sysctl-aks.conf
  permissions: "0644"
  owner: root
  content: |
    # This is a partial workaround to this upstream Kubernetes issue:
    # https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
    net.ipv4.tcp_retries2=8
    net.core.message_burst=80
    net.core.message_cost=40
{{- if GetCustomSysctlConfigByName "NetCoreSomaxconn"}}
    net.core.somaxconn={{.CustomLinuxOSConfig.Sysctls.NetCoreSomaxconn}}
{{- else}}
    net.core.somaxconn=16384
{{- end}}
{{- if GetCustomSysctlConfigByName "NetIpv4TcpMaxSynBacklog"}}
    net.ipv4.tcp_max_syn_backlog={{.CustomLinuxOSConfig.Sysctls.NetIpv4TcpMaxSynBacklog}}
{{- else}}
    net.ipv4.tcp_max_syn_backlog=16384
{{- end}}
{{- if GetCustomSysctlConfigByName "NetIpv4NeighDefaultGcThresh1"}}
    net.ipv4.neigh.default.gc_thresh1={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh1}}
{{- else}}
    net.ipv4.neigh.default.gc_thresh1=4096
{{- end}}
{{- if GetCustomSysctlConfigByName "NetIpv4NeighDefaultGcThresh2"}}
    net.ipv4.neigh.default.gc_thresh2={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh2}}
{{- else}}
    net.ipv4.neigh.default.gc_thresh2=8192
{{- end}}
{{- if GetCustomSysctlConfigByName "NetIpv4NeighDefaultGcThresh3"}}
    net.ipv4.neigh.default.gc_thresh3={{.CustomLinuxOSConfig.Sysctls.NetIpv4NeighDefaultGcThresh3}}
{{- else}}
    net.ipv4.neigh.default.gc_thresh3=16384
{{- end}}
{{if ShouldConfigCustomSysctl}}
    # The following are sysctl configs passed from API
{{- $s:=.CustomLinuxOSConfig.Sysctls}}
{{- if $s.NetCoreNetdevMaxBacklog}}
    net.core.netdev_max_backlog={{$s.NetCoreNetdevMaxBacklog}}
{{- end}}
{{- if $s.NetCoreRmemDefault}}
    net.core.rmem_default={{$s.NetCoreRmemDefault}}
{{- end}}
{{- if $s.NetCoreRmemMax}}
    net.core.rmem_max={{$s.NetCoreRmemMax}}
{{- end}}
{{- if $s.NetCoreWmemDefault}}
    net.core.wmem_default={{$s.NetCoreWmemDefault}}
{{- end}}
{{- if $s.NetCoreWmemMax}}
    net.core.wmem_max={{$s.NetCoreWmemMax}}
{{- end}}
{{- if $s.NetCoreOptmemMax}}
    net.core.optmem_max={{$s.NetCoreOptmemMax}}
{{- end}}
{{- if $s.NetIpv4TcpMaxTwBuckets}}
    net.ipv4.tcp_max_tw_buckets={{$s.NetIpv4TcpMaxTwBuckets}}
{{- end}}
{{- if $s.NetIpv4TcpFinTimeout}}
    net.ipv4.tcp_fin_timeout={{$s.NetIpv4TcpFinTimeout}}
{{- end}}
{{- if $s.NetIpv4TcpKeepaliveTime}}
    net.ipv4.tcp_keepalive_time={{$s.NetIpv4TcpKeepaliveTime}}
{{- end}}
{{- if $s.NetIpv4TcpKeepaliveProbes}}
    net.ipv4.tcp_keepalive_probes={{$s.NetIpv4TcpKeepaliveProbes}}
{{- end}}
{{- if $s.NetIpv4TcpkeepaliveIntvl}}
    net.ipv4.tcp_keepalive_intvl={{$s.NetIpv4TcpkeepaliveIntvl}}
{{- end}}
{{- if $s.NetIpv4TcpTwReuse}}
    net.ipv4.tcp_tw_reuse={{BoolPtrToInt $s.NetIpv4TcpTwReuse}}
{{- end}}
{{- if $s.NetIpv4IpLocalPortRange}}
    net.ipv4.ip_local_port_range={{$s.NetIpv4IpLocalPortRange}}
{{- end}}
{{- if $s.NetNetfilterNfConntrackMax}}
    net.netfilter.nf_conntrack_max={{$s.NetNetfilterNfConntrackMax}}
{{- end}}
{{- if $s.NetNetfilterNfConntrackBuckets}}
    net.netfilter.nf_conntrack_buckets={{$s.NetNetfilterNfConntrackBuckets}}
{{- end}}
{{- if $s.FsInotifyMaxUserWatches}}
    fs.inotify.max_user_watches={{$s.FsInotifyMaxUserWatches}}
{{- end}}
{{- if $s.FsFileMax}}
    fs.file-max={{$s.FsFileMax}}
{{- end}}
{{- if $s.FsAioMaxNr}}
    fs.aio-max-nr={{$s.FsAioMaxNr}}
{{- end}}
{{- if $s.FsNrOpen}}
    fs.nr_open={{$s.FsNrOpen}}
{{- end}}
{{- if $s.KernelThreadsMax}}
    kernel.threads-max={{$s.KernelThreadsMax}}
{{- end}}
{{- if $s.VMMaxMapCount}}
    vm.max_map_count={{$s.VMMaxMapCount}}
{{- end}}
{{- if $s.VMSwappiness}}
    vm.swappiness={{$s.VMSwappiness}}
{{- end}}
{{- if $s.VMVfsCachePressure}}
    vm.vfs_cache_pressure={{$s.VMVfsCachePressure}}
{{- end}}
{{- end}}
    #EOF

runcmd:
- set -x
- . {{GetCSEHelpersScriptFilepath}}
- . {{GetCSEHelpersScriptDistroFilepath}}
- aptmarkWALinuxAgent hold{{GetKubernetesAgentPreprovisionYaml .}}
`)

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

var _windowsContainerdtemplateToml = []byte(`root = "C:\\ProgramData\\containerd\\root"
state = "C:\\ProgramData\\containerd\\state"

[grpc]
  address = "\\\\.\\pipe\\containerd-containerd"
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[ttrpc]
  address = ""

[debug]
  address = ""
  level = "info"

[metrics]
  address = "0.0.0.0:10257"
  grpc_histogram = false

[cgroup]
  path = ""

[plugins]
  [plugins.cri]
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    enable_selinux = false
    sandbox_image = "{{pauseImage}}-windows-{{currentversion}}-amd64"
    stats_collect_period = 10
    systemd_cgroup = false
    enable_tls_streaming = false
    max_container_log_line_size = 16384
    disable_http2_client = false
    [plugins.cri.containerd]
      snapshotter = "windows"
      discard_unpacked_layers = true
      no_pivot = false
      [plugins.cri.containerd.default_runtime]
        runtime_type = "io.containerd.runhcs.v1"
        [plugins.cri.containerd.default_runtime.options]
          Debug = false
          DebugType = 0
          SandboxImage = "{{pauseImage}}-windows-{{currentversion}}-amd64"
          SandboxPlatform = "windows/amd64"
          SandboxIsolation = {{sandboxIsolation}}
      [plugins.cri.containerd.runtimes]
        [plugins.cri.containerd.runtimes.runhcs-wcow-process]
          runtime_type = "io.containerd.runhcs.v1"
          [plugins.cri.containerd.runtimes.runhcs-wcow-process.options]
            Debug = true
            DebugType = 2
            SandboxImage = "{{pauseImage}}-windows-{{currentversion}}-amd64"
            SandboxPlatform = "windows/amd64"
{{hypervisors}}
    [plugins.cri.cni]
      bin_dir = "{{cnibin}}"
      conf_dir = "{{cniconf}}"
    [plugins.cri.registry]
      [plugins.cri.registry.mirrors]
        [plugins.cri.registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
  [plugins.diff-service]
    default = ["windows"]
  [plugins.scheduler]
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = "0s"
    startup_delay = "100ms"`)

func windowsContainerdtemplateTomlBytes() ([]byte, error) {
	return _windowsContainerdtemplateToml, nil
}

func windowsContainerdtemplateToml() (*asset, error) {
	bytes, err := windowsContainerdtemplateTomlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/containerdtemplate.toml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsCsecmdPs1 = []byte(`powershell.exe -ExecutionPolicy Unrestricted -command \"
$arguments = '
-MasterIP {{ GetKubernetesEndpoint }}
-KubeDnsServiceIp {{ GetParameter "kubeDNSServiceIP" }}
-MasterFQDNPrefix {{ GetParameter "masterEndpointDNSNamePrefix" }}
-Location {{ GetVariable "location" }}
{{if UserAssignedIDEnabled}}
-UserAssignedClientID {{ GetVariable "userAssignedIdentityID" }}
{{ end }}
-TargetEnvironment {{ GetTargetEnvironment }}
-AgentKey {{ GetParameter "clientPrivateKey" }}
-AADClientId {{ GetParameter "servicePrincipalClientId" }}
-AADClientSecret ''{{ GetParameter "encodedServicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01
-LogFile %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log
-CSEResultFilePath %SYSTEMDRIVE%\AzureData\CSEResult.log';
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
Copy-Item $inputFile $outputFile;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\" >> %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1; $code=(Get-Content %SYSTEMDRIVE%\AzureData\CSEResult.log); exit $code`)

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

var _windowsKuberneteswindowsfunctionsPs1 = []byte(`# This filter removes null characters (\0) which are captured in nssm.exe output when logged through powershell
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
        $DestinationPath
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
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_FILE_WITH_RETRY -ErrorMessage "Failed in downloading $Url. Error: $_"
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

function Get-ProvisioningScripts {
    Write-Log "Getting provisioning scripts"
    DownloadFileOverHttp -Url $global:ProvisioningScriptsPackageUrl -DestinationPath 'c:\k\provisioningscripts.zip'
    Expand-Archive -Path 'c:\k\provisioningscripts.zip' -DestinationPath 'c:\k' -Force
    Remove-Item -Path 'c:\k\provisioningscripts.zip' -Force
}

function Get-WindowsVersion {
    $systemInfo = Get-ItemProperty -Path "HKLM:SOFTWARE\Microsoft\Windows NT\CurrentVersion"
    return "$($systemInfo.CurrentBuildNumber).$($systemInfo.UBR)"
}

function Get-CniVersion {
    switch ($global:NetworkPlugin) {
        "azure" {
            if ($global:VNetCNIPluginsURL -match "(v[0-9`+"`"+`.]+).(zip|tar)") {
                return $matches[1]
            }
            else {
                return ""
            }
            break;
        }
        default {
            return ""
        }
    }
}

function Get-InstanceMetadataServiceTelemetry {
    $keys = @{ }

    try {
        # Write-Log "Querying instance metadata service..."
        # Note: 2019-04-30 is latest api available in all clouds
        $metadata = Invoke-RestMethod -Headers @{"Metadata" = "true" } -URI "http://169.254.169.254/metadata/instance?api-version=2019-04-30" -Method get
        # Write-Log ($metadata | ConvertTo-Json)

        $keys.Add("vm_size", $metadata.compute.vmSize)
    }
    catch {
        Write-Log "Error querying instance metadata service."
    }

    return $keys
}

# https://stackoverflow.com/a/34559554/697126
function New-TemporaryDirectory {
    $parent = [System.IO.Path]::GetTempPath()
    [string] $name = [System.Guid]::NewGuid()
    New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

function Initialize-DataDirectories {
    # Some of the Kubernetes tests that were designed for Linux try to mount /tmp into a pod
    # On Windows, Go translates to c:\tmp. If that path doesn't exist, then some node tests fail

    $requiredPaths = 'c:\tmp'

    $requiredPaths | ForEach-Object {
        Create-Directory -FullPath $_
    }
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
        [string]
        $Executable,
        [string[]]
        $ArgList,
        [int[]]
        $AllowedExitCodes = @(0),
        [int]
        $Retries = 1,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -lt $Retries; $i++) {
        Write-Log "Running $Executable $ArgList ..."
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

    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INVOKE_EXECUTABLE -ErrorMessage "Exhausted retries for $Executable $ArgList"
}

function Get-LogCollectionScripts {
    Write-Log "Getting various log collect scripts and depencencies"
    Create-Directory -FullPath 'c:\k\debug' -DirectoryUsage "storing debug scripts"
    DownloadFileOverHttp -Url 'https://github.com/Azure/AgentBaker/raw/master/vhdbuilder/scripts/windows/collect-windows-logs.ps1' -DestinationPath 'c:\k\debug\collect-windows-logs.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1' -DestinationPath 'c:\k\debug\collectlogs.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1' -DestinationPath 'c:\k\debug\dumpVfpPolicies.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/portReservationTest.ps1' -DestinationPath 'c:\k\debug\portReservationTest.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd' -DestinationPath 'c:\k\debug\starthnstrace.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd' -DestinationPath 'c:\k\debug\startpacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd' -DestinationPath 'c:\k\debug\stoppacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1' -DestinationPath 'c:\k\debug\VFP.psm1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1' -DestinationPath 'c:\k\debug\helper.psm1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1' -DestinationPath 'c:\k\debug\hns.psm1'
}

function Register-LogsCleanupScriptTask {
    Write-Log "Creating a scheduled task to run windowslogscleanup.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `+"`"+`"c:\k\windowslogscleanup.ps1`+"`"+`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -Daily -At "00:00" -DaysInterval 1
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "log-cleanup-task"
    Register-ScheduledTask -TaskName "log-cleanup-task" -InputObject $definition
}

function Register-NodeResetScriptTask {
    Write-Log "Creating a startup task to run windowsnodereset.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `+"`"+`"c:\k\windowsnodereset.ps1`+"`"+`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -AtStartup -RandomDelay 00:00:05
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "k8s-restart-job"
    Register-ScheduledTask -TaskName "k8s-restart-job" -InputObject $definition
}

# TODO ksubrmnn parameterize this fully
function Write-KubeClusterConfig {
    param(
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $KubeDnsServiceIp
    )

    $Global:ClusterConfiguration = [PSCustomObject]@{ }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Cri -Value @{
        Name   = $global:ContainerRuntime;
        Images = @{
            # e.g. "mcr.microsoft.com/oss/kubernetes/pause:1.4.1"
            "Pause" = $global:WindowsPauseImageURL
        }
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Cni -Value @{
        Name   = $global:NetworkPlugin;
        Plugin = @{
            Name = "bridge";
        };
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Csi -Value @{
        EnableProxy = $global:EnableCsiProxy
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Kubernetes -Value @{
        Source       = @{
            Release = $global:KubeBinariesVersion;
        };
        ControlPlane = @{
            IpAddress    = $MasterIP;
            Username     = "azureuser"
            MasterSubnet = $global:MasterSubnet
        };
        Network      = @{
            ServiceCidr = $global:KubeServiceCIDR;
            ClusterCidr = $global:KubeClusterCIDR;
            DnsIp       = $KubeDnsServiceIp
        };
        Kubelet      = @{
            NodeLabels = $global:KubeletNodeLabels;
            ConfigArgs = $global:KubeletConfigArgs
        };
        Kubeproxy    = @{
            FeatureGates = $global:KubeproxyFeatureGates
        };
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Install -Value @{
        Destination = "c:\k";
    }

    $Global:ClusterConfiguration | ConvertTo-Json -Depth 10 | Out-File -FilePath $global:KubeClusterConfigPath
}

function Assert-FileExists {
    Param(
        [Parameter(Mandatory = $true, Position = 0)][string]
        $Filename
    )

    if (-Not (Test-Path $Filename)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_FILE_NOT_EXIST -ErrorMessage "$Filename does not exist"
    }
}

function Update-DefenderPreferences {
    Add-MpPreference -ExclusionProcess "c:\k\kubelet.exe"

    if ($global:EnableCsiProxy) {
        Add-MpPreference -ExclusionProcess "c:\k\csi-proxy-server.exe"
    }

    if ($global:ContainerRuntime -eq 'containerd') {
        Add-MpPreference -ExclusionProcess "c:\program files\containerd\containerd.exe"
    }
}

function Check-APIServerConnectivity {
    Param(
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $false)][int]
        $RetryInterval = 1,
        [Parameter(Mandatory = $false)][int]
        $ConnectTimeout = 10,  #seconds
        [Parameter(Mandatory = $false)][int]
        $MaxRetryCount = 100
    )
    $retryCount=0

    do {
        try {
            $tcpClient=New-Object Net.Sockets.TcpClient
            Write-Log "Retry $retryCount : Trying to connect to API server $MasterIP"
            $tcpClient.ConnectAsync($MasterIP, 443).wait($ConnectTimeout*1000)
            if ($tcpClient.Connected) {
                $tcpClient.Close()
                Write-Log "Retry $retryCount : Connected to API server successfully"
                return
            }
            $tcpClient.Close()
        } catch {
            Write-Log "Retry $retryCount : Failed to connect to API server $MasterIP. Error: $_"
        }
        $retryCount++
        Write-Log "Retry $retryCount : Sleep $RetryInterval and then retry to connect to API server"
        Sleep $RetryInterval
    } while ($retryCount -lt $MaxRetryCount)

    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY -ErrorMessage "Failed to connect to API server $MasterIP after $retryCount retries"
}

function Get-CACertificates {
    try {
        Write-Log "Get CA certificates"
        $caFolder = "C:\ca"
        $uri = 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json'

        Create-Directory -FullPath $caFolder -DirectoryUsage "storing CA certificates"

        Write-Log "Download CA certificates rawdata"
        # This is required when the root CA certs are different for some clouds.
        try {
            $rawData = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$uri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CA_CERTIFICATES -ErrorMessage "Failed to download CA certificates rawdata. Error: $_"
        }

        Write-Log "Convert CA certificates rawdata"
        $caCerts=($rawData.Content) | ConvertFrom-Json
        if ([string]::IsNullOrEmpty($caCerts)) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_EMPTY_CA_CERTIFICATES -ErrorMessage "CA certificates rawdata is empty"
        }

        $certificates = $caCerts.Certificates
        for ($index = 0; $index -lt $certificates.Length ; $index++) {
            $name=$certificates[$index].Name
            $certFilePath = Join-Path $caFolder $name
            Write-Log "Write certificate $name to $certFilePath"
            $certificates[$index].CertBody > $certFilePath
        }
    }
    catch {
        # Catch all exceptions in this function. NOTE: exit cannot be caught.
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_CA_CERTIFICATES -ErrorMessage $_
    }
}`)

func windowsKuberneteswindowsfunctionsPs1Bytes() ([]byte, error) {
	return _windowsKuberneteswindowsfunctionsPs1, nil
}

func windowsKuberneteswindowsfunctionsPs1() (*asset, error) {
	bytes, err := windowsKuberneteswindowsfunctionsPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/kuberneteswindowsfunctions.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
$global:ContainerRuntime = "{{GetParameter "containerRuntime"}}"
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

$global:KubeproxyFeatureGates = @( {{GetKubeProxyFeatureGatesPsh}} )

$global:UseManagedIdentityExtension = "{{GetVariable "useManagedIdentityExtension"}}"
$global:UseInstanceMetadata = "{{GetVariable "useInstanceMetadata"}}"

$global:LoadBalancerSku = "{{GetVariable "loadBalancerSku"}}"
$global:ExcludeMasterFromStandardLB = "{{GetVariable "excludeMasterFromStandardLB"}}"


# Windows defaults, not changed by aks-engine
$global:CacheDir = "c:\akse-cache"
$global:KubeDir = "c:\k"
$global:HNSModule = [Io.path]::Combine("$global:KubeDir", "hns.psm1")

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

# Telemetry settings
$global:EnableTelemetry = [System.Convert]::ToBoolean("{{GetVariable "enableTelemetry" }}");
$global:TelemetryKey = "{{GetVariable "applicationInsightsKey" }}";

# CSI Proxy settings
$global:EnableCsiProxy = [System.Convert]::ToBoolean("{{GetVariable "windowsEnableCSIProxy" }}");
$global:CsiProxyUrl = "{{GetVariable "windowsCSIProxyURL" }}";

# Hosts Config Agent settings
$global:EnableHostsConfigAgent = [System.Convert]::ToBoolean("{{ EnableHostsConfigAgent }}");

$global:ProvisioningScriptsPackageUrl = "{{GetVariable "windowsProvisioningScriptsPackageURL" }}";

# PauseImage
$global:WindowsPauseImageURL = "{{GetVariable "windowsPauseImageURL" }}";
$global:AlwaysPullWindowsPauseImage = [System.Convert]::ToBoolean("{{GetVariable "alwaysPullWindowsPauseImage" }}");

# Calico
$global:WindowsCalicoPackageURL = "{{GetVariable "windowsCalicoPackageURL" }}";

# GMSA
$global:WindowsGmsaPackageUrl = "{{GetVariable "windowsGmsaPackageUrl" }}";

# TLS Bootstrap Token
$global:TLSBootstrapToken = "{{GetTLSBootstrapTokenForKubeConfig}}"

# Base64 representation of ZIP archive
$zippedFiles = "{{ GetKubernetesWindowsAgentFunctions }}"

# Extract ZIP from script
[io.file]::WriteAllBytes("scripts.zip", [System.Convert]::FromBase64String($zippedFiles))
Expand-Archive scripts.zip -DestinationPath "C:\\AzureData\\"

# Dot-source scripts with functions that are called in this script
. c:\AzureData\windows\kuberneteswindowsfunctions.ps1
. c:\AzureData\windows\windowsconfigfunc.ps1
. c:\AzureData\windows\windowskubeletfunc.ps1
. c:\AzureData\windows\windowscnifunc.ps1
. c:\AzureData\windows\windowsazurecnifunc.ps1
. c:\AzureData\windows\windowscsiproxyfunc.ps1
. c:\AzureData\windows\windowsinstallopensshfunc.ps1
. c:\AzureData\windows\windowscontainerdfunc.ps1
. c:\AzureData\windows\windowshostsconfigagentfunc.ps1
. c:\AzureData\windows\windowscalicofunc.ps1
. c:\AzureData\windows\windowscsehelper.ps1

$useContainerD = ($global:ContainerRuntime -eq "containerd")
$global:KubeClusterConfigPath = "c:\k\kubeclusterconfig.json"
$fipsEnabled = [System.Convert]::ToBoolean("{{ FIPSEnabled }}")
$windowsSecureTlsEnabled = [System.Convert]::ToBoolean("{{GetVariable "windowsSecureTlsEnabled" }}");

try
{
        # Exit early if the script has been executed
        if (Test-Path -Path $CSEResultFilePath -PathType Leaf) {
            Write-Log "The script has been executed before, will exit without doing anything."
            return
        }

        Write-Log ".\CustomDataSetupScript.ps1 -MasterIP $MasterIP -KubeDnsServiceIp $KubeDnsServiceIp -MasterFQDNPrefix $MasterFQDNPrefix -Location $Location -AADClientId $AADClientId -NetworkAPIVersion $NetworkAPIVersion -TargetEnvironment $TargetEnvironment"

        if ($global:EnableTelemetry) {
            $global:globalTimer = [System.Diagnostics.Stopwatch]::StartNew()

            $configAppInsightsClientTimer = [System.Diagnostics.Stopwatch]::StartNew()
            # Get app insights binaries and set up app insights client
            Create-Directory -FullPath c:\k\appinsights -DirectoryUsage "storing appinsights"
            DownloadFileOverHttp -Url "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg" -DestinationPath "c:\k\appinsights\microsoft.applicationinsights.2.11.0.zip"
            Expand-Archive -Path "c:\k\appinsights\microsoft.applicationinsights.2.11.0.zip" -DestinationPath "c:\k\appinsights"
            $appInsightsDll = "c:\k\appinsights\lib\net46\Microsoft.ApplicationInsights.dll"
            [Reflection.Assembly]::LoadFile($appInsightsDll)
            $conf = New-Object "Microsoft.ApplicationInsights.Extensibility.TelemetryConfiguration"
            $conf.DisableTelemetry = -not $global:EnableTelemetry
            $conf.InstrumentationKey = $global:TelemetryKey
            $global:AppInsightsClient = New-Object "Microsoft.ApplicationInsights.TelemetryClient"($conf)

            $global:AppInsightsClient.Context.Properties["correlation_id"] = New-Guid
            $global:AppInsightsClient.Context.Properties["cri"] = $global:ContainerRuntime
            # TODO: Update once containerd versioning story is decided
            $global:AppInsightsClient.Context.Properties["cri_version"] = if ($global:ContainerRuntime -eq "docker") { $global:DockerVersion } else { "" }
            $global:AppInsightsClient.Context.Properties["k8s_version"] = $global:KubeBinariesVersion
            $global:AppInsightsClient.Context.Properties["lb_sku"] = $global:LoadBalancerSku
            $global:AppInsightsClient.Context.Properties["location"] = $Location
            $global:AppInsightsClient.Context.Properties["os_type"] = "windows"
            $global:AppInsightsClient.Context.Properties["os_version"] = Get-WindowsVersion
            $global:AppInsightsClient.Context.Properties["network_plugin"] = $global:NetworkPlugin
            $global:AppInsightsClient.Context.Properties["network_plugin_version"] = Get-CniVersion
            $global:AppInsightsClient.Context.Properties["network_mode"] = $global:NetworkMode
            $global:AppInsightsClient.Context.Properties["subscription_id"] = $global:SubscriptionId

            $vhdId = ""
            if (Test-Path "c:\vhd-id.txt") {
                $vhdId = Get-Content "c:\vhd-id.txt"
            }
            $global:AppInsightsClient.Context.Properties["vhd_id"] = $vhdId

            $imdsProperties = Get-InstanceMetadataServiceTelemetry
            foreach ($key in $imdsProperties.keys) {
                $global:AppInsightsClient.Context.Properties[$key] = $imdsProperties[$key]
            }

            $configAppInsightsClientTimer.Stop()
            $global:AppInsightsClient.TrackMetric("Config-AppInsightsClient", $configAppInsightsClientTimer.Elapsed.TotalSeconds)
        }

        # Install OpenSSH if SSH enabled
        $sshEnabled = [System.Convert]::ToBoolean("{{ WindowsSSHEnabled }}")

        if ( $sshEnabled ) {
            Write-Log "Install OpenSSH"
            if ($global:EnableTelemetry) {
                $installOpenSSHTimer = [System.Diagnostics.Stopwatch]::StartNew()
            }
            Install-OpenSSH -SSHKeys $SSHKeys
            if ($global:EnableTelemetry) {
                $installOpenSSHTimer.Stop()
                $global:AppInsightsClient.TrackMetric("Install-OpenSSH", $installOpenSSHTimer.Elapsed.TotalSeconds)
            }
        }

        Write-Log "Apply telemetry data setting"
        Set-TelemetrySetting -WindowsTelemetryGUID $global:WindowsTelemetryGUID

        Write-Log "Resize os drive if possible"
        if ($global:EnableTelemetry) {
            $resizeTimer = [System.Diagnostics.Stopwatch]::StartNew()
        }
        Resize-OSDrive
        if ($global:EnableTelemetry) {
            $resizeTimer.Stop()
            $global:AppInsightsClient.TrackMetric("Resize-OSDrive", $resizeTimer.Elapsed.TotalSeconds)
        }

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

        if ($useContainerD) {
            Write-Log "Installing ContainerD"
            if ($global:EnableTelemetry) {
                $containerdTimer = [System.Diagnostics.Stopwatch]::StartNew()
            }
            $cniBinPath = $global:AzureCNIBinDir
            $cniConfigPath = $global:AzureCNIConfDir
            if ($global:NetworkPlugin -eq "kubenet") {
                $cniBinPath = $global:CNIPath
                $cniConfigPath = $global:CNIConfigPath
            }
            Install-Containerd -ContainerdUrl $global:ContainerdUrl -CNIBinDir $cniBinPath -CNIConfDir $cniConfigPath -KubeDir $global:KubeDir
            if ($global:EnableTelemetry) {
                $containerdTimer.Stop()
                $global:AppInsightsClient.TrackMetric("Install-ContainerD", $containerdTimer.Elapsed.TotalSeconds)
            }
        } else {
            Write-Log "Install docker"
            if ($global:EnableTelemetry) {
                $dockerTimer = [System.Diagnostics.Stopwatch]::StartNew()
            }
            Install-Docker -DockerVersion $global:DockerVersion
            Set-DockerLogFileOptions
            if ($global:EnableTelemetry) {
                $dockerTimer.Stop()
                $global:AppInsightsClient.TrackMetric("Install-Docker", $dockerTimer.Elapsed.TotalSeconds)
            }
        }

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

        Write-Log "Create the Pause Container kubletwin/pause"
        if ($global:EnableTelemetry) {
            $infraContainerTimer = [System.Diagnostics.Stopwatch]::StartNew()
        }
        New-InfraContainer -KubeDir $global:KubeDir -ContainerRuntime $global:ContainerRuntime
        if ($global:EnableTelemetry) {
            $infraContainerTimer.Stop()
            $global:AppInsightsClient.TrackMetric("New-InfraContainer", $infraContainerTimer.Elapsed.TotalSeconds)
        }

        if (-not (Test-ContainerImageExists -Image "kubletwin/pause" -ContainerRuntime $global:ContainerRuntime)) {
            Write-Log "Could not find container with name kubletwin/pause"
            if ($useContainerD) {
                $o = ctr -n k8s.io image list
                Write-Log $o
            } else {
                $o = docker image list
                Write-Log $o
            }
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_PAUSE_IMAGE_NOT_EXIST -ErrorMessage "kubletwin/pause container does not exist!"
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
            -IsDualStackEnabled $global:IsDualStackEnabled

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
            -KubeDir $global:KubeDir `+"`"+`
            -ContainerRuntime $global:ContainerRuntime

        Get-LogCollectionScripts

        Write-Log "Disable Internet Explorer compat mode and set homepage"
        Set-Explorer

        Write-Log "Adjust pagefile size"
        Adjust-PageFileSize

        Write-Log "Start preProvisioning script"
        PREPROVISION_EXTENSION

        Write-Log "Update service failure actions"
        Update-ServiceFailureActions -ContainerRuntime $global:ContainerRuntime
        Adjust-DynamicPortRange
        Register-LogsCleanupScriptTask
        Register-NodeResetScriptTask
        Update-DefenderPreferences

        if ($windowsSecureTlsEnabled) {
            Write-Host "Enable secure TLS protocols"
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

        if ($global:EnableTelemetry) {
            $global:globalTimer.Stop()
            $global:AppInsightsClient.TrackMetric("TotalDuration", $global:globalTimer.Elapsed.TotalSeconds)
            $global:AppInsightsClient.Flush()
        }

        if ($global:TLSBootstrapToken) {
            Write-Log "Removing temporary kube config"
            $kubeConfigFile = [io.path]::Combine($KubeDir, "config")
            Remove-Item $kubeConfigFile
        }

        # Postpone restart-computer so we can generate CSE response before restarting computer
        Write-Log "Setup Complete, reboot computer"
        Postpone-RestartComputer
}
catch
{
    if ($global:EnableTelemetry) {
        $exceptionTelemtry = New-Object "Microsoft.ApplicationInsights.DataContracts.ExceptionTelemetry"
        $exceptionTelemtry.Exception = $_.Exception
        $global:AppInsightsClient.TrackException($exceptionTelemtry)
        $global:AppInsightsClient.Flush()
    }

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

var _windowsWindowsazurecnifuncPs1 = []byte(`function
Install-VnetPlugins
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory=$true)][string]
        $AzureCNIBinDir,
        [Parameter(Mandatory=$true)][string]
        $VNetCNIPluginsURL
    )
    # Create CNI directories.
    Create-Directory -FullPath $AzureCNIBinDir -DirectoryUsage "storing Azure CNI binaries"
    Create-Directory -FullPath $AzureCNIConfDir -DirectoryUsage "storing Azure CNI configuration"

    # Download Azure VNET CNI plugins.
    # Mirror from https://github.com/Azure/azure-container-networking/releases
    $zipfile =  [Io.path]::Combine("$AzureCNIDir", "azure-vnet.zip")
    DownloadFileOverHttp -Url $VNetCNIPluginsURL -DestinationPath $zipfile
    Expand-Archive -path $zipfile -DestinationPath $AzureCNIBinDir
    del $zipfile

    # Windows does not need a separate CNI loopback plugin because the Windows
    # kernel automatically creates a loopback interface for each network namespace.
    # Copy CNI network config file and set bridge mode.
    move $AzureCNIBinDir/*.conflist $AzureCNIConfDir
}

function
Set-AzureCNIConfig
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory=$true)][string]
        $KubeDnsSearchPath,
        [Parameter(Mandatory=$true)][string]
        $KubeClusterCIDR,
        [Parameter(Mandatory=$true)][string]
        $KubeServiceCIDR,
        [Parameter(Mandatory=$true)][string]
        $VNetCIDR,
        [Parameter(Mandatory=$true)][bool]
        $IsDualStackEnabled
    )
    # Fill in DNS information for kubernetes.
    if ($IsDualStackEnabled){
        $subnetToPass = $KubeClusterCIDR -split ","
        $exceptionAddresses = @($subnetToPass[0])
    }
    else {
        $exceptionAddresses = @($KubeClusterCIDR)
    }
    $vnetCIDRs = $VNetCIDR -split ","
    foreach ($cidr in $vnetCIDRs) {
        $exceptionAddresses += $cidr
    }

    $fileName  = [Io.path]::Combine("$AzureCNIConfDir", "10-azure.conflist")
    $configJson = Get-Content $fileName | ConvertFrom-Json
    $configJson.plugins.dns.Nameservers[0] = $KubeDnsServiceIp
    $configJson.plugins.dns.Search[0] = $KubeDnsSearchPath

    $osBuildNumber = (get-wmiobject win32_operatingsystem).BuildNumber
    if ($osBuildNumber -le 17763){
        # In WS2019 and below rules in the exception list are generated by dropping the prefix lenght and removing duplicate rules.
        # If multiple execptions are specified with different ranges we should only include the broadest range for each address.
        # This issue has been addressed in 19h1+ builds

        $processedExceptions = GetBroadestRangesForEachAddress $exceptionAddresses
        Write-Log "Filtering CNI config exception list values to work around WS2019 issue processing rules. Original exception list: $exceptionAddresses, processed exception list: $processedExceptions"
        $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $processedExceptions
    }
    else {
        $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $exceptionAddresses
    }

    if ($IsDualStackEnabled){
        $configJson.plugins[0]|Add-Member -Name "ipv6Mode" -Value "ipv6nat" -MemberType NoteProperty
        $serviceCidr = $KubeServiceCIDR -split ","
        $configJson.plugins[0].AdditionalArgs[1].Value.DestinationPrefix = $serviceCidr[0]
        $valueObj = [PSCustomObject]@{
            Type = 'ROUTE'
            DestinationPrefix = $serviceCidr[1]
            NeedEncap = $True
        }

        $jsonContent = [PSCustomObject]@{
            Name = 'EndpointPolicy'
            Value = $valueObj
        }
        $configJson.plugins[0].AdditionalArgs += $jsonContent
    }
    else {
        $configJson.plugins[0].AdditionalArgs[1].Value.DestinationPrefix = $KubeServiceCIDR
    }

    $aclRule1 = [PSCustomObject]@{
        Type = 'ACL'
        Protocols = '6'
        Action = 'Block'
        Direction = 'Out'
        RemoteAddresses = '168.63.129.16/32'
        RemotePorts = '80'
        Priority = 200
        RuleType = 'Switch'
    }
    $aclRule2 = [PSCustomObject]@{
        Type = 'ACL'
        Action = 'Allow'
        Direction = 'In'
        Priority = 65500
    }
    $aclRule3 = [PSCustomObject]@{
        Type = 'ACL'
        Action = 'Allow'
        Direction = 'Out'
        Priority = 65500
    }
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule1
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule2
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule3
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent

    $configJson | ConvertTo-Json -depth 20 | Out-File -encoding ASCII -filepath $fileName
}

function GetBroadestRangesForEachAddress{
    param([string[]] $values)

    # Create a map of range values to IP addresses
    $map = @{}

    foreach ($value in $Values) {
        if ($value -match '([0-9\.]+)\/([0-9]+)') {
            if (!$map.contains($matches[1])) {
                $map.Add($matches[1], @())
            }

            $map[$matches[1]] += [int]$matches[2]
        }
    }

    # For each IP address select the range with the lagest scope (smallest value)
    $returnValues = @()
    foreach ($ip in $map.Keys) {
        $range = $map[$ip] | Sort-Object | Select-Object -First 1

        $returnValues += $ip + "/" + $range
    }

    # prefix $returnValues with common to ensure single values get returned as an array otherwise invalid json may be generated
    return ,$returnValues
}

function GetSubnetPrefix
{
    Param(
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $Token,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $SubnetId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $ResourceManagerEndpoint,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $NetworkAPIVersion
    )

    $uri = "$($ResourceManagerEndpoint)$($SubnetId)?api-version=$NetworkAPIVersion"
    $headers = @{Authorization="Bearer $Token"}

    try {
        $response = Retry-Command -Command "Invoke-RestMethod" -Args @{Uri=$uri; Method="Get"; ContentType="application/json"; Headers=$headers} -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_SUBNET_PREFIX -ErrorMessage "Error getting subnet prefix. Error: $_"
    }

    $response.properties.addressPrefix
}

function GenerateAzureStackCNIConfig
{
    Param(
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $TenantId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $SubscriptionId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AADClientId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AADClientSecret,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $ResourceGroup,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $NetworkAPIVersion,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AzureEnvironmentFilePath,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $IdentitySystem,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $KubeDir

    )

    $networkInterfacesFile = "$KubeDir\network-interfaces.json"
    $azureCNIConfigFile = "$KubeDir\interfaces.json"
    $azureEnvironment = Get-Content $AzureEnvironmentFilePath | ConvertFrom-Json

    Write-Log "------------------------------------------------------------------------"
    Write-Log "Parameters"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "TenantId:                  $TenantId"
    Write-Log "SubscriptionId:            $SubscriptionId"
    Write-Log "AADClientId:               ..."
    Write-Log "AADClientSecret:           ..."
    Write-Log "ResourceGroup:             $ResourceGroup"
    Write-Log "NetworkAPIVersion:         $NetworkAPIVersion"
    Write-Log "ServiceManagementEndpoint: $($azureEnvironment.serviceManagementEndpoint)"
    Write-Log "ActiveDirectoryEndpoint:   $($azureEnvironment.activeDirectoryEndpoint)"
    Write-Log "ResourceManagerEndpoint:   $($azureEnvironment.resourceManagerEndpoint)"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "Variables"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "azureCNIConfigFile: $azureCNIConfigFile"
    Write-Log "networkInterfacesFile: $networkInterfacesFile"
    Write-Log "------------------------------------------------------------------------"

    Write-Log "Generating token for Azure Resource Manager"

    $tokenURL = ""
    if($IdentitySystem -ieq "adfs") {
        $tokenURL = "$($azureEnvironment.activeDirectoryEndpoint)adfs/oauth2/token"
    } else {
        $tokenURL = "$($azureEnvironment.activeDirectoryEndpoint)$TenantId/oauth2/token"
    }

    Add-Type -AssemblyName System.Web
    $encodedSecret = [System.Web.HttpUtility]::UrlEncode($AADClientSecret)

    $body = "grant_type=client_credentials&client_id=$AADClientId&client_secret=$encodedSecret&resource=$($azureEnvironment.serviceManagementEndpoint)"
    $args = @{Uri=$tokenURL; Method="Post"; Body=$body; ContentType='application/x-www-form-urlencoded'}
    try {
        $tokenResponse = Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GENERATE_TOKEN_FOR_ARM -ErrorMessage "Error generating token for Azure Resource Manager. Error: $_"
    }

    $token = $tokenResponse | Select-Object -ExpandProperty access_token

    Write-Log "Fetching network interface configuration for node"

    $interfacesUri = "$($azureEnvironment.resourceManagerEndpoint)subscriptions/$SubscriptionId/resourceGroups/$ResourceGroup/providers/Microsoft.Network/networkInterfaces?api-version=$NetworkAPIVersion"
    $headers = @{Authorization="Bearer $token"}
    $args = @{Uri=$interfacesUri; Method="Get"; ContentType="application/json"; Headers=$headers; OutFile=$networkInterfacesFile}
    try {
        Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST -ErrorMessage "Error fetching network interface configuration for node. Error: $_"
    }

    Write-Log "Generating Azure CNI interface file"

    $localNics = Get-NetAdapter | Select-Object -ExpandProperty MacAddress | ForEach-Object {$_ -replace "-",""}

    $sdnNics = Get-Content $networkInterfacesFile `+"`"+`
        | ConvertFrom-Json `+"`"+`
        | Select-Object -ExpandProperty value `+"`"+`
        | Where-Object { $localNics.Contains($_.properties.macAddress) } `+"`"+`
        | Where-Object { $_.properties.ipConfigurations.Count -gt 0}

    $interfaces = @{
        Interfaces = @( $sdnNics | ForEach-Object { @{
            MacAddress = $_.properties.macAddress
            IsPrimary = $_.properties.primary
            IPSubnets = @(@{
                Prefix = GetSubnetPrefix `+"`"+`
                            -Token $token `+"`"+`
                            -SubnetId $_.properties.ipConfigurations[0].properties.subnet.id `+"`"+`
                            -NetworkAPIVersion $NetworkAPIVersion `+"`"+`
                            -ResourceManagerEndpoint $($azureEnvironment.resourceManagerEndpoint)
                IPAddresses = $_.properties.ipConfigurations | ForEach-Object { @{
                    Address = $_.properties.privateIPAddress
                    IsPrimary = $_.properties.primary
                }}
            })
        }})
    }

    ConvertTo-Json $interfaces -Depth 6 | Out-File -FilePath $azureCNIConfigFile -Encoding ascii

    Set-ItemProperty -Path $azureCNIConfigFile -Name IsReadOnly -Value $true
}

function New-ExternalHnsNetwork
{
    param (
        [Parameter(Mandatory=$true)][bool]
        $IsDualStackEnabled
    )

    Write-Log "Creating new HNS network `+"`"+`"ext`+"`"+`""
    $externalNetwork = "ext"
    $na = @(Get-NetAdapter -Physical)

    if ($na.Count -eq 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NETWORK_ADAPTER_NOT_EXIST -ErrorMessage "Failed to find any physical network adapters"
    }

    # If there is more than one adapter, use the first adapter.
    $managementIP = (Get-NetIPAddress -ifIndex $na[0].ifIndex -AddressFamily IPv4).IPAddress
    $adapterName = $na[0].Name
    Write-Log "Using adapter $adapterName with IP address $managementIP"
    $mgmtIPAfterNetworkCreate

    $stopWatch = New-Object System.Diagnostics.Stopwatch
    $stopWatch.Start()

    # Fixme : use a smallest range possible, that will not collide with any pod space
    if ($IsDualStackEnabled) {
        New-HNSNetwork -Type $global:NetworkMode -AddressPrefix @("192.168.255.0/30","192:168:255::0/127") -Gateway @("192.168.255.1","192:168:255::1") -AdapterName $adapterName -Name $externalNetwork -Verbose
    }
    else {
        New-HNSNetwork -Type $global:NetworkMode -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -AdapterName $adapterName -Name $externalNetwork -Verbose
    }
    # Wait for the switch to be created and the ip address to be assigned.
    for ($i = 0; $i -lt 60; $i++) {
        $mgmtIPAfterNetworkCreate = Get-NetIPAddress $managementIP -ErrorAction SilentlyContinue
        if ($mgmtIPAfterNetworkCreate) {
            break
        }
        Start-Sleep -Milliseconds 500
    }

    $stopWatch.Stop()
    if (-not $mgmtIPAfterNetworkCreate) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_MANAGEMENT_IP_NOT_EXIST -ErrorMessage "Failed to find $managementIP after creating $externalNetwork network"
    }
    Write-Log "It took $($StopWatch.Elapsed.Seconds) seconds to create the $externalNetwork network."
}
`)

func windowsWindowsazurecnifuncPs1Bytes() ([]byte, error) {
	return _windowsWindowsazurecnifuncPs1, nil
}

func windowsWindowsazurecnifuncPs1() (*asset, error) {
	bytes, err := windowsWindowsazurecnifuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowsazurecnifunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowsazurecnifuncTestsPs1 = []byte(`. $PSScriptRoot\windowsazurecnifunc.ps1

Describe 'GetBroadestRangesForEachAddress' {

    It "Values '<Values>' should return '<Expected>'" -TestCases @(
        @{ Values = @('10.240.0.0/12', '10.0.0.0/8'); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
        @{ Values = @('10.0.0.0/8', '10.0.0.0/16'); Expected = @('10.0.0.0/8')}
        @{ Values = @('10.0.0.0/16', '10.240.0.0/12', '10.0.0.0/8' ); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
        @{ Values = @(); Expected = @()}
        @{ Values = @('foobar'); Expected = @()}
    ){
        param ($Values, $Expected)

        $actual = GetBroadestRangesForEachAddress -values $Values
        $actual | Should -Be $Expected
    }
}`)

func windowsWindowsazurecnifuncTestsPs1Bytes() ([]byte, error) {
	return _windowsWindowsazurecnifuncTestsPs1, nil
}

func windowsWindowsazurecnifuncTestsPs1() (*asset, error) {
	bytes, err := windowsWindowsazurecnifuncTestsPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowsazurecnifunc.tests.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowscalicofuncPs1 = []byte(`function Get-CalicoPackage {
    param(
        [parameter(Mandatory=$true)] $RootDir
    )

    Write-Log "Getting Calico package"
    DownloadFileOverHttp -Url $global:WindowsCalicoPackageURL -DestinationPath 'c:\calicowindows.zip'
    Expand-Archive -Path 'c:\calicowindows.zip' -DestinationPath $RootDir -Force
    Remove-Item -Path 'c:\calicowindows.zip' -Force
}

function Set-CalicoStaticRules {
    param(
        [parameter(Mandatory=$true)] $CalicoRootDir
    )
    $fileName  = [Io.path]::Combine("$CalicoRootDir", "static-rules.json")
    echo '{
    "Provider": "AKS",
    "Rules": [
        {
            "Name": "EndpointPolicy",
            "Rule": {
                "Action": "Block",
                "Direction": "Out",
                "Id": "block-wireserver",
                "Priority": 200,
                "Protocol": 6,
                "RemoteAddresses": "168.63.129.16/32",
                "RemotePorts": "80",
                "RuleType": "Switch",
                "Type": "ACL"
            }
        }
    ],
    "version": "0.1.0"
}' | Out-File -encoding ASCII -filepath $fileName
}

function SetConfigParameters {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $OldString,
        [parameter(Mandatory=$true)] $NewString
    )

    (Get-Content $RootDir\config.ps1).replace($OldString, $NewString) | Set-Content $RootDir\config.ps1 -Force
}

function GetCalicoKubeConfig {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $CalicoNamespace,
        [parameter(Mandatory=$false)] $SecretName = "calico-node",
        [parameter(Mandatory=$false)] $KubeConfigPath = "c:\\k\\config"
    )

    # When creating Windows agent pools with the system Linux agent pool, the service account for calico may not be available in provisioning Windows agent nodes.
    # So we need to wait here until the service account for calico is available
    $name=""
    $retryCount=0
    $retryInterval=5
    $maxRetryCount=120 # 10 minutes

    do {
        try {
            Write-Log "Retry $retryCount : Trying to get service account $SecretName"
            $name=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret -n $CalicoNamespace --field-selector=type=kubernetes.io/service-account-token --no-headers -o custom-columns=":metadata.name" | findstr $SecretName | select -first 1
            if (![string]::IsNullOrEmpty($name)) {
                break
            }
        } catch {
            Write-Log "Retry $retryCount : Failed to get service account $SecretName. Error: $_"
        }
        $retryCount++
        Write-Log "Retry $retryCount : Sleep $retryInterval and then retry to get service account $SecretName"
        Sleep $retryInterval
    } while ($retryCount -lt $maxRetryCount)

    if ([string]::IsNullOrEmpty($name)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CALICO_SERVICE_ACCOUNT_NOT_EXIST -ErrorMessage "$SecretName service account does not exist."
    }

    $ca=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret/$name -o jsonpath='{.data.ca\.crt}' -n $CalicoNamespace
    $tokenBase64=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret/$name -o jsonpath='{.data.token}' -n $CalicoNamespace
    $token=[System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($tokenBase64))

    $server=findstr https:// $KubeConfigPath

    (Get-Content $RootDir\calico-kube-config.template).replace('<ca>', $ca).replace('<server>', $server.Trim()).replace('<token>', $token) | Set-Content $RootDir\calico-kube-config -Force
}

function Start-InstallCalico {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $KubeServiceCIDR,
        [parameter(Mandatory=$true)] $KubeDnsServiceIp,
        [parameter(Mandatory=$false)] $CalicoNs = "calico-system"
    )

    Write-Log "Download Calico"
    Get-CalicoPackage -RootDir $RootDir

    $CalicoDir  = [Io.path]::Combine("$RootDir", "CalicoWindows")

    Set-CalicoStaticRules -CalicoRootDir $CalicoDir

    SetConfigParameters -RootDir $CalicoDir -OldString "<your datastore type>" -NewString "kubernetes"
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd endpoints>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd key>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd ca cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your service cidr>" -NewString $KubeServiceCIDR
    SetConfigParameters -RootDir $CalicoDir -OldString "<your dns server ips>" -NewString $KubeDnsServiceIp
    SetConfigParameters -RootDir $CalicoDir -OldString "CALICO_NETWORKING_BACKEND=`+"`"+`"vxlan`+"`"+`"" -NewString "CALICO_NETWORKING_BACKEND=`+"`"+`"none`+"`"+`""
    SetConfigParameters -RootDir $CalicoDir -OldString "KUBE_NETWORK = `+"`"+`"Calico.*`+"`"+`"" -NewString "KUBE_NETWORK = `+"`"+`"azure.*`+"`"+`""

    GetCalicoKubeConfig -RootDir $CalicoDir -CalicoNamespace $CalicoNs

    Write-Log "Install Calico"

    pushd $CalicoDir
    .\install-calico.ps1
    popd
}
`)

func windowsWindowscalicofuncPs1Bytes() ([]byte, error) {
	return _windowsWindowscalicofuncPs1, nil
}

func windowsWindowscalicofuncPs1() (*asset, error) {
	bytes, err := windowsWindowscalicofuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowscalicofunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowscnifuncPs1 = []byte(`function Get-HnsPsm1
{
    Param(
        [string]
        $HnsUrl = "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1",
        [Parameter(Mandatory=$true)][string]
        $HNSModule
    )
    DownloadFileOverHttp -Url $HnsUrl -DestinationPath "$HNSModule"
}

# TODO: Move the code that creates the wincni configuration file out of windowskubeletfunc.ps1 and put it here`)

func windowsWindowscnifuncPs1Bytes() ([]byte, error) {
	return _windowsWindowscnifuncPs1, nil
}

func windowsWindowscnifuncPs1() (*asset, error) {
	bytes, err := windowsWindowscnifuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowscnifunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowsconfigfuncPs1 = []byte(`

# Set the service telemetry GUID. This is used with Windows Analytics https://docs.microsoft.com/en-us/sccm/core/clients/manage/monitor-windows-analytics
function Set-TelemetrySetting
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $WindowsTelemetryGUID
    )
    Set-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Policies\DataCollection" -Name "CommercialId" -Value $WindowsTelemetryGUID -Force
}

# Resize the system partition to the max available size. Azure can resize a managed disk, but the VM still needs to extend the partition boundary
function Resize-OSDrive
{
    $osDrive = ((Get-WmiObject Win32_OperatingSystem).SystemDrive).TrimEnd(":")
    $size = (Get-Partition -DriveLetter $osDrive).Size
    $maxSize = (Get-PartitionSupportedSize -DriveLetter $osDrive).SizeMax
    if ($size -lt $maxSize)
    {
        Resize-Partition -DriveLetter $osDrive -Size $maxSize
    }
}

# https://docs.microsoft.com/en-us/powershell/module/storage/new-partition
function Initialize-DataDisks
{
    Get-Disk | Where-Object PartitionStyle -eq 'raw' | Initialize-Disk -PartitionStyle MBR -PassThru | New-Partition -UseMaximumSize -AssignDriveLetter | Format-Volume -FileSystem NTFS -Force
}

# Set the Internet Explorer to use the latest rendering mode on all sites
# https://docs.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/microsoft-windows-ie-internetexplorer-intranetcompatibilitymode
# (This only affects installations with UI)
function Set-Explorer
{
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer"
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation" -Name IntranetCompatibilityMode -Value 0 -Type DWord
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main" -Name "Start Page" -Type String -Value http://bing.com
}

function Install-Docker
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $DockerVersion
    )

    Write-Log "Docker version $DockerVersion found, clearing DOCKER_API_VERSION"
    [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', $null, [System.EnvironmentVariableTarget]::Machine)

    try {
        $installDocker = $true
        $dockerService = Get-Service | ? Name -like 'docker'
        if ($dockerService.Count -eq 0) {
            Write-Log "Docker is not installed. Install docker version($DockerVersion)."
        }
        else {
            $dockerServerVersion = docker version --format '{{.Server.Version}}'
            Write-Log "Docker service is installed with docker version($dockerServerVersion)."
            if ($dockerServerVersion -eq $DockerVersion) {
                $installDocker = $false
                Write-Log "Same version docker installed will skip installing docker version($dockerServerVersion)."
            }
            else {
                Write-Log "Same version docker is not installed. Will install docker version($DockerVersion)."
            }
        }

        if ($installDocker) {
            Find-Package -Name Docker -ProviderName DockerMsftProvider -RequiredVersion $DockerVersion -ErrorAction Stop
            Write-Log "Found version $DockerVersion. Installing..."
            Install-Package -Name Docker -ProviderName DockerMsftProvider -Update -Force -RequiredVersion $DockerVersion
            net start docker
            Write-Log "Installed version $DockerVersion"
        }
    } catch {
        Write-Log "Error while installing package: $_.Exception.Message"
        $currentDockerVersion = (Get-Package -Name Docker -ProviderName DockerMsftProvider).Version
        Write-Log "Not able to install docker version. Using default version $currentDockerVersion"
    }
}

function Set-DockerLogFileOptions {
    Write-Log "Updating log file options in docker config"
    $dockerConfigPath = "C:\ProgramData\docker\config\daemon.json"

    if (-not (Test-Path $dockerConfigPath)) {
        "{}" | Out-File $dockerConfigPath
    }

    $dockerConfig = Get-Content $dockerConfigPath | ConvertFrom-Json
    $dockerConfig | Add-Member -Name "log-driver" -Value "json-file" -MemberType NoteProperty
    $logOpts = @{ "max-size" = "50m"; "max-file" = "5" }
    $dockerConfig | Add-Member -Name "log-opts" -Value $logOpts -MemberType NoteProperty
    $dockerConfig = $dockerConfig | ConvertTo-Json -Depth 10

    Write-Log "New docker config:"
    Write-Log $dockerConfig

    # daemon.json MUST be encoded as UTF8-no-BOM!
    Remove-Item $dockerConfigPath
    $fileEncoding = New-Object System.Text.UTF8Encoding $false
    [IO.File]::WriteAllLInes($dockerConfigPath, $dockerConfig, $fileEncoding)

    Restart-Service docker
}

# Pagefile adjustments
function Adjust-PageFileSize()
{
    wmic pagefileset set InitialSize=8096,MaximumSize=8096
}

function Adjust-DynamicPortRange()
{
    # Kube-proxy reserves 63 ports per service which limits clusters with Windows nodes
    # to ~225 services if default port reservations are used.
    # https://docs.microsoft.com/en-us/virtualization/windowscontainers/kubernetes/common-problems#load-balancers-are-plumbed-inconsistently-across-the-cluster-nodes
    # Kube-proxy load balancing should be set to DSR mode when it releases with future versions of the OS

    Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "set", "dynamicportrange", "tcp", "16385", "49151")
}

# TODO: should this be in this PR?
# Service start actions. These should be split up later and included in each install step
function Update-ServiceFailureActions
{
    Param(
        [Parameter(Mandatory = $true)][string]
        $ContainerRuntime
    )
    sc.exe failure "kubelet" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "kubeproxy" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure $ContainerRuntime actions= restart/60000/restart/60000/restart/60000 reset= 900
}

function Add-SystemPathEntry
{
    Param(
        [Parameter(Mandatory = $true)][string]
        $Directory
    )
    # update the path variable if it doesn't have the needed paths
    $path = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
    $updated = $false
    if(-not ($path -match $Directory.Replace("\","\\")+"(;|$)"))
    {
        $path += ";"+$Directory
        $updated = $true
    }
    if($updated)
    {
        Write-Log "Updating path, added $Directory"
        [Environment]::SetEnvironmentVariable("Path", $path, [EnvironmentVariableTarget]::Machine)
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
    }
}

function Enable-FIPSMode
{
    Param(
        [Parameter(Mandatory = $true)][bool]
        $FipsEnabled
    )

    if ( $FipsEnabled ) {
        Write-Log "Set the registry to enable fips-mode"
        Set-ItemProperty -Path "HKLM:\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy" -Name "Enabled" -Value 1 -Type DWORD -Force
    }
    else
    {
        Write-Log "Leave FipsAlgorithmPolicy as it is."
    }
}

function Enable-Privilege {
    param($Privilege)
    $Definition = @'
  using System;
  using System.Runtime.InteropServices;
  public class AdjPriv {
    [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern bool AdjustTokenPrivileges(IntPtr htok, bool disall,
      ref TokPriv1Luid newst, int len, IntPtr prev, IntPtr rele);
    [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern bool OpenProcessToken(IntPtr h, int acc, ref IntPtr phtok);
    [DllImport("advapi32.dll", SetLastError = true)]
    internal static extern bool LookupPrivilegeValue(string host, string name,
      ref long pluid);
    [StructLayout(LayoutKind.Sequential, Pack = 1)]
    internal struct TokPriv1Luid {
      public int Count;
      public long Luid;
      public int Attr;
    }
    internal const int SE_PRIVILEGE_ENABLED = 0x00000002;
    internal const int TOKEN_QUERY = 0x00000008;
    internal const int TOKEN_ADJUST_PRIVILEGES = 0x00000020;
    public static bool EnablePrivilege(long processHandle, string privilege) {
      bool retVal;
      TokPriv1Luid tp;
      IntPtr hproc = new IntPtr(processHandle);
      IntPtr htok = IntPtr.Zero;
      retVal = OpenProcessToken(hproc, TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY,
        ref htok);
      tp.Count = 1;
      tp.Luid = 0;
      tp.Attr = SE_PRIVILEGE_ENABLED;
      retVal = LookupPrivilegeValue(null, privilege, ref tp.Luid);
      retVal = AdjustTokenPrivileges(htok, false, ref tp, 0, IntPtr.Zero,
        IntPtr.Zero);
      return retVal;
    }
  }
'@
    $ProcessHandle = (Get-Process -id $pid).Handle
    $type = Add-Type $definition -PassThru
    $type[0]::EnablePrivilege($processHandle, $Privilege)
}

function Install-GmsaPlugin {
    Param(
        [Parameter(Mandatory=$true)]
        [String] $GmsaPackageUrl
    )

    $tempInstallPackageFoler = [Io.path]::Combine($env:TEMP, "CCGAKVPlugin")
    $tempPluginZipFile = [Io.path]::Combine($ENV:TEMP, "gmsa.zip")

    Write-Log "Getting the GMSA plugin package"
    DownloadFileOverHttp -Url $GmsaPackageUrl -DestinationPath $tempPluginZipFile
    Expand-Archive -Path $tempPluginZipFile -DestinationPath $tempInstallPackageFoler -Force
    if ($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE -ErrorMessage "Failed to extract the '$tempPluginZipFile' archive."
    }
    Remove-Item -Path $tempPluginZipFile -Force

    # Copy the plugin DLL file.
    Write-Log "Installing the GMSA plugin"
    Copy-Item -Force -Path "$tempInstallPackageFoler\CCGAKVPlugin.dll" -Destination "${env:SystemRoot}\System32\"

    # Enable the PowerShell privilege to set the registry permissions.
    Write-Log "Enabling the PowerShell privilege"
    $enablePrivilegeResponse=$false
    for($i = 0; $i -lt 10; $i++) {
        Write-Log "Retry $i : Trying to enable the PowerShell privilege"
        $enablePrivilegeResponse = Enable-Privilege -Privilege "SeTakeOwnershipPrivilege"
        if ($enablePrivilegeResponse) {
            break
        }
        Start-Sleep 1
    }
    if(!$enablePrivilegeResponse) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE -ErrorMessage "Failed to enable the PowerShell privilege."
    }

    # Set the registry permissions.
    Write-Log "Setting GMSA plugin registry permissions"
    try {
        $ccgKeyPath = "System\CurrentControlSet\Control\CCG\COMClasses"
        $owner = [System.Security.Principal.NTAccount]"BUILTIN\Administrators"

        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            $ccgKeyPath,
            [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
            [System.Security.AccessControl.RegistryRights]::TakeOwnership)
        $acl = $key.GetAccessControl()
        $acl.SetOwner($owner)
        $key.SetAccessControl($acl)
        
        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            $ccgKeyPath,
            [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
            [System.Security.AccessControl.RegistryRights]::ChangePermissions)
        $acl = $key.GetAccessControl()
        $rule = New-Object System.Security.AccessControl.RegistryAccessRule(
            $owner,
            [System.Security.AccessControl.RegistryRights]::FullControl,
            [System.Security.AccessControl.AccessControlType]::Allow)
        $acl.SetAccessRule($rule)
        $key.SetAccessControl($acl)
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION -ErrorMessage "Failed to set GMSA plugin registry permissions. $_"
    }
  
    # Set the appropriate registry values.
    try {
        Write-Log "Setting the appropriate GMSA plugin registry values"
        reg.exe import "$tempInstallPackageFoler\registerplugin.reg" 2>$null 1>$null
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES -ErrorMessage  "Failed to set GMSA plugin registry values. $_"
    }

    # Enable the logging manifest.
    Write-Log "Importing the CCGEvents manifest file"
    wevtutil.exe im "$tempInstallPackageFoler\CCGEvents.man"
    if ($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS -ErrorMessage "Failed to import the CCGEvents.man manifest file. $LASTEXITCODE"
    }

    Write-Log "Removing $tempInstallPackageFoler"
    Remove-Item -Path $tempInstallPackageFoler -Force -Recurse

    Write-Log "Successfully installed the GMSA plugin"
}
`)

func windowsWindowsconfigfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowsconfigfuncPs1, nil
}

func windowsWindowsconfigfuncPs1() (*asset, error) {
	bytes, err := windowsWindowsconfigfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowsconfigfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowscontainerdfuncPs1 = []byte(`# this is $global to persist across all functions since this is dot-sourced
$global:ContainerdInstallLocation = "$Env:ProgramFiles\containerd"
$global:Containerdbinary = (Join-Path $global:ContainerdInstallLocation containerd.exe)

function RegisterContainerDService {
  Param(
    [Parameter(Mandatory = $true)][string]
    $kubedir
  )

  Assert-FileExists $global:Containerdbinary

  # in the past service was not installed via nssm so remove it in case
  $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
  if ($null -ne $svc) {
    sc.exe delete containerd
  }

  Write-Log "Registering containerd as a service"
  # setup containerd
  & "$KubeDir\nssm.exe" install containerd $global:Containerdbinary | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppDirectory $KubeDir | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd DisplayName containerd | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Description containerd | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Start SERVICE_DEMAND_START | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd ObjectName LocalSystem | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppThrottle 1500 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppStdout "$KubeDir\containerd.log" | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppStderr "$KubeDir\containerd.err.log" | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateFiles 1 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateOnline 1 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateSeconds 86400 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateBytes 10485760 | RemoveNulls

  $retryCount=0
  $retryInterval=10
  $maxRetryCount=6 # 1 minutes

  do {
    $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED -ErrorMessage "containerd.exe did not get installed as a service correctly."
    }
    if ($svc.Status -eq "Running") {
      break
    }
    Write-Log "Starting containerd, current status: $svc.Status"
    Start-Service containerd
    $retryCount++
    Write-Log "Retry $retryCount : Sleep $retryInterval and check containerd status"
    Sleep $retryInterval
  } while ($retryCount -lt $maxRetryCount)

  if ($svc.Status -ne "Running") {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_RUNNING -ErrorMessage "containerd service is not running"
  }

}

function CreateHypervisorRuntime {
  Param(
    [Parameter(Mandatory = $true)][string]
    $image,
    [Parameter(Mandatory = $true)][string]
    $version,
    [Parameter(Mandatory = $true)][string]
    $buildNumber
  )

  return @"
        [plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber]
          runtime_type = "io.containerd.runhcs.v1"
          [plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber.options]
            Debug = true
            DebugType = 2
            SandboxImage = "$image-windows-$version-amd64"
            SandboxPlatform = "windows/amd64"
            SandboxIsolation = 1
            ScaleCPULimitsToSandbox = true
"@
}

function CreateHypervisorRuntimes {
  Param(
    [Parameter(Mandatory = $true)][string[]]
    $builds,
    [Parameter(Mandatory = $true)][string]
    $image
  )
  
  Write-Log "Adding hyperv runtimes $builds"
  $hypervRuntimes = ""
  ForEach ($buildNumber in $builds) {
    $windowsVersion = Select-Windows-Version -buildNumber $buildNumber
    $runtime = createHypervisorRuntime -image $pauseImage -version $windowsVersion -buildNumber $buildNumber
    if ($hypervRuntimes -eq "") {
      $hypervRuntimes = $runtime
    }
    else {
      $hypervRuntimes = $hypervRuntimes + "`+"`"+`r`+"`"+`n" + $runtime
    }
  }

  return $hypervRuntimes
}

function Select-Windows-Version {
  param (
    [Parameter()]
    [string]
    $buildNumber
  )

  switch ($buildNumber) {
    "17763" { return "1809" }
    "18362" { return "1903" }
    "18363" { return "1909" }
    "19041" { return "2004" }
    Default { return "" } 
  }
}

function Enable-Logging {
  if ((Test-Path "$global:ContainerdInstallLocation\diag.ps1") -And (Test-Path "$global:ContainerdInstallLocation\ContainerPlatform.wprp")) {
    $logs = Join-path $pwd.drive.Root logs
    Write-Log "Containerd hyperv logging enabled; temp location $logs"
    $diag = Join-Path $global:ContainerdInstallLocation diag.ps1
    Create-Directory -FullPath $logs -DirectoryUsage "storing containerd logs"
    # !ContainerPlatformPersistent profile is made to work with long term and boot tracing
    & $diag -Start -ProfilePath "$global:ContainerdInstallLocation\ContainerPlatform.wprp!ContainerPlatformPersistent" -TempPath $logs
  }
  else {
    Write-Log "Containerd hyperv logging script not avalaible"
  }
}

function Install-Containerd {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir
  )

  $svc = Get-Service -Name containerd -ErrorAction SilentlyContinue
  if ($null -ne $svc) {
    Write-Log "Stoping containerd service"
    $svc | Stop-Service
  }

  # TODO: check if containerd is already installed and is the same version before this.
  
  # Extract the package
  if ($ContainerdUrl.endswith(".zip")) {
    $zipfile = [Io.path]::Combine($ENV:TEMP, "containerd.zip")
    DownloadFileOverHttp -Url $ContainerdUrl -DestinationPath $zipfile
    Expand-Archive -path $zipfile -DestinationPath $global:ContainerdInstallLocation -Force
    Remove-Item -Path $zipfile -Force
  }
  elseif ($ContainerdUrl.endswith(".tar.gz")) {
    # upstream containerd package is a tar 
    $tarfile = [Io.path]::Combine($ENV:TEMP, "containerd.tar.gz")
    DownloadFileOverHttp -Url $ContainerdUrl -DestinationPath $tarfile
    Create-Directory -FullPath $global:ContainerdInstallLocation -DirectoryUsage "storing containerd"
    tar -xzf $tarfile -C $global:ContainerdInstallLocation

    mv -Force $global:ContainerdInstallLocation\bin\* $global:ContainerdInstallLocation\
    Remove-Item -Path $tarfile -Force
    Remove-Item -Path $global:ContainerdInstallLocation\bin -Force -Recurse
  }

  # get configuration options
  Add-SystemPathEntry $global:ContainerdInstallLocation
  $configFile = [Io.Path]::Combine($global:ContainerdInstallLocation, "config.toml")
  $clusterConfig = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
  $pauseImage = $clusterConfig.Cri.Images.Pause
  $formatedbin = $(($CNIBinDir).Replace("\", "/"))
  $formatedconf = $(($CNIConfDir).Replace("\", "/"))
  $sandboxIsolation = 0
  $windowsVersion = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").ReleaseId
  $hypervRuntimes = ""
  $hypervHandlers = $global:ContainerdWindowsRuntimeHandlers.split(",", [System.StringSplitOptions]::RemoveEmptyEntries)

  # configure
  if ($global:DefaultContainerdWindowsSandboxIsolation -eq "hyperv") {
    Write-Log "default runtime for containerd set to hyperv"
    $sandboxIsolation = 1
  }

  $template = Get-Content -Path "c:\AzureData\windows\containerdtemplate.toml" 
  if ($sandboxIsolation -eq 0 -And $hypervHandlers.Count -eq 0) {
    # remove the value hypervisor place holder
    $template = $template | Select-String -Pattern 'hypervisors' -NotMatch | Out-String
  }
  else {
    $hypervRuntimes = CreateHypervisorRuntimes -builds @($hypervHandlers) -image $pauseImage
  }

  $template.Replace('{{sandboxIsolation}}', $sandboxIsolation).
  Replace('{{pauseImage}}', $pauseImage).
  Replace('{{hypervisors}}', $hypervRuntimes).
  Replace('{{cnibin}}', $formatedbin).
  Replace('{{cniconf}}', $formatedconf).
  Replace('{{currentversion}}', $windowsVersion) | `+"`"+`
    Out-File -FilePath "$configFile" -Encoding ascii

  RegisterContainerDService -KubeDir $KubeDir
  Enable-Logging
}
`)

func windowsWindowscontainerdfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowscontainerdfuncPs1, nil
}

func windowsWindowscontainerdfuncPs1() (*asset, error) {
	bytes, err := windowsWindowscontainerdfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowscontainerdfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowscsehelperPs1 = []byte(`# Define all exit codes in Windows CSE

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

function Postpone-RestartComputer
{
    Write-Log "Creating an one-time task to restart the VM"
    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument " -Command `+"`"+`"Restart-Computer -Force`+"`"+`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    # trigger this task once
    $trigger = New-JobTrigger -At  (Get-Date).AddSeconds(15).DateTime -Once
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "Restart computer after provisioning the VM"
    Register-ScheduledTask -TaskName "restart-computer" -InputObject $definition
    Write-Log "Created an one-time task to restart the VM"
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

var _windowsWindowscsiproxyfuncPs1 = []byte(`function New-CsiProxyService {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CsiProxyPackageUrl,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )

    $tempdir = New-TemporaryDirectory
    $binaryPackage = "$tempdir\csiproxy.tar"

    DownloadFileOverHttp -Url $CsiProxyPackageUrl -DestinationPath $binaryPackage

    tar -xzf $binaryPackage -C $tempdir
    cp "$tempdir\bin\csi-proxy.exe" "$KubeDir\csi-proxy.exe"

    del $tempdir -Recurse

    & "$KubeDir\nssm.exe" install csi-proxy "$KubeDir\csi-proxy.exe" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppDirectory "$KubeDir" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Description csi-proxy | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStdout "$KubeDir\csi-proxy.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStderr "$KubeDir\csi-proxy.err.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateBytes 10485760 | RemoveNulls
}`)

func windowsWindowscsiproxyfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowscsiproxyfuncPs1, nil
}

func windowsWindowscsiproxyfuncPs1() (*asset, error) {
	bytes, err := windowsWindowscsiproxyfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowscsiproxyfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowshostsconfigagentfuncPs1 = []byte(`function New-HostsConfigService {
    $HostsConfigParameters = [io.path]::Combine($KubeDir, "hostsconfigagent.ps1")

    & "$KubeDir\nssm.exe" install hosts-config-agent C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppDirectory "$KubeDir" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppParameters $HostsConfigParameters | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Description hosts-config-agent | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStdout "$KubeDir\hosts-config-agent.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStderr "$KubeDir\hosts-config-agent.err.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateBytes 10485760 | RemoveNulls
}`)

func windowsWindowshostsconfigagentfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowshostsconfigagentfuncPs1, nil
}

func windowsWindowshostsconfigagentfuncPs1() (*asset, error) {
	bytes, err := windowsWindowshostsconfigagentfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowshostsconfigagentfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowsinstallopensshfuncPs1 = []byte(`function
Install-OpenSSH {
    Param(
        [Parameter(Mandatory = $true)][string[]] 
        $SSHKeys
    )

    $adminpath = "c:\ProgramData\ssh"
    $adminfile = "administrators_authorized_keys"

    $sshdService = Get-Service | ? Name -like 'sshd'
    if ($sshdService.Count -eq 0)
    {
        Write-Log "Installing OpenSSH"
        $isAvailable = Get-WindowsCapability -Online | ? Name -like 'OpenSSH*'

        if (!$isAvailable) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED -ErrorMessage "OpenSSH is not available on this machine"
        }

        Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    }
    else
    {
        Write-Log "OpenSSH Server service detected - skipping online install..."
    }

    Start-Service sshd

    if (!(Test-Path "$adminpath")) {
        Write-Log "Created new file and text content added"
        New-Item -path $adminpath -name $adminfile -type "file" -value ""
    }

    Write-Log "$adminpath found."
    Write-Log "Adding keys to: $adminpath\$adminfile ..."
    $SSHKeys | foreach-object {
        Add-Content $adminpath\$adminfile $_
    }

    Write-Log "Setting required permissions..."
    icacls $adminpath\$adminfile /remove "NT AUTHORITY\Authenticated Users"
    icacls $adminpath\$adminfile /inheritance:r
    icacls $adminpath\$adminfile /grant SYSTEM:`+"`"+`(F`+"`"+`)
    icacls $adminpath\$adminfile /grant BUILTIN\Administrators:`+"`"+`(F`+"`"+`)

    Write-Log "Restarting sshd service..."
    Restart-Service sshd
    # OPTIONAL but recommended:
    Set-Service -Name sshd -StartupType 'Automatic'

    # Confirm the Firewall rule is configured. It should be created automatically by setup. 
    $firewall = Get-NetFirewallRule -Name *ssh*

    if (!$firewall) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED -ErrorMessage "OpenSSH's firewall is not configured properly"
    }
    Write-Log "OpenSSH installed and configured successfully"
}
`)

func windowsWindowsinstallopensshfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowsinstallopensshfuncPs1, nil
}

func windowsWindowsinstallopensshfuncPs1() (*asset, error) {
	bytes, err := windowsWindowsinstallopensshfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowsinstallopensshfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _windowsWindowskubeletfuncPs1 = []byte(`function
Write-AzureConfig {
    Param(

        [Parameter(Mandatory = $true)][string]
        $AADClientId,
        [Parameter(Mandatory = $true)][string]
        $AADClientSecret,
        [Parameter(Mandatory = $true)][string]
        $TenantId,
        [Parameter(Mandatory = $true)][string]
        $SubscriptionId,
        [Parameter(Mandatory = $true)][string]
        $ResourceGroup,
        [Parameter(Mandatory = $true)][string]
        $Location,
        [Parameter(Mandatory = $true)][string]
        $VmType,
        [Parameter(Mandatory = $true)][string]
        $SubnetName,
        [Parameter(Mandatory = $true)][string]
        $SecurityGroupName,
        [Parameter(Mandatory = $true)][string]
        $VNetName,
        [Parameter(Mandatory = $true)][string]
        $RouteTableName,
        [Parameter(Mandatory = $false)][string] # Need one of these configured
        $PrimaryAvailabilitySetName,
        [Parameter(Mandatory = $false)][string] # Need one of these configured
        $PrimaryScaleSetName,
        [Parameter(Mandatory = $true)][string]
        $UseManagedIdentityExtension,
        [string]
        $UserAssignedClientID,
        [Parameter(Mandatory = $true)][string]
        $UseInstanceMetadata,
        [Parameter(Mandatory = $true)][string]
        $LoadBalancerSku,
        [Parameter(Mandatory = $true)][string]
        $ExcludeMasterFromStandardLB,
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        [Parameter(Mandatory = $true)][string]
        $TargetEnvironment,
        [Parameter(Mandatory = $false)][bool]
        $UseContainerD = $false
    )

    if ( -Not $PrimaryAvailabilitySetName -And -Not $PrimaryScaleSetName ) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INVALID_PARAMETER_IN_AZURE_CONFIG -ErrorMessage "Either PrimaryAvailabilitySetName or PrimaryScaleSetName must be set"
    }

    $azureConfigFile = [io.path]::Combine($KubeDir, "azure.json")

    $azureConfig = @"
{
    "cloud": "$TargetEnvironment",
    "tenantId": "$TenantId",
    "subscriptionId": "$SubscriptionId",
    "aadClientId": "$AADClientId",
    "aadClientSecret": "$AADClientSecret",
    "resourceGroup": "$ResourceGroup",
    "location": "$Location",
    "vmType": "$VmType",
    "subnetName": "$SubnetName",
    "securityGroupName": "$SecurityGroupName",
    "vnetName": "$VNetName",
    "routeTableName": "$RouteTableName",
    "primaryAvailabilitySetName": "$PrimaryAvailabilitySetName",
    "primaryScaleSetName": "$PrimaryScaleSetName",
    "useManagedIdentityExtension": $UseManagedIdentityExtension,
    "userAssignedIdentityID": "$UserAssignedClientID",
    "useInstanceMetadata": $UseInstanceMetadata,
    "loadBalancerSku": "$LoadBalancerSku",
    "excludeMasterFromStandardLB": $ExcludeMasterFromStandardLB
}
"@

    $azureConfig | Out-File -encoding ASCII -filepath "$azureConfigFile"
}


function
Write-CACert {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    $caFile = [io.path]::Combine($KubeDir, "ca.crt")
    [System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($CACertificate)) | Out-File -Encoding ascii $caFile
}

function
Write-KubeConfig {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $MasterFQDNPrefix,
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $AgentKey,
        [Parameter(Mandatory = $true)][string]
        $AgentCertificate,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    $kubeConfigFile = [io.path]::Combine($KubeDir, "config")

    $kubeConfig = @"
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "$CACertificate"
    server: https://${MasterIP}:443
  name: "$MasterFQDNPrefix"
contexts:
- context:
    cluster: "$MasterFQDNPrefix"
    user: "$MasterFQDNPrefix-admin"
  name: "$MasterFQDNPrefix"
current-context: "$MasterFQDNPrefix"
kind: Config
users:
- name: "$MasterFQDNPrefix-admin"
  user:
    client-certificate-data: "$AgentCertificate"
    client-key-data: "$AgentKey"
"@

    $kubeConfig | Out-File -encoding ASCII -filepath "$kubeConfigFile"
}

function
Write-BootstrapKubeConfig {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $MasterFQDNPrefix,
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $TLSBootstrapToken,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    $bootstrapKubeConfigFile = [io.path]::Combine($KubeDir, "bootstrap-config")

    $bootstrapKubeConfig = @"
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "$CACertificate"
    server: https://${MasterIP}:443
  name: "$MasterFQDNPrefix"
contexts:
- context:
    cluster: "$MasterFQDNPrefix"
    user: "kubelet-bootstrap"
  name: "$MasterFQDNPrefix"
current-context: "$MasterFQDNPrefix"
kind: Config
users:
- name: "kubelet-bootstrap"
  user:
    token: "$TLSBootstrapToken"
"@

    $bootstrapKubeConfig | Out-File -encoding ASCII -filepath "$bootstrapKubeConfigFIle"
}

function
Test-ContainerImageExists {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Image,
        [Parameter(Mandatory = $false)][string]
        $Tag,
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )

    $target = $Image
    if ($Tag) {
        $target += ":$Tag"
    }

    if ($ContainerRuntime -eq "docker") {
        $images = docker image list $target --format "{{json .}}"
        return $images.Count -gt 0
    }
    else {
        return ( (ctr.exe -n k8s.io images list) | Select-String $target) -ne $Null
    }
}

function
Build-PauseContainer {
    Param(
        [Parameter(Mandatory = $true)][string]
        $WindowsBase,
        $DestinationTag,
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )
    # Future work: This needs to build wincat - see https://github.com/Azure/aks-engine/issues/1461
    # Otherwise, delete this code and require a prebuilt pause image (or override with one from an Azure Container Registry instance)
    # ContainerD can't build, so doing the builds outside of node deployment is probably the right long-term solution.
    "FROM $($WindowsBase)" | Out-File -encoding ascii -FilePath Dockerfile
    "CMD cmd /c ping -t localhost" | Out-File -encoding ascii -FilePath Dockerfile -Append
    if ($ContainerRuntime -eq "docker") {
        Invoke-Executable -Executable "docker" -ArgList @("build", "-t", "$DestinationTag", ".")
    }
    else {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NO_DOCKER_TO_BUILD_PAUSE_CONTAINER -ErrorMessage "Cannot build pause container without Docker"
    }
}

function
New-InfraContainer {
    Param(
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        $DestinationTag = "kubletwin/pause",
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )
    cd $KubeDir
    $windowsVersion = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").ReleaseId

    # Reference for these tags: curl -L https://mcr.microsoft.com/v2/k8s/core/pause/tags/list
    # Then docker run --rm mplatform/manifest-tool inspect mcr.microsoft.com/k8s/core/pause:<tag>

    $clusterConfig = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
    $defaultPauseImage = $clusterConfig.Cri.Images.Pause

    $pauseImageVersions = @("1809", "1903", "1909", "2004", "2009", "20h2")

    if ($pauseImageVersions -icontains $windowsVersion) {
        if ($ContainerRuntime -eq "docker") {
            if (-not (Test-ContainerImageExists -Image $defaultPauseImage -ContainerRuntime $ContainerRuntime) -or $global:AlwaysPullWindowsPauseImage) {
                Invoke-Executable -Executable "docker" -ArgList @("pull", "$defaultPauseImage") -Retries 5 -RetryDelaySeconds 30
            }
            Invoke-Executable -Executable "docker" -ArgList @("tag", "$defaultPauseImage", "$DestinationTag")
        }
        else {
            # containerd
            if (-not (Test-ContainerImageExists -Image $defaultPauseImage -ContainerRuntime $ContainerRuntime) -or $global:AlwaysPullWindowsPauseImage) {
                Invoke-Executable -Executable "ctr" -ArgList @("-n", "k8s.io", "image", "pull", "$defaultPauseImage") -Retries 5 -RetryDelaySeconds 30
            }
            Invoke-Executable -Executable "ctr" -ArgList @("-n", "k8s.io", "image", "tag", "$defaultPauseImage", "$DestinationTag")
        }
    }
    else {
        Build-PauseContainer -WindowsBase "mcr.microsoft.com/nanoserver-insider" -DestinationTag $DestinationTag -ContainerRuntime $ContainerRuntime
    }
}

function
Test-ContainerImageExists {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Image,
        [Parameter(Mandatory = $false)][string]
        $Tag,
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )

    $target = $Image
    if ($Tag) {
        $target += ":$Tag"
    }

    if ($ContainerRuntime -eq "docker") {
        $images = docker image list $target --format "{{json .}}"
        return $images.Count -gt 0
    }
    else {
        return ( (ctr.exe -n k8s.io images list) | Select-String $target) -ne $Null
    }
}

# TODO: Deprecate this and replace with methods that get individual components instead of zip containing everything
# This expects the ZIP file created by Azure Pipelines.
function
Get-KubePackage {
    Param(
        [Parameter(Mandatory = $true)][string]
        $KubeBinariesSASURL
    )

    $zipfile = "c:\k.zip"
    for ($i = 0; $i -le 10; $i++) {
        DownloadFileOverHttp -Url $KubeBinariesSASURL -DestinationPath $zipfile
        if ($?) {
            break
        }
        else {
            Write-Log $Error[0].Exception.Message
        }
    }
    Expand-Archive -path $zipfile -DestinationPath C:\
    Remove-Item $zipfile
}

function
Get-KubeBinaries {
    Param(
        [Parameter(Mandatory = $true)][string]
        $KubeBinariesURL
    )

    $tempdir = New-TemporaryDirectory
    $binaryPackage = "$tempdir\k.tar.gz"
    for ($i = 0; $i -le 10; $i++) {
        DownloadFileOverHttp -Url $KubeBinariesURL -DestinationPath $binaryPackage
        if ($?) {
            break
        }
        else {
            Write-Log $Error[0].Exception.Message
        }
    }

    # using tar to minimize dependencies
    # tar should be avalible on 1803+
    tar -xzf $binaryPackage -C $tempdir

    # copy binaries over to kube folder
    $windowsbinariespath = "c:\k\"
    Create-Directory -FullPath $windowsbinariespath
    cp $tempdir\kubernetes\node\bin\* $windowsbinariespath -Recurse

    #remove temp folder created when unzipping
    del $tempdir -Recurse
}

# TODO: replace KubeletStartFile with a Kubelet config, remove NSSM, and use built-in service integration
function
New-NSSMService {
    Param(
        [string]
        [Parameter(Mandatory = $true)]
        $KubeDir,
        [string]
        [Parameter(Mandatory = $true)]
        $KubeletStartFile,
        [string]
        [Parameter(Mandatory = $true)]
        $KubeProxyStartFile,
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )

    $kubeletDependOnServices = $ContainerRuntime
    if ($global:EnableCsiProxy) {
        $kubeletDependOnServices += " csi-proxy"
    }
    if ($global:EnableHostsConfigAgent) {
        $kubeletDependOnServices += " hosts-config-agent"
    }

    # setup kubelet
    & "$KubeDir\nssm.exe" install Kubelet C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppParameters $KubeletStartFile | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet DisplayName Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Description Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStdout C:\k\kubelet.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStderr C:\k\kubelet.err.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateBytes 10485760 | RemoveNulls
    # Do not use & when calling DependOnService since 'docker csi-proxy'
    # is parsed as a single string instead of two separate strings
    Invoke-Expression "$KubeDir\nssm.exe set Kubelet DependOnService $kubeletDependOnServices | RemoveNulls"

    # setup kubeproxy
    & "$KubeDir\nssm.exe" install Kubeproxy C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppParameters $KubeProxyStartFile | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy DisplayName Kubeproxy | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy DependOnService Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Description Kubeproxy | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppStdout C:\k\kubeproxy.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppStderr C:\k\kubeproxy.err.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateBytes 10485760 | RemoveNulls
}

# Renamed from Write-KubernetesStartFiles
function
Install-KubernetesServices {
    param(
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        [Parameter(Mandatory = $false)][string]
        $ContainerRuntime = "docker"
    )

    # TODO ksbrmnn fix callers to this function

    $KubeletStartFile = [io.path]::Combine($KubeDir, "kubeletstart.ps1")
    $KubeProxyStartFile = [io.path]::Combine($KubeDir, "kubeproxystart.ps1")

    New-NSSMService -KubeDir $KubeDir `+"`"+`
        -KubeletStartFile $KubeletStartFile `+"`"+`
        -KubeProxyStartFile $KubeProxyStartFile `+"`"+`
        -ContainerRuntime $ContainerRuntime
}
`)

func windowsWindowskubeletfuncPs1Bytes() ([]byte, error) {
	return _windowsWindowskubeletfuncPs1, nil
}

func windowsWindowskubeletfuncPs1() (*asset, error) {
	bytes, err := windowsWindowskubeletfuncPs1Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "windows/windowskubeletfunc.ps1", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"linux/cloud-init/artifacts/10-componentconfig.conf":                   linuxCloudInitArtifacts10ComponentconfigConf,
	"linux/cloud-init/artifacts/10-containerd.conf":                        linuxCloudInitArtifacts10ContainerdConf,
	"linux/cloud-init/artifacts/10-httpproxy.conf":                         linuxCloudInitArtifacts10HttpproxyConf,
	"linux/cloud-init/artifacts/10-tlsbootstrap.conf":                      linuxCloudInitArtifacts10TlsbootstrapConf,
	"linux/cloud-init/artifacts/apt-preferences":                           linuxCloudInitArtifactsAptPreferences,
	"linux/cloud-init/artifacts/bind-mount.service":                        linuxCloudInitArtifactsBindMountService,
	"linux/cloud-init/artifacts/bind-mount.sh":                             linuxCloudInitArtifactsBindMountSh,
	"linux/cloud-init/artifacts/cis.sh":                                    linuxCloudInitArtifactsCisSh,
	"linux/cloud-init/artifacts/containerd-monitor.service":                linuxCloudInitArtifactsContainerdMonitorService,
	"linux/cloud-init/artifacts/containerd-monitor.timer":                  linuxCloudInitArtifactsContainerdMonitorTimer,
	"linux/cloud-init/artifacts/cse_cmd.sh":                                linuxCloudInitArtifactsCse_cmdSh,
	"linux/cloud-init/artifacts/cse_config.sh":                             linuxCloudInitArtifactsCse_configSh,
	"linux/cloud-init/artifacts/cse_helpers.sh":                            linuxCloudInitArtifactsCse_helpersSh,
	"linux/cloud-init/artifacts/cse_install.sh":                            linuxCloudInitArtifactsCse_installSh,
	"linux/cloud-init/artifacts/cse_main.sh":                               linuxCloudInitArtifactsCse_mainSh,
	"linux/cloud-init/artifacts/cse_start.sh":                              linuxCloudInitArtifactsCse_startSh,
	"linux/cloud-init/artifacts/dhcpv6.service":                            linuxCloudInitArtifactsDhcpv6Service,
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
	"linux/cloud-init/artifacts/kms.service":                               linuxCloudInitArtifactsKmsService,
	"linux/cloud-init/artifacts/krustlet.service":                          linuxCloudInitArtifactsKrustletService,
	"linux/cloud-init/artifacts/kubelet-monitor.service":                   linuxCloudInitArtifactsKubeletMonitorService,
	"linux/cloud-init/artifacts/kubelet-monitor.timer":                     linuxCloudInitArtifactsKubeletMonitorTimer,
	"linux/cloud-init/artifacts/kubelet.service":                           linuxCloudInitArtifactsKubeletService,
	"linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh":            linuxCloudInitArtifactsMarinerCse_helpers_marinerSh,
	"linux/cloud-init/artifacts/mariner/cse_install_mariner.sh":            linuxCloudInitArtifactsMarinerCse_install_marinerSh,
	"linux/cloud-init/artifacts/mig-partition.service":                     linuxCloudInitArtifactsMigPartitionService,
	"linux/cloud-init/artifacts/mig-partition.sh":                          linuxCloudInitArtifactsMigPartitionSh,
	"linux/cloud-init/artifacts/modprobe-CIS.conf":                         linuxCloudInitArtifactsModprobeCisConf,
	"linux/cloud-init/artifacts/nvidia-device-plugin.service":              linuxCloudInitArtifactsNvidiaDevicePluginService,
	"linux/cloud-init/artifacts/nvidia-docker-daemon.json":                 linuxCloudInitArtifactsNvidiaDockerDaemonJson,
	"linux/cloud-init/artifacts/nvidia-modprobe.service":                   linuxCloudInitArtifactsNvidiaModprobeService,
	"linux/cloud-init/artifacts/pam-d-common-auth":                         linuxCloudInitArtifactsPamDCommonAuth,
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
	"linux/cloud-init/artifacts/sysctl-d-60-CIS.conf":                      linuxCloudInitArtifactsSysctlD60CisConf,
	"linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh":              linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh,
	"linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh":              linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh,
	"linux/cloud-init/nodecustomdata.yml":                                  linuxCloudInitNodecustomdataYml,
	"windows/containerdtemplate.toml":                                      windowsContainerdtemplateToml,
	"windows/csecmd.ps1":                                                   windowsCsecmdPs1,
	"windows/kuberneteswindowsfunctions.ps1":                               windowsKuberneteswindowsfunctionsPs1,
	"windows/kuberneteswindowssetup.ps1":                                   windowsKuberneteswindowssetupPs1,
	"windows/windowsazurecnifunc.ps1":                                      windowsWindowsazurecnifuncPs1,
	"windows/windowsazurecnifunc.tests.ps1":                                windowsWindowsazurecnifuncTestsPs1,
	"windows/windowscalicofunc.ps1":                                        windowsWindowscalicofuncPs1,
	"windows/windowscnifunc.ps1":                                           windowsWindowscnifuncPs1,
	"windows/windowsconfigfunc.ps1":                                        windowsWindowsconfigfuncPs1,
	"windows/windowscontainerdfunc.ps1":                                    windowsWindowscontainerdfuncPs1,
	"windows/windowscsehelper.ps1":                                         windowsWindowscsehelperPs1,
	"windows/windowscsiproxyfunc.ps1":                                      windowsWindowscsiproxyfuncPs1,
	"windows/windowshostsconfigagentfunc.ps1":                              windowsWindowshostsconfigagentfuncPs1,
	"windows/windowsinstallopensshfunc.ps1":                                windowsWindowsinstallopensshfuncPs1,
	"windows/windowskubeletfunc.ps1":                                       windowsWindowskubeletfuncPs1,
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
				"10-componentconfig.conf":                   &bintree{linuxCloudInitArtifacts10ComponentconfigConf, map[string]*bintree{}},
				"10-containerd.conf":                        &bintree{linuxCloudInitArtifacts10ContainerdConf, map[string]*bintree{}},
				"10-httpproxy.conf":                         &bintree{linuxCloudInitArtifacts10HttpproxyConf, map[string]*bintree{}},
				"10-tlsbootstrap.conf":                      &bintree{linuxCloudInitArtifacts10TlsbootstrapConf, map[string]*bintree{}},
				"apt-preferences":                           &bintree{linuxCloudInitArtifactsAptPreferences, map[string]*bintree{}},
				"bind-mount.service":                        &bintree{linuxCloudInitArtifactsBindMountService, map[string]*bintree{}},
				"bind-mount.sh":                             &bintree{linuxCloudInitArtifactsBindMountSh, map[string]*bintree{}},
				"cis.sh":                                    &bintree{linuxCloudInitArtifactsCisSh, map[string]*bintree{}},
				"containerd-monitor.service":                &bintree{linuxCloudInitArtifactsContainerdMonitorService, map[string]*bintree{}},
				"containerd-monitor.timer":                  &bintree{linuxCloudInitArtifactsContainerdMonitorTimer, map[string]*bintree{}},
				"cse_cmd.sh":                                &bintree{linuxCloudInitArtifactsCse_cmdSh, map[string]*bintree{}},
				"cse_config.sh":                             &bintree{linuxCloudInitArtifactsCse_configSh, map[string]*bintree{}},
				"cse_helpers.sh":                            &bintree{linuxCloudInitArtifactsCse_helpersSh, map[string]*bintree{}},
				"cse_install.sh":                            &bintree{linuxCloudInitArtifactsCse_installSh, map[string]*bintree{}},
				"cse_main.sh":                               &bintree{linuxCloudInitArtifactsCse_mainSh, map[string]*bintree{}},
				"cse_start.sh":                              &bintree{linuxCloudInitArtifactsCse_startSh, map[string]*bintree{}},
				"dhcpv6.service":                            &bintree{linuxCloudInitArtifactsDhcpv6Service, map[string]*bintree{}},
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
				"kms.service":                               &bintree{linuxCloudInitArtifactsKmsService, map[string]*bintree{}},
				"krustlet.service":                          &bintree{linuxCloudInitArtifactsKrustletService, map[string]*bintree{}},
				"kubelet-monitor.service":                   &bintree{linuxCloudInitArtifactsKubeletMonitorService, map[string]*bintree{}},
				"kubelet-monitor.timer":                     &bintree{linuxCloudInitArtifactsKubeletMonitorTimer, map[string]*bintree{}},
				"kubelet.service":                           &bintree{linuxCloudInitArtifactsKubeletService, map[string]*bintree{}},
				"mariner": &bintree{nil, map[string]*bintree{
					"cse_helpers_mariner.sh": &bintree{linuxCloudInitArtifactsMarinerCse_helpers_marinerSh, map[string]*bintree{}},
					"cse_install_mariner.sh": &bintree{linuxCloudInitArtifactsMarinerCse_install_marinerSh, map[string]*bintree{}},
				}},
				"mig-partition.service":           &bintree{linuxCloudInitArtifactsMigPartitionService, map[string]*bintree{}},
				"mig-partition.sh":                &bintree{linuxCloudInitArtifactsMigPartitionSh, map[string]*bintree{}},
				"modprobe-CIS.conf":               &bintree{linuxCloudInitArtifactsModprobeCisConf, map[string]*bintree{}},
				"nvidia-device-plugin.service":    &bintree{linuxCloudInitArtifactsNvidiaDevicePluginService, map[string]*bintree{}},
				"nvidia-docker-daemon.json":       &bintree{linuxCloudInitArtifactsNvidiaDockerDaemonJson, map[string]*bintree{}},
				"nvidia-modprobe.service":         &bintree{linuxCloudInitArtifactsNvidiaModprobeService, map[string]*bintree{}},
				"pam-d-common-auth":               &bintree{linuxCloudInitArtifactsPamDCommonAuth, map[string]*bintree{}},
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
				"sysctl-d-60-CIS.conf":            &bintree{linuxCloudInitArtifactsSysctlD60CisConf, map[string]*bintree{}},
				"ubuntu": &bintree{nil, map[string]*bintree{
					"cse_helpers_ubuntu.sh": &bintree{linuxCloudInitArtifactsUbuntuCse_helpers_ubuntuSh, map[string]*bintree{}},
					"cse_install_ubuntu.sh": &bintree{linuxCloudInitArtifactsUbuntuCse_install_ubuntuSh, map[string]*bintree{}},
				}},
			}},
			"nodecustomdata.yml": &bintree{linuxCloudInitNodecustomdataYml, map[string]*bintree{}},
		}},
	}},
	"windows": &bintree{nil, map[string]*bintree{
		"containerdtemplate.toml":         &bintree{windowsContainerdtemplateToml, map[string]*bintree{}},
		"csecmd.ps1":                      &bintree{windowsCsecmdPs1, map[string]*bintree{}},
		"kuberneteswindowsfunctions.ps1":  &bintree{windowsKuberneteswindowsfunctionsPs1, map[string]*bintree{}},
		"kuberneteswindowssetup.ps1":      &bintree{windowsKuberneteswindowssetupPs1, map[string]*bintree{}},
		"windowsazurecnifunc.ps1":         &bintree{windowsWindowsazurecnifuncPs1, map[string]*bintree{}},
		"windowsazurecnifunc.tests.ps1":   &bintree{windowsWindowsazurecnifuncTestsPs1, map[string]*bintree{}},
		"windowscalicofunc.ps1":           &bintree{windowsWindowscalicofuncPs1, map[string]*bintree{}},
		"windowscnifunc.ps1":              &bintree{windowsWindowscnifuncPs1, map[string]*bintree{}},
		"windowsconfigfunc.ps1":           &bintree{windowsWindowsconfigfuncPs1, map[string]*bintree{}},
		"windowscontainerdfunc.ps1":       &bintree{windowsWindowscontainerdfuncPs1, map[string]*bintree{}},
		"windowscsehelper.ps1":            &bintree{windowsWindowscsehelperPs1, map[string]*bintree{}},
		"windowscsiproxyfunc.ps1":         &bintree{windowsWindowscsiproxyfuncPs1, map[string]*bintree{}},
		"windowshostsconfigagentfunc.ps1": &bintree{windowsWindowshostsconfigagentfuncPs1, map[string]*bintree{}},
		"windowsinstallopensshfunc.ps1":   &bintree{windowsWindowsinstallopensshfuncPs1, map[string]*bintree{}},
		"windowskubeletfunc.ps1":          &bintree{windowsWindowskubeletfuncPs1, map[string]*bintree{}},
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
