locals {
  // Managed Image settings - empty for ARM64 builds
  managed_image_resource_group_name  = lower("${var.architecture}") == "arm64" ? "" : "${var.resource_group_name}"
  managed_image_name                 = lower("${var.architecture}") == "arm64" ? "" : "${var.sig_image_name}-${var.captured_sig_version}"

  // Confidential VM settings, if enabled via feature flags
  secure_boot_enabled = can(regex("cvm", lower(var.feature_flags))) ? true : false
  vtpm_enabled = can(regex("cvm", lower(var.feature_flags))) ? true : false
  security_type = can(regex("cvm", lower(var.feature_flags))) ? "ConfidentialVM" : ""
  security_encryption_type = can(regex("cvm", lower(var.feature_flags))) ? "VMGuestStateOnly" : ""
  specialized_image = can(regex("cvm", lower(var.feature_flags))) ? true : false
  cvm_encryption_type = can(regex("cvm", lower(var.feature_flags))) ? "EncryptedVMGuestStateOnlyWithPmk" : ""

  // File uploads for build process
  custom_data_file = lower(var.os_version) == "flatcar" ? "./vhdbuilder/packer/flatcar-customdata.json" : ""
  aks_node_controller = "${var.architecture}" == "X86_64" ? "aks-node-controller/bin/aks-node-controller-linux-amd64" : "aks-node-controller/bin/aks-node-controller-linux-arm64"
  common_file_upload = jsondecode(file(var.common_file_upload)).files
  ubuntu_file_upload = jsondecode(file(var.ubuntu_file_upload)).files
  azlinux_file_upload = jsondecode(file(var.azlinux_file_upload)).files
  flatcar_file_upload = jsondecode(file(var.flatcar_file_upload)).files

  // File downloads for artifact creation
  midway_file_downloads = jsondecode(file(var.file_downloads)).midway
  post_build_file_downloads = jsondecode(file(var.file_downloads)).post-build
}

// Variables for resolving locals

variable "architecture" {
  type    = string
  default = "${env("ARCHITECTURE")}"
}

variable "feature_flags" {
  type    = string
  default = "${env("FEATURE_FLAGS")}"
}



// Provisioner files

variable "common_file_upload" {
  type    = string
  default = "vhdbuilder/packer/buildconfig/dynamic-provisioners/common_file_upload_for_packer_vm.json"
}

variable "ubuntu_file_upload" {
  type    = string
  default = "vhdbuilder/packer/buildconfig/dynamic-provisioners/ubuntu_file_upload.json"
}

variable "azlinux_file_upload" {
  type    = string
  default = "vhdbuilder/packer/buildconfig/dynamic-provisioners/azlinux_file_upload.json"
}

variable "flatcar_file_upload" {
  type    = string
  default = "vhdbuilder/packer/buildconfig/dynamic-provisioners/flatcar_file_upload.json"
}

variable "post_build_file_downloads" {
  type    = string
  default = "vhdbuilder/packer/buildconfig/dynamic-provisioners/post-build-downloads.json"
}





// Base Marketplace Image Variables

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



// Azure Infrastructure / Resources Variables

variable "subscription_id" {
  type    = string
  default = "${env("AZURE_SUBSCRIPTION_ID")}"
}

variable "msi_resource_strings" {
  type    = string
  default = "${env("msi_resource_strings")}"
}

variable "use_azure_cli_auth" {
  type    = bool
  default = true
}

variable "location" {
  type    = string
  default = "${env("PACKER_BUILD_LOCATION")}"
}

variable "vm_size" {
  type    = string
  default = "${env("AZURE_VM_SIZE")}"
}



// Packer Virtual Network Variables

variable "vnet_resource_group_name" {
  type    = string
  default = "${env("VNET_RESOURCE_GROUP_NAME")}"
}

variable "vnet_name" {
  type    = string
  default = "${env("VNET_NAME")}"
}

variable "subnet_name" {
  type    = string
  default = "${env("SUBNET_NAME")}"
}



// Azure Compute Gallery Variables

variable "sig_gallery_name" {
  type    = string
  default = "${env("SIG_GALLERY_NAME")}"
}

variable "sig_image_name" {
  type    = string
  default = "${env("SIG_IMAGE_NAME")}"
}

variable "captured_sig_version" {
  type    = string
  default = "${env("$${CAPTURED_SIG_VERSION")}"
}



// Tag variables

variable "SkipLinuxAzSecPack" {
  type    = string
  default = true
}

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



// General Variables - Typically used in provisioners and scripts

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

variable "hyperv_generation" {
  type    = string
  default = "${env("HYPERV_GENERATION")}"
}

variable "os_version" {
  type    = string
  default = "${env("OS_VERSION")}"
}

variable "private_packages_url" {
  type    = string
  default = "${env("PRIVATE_PACKAGES_URL")}"
}

variable "sku_name" {
  type    = string
  default = "${env("SKU_NAME")}"
}

variable "teleportd_plugin_download_url" {
  type    = string
  default = "${env("TELEPORTD_PLUGIN_DOWNLOAD_URL")}"
}

variable "ua_token" {
  type    = string
  default = "${env("UA_TOKEN")}"
}

variable "image_version" {
  type    = string
  default = "${env("IMAGE_VERSION")}"
}

variable "os_sku" {
  type    = string
  default = "${env("OS_SKU")}"
}

