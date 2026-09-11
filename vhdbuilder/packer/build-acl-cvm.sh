#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
# Packer picks its parser from the file extension, so keep the .json suffix.
cvm_template="${workdir}/vhd-image-builder-acl-cvm.json"

# A TrustedLaunchAndConfidentialVmSupported gallery rejects capture straight from the VM, so publish
# through a managed image. Only the ACL CVM job runs this script, ACL TL builds use the base template
# unchanged and keep publishing directly from the VM.
jq '.builders[0] += {
      "managed_image_name": "{{user `sig_image_name`}}-{{user `captured_sig_version`}}",
      "managed_image_resource_group_name": "{{user `resource_group_name`}}"
    }' "$base_template" > "$cvm_template"

echo "Using pre-CPS ACL image settings derived from $base_template"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$cvm_template"
