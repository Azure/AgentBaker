#!/bin/bash
# shellcheck disable=SC2329

Describe 'generate-vhd-publishing-info.sh'
  setup() {
    SCRIPT_PATH="${PWD}/vhdbuilder/packer/generate-vhd-publishing-info.sh"
    TEST_DIR=$(mktemp -d "${SHELLSPEC_WORKDIR}/publishing-info.XXXXXX")
    OUTPUT="${TEST_DIR}/vhd-publishing-info.json"
    export AZ_CALLS="${TEST_DIR}/az-calls"
    : > "$AZ_CALLS"
    export AZ_STATUS=0
    export AZ_VERSIONS=$'2026.09.02\n2026.09.01\n2026.09.02'
    export STORAGE_ACCT_BLOB_URL=https://example.invalid/images VHD_NAME=test.vhd
    export OS_NAME=linux OFFER_NAME=Ubuntu SKU_NAME=2204 HYPERV_GENERATION=V2
    export IMAGE_VERSION=202610.01.0 SECURITY_TYPE_FEATURE=TrustedLaunch ARCHITECTURE=amd64
    export SUBSCRIPTION_ID=test-subscription RESOURCE_GROUP_NAME=test-rg
    export SIG_IMAGE_NAME=captured-image CAPTURED_SIG_VERSION=202610.01.1
    export IMG_PUBLISHER=Canonical IMG_OFFER=ubuntu-offer IMG_SKU=ubuntu-sku
    export IMG_VERSION=2026.09.01
    unset SIG_SOURCE_GALLERY_UNIQUE_NAME SIG_SOURCE_IMAGE_NAME SIG_SOURCE_IMAGE_VERSION
    unset PUBLISHER_BASE_IMAGE_VERSION PUBLISHER_BASE_IMAGE_SKU
    unset BUILDER BASE_IMG BASE_IMG_VERSION
  }
  BeforeEach 'setup'

  az() {
    printf '<%s>\n' "$@" >> "$AZ_CALLS"
    if [ "$AZ_STATUS" -ne 0 ]; then
      echo 'mock Azure CLI lookup failed' >&2
    fi
    printf '%s\n' "$AZ_VERSIONS"
    return "$AZ_STATUS"
  }

  generate_publishing_info() (
    cd "$TEST_DIR" || return
    export -f az
    bash -e "$SCRIPT_PATH" 2> "${TEST_DIR}/stderr"
  )

  read_metadata() {
    jq -cr "$1" "$OUTPUT"
  }

  set_acg_source() {
    export SIG_SOURCE_GALLERY_UNIQUE_NAME=shared-acl-gallery
    export SIG_SOURCE_IMAGE_NAME=acl-3.0-amd64 SIG_SOURCE_IMAGE_VERSION=20260923.1209152.2
    OFFER_NAME=AzureContainerLinux
    SKU_NAME=acl-tl
  }

  Describe 'ACG sources'
    Parameters
      amd64 acl-3.0-amd64 20260923.1209152.2 acl-tl x64
      amd64 acl-3.0-amd64 20260923.1209152.2 acl-cvm x64
      arm64 acl-3.0-arm64 20260923.1209110.1 acl-arm64-tl Arm64
    End

    It "records the pinned $2 source for $4 without a Marketplace lookup"
      set_acg_source
      ARCHITECTURE="$1" SIG_SOURCE_IMAGE_NAME="$2" SIG_SOURCE_IMAGE_VERSION="$3" SKU_NAME="$4"
      unset IMG_PUBLISHER IMG_OFFER IMG_SKU

      When call generate_publishing_info
      The status should be success
      The output should include "ACG base image shared-acl-gallery/$2/$3"
      The contents of file "$AZ_CALLS" should equal ''
      The contents of file "${TEST_DIR}/stderr" should not include 'WARNING:'
      The value "$(read_metadata '.publisher_base_image_version')" should equal "$3"
      The value "$(read_metadata '.publisher_base_image_sku')" should equal "$2"
      The value "$(read_metadata '.image_architecture')" should equal "$5"
      The value "$(read_metadata '.sku_name')" should equal "$4"
      The value "$(read_metadata '.replication_inverse')" should equal 4
      The value "$(read_metadata '.vhd_url')" should equal 'https://example.invalid/images/test.vhd'
      The value "$(read_metadata '.captured_sig_resource_id')" should equal '/subscriptions/test-subscription/resourceGroups/test-rg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/captured-image/versions/202610.01.1'
      The value "$(read_metadata '[.os_name, .offer_name, .hyperv_generation, .image_version, .security_type_feature]')" should equal '["linux","AzureContainerLinux","V2","202610.01.0","TrustedLaunch"]'
      The value "$(read_metadata 'keys | length')" should equal 12
    End
  End

  It 'prefers ACG source metadata over stale Marketplace values'
    set_acg_source

    When call generate_publishing_info
    The status should be success
    The output should include 'ACG base image shared-acl-gallery/acl-3.0-amd64/20260923.1209152.2'
    The contents of file "$AZ_CALLS" should equal ''
    The value "$(read_metadata '.publisher_base_image_version')" should equal '20260923.1209152.2'
    The value "$(read_metadata '.publisher_base_image_sku')" should equal 'acl-3.0-amd64'
  End

  Describe 'incomplete ACG configuration'
    Parameters
      '' acl-3.0-amd64 20260923.1209152.2 SIG_SOURCE_GALLERY_UNIQUE_NAME
      shared-acl-gallery '' 20260923.1209152.2 SIG_SOURCE_IMAGE_NAME
      shared-acl-gallery acl-3.0-amd64 '' SIG_SOURCE_IMAGE_VERSION
      shared-acl-gallery '' '' SIG_SOURCE_IMAGE_NAME
      '' acl-3.0-amd64 '' SIG_SOURCE_GALLERY_UNIQUE_NAME
      '' '' 20260923.1209152.2 SIG_SOURCE_GALLERY_UNIQUE_NAME
    End

    It "warns about missing $4 without blocking publication or using Marketplace metadata"
      OFFER_NAME=AzureContainerLinux
      export SIG_SOURCE_GALLERY_UNIQUE_NAME="$1" SIG_SOURCE_IMAGE_NAME="$2" SIG_SOURCE_IMAGE_VERSION="$3"

      When call generate_publishing_info
      The status should be success
      The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
      The contents of file "${TEST_DIR}/stderr" should include "WARNING: $4 is not set for ACG source metadata; continuing"
      The contents of file "$AZ_CALLS" should equal ''
      The value "$(read_metadata '.publisher_base_image_version')" should equal "$3"
      The value "$(read_metadata '.publisher_base_image_sku')" should equal "$2"
      The value "$(read_metadata 'keys | length')" should equal 12
    End
  End

  It 'warns and still publishes when all ACL source metadata is absent'
    OFFER_NAME=AzureContainerLinux
    unset IMG_PUBLISHER IMG_OFFER IMG_SKU

    When call generate_publishing_info
    The status should be success
    The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
    The contents of file "$AZ_CALLS" should equal ''
    The contents of file "${TEST_DIR}/stderr" should include 'WARNING: SIG_SOURCE_GALLERY_UNIQUE_NAME is not set'
    The contents of file "${TEST_DIR}/stderr" should include 'WARNING: SIG_SOURCE_IMAGE_NAME is not set'
    The contents of file "${TEST_DIR}/stderr" should include 'WARNING: SIG_SOURCE_IMAGE_VERSION is not set'
    The value "$(read_metadata '.publisher_base_image_version')" should equal ''
    The value "$(read_metadata '.publisher_base_image_sku')" should equal ''
    The value "$(read_metadata 'keys | length')" should equal 12
  End

  Describe 'non-ACL source selection'
    Parameters
      Ubuntu
      AzureLinux
      AzureLinuxOSGuard
    End

    It "does not select ACG metadata for $1 because of a leftover gallery variable"
      OFFER_NAME="$1"
      export SIG_SOURCE_GALLERY_UNIQUE_NAME=stale-gallery

      When call generate_publishing_info
      The status should be success
      The output should include 'Latest Canonical base image version'
      The contents of file "$AZ_CALLS" should include '<vm>'
      The contents of file "${TEST_DIR}/stderr" should not include 'WARNING:'
      The value "$(read_metadata '.publisher_base_image_version')" should equal '2026.09.02'
      The value "$(read_metadata '.publisher_base_image_sku')" should equal 'ubuntu-sku'
    End
  End

  It 'preserves Marketplace version selection and Linux metadata'
    When call generate_publishing_info
    The status should be success
    The output should include 'Latest Canonical base image version for offer ubuntu-offer and sku ubuntu-sku is 2026.09.02'
    The value "$(read_metadata '.publisher_base_image_version')" should equal '2026.09.02'
    The value "$(read_metadata '.publisher_base_image_sku')" should equal 'ubuntu-sku'
    The value "$(read_metadata '[.os_name, .offer_name, .sku_name, .hyperv_generation, .image_architecture, .image_version, .security_type_feature, .replication_inverse]')" should equal '["linux","Ubuntu","2204","V2","x64","202610.01.0","TrustedLaunch","1"]'
    The value "$(read_metadata 'keys | length')" should equal 12
    The contents of file "$AZ_CALLS" should equal "$(printf '<%s>\n' vm image list -p Canonical -s ubuntu-sku --query "[?offer=='ubuntu-offer'].version" -o tsv --all)"
  End

  It 'preserves OSGuard publishing when its legacy Marketplace lookup fails'
    export BUILDER=imagecustomizer BASE_IMG=mcr.microsoft.com/azurelinux/3.0/image/osguard BASE_IMG_VERSION=3.0.20260107
    OFFER_NAME=AzureLinuxOSGuard SKU_NAME=OSGuardV3gen2fipsTL IMG_SKU=azure-linux-osguard-3
    unset IMG_PUBLISHER IMG_OFFER
    AZ_STATUS=2 AZ_VERSIONS=''

    When call generate_publishing_info
    The status should be success
    The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
    The contents of file "${TEST_DIR}/stderr" should include 'mock Azure CLI lookup failed'
    The contents of file "$AZ_CALLS" should include '<azure-linux-osguard-3>'
    The value "$(read_metadata '.publisher_base_image_version')" should equal ''
    The value "$(read_metadata '.publisher_base_image_sku')" should equal 'azure-linux-osguard-3'
    The value "$(read_metadata 'keys | length')" should equal 12
  End

  Describe 'missing Marketplace inputs'
    Parameters
      IMG_PUBLISHER
      IMG_OFFER
      IMG_SKU
    End

    It "does not introduce a new failure for missing $1"
      unset "$1"
      AZ_STATUS=2 AZ_VERSIONS=''

      When call generate_publishing_info
      The status should be success
      The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
      The contents of file "${TEST_DIR}/stderr" should include 'mock Azure CLI lookup failed'
      The contents of file "$AZ_CALLS" should include '<vm>'
      The value "$(read_metadata '.publisher_base_image_version')" should equal ''
      The value "$(read_metadata '.publisher_base_image_sku')" should equal "${IMG_SKU:-}"
    End
  End

  It 'preserves legacy version selection when the CLI fails with partial output'
    AZ_STATUS=2

    When call generate_publishing_info
    The status should be success
    The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
    The contents of file "${TEST_DIR}/stderr" should include 'mock Azure CLI lookup failed'
    The value "$(read_metadata '.publisher_base_image_version')" should equal '2026.09.02'
    The value "$(read_metadata '.publisher_base_image_sku')" should equal 'ubuntu-sku'
  End

  It 'preserves publishing when the Marketplace lookup finds no versions'
    AZ_VERSIONS=''

    When call generate_publishing_info
    The status should be success
    The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
    The value "$(read_metadata '.publisher_base_image_version')" should equal ''
    The value "$(read_metadata '.publisher_base_image_sku')" should equal 'ubuntu-sku'
  End

  Describe 'Windows output'
    Parameters
      ''
      stale-gallery
    End

    It 'preserves the existing lookup and Windows JSON output'
      OS_NAME=windows OFFER_NAME=WindowsServer SKU_NAME=windows-2022 SECURITY_TYPE_FEATURE=Standard
      unset IMG_PUBLISHER IMG_OFFER IMG_SKU
      export SIG_SOURCE_GALLERY_UNIQUE_NAME="$1"
      AZ_STATUS=2 AZ_VERSIONS=''

      When call generate_publishing_info
      The status should be success
      The output should include 'COPY ME ---> https://example.invalid/images/test.vhd'
      The contents of file "$AZ_CALLS" should include '<vm>'
      The contents of file "${TEST_DIR}/stderr" should include 'mock Azure CLI lookup failed'
      The value "$(read_metadata '.')" should equal '{"vhd_url":"https://example.invalid/images/test.vhd","os_name":"windows","sku_name":"windows-2022","offer_name":"WindowsServer","hyperv_generation":"V2","image_architecture":"x64","image_version":"202610.01.0","security_type_feature":"Standard","replication_inverse":"2"}'
    End
  End
End
