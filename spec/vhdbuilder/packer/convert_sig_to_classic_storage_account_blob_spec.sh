#!/bin/bash
# shellcheck disable=SC2329
# Exercise the export script offline, recording Azure calls and controlling upload outcomes.

Describe 'convert-sig-to-classic-storage-account-blob.sh'
  setup() {
    SCRIPT_PATH="${PWD}/vhdbuilder/packer/convert-sig-to-classic-storage-account-blob.sh"
    TEST_DIR="${SHELLSPEC_WORKDIR}/sig-export.${SHELLSPEC_EXAMPLE_NO}"
    mkdir -p "${TEST_DIR}/parts/linux/cloud-init/artifacts"
    cp parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh \
      "${TEST_DIR}/parts/linux/cloud-init/artifacts/"
    export AZ_CALLS="${TEST_DIR}/az-calls"
    : > "$AZ_CALLS"
    export AZURE_MSI_RESOURCE_STRING=test-identity SUBSCRIPTION_ID=test-subscription
    export RESOURCE_GROUP_NAME=test-rg CAPTURED_SIG_VERSION=1.2.3
    export OS_TYPE=Linux SIG_IMAGE_NAME=test-image PACKER_BUILD_LOCATION=eastus
    export ENABLE_TRUSTED_LAUNCH=false FEATURE_FLAGS=None
    export DESTINATION_STORAGE_CONTAINER=https://storage.invalid/vhd
    export MOCK_AZCOPY_STATUS=0
    unset PERFORMANCE_DATA_FILE SIG_IMAGE_VERSION SIG_GALLERY_NAME
  }
  BeforeEach 'setup'

  az() {
    printf '%s\n' "$*" >> "$AZ_CALLS"
    case "$*" in
      'disk grant-access '*) echo https://disk.invalid/export ;;
      'resource create '*|'disk revoke-access '*|'resource delete '*) ;;
      *) return 1 ;;
    esac
  }

  azcopy() {
    printf 'COPY_DEST=%s\n' "$3"
    return "$MOCK_AZCOPY_STATUS"
  }

  # Child Bash preserves the script's set -e/exit behavior; exported fakes prevent Azure access.
  export_vhd() (
    cd "$TEST_DIR" || return
    export -f az azcopy
    bash "$SCRIPT_PATH"
  )

  # TME stages before promotion; non-TME exports directly to the final container.
  Describe 'configured upload destinations'
    Parameters
      https://storage.invalid/vhdstaging
      https://storage.invalid/vhd
    End

    It "uploads to $1 and completes cleanup"
      DESTINATION_STORAGE_CONTAINER="$1"
      When call export_vhd
      The status should be success
      The output should include "COPY_DEST=$1/1.2.3.vhd"
      The output should include 'Uploaded '
      The contents of file "$AZ_CALLS" should include 'disk revoke-access '
      The contents of file "$AZ_CALLS" should include 'resource delete '
    End
  End

  It 'rejects an empty destination before creating Azure resources'
    DESTINATION_STORAGE_CONTAINER=''
    When call export_vhd
    The status should be failure
    The output should include 'DESTINATION_STORAGE_CONTAINER was not set!'
    The contents of file "$AZ_CALLS" should equal ''
  End

  # Regression: negating AzCopy before capturing $? made failed uploads exit zero.
  It 'preserves the AzCopy failure status and stops publication'
    MOCK_AZCOPY_STATUS=23
    When call export_vhd
    The status should equal 23
    The output should include 'COPY_DEST=https://storage.invalid/vhd/1.2.3.vhd'
    The output should not include 'Uploaded '
    The contents of file "$AZ_CALLS" should not include 'disk revoke-access '
    The contents of file "$AZ_CALLS" should not include 'resource delete '
  End
End
