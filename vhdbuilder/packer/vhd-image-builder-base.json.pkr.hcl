variable "branch" {
  type    = string
  default = "${env("BRANCH")}"
}

variable "build_definition_name" {
  type    = string
  default = "${env("BUILD_DEFINITION_NAME")}"
}

variable "build_id" {
  type    = string
  default = "${env("BUILD_ID")}"
}

variable "build_number" {
  type    = string
  default = "${env("BUILD_NUMBER")}"
}

variable "captured_sig_version" {
  type    = string
  default = "${env("CAPTURED_SIG_VERSION")}"
}

variable "commit" {
  type    = string
  default = "${env("GIT_VERSION")}"
}

variable "container_runtime" {
  type    = string
  default = "${env("CONTAINER_RUNTIME")}"
}

variable "enable_fips" {
  type    = string
  default = "${env("ENABLE_FIPS")}"
}

variable "feature_flags" {
  type    = string
  default = "${env("FEATURE_FLAGS")}"
}

variable "hyperv_generation" {
  type    = string
  default = "${env("HYPERV_GENERATION")}"
}

variable "image_version" {
  type    = string
  default = "${env("IMAGE_VERSION")}"
}

variable "img_offer" {
  type    = string
  default = "${env("IMG_OFFER")}"
}

variable "img_publisher" {
  type    = string
  default = "${env("IMG_PUBLISHER")}"
}

variable "img_sku" {
  type    = string
  default = "${env("IMG_SKU")}"
}

variable "img_version" {
  type    = string
  default = "${env("IMG_VERSION")}"
}

variable "location" {
  type    = string
  default = "${env("AZURE_LOCATION")}"
}

variable "os_version" {
  type    = string
  default = "${env("OS_VERSION")}"
}

variable "private_packages_url" {
  type    = string
  default = "${env("PRIVATE_PACKAGES_URL")}"
}

variable "sgx_install" {
  type    = string
  default = "${env("SGX_INSTALL")}"
}

variable "sig_gallery_name" {
  type    = string
  default = "${env("SIG_GALLERY_NAME")}"
}

variable "sig_image_name" {
  type    = string
  default = "${env("SIG_IMAGE_NAME")}"
}

variable "sig_image_version" {
  type    = string
  default = "${env("SIG_IMAGE_VERSION")}"
}

variable "sku_name" {
  type    = string
  default = "${env("SKU_NAME")}"
}

variable "subnet_name" {
  type    = string
  default = "${env("SUBNET_NAME")}"
}

variable "subscription_id" {
  type    = string
  default = "${env("AZURE_SUBSCRIPTION_ID")}"
}

variable "teleportd_plugin_download_url" {
  type    = string
  default = "${env("TELEPORTD_PLUGIN_DOWNLOAD_URL")}"
}

variable "vm_size" {
  type    = string
  default = "${env("AZURE_VM_SIZE")}"
}

variable "vnet_name" {
  type    = string
  default = "${env("VNET_NAME")}"
}

variable "vnet_rg_name" {
  type    = string
  default = "${env("vnet_resource_group_name")}"
}

