#!/bin/bash
# ShellSpec Parameters blocks contain data, not shell commands.
# shellcheck disable=SC2286,SC2288

Describe 'L1VH Kata preview build'
  Include ./vhdbuilder/packer/l1vh-kata-preview.sh

  setup() {
    export ENABLE_L1VH=True MODE=linuxVhdMode OS_SKU=AzureLinux OS_VERSION=V3kata
    export ARCHITECTURE=X86_64 HYPERV_GENERATION=V2 FEATURE_FLAGS=kata
    export ENABLE_FIPS=false ENABLE_TRUSTED_LAUNCH=False TRUSTED_LAUNCH_SUPPORTED=False
    export SKU_NAME=V3katagen2l1vhpreview SIG_IMAGE_NAME=AzureLinuxV3katagen2l1vhpreview
    export L1VH_SOURCE_IMAGE_VERSION_ID=/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/base-rg/providers/Microsoft.Compute/galleries/base/images/azl3/versions/1.2.3
    export L1VH_SCHEDULER_TYPE=GuestManaged
    export GALLERY_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
    export AZURE_RESOURCE_GROUP_NAME=build-rg SIG_GALLERY_NAME=buildgallery AZURE_LOCATION=eastus
    mock_definition=$(jq -nc --argjson features "$(l1vh_image_features)" '{properties: {
      osType: "Linux", osState: "Generalized", hyperVGeneration: "V2", architecture: "x64",
      provisioningState: "Succeeded", features: $features
    }}')
    mock_definitions='[]'
    list_status=0 put_status=0 get_status=0
    put_body="${SHELLSPEC_TMPBASE}/l1vh-put.json"
    calls="${SHELLSPEC_TMPBASE}/l1vh-calls"
    : > "$calls"
    rm -f "$put_body"
  }
  BeforeEach setup

  # Capture real JSON and command arguments, rather than mocking jq.
  az() {
    printf '%s\n' "$*" >> "$calls"
    case "$1 $2 $3" in
      'sig image-definition list') printf '%s\n' "$mock_definitions"; return "$list_status" ;;
      'rest --method put')
        while [ "$#" -gt 0 ]; do
          if [ "$1" = --body ]; then printf '%s\n' "$2" > "$put_body"; break; fi
          shift
        done
        return "$put_status"
        ;;
      'rest --method get') printf '%s\n' "$mock_definition"; return "$get_status" ;;
      *) echo "Unexpected Azure call: $*" >&2; return 1 ;;
    esac
  }
  sleep() { :; }

  It 'leaves ordinary builds independent of preview inputs'
    ENABLE_L1VH=False
    unset L1VH_SOURCE_IMAGE_VERSION_ID L1VH_SCHEDULER_TYPE
    When call validate_l1vh_kata_preview
    The status should be success
    The output should be blank
  End

  It 'accepts a pinned private gallery version and an explicitly selected scheduler'
    When call validate_l1vh_kata_preview
    The status should be success
  End

  Describe 'scheduler validation'
  Parameters
    ''
    Unspecified
    RootOnCore
  End
  It 'rejects missing or unsupported scheduler values'
    L1VH_SCHEDULER_TYPE="$1"
    When call validate_l1vh_kata_preview
    The status should be failure
    The stderr should include 'Set L1VH_SCHEDULER_TYPE'
  End
  End

  Describe 'source validation'
  Parameters
    ''
    /subscriptions/00000000/resourceGroups/rg/providers/Microsoft.Compute/galleries/g/images/i/versions/latest
    https://example.invalid/base.vhd
  End
  It 'rejects absent, unpinned or non-gallery source IDs'
    L1VH_SOURCE_IMAGE_VERSION_ID="$1"
    When call validate_l1vh_kata_preview
    The status should be failure
    The stderr should include 'pinned numeric version'
  End
  End

  Describe 'build variant validation'
  Parameters
    ARCHITECTURE ARM64
    HYPERV_GENERATION V1
    OS_SKU Ubuntu
    OS_VERSION V2kata
    FEATURE_FLAGS kata,cvm
    ENABLE_FIPS true
    ENABLE_TRUSTED_LAUNCH True
    TRUSTED_LAUNCH_SUPPORTED True
  End
  It 'rejects incompatible build variants'
    export "$1=$2"
    When call validate_l1vh_kata_preview
    The status should be failure
    The stderr should include 'requires the non-FIPS, non-TL'
  End
  End

  Describe 'image family isolation'
  Parameters
    SKU_NAME V3katagen2
    SIG_IMAGE_NAME AzureLinuxV3katagen2
  End
  It 'rejects the standard Kata image family'
    export "$1=$2"
    When call validate_l1vh_kata_preview
    The status should be failure
    The stderr should include 'dedicated'
  End
  End

  validate_rendered_template() {
    local base=vhdbuilder/packer/vhd-image-builder-mariner.json
    render_l1vh_packer_template "$base" | jq -e --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" --slurpfile base "$base" '
      .builders[0] as $b |
      $b.shared_image_gallery == {id: $source} and
      ($b | has("image_publisher") or has("image_offer") or has("image_sku") or has("image_version") | not) and
      $b.shared_image_gallery_destination == $base[0].builders[0].shared_image_gallery_destination and
      .provisioners == $base[0].provisioners and .variables == $base[0].variables
    ' >/dev/null
  }

  It 'replaces Marketplace source fields while preserving provisioning, variables and destination'
    When call validate_rendered_template
    The status should be success
  End

  validate_created_definition() {
    ensure_l1vh_image_definition || return 1
    jq -e '.location == "eastus" and
      .properties.identifier == {publisher: "microsoft-aks", offer: "buildgallery", sku: "AzureLinuxV3katagen2l1vhpreview"}' "$put_body" >/dev/null &&
      validate_l1vh_image_definition "$(<"$put_body")"
  }

  It 'creates a separate generalized Gen2 definition with string-valued features and reads it back'
    When call validate_created_definition
    The status should be success
    The output should include 'Validated L1VH preview'
    The contents of file "$calls" should include 'api-version=2025-12-03'
  End

  It 'reuses a matching definition without a PUT'
    mock_definitions='[{"name":"AzureLinuxV3katagen2l1vhpreview","id":"existing"}]'
    When call ensure_l1vh_image_definition
    The status should be success
    The output should include 'Validated L1VH preview'
    The file "$put_body" should not be exist
  End

  Describe 'existing definition validation'
  Parameters
    '.properties.features |= map(select(.name != "VirtualizationType"))'
    '(.properties.features[] | select(.name == "DirectVirtualizationSchedulerType").value) = "AzureManaged"'
    '(.properties.features[] | select(.name == "VirtualizationType")) += {startsAtVersion:"9.0.0"}'
    '.properties.osState = "Specialized"'
    '.properties.features += [{name:"SecurityType",value:"TrustedLaunch"}]'
  End
  It 'rejects incompatible existing definitions without modifying them'
    mock_definitions='[{"name":"AzureLinuxV3katagen2l1vhpreview","id":"existing"}]'
    mock_definition=$(jq "$1" <<< "$mock_definition")
    When call ensure_l1vh_image_definition
    The status should be failure
    The stderr should include 'incompatible properties/features'
    The file "$put_body" should not be exist
  End
  End

  It 'does not treat a failed list as a missing definition'
    list_status=1
    When call ensure_l1vh_image_definition
    The status should be failure
    The file "$put_body" should not be exist
  End

  It 'propagates a failed PUT'
    put_status=1
    When call ensure_l1vh_image_definition
    The status should be failure
  End

  It 'propagates a failed read-back'
    get_status=1
    When call ensure_l1vh_image_definition
    The status should be failure
  End

  It 'fails on terminal provisioning failure'
    mock_definition=$(jq '.properties.provisioningState="Failed"' <<< "$mock_definition")
    When call ensure_l1vh_image_definition
    The status should be failure
    The stderr should include 'provisioning failed: Failed'
  End

  It 'bounds the wait for definition provisioning'
    mock_definition=$(jq '.properties.provisioningState="Creating"' <<< "$mock_definition")
    When call ensure_l1vh_image_definition
    The status should be failure
    The stderr should include 'Timed out'
  End

  validate_publishing_metadata() {
    add_l1vh_publishing_info '{"sku_name":"V3katagen2l1vhpreview","vhd_url":"https://example.invalid/result.vhd","publisher_base_image_version":"latest","publisher_base_image_sku":"marketplace"}' |
      jq -e --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" --argjson features "$(l1vh_image_features)" '
        .source_image_version_id == $source and .gallery_image_features == $features and
        .sku_name == "V3katagen2l1vhpreview" and .vhd_url == "https://example.invalid/result.vhd" and
        (has("publisher_base_image_version") or has("publisher_base_image_sku") | not)
      ' >/dev/null
  }

  It 'carries exact source provenance and final-definition features without claiming Marketplace ancestry'
    When call validate_publishing_metadata
    The status should be success
  End

  run_publishing_script() {
    local script="$PWD/vhdbuilder/packer/generate-vhd-publishing-info.sh"
    local output_dir="${SHELLSPEC_TMPBASE}/l1vh-publishing"
    mkdir -p "$output_dir"
    (
      cd "$output_dir" || exit 1
      # The preview script should never need an az vm image list call.
      export -f az
      export calls
      export STORAGE_ACCT_BLOB_URL=https://example.invalid/images VHD_NAME=preview.vhd
      export OS_NAME=Linux OFFER_NAME=AzureLinux IMAGE_VERSION=202609.28.0 SECURITY_TYPE_FEATURE=Standard
      export SUBSCRIPTION_ID="$GALLERY_SUBSCRIPTION_ID" RESOURCE_GROUP_NAME="$AZURE_RESOURCE_GROUP_NAME"
      export CAPTURED_SIG_VERSION=1.2.4 IMG_SKU=azure-linux-3-gen2
      bash "$script" > build.log 2>&1 || { cat build.log >&2; exit 1; }
      jq -e --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" '
        .sku_name == "V3katagen2l1vhpreview" and .source_image_version_id == $source and
        .image_version == "202609.28.0" and
        (.captured_sig_resource_id | endswith("/images/AzureLinuxV3katagen2l1vhpreview/versions/1.2.4")) and
        (.gallery_image_features | length == 3) and (has("publisher_base_image_version") | not)
      ' vhd-publishing-info.json >/dev/null
    )
  }

  It 'runs the publishing entrypoint without a Marketplace query and retains preview image identity'
    When call run_publishing_script
    The status should be success
    The contents of file "$calls" should be blank
  End

  run_build_script() {
    export captured_template="${SHELLSPEC_TMPBASE}/l1vh-captured-template.json"
    packer() {
      [ "$1" = build ] && [ "$2" = -timestamp-ui ] &&
        [ "$3" = -var-file=vhdbuilder/packer/settings.json ] || return 1
      cp "$4" "$captured_template"
    }
    export -f packer
    bash vhdbuilder/packer/build-l1vh-kata-preview.sh || return 1
    jq -e --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" '
      .builders[0].shared_image_gallery.id == $source and
      (.builders[0] | has("image_version") | not)
    ' "$captured_template" >/dev/null
  }

  It 'passes the generated gallery-source JSON to Packer through the build entrypoint'
    When call run_build_script
    The status should be success
    The output should include 'Building L1VH Kata preview from'
  End
End
