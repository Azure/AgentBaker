// This build block is used for all Linux VHD builds with Packer
build {
  sources = ["source.azure-arm.nodelifecycle-image-builder"]

  provisioner "shell" {
    inline = ["sudo mkdir -p /opt/azure/containers", "sudo mkdir -p /opt/scripts", "sudo mkdir -p /opt/certs"]
  }

  // These files are common to all VHDs, and will be uploaded to the Packer VM regardless of distro
  dynamic "provisioner" {
    for_each = "${local.common_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
    }
  }

  // Ubuntu-specific file uploads
  dynamic "provisioner" {
    for_each = "${local.ubuntu_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "ubuntu"
    }
  }

  // AzureLinux-specific file uploads
  dynamic "provisioner" {
    for_each = "${local.azlinux_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "cblmariner"
    }
  }

  // Flatcar-specific file uploads
  dynamic "provisioner" {
    for_each = "${local.flatcar_file_upload}"
    content {
      type        = "file"
      source      = provisioner.value.source
      destination = provisioner.value.destination
      when        = lower(var.os_sku) == "flatcar"
    }
  }

  // Architecture-specific aks-node-controller upload
  provisioner "file" {
    destination = "${var.aks_node_controller}"
    source      = "/home/packer/aks-node-controller"
  }


  // Build Process begins
  // pre-install-dependencies.sh, install-dependencies.sh, post-install-dependencies.sh, and list-images.sh are run in order, typically with reboots and file downloads in between
  provisioner "shell" {
    inline = ["/bin/bash -ux /home/packer/pre-install-dependencies.sh"]
    environment_vars = [
      "FEATURE_FLAGS=${var.feature_flags}",
      "BUILD_NUMBER=${var.build_number}",
      "BUILD_ID=${var.build_id}",
      "COMMIT=${var.commit}",
      "HYPERV_GENERATION=${var.hyperv_generation}",
      "CONTAINER_RUNTIME=${var.container_runtime}",
      "TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url}",
      "ENABLE_FIPS=${var.enable_fips}",
      "IMG_SKU=${var.img_sku}",
      "UA_TOKEN=${var.ua_token}",
      "VHD_BUILD_TIMESTAMP=${local.vhd_build_timestamp}"
    ]
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "${local.reboot_command}"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["/bin/bash -ux /home/packer/install-dependencies.sh"]
    environment_vars = [
      "FEATURE_FLAGS=${var.feature_flags}",
      "BUILD_NUMBER=${var.build_number}",
      "BUILD_ID=${var.build_id}",
      "COMMIT=${var.commit}",
      "HYPERV_GENERATION=${var.hyperv_generation}",
      "CONTAINER_RUNTIME=${var.container_runtime}",
      "TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url}",
      "ENABLE_FIPS=${var.enable_fips}",
      "IMG_SKU=${var.img_sku}",
      "PRIVATE_PACKAGES_URL=${var.private_packages_url}",
      "VHD_BUILD_TIMESTAMP=${local.vhd_build_timestamp}"
    ]
  }

  provisioner "shell" {
    inline = ["sudo /bin/bash /home/packer/generate-disk-usage.sh"]
  }

  dynamic "provisioner" {
    for_each = "${local.midway_file_downloads}"
    content {
      type        = "file"
      direction   = "download"
      source      = provisioner.value.source
      destination = provisioner.value.destination
    }
  }

  provisioner "shell" {
    expect_disconnect = true
    inline            = "${local.reboot_command}"
    pause_after       = "60s"
    skip_clean        = true
  }

  provisioner "shell" {
    inline = ["/bin/bash -ux /home/packer/post-install-dependencies.sh"]
    environment_vars = [
      "FEATURE_FLAGS=${var.feature_flags}",
      "BUILD_NUMBER=${var.build_number}",
      "BUILD_ID=${var.build_id}",
      "COMMIT=${var.commit}",
      "HYPERV_GENERATION=${var.hyperv_generation}",
      "CONTAINER_RUNTIME=${var.container_runtime}",
      "TELEPORTD_PLUGIN_DOWNLOAD_URL=${var.teleportd_plugin_download_url}",
      "ENABLE_FIPS=${var.enable_fips}",
      "IMG_SKU=${var.img_sku}"
    ]
  }

  provisioner "shell" {
    inline = ["/bin/bash -ux /home/packer/list-images.sh"]
    environment_vars = [
      SKU_NAME=${var.sku_name},
      IMAGE_VERSION=${var.image_version},
      CONTAINER_RUNTIME=${var.container_runtime}
    ]
  }

  dynamic "provisioner" {
    for_each = "${local.post_build_file_downloads}"
    content {
      type        = "file"
      direction   = "download"
      source      = provisioner.value.source
      destination = provisioner.value.destination
    }
  }

  provisioner "shell" {
    inline = ["sudo rm /opt/azure/vhd-build-performance-data.json", "sudo rm /opt/azure/vhd-grid-compatibility-data.json", "sudo rm /var/log/bcc_installation.log"]
  }

  provisioner "shell" {
    inline = ["sudo /bin/bash -eux /home/packer/cis.sh", "sudo /bin/bash -eux /opt/azure/containers/cleanup-vhd.sh", "sudo /usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync || exit 125"]
  }

  error-cleanup-provisioner "shell" {
    inline = ["sudo /bin/bash /home/packer/generate-disk-usage.sh"]
  }
}
