// Code generated for package templates by go-bindata DO NOT EDIT. (@generated)
// sources:
// linux/cloud-init/artifacts/apt-preferences
// linux/cloud-init/artifacts/auditd-rules
// linux/cloud-init/artifacts/cis.sh
// linux/cloud-init/artifacts/containerd.service
// linux/cloud-init/artifacts/cse_cmd.sh
// linux/cloud-init/artifacts/cse_config.sh
// linux/cloud-init/artifacts/cse_helpers.sh
// linux/cloud-init/artifacts/cse_install.sh
// linux/cloud-init/artifacts/cse_main.sh
// linux/cloud-init/artifacts/dhcpv6.service
// linux/cloud-init/artifacts/docker-monitor.service
// linux/cloud-init/artifacts/docker-monitor.timer
// linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf
// linux/cloud-init/artifacts/enable-dhcpv6.sh
// linux/cloud-init/artifacts/etc-issue
// linux/cloud-init/artifacts/etc-issue.net
// linux/cloud-init/artifacts/etcd.service
// linux/cloud-init/artifacts/health-monitor.sh
// linux/cloud-init/artifacts/init-aks-custom-cloud.sh
// linux/cloud-init/artifacts/kms.service
// linux/cloud-init/artifacts/kubelet-monitor.service
// linux/cloud-init/artifacts/kubelet-monitor.timer
// linux/cloud-init/artifacts/kubelet.service
// linux/cloud-init/artifacts/label-nodes.service
// linux/cloud-init/artifacts/label-nodes.sh
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
// linux/cloud-init/artifacts/sys-fs-bpf.mount
// linux/cloud-init/artifacts/sysctl-d-60-CIS.conf
// linux/cloud-init/nodecustomdata.yml
// windows/csecmd.ps1
// windows/kuberneteswindowsfunctions.ps1
// windows/kuberneteswindowssetup.ps1
// windows/windowsazurecnifunc.ps1
// windows/windowsazurecnifunc.tests.ps1
// windows/windowscnifunc.ps1
// windows/windowsconfigfunc.ps1
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

var _linuxCloudInitArtifactsAuditdRules = []byte(`# increase kernel audit buffers since we have a lot of rules
-b 8192

# 4.1.4 Ensure events that modify date and time information are collected
-a always,exit -F arch=b64 -S adjtimex -S settimeofday -k time-change
-a always,exit -F arch=b32 -S adjtimex -S settimeofday -S stime -k time-change
-a always,exit -F arch=b64 -S clock_settime -k time-change
-a always,exit -F arch=b32 -S clock_settime -k time-change
-w /etc/localtime -p wa -k time-change

# 4.1.5 Ensure events that modify user/group information are collected
-w /etc/group -p wa -k identity
-w /etc/passwd -p wa -k identity
-w /etc/gshadow -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/security/opasswd -p wa -k identity

# 4.1.6 Ensure events that modify the system's network environment are collected
-a always,exit -F arch=b64 -S sethostname -S setdomainname -k system-locale
-a always,exit -F arch=b32 -S sethostname -S setdomainname -k system-locale
-w /etc/issue -p wa -k system-locale
-w /etc/issue.net -p wa -k system-locale
-w /etc/hosts -p wa -k system-locale
-w /etc/network -p wa -k system-locale
-w /etc/networks -p wa -k system-locale

# 4.1.7 Ensure events that modify the system's Mandatory Access Controls are collected
-w /etc/selinux/ -p wa -k MAC-policy

# 4.1.8 Ensure login and logout events are collected
-w /var/log/faillog -p wa -k logins
-w /var/log/lastlog -p wa -k logins
-w /var/log/tallylog -p wa -k logins

# 4.1.9 Ensure session initiation information is collected
-w /var/run/utmp -p wa -k session
-w /var/log/wtmp -p wa -k session
-w /var/log/btmp -p wa -k session

# 4.1.10 Ensure discretionary access control permission modification events are collected
-a always,exit -F arch=b64 -S chmod -S fchmod -S fchmodat -F auid>=1000 -F auid!=4294967295 -k perm_mod
-a always,exit -F arch=b32 -S chmod -S fchmod -S fchmodat -F auid>=1000 -F auid!=4294967295 -k perm_mod
-a always,exit -F arch=b64 -S chown -S fchown -S fchownat -S lchown -F auid>=1000 -F auid!=4294967295 -k perm_mod
-a always,exit -F arch=b32 -S chown -S fchown -S fchownat -S lchown -F auid>=1000 -F auid!=4294967295 -k perm_mod
-a always,exit -F arch=b64 -S setxattr -S lsetxattr -S fsetxattr -S removexattr -S lremovexattr -S fremovexattr -F auid>=1000 -F auid!=4294967295 -k perm_mod
-a always,exit -F arch=b32 -S setxattr -S lsetxattr -S fsetxattr -S removexattr -S lremovexattr -S fremovexattr -F auid>=1000 -F auid!=4294967295 -k perm_mod

# 4.1.11 Ensure unsuccessful unauthorized file access attempts are collected
-a always,exit -F arch=b64 -S creat -S open -S openat -S truncate -S ftruncate -F exit=-EACCES -F auid>=1000 -F auid!=4294967295 -k access
-a always,exit -F arch=b32 -S creat -S open -S openat -S truncate -S ftruncate -F exit=-EACCES -F auid>=1000 -F auid!=4294967295 -k access
-a always,exit -F arch=b64 -S creat -S open -S openat -S truncate -S ftruncate -F exit=-EPERM -F auid>=1000 -F auid!=4294967295 -k access
-a always,exit -F arch=b32 -S creat -S open -S openat -S truncate -S ftruncate -F exit=-EPERM -F auid>=1000 -F auid!=4294967295 -k access

# 4.1.12 Ensure use of privileged commands is collected
-a always,exit -F path=/usr/lib/dbus-1.0/dbus-daemon-launch-helper -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/lib/openssh/ssh-keysign -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/lib/eject/dmcrypt-get-device -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/sudo -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/wall -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/ssh-agent -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/expiry -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/chfn -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/pkexec -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/screen -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/chsh -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/newgidmap -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/chage -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/crontab -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/at -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/newgrp -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/mlocate -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/gpasswd -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/newuidmap -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/passwd -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/usr/bin/bsd-write -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/umount -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/mount -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/ntfs-3g -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/ping6 -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/su -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/ping -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/bin/fusermount -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/sbin/pam_extrausers_chkpwd -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/sbin/mount.nfs -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged
-a always,exit -F path=/sbin/unix_chkpwd -F perm=x -F auid>=1000 -F auid!=4294967295  -k privileged

# 4.1.13 Ensure successful file system mounts are collected
-a always,exit -F arch=b64 -S mount -F auid>=1000 -F auid!=4294967295 -k mounts
-a always,exit -F arch=b32 -S mount -F auid>=1000 -F auid!=4294967295 -k mounts

# 4.1.14 Ensure file deletion events by users are collected
-a always,exit -F arch=b64 -S unlink -S unlinkat -S rename -S renameat -F auid>=1000 -F auid!=4294967295 -k delete
-a always,exit -F arch=b32 -S unlink -S unlinkat -S rename -S renameat -F auid>=1000 -F auid!=4294967295 -k delete

# 4.1.15 Ensure changes to system administration scope (sudoers) is collected
-w /etc/sudoers -p wa -k scope
-w /etc/sudoers.d -p wa -k scope

# 4.1.16 Ensure system administrator actions (sudolog) are collected
-w /var/log/sudo.log -p wa -k actions

# 4.1.17 Ensure kernel module loading and unloading is collected
-w /sbin/insmod -p x -k modules
-w /sbin/rmmod -p x -k modules
-w /sbin/modprobe -p x -k modules
-a always,exit -F arch=b64 -S init_module -S delete_module -k modules

# 4.1.18 Ensure the audit configuration is immutable
-e 2
`)

func linuxCloudInitArtifactsAuditdRulesBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsAuditdRules, nil
}

func linuxCloudInitArtifactsAuditdRules() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsAuditdRulesBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/auditd-rules", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
    chmod 644 /etc/default/grub || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    for filepath in /etc/crontab /etc/cron.hourly /etc/cron.daily /etc/cron.weekly /etc/cron.monthly /etc/cron.d; do
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

var _linuxCloudInitArtifactsContainerdService = []byte(`[Unit]
Description=containerd daemon
[Service]
ExecStart=/usr/bin/containerd
[Install]
WantedBy=multi-user.target
#EOF`)

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

var _linuxCloudInitArtifactsCse_cmdSh = []byte(`echo $(date),$(hostname);
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
CONTAINERD_DOWNLOAD_URL_BASE={{GetParameter "containerdDownloadURLBase"}}
NETWORK_MODE={{GetParameter "networkMode"}}
KUBE_BINARY_URL={{GetParameter "kubeBinaryURL"}}
USER_ASSIGNED_IDENTITY_ID={{GetVariable "userAssignedIdentityID"}}
API_SERVER_NAME={{GetKubernetesEndpoint}}
IS_VHD={{GetVariable "isVHD"}}
GPU_NODE={{GetVariable "gpuNode"}}
SGX_NODE={{GetVariable "sgxNode"}}
AUDITD_ENABLED={{GetVariable "auditdEnabled"}}
CONFIG_GPU_DRIVER_IF_NEEDED={{GetVariable "configGPUDriverIfNeeded"}}
ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED={{GetVariable "enableGPUDevicePluginIfNeeded"}}
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1 && systemctl --no-pager -l status kubelet 2>&1 | head -n 100"`)

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
if [[ $OS == $COREOS_OS_NAME ]]; then
    PRIVATE_IP=$(ip a show eth0 | grep -Po 'inet \K[\d.]+')
else
    PRIVATE_IP=$(hostname -I | cut -d' ' -f1)
fi
ETCD_PEER_URL="https://${PRIVATE_IP}:2380"
ETCD_CLIENT_URL="https://${PRIVATE_IP}:2379"

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
}

