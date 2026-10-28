#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
# Packer picks its parser from the file extension, so keep the .json suffix.
cvm_template="${workdir}/vhd-image-builder-acl-cvm.json"

# Boot with the source image's firmware trust and capture its guest state directly.
# ACL remains generalized, unlike the other Linux CVM templates.
jq '.builders[0] += {
      "security_type": "ConfidentialVM",
      "secure_boot_enabled": true,
      "vtpm_enabled": true,
      "security_encryption_type": "VMGuestStateOnly"
    } |
    .builders[0].shared_image_gallery_destination += {
      "use_shallow_replication": false,
      "confidential_vm_image_encryption_type": "EncryptedVMGuestStateOnlyWithPmk"
    }' "$base_template" > "$cvm_template"

echo "Using Confidential VM ACL image settings derived from $base_template"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$cvm_template"
