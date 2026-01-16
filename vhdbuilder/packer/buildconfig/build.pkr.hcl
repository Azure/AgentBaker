build {
  sources = ["source.azure-arm.nodelifecycle-image-builder"]

  provisioner "shell" {
    inline = [
      "sudo mkdir -p /opt/azure/containers",
      "sudo mkdir -p /opt/scripts",
      "sudo mkdir -p /opt/certs"
    ]
  }

  dynamic "provisioner" {
    for_each = "${local.common_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
    }
  }

  dynamic "provisioner" {
    for_each = "${local.ubuntu_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "ubuntu"
    }
  }

  dynamic "provisioner" {
    for_each = "${local.azlinux_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "cblmariner"
    }
  }

  dynamic "provisioner" {
    for_each = "${local.flatcar_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "flatcar"
    }
  }

  provisioner "file" {
    destination = "${var.aks_node_controller}"
    source      = "/home/packer/aks-node-controller"
  }

  provisioner "file" {
    destination = "/home/packer/lister"
    source      = "vhdbuilder/lister/bin/lister"
  }

  provisioner "file" {
    destination = "/home/packer/aks-node-controller"
    source      = "aks-node-controller/bin/aks-node-controller-linux-amd64"
  }

  provisioner "file" {
    destination = "/home/packer/aks-node-controller.service"
    source      = "parts/linux/cloud-init/artifacts/aks-node-controller.service"
  }

  provisioner "file" {
    destination = "/home/packer/aks-node-controller-wrapper.sh"
    source      = "parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh"
  }

  provisioner "file" {
    destination = "/home/packer/cloud-init-status-check.sh"
    source      = "parts/linux/cloud-init/artifacts/cloud-init-status-check.sh"
  }

  provisioner "file" {
    destination = "/home/packer/prefetch.sh"
    source      = "vhdbuilder/packer/prefetch.sh"
  }

  provisioner "file" {
    destination = "/home/packer/cleanup-vhd.sh"
    source      = "vhdbuilder/packer/cleanup-vhd.sh"
  }

  provisioner "file" {
    destination = "/home/packer/packer_source.sh"
    source      = "vhdbuilder/packer/packer_source.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_installs.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_install.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_installs_distro.sh"
    source      = "parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_source.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_helpers.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_source_benchmarks.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_source_distro.sh"
    source      = "parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_configs.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_config.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_main.sh"
  }

  provisioner "file" {
    destination = "/home/packer/provision_start.sh"
    source      = "parts/linux/cloud-init/artifacts/cse_start.sh"
  }

  provisioner "file" {
    destination = "/home/packer/containerd_exec_start.conf"
    source      = "parts/linux/cloud-init/artifacts/containerd_exec_start.conf"
  }

  provisioner "file" {
    destination = "/home/packer/kubelet.service"
    source      = "parts/linux/cloud-init/artifacts/kubelet.service"
  }

  provisioner "file" {
    destination = "/home/packer/secure-tls-bootstrap.service"
    source      = "parts/linux/cloud-init/artifacts/secure-tls-bootstrap.service"
  }

  provisioner "file" {
    destination = "/home/packer/reconcile-private-hosts.sh"
    source      = "parts/linux/cloud-init/artifacts/reconcile-private-hosts.sh"
  }

  provisioner "file" {
    destination = "/home/packer/block_wireserver.sh"
    source      = "parts/linux/cloud-init/artifacts/block_wireserver.sh"
  }

  provisioner "file" {
    destination = "/home/packer/ensure_imds_restriction.sh"
    source      = "parts/linux/cloud-init/artifacts/ensure_imds_restriction.sh"
  }

  provisioner "file" {
    destination = "/home/packer/measure-tls-bootstrapping-latency.sh"
    source      = "parts/linux/cloud-init/artifacts/measure-tls-bootstrapping-latency.sh"
  }

  provisioner "file" {
    destination = "/home/packer/measure-tls-bootstrapping-latency.service"
    source      = "parts/linux/cloud-init/artifacts/measure-tls-bootstrapping-latency.service"
  }

  provisioner "file" {
    destination = "/home/packer/validate-kubelet-credentials.sh"
    source      = "parts/linux/cloud-init/artifacts/validate-kubelet-credentials.sh"
  }

  provisioner "file" {
    destination = "/home/packer/cse_redact_cloud_config.py"
    source      = "parts/linux/cloud-init/artifacts/cse_redact_cloud_config.py"
  }

  provisioner "file" {
    destination = "/home/packer/cse_send_logs.py"
    source      = "parts/linux/cloud-init/artifacts/cse_send_logs.py"
  }

  provisioner "file" {
    destination = "/home/packer/init-aks-custom-cloud.sh"
    source      = "parts/linux/cloud-init/artifacts/init-aks-custom-cloud.sh"
  }

  provisioner "file" {
    destination = "/home/packer/reconcile-private-hosts.service"
    source      = "parts/linux/cloud-init/artifacts/reconcile-private-hosts.service"
  }

  provisioner "file" {
    destination = "/home/packer/mig-partition.service"
    source      = "parts/linux/cloud-init/artifacts/mig-partition.service"
  }

  provisioner "file" {
    destination = "/home/packer/bind-mount.sh"
    source      = "parts/linux/cloud-init/artifacts/bind-mount.sh"
  }

  provisioner "file" {
    destination = "/home/packer/bind-mount.service"
    source      = "parts/linux/cloud-init/artifacts/bind-mount.service"
  }

  provisioner "file" {
    destination = "/home/packer/enable-dhcpv6.sh"
    source      = "parts/linux/cloud-init/artifacts/enable-dhcpv6.sh"
  }

  provisioner "file" {
    destination = "/home/packer/dhcpv6.service"
    source      = "parts/linux/cloud-init/artifacts/dhcpv6.service"
  }

  provisioner "file" {
    destination = "/home/packer/sync-container-logs.sh"
    source      = "parts/linux/cloud-init/artifacts/sync-container-logs.sh"
  }

  provisioner "file" {
    destination = "/home/packer/sync-container-logs.service"
    source      = "parts/linux/cloud-init/artifacts/sync-container-logs.service"
  }

  provisioner "file" {
    destination = "/home/packer/crictl.yaml"
    source      = "parts/linux/cloud-init/artifacts/crictl.yaml"
  }

  provisioner "file" {
    destination = "/home/packer/ensure-no-dup.sh"
    source      = "parts/linux/cloud-init/artifacts/ensure-no-dup.sh"
  }

  provisioner "file" {
    destination = "/home/packer/ensure-no-dup.service"
    source      = "parts/linux/cloud-init/artifacts/ensure-no-dup.service"
  }

  provisioner "file" {
    destination = "/home/packer/teleportd.service"
    source      = "parts/linux/cloud-init/artifacts/teleportd.service"
  }

  provisioner "file" {
    destination = "/home/packer/setup-custom-search-domains.sh"
    source      = "parts/linux/cloud-init/artifacts/setup-custom-search-domains.sh"
  }

  provisioner "file" {
    destination = "/home/packer/ubuntu-snapshot-update.sh"
    source      = "parts/linux/cloud-init/artifacts/ubuntu/ubuntu-snapshot-update.sh"
  }

  provisioner "file" {
    destination = "/home/packer/snapshot-update.service"
    source      = "parts/linux/cloud-init/artifacts/ubuntu/snapshot-update.service"
  }

  provisioner "file" {
    destination = "/home/packer/snapshot-update.timer"
    source      = "parts/linux/cloud-init/artifacts/ubuntu/snapshot-update.timer"
  }

  provisioner "file" {
    destination = "/home/packer/cis.sh"
    source      = "parts/linux/cloud-init/artifacts/cis.sh"
  }

  provisioner "file" {
    destination = "/home/packer/tool_installs.sh"
    source      = "vhdbuilder/scripts/linux/tool_installs.sh"
  }

  provisioner "file" {
    destination = "/home/packer/tool_installs_distro.sh"
    source      = "vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh"
  }

  provisioner "file" {
    destination = "/home/packer/pre-install-dependencies.sh"
    source      = "vhdbuilder/packer/pre-install-dependencies.sh"
  }

  provisioner "file" {
    destination = "/home/packer/install-dependencies.sh"
    source      = "vhdbuilder/packer/install-dependencies.sh"
  }

  provisioner "file" {
    destination = "/home/packer/generate-disk-usage.sh"
    source      = "vhdbuilder/packer/generate-disk-usage.sh"
  }

  provisioner "file" {
    destination = "/home/packer/post-install-dependencies.sh"
    source      = "vhdbuilder/packer/post-install-dependencies.sh"
  }

  provisioner "file" {
    destination = "/home/packer/components.json"
    source      = "parts/common/components.json"
  }

  provisioner "file" {
    destination = "/home/packer/manifest.json"
    source      = "parts/linux/cloud-init/artifacts/manifest.json"
  }

  provisioner "file" {
    destination = "/home/packer/sysctl-d-60-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/sysctl-d-60-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/sshd_config"
    source      = "parts/linux/cloud-init/artifacts/sshd_config"
  }

  provisioner "file" {
    destination = "/home/packer/sshd_config_2204_fips"
    source      = "parts/linux/cloud-init/artifacts/sshd_config_2204_fips"
  }

  provisioner "file" {
    destination = "/home/packer/rsyslog-d-60-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/rsyslog-d-60-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/logrotate-d-rsyslog-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/logrotate-d-rsyslog-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/etc-issue"
    source      = "parts/linux/cloud-init/artifacts/etc-issue"
  }

  provisioner "file" {
    destination = "/home/packer/etc-issue.net"
    source      = "parts/linux/cloud-init/artifacts/etc-issue.net"
  }

  provisioner "file" {
    destination = "/home/packer/modprobe-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/modprobe-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/faillock-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/faillock-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/pwquality-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/pwquality-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-su"
    source      = "parts/linux/cloud-init/artifacts/pam-d-su"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-account"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-account"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-auth"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-auth"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-auth-2204"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-auth-2204"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-password"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-password"
  }

  provisioner "file" {
    destination = "/home/packer/profile-d-cis.sh"
    source      = "parts/linux/cloud-init/artifacts/profile-d-cis.sh"
  }

  provisioner "file" {
    destination = "/home/packer/disk_queue.sh"
    source      = "parts/linux/cloud-init/artifacts/disk_queue.sh"
  }

  provisioner "file" {
    destination = "/home/packer/disk_queue.service"
    source      = "parts/linux/cloud-init/artifacts/disk_queue.service"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-memory-telemetry.sh"
    source      = "parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.sh"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-memory-telemetry.service"
    source      = "parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.service"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-memory-telemetry.timer"
    source      = "parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.timer"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-pressure-telemetry.sh"
    source      = "parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-pressure-telemetry.service"
    source      = "parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.service"
  }

  provisioner "file" {
    destination = "/home/packer/cgroup-pressure-telemetry.timer"
    source      = "parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.timer"
  }

  provisioner "file" {
    destination = "/home/packer/update_certs.service"
    source      = "parts/linux/cloud-init/artifacts/update_certs.service"
  }

  provisioner "file" {
    destination = "/home/packer/update_certs.path"
    source      = "parts/linux/cloud-init/artifacts/update_certs.path"
  }

  provisioner "file" {
    destination = "/home/packer/update_certs.sh"
    source      = "parts/linux/cloud-init/artifacts/update_certs.sh"
  }

  provisioner "file" {
    destination = "/home/packer/ci-syslog-watcher.path"
    source      = "parts/linux/cloud-init/artifacts/ci-syslog-watcher.path"
  }

  provisioner "file" {
    destination = "/home/packer/ci-syslog-watcher.service"
    source      = "parts/linux/cloud-init/artifacts/ci-syslog-watcher.service"
  }

  provisioner "file" {
    destination = "/home/packer/ci-syslog-watcher.sh"
    source      = "parts/linux/cloud-init/artifacts/ci-syslog-watcher.sh"
  }

  provisioner "file" {
    destination = "/home/packer/aks-diagnostic.py"
    source      = "parts/linux/cloud-init/artifacts/aks-diagnostic.py"
  }

  provisioner "file" {
    destination = "/home/packer/aks-log-collector.sh"
    source      = "parts/linux/cloud-init/artifacts/aks-log-collector.sh"
  }

  provisioner "file" {
    destination = "/home/packer/aks-log-collector-send.py"
    source      = "parts/linux/cloud-init/artifacts/aks-log-collector-send.py"
  }

  provisioner "file" {
    destination = "/home/packer/aks-log-collector.service"
    source      = "parts/linux/cloud-init/artifacts/aks-log-collector.service"
  }

  provisioner "file" {
    destination = "/home/packer/aks-log-collector.slice"
    source      = "parts/linux/cloud-init/artifacts/aks-log-collector.slice"
  }

  provisioner "file" {
    destination = "/home/packer/aks-log-collector.timer"
    source      = "parts/linux/cloud-init/artifacts/aks-log-collector.timer"
  }

  provisioner "file" {
    destination = "/home/packer/aks-check-network.sh"
    source      = "parts/linux/cloud-init/artifacts/aks-check-network.sh"
  }

  provisioner "file" {
    destination = "/home/packer/aks-check-network.service"
    source      = "parts/linux/cloud-init/artifacts/aks-check-network.service"
  }

  provisioner "file" {
    destination = "/home/packer/logrotate.sh"
    source      = "parts/linux/cloud-init/artifacts/aks-logrotate.sh"
  }

  provisioner "file" {
    destination = "/home/packer/logrotate.service"
    source      = "parts/linux/cloud-init/artifacts/aks-logrotate.service"
  }

  provisioner "file" {
    destination = "/home/packer/logrotate.timer"
    source      = "parts/linux/cloud-init/artifacts/aks-logrotate.timer"
  }

  provisioner "file" {
    destination = "/home/packer/override.conf"
    source      = "parts/linux/cloud-init/artifacts/aks-logrotate-override.conf"
  }

  provisioner "file" {
    destination = "/home/packer/rsyslog"
    source      = "parts/linux/cloud-init/artifacts/aks-rsyslog"
  }

  provisioner "file" {
    destination = "/home/packer/ipv6_nftables"
    source      = "parts/linux/cloud-init/artifacts/ipv6_nftables"
  }

  provisioner "file" {
    destination = "/home/packer/ipv6_nftables.service"
    source      = "parts/linux/cloud-init/artifacts/ipv6_nftables.service"
  }

  provisioner "file" {
    destination = "/home/packer/ipv6_nftables.sh"
    source      = "parts/linux/cloud-init/artifacts/ipv6_nftables.sh"
  }

  provisioner "file" {
    destination = "/home/packer/apt-preferences"
    source      = "parts/linux/cloud-init/artifacts/apt-preferences"
  }

  provisioner "file" {
    destination = "/home/packer/kms.service"
    source      = "parts/linux/cloud-init/artifacts/kms.service"
  }

  provisioner "file" {
    destination = "/home/packer/mig-partition.sh"
    source      = "parts/linux/cloud-init/artifacts/mig-partition.sh"
  }

  provisioner "file" {
    destination = "/home/packer/docker_clear_mount_propagation_flags.conf"
    source      = "parts/linux/cloud-init/artifacts/docker_clear_mount_propagation_flags.conf"
  }

  provisioner "file" {
    destination = "/home/packer/nvidia-modprobe.service"
    source      = "parts/linux/cloud-init/artifacts/nvidia-modprobe.service"
  }

  provisioner "file" {
    destination = "/home/packer/nvidia-docker-daemon.json"
    source      = "parts/linux/cloud-init/artifacts/nvidia-docker-daemon.json"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-auth"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-auth"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-common-password"
    source      = "parts/linux/cloud-init/artifacts/pam-d-common-password"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-su"
    source      = "parts/linux/cloud-init/artifacts/pam-d-su"
  }

  provisioner "file" {
    destination = "/home/packer/NOTICE.txt"
    source      = "vhdbuilder/notice.txt"
  }

  provisioner "file" {
    destination = "/home/packer/localdns.sh"
    source      = "parts/linux/cloud-init/artifacts/localdns.sh"
  }

  provisioner "file" {
    destination = "/home/packer/localdns.service"
    source      = "parts/linux/cloud-init/artifacts/localdns.service"
  }

  provisioner "file" {
    destination = "/home/packer/localdns-delegate.conf"
    source      = "parts/linux/cloud-init/artifacts/localdns-delegate.conf"
  }

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} IMG_SKU=${var.img_sku} UA_TOKEN=${var.ua_token} VHD_BUILD_TIMESTAMP=${local.vhd_build_timestamp} /bin/bash -ux /home/packer/pre-install-dependencies.sh"]
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "sudo reboot"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} IMG_SKU=${var.img_sku} PRIVATE_PACKAGES_URL=${var.private_packages_url} VHD_BUILD_TIMESTAMP=${local.vhd_build_timestamp} /bin/bash -ux /home/packer/install-dependencies.sh"]
  }

  provisioner "file" {
    destination = "bcc-tools-installation.log"
    direction   = "download"
    source      = "/var/log/bcc_installation.log"
  }

  provisioner "shell" {
    inline = ["sudo /bin/bash /home/packer/generate-disk-usage.sh"]
  }

  provisioner "file" {
    destination = "disk-usage.txt"
    direction   = "download"
    source      = "/opt/azure/disk-usage.txt"
  }

  provisioner "shell" {
    inline = ["sudo rm /var/log/bcc_installation.log"]
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "sudo reboot"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} IMG_SKU=${var.img_sku} /bin/bash -ux /home/packer/post-install-dependencies.sh"]
  }

  provisioner "file" {
    destination = "/home/packer/list-images.sh"
    source      = "vhdbuilder/packer/list-images.sh"
  }

  provisioner "shell" {
    inline = ["sudo SKU_NAME=${var.sku_name} IMAGE_VERSION=${var.image_version} CONTAINER_RUNTIME=${var.container_runtime} /bin/bash -ux /home/packer/list-images.sh"]
  }

  provisioner "file" {
    destination = "image-bom.json"
    direction   = "download"
    source      = "/opt/azure/containers/image-bom.json"
  }

  provisioner "file" {
    destination = "release-notes.txt"
    direction   = "download"
    source      = "/opt/azure/vhd-install.complete"
  }

  provisioner "file" {
    destination = "vhd-build-performance-data.json"
    direction   = "download"
    source      = "/opt/azure/vhd-build-performance-data.json"
  }

  provisioner "file" {
    destination = "vhd-grid-compatibility-data.json"
    direction   = "download"
    source      = "/opt/azure/vhd-grid-compatibility-data.json"
  }

  provisioner "shell" {
    inline = ["sudo rm /opt/azure/vhd-build-performance-data.json", "sudo rm /opt/azure/vhd-grid-compatibility-data.json"]
  }

  provisioner "shell" {
    inline = ["sudo /bin/bash -eux /home/packer/cis.sh", "sudo /bin/bash -eux /opt/azure/containers/cleanup-vhd.sh", "sudo /usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync || exit 125"]
  }

  error-cleanup-provisioner "shell" {
    inline = ["sudo /bin/bash /home/packer/generate-disk-usage.sh"]
  }
}
