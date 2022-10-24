#!/bin/bash

copyPackerFiles() {
  SYSCTL_CONFIG_SRC=/home/packer/sysctl-d-60-CIS.conf
  SYSCTL_CONFIG_DEST=/etc/sysctl.d/60-CIS.conf
  RSYSLOG_CONFIG_SRC=/home/packer/rsyslog-d-60-CIS.conf
  RSYSLOG_CONFIG_DEST=/etc/rsyslog.d/60-CIS.conf
  ETC_ISSUE_CONFIG_SRC=/home/packer/etc-issue
  ETC_ISSUE_CONFIG_DEST=/etc/issue
  ETC_ISSUE_NET_CONFIG_SRC=/home/packer/etc-issue.net
  ETC_ISSUE_NET_CONFIG_DEST=/etc/issue.net
  SSHD_CONFIG_SRC=/home/packer/sshd_config
  SSHD_CONFIG_DEST=/etc/ssh/sshd_config
  MODPROBE_CIS_SRC=/home/packer/modprobe-CIS.conf
  MODPROBE_CIS_DEST=/etc/modprobe.d/CIS.conf
  PWQUALITY_CONF_SRC=/home/packer/pwquality-CIS.conf
  PWQUALITY_CONF_DEST=/etc/security/pwquality.conf
  PAM_D_COMMON_AUTH_SRC=/home/packer/pam-d-common-auth
  PAM_D_COMMON_AUTH_DEST=/etc/pam.d/common-auth
  PAM_D_COMMON_PASSWORD_SRC=/home/packer/pam-d-common-password
  PAM_D_COMMON_PASSWORD_DEST=/etc/pam.d/common-password
  PAM_D_SU_SRC=/home/packer/pam-d-su
  PAM_D_SU_DEST=/etc/pam.d/su
  PROFILE_D_CIS_SH_SRC=/home/packer/profile-d-cis.sh
  PROFILE_D_CIS_SH_DEST=/etc/profile.d/CIS.sh
  CIS_SRC=/home/packer/cis.sh
  CIS_DEST=/opt/azure/containers/provision_cis.sh
  APT_PREFERENCES_SRC=/home/packer/apt-preferences
  APT_PREFERENCES_DEST=/etc/apt/preferences
  KMS_SERVICE_SRC=/home/packer/kms.service
  KMS_SERVICE_DEST=/etc/systemd/system/kms.service
  HEALTH_MONITOR_SRC=/home/packer/health-monitor.sh
  HEALTH_MONITOR_DEST=/usr/local/bin/health-monitor.sh
  KUBELET_MONITOR_SERVICE_SRC=/home/packer/kubelet-monitor.service
  KUBELET_MONITOR_SERVICE_DEST=/etc/systemd/system/kubelet-monitor.service
  DOCKER_MONITOR_SERVICE_SRC=/home/packer/docker-monitor.service
  DOCKER_MONITOR_SERVICE_DEST=/etc/systemd/system/docker-monitor.service
  DOCKER_MONITOR_TIMER_SRC=/home/packer/docker-monitor.timer
  DOCKER_MONITOR_TIMER_DEST=/etc/systemd/system/docker-monitor.timer
  CONTAINERD_MONITOR_SERVICE_SRC=/home/packer/containerd-monitor.service
  CONTAINERD_MONITOR_SERVICE_DEST=/etc/systemd/system/containerd-monitor.service
  CONTAINERD_MONITOR_TIMER_SRC=/home/packer/containerd-monitor.timer
  CONTAINERD_MONITOR_TIMER_DEST=/etc/systemd/system/containerd-monitor.timer
  DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_SRC=/home/packer/docker_clear_mount_propagation_flags.conf
  DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_DEST=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
  IPV6_NFTABLES_RULES_SRC=/home/packer/ipv6_nftables
  IPV6_NFTABLES_RULES_DEST=/etc/systemd/system/ipv6_nftables
  IPV6_NFTABLES_SCRIPT_SRC=/home/packer/ipv6_nftables.sh
  IPV6_NFTABLES_SCRIPT_DEST=/opt/scripts/ipv6_nftables.sh
  IPV6_NFTABLES_SERVICE_SRC=/home/packer/ipv6_nftables.service
  IPV6_NFTABLES_SERVICE_DEST=/etc/systemd/system/ipv6_nftables.service
  NVIDIA_MODPROBE_SERVICE_SRC=/home/packer/nvidia-modprobe.service
  NVIDIA_MODPROBE_SERVICE_DEST=/etc/systemd/system/nvidia-modprobe.service
  NVIDIA_DOCKER_DAEMON_SRC=/home/packer/nvidia-docker-daemon.json
  NVIDIA_DOCKER_DAEMON_DEST=/etc/systemd/system/nvidia-docker-daemon.json
  NVIDIA_DEVICE_PLUGIN_SERVICE_SRC=/home/packer/nvidia-device-plugin.service
  NVIDIA_DEVICE_PLUGIN_SERVICE_DEST=/etc/systemd/system/nvidia-device-plugin.service
  DISK_QUEUE_SERVICE_SRC=/home/packer/disk_queue.service
  DISK_QUEUE_SERVICE_DEST=/etc/systemd/system/disk_queue.service
  UPDATE_CERTS_SERVICE_SRC=/home/packer/update_certs.service
  UPDATE_CERTS_SERVICE_DEST=/etc/systemd/system/update_certs.service
  UPDATE_CERTS_PATH_SRC=/home/packer/update_certs.path
  UPDATE_CERTS_PATH_DEST=/etc/systemd/system/update_certs.path
  UPDATE_CERTS_TIMER_SRC=/home/packer/update_certs.timer
  UPDATE_CERTS_TIMER_DEST=/etc/systemd/system/update_certs.timer
  UPDATE_CERTS_SCRIPT_SRC=/home/packer/update_certs.sh
  UPDATE_CERTS_SCRIPT_DEST=/opt/scripts/update_certs.sh
  NOTICE_SRC=/home/packer/NOTICE.txt
  NOTICE_DEST=/NOTICE.txt
  if [[ ${UBUNTU_RELEASE} == "16.04" ]]; then
    SSHD_CONFIG_SRC=/home/packer/sshd_config_1604
  elif [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
    SSHD_CONFIG_SRC=/home/packer/sshd_config_1804_fips
  fi

  cpAndMode $SYSCTL_CONFIG_SRC $SYSCTL_CONFIG_DEST 644
  cpAndMode $RSYSLOG_CONFIG_SRC $RSYSLOG_CONFIG_DEST 644
  cpAndMode $ETC_ISSUE_CONFIG_SRC $ETC_ISSUE_CONFIG_DEST 644
  cpAndMode $ETC_ISSUE_NET_CONFIG_SRC $ETC_ISSUE_NET_CONFIG_DEST 644
  cpAndMode $SSHD_CONFIG_SRC $SSHD_CONFIG_DEST 644
  cpAndMode $MODPROBE_CIS_SRC $MODPROBE_CIS_DEST 644
  cpAndMode $PWQUALITY_CONF_SRC $PWQUALITY_CONF_DEST 600
  cpAndMode $PAM_D_COMMON_AUTH_SRC $PAM_D_COMMON_AUTH_DEST 644
  cpAndMode $PAM_D_COMMON_PASSWORD_SRC $PAM_D_COMMON_PASSWORD_DEST 644
  cpAndMode $PAM_D_SU_SRC $PAM_D_SU_DEST 644
  cpAndMode $PROFILE_D_CIS_SH_SRC $PROFILE_D_CIS_SH_DEST 755
  cpAndMode $CIS_SRC $CIS_DEST 744
  cpAndMode $APT_PREFERENCES_SRC $APT_PREFERENCES_DEST 644
  cpAndMode $KMS_SERVICE_SRC $KMS_SERVICE_DEST 644
  cpAndMode $HEALTH_MONITOR_SRC $HEALTH_MONITOR_DEST 544
  cpAndMode $KUBELET_MONITOR_SERVICE_SRC $KUBELET_MONITOR_SERVICE_DEST 644
  cpAndMode $CONTAINERD_MONITOR_SERVICE_SRC $CONTAINERD_MONITOR_SERVICE_DEST 644
  cpAndMode $CONTAINERD_MONITOR_TIMER_SRC $CONTAINERD_MONITOR_TIMER_DEST 644
  cpAndMode $DISK_QUEUE_SERVICE_SRC $DISK_QUEUE_SERVICE_DEST 644
  cpAndMode $UPDATE_CERTS_SERVICE_SRC $UPDATE_CERTS_SERVICE_DEST 644
  cpAndMode $UPDATE_CERTS_PATH_SRC $UPDATE_CERTS_PATH_DEST 644
  cpAndMode $UPDATE_CERTS_TIMER_SRC $UPDATE_CERTS_TIMER_DEST 644
  cpAndMode $UPDATE_CERTS_SCRIPT_SRC $UPDATE_CERTS_SCRIPT_DEST 755
  cpAndMode $IPV6_NFTABLES_RULES_SRC $IPV6_NFTABLES_RULES_DEST 644
  cpAndMode $IPV6_NFTABLES_SCRIPT_SRC $IPV6_NFTABLES_SCRIPT_DEST 755
  cpAndMode $IPV6_NFTABLES_SERVICE_SRC $IPV6_NFTABLES_SERVICE_DEST 644
  if [[ $OS != $MARINER_OS_NAME ]]; then
    cpAndMode $DOCKER_MONITOR_SERVICE_SRC $DOCKER_MONITOR_SERVICE_DEST 644
    cpAndMode $DOCKER_MONITOR_TIMER_SRC $DOCKER_MONITOR_TIMER_DEST 644
    cpAndMode $DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_SRC $DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_DEST 644
  fi
  if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    cpAndMode $NVIDIA_MODPROBE_SERVICE_SRC $NVIDIA_MODPROBE_SERVICE_DEST 644
    cpAndMode $NVIDIA_DOCKER_DAEMON_SRC $NVIDIA_DOCKER_DAEMON_DEST 644
    if grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
      cpAndMode $NVIDIA_DEVICE_PLUGIN_SERVICE_SRC $NVIDIA_DEVICE_PLUGIN_SERVICE_DEST 644
    fi
  fi

  cpAndMode $NOTICE_SRC $NOTICE_DEST 444
}

cpAndMode() {
  src=$1; dest=$2; mode=$3
  DIR=$(dirname "$dest") && mkdir -p ${DIR} && cp $src $dest && chmod $mode $dest || exit $ERR_PACKER_COPY_FILE
}