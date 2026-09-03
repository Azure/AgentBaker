#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}
cvm_template=$(mktemp "${TMPDIR:-/tmp}/acl-cvm.XXXXXX.json")
trap 'rm -f "$cvm_template"' EXIT

echo "Using pre-CPS ACL image settings derived from $base_template"
jq '
	.builders[0].secure_boot_enabled = true |
	.builders[0].vtpm_enabled = true |
	.builders[0].security_type = "ConfidentialVM" |
	.builders[0].security_encryption_type = "VMGuestStateOnly" |
	.builders[0].shared_image_gallery_destination.specialized = true |
	.builders[0].shared_image_gallery_destination.confidential_vm_image_encryption_type = "EncryptedVMGuestStateOnlyWithPmk"
' "$base_template" > "$cvm_template"

packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$cvm_template"
