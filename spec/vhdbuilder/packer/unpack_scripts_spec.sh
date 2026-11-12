#!/bin/bash
# shellcheck disable=SC2329

Describe 'embedded script installation'
  anc() {
    echo unpack
    return "$UNPACK_STATUS"
  }

  source() {
    echo sourced
  }

  run_unpack() {
    local commands
    case "$1" in
      *.json)
        commands=$(jq -r '.provisioners[] | select(.type == "shell") | .inline |
          if type == "array" then .[] elif type == "string" then . else empty end |
          select(contains("unpack-scripts") or contains("/opt/azure/image-build/pre-install-dependencies.sh"))' "$1" |
          sed -E 's|sudo /home/packer/aks-node-controller |anc |; s|^sudo FEATURE_FLAGS=.*pre-install-dependencies.sh$|source|')
        ;;
      *)
        commands=$(awk '
          /aks-node-controller unpack-scripts/ { print }
          /^source / { print; exit }
        ' "$1" | sed -E 's|/[^ ]*/aks-node-controller |anc |')
        ;;
    esac
    set -e
    eval "$commands"
  }

  Parameters
    vhdbuilder/packer/vhd-image-builder-base.json
    vhdbuilder/packer/vhd-image-builder-cvm.json
    vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
    vhdbuilder/packer/vhd-image-builder-arm64-gb.json
    vhdbuilder/packer/vhd-image-builder-mariner.json
    vhdbuilder/packer/vhd-image-builder-mariner-cvm.json
    vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
    vhdbuilder/packer/vhd-image-builder-acl.json
    vhdbuilder/packer/vhd-image-builder-acl-arm64.json
    vhdbuilder/packer/imagecustomizer/azlosguard/scripts/azlosguard-postinstall.sh
  End

  It "unpacks before sourcing helpers in $1"
    UNPACK_STATUS=0
    When run run_unpack "$1"
    The status should be success
    The line 1 of output should equal unpack
    The output should end with sourced
  End

  It "stops before sourcing helpers if unpacking fails in $1"
    UNPACK_STATUS=1
    When run run_unpack "$1"
    The status should be failure
    The output should equal unpack
  End
End
