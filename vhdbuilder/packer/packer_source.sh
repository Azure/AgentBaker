#!/bin/bash

copyPackerFiles() {
  SYSCTL_CONFIG_SRC=/home/packer/sysctl-d-60-CIS.conf
  SYSCTL_CONFIG_DEST=/etc/sysctl.d/60-CIS.conf
  RSYSLOG_CONFIG_SRC=/home/packer/rsyslog-d-60-CIS.conf
  RSYSLOG_CONFIG_DEST=/etc/rsyslog.d/60-CIS.conf
  LOGROTATE_CIS_CONFIG_SRC=/home/packer/logrotate-d-rsyslog-CIS.conf
  LOGROTATE_CIS_CONFIG_DEST=/etc/logrotate.d/rsyslog-cis.conf
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
  PAM_D_SYSTEM_AUTH_SRC=/home/packer/pam-d-system-auth
  PAM_D_SYSTEM_AUTH_DEST=/etc/pam.d/system-auth
  PAM_D_SYSTEM_PASSWORD_SRC=/home/packer/pam-d-system-password
  PAM_D_SYSTEM_PASSWORD_DEST=/etc/pam.d/system-password
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
  MIG_PARTITION_SRC=/home/packer/mig-partition.sh
  MIG_PARTITION_DEST=/opt/azure/containers/mig-partition.sh
  CONTAINERD_EXEC_START_SRC=/home/packer/containerd_exec_start.conf
  CONTAINERD_EXEC_START_DEST=/etc/systemd/system/containerd.service.d/exec_start.conf
  CONTAINERD_SERVICE_SRC=/home/packer/containerd.service
  CONTAINERD_SERVICE_DEST=/etc/systemd/system/containerd.service
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
  CGROUP_MEMORY_TELEMETRY_SERVICE_SRC=/home/packer/cgroup-memory-telemetry.service
  CGROUP_MEMORY_TELEMETRY_SERVICE_DEST=/etc/systemd/system/cgroup-memory-telemetry.service
  CGROUP_MEMORY_TELEMETRY_SCRIPT_SRC=/home/packer/cgroup-memory-telemetry.sh
  CGROUP_MEMORY_TELEMETRY_SCRIPT_DEST=/opt/scripts/cgroup-memory-telemetry.sh
  CGROUP_MEMORY_TELEMETRY_TIMER_SRC=/home/packer/cgroup-memory-telemetry.timer
  CGROUP_MEMORY_TELEMETRY_TIMER_DEST=/etc/systemd/system/cgroup-memory-telemetry.timer
  CGROUP_PRESSURE_TELEMETRY_SERVICE_SRC=/home/packer/cgroup-pressure-telemetry.service
  CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST=/etc/systemd/system/cgroup-pressure-telemetry.service
  CGROUP_PRESSURE_TELEMETRY_SCRIPT_SRC=/home/packer/cgroup-pressure-telemetry.sh
  CGROUP_PRESSURE_TELEMETRY_SCRIPT_DEST=/opt/scripts/cgroup-pressure-telemetry.sh
  CGROUP_PRESSURE_TELEMETRY_TIMER_SRC=/home/packer/cgroup-pressure-telemetry.timer
  CGROUP_PRESSURE_TELEMETRY_TIMER_DEST=/etc/systemd/system/cgroup-pressure-telemetry.timer
  UPDATE_CERTS_SERVICE_SRC=/home/packer/update_certs.service
  UPDATE_CERTS_SERVICE_DEST=/etc/systemd/system/update_certs.service
  UPDATE_CERTS_PATH_SRC=/home/packer/update_certs.path
  UPDATE_CERTS_PATH_DEST=/etc/systemd/system/update_certs.path
  UPDATE_CERTS_SCRIPT_SRC=/home/packer/update_certs.sh
  UPDATE_CERTS_SCRIPT_DEST=/opt/scripts/update_certs.sh
  CI_SYSLOG_WATCHER_PATH_SRC=/home/packer/ci-syslog-watcher.path
  CI_SYSLOG_WATCHER_PATH_DEST=/etc/systemd/system/ci-syslog-watcher.path
  CI_SYSLOG_WATCHER_SERVICE_SRC=/home/packer/ci-syslog-watcher.service
  CI_SYSLOG_WATCHER_SERVICE_DEST=/etc/systemd/system/ci-syslog-watcher.service
  CI_SYSLOG_WATCHER_SCRIPT_SRC=/home/packer/ci-syslog-watcher.sh
  CI_SYSLOG_WATCHER_SCRIPT_DEST=/usr/local/bin/ci-syslog-watcher.sh
  AKS_LOG_COLLECTOR_SCRIPT_SRC=/home/packer/aks-log-collector.sh
  AKS_LOG_COLLECTOR_SCRIPT_DEST=/opt/azure/containers/aks-log-collector.sh
  AKS_LOG_COLLECTOR_SEND_SCRIPT_SRC=/home/packer/aks-log-collector-send.py
  AKS_LOG_COLLECTOR_SEND_SCRIPT_DEST=/opt/azure/containers/aks-log-collector-send.py
  AKS_LOG_COLLECTOR_SERVICE_SRC=/home/packer/aks-log-collector.service
  AKS_LOG_COLLECTOR_SERVICE_DEST=/etc/systemd/system/aks-log-collector.service
  AKS_LOG_COLLECTOR_SLICE_SRC=/home/packer/aks-log-collector.slice
  AKS_LOG_COLLECTOR_SLICE_DEST=/etc/systemd/system/aks-log-collector.slice
  AKS_LOG_COLLECTOR_TIMER_SRC=/home/packer/aks-log-collector.timer
  AKS_LOG_COLLECTOR_TIMER_DEST=/etc/systemd/system/aks-log-collector.timer
  AKS_LOGROTATE_SCRIPT_SRC=/home/packer/logrotate.sh
  AKS_LOGROTATE_SCRIPT_DEST=/usr/local/bin/logrotate.sh
  AKS_LOGROTATE_SERVICE_SRC=/home/packer/logrotate.service
  AKS_LOGROTATE_SERVICE_DEST=/etc/systemd/system/logrotate.service
  AKS_LOGROTATE_TIMER_SRC=/home/packer/logrotate.timer
  AKS_LOGROTATE_TIMER_DEST=/etc/systemd/system/logrotate.timer
  AKS_LOGROTATE_TIMER_DROPIN_SRC=/home/packer/override.conf
  AKS_LOGROTATE_TIMER_DROPIN_DEST=/etc/systemd/system/logrotate.timer.d/override.conf
  AKS_LOGROTATE_CONF_SRC=/home/packer/rsyslog
  AKS_LOGROTATE_CONF_DEST=/etc/logrotate.d/rsyslog
  AKS_CHECK_NETWORK_SCRIPT_SRC=/home/packer/aks-check-network.sh
  AKS_CHECK_NETWORK_SCRIPT_DEST=/opt/azure/containers/aks-check-network.sh
  AKS_CHECK_NETWORK_SERVICE_SRC=/home/packer/aks-check-network.service
  AKS_CHECK_NETWORK_SERVICE_DEST=/etc/systemd/system/aks-check-network.service
  BLOCK_WIRESERVER_SRC=/home/packer/block_wireserver.sh
  BLOCK_WIRESERVER_DEST=/opt/azure/containers/kubelet.sh
  ENSURE_IMDS_RESTRICTION_SRC=/home/packer/ensure_imds_restriction.sh
  ENSURE_IMDS_RESTRICTION_DEST=/opt/azure/containers/ensure_imds_restriction.sh
  RECONCILE_PRIVATE_HOSTS_SRC=/home/packer/reconcile-private-hosts.sh
  RECONCILE_PRIVATE_HOSTS_DEST=/opt/azure/containers/reconcilePrivateHosts.sh
  KUBELET_SERVICE_SRC=/home/packer/kubelet.service
  KUBELET_SERVICE_DEST=/etc/systemd/system/kubelet.service
  USU_SH_SRC=/home/packer/ubuntu-snapshot-update.sh
  USU_SH_DEST=/opt/azure/containers/ubuntu-snapshot-update.sh
  MPU_SH_SRC=/home/packer/mariner-package-update.sh
  MPU_SH_DEST=/opt/azure/containers/mariner-package-update.sh
  SNAPSHOT_UPDATE_SERVICE_SRC=/home/packer/snapshot-update.service
  SNAPSHOT_UPDATE_SERVICE_DEST=/etc/systemd/system/snapshot-update.service
  SNAPSHOT_UPDATE_TIMER_SRC=/home/packer/snapshot-update.timer
  SNAPSHOT_UPDATE_TIMER_DEST=/etc/systemd/system/snapshot-update.timer
  VHD_CLEANUP_SCRIPT_SRC=/home/packer/cleanup-vhd.sh
  VHD_CLEANUP_SCRIPT_DEST=/opt/azure/containers/cleanup-vhd.sh
  CONTAINER_IMAGE_PREFETCH_SCRIPT_SRC=/home/packer/prefetch.sh
  CONTAINER_IMAGE_PREFETCH_SCRIPT_DEST=/opt/azure/containers/prefetch.sh

  CSE_REDACT_SRC=/home/packer/cse_redact_cloud_config.py
  CSE_REDACT_DEST=/opt/azure/containers/provision_redact_cloud_config.py
  cpAndMode $CSE_REDACT_SRC $CSE_REDACT_DEST 0744

  CSE_SEND_SRC=/home/packer/cse_send_logs.py
  CSE_SEND_DEST=/opt/azure/containers/provision_send_logs.py
  cpAndMode $CSE_SEND_SRC $CSE_SEND_DEST 0744

  INIT_CUSTOM_CLOUD_SRC=/home/packer/init-aks-custom-cloud.sh
  INIT_CUSTOM_CLOUD_DEST=/opt/azure/containers/init-aks-custom-cloud.sh
  cpAndMode $INIT_CUSTOM_CLOUD_SRC $INIT_CUSTOM_CLOUD_DEST 0744

  PVT_HOST_SVC_SRC=/home/packer/reconcile-private-hosts.service
  PVT_HOST_SVC_DEST=/etc/systemd/system/reconcile-private-hosts.service
  cpAndMode $CSE_REDACT_SRC $CSE_REDACT_DEST 600

  if grep -q "kata" <<< "$FEATURE_FLAGS"; then
    # KataCC SPEC file assumes kata config points to the files exactly under this path
    KATA_CONFIG_DIR=/var/cache/kata-containers/osbuilder-images/kernel-uvm/
    KATACC_CONFIG_DIR=/opt/confidential-containers/share/kata-containers

    IGVM_DEBUG_BIN_SRC=/home/packer/kata-containers-igvm-debug.img
    IGVM_DEBUG_BIN_DEST=$KATACC_CONFIG_DIR/kata-containers-igvm-debug.img
    cpAndMode $IGVM_DEBUG_BIN_SRC $IGVM_DEBUG_BIN_DEST 0755

    IGVM_BIN_SRC=/home/packer/kata-containers-igvm.img
    IGVM_BIN_DEST=$KATACC_CONFIG_DIR/kata-containers-igvm.img
    cpAndMode $IGVM_BIN_SRC $IGVM_BIN_DEST 0755

    KATA_INITRD_SRC=/home/packer/kata-containers-initrd-base.img
    KATA_INITRD_DEST=$KATA_CONFIG_DIR/kata-containers-initrd.img
    cpAndMode $KATA_INITRD_SRC $KATA_INITRD_DEST 0755

    KATACC_IMAGE_SRC=/home/packer/kata-containers.img
    KATACC_IMAGE_DEST=$KATACC_CONFIG_DIR/kata-containers.img
    cpAndMode $KATACC_IMAGE_SRC $KATACC_IMAGE_DEST 0755

    REF_INFO_SRC=/home/packer/reference-info-base64
    REF_INFO_DEST=$KATACC_CONFIG_DIR/reference-info-base64
    cpAndMode $REF_INFO_SRC $REF_INFO_DEST 0755
  fi

  MIG_PART_SRC=/home/packer/mig-partition.service
  MIG_PART_DEST=/etc/systemd/system/mig-partition.service
  cpAndMode $MIG_PART_SRC $MIG_PART_DEST 600

  MNT_SH_SRC=/home/packer/bind-mount.sh
  MNT_SH_DEST=/opt/azure/containers/bind-mount.sh
  cpAndMode $MNT_SH_SRC $MNT_SH_DEST 0544

  MNT_SVC_SRC=/home/packer/bind-mount.service
  MNT_SVC_DEST=/etc/systemd/system/bind-mount.service
  cpAndMode $MNT_SVC_SRC $MNT_SVC_DEST 600

  DHCP6_SH_SRC=/home/packer/enable-dhcpv6.sh
  DHCP6_SH_DEST=/opt/azure/containers/enable-dhcpv6.sh
  cpAndMode $DHCP6_SH_SRC $DHCP6_SH_DEST 0544

  DHCP6_SVC_SRC=/home/packer/dhcpv6.service
  DHCP6_SVC_DEST=/etc/systemd/system/dhcpv6.service
  cpAndMode $DHCP6_SVC_SRC $DHCP6_SVC_DEST 600

  SYNC_LOGS_SH_SRC=/home/packer/sync-container-logs.sh
  SYNC_LOGS_SH_DEST=/opt/azure/containers/sync-container-logs.sh
  cpAndMode $SYNC_LOGS_SH_SRC $SYNC_LOGS_SH_DEST 0544

  SYNC_LOGS_SVC_SRC=/home/packer/sync-container-logs.service
  SYNC_LOGS_SVC_DEST=/etc/systemd/system/sync-container-logs.service
  cpAndMode $SYNC_LOGS_SVC_SRC $SYNC_LOGS_SVC_DEST 600

  CRICTL_SRC=/home/packer/crictl.yaml
  CRICTL_DEST=/etc/crictl.yaml
  cpAndMode $CRICTL_SRC $CRICTL_DEST 0644

  NO_DUP_SH_SRC=/home/packer/ensure-no-dup.sh
  NO_DUP_SH_DEST=/opt/azure/containers/ensure-no-dup.sh
  cpAndMode $NO_DUP_SH_SRC $NO_DUP_SH_DEST 0755

  NO_DUP_SVC_SRC=/home/packer/ensure-no-dup.service
  NO_DUP_SVC_DEST=/etc/systemd/system/ensure-no-dup.service
  cpAndMode $NO_DUP_SVC_SRC $NO_DUP_SVC_DEST 600

  TELED_SRC=/home/packer/teleportd.service
  TELED_DEST=/etc/systemd/system/teleportd.service
  cpAndMode $TELED_SRC $TELED_DEST 600

  SETUP_SEARCH_SRC=/home/packer/setup-custom-search-domains.sh
  SETUP_SEARCH_DEST=/opt/azure/containers/setup-custom-search-domains.sh
  cpAndMode $SETUP_SEARCH_SRC $SETUP_SEARCH_DEST 0744

  CSE_MAIN_SRC=/home/packer/provision.sh
  CSE_MAIN_DEST=/opt/azure/containers/provision.sh
  cpAndMode $CSE_MAIN_SRC $CSE_MAIN_DEST 0744

  CSE_START_SRC=/home/packer/provision_start.sh
  CSE_START_DEST=/opt/azure/containers/provision_start.sh
  cpAndMode $CSE_START_SRC $CSE_START_DEST 0744

  CSE_CONFIG_SRC=/home/packer/provision_configs.sh
  CSE_CONFIG_DEST=/opt/azure/containers/provision_configs.sh
  cpAndMode $CSE_CONFIG_SRC $CSE_CONFIG_DEST 0744

  CSE_INSTALL_SRC=/home/packer/provision_installs.sh
  CSE_INSTALL_DEST=/opt/azure/containers/provision_installs.sh
  cpAndMode $CSE_INSTALL_SRC $CSE_INSTALL_DEST 0744

  CSE_INSTALL_DISTRO_SRC=/home/packer/provision_installs_distro.sh
  CSE_INSTALL_DISTRO_DEST=/opt/azure/containers/provision_installs_distro.sh
  cpAndMode $CSE_INSTALL_DISTRO_SRC $CSE_INSTALL_DISTRO_DEST 0744

  CSE_HELPERS_SRC=/home/packer/provision_source.sh
  CSE_HELPERS_DEST=/opt/azure/containers/provision_source.sh
  cpAndMode $CSE_HELPERS_SRC $CSE_HELPERS_DEST 0744

  CSE_HELPERS_DISTRO_SRC=/home/packer/provision_source_distro.sh
  CSE_HELPERS_DISTRO_DEST=/opt/azure/containers/provision_source_distro.sh
  cpAndMode $CSE_HELPERS_DISTRO_SRC $CSE_HELPERS_DISTRO_DEST 0744

  NBC_PARSER_SRC=/home/packer/nbcparser
  NBC_PARSER_DEST=/opt/azure/containers/nbcparser
  cpAndMode $NBC_PARSER_SRC $NBC_PARSER_DEST 0755

  NOTICE_SRC=/home/packer/NOTICE.txt
  NOTICE_DEST=/NOTICE.txt

  if [[ ${UBUNTU_RELEASE} == "16.04" ]]; then
    SSHD_CONFIG_SRC=/home/packer/sshd_config_1604
  elif [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
    SSHD_CONFIG_SRC=/home/packer/sshd_config_1804_fips
  elif [[ ${UBUNTU_RELEASE} == "22.04" && ${ENABLE_FIPS,,} == "true" ]]; then
    SSHD_CONFIG_SRC=/home/packer/sshd_config_2204_fips
  fi

  # Install AKS log collector
  cpAndMode $AKS_LOG_COLLECTOR_SCRIPT_SRC $AKS_LOG_COLLECTOR_SCRIPT_DEST 755
  cpAndMode $AKS_LOG_COLLECTOR_SEND_SCRIPT_SRC $AKS_LOG_COLLECTOR_SEND_SCRIPT_DEST 755
  cpAndMode $AKS_LOG_COLLECTOR_SERVICE_SRC $AKS_LOG_COLLECTOR_SERVICE_DEST 644
  cpAndMode $AKS_LOG_COLLECTOR_SLICE_SRC $AKS_LOG_COLLECTOR_SLICE_DEST 644
  cpAndMode $AKS_LOG_COLLECTOR_TIMER_SRC $AKS_LOG_COLLECTOR_TIMER_DEST 644

  cpAndMode $AKS_LOGROTATE_CONF_SRC $AKS_LOGROTATE_CONF_DEST 644
  # If a logrotation timer does not exist on the base image
  if [ ! -f /etc/systemd/system/logrotate.timer ] && [ ! -f /usr/lib/systemd/system/logrotate.timer ]; then
    cpAndMode $AKS_LOGROTATE_SCRIPT_SRC $AKS_LOGROTATE_SCRIPT_DEST 544
    cpAndMode $AKS_LOGROTATE_SERVICE_SRC $AKS_LOGROTATE_SERVICE_DEST 644
    cpAndMode $AKS_LOGROTATE_TIMER_SRC $AKS_LOGROTATE_TIMER_DEST 644
  else
    cpAndMode $AKS_LOGROTATE_TIMER_DROPIN_SRC $AKS_LOGROTATE_TIMER_DROPIN_DEST 644
  fi

  cpAndMode $AKS_CHECK_NETWORK_SCRIPT_SRC $AKS_CHECK_NETWORK_SCRIPT_DEST 755
  cpAndMode $AKS_CHECK_NETWORK_SERVICE_SRC $AKS_CHECK_NETWORK_SERVICE_DEST 644

  if [[ ${UBUNTU_RELEASE} == "22.04" ]]; then
    PAM_D_COMMON_AUTH_SRC=/home/packer/pam-d-common-auth-2204
  fi

  cpAndMode $KUBELET_SERVICE_SRC $KUBELET_SERVICE_DEST 600
  cpAndMode $BLOCK_WIRESERVER_SRC $BLOCK_WIRESERVER_DEST 755
  cpAndMode $ENSURE_IMDS_RESTRICTION_SRC $ENSURE_IMDS_RESTRICTION_DEST 755
  cpAndMode $RECONCILE_PRIVATE_HOSTS_SRC $RECONCILE_PRIVATE_HOSTS_DEST 744
  cpAndMode $SYSCTL_CONFIG_SRC $SYSCTL_CONFIG_DEST 644
  cpAndMode $RSYSLOG_CONFIG_SRC $RSYSLOG_CONFIG_DEST 644
  cpAndMode $LOGROTATE_CIS_CONFIG_SRC $LOGROTATE_CIS_CONFIG_DEST 644
  cpAndMode $ETC_ISSUE_CONFIG_SRC $ETC_ISSUE_CONFIG_DEST 644
  cpAndMode $ETC_ISSUE_NET_CONFIG_SRC $ETC_ISSUE_NET_CONFIG_DEST 644
  cpAndMode $SSHD_CONFIG_SRC $SSHD_CONFIG_DEST 600
  cpAndMode $MODPROBE_CIS_SRC $MODPROBE_CIS_DEST 644
  cpAndMode $PWQUALITY_CONF_SRC $PWQUALITY_CONF_DEST 600
  cpAndMode $PAM_D_SU_SRC $PAM_D_SU_DEST 644
  cpAndMode $PROFILE_D_CIS_SH_SRC $PROFILE_D_CIS_SH_DEST 755
  cpAndMode $CIS_SRC $CIS_DEST 744
  cpAndMode $APT_PREFERENCES_SRC $APT_PREFERENCES_DEST 644
  cpAndMode $KMS_SERVICE_SRC $KMS_SERVICE_DEST 644
  cpAndMode $MIG_PARTITION_SRC $MIG_PARTITION_DEST 544
  cpAndMode $CONTAINERD_EXEC_START_SRC $CONTAINERD_EXEC_START_DEST 644
  cpAndMode $DISK_QUEUE_SERVICE_SRC $DISK_QUEUE_SERVICE_DEST 644
  cpAndMode $CGROUP_MEMORY_TELEMETRY_SERVICE_SRC $CGROUP_MEMORY_TELEMETRY_SERVICE_DEST 644
  cpAndMode $CGROUP_MEMORY_TELEMETRY_SCRIPT_SRC $CGROUP_MEMORY_TELEMETRY_SCRIPT_DEST 755
  cpAndMode $CGROUP_MEMORY_TELEMETRY_TIMER_SRC $CGROUP_MEMORY_TELEMETRY_TIMER_DEST 644
  cpAndMode $CGROUP_PRESSURE_TELEMETRY_SERVICE_SRC $CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST 644
  cpAndMode $CGROUP_PRESSURE_TELEMETRY_SCRIPT_SRC $CGROUP_PRESSURE_TELEMETRY_SCRIPT_DEST 755
  cpAndMode $CGROUP_PRESSURE_TELEMETRY_TIMER_SRC $CGROUP_PRESSURE_TELEMETRY_TIMER_DEST 644
  cpAndMode $UPDATE_CERTS_SERVICE_SRC $UPDATE_CERTS_SERVICE_DEST 644
  cpAndMode $UPDATE_CERTS_PATH_SRC $UPDATE_CERTS_PATH_DEST 644
  cpAndMode $UPDATE_CERTS_SCRIPT_SRC $UPDATE_CERTS_SCRIPT_DEST 755
  cpAndMode $IPV6_NFTABLES_RULES_SRC $IPV6_NFTABLES_RULES_DEST 644
  cpAndMode $IPV6_NFTABLES_SCRIPT_SRC $IPV6_NFTABLES_SCRIPT_DEST 755
  cpAndMode $IPV6_NFTABLES_SERVICE_SRC $IPV6_NFTABLES_SERVICE_DEST 644
  cpAndMode $CI_SYSLOG_WATCHER_PATH_SRC $CI_SYSLOG_WATCHER_PATH_DEST 644
  cpAndMode $CI_SYSLOG_WATCHER_SERVICE_SRC $CI_SYSLOG_WATCHER_SERVICE_DEST 644
  cpAndMode $CI_SYSLOG_WATCHER_SCRIPT_SRC $CI_SYSLOG_WATCHER_SCRIPT_DEST 755
  cpAndMode $SNAPSHOT_UPDATE_SERVICE_SRC $SNAPSHOT_UPDATE_SERVICE_DEST 644
  cpAndMode $SNAPSHOT_UPDATE_TIMER_SRC $SNAPSHOT_UPDATE_TIMER_DEST 644

  if ! isMarinerOrAzureLinux "$OS"; then
    cpAndMode $DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_SRC $DOCKER_CLEAR_MOUNT_PROPAGATION_FLAGS_DEST 644
    cpAndMode $NVIDIA_MODPROBE_SERVICE_SRC $NVIDIA_MODPROBE_SERVICE_DEST 644
    cpAndMode $PAM_D_COMMON_AUTH_SRC $PAM_D_COMMON_AUTH_DEST 644
    cpAndMode $PAM_D_COMMON_PASSWORD_SRC $PAM_D_COMMON_PASSWORD_DEST 644
    cpAndMode $USU_SH_SRC $USU_SH_DEST 544
  fi
  if isMarinerOrAzureLinux "$OS"; then
    cpAndMode $CONTAINERD_SERVICE_SRC $CONTAINERD_SERVICE_DEST 644
    cpAndMode $MPU_SH_SRC $MPU_SH_DEST 544

    # Mariner/AzureLinux uses system-auth and system-password instead of common-auth and common-password.
    if isMarinerOrAzureLinux "$OS"; then
      cpAndMode $PAM_D_SYSTEM_AUTH_SRC $PAM_D_SYSTEM_AUTH_DEST 644
      cpAndMode $PAM_D_SYSTEM_PASSWORD_SRC $PAM_D_SYSTEM_PASSWORD_DEST 644
    else
      cpAndMode $PAM_D_COMMON_AUTH_SRC $PAM_D_COMMON_AUTH_DEST 644
      cpAndMode $PAM_D_COMMON_PASSWORD_SRC $PAM_D_COMMON_PASSWORD_DEST 644
    fi
  fi

  if grep -q "fullgpu" <<<"$FEATURE_FLAGS"; then
    cpAndMode $NVIDIA_DOCKER_DAEMON_SRC $NVIDIA_DOCKER_DAEMON_DEST 644
    if grep -q "gpudaemon" <<<"$FEATURE_FLAGS"; then
      cpAndMode $NVIDIA_DEVICE_PLUGIN_SERVICE_SRC $NVIDIA_DEVICE_PLUGIN_SERVICE_DEST 644
    fi
  fi

  cpAndMode $NOTICE_SRC $NOTICE_DEST 444

  # Always copy the VHD cleanup script responsible for prepping the instance for first boot
  # to disk so we can run it again if needed in subsequent builds/releases (prefetch during SIG release)
  cpAndMode $VHD_CLEANUP_SCRIPT_SRC $VHD_CLEANUP_SCRIPT_DEST 644

  # Copy the generated CNI prefetch script to the appropriate location so AIB can invoke it later
  cpAndMode $CONTAINER_IMAGE_PREFETCH_SCRIPT_SRC $CONTAINER_IMAGE_PREFETCH_SCRIPT_DEST 644
}

cpAndMode() {
  src=$1
  dest=$2
  mode=$3
  DIR=$(dirname "$dest") && mkdir -p ${DIR} && cp $src $dest && chmod $mode $dest || exit $ERR_PACKER_COPY_FILE
}