configureSecrets(){
    APISERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/apiserver.key"
    touch "${APISERVER_PRIVATE_KEY_PATH}"
    chmod 0600 "${APISERVER_PRIVATE_KEY_PATH}"
    chown root:root "${APISERVER_PRIVATE_KEY_PATH}"

    CA_PRIVATE_KEY_PATH="/etc/kubernetes/certs/ca.key"
    touch "${CA_PRIVATE_KEY_PATH}"
    chmod 0600 "${CA_PRIVATE_KEY_PATH}"
    chown root:root "${CA_PRIVATE_KEY_PATH}"

    ETCD_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/etcdserver.key"
    touch "${ETCD_SERVER_PRIVATE_KEY_PATH}"
    chmod 0600 "${ETCD_SERVER_PRIVATE_KEY_PATH}"
    if [[ -z "${COSMOS_URI}" ]]; then
      chown etcd:etcd "${ETCD_SERVER_PRIVATE_KEY_PATH}"
    fi

    ETCD_CLIENT_PRIVATE_KEY_PATH="/etc/kubernetes/certs/etcdclient.key"
    touch "${ETCD_CLIENT_PRIVATE_KEY_PATH}"
    chmod 0600 "${ETCD_CLIENT_PRIVATE_KEY_PATH}"
    chown root:root "${ETCD_CLIENT_PRIVATE_KEY_PATH}"

    ETCD_PEER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/etcdpeer${NODE_INDEX}.key"
    touch "${ETCD_PEER_PRIVATE_KEY_PATH}"
    chmod 0600 "${ETCD_PEER_PRIVATE_KEY_PATH}"
    if [[ -z "${COSMOS_URI}" ]]; then
      chown etcd:etcd "${ETCD_PEER_PRIVATE_KEY_PATH}"
    fi

    ETCD_SERVER_CERTIFICATE_PATH="/etc/kubernetes/certs/etcdserver.crt"
    touch "${ETCD_SERVER_CERTIFICATE_PATH}"
    chmod 0644 "${ETCD_SERVER_CERTIFICATE_PATH}"
    chown root:root "${ETCD_SERVER_CERTIFICATE_PATH}"

    ETCD_CLIENT_CERTIFICATE_PATH="/etc/kubernetes/certs/etcdclient.crt"
    touch "${ETCD_CLIENT_CERTIFICATE_PATH}"
    chmod 0644 "${ETCD_CLIENT_CERTIFICATE_PATH}"
    chown root:root "${ETCD_CLIENT_CERTIFICATE_PATH}"

    ETCD_PEER_CERTIFICATE_PATH="/etc/kubernetes/certs/etcdpeer${NODE_INDEX}.crt"
    touch "${ETCD_PEER_CERTIFICATE_PATH}"
    chmod 0644 "${ETCD_PEER_CERTIFICATE_PATH}"
    chown root:root "${ETCD_PEER_CERTIFICATE_PATH}"

    set +x
    echo "${APISERVER_PRIVATE_KEY}" | base64 --decode > "${APISERVER_PRIVATE_KEY_PATH}"
    echo "${CA_PRIVATE_KEY}" | base64 --decode > "${CA_PRIVATE_KEY_PATH}"
    echo "${ETCD_SERVER_PRIVATE_KEY}" | base64 --decode > "${ETCD_SERVER_PRIVATE_KEY_PATH}"
    echo "${ETCD_CLIENT_PRIVATE_KEY}" | base64 --decode > "${ETCD_CLIENT_PRIVATE_KEY_PATH}"
    echo "${ETCD_PEER_KEY}" | base64 --decode > "${ETCD_PEER_PRIVATE_KEY_PATH}"
    echo "${ETCD_SERVER_CERTIFICATE}" | base64 --decode > "${ETCD_SERVER_CERTIFICATE_PATH}"
    echo "${ETCD_CLIENT_CERTIFICATE}" | base64 --decode > "${ETCD_CLIENT_CERTIFICATE_PATH}"
    echo "${ETCD_PEER_CERT}" | base64 --decode > "${ETCD_PEER_CERTIFICATE_PATH}"
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

ensureAuditD() {
  if [[ "${AUDITD_ENABLED}" == true ]]; then
    systemctlEnableAndStart auditd || exit $ERR_SYSTEMCTL_START_FAIL
  else
    if apt list --installed | grep 'auditd'; then
      apt_get_purge 20 30 120 auditd &
    fi
  fi
}

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

{{- if IsDynamicKubeletEnabled}}
    set +x
    KUBELET_CONFIG_JSON_PATH="/etc/default/kubeletconfig.json"
    touch "${KUBELET_CONFIG_JSON_PATH}"
    chmod 0644 "${KUBELET_CONFIG_JSON_PATH}"
    chown root:root "${KUBELET_CONFIG_JSON_PATH}"
    cat << EOF > "${KUBELET_CONFIG_JSON_PATH}"
{{GetDynamicKubeletConfigFileContent}}
EOF
    set -x
{{- end}}
}

