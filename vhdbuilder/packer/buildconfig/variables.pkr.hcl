packer {
  required_plugins {
    azure = {
      source  = "github.com/hashicorp/azure"
      version = "~> 1"
    }
  }
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

variable "captured_sig_version" {
  type    = string
  default = "${env("$${CAPTURED_SIG_VERSION")}"
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
  default = "${env("PACKER_BUILD_LOCATION")}"
}

variable "os_version" {
  type    = string
  default = "${env("OS_VERSION")}"
}

variable "private_packages_url" {
  type    = string
  default = "${env("PRIVATE_PACKAGES_URL")}"
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

variable "ua_token" {
  type    = string
  default = "${env("UA_TOKEN")}"
}

variable "vm_size" {
  type    = string
  default = "${env("AZURE_VM_SIZE")}"
}

variable "vnet_name" {
  type    = string
  default = "${env("VNET_NAME")}"
}

variable "vnet_resource_group_name" {
  type    = string
  default = "${env("VNET_RESOURCE_GROUP_NAME")}"
}

locals {
  gallery_subscription_id = "${local.gallery_subscription_id}"
  vhd_build_timestamp     = "${var.VHD_BUILD_TIMESTAMP}"
}
