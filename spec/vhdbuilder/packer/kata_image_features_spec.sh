#!/bin/bash

Describe 'Kata image definition features'
  Include ./vhdbuilder/packer/kata-image-features.sh
  Include ./vhdbuilder/packer/produce-packer-settings-functions.sh

  setup() {
    export OS_SKU=AzureLinux SKU_NAME=V3katagen2
    export GALLERY_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
    export AZURE_RESOURCE_GROUP_NAME=build-rg SIG_GALLERY_NAME=buildgallery SIG_IMAGE_NAME=AzureLinuxV3katagen2
    mock_definition_file="${SHELLSPEC_TMPBASE}/kata-definition.json"
    mock_patch_file="${SHELLSPEC_TMPBASE}/kata-patch.json"
    mock_calls="${SHELLSPEC_TMPBASE}/kata-calls"
    mock_get_status=0 mock_patch_status=0 mock_apply_patch=true
    mock_definition_exists=true
    export mock_calls
    : > "$mock_calls"
    rm -f "$mock_patch_file"
    # Include an unrelated version-scoped feature to detect accidental loss of metadata.
    jq -n '{tags:{owner:"test"}, properties: {
      osType:"Linux", osState:"Generalized", hyperVGeneration:"V2", architecture:"x64",
      identifier:{publisher:"microsoft-aks",offer:"buildgallery",sku:"AzureLinuxV3katagen2"},
      description:"preserve me", provisioningState:"Succeeded",
      features:[{name:"DiskControllerTypes",value:"SCSI,NVMe"},
        {name:"SecurityType",value:"TrustedLaunchSupported",startsAtVersion:"1.0.0"}]
    }}' > "$mock_definition_file"
  }
  BeforeEach setup

  az() {
    printf '%s\n' "$*" >> "$mock_calls"
    case "$1 $2 $3" in
      'sig show --resource-group') printf '{"provisioningState":"Succeeded"}\n' ;;
      'sig image-definition show')
        [ "$mock_definition_exists" = true ] || return 1
        printf '{"id":"existing"}\n'
        ;;
      'sig image-definition create') printf '{}\n' ;;
      'rest --method get')
        cat "$mock_definition_file"
        return "$mock_get_status"
        ;;
      'rest --method patch')
        while [ "$#" -gt 0 ]; do
          if [ "$1" = --body ]; then printf '%s\n' "$2" > "$mock_patch_file"; break; fi
          shift
        done
        if [ "$mock_apply_patch" = true ] && [ "$mock_patch_status" -eq 0 ]; then
          local updated
          updated=$(jq --slurpfile patch "$mock_patch_file" '.properties.features = $patch[0].properties.features' "$mock_definition_file")
          printf '%s\n' "$updated" > "$mock_definition_file"
        fi
        return "$mock_patch_status"
        ;;
      'vm image list') printf '3.0.20260928\n' ;;
      *) echo "Unexpected Azure call: $*" >&2; return 1 ;;
    esac
  }
  sleep() { :; }

  Describe 'scope'
    Parameters
      AzureLinux V3katagen2 success
      AzureLinux V3gen2 failure
      AzureLinux V2katagen2 failure
      AzureLinux V3katagen2fips failure
      AzureLinux V3katagen2TL failure
      CBLMariner V3katagen2 failure
      Ubuntu V3katagen2 failure
    End
    It 'selects only the current Azure Linux 3 Kata release SKU'
      OS_SKU="$1" SKU_NAME="$2"
      When call is_azurelinux3_kata_image
      The status should be "$3"
    End
  End

  It 'does not call Azure for other images'
    SKU_NAME=V3gen2
    When call ensure_kata_image_features
    The status should be success
    The contents of file "$mock_calls" should be blank
  End

  Describe 'existing build setup integration'
    Parameters
      true
      false
    End
    It 'applies the features after either reusing or creating the normal Kata definition'
      mock_definition_exists="$1"
      export MODE=linuxVhdMode ARCHITECTURE=X86_64 FEATURE_FLAGS=kata HYPERV_GENERATION=V2
      export OS_TYPE=Linux AZURE_LOCATION=eastus ENABLE_TRUSTED_LAUNCH=False TRUSTED_LAUNCH_SUPPORTED=False
      When call ensure_sig_vhd_exists
      The status should be success
      The output should include 'Updated Kata image definition AzureLinuxV3katagen2'
      The contents of file "$mock_calls" should include 'rest --method patch'
    End
  End

  update_and_check() {
    ensure_kata_image_features || return 1
    jq -e --argjson required "$(kata_image_features)" '
      .properties.allowUpdateImage == true and .properties.osType == "Linux" and
      .properties.osState == "Generalized" and .properties.hyperVGeneration == "V2" and
      .properties.identifier.sku == "AzureLinuxV3katagen2" and
      .properties.features == ([{name:"DiskControllerTypes",value:"SCSI,NVMe"},
        {name:"SecurityType",value:"TrustedLaunchSupported",startsAtVersion:"1.0.0"}] + $required) and
      (has("tags") | not)
    ' "$mock_patch_file" >/dev/null &&
    jq -e '.tags.owner == "test" and .properties.description == "preserve me"' "$mock_definition_file" >/dev/null
  }

  It 'adds both features using PATCH, preserving existing features and required identity fields'
    When call update_and_check
    The status should be success
    The output should include 'Updated Kata image definition'
    The contents of file "$mock_calls" should include 'api-version=2025-12-03'
  End

  It 'replaces stale virtualization features rather than creating duplicates'
    local_definition=$(jq '.properties.features += [
      {name:"virtualizationtype",value:"Undefined"},
      {name:"DirectVirtualizationSchedulerType",value:"AzureManaged",startsAtVersion:"9.0.0"}
    ]' "$mock_definition_file")
    printf '%s\n' "$local_definition" > "$mock_definition_file"
    When call update_and_check
    The status should be success
    The output should include 'Updated Kata image definition'
  End

  It 'does not PATCH a matching definition even when feature order differs'
    local_definition=$(jq --argjson required "$(kata_image_features)" '.properties.features = ($required + .properties.features)' "$mock_definition_file")
    printf '%s\n' "$local_definition" > "$mock_definition_file"
    When call ensure_kata_image_features
    The status should be success
    The output should include 'already has direct virtualization features'
    The file "$mock_patch_file" should not be exist
  End

  It 'handles a definition without features'
    local_definition=$(jq 'del(.properties.features)' "$mock_definition_file")
    printf '%s\n' "$local_definition" > "$mock_definition_file"
    When call ensure_kata_image_features
    The status should be success
    The output should include 'Updated Kata image definition'
  End

  It 'does not PATCH when the initial GET fails'
    mock_get_status=1
    When call ensure_kata_image_features
    The status should be failure
    The file "$mock_patch_file" should not be exist
  End

  It 'propagates PATCH failure'
    mock_patch_status=1
    When call ensure_kata_image_features
    The status should be failure
  End

  It 'fails when read-back does not contain the requested features'
    mock_apply_patch=false
    When call ensure_kata_image_features
    The status should be failure
    The stderr should include 'did not match after update'
  End

  Describe 'provisioning states'
    Parameters
      Failed 'provisioning failed'
      Updating 'Timed out'
    End
    It 'fails on a terminal error or a bounded provisioning timeout'
      local_definition=$(jq --arg state "$1" '.properties.provisioningState=$state' "$mock_definition_file")
      printf '%s\n' "$local_definition" > "$mock_definition_file"
      When call ensure_kata_image_features
      The status should be failure
      The stderr should include "$2"
    End
  End

  run_publishing_script() {
    local script="$PWD/vhdbuilder/packer/generate-vhd-publishing-info.sh"
    local output_dir="${SHELLSPEC_TMPBASE}/kata-publishing"
    mkdir -p "$output_dir"
    (
      cd "$output_dir" || exit 1
      export -f az
      export STORAGE_ACCT_BLOB_URL=https://example.invalid/images VHD_NAME=kata.vhd
      export OS_NAME=Linux OFFER_NAME=AzureLinux IMAGE_VERSION=202609.28.0 SECURITY_TYPE_FEATURE=Standard
      export SUBSCRIPTION_ID="$GALLERY_SUBSCRIPTION_ID" RESOURCE_GROUP_NAME="$AZURE_RESOURCE_GROUP_NAME"
      export CAPTURED_SIG_VERSION=1.2.4 IMG_PUBLISHER=MicrosoftCBLMariner IMG_OFFER=azure-linux-3
      export IMG_SKU=azure-linux-3-gen2 ARCHITECTURE=X86_64 HYPERV_GENERATION=V2
      bash "$script" > build.log 2>&1 || { cat build.log >&2; exit 1; }
      jq -e --arg sku "$SKU_NAME" '
        .sku_name == $sku and .publisher_base_image_version == "3.0.20260928" and
        .publisher_base_image_sku == "azure-linux-3-gen2" and .image_version == "202609.28.0" and
        (has("source_image_version_id") | not)
      ' vhd-publishing-info.json >/dev/null || exit 1
      if [ "$SKU_NAME" = V3katagen2 ]; then
        jq -e --argjson required "$(kata_image_features)" '.gallery_image_features == $required' vhd-publishing-info.json >/dev/null
      else
        jq -e 'has("gallery_image_features") | not' vhd-publishing-info.json >/dev/null
      fi
    )
  }

  Describe 'publishing metadata'
    Parameters
      V3katagen2
      V3gen2
    End
    It 'retains Marketplace provenance and adds features only for the existing Kata SKU'
      export SKU_NAME="$1"
      When call run_publishing_script
      The status should be success
      The contents of file "$mock_calls" should include 'vm image list'
    End
  End
End