configureCNI() {
    {{/* needed for the iptables rules to work on bridges */}}
    retrycmd_if_failure 120 5 25 modprobe br_netfilter || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
    configureCNIIPTables
    {{if HasCiliumNetworkPlugin}}
    systemctl enable sys-fs-bpf.mount
    systemctl restart sys-fs-bpf.mount
    REBOOTREQUIRED=true
    {{end}}
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

ensureContainerRuntime() {
    if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
        ensureDocker
    fi
    {{if NeedsContainerd}}
        ensureContainerd
    {{end}}
}

{{if NeedsContainerd}}
ensureContainerd() {
  wait_for_file 1200 1 /etc/systemd/system/containerd.service.d/exec_start.conf || exit $ERR_FILE_WATCH_TIMEOUT
  wait_for_file 1200 1 /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  {{if IsKubenet }}
  wait_for_file 1200 1 /etc/sysctl.d/11-containerd.conf || exit $ERR_FILE_WATCH_TIMEOUT
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD
  {{end}}
  systemctl is-active --quiet docker && (systemctl_disable 20 30 120 docker || exit $ERR_SYSTEMD_DOCKER_STOP_FAIL)
  systemctlEnableAndStart containerd || exit $ERR_SYSTEMCTL_START_FAIL
}
{{end}}

ensureDocker() {
    DOCKER_SERVICE_EXEC_START_FILE=/etc/systemd/system/docker.service.d/exec_start.conf
    wait_for_file 1200 1 $DOCKER_SERVICE_EXEC_START_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    usermod -aG docker ${ADMINUSER}
    DOCKER_MOUNT_FLAGS_SYSTEMD_FILE=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
    if [[ $OS != $COREOS_OS_NAME ]]; then
        wait_for_file 1200 1 $DOCKER_MOUNT_FLAGS_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    fi
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
    systemctlEnableAndStart docker || exit $ERR_DOCKER_START_FAIL
    {{/* Delay start of docker-monitor for 30 mins after booting */}}
    DOCKER_MONITOR_SYSTEMD_TIMER_FILE=/etc/systemd/system/docker-monitor.timer
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_TIMER_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    DOCKER_MONITOR_SYSTEMD_FILE=/etc/systemd/system/docker-monitor.service
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart docker-monitor.timer || exit $ERR_SYSTEMCTL_START_FAIL
}

{{if EnableEncryptionWithExternalKms}}
ensureKMS() {
    systemctlEnableAndStart kms || exit $ERR_SYSTEMCTL_START_FAIL
}
{{end}}

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
    KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
    wait_for_file 1200 1 $KUBECONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    wait_for_file 1200 1 $KUBELET_RUNTIME_CONFIG_SCRIPT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart kubelet || exit $ERR_KUBELET_START_FAIL
    {{if HasCiliumNetworkPolicy}}
    while [ ! -f /etc/cni/net.d/05-cilium.conf ]; do
        sleep 3
    done
    {{end}}
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

ensureLabelNodes() {
    LABEL_NODES_SCRIPT_FILE=/opt/azure/containers/label-nodes.sh
    wait_for_file 1200 1 $LABEL_NODES_SCRIPT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    LABEL_NODES_SYSTEMD_FILE=/etc/systemd/system/label-nodes.service
    wait_for_file 1200 1 $LABEL_NODES_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart label-nodes || exit $ERR_SYSTEMCTL_START_FAIL
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

{{if HasNSeriesSKU}}
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
ERR_NTP_INSTALL_TIMEOUT=10 {{/*Unable to install NTP */}}
ERR_NTP_START_TIMEOUT=11 {{/* Unable to start NTP */}}
ERR_STOP_SYSTEMD_TIMESYNCD_TIMEOUT=12 {{/* Timeout waiting for systemd-timesyncd stop */}}
ERR_DOCKER_INSTALL_TIMEOUT=20 {{/* Timeout waiting for docker install */}}
ERR_DOCKER_DOWNLOAD_TIMEOUT=21 {{/* Timout waiting for docker downloads */}}
ERR_DOCKER_KEY_DOWNLOAD_TIMEOUT=22 {{/* Timeout waiting to download docker repo key */}}
ERR_DOCKER_APT_KEY_TIMEOUT=23 {{/* Timeout waiting for docker apt-key */}}
ERR_DOCKER_START_FAIL=24 {{/* Docker could not be started by systemctl */}}
ERR_MOBY_APT_LIST_TIMEOUT=25 {{/* Timeout waiting for moby apt sources */}}
ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT=26 {{/* Timeout waiting for MS GPG key download */}}
ERR_MOBY_INSTALL_TIMEOUT=27 {{/* Timeout waiting for moby install */}}
ERR_K8S_RUNNING_TIMEOUT=30 {{/* Timeout waiting for k8s cluster to be healthy */}}
ERR_K8S_DOWNLOAD_TIMEOUT=31 {{/* Timeout waiting for Kubernetes downloads */}}
ERR_KUBECTL_NOT_FOUND=32 {{/* kubectl client binary not found on local disk */}}
ERR_IMG_DOWNLOAD_TIMEOUT=33 {{/* Timeout waiting for img download */}}
ERR_KUBELET_START_FAIL=34 {{/* kubelet could not be started by systemctl */}}
ERR_CONTAINER_IMG_PULL_TIMEOUT=35 {{/* Timeout trying to pull a container image */}}
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

ERR_VHD_FILE_NOT_FOUND=124 {{/* VHD log file not found on VM built from VHD distro */}}
ERR_VHD_BUILD_ERROR=125 {{/* Reserved for VHD CI exit conditions */}}

{{/* Azure Stack specific errors */}}
ERR_AZURE_STACK_GET_ARM_TOKEN=120 {{/* Error generating a token to use with Azure Resource Manager */}}
ERR_AZURE_STACK_GET_NETWORK_CONFIGURATION=121 {{/* Error fetching the network configuration for the node */}}
ERR_AZURE_STACK_GET_SUBNET_PREFIX=122 {{/* Error fetching the subnet address prefix for a subnet ID */}}

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"
RHEL_OS_NAME="RHEL"
COREOS_OS_NAME="COREOS"
KUBECTL=/usr/local/bin/kubectl
DOCKER=/usr/bin/docker
export GPU_DV=450.51.06
export GPU_DEST=/usr/local/nvidia
NVIDIA_DOCKER_VERSION=2.0.3
DOCKER_VERSION=1.13.1-1
NVIDIA_CONTAINER_RUNTIME_VERSION=2.0.0
NVIDIA_DOCKER_SUFFIX=docker18.09.2-1

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
retrycmd_get_executable() {
    retries=$1; wait_sleep=$2; filepath=$3; url=$4; validation_args=$5
    echo "${retries} retries"
    for i in $(seq 1 $retries); do
        $filepath $validation_args && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            timeout 30 curl -fsSL $url -o $filepath
            chmod +x $filepath
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
    if [ systemctl list-units --full --all | grep -q "$1.service" ]; then
        systemctl_stop 20 5 25 $1 || echo "$1 could not be stopped"
        systemctl_disable 20 5 25 $1 || echo "$1 could not be disabled"
    fi
}

# return true if a >= b 
semverCompare() {
    VERSION_A=$(echo $1 | cut -d "+" -f 1)
    VERSION_B=$(echo $2 | cut -d "+" -f 1)
    [[ "${VERSION_A}" == "${VERSION_B}" ]] && return 0
    sorted=( $( echo ${VERSION_A} ${VERSION_B} | tr ' ' '\n' | sort -V ) )
    [[ "${VERSION_A}" == ${sorted[1]} ]] && return 0
    return 1
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
CONTAINERD_DOWNLOADS_DIR="/opt/containerd/downloads"
K8S_DOWNLOADS_DIR="/opt/kubernetes/downloads"
UBUNTU_RELEASE=$(lsb_release -r -s)

removeMoby() {
    apt-get purge -y moby-engine moby-cli
}

removeContainerd() {
    apt-get purge -y moby-containerd
}

cleanupContainerdDlFiles() {
    rm -rf $CONTAINERD_DOWNLOADS_DIR
}

installDeps() {
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL
    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for apt_package in apache2-utils apt-transport-https blobfuse=1.1.1 ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool fuse git glusterfs-client htop iftop init-system-helpers iotop iproute2 ipset iptables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysstat traceroute util-linux xz-utils zip; do
      if ! apt_get_install 30 1 600 $apt_package; then
        journalctl --no-pager -u $apt_package
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
    if [[ "${AUDITD_ENABLED}" == true ]]; then
      if ! apt_get_install 30 1 600 auditd; then
        journalctl --no-pager -u auditd
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    fi
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

installContainerRuntime() {
    if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
        installMoby
    fi
    {{if NeedsContainerd}}
        installContainerd
    {{end}}
}

installMoby() {
    CURRENT_VERSION=$(dockerd --version | grep "Docker version" | cut -d "," -f 1 | cut -d " " -f 3 | cut -d "+" -f 1)
    local MOBY_VERSION="19.03.12"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${MOBY_VERSION}; then
        echo "currently installed moby-docker version ${CURRENT_VERSION} is greater than (or equal to) target base version ${MOBY_VERSION}. skipping installMoby."
    else
        removeMoby
        getMobyPkg
        MOBY_CLI=${MOBY_VERSION}
        if [[ "${MOBY_CLI}" == "3.0.4" ]]; then
            MOBY_CLI="3.0.3"
        fi
        apt_get_install 20 30 120 moby-engine=${MOBY_VERSION}* moby-cli=${MOBY_CLI}* --allow-downgrades || exit $ERR_MOBY_INSTALL_TIMEOUT
    fi
}

{{if NeedsContainerd}}
# CSE+VHD can dicate the containerd version, users don't care as long as it works
installContainerd() {
    # azure-built runtimes have a "+azure" prefix in their version strings (i.e 1.3.7+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.3.7 is our lowest supported version of containerd
    local BASE_VERSION="1.3.7"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${BASE_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${BASE_VERSION}. skipping installContainerd."
    else 
        apt_get_purge 20 30 120 moby-engine || exit $ERR_MOBY_INSTALL_TIMEOUT 
        retrycmd_if_failure 30 5 3600 apt-get install -y moby-containerd=${BASE_VERSION}* || exit $ERR_MOBY_INSTALL_TIMEOUT
        rm -Rf $CONTAINERD_DOWNLOADS_DIR &
    fi  
}
{{end}}

getMobyPkg() {
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
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

downloadAzureCNI() {
    mkdir -p $CNI_DOWNLOADS_DIR
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadContainerd() {
    CONTAINERD_DOWNLOAD_URL="${CONTAINERD_DOWNLOAD_URL_BASE}cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz"
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_TGZ_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_get_tarball 120 5 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_TGZ_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
}

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

installImg() {
    img_filepath=/usr/local/bin/img
    retrycmd_get_executable 120 5 $img_filepath "https://acs-mirror.azureedge.net/img/img-linux-amd64-v0.5.6" ls || exit $ERR_IMG_DOWNLOAD_TIMEOUT
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
    if [[ "$CLI_TOOL" == "docker" ]]; then
        mkdir -p "$path"
        # Check if we can extract kubelet and kubectl directly from hyperkube's binary folder
        if docker run --rm --entrypoint "" -v $path:$path ${HYPERKUBE_URL} /bin/bash -c "cp /usr/local/bin/{kubelet,kubectl} $path"; then
            mv "$path/kubelet" "/usr/local/bin/kubelet-${KUBERNETES_VERSION}"
            mv "$path/kubectl" "/usr/local/bin/kubectl-${KUBERNETES_VERSION}"
            return
        else
            docker run --rm -v $path:$path ${HYPERKUBE_URL} /bin/bash -c "cp /hyperkube $path"
        fi
    else
        img unpack -o "$path" ${HYPERKUBE_URL}
    fi

    if [[ $OS == $COREOS_OS_NAME ]]; then
        cp "$path/hyperkube" "/opt/kubelet"
        mv "$path/hyperkube" "/opt/kubectl"
        chmod a+x /opt/kubelet /opt/kubectl
    else
        cp "$path/hyperkube" "/usr/local/bin/kubelet-${KUBERNETES_VERSION}"
        mv "$path/hyperkube" "/usr/local/bin/kubectl-${KUBERNETES_VERSION}"
    fi
}

installKubeletKubectlAndKubeProxy() {
    if [[ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ]]; then
        #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
        if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
            extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL}
        else
            if [[ "$CONTAINER_RUNTIME" == "docker" ]]; then
                extractHyperkube "docker"
            else
                installImg
                extractHyperkube "img"
            fi
        fi
    fi

    mv "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" "/usr/local/bin/kubelet"
    mv "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" "/usr/local/bin/kubectl"
    chmod a+x /usr/local/bin/kubelet /usr/local/bin/kubectl
    rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-* /home/hyperkube-downloads &

    if [ -n "${KUBEPROXY_URL}" ]; then
        #kubeproxy is a system addon that is dictated by control plane so it shouldn't block node provisioning
        pullContainerImage "docker" ${KUBEPROXY_URL} &
    fi
}

pullContainerImage() {
    CLI_TOOL=$1
    DOCKER_IMAGE_URL=$2
    retrycmd_if_failure 60 1 1200 $CLI_TOOL pull $DOCKER_IMAGE_URL || exit $ERR_CONTAINER_IMG_PULL_TIMEOUT
}

cleanUpHyperkubeImages() {
    echo $(date),$(hostname), startCleanUpHyperkubeImages
    function cleanUpHyperkubeImagesRun() {
        images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep 'hyperkube')
        local exit_code=$?
        if [[ $exit_code != 0 ]]; then
            exit $exit_code
        elif [[ "${images_to_delete}" != "" ]]; then
            docker rmi ${images_to_delete[@]}
        fi
    }
    export -f cleanUpHyperkubeImagesRun
    retrycmd_if_failure 10 5 120 bash -c cleanUpHyperkubeImagesRun
    echo $(date),$(hostname), endCleanUpHyperkubeImages
}

cleanUpKubeProxyImages() {
    echo $(date),$(hostname), startCleanUpKubeProxyImages
    function cleanUpKubeProxyImagesRun() {
        images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep 'kube-proxy')
        local exit_code=$?
        if [[ $exit_code != 0 ]]; then
            exit $exit_code
        elif [[ "${images_to_delete}" != "" ]]; then
            docker rmi ${images_to_delete[@]}
        fi
    }
    export -f cleanUpKubeProxyImagesRun
    retrycmd_if_failure 10 5 120 bash -c cleanUpKubeProxyImagesRun
    echo $(date),$(hostname), endCleanUpKubeProxyImages
}

cleanUpContainerImages() {
    # run cleanUpHyperkubeImages and cleanUpKubeProxyImages concurrently
    export -f retrycmd_if_failure
    export -f cleanUpHyperkubeImages
    export -f cleanUpKubeProxyImages
    export KUBERNETES_VERSION
    bash -c cleanUpHyperkubeImages &
    bash -c cleanUpKubeProxyImages &
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST
    rm -f /etc/apt/sources.list.d/nvidia-docker.list
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

wait_for_file 3600 1 {{GetCSEInstallScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEInstallScriptFilepath}}

wait_for_file 3600 1 {{GetCSEConfigScriptFilepath}} || exit $ERR_FILE_WATCH_TIMEOUT
source {{GetCSEConfigScriptFilepath}}

set +x
ETCD_PEER_CERT=$(echo ${ETCD_PEER_CERTIFICATES} | cut -d'[' -f 2 | cut -d']' -f 1 | cut -d',' -f $((${NODE_INDEX}+1)))
ETCD_PEER_KEY=$(echo ${ETCD_PEER_PRIVATE_KEYS} | cut -d'[' -f 2 | cut -d']' -f 1 | cut -d',' -f $((${NODE_INDEX}+1)))
set -x

if [[ $OS == $COREOS_OS_NAME ]]; then
    echo "Changing default kubectl bin location"
    KUBECTL=/opt/kubectl
fi

if [ -f /var/run/reboot-required ]; then
    REBOOTREQUIRED=true
else
    REBOOTREQUIRED=false
fi

configureAdminUser

{{- if not NeedsContainerd}}
cleanUpContainerd
{{end}}

if [[ "${GPU_NODE}" != "true" ]]; then
    cleanUpGPUDrivers
fi

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

if [[ $OS == $UBUNTU_OS_NAME ]]; then
    ensureAuditD
fi

installContainerRuntime

installNetworkPlugin

{{- if HasNSeriesSKU}}
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
docker login -u $SERVICE_PRINCIPAL_CLIENT_ID -p $SERVICE_PRINCIPAL_CLIENT_SECRET {{GetPrivateAzureRegistryServer}}
{{end}}

installKubeletKubectlAndKubeProxy

if [[ $OS != $COREOS_OS_NAME ]]; then
    ensureRPC
fi

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

ensureContainerRuntime

configureK8s

configureCNI

{{/* configure and enable dhcpv6 for dual stack feature */}}
{{- if IsIPv6DualStackFeatureEnabled}}
ensureDHCPv6
{{end}}

{{- if EnableHostsConfigAgent}}
configPrivateClusterHosts
{{- end}}

ensureKubelet
ensureJournal

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

API_SERVER_DNS_RETRIES=20
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
  API_SERVER_DNS_RETRIES=200
fi
{{- if not EnableHostsConfigAgent}}
RES=$(retrycmd_if_failure ${API_SERVER_DNS_RETRIES} 1 3 nslookup ${API_SERVER_NAME})
STS=$?
{{- else}}
STS=0
{{- end}}
if [[ $STS != 0 ]]; then
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
    retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 3 nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
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
ExecStart=/usr/local/bin/health-monitor.sh container-runtime
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
OnBootSec=30min
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

var _linuxCloudInitArtifactsEtcdService = []byte(`[Unit]
Description=etcd - highly-available key value store
Documentation=https://github.com/coreos/etcd
Documentation=man:etcd
After=network.target
Wants=network-online.target
RequiresMountsFor=/var/lib/etcddisk
[Service]
Environment=DAEMON_ARGS=
Environment=ETCD_NAME=%H
Environment=ETCD_DATA_DIR=
EnvironmentFile=-/etc/default/%p
Type=notify
User=etcd
PermissionsStartOnly=true
ExecStart=/usr/bin/etcd $DAEMON_ARGS
Restart=always
[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsEtcdServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsEtcdService, nil
}

func linuxCloudInitArtifactsEtcdService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsEtcdServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/etcd.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
  local -r crictl="${KUBE_HOME}/bin/crictl"
  local -r container_runtime_name="${CONTAINER_RUNTIME_NAME:-docker}"
  local healthcheck_command="docker ps"
  if [[ "${CONTAINER_RUNTIME:-docker}" != "docker" ]]; then
    healthcheck_command="${crictl} pods"
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
      if [[ "$container_runtime_name" == "docker" ]]; then
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

if [[ "$#" -ne 1 ]]; then
  echo "Usage: health-monitor.sh <container-runtime/kubelet>"
  exit 1
fi

KUBE_HOME="/usr/local/bin"
KUBE_ENV="/etc/default/kube-env"
if [[  -e "${KUBE_ENV}" ]]; then
  source "${KUBE_ENV}"
fi

SLEEP_SECONDS=10
component=$1
echo "Start kubernetes health monitoring for ${component}"

if [[ "${component}" == "container-runtime" ]]; then
  container_runtime_monitoring
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
{{if EnableEncryptionWithExternalKms}}
Requires=kms.service
{{end}}

[Service]
Restart=always
EnvironmentFile=/etc/default/kubelet
SuccessExitStatus=143
ExecStartPre=/bin/bash /opt/azure/containers/kubelet.sh
ExecStartPre=/bin/mkdir -p /var/lib/kubelet
ExecStartPre=/bin/mkdir -p /var/lib/cni
ExecStartPre=/bin/bash -c "if [ $(mount | grep \"/var/lib/kubelet\" | wc -l) -le 0 ] ; then /bin/mount --bind /var/lib/kubelet /var/lib/kubelet ; fi"
ExecStartPre=/bin/mount --make-shared /var/lib/kubelet
{{/* This is a partial workaround to this upstream Kubernetes issue: */}}
{{/* https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731 */}}
ExecStartPre=/sbin/sysctl -w net.ipv4.tcp_retries2=8
ExecStartPre=/sbin/sysctl -w net.core.somaxconn=16384
ExecStartPre=/sbin/sysctl -w net.ipv4.tcp_max_syn_backlog=16384
ExecStartPre=/sbin/sysctl -w net.core.message_cost=40
ExecStartPre=/sbin/sysctl -w net.core.message_burst=80

ExecStartPre=/sbin/sysctl -w net.ipv4.neigh.default.gc_thresh1=4096
ExecStartPre=/sbin/sysctl -w net.ipv4.neigh.default.gc_thresh2=8192
ExecStartPre=/sbin/sysctl -w net.ipv4.neigh.default.gc_thresh3=16384

ExecStartPre=-/sbin/ebtables -t nat --list
ExecStartPre=-/sbin/iptables -t nat --numeric --list
ExecStart=/usr/local/bin/kubelet \
        --enable-server \
        --node-labels="${KUBELET_NODE_LABELS}" \
        --v=2 {{if NeedsContainerd}}--container-runtime=remote --runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock{{end}} \
        --volume-plugin-dir=/etc/kubernetes/volumeplugins \
        {{- if IsDynamicKubeletEnabled}}
        --config /etc/default/kubeletconfig.json \
        {{- end}}
        $KUBELET_FLAGS \
        $KUBELET_REGISTER_NODE $KUBELET_REGISTER_WITH_TAINTS

[Install]
WantedBy=multi-user.target`)

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

var _linuxCloudInitArtifactsLabelNodesService = []byte(`[Unit]
Description=Label Kubernetes nodes as masters or agents
After=kubelet.service
[Service]
Restart=always
RestartSec=60
ExecStart=/bin/bash /opt/azure/containers/label-nodes.sh
#EOF
`)

func linuxCloudInitArtifactsLabelNodesServiceBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsLabelNodesService, nil
}

func linuxCloudInitArtifactsLabelNodesService() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsLabelNodesServiceBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/label-nodes.service", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _linuxCloudInitArtifactsLabelNodesSh = []byte(`#!/usr/bin/env bash

# Applies missing master and agent labels to Kubernetes nodes.
#
# Kubelet 1.16+ rejects the `+"`"+`kubernetes.io/role`+"`"+` and `+"`"+`node-role.kubernetes.io`+"`"+`
# labels in its `+"`"+`--node-labels`+"`"+` argument, but they need to be present for
# backward compatibility.

set -euo pipefail

MASTER_SELECTOR="kubernetes.azure.com/role!=agent,kubernetes.io/role!=agent"
MASTER_LABELS="kubernetes.azure.com/role=master kubernetes.io/role=master node-role.kubernetes.io/master="
AGENT_SELECTOR="kubernetes.azure.com/role!=master,kubernetes.io/role!=master"
AGENT_LABELS="kubernetes.azure.com/role=agent kubernetes.io/role=agent node-role.kubernetes.io/agent="

kubectl label nodes --overwrite -l $MASTER_SELECTOR $MASTER_LABELS
kubectl label nodes --overwrite -l $AGENT_SELECTOR $AGENT_LABELS
#EOF
`)

func linuxCloudInitArtifactsLabelNodesShBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsLabelNodesSh, nil
}

func linuxCloudInitArtifactsLabelNodesSh() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsLabelNodesShBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/label-nodes.sh", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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

var _linuxCloudInitArtifactsSysFsBpfMount = []byte(`[Unit]
Description=Cilium BPF mounts
Documentation=http://docs.cilium.io/
DefaultDependencies=no
Before=local-fs.target umount.target
After=swap.target

[Mount]
What=bpffs
Where=/sys/fs/bpf
Type=bpf

[Install]
WantedBy=multi-user.target`)

func linuxCloudInitArtifactsSysFsBpfMountBytes() ([]byte, error) {
	return _linuxCloudInitArtifactsSysFsBpfMount, nil
}

func linuxCloudInitArtifactsSysFsBpfMount() (*asset, error) {
	bytes, err := linuxCloudInitArtifactsSysFsBpfMountBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "linux/cloud-init/artifacts/sys-fs-bpf.mount", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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

var _linuxCloudInitNodecustomdataYml = []byte(`#cloud-config

write_files:
- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}

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

{{if not .IsVHDDistro}}
  {{if .IsAuditDEnabled}}
- path: /etc/audit/rules.d/CIS.rules
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "auditdRules"}}
  {{end}}
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

- path: /etc/systemd/system/kubelet.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "kubeletSystemdService"}}

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

