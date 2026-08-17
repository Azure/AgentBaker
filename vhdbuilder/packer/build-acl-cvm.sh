#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}
cvm_template=$(mktemp --suffix=.json)
trap 'rm -f "$cvm_template"' EXIT

jq '
    .builders[0] += {
        "secure_boot_enabled": true,
        "vtpm_enabled": true,
        "security_type": "ConfidentialVM",
        "security_encryption_type": "VMGuestStateOnly"
    }
    | .builders[0].shared_image_gallery_destination += {
        "specialized": true,
        "confidential_vm_image_encryption_type": "EncryptedVMGuestStateOnlyWithPmk"
    }
' "$base_template" > "$cvm_template"

echo "Using ACL CVM settings derived from $base_template"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$cvm_template"