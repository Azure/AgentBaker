#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}

echo "Using pre-CPS ACL image settings derived from $base_template"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$base_template"