- path: /etc/systemd/system/kms.service
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "kmsSystemdService"}}

- path: /etc/apt/preferences
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "aptPreferences"}}
{{end}}

- path: /etc/apt/apt.conf.d/99periodic
  permissions: "0644"
  owner: root
  content: |
    APT::Periodic::Update-Package-Lists "0";
    APT::Periodic::Download-Upgradeable-Packages "0";
    APT::Periodic::AutocleanInterval "0";
    APT::Periodic::Unattended-Upgrade "0";

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
      }{{if IsNSeriesSKU .}}
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

{{if HasCiliumNetworkPlugin }}
- path: /etc/systemd/system/sys-fs-bpf.mount
  permissions: "0644"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "systemdBPFMount"}}
{{end}}

{{if NeedsContainerd}}
- path: /etc/containerd/config.toml
  permissions: "0644"
  owner: root
  content: |
    subreaper = false{{if HasDataDir }}
    root = "{{GetDataDir}}"{{- end}}
    oom_score = 0
    [plugins.cri]
    sandbox_image = "{{GetPodInfraContainerSpec}}"
    [plugins.cri.containerd.untrusted_workload_runtime]
    runtime_type = "io.containerd.runtime.v1.linux" {{if IsNSeriesSKU .}}
    runtime_engine = "/usr/bin/nvidia-container-runtime"
    {{- else}}
    runtime_engine = "/usr/bin/runc"
    {{- end}}
    [plugins.cri.containerd.default_runtime]
    runtime_type = "io.containerd.runtime.v1.linux" {{if IsNSeriesSKU .}}
    runtime_engine = "/usr/bin/nvidia-container-runtime"
    {{- else}}
    runtime_engine = "/usr/bin/runc"
    {{- end}}
    {{if IsKubenet }}
    [plugins.cri.cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
    {{end}}
    #EOF

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
            "ipMasq": true,
            "promisMode": true,
            "hairpinMode": false,
            "ipam": {
                "type": "host-local",
                "subnet": "{{`+"`"+`{{.PodCIDR}}`+"`"+`}}",
                "routes": [{ "dst": "0.0.0.0/0" }]
            }
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
    
{{if IsKubenet }}
- path: /etc/sysctl.d/11-containerd.conf
  permissions: "0644"
  owner: root
  content: |
    net.ipv4.ip_forward = 1
    net.bridge.bridge-nf-call-iptables = 1
    #EOF
{{end}}
{{end}}

{{if IsNSeriesSKU .}}
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

- path: /etc/kubernetes/certs/client.crt
  permissions: "0644"
  encoding: base64
  owner: root
  content: |
    {{GetParameter "clientCertificate"}}

{{if HasCustomSearchDomain}}
- path: {{GetCustomSearchDomainsCSEScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "customSearchDomainsScript"}}
{{end}}

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

- path: /etc/default/kubelet
  permissions: "0644"
  owner: root
  content: |
    KUBELET_FLAGS={{GetKubeletConfigKeyVals .KubernetesConfig}}
    KUBELET_REGISTER_SCHEDULABLE=true
{{- if not (IsKubernetesVersionGe "1.17.0")}}
    KUBELET_IMAGE={{GetHyperkubeImageReference}}
{{end}}
{{if IsKubernetesVersionGe "1.16.0"}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabels . "',variables('labelResourceGroup'),'"}}
{{else}}
    KUBELET_NODE_LABELS={{GetAgentKubernetesLabelsDeprecated . "',variables('labelResourceGroup'),'"}}
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
    IFS=','
    read -ra vnetCidrs <<< {{GetParameter "vnetCidr"}}
    for vnetCidr in "${vnetCidrs[@]}"; do
      iptables -t nat -A POSTROUTING -m iprange ! --dst-range 168.63.129.16 -m addrtype ! --dst-type local ! -d $vnetCidr -j MASQUERADE
    done
    IFS=' '
    {{end}}
{{end}}

    # Disallow container from reaching out to the special IP address 168.63.129.16
    # for TCP protocol (which http uses)
    #
    # 168.63.129.16 contains protected settings that have priviledged info.
    #
    # The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
    #
    # Note: we shouldn't block all traffic to 168.63.129.16. For example UDP traffic is still needed
    # for DNS.
    iptables -I FORWARD -d 168.63.129.16 -p tcp -j DROP
    #EOF

runcmd:
- set -x
- . {{GetCSEHelpersScriptFilepath}}
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

var _windowsCsecmdPs1 = []byte(`"[concat('echo %DATE%,%TIME%,%COMPUTERNAME% && powershell.exe -ExecutionPolicy Unrestricted -command \"', '$arguments = ', variables('singleQuote'),'-MasterIP ',parameters('kubernetesEndpoint'),' -KubeDnsServiceIp ',parameters('kubeDnsServiceIp'),' -MasterFQDNPrefix ',variables('masterFqdnPrefix'),' -Location ',variables('location'),' -TargetEnvironment ',parameters('targetEnvironment'),' -AgentKey ',parameters('clientPrivateKey'),' -AADClientId ',variables('servicePrincipalClientId'),' -AADClientSecret ',variables('singleQuote'),variables('singleQuote'),base64(variables('servicePrincipalClientSecret')),variables('singleQuote'),variables('singleQuote'),' -NetworkAPIVersion ',variables('apiVersionNetwork'),' ',variables('singleQuote'), ' ; ', variables('windowsCustomScriptSuffix'), '\" > %SYSTEMDRIVE%\\AzureData\\CustomDataSetupScript.log 2>&1 ; exit $LASTEXITCODE')]"`)

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

var _windowsKuberneteswindowsfunctionsPs1 = []byte(`# This is a temporary file to test dot-sourcing functions stored in separate scripts in a zip file

filter Timestamp {"$(Get-Date -Format o): $_"}

function
Write-Log($message)
{
    $msg = $message | Timestamp
    Write-Output $msg
}

function DownloadFileOverHttp
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $Url,
        [Parameter(Mandatory=$true)][string]
        $DestinationPath
    )

    # First check to see if a file with the same name is already cached on the VHD
    $fileName = [IO.Path]::GetFileName($Url)
    
    $search = @()
    if (Test-Path $global:CacheDir)
    {
        $search = [IO.Directory]::GetFiles($global:CacheDir, $fileName, [IO.SearchOption]::AllDirectories)
    }

    if ($search.Count -ne 0)
    {
        Write-Log "Using cached version of $fileName - Copying file from $($search[0]) to $DestinationPath"
        Move-Item -Path $search[0] -Destination $DestinationPath -Force
    }
    else 
    {
        $secureProtocols = @()
        $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)
    
        foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType]))
        {
            if ($insecureProtocols -notcontains $protocol)
            {
                $secureProtocols += $protocol
            }
        }
        [System.Net.ServicePointManager]::SecurityProtocol = $secureProtocols
    
        $oldProgressPreference = $ProgressPreference
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest $Url -UseBasicParsing -OutFile $DestinationPath -Verbose
        $ProgressPreference = $oldProgressPreference
        Write-Log "Downloaded file to $DestinationPath"
    }
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
        if (-Not (Test-Path $_)) {
            New-Item -ItemType Directory -Path $_
        }
    }
}

function Retry-Command
{
    Param(
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string]
        $Command,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][hashtable]
        $Args,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][int]
        $Retries,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][int]
        $RetryDelaySeconds
    )

    for ($i = 0; $i -lt $Retries; $i++) {
        try {
            return & $Command @Args
        } catch {
            Start-Sleep $RetryDelaySeconds
        }
    }
}

function Invoke-Executable
{
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
        } else {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    throw "Exhausted retries for $Executable $ArgList"
}

function Get-NetworkLogCollectionScripts {
    Write-Log "Getting CollectLogs.ps1 and depencencies"
    mkdir 'c:\k\debug'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1' -DestinationPath 'c:\k\debug\collectlogs.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1' -DestinationPath 'c:\k\debug\dumpVfpPolicies.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd' -DestinationPath 'c:\k\debug\starthnstrace.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd' -DestinationPath 'c:\k\debug\startpacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd' -DestinationPath 'c:\k\debug\stoppacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1' -DestinationPath 'c:\k\debug\helper.psm1'
}
`)

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
    $TargetEnvironment
)



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

## Docker Version
$global:DockerVersion = "{{GetParameter "windowsDockerVersion"}}"

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
$global:MasterSubnet = "{{GetWindowsMasterSubnetARMParam}}"
$global:SecurityGroupName = "{{GetVariable "nsgName"}}"
$global:VNetName = "{{GetVariable "virtualNetworkName"}}"
$global:RouteTableName = "{{GetVariable "routeTableName"}}"
$global:PrimaryAvailabilitySetName = "{{GetVariable "primaryAvailabilitySetName"}}"
$global:PrimaryScaleSetName = "{{GetVariable "primaryScaleSetName"}}"

$global:KubeClusterCIDR = "{{GetParameter "kubeClusterCidr"}}"
$global:KubeServiceCIDR = "{{GetParameter "kubeServiceCidr"}}"
$global:VNetCIDR = "{{GetParameter "vnetCidr"}}"
{{if IsKubernetesVersionGe "1.16.0"}}
$global:KubeletNodeLabels = "{{GetAgentKubernetesLabels . "',variables('labelResourceGroup'),'"}}"
{{else}}
$global:KubeletNodeLabels = "{{GetAgentKubernetesLabelsDeprecated . "',variables('labelResourceGroup'),'"}}"
{{end}}
$global:KubeletConfigArgs = @( {{GetKubeletConfigKeyValsPsh .KubernetesConfig }} )

