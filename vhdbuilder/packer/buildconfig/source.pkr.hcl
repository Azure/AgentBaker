source "azure-arm" "nodelifecycle-image-builder-source" {
  image_offer                        = "${var.img_offer}"
  image_publisher                    = "${var.img_publisher}"
  image_sku                          = "${var.img_sku}"
  image_version                      = "${var.img_version}"

  subscription_id                     = "${var.subscription_id}"
  user_assigned_managed_identities    = "${var.msi_resource_strings}"
  use_azure_cli_auth                  = "true"
  location                            = "${var.location}"
  vm_size                             = "${var.vm_size}"

  managed_image_resource_group_name  = "${local.managed_image_resource_group_name}"
  managed_image_name                 = "${local.managed_image_name}"
  managed_image_storage_account_type = "Premium_LRS"

  virtual_network_resource_group_name = "${var.vnet_resource_group_name}"
  virtual_network_name                = "${var.vnet_name}"
  virtual_network_subnet_name         = "${var.subnet_name}"

  secure_boot_enabled = "${local.secure_boot_enabled}"
  vtpm_enabled = "${local.vtpm_enabled}"
  security_type = "${local.security_type}"
  security_encryption_type = "${local.security_encryption_type}"

  os_disk_size_gb                    = 30
  os_type                            = "Linux"
  polling_duration_timeout           = "1h"
  ssh_read_write_timeout              = "5m"

  shared_image_gallery_destination {
    gallery_name        = "${var.sig_gallery_name}"
    image_name          = "${var.sig_image_name}"
    image_version       = "${var.captured_sig_version}"
    replication_regions = ["${var.location}"]
    resource_group      = "${var.resource_group_name}"
    subscription        = "${local.gallery_subscription_id}"
  }

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
}