source "azure-arm" "vhd" { 
  azure_tags = {
    SkipLinuxAzSecPack  = "true"
    branch              = "${var.branch}"
    buildDefinitionName = "${var.build_definition_name}"
    buildId             = "${var.build_id}"
    buildNumber         = "${var.build_number}"
    createdBy           = "aks-vhd-pipeline"
    image_sku           = "${var.img_sku}"
    now                 = "${var.create_time}"
    os                  = "Linux"
  }
  image_offer                       = "${var.img_offer}"
  image_publisher                   = "${var.img_publisher}"
  image_sku                         = "${var.img_sku}"
  image_version                     = "${var.img_version}"
  location                          = "${var.location}"
  managed_image_name                = "${var.sig_image_name}-${var.captured_sig_version}"
  managed_image_resource_group_name = "${var.resource_group_name}"
  os_disk_size_gb                   = 30
  os_type                           = "Linux"
  polling_duration_timeout          = "1h"
  shared_image_gallery_destination {
    gallery_name            = "${var.sig_gallery_name}"
    image_name              = "${var.sig_image_name}"
    image_version           = "${var.captured_sig_version}"
    replication_regions     = ["${var.location}"]
    resource_group          = "${var.resource_group_name}"
    use_shallow_replication = "true"
  }
  ssh_read_write_timeout              = "5m"
  subscription_id                     = "${var.subscription_id}"
  user_assigned_managed_identities    = "${var.msi_resource_strings}"
  virtual_network_name                = "${var.vnet_name}"
  virtual_network_subnet_name         = "${var.subnet_name}"
  virtual_network_resource_group_name = "${var.vnet_rg_name}"
  vm_size                             = "${var.vm_size}"
}
build {
  sources = ["source.azure-arm.vhd"]

  provisioner "shell" {
    inline = ["sudo mkdir -p /opt/azure/containers", "sudo chown -R $USER /opt/azure/containers"]
  }

  provisioner "shell" {
    inline = ["sudo mkdir -p /opt/scripts", "sudo chown -R $USER /opt/scripts", "sudo mkdir -p /opt/certs", "sudo chown -R $USER /opt/certs"]
  }

  provisioner "file" {
    destination = "/home/packer/nbcparser"
    source      = "nbcparser/bin/nbcparser-amd64"
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
    destination = "/home/packer/reconcile-private-hosts.sh"
    source      = "parts/linux/cloud-init/artifacts/reconcile-private-hosts.sh"
  }

  provisioner "file" {
    destination = "/home/packer/block_wireserver.sh"
    source      = "parts/linux/cloud-init/artifacts/block_wireserver.sh"
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
    destination = "/home/packer/asc-baseline.deb"
    source      = "vhdbuilder/packer/asc-baseline-1.1.0-268.amd64.deb"
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
    destination = "/home/packer/post-install-dependencies.sh"
    source      = "vhdbuilder/packer/post-install-dependencies.sh"
  }

  provisioner "file" {
    destination = "/home/packer/components.json"
    source      = "vhdbuilder/packer/components.json"
  }

  provisioner "file" {
    destination = "/home/packer/manifest.json"
    source      = "parts/linux/cloud-init/artifacts/manifest.json"
  }

  provisioner "file" {
    destination = "/home/packer/kube-proxy-images.json"
    source      = "vhdbuilder/packer/kube-proxy-images.json"
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
    destination = "/home/packer/sshd_config_1604"
    source      = "parts/linux/cloud-init/artifacts/sshd_config_1604"
  }

  provisioner "file" {
    destination = "/home/packer/sshd_config_1804_fips"
    source      = "parts/linux/cloud-init/artifacts/sshd_config_1804_fips"
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
    destination = "/home/packer/pwquality-CIS.conf"
    source      = "parts/linux/cloud-init/artifacts/pwquality-CIS.conf"
  }

  provisioner "file" {
    destination = "/home/packer/pam-d-su"
    source      = "parts/linux/cloud-init/artifacts/pam-d-su"
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
    destination = "/home/packer/nvidia-device-plugin.service"
    source      = "parts/linux/cloud-init/artifacts/nvidia-device-plugin.service"
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

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} SGX_INSTALL=${var.sgx_install} IMG_SKU=${var.img_sku} /bin/bash -ux /home/packer/pre-install-dependencies.sh"]
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "sudo reboot"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} SGX_INSTALL=${var.sgx_install} IMG_SKU=${var.img_sku} PRIVATE_PACKAGES_URL=${var.private_packages_url} /bin/bash -ux /home/packer/install-dependencies.sh"]
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "sudo reboot"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["sudo FEATURE_FLAGS=${var.feature_flags} BUILD_NUMBER=${var.build_number} BUILD_ID=${var.build_id} COMMIT=${var.commit} HYPERV_GENERATION=${var.hyperv_generation} CONTAINER_RUNTIME=${var.container_runtime} TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url} ENABLE_FIPS=${var.enable_fips} SGX_INSTALL=${var.sgx_install} IMG_SKU=${var.img_sku} /bin/bash -ux /home/packer/post-install-dependencies.sh"]
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

  provisioner "shell" {
    inline = ["sudo /bin/bash -eux /home/packer/cis.sh", "sudo /bin/bash -eux /opt/azure/containers/cleanup-vhd.sh", "sudo /usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync || exit 125"]
  }
}