$global:UseManagedIdentityExtension = "{{GetVariable "useManagedIdentityExtension"}}"
$global:UserAssignedClientID = "{{GetVariable "userAssignedClientID"}}"
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

# Base64 representation of ZIP archive
$zippedFiles = "{{ GetKubernetesWindowsAgentFunctions }}"

# Extract ZIP from script
[io.file]::WriteAllBytes("scripts.zip", [System.Convert]::FromBase64String($zippedFiles))
Expand-Archive scripts.zip -DestinationPath "C:\\AzureData\\"

# Dot-source contents of zip. This should match the list in template_generator.go GetKubernetesWindowsAgentFunctions
. c:\AzureData\k8s\kuberneteswindowsfunctions.ps1
. c:\AzureData\k8s\windowsconfigfunc.ps1
. c:\AzureData\k8s\windowskubeletfunc.ps1
. c:\AzureData\k8s\windowscnifunc.ps1
. c:\AzureData\k8s\windowsazurecnifunc.ps1
. c:\AzureData\k8s\windowsinstallopensshfunc.ps1

function
Update-ServiceFailureActions()
{
    sc.exe failure "kubelet" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "kubeproxy" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "docker" actions= restart/60000/restart/60000/restart/60000 reset= 900
}

try
{
    # Set to false for debugging.  This will output the start script to
    # c:\AzureData\CustomDataSetupScript.log, and then you can RDP
    # to the windows machine, and run the script manually to watch
    # the output.
    if ($true) {
        Write-Log "Provisioning $global:DockerServiceName... with IP $MasterIP"

        # Install OpenSSH if SSH enabled
        $sshEnabled = [System.Convert]::ToBoolean("{{ WindowsSSHEnabled }}")

        if ( $sshEnabled ) {
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

        Write-Log "Install docker"
        Install-Docker -DockerVersion $global:DockerVersion

        Write-Log "Download kubelet binaries and unzip"
        Get-KubePackage -KubeBinariesSASURL $global:KubeBinariesPackageSASURL

        # this overwrite the binaries that are download from the custom packge with binaries
        # The custom package has a few files that are nessary for future steps (nssm.exe)
        # this is a temporary work around to get the binaries until we depreciate
        # custom package and nssm.exe as defined in #3851.
        if ($global:WindowsKubeBinariesURL){
            Write-Log "Overwriting kube node binaries from $global:WindowsKubeBinariesURL"
            Get-KubeBinaries -KubeBinariesURL $global:WindowsKubeBinariesURL
        }

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
            -UserAssignedClientID $global:UserAssignedClientID `+"`"+`
            -UseInstanceMetadata $global:UseInstanceMetadata `+"`"+`
            -LoadBalancerSku $global:LoadBalancerSku `+"`"+`
            -ExcludeMasterFromStandardLB $global:ExcludeMasterFromStandardLB `+"`"+`
            -TargetEnvironment $TargetEnvironment

        Write-Log "Write ca root"
        Write-CACert -CACertificate $global:CACertificate `+"`"+`
                     -KubeDir $global:KubeDir

        Write-Log "Write kube config"
        Write-KubeConfig -CACertificate $global:CACertificate `+"`"+`
                         -KubeDir $global:KubeDir `+"`"+`
                         -MasterFQDNPrefix $MasterFQDNPrefix `+"`"+`
                         -MasterIP $MasterIP `+"`"+`
                         -AgentKey $AgentKey `+"`"+`
                         -AgentCertificate $global:AgentCertificate

        Write-Log "Create the Pause Container kubletwin/pause"
        New-InfraContainer -KubeDir $global:KubeDir

        if (-not (Test-ContainerImageExists -Image "kubletwin/pause")) {
            Write-Log "Could not find container with name kubletwin/pause"
            $o = docker image list
            Write-Log $o
            throw "kubletwin/pause container does not exist!"
        }

        Write-Log "Configuring networking with NetworkPlugin:$global:NetworkPlugin"

        # Configure network policy.
        if ($global:NetworkPlugin -eq "azure") {
            Write-Log "Installing Azure VNet plugins"
            Install-VnetPlugins -AzureCNIConfDir $global:AzureCNIConfDir `+"`"+`
                                -AzureCNIBinDir $global:AzureCNIBinDir `+"`"+`
                                -VNetCNIPluginsURL $global:VNetCNIPluginsURL
            Set-AzureCNIConfig -AzureCNIConfDir $global:AzureCNIConfDir `+"`"+`
                               -KubeDnsSearchPath $global:KubeDnsSearchPath `+"`"+`
                               -KubeClusterCIDR $global:KubeClusterCIDR `+"`"+`
                               -MasterSubnet $global:MasterSubnet `+"`"+`
                               -KubeServiceCIDR $global:KubeServiceCIDR `+"`"+`
                               -VNetCIDR $global:VNetCIDR `+"`"+`
                               -TargetEnvironment $TargetEnvironment

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

        } elseif ($global:NetworkPlugin -eq "kubenet") {
            Write-Log "Fetching additional files needed for kubenet"
            Update-WinCNI -CNIPath $global:CNIPath
            Get-HnsPsm1 -HNSModule $global:HNSModule
        }

        Write-Log "Write kubelet startfile with pod CIDR of $podCIDR"
        Install-KubernetesServices `+"`"+`
            -KubeletConfigArgs $global:KubeletConfigArgs `+"`"+`
            -KubeBinariesVersion $global:KubeBinariesVersion `+"`"+`
            -NetworkPlugin $global:NetworkPlugin `+"`"+`
            -NetworkMode $global:NetworkMode `+"`"+`
            -KubeDir $global:KubeDir `+"`"+`
            -AzureCNIBinDir $global:AzureCNIBinDir `+"`"+`
            -AzureCNIConfDir $global:AzureCNIConfDir `+"`"+`
            -CNIPath $global:CNIPath `+"`"+`
            -CNIConfig $global:CNIConfig `+"`"+`
            -CNIConfigPath $global:CNIConfigPath `+"`"+`
            -MasterIP $MasterIP `+"`"+`
            -KubeDnsServiceIp $KubeDnsServiceIp `+"`"+`
            -MasterSubnet $global:MasterSubnet `+"`"+`
            -KubeClusterCIDR $global:KubeClusterCIDR `+"`"+`
            -KubeServiceCIDR $global:KubeServiceCIDR `+"`"+`
            -HNSModule $global:HNSModule `+"`"+`
            -KubeletNodeLabels $global:KubeletNodeLabels

        Get-NetworkLogCollectionScripts

        Write-Log "Disable Internet Explorer compat mode and set homepage"
        Set-Explorer

        Write-Log "Adjust pagefile size"
        Adjust-PageFileSize

        Write-Log "Start preProvisioning script"
        PREPROVISION_EXTENSION

        Write-Log "Update service failure actions"
        Update-ServiceFailureActions

        if (Test-Path $CacheDir)
        {
            Write-Log "Removing aks-engine bits cache directory"
            Remove-Item $CacheDir -Recurse -Force
        }

        Write-Log "Setup Complete, reboot computer"
        Restart-Computer
    }
    else
    {
        # keep for debugging purposes
        Write-Log ".\CustomDataSetupScript.ps1 -MasterIP $MasterIP -KubeDnsServiceIp $KubeDnsServiceIp -MasterFQDNPrefix $MasterFQDNPrefix -Location $Location -AgentKey $AgentKey -AADClientId $AADClientId -AADClientSecret $AADClientSecret"
    }
}
catch
{
    Write-Error $_
    exit 1
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

var _windowsWindowsazurecnifuncPs1 = []byte(`

# TODO: remove - dead code?
function
Set-VnetPluginMode()
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory=$true)][string]
        $Mode
    )
    # Sets Azure VNET CNI plugin operational mode.
    $fileName  = [Io.path]::Combine("$AzureCNIConfDir", "10-azure.conflist")
    (Get-Content $fileName) | %{$_ -replace "`+"`"+`"mode`+"`"+`":.*", "`+"`"+`"mode`+"`"+`": `+"`"+`"$Mode`+"`"+`","} | Out-File -encoding ASCII -filepath $fileName
}


function
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
    mkdir $AzureCNIBinDir
    mkdir $AzureCNIConfDir

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

# TODO: remove - dead code?
function
Set-AzureNetworkPlugin()
{
    # Azure VNET network policy requires tunnel (hairpin) mode because policy is enforced in the host.
    Set-VnetPluginMode "tunnel"
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
        $MasterSubnet,
        [Parameter(Mandatory=$true)][string]
        $KubeServiceCIDR,
        [Parameter(Mandatory=$true)][string]
        $VNetCIDR,
        [Parameter(Mandatory=$true)][string]
        $TargetEnvironment
    )
    # Fill in DNS information for kubernetes.
    $exceptionAddresses = @($KubeClusterCIDR, $MasterSubnet, $VNetCIDR)

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
        Write-Host "Filtering CNI config exception list values to work around WS2019 issue processing rules. Original exception list: $exceptionAddresses, processed exception list: $processedExceptions"
        $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $processedExceptions
    }
    else {
        $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $exceptionAddresses
    }

    $configJson.plugins.AdditionalArgs[1].Value.DestinationPrefix  = $KubeServiceCIDR

    if ($TargetEnvironment -ieq "AzureStackCloud") {
        Add-Member -InputObject $configJson.plugins[0].ipam -MemberType NoteProperty -Name "environment" -Value "mas"
    }

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

    $response = Retry-Command -Command "Invoke-RestMethod" -Args @{Uri=$uri; Method="Get"; ContentType="application/json"; Headers=$headers} -Retries 5 -RetryDelaySeconds 10

    if(!$response) {
        throw 'Error getting subnet prefix'
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
    $tokenResponse = Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10

    if(!$tokenResponse) {
        throw 'Error generating token for Azure Resource Manager'
    }

    $token = $tokenResponse | Select-Object -ExpandProperty access_token

    Write-Log "Fetching network interface configuration for node"

    $interfacesUri = "$($azureEnvironment.resourceManagerEndpoint)subscriptions/$SubscriptionId/resourceGroups/$ResourceGroup/providers/Microsoft.Network/networkInterfaces?api-version=$NetworkAPIVersion"
    $headers = @{Authorization="Bearer $token"}
    $args = @{Uri=$interfacesUri; Method="Get"; ContentType="application/json"; Headers=$headers; OutFile=$networkInterfacesFile}
    Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10

    if(!$(Test-Path $networkInterfacesFile)) {
        throw 'Error fetching network interface configuration for node'
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

function Update-WinCNI
{
    Param(
        [string]
        $WinCniUrl = "https://github.com/Microsoft/SDN/raw/master/Kubernetes/flannel/l2bridge/cni/win-bridge.exe",
        [Parameter(Mandatory=$true)][string]
        $CNIPath
    )
    $wincni = "win-bridge.exe"
    $wincniFile = [Io.path]::Combine($CNIPath, $wincni)
    DownloadFileOverHttp -Url $WinCniUrl -DestinationPath $wincniFile
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

    # DOCKER_API_VERSION needs to be set for Docker versions older than 18.09.0 EE
    # due to https://github.com/kubernetes/kubernetes/issues/69996
    # this issue was fixed by https://github.com/kubernetes/kubernetes/issues/69996#issuecomment-438499024
    # Note: to get a list of all versions, use this snippet
    # $versions = (curl.exe -L "https://go.microsoft.com/fwlink/?LinkID=825636&clcid=0x409" | ConvertFrom-Json).Versions | Get-Member -Type NoteProperty | Select-Object Name
    # Docker version to API version decoder: https://docs.docker.com/develop/sdk/#api-version-matrix

    switch ($DockerVersion.Substring(0,5))
    {
        "17.06" {
            Write-Log "Docker 17.06 found, setting DOCKER_API_VERSION to 1.30"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', '1.30', [System.EnvironmentVariableTarget]::Machine)
        }

        "18.03" {
            Write-Log "Docker 18.03 found, setting DOCKER_API_VERSION to 1.37"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', '1.37', [System.EnvironmentVariableTarget]::Machine)
        }

        default {
            Write-Log "Docker version $DockerVersion found, clearing DOCKER_API_VERSION"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', $null, [System.EnvironmentVariableTarget]::Machine)
        }
    }

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

# Pagefile adjustments
function Adjust-PageFileSize()
{
    wmic pagefileset set InitialSize=8096,MaximumSize=8096
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
        Write-Host "Installing OpenSSH"
        $isAvailable = Get-WindowsCapability -Online | ? Name -like 'OpenSSH*'

        if (!$isAvailable) {
            throw "OpenSSH is not available on this machine"
        }

        Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    }
    else
    {
        Write-Host "OpenSSH Server service detected - skipping online install..."
    }

    Start-Service sshd

    if (!(Test-Path "$adminpath")) {
        Write-Host "Created new file and text content added"
        New-Item -path $adminpath -name $adminfile -type "file" -value ""
    }

    Write-Host "$adminpath found."
    Write-Host "Adding keys to: $adminpath\$adminfile ..."
    $SSHKeys | foreach-object {
        Add-Content $adminpath\$adminfile $_
    }

    Write-Host "Setting required permissions..."
    icacls $adminpath\$adminfile /remove "NT AUTHORITY\Authenticated Users"
    icacls $adminpath\$adminfile /inheritance:r
    icacls $adminpath\$adminfile /grant SYSTEM:`+"`"+`(F`+"`"+`)
    icacls $adminpath\$adminfile /grant BUILTIN\Administrators:`+"`"+`(F`+"`"+`)

    Write-Host "Restarting sshd service..."
    Restart-Service sshd
    # OPTIONAL but recommended:
    Set-Service -Name sshd -StartupType 'Automatic'

    # Confirm the Firewall rule is configured. It should be created automatically by setup. 
    $firewall = Get-NetFirewallRule -Name *ssh*

    if (!$firewall) {
        throw "OpenSSH is firewall is not configured properly"
    }
    Write-Host "OpenSSH installed and configured successfully"
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
        $TargetEnvironment
    )

    if ( -Not $PrimaryAvailabilitySetName -And -Not $PrimaryScaleSetName ) {
        throw "Either PrimaryAvailabilitySetName or PrimaryScaleSetName must be set"
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
Build-PauseContainer {
    Param(
        [Parameter(Mandatory = $true)][string]
        $WindowsBase,
        $DestinationTag
    )
    # Future work: This needs to build wincat - see https://github.com/Azure/aks-engine/issues/1461
    "FROM $($WindowsBase)" | Out-File -encoding ascii -FilePath Dockerfile
    "CMD cmd /c ping -t localhost" | Out-File -encoding ascii -FilePath Dockerfile -Append
    Invoke-Executable -Executable "docker" -ArgList @("build", "-t", "$DestinationTag", ".")
}

function
New-InfraContainer {
    Param(
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        $DestinationTag = "kubletwin/pause"
    )
    cd $KubeDir
    $computerInfo = Get-ComputerInfo

    # Reference for these tags: curl -L https://mcr.microsoft.com/v2/k8s/core/pause/tags/list
    # Then docker run --rm mplatform/manifest-tool inspect mcr.microsoft.com/k8s/core/pause:<tag>

    $defaultPauseImage = "mcr.microsoft.com/k8s/core/pause:1.2.0"

    switch ($computerInfo.WindowsVersion) {
        "1803" { 
            $imageList = docker images $defaultPauseImage --format "{{.Repository}}:{{.Tag}}"
            if (-not $imageList) {
                Invoke-Executable -Executable "docker" -ArgList @("pull", "$defaultPauseImage") -Retries 5 -RetryDelaySeconds 30
            }
            Invoke-Executable -Executable "docker" -ArgList @("tag", "$defaultPauseImage", "$DestinationTag")
        }
        "1809" { 
            $imageList = docker images $defaultPauseImage --format "{{.Repository}}:{{.Tag}}"
            if (-not $imageList) {
                Invoke-Executable -Executable "docker" -ArgList @("pull", "$defaultPauseImage") -Retries 5 -RetryDelaySeconds 30
            }
            Invoke-Executable -Executable "docker" -ArgList @("tag", "$defaultPauseImage", "$DestinationTag")
        }
        "1903" { Build-PauseContainer -WindowsBase "mcr.microsoft.com/windows/nanoserver:1903" -DestinationTag $DestinationTag}
        default { Build-PauseContainer -WindowsBase "mcr.microsoft.com/nanoserver-insider" -DestinationTag $DestinationTag}
    }
}

function
Test-ContainerImageExists {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Image,
        [Parameter(Mandatory = $false)][string]
        $Tag
    )

    $target = $Image
    if ($Tag) {
        $target += ":$Tag"
    }

    $images = docker image list $target --format "{{json .}}"

    return $images.Count -gt 0
}


# TODO: Deprecate this and replace with methods that get individual components instead of zip containing everything
# This expects the ZIP file to be created by scripts/build-windows-k8s.sh
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
    if (!(Test-path $windowsbinariespath)) {
        mkdir $windowsbinariespath
    }
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
        $KubeProxyStartFile
    )

    # setup kubelet
    & "$KubeDir\nssm.exe" install Kubelet C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe
    & "$KubeDir\nssm.exe" set Kubelet AppDirectory $KubeDir
    & "$KubeDir\nssm.exe" set Kubelet AppParameters $KubeletStartFile
    & "$KubeDir\nssm.exe" set Kubelet DisplayName Kubelet
    & "$KubeDir\nssm.exe" set Kubelet AppRestartDelay 5000
    & "$KubeDir\nssm.exe" set Kubelet DependOnService docker
    & "$KubeDir\nssm.exe" set Kubelet Description Kubelet
    & "$KubeDir\nssm.exe" set Kubelet Start SERVICE_AUTO_START
    & "$KubeDir\nssm.exe" set Kubelet ObjectName LocalSystem
    & "$KubeDir\nssm.exe" set Kubelet Type SERVICE_WIN32_OWN_PROCESS
    & "$KubeDir\nssm.exe" set Kubelet AppThrottle 1500
    & "$KubeDir\nssm.exe" set Kubelet AppStdout C:\k\kubelet.log
    & "$KubeDir\nssm.exe" set Kubelet AppStderr C:\k\kubelet.err.log
    & "$KubeDir\nssm.exe" set Kubelet AppStdoutCreationDisposition 4
    & "$KubeDir\nssm.exe" set Kubelet AppStderrCreationDisposition 4
    & "$KubeDir\nssm.exe" set Kubelet AppRotateFiles 1
    & "$KubeDir\nssm.exe" set Kubelet AppRotateOnline 1
    & "$KubeDir\nssm.exe" set Kubelet AppRotateSeconds 86400
    & "$KubeDir\nssm.exe" set Kubelet AppRotateBytes 1048576

    # setup kubeproxy
    & "$KubeDir\nssm.exe" install Kubeproxy C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe
    & "$KubeDir\nssm.exe" set Kubeproxy AppDirectory $KubeDir
    & "$KubeDir\nssm.exe" set Kubeproxy AppParameters $KubeProxyStartFile
    & "$KubeDir\nssm.exe" set Kubeproxy DisplayName Kubeproxy
    & "$KubeDir\nssm.exe" set Kubeproxy DependOnService Kubelet
    & "$KubeDir\nssm.exe" set Kubeproxy Description Kubeproxy
    & "$KubeDir\nssm.exe" set Kubeproxy Start SERVICE_AUTO_START
    & "$KubeDir\nssm.exe" set Kubeproxy ObjectName LocalSystem
    & "$KubeDir\nssm.exe" set Kubeproxy Type SERVICE_WIN32_OWN_PROCESS
    & "$KubeDir\nssm.exe" set Kubeproxy AppThrottle 1500
    & "$KubeDir\nssm.exe" set Kubeproxy AppStdout C:\k\kubeproxy.log
    & "$KubeDir\nssm.exe" set Kubeproxy AppStderr C:\k\kubeproxy.err.log
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateFiles 1
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateOnline 1
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateSeconds 86400
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateBytes 1048576
}

# Renamed from Write-KubernetesStartFiles
function
Install-KubernetesServices {
    param(
        [Parameter(Mandatory = $true)][string[]]
        $KubeletConfigArgs,
        [Parameter(Mandatory = $true)][string]
        $KubeBinariesVersion,
        [Parameter(Mandatory = $true)][string]
        $NetworkPlugin,
        [Parameter(Mandatory = $true)][string]
        $NetworkMode,
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        [Parameter(Mandatory = $true)][string]
        $AzureCNIBinDir,
        [Parameter(Mandatory = $true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory = $true)][string]
        $CNIPath,
        [Parameter(Mandatory = $true)][string]
        $CNIConfig,
        [Parameter(Mandatory = $true)][string]
        $CNIConfigPath,
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $KubeDnsServiceIp,
        [Parameter(Mandatory = $true)][string]
        $MasterSubnet,
        [Parameter(Mandatory = $true)][string]
        $KubeClusterCIDR,
        [Parameter(Mandatory = $true)][string]
        $KubeServiceCIDR,
        [Parameter(Mandatory = $true)][string]
        $HNSModule,
        [Parameter(Mandatory = $true)][string]
        $KubeletNodeLabels
    )

    # Calculate some local paths
    $VolumePluginDir = [Io.path]::Combine($KubeDir, "volumeplugins")
    $KubeletStartFile = [io.path]::Combine($KubeDir, "kubeletstart.ps1")
    $KubeProxyStartFile = [io.path]::Combine($KubeDir, "kubeproxystart.ps1")

    mkdir $VolumePluginDir
    $KubeletArgList = $KubeletConfigArgs # This is the initial list passed in from aks-engine
    $KubeletArgList += "--node-labels=`+"`"+`$global:KubeletNodeLabels"
    # $KubeletArgList += "--hostname-override=`+"`"+`$global:AzureHostname" TODO: remove - dead code?
    $KubeletArgList += "--volume-plugin-dir=`+"`"+`$global:VolumePluginDir"
    # If you are thinking about adding another arg here, you should be considering pkg/engine/defaults-kubelet.go first
    # Only args that need to be calculated or combined with other ones on the Windows agent should be added here.


    # Regex to strip version to Major.Minor.Build format such that the following check does not crash for version like x.y.z-alpha
    [regex]$regex = "^[0-9.]+"
    $KubeBinariesVersionStripped = $regex.Matches($KubeBinariesVersion).Value
    if ([System.Version]$KubeBinariesVersionStripped -lt [System.Version]"1.8.0") {
        # --api-server deprecates from 1.8.0
        $KubeletArgList += "--api-servers=https://`+"`"+`${global:MasterIP}:443"
    }

    # Configure kubelet to use CNI plugins if enabled.
    if ($NetworkPlugin -eq "azure") {
        $KubeletArgList += @("--cni-bin-dir=$AzureCNIBinDir", "--cni-conf-dir=$AzureCNIConfDir")
    }
    elseif ($NetworkPlugin -eq "kubenet") {
        $KubeletArgList += @("--cni-bin-dir=$CNIPath", "--cni-conf-dir=$CNIConfigPath")
        # handle difference in naming between Linux & Windows reference plugin
        $KubeletArgList = $KubeletArgList -replace "kubenet", "cni"
    }
    else {
        throw "Unknown network type $NetworkPlugin, can't configure kubelet"
    }

    # Used in WinCNI version of kubeletstart.ps1
    $KubeletArgListStr = ""
    $KubeletArgList | Foreach-Object {
        # Since generating new code to be written to a file, need to escape quotes again
        if ($KubeletArgListStr.length -gt 0) {
            $KubeletArgListStr = $KubeletArgListStr + ", "
        }
        $KubeletArgListStr = $KubeletArgListStr + "`+"`"+`"" + $_.Replace("`+"`"+`"`+"`"+`"", "`+"`"+`"`+"`"+`"`+"`"+`"`+"`"+`"") + "`+"`"+`""
    }
    $KubeletArgListStr = "@`+"`"+`($KubeletArgListStr`+"`"+`)"

    # Used in Azure-CNI version of kubeletstart.ps1
    $KubeletCommandLine = "$KubeDir\kubelet.exe " + ($KubeletArgList -join " ")

    $kubeStartStr = @"
`+"`"+`$global:MasterIP = "$MasterIP"
`+"`"+`$global:KubeDnsSearchPath = "svc.cluster.local"
`+"`"+`$global:KubeDnsServiceIp = "$KubeDnsServiceIp"
`+"`"+`$global:MasterSubnet = "$MasterSubnet"
`+"`"+`$global:KubeClusterCIDR = "$KubeClusterCIDR"
`+"`"+`$global:KubeServiceCIDR = "$KubeServiceCIDR"
`+"`"+`$global:KubeBinariesVersion = "$KubeBinariesVersion"
`+"`"+`$global:CNIPath = "$CNIPath"
`+"`"+`$global:NetworkMode = "$NetworkMode"
`+"`"+`$global:ExternalNetwork = "ext"
`+"`"+`$global:CNIConfig = "$CNIConfig"
`+"`"+`$global:HNSModule = "$HNSModule"
`+"`"+`$global:VolumePluginDir = "$VolumePluginDir"
`+"`"+`$global:NetworkPlugin="$NetworkPlugin"
`+"`"+`$global:KubeletNodeLabels="$KubeletNodeLabels"

"@

    if ($NetworkPlugin -eq "azure") {
        $KubeNetwork = "azure"
        $kubeStartStr += @"
Write-Host "NetworkPlugin azure, starting kubelet."

# Turn off Firewall to enable pods to talk to service endpoints. (Kubelet should eventually do this)
netsh advfirewall set allprofiles state off
# startup the service

# Find if the primary external switch network exists. If not create one.
# This is done only once in the lifetime of the node
`+"`"+`$hnsNetwork = Get-HnsNetwork | ? Name -EQ `+"`"+`$global:ExternalNetwork
if (!`+"`"+`$hnsNetwork)
{
    Write-Host "Creating a new hns Network"
    ipmo `+"`"+`$global:HNSModule

    `+"`"+`$na = @(Get-NetAdapter -Physical)
    if (`+"`"+`$na.Count -eq 0)
    {
        throw "Failed to find any physical network adapters"
    }

    # If there is more than one adapter, use the first adapter.
    `+"`"+`$managementIP = (Get-NetIPAddress -ifIndex `+"`"+`$na[0].ifIndex -AddressFamily IPv4).IPAddress
    `+"`"+`$adapterName = `+"`"+`$na[0].Name
    write-host "Using adapter `+"`"+`$adapterName with IP address `+"`"+`$managementIP"
    `+"`"+`$mgmtIPAfterNetworkCreate

    `+"`"+`$stopWatch = New-Object System.Diagnostics.Stopwatch
    `+"`"+`$stopWatch.Start()
    # Fixme : use a smallest range possible, that will not collide with any pod space
    New-HNSNetwork -Type `+"`"+`$global:NetworkMode -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -AdapterName `+"`"+`$adapterName -Name `+"`"+`$global:ExternalNetwork -Verbose

    # Wait for the switch to be created and the ip address to be assigned.
    for (`+"`"+`$i=0;`+"`"+`$i -lt 180;`+"`"+`$i++)
    {
        `+"`"+`$mgmtIPAfterNetworkCreate = Get-NetIPAddress `+"`"+`$managementIP -ErrorAction SilentlyContinue
        if (`+"`"+`$mgmtIPAfterNetworkCreate)
        {
            break
        }
        sleep -Milliseconds 1000
    }

    `+"`"+`$stopWatch.Stop()
    if (-not `+"`"+`$mgmtIPAfterNetworkCreate)
    {
        throw "Failed to find `+"`"+`$managementIP after creating `+"`"+`$global:ExternalNetwork network"
    }
    write-host "It took `+"`"+`$(`+"`"+`$StopWatch.Elapsed.Seconds) seconds to create the `+"`"+`$global:ExternalNetwork network."
}

# Find if network created by CNI exists, if yes, remove it
# This is required to keep the network non-persistent behavior
# Going forward, this would be done by HNS automatically during restart of the node

`+"`"+`$hnsNetwork = Get-HnsNetwork | ? Name -EQ $KubeNetwork
if (`+"`"+`$hnsNetwork)
{
    # Cleanup all containers
    docker ps -q | foreach {docker rm `+"`"+`$_ -f}

    Write-Host "Cleaning up old HNS network found"
    Remove-HnsNetwork `+"`"+`$hnsNetwork
    # Kill all cni instances & stale data left by cni
    # Cleanup all files related to cni
    taskkill /IM azure-vnet.exe /f
    taskkill /IM azure-vnet-ipam.exe /f
    `+"`"+`$cnijson = [io.path]::Combine("$KubeDir", "azure-vnet-ipam.json")
    if ((Test-Path `+"`"+`$cnijson))
    {
        Remove-Item `+"`"+`$cnijson
    }
    `+"`"+`$cnilock = [io.path]::Combine("$KubeDir", "azure-vnet-ipam.json.lock")
    if ((Test-Path `+"`"+`$cnilock))
    {
        Remove-Item `+"`"+`$cnilock
    }

    `+"`"+`$cnijson = [io.path]::Combine("$KubeDir", "azure-vnet.json")
    if ((Test-Path `+"`"+`$cnijson))
    {
        Remove-Item `+"`"+`$cnijson
    }
    `+"`"+`$cnilock = [io.path]::Combine("$KubeDir", "azure-vnet.json.lock")
    if ((Test-Path `+"`"+`$cnilock))
    {
        Remove-Item `+"`"+`$cnilock
    }
}

# Restart Kubeproxy, which would wait, until the network is created
# This was fixed in 1.15, workaround still needed for 1.14 https://github.com/kubernetes/kubernetes/pull/78612
Restart-Service Kubeproxy

`+"`"+`$env:AZURE_ENVIRONMENT_FILEPATH="c:\k\azurestackcloud.json"

$KubeletCommandLine

"@
    }
    else {
        # using WinCNI. TODO: If WinCNI support is removed, then delete this as dead code later
        $KubeNetwork = "l2bridge"
        $kubeStartStr += @"

function
Get-DefaultGateway(`+"`"+`$CIDR)
{
    return `+"`"+`$CIDR.substring(0,`+"`"+`$CIDR.lastIndexOf(".")) + ".1"
}

function
Get-PodCIDR()
{
    `+"`"+`$podCIDR = c:\k\kubectl.exe --kubeconfig=c:\k\config get nodes/`+"`"+`$(`+"`"+`$env:computername.ToLower()) -o custom-columns=podCidr:.spec.podCIDR --no-headers
    return `+"`"+`$podCIDR
}

function
Test-PodCIDR(`+"`"+`$podCIDR)
{
    return `+"`"+`$podCIDR.length -gt 0
}

function
Update-CNIConfig(`+"`"+`$podCIDR, `+"`"+`$masterSubnetGW)
{
    `+"`"+`$jsonSampleConfig =
"{
    ""cniVersion"": ""0.2.0"",
    ""name"": ""<NetworkMode>"",
    ""type"": ""win-bridge"",
    ""master"": ""Ethernet"",
    ""dns"" : {
        ""Nameservers"" : [ ""<NameServers>"" ],
        ""Search"" : [ ""<Cluster DNS Suffix or Search Path>"" ]
    },
    ""policies"": [
    {
        ""Name"" : ""EndpointPolicy"", ""Value"" : { ""Type"" : ""OutBoundNAT"", ""ExceptionList"": [ ""<ClusterCIDR>"", ""<MgmtSubnet>"" ] }
    },
    {
        ""Name"" : ""EndpointPolicy"", ""Value"" : { ""Type"" : ""ROUTE"", ""DestinationPrefix"": ""<ServiceCIDR>"", ""NeedEncap"" : true }
    }
    ]
}"

    `+"`"+`$configJson = ConvertFrom-Json `+"`"+`$jsonSampleConfig
    `+"`"+`$configJson.name = `+"`"+`$global:NetworkMode.ToLower()
    `+"`"+`$configJson.dns.Nameservers[0] = `+"`"+`$global:KubeDnsServiceIp
    `+"`"+`$configJson.dns.Search[0] = `+"`"+`$global:KubeDnsSearchPath

    `+"`"+`$configJson.policies[0].Value.ExceptionList[0] = `+"`"+`$global:KubeClusterCIDR
    `+"`"+`$configJson.policies[0].Value.ExceptionList[1] = `+"`"+`$global:MasterSubnet
    `+"`"+`$configJson.policies[1].Value.DestinationPrefix  = `+"`"+`$global:KubeServiceCIDR

    if (Test-Path `+"`"+`$global:CNIConfig)
    {
        Clear-Content -Path `+"`"+`$global:CNIConfig
    }

    Write-Host "Generated CNI Config [`+"`"+`$configJson]"

    Add-Content -Path `+"`"+`$global:CNIConfig -Value (ConvertTo-Json `+"`"+`$configJson -Depth 20)
}

try
{
    `+"`"+`$env:AZURE_ENVIRONMENT_FILEPATH="c:\k\azurestackcloud.json"

    `+"`"+`$masterSubnetGW = Get-DefaultGateway `+"`"+`$global:MasterSubnet
    `+"`"+`$podCIDR=Get-PodCIDR
    `+"`"+`$podCidrDiscovered=Test-PodCIDR(`+"`"+`$podCIDR)

    # if the podCIDR has not yet been assigned to this node, start the kubelet process to get the podCIDR, and then promptly kill it.
    if (-not `+"`"+`$podCidrDiscovered)
    {
        `+"`"+`$argList = $KubeletArgListStr

        `+"`"+`$process = Start-Process -FilePath c:\k\kubelet.exe -PassThru -ArgumentList `+"`"+`$argList

        # run kubelet until podCidr is discovered
        Write-Host "waiting to discover pod CIDR"
        while (-not `+"`"+`$podCidrDiscovered)
        {
            Write-Host "Sleeping for 10s, and then waiting to discover pod CIDR"
            Start-Sleep 10

            `+"`"+`$podCIDR=Get-PodCIDR
            `+"`"+`$podCidrDiscovered=Test-PodCIDR(`+"`"+`$podCIDR)
        }

        # stop the kubelet process now that we have our CIDR, discard the process output
        `+"`"+`$process | Stop-Process | Out-Null
    }

    # Turn off Firewall to enable pods to talk to service endpoints. (Kubelet should eventually do this)
    netsh advfirewall set allprofiles state off

    # startup the service
    `+"`"+`$hnsNetwork = Get-HnsNetwork | ? Name -EQ `+"`"+`$global:NetworkMode.ToLower()

    if (`+"`"+`$hnsNetwork)
    {
        # Kubelet has been restarted with existing network.
        # Cleanup all containers
        docker ps -q | foreach {docker rm `+"`"+`$_ -f}
        # cleanup network
        Write-Host "Cleaning up old HNS network found"
        Remove-HnsNetwork `+"`"+`$hnsNetwork
        Start-Sleep 10
    }

    Write-Host "Creating a new hns Network"
    ipmo `+"`"+`$global:HNSModule

    `+"`"+`$hnsNetwork = New-HNSNetwork -Type `+"`"+`$global:NetworkMode -AddressPrefix `+"`"+`$podCIDR -Gateway `+"`"+`$masterSubnetGW -Name `+"`"+`$global:NetworkMode.ToLower() -Verbose
    # New network has been created, Kubeproxy service has to be restarted
    # This was fixed in 1.15, workaround still needed for 1.14 https://github.com/kubernetes/kubernetes/pull/78612
    Restart-Service Kubeproxy

    Start-Sleep 10
    # Add route to all other POD networks
    Update-CNIConfig `+"`"+`$podCIDR `+"`"+`$masterSubnetGW

    $KubeletCommandLine
}
catch
{
    Write-Error `+"`"+`$_
}

"@
    } # end else using WinCNI.

    # Now that the script is generated, based on what CNI plugin and startup options are needed, write it to disk
    $kubeStartStr | Out-File -encoding ASCII -filepath $KubeletStartFile

    $kubeProxyStartStr = @"
`+"`"+`$env:KUBE_NETWORK = "$KubeNetwork"
`+"`"+`$global:NetworkMode = "$NetworkMode"
`+"`"+`$global:HNSModule = "$HNSModule"
`+"`"+`$hnsNetwork = Get-HnsNetwork | ? Name -EQ $KubeNetwork
while (!`+"`"+`$hnsNetwork)
{
    Write-Host "Waiting for Network [$KubeNetwork] to be created . . ."
    Start-Sleep 10
    `+"`"+`$hnsNetwork = Get-HnsNetwork | ? Name -EQ $KubeNetwork
}

#
# cleanup the persisted policy lists
#
ipmo `+"`"+`$global:HNSModule
# Workaround for https://github.com/kubernetes/kubernetes/pull/68923 in < 1.14,
# and https://github.com/kubernetes/kubernetes/pull/78612 for <= 1.15
Get-HnsPolicyList | Remove-HnsPolicyList

$KubeDir\kube-proxy.exe --v=3 --proxy-mode=kernelspace --hostname-override=$env:computername --kubeconfig=$KubeDir\config
"@

    $kubeProxyStartStr | Out-File -encoding ASCII -filepath $KubeProxyStartFile

    New-NSSMService -KubeDir $KubeDir `+"`"+`
        -KubeletStartFile $KubeletStartFile `+"`"+`
        -KubeProxyStartFile $KubeProxyStartFile
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
	"linux/cloud-init/artifacts/apt-preferences":                           linuxCloudInitArtifactsAptPreferences,
	"linux/cloud-init/artifacts/auditd-rules":                              linuxCloudInitArtifactsAuditdRules,
	"linux/cloud-init/artifacts/cis.sh":                                    linuxCloudInitArtifactsCisSh,
	"linux/cloud-init/artifacts/containerd.service":                        linuxCloudInitArtifactsContainerdService,
	"linux/cloud-init/artifacts/cse_cmd.sh":                                linuxCloudInitArtifactsCse_cmdSh,
	"linux/cloud-init/artifacts/cse_config.sh":                             linuxCloudInitArtifactsCse_configSh,
	"linux/cloud-init/artifacts/cse_helpers.sh":                            linuxCloudInitArtifactsCse_helpersSh,
	"linux/cloud-init/artifacts/cse_install.sh":                            linuxCloudInitArtifactsCse_installSh,
	"linux/cloud-init/artifacts/cse_main.sh":                               linuxCloudInitArtifactsCse_mainSh,
	"linux/cloud-init/artifacts/dhcpv6.service":                            linuxCloudInitArtifactsDhcpv6Service,
	"linux/cloud-init/artifacts/docker-monitor.service":                    linuxCloudInitArtifactsDockerMonitorService,
	"linux/cloud-init/artifacts/docker-monitor.timer":                      linuxCloudInitArtifactsDockerMonitorTimer,
	"linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf": linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf,
	"linux/cloud-init/artifacts/enable-dhcpv6.sh":                          linuxCloudInitArtifactsEnableDhcpv6Sh,
	"linux/cloud-init/artifacts/etc-issue":                                 linuxCloudInitArtifactsEtcIssue,
	"linux/cloud-init/artifacts/etc-issue.net":                             linuxCloudInitArtifactsEtcIssueNet,
	"linux/cloud-init/artifacts/etcd.service":                              linuxCloudInitArtifactsEtcdService,
	"linux/cloud-init/artifacts/health-monitor.sh":                         linuxCloudInitArtifactsHealthMonitorSh,
	"linux/cloud-init/artifacts/init-aks-custom-cloud.sh":                  linuxCloudInitArtifactsInitAksCustomCloudSh,
	"linux/cloud-init/artifacts/kms.service":                               linuxCloudInitArtifactsKmsService,
	"linux/cloud-init/artifacts/kubelet-monitor.service":                   linuxCloudInitArtifactsKubeletMonitorService,
	"linux/cloud-init/artifacts/kubelet-monitor.timer":                     linuxCloudInitArtifactsKubeletMonitorTimer,
	"linux/cloud-init/artifacts/kubelet.service":                           linuxCloudInitArtifactsKubeletService,
	"linux/cloud-init/artifacts/label-nodes.service":                       linuxCloudInitArtifactsLabelNodesService,
	"linux/cloud-init/artifacts/label-nodes.sh":                            linuxCloudInitArtifactsLabelNodesSh,
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
	"linux/cloud-init/artifacts/sys-fs-bpf.mount":                          linuxCloudInitArtifactsSysFsBpfMount,
	"linux/cloud-init/artifacts/sysctl-d-60-CIS.conf":                      linuxCloudInitArtifactsSysctlD60CisConf,
	"linux/cloud-init/nodecustomdata.yml":                                  linuxCloudInitNodecustomdataYml,
	"windows/csecmd.ps1":                                                   windowsCsecmdPs1,
	"windows/kuberneteswindowsfunctions.ps1":                               windowsKuberneteswindowsfunctionsPs1,
	"windows/kuberneteswindowssetup.ps1":                                   windowsKuberneteswindowssetupPs1,
	"windows/windowsazurecnifunc.ps1":                                      windowsWindowsazurecnifuncPs1,
	"windows/windowsazurecnifunc.tests.ps1":                                windowsWindowsazurecnifuncTestsPs1,
	"windows/windowscnifunc.ps1":                                           windowsWindowscnifuncPs1,
	"windows/windowsconfigfunc.ps1":                                        windowsWindowsconfigfuncPs1,
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
				"apt-preferences":                           &bintree{linuxCloudInitArtifactsAptPreferences, map[string]*bintree{}},
				"auditd-rules":                              &bintree{linuxCloudInitArtifactsAuditdRules, map[string]*bintree{}},
				"cis.sh":                                    &bintree{linuxCloudInitArtifactsCisSh, map[string]*bintree{}},
				"containerd.service":                        &bintree{linuxCloudInitArtifactsContainerdService, map[string]*bintree{}},
				"cse_cmd.sh":                                &bintree{linuxCloudInitArtifactsCse_cmdSh, map[string]*bintree{}},
				"cse_config.sh":                             &bintree{linuxCloudInitArtifactsCse_configSh, map[string]*bintree{}},
				"cse_helpers.sh":                            &bintree{linuxCloudInitArtifactsCse_helpersSh, map[string]*bintree{}},
				"cse_install.sh":                            &bintree{linuxCloudInitArtifactsCse_installSh, map[string]*bintree{}},
				"cse_main.sh":                               &bintree{linuxCloudInitArtifactsCse_mainSh, map[string]*bintree{}},
				"dhcpv6.service":                            &bintree{linuxCloudInitArtifactsDhcpv6Service, map[string]*bintree{}},
				"docker-monitor.service":                    &bintree{linuxCloudInitArtifactsDockerMonitorService, map[string]*bintree{}},
				"docker-monitor.timer":                      &bintree{linuxCloudInitArtifactsDockerMonitorTimer, map[string]*bintree{}},
				"docker_clear_mount_propagation_flags.conf": &bintree{linuxCloudInitArtifactsDocker_clear_mount_propagation_flagsConf, map[string]*bintree{}},
				"enable-dhcpv6.sh":                          &bintree{linuxCloudInitArtifactsEnableDhcpv6Sh, map[string]*bintree{}},
				"etc-issue":                                 &bintree{linuxCloudInitArtifactsEtcIssue, map[string]*bintree{}},
				"etc-issue.net":                             &bintree{linuxCloudInitArtifactsEtcIssueNet, map[string]*bintree{}},
				"etcd.service":                              &bintree{linuxCloudInitArtifactsEtcdService, map[string]*bintree{}},
				"health-monitor.sh":                         &bintree{linuxCloudInitArtifactsHealthMonitorSh, map[string]*bintree{}},
				"init-aks-custom-cloud.sh":                  &bintree{linuxCloudInitArtifactsInitAksCustomCloudSh, map[string]*bintree{}},
				"kms.service":                               &bintree{linuxCloudInitArtifactsKmsService, map[string]*bintree{}},
				"kubelet-monitor.service":                   &bintree{linuxCloudInitArtifactsKubeletMonitorService, map[string]*bintree{}},
				"kubelet-monitor.timer":                     &bintree{linuxCloudInitArtifactsKubeletMonitorTimer, map[string]*bintree{}},
				"kubelet.service":                           &bintree{linuxCloudInitArtifactsKubeletService, map[string]*bintree{}},
				"label-nodes.service":                       &bintree{linuxCloudInitArtifactsLabelNodesService, map[string]*bintree{}},
				"label-nodes.sh":                            &bintree{linuxCloudInitArtifactsLabelNodesSh, map[string]*bintree{}},
				"modprobe-CIS.conf":                         &bintree{linuxCloudInitArtifactsModprobeCisConf, map[string]*bintree{}},
				"nvidia-device-plugin.service":              &bintree{linuxCloudInitArtifactsNvidiaDevicePluginService, map[string]*bintree{}},
				"nvidia-docker-daemon.json":                 &bintree{linuxCloudInitArtifactsNvidiaDockerDaemonJson, map[string]*bintree{}},
				"nvidia-modprobe.service":                   &bintree{linuxCloudInitArtifactsNvidiaModprobeService, map[string]*bintree{}},
				"pam-d-common-auth":                         &bintree{linuxCloudInitArtifactsPamDCommonAuth, map[string]*bintree{}},
				"pam-d-common-password":                     &bintree{linuxCloudInitArtifactsPamDCommonPassword, map[string]*bintree{}},
				"pam-d-su":                                  &bintree{linuxCloudInitArtifactsPamDSu, map[string]*bintree{}},
				"profile-d-cis.sh":                          &bintree{linuxCloudInitArtifactsProfileDCisSh, map[string]*bintree{}},
				"pwquality-CIS.conf":                        &bintree{linuxCloudInitArtifactsPwqualityCisConf, map[string]*bintree{}},
				"reconcile-private-hosts.service":           &bintree{linuxCloudInitArtifactsReconcilePrivateHostsService, map[string]*bintree{}},
				"reconcile-private-hosts.sh":                &bintree{linuxCloudInitArtifactsReconcilePrivateHostsSh, map[string]*bintree{}},
				"rsyslog-d-60-CIS.conf":                     &bintree{linuxCloudInitArtifactsRsyslogD60CisConf, map[string]*bintree{}},
				"setup-custom-search-domains.sh":            &bintree{linuxCloudInitArtifactsSetupCustomSearchDomainsSh, map[string]*bintree{}},
				"sshd_config":                               &bintree{linuxCloudInitArtifactsSshd_config, map[string]*bintree{}},
				"sshd_config_1604":                          &bintree{linuxCloudInitArtifactsSshd_config_1604, map[string]*bintree{}},
				"sys-fs-bpf.mount":                          &bintree{linuxCloudInitArtifactsSysFsBpfMount, map[string]*bintree{}},
				"sysctl-d-60-CIS.conf":                      &bintree{linuxCloudInitArtifactsSysctlD60CisConf, map[string]*bintree{}},
			}},
			"nodecustomdata.yml": &bintree{linuxCloudInitNodecustomdataYml, map[string]*bintree{}},
		}},
	}},
	"windows": &bintree{nil, map[string]*bintree{
		"csecmd.ps1":                     &bintree{windowsCsecmdPs1, map[string]*bintree{}},
		"kuberneteswindowsfunctions.ps1": &bintree{windowsKuberneteswindowsfunctionsPs1, map[string]*bintree{}},
		"kuberneteswindowssetup.ps1":     &bintree{windowsKuberneteswindowssetupPs1, map[string]*bintree{}},
		"windowsazurecnifunc.ps1":        &bintree{windowsWindowsazurecnifuncPs1, map[string]*bintree{}},
		"windowsazurecnifunc.tests.ps1":  &bintree{windowsWindowsazurecnifuncTestsPs1, map[string]*bintree{}},
		"windowscnifunc.ps1":             &bintree{windowsWindowscnifuncPs1, map[string]*bintree{}},
		"windowsconfigfunc.ps1":          &bintree{windowsWindowsconfigfuncPs1, map[string]*bintree{}},
		"windowsinstallopensshfunc.ps1":  &bintree{windowsWindowsinstallopensshfuncPs1, map[string]*bintree{}},
		"windowskubeletfunc.ps1":         &bintree{windowsWindowskubeletfuncPs1, map[string]*bintree{}},
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
