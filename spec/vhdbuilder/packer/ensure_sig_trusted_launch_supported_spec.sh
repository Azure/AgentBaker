#!/bin/bash
# shellcheck disable=SC2329

Describe 'ensure_sig_trusted_launch_supported'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  setup() {
    TEST_DIR=$(mktemp -d "${SHELLSPEC_WORKDIR}/trusted-launch.XXXXXX")
    AZ_CALLS="${TEST_DIR}/az-calls"
    PUT_BODY="${TEST_DIR}/put-body"
    : > "${AZ_CALLS}"
    AZURE_RESOURCE_GROUP_NAME="test-rg"
    SIG_GALLERY_NAME="test-gallery"
    SIG_IMAGE_NAME="windows-2025-gen2"
    SIG_IMAGE_VERSION="26100.1.261006"
    DEFINITION_ID="/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.Compute/galleries/test-gallery/images/windows-2025-gen2"
    INITIAL_DEFINITION='{"id":"definition-id","name":"windows-2025-gen2","type":"Microsoft.Compute/galleries/images","location":"eastus","tags":{"preserve":"true"},"properties":{"hyperVGeneration":"V2","provisioningState":"Succeeded","features":[{"name":"DiskControllerTypes","value":"SCSI,NVMe"}]}}'
    UPDATED_DEFINITION='{"properties":{"provisioningState":"Succeeded","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.261006"}]}}'
    FAIL_OPERATION=""
  }
  BeforeEach 'setup'

  az() {
    printf '%s\n' "$*" >> "${AZ_CALLS}"
    case "$1 $2 $3" in
      'rest --method get')
        if [ "${FAIL_OPERATION}" = "get" ]; then return 1; fi
        if [ -f "${PUT_BODY}" ]; then
          printf '%s\n' "${UPDATED_DEFINITION}"
        else
          printf '%s\n' "${INITIAL_DEFINITION}"
        fi
        ;;
      'rest --method put')
        if [ "${FAIL_OPERATION}" = "put" ]; then return 1; fi
        shift 3
        while [ "$#" -gt 0 ]; do
          if [ "$1" = "--body" ]; then
            printf '%s\n' "$2" > "${PUT_BODY}"
            break
          fi
          shift
        done
        ;;
      'sig image-definition wait')
        if [ "${FAIL_OPERATION}" = "wait" ]; then return 1; fi
        ;;
      *) return 1 ;;
    esac
  }

  It 'adds versioned capability while preserving the existing definition'
    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be success
    The output should include 'from version 26100.1.261006'
    The contents of file "${AZ_CALLS}" should include '--updated --interval 5 --timeout 120'
    The value "$(jq -cr '[.id, .name, .type, .properties.provisioningState]' "${PUT_BODY}")" should equal '[null,null,null,null]'
    The value "$(jq -cr '[.location, .tags, .properties.hyperVGeneration, .properties.allowUpdateImage]' "${PUT_BODY}")" should equal '["eastus",{"preserve":"true"},"V2",true]'
    The value "$(jq -cr '.properties.features' "${PUT_BODY}")" should equal '[{"name":"DiskControllerTypes","value":"SCSI,NVMe"},{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.261006"}]'
  End

  It 'does not move the boundary of an already-supported definition'
    INITIAL_DEFINITION='{"properties":{"hyperVGeneration":"V2","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.260901"}]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be success
    The output should equal ''
    The contents of file "${AZ_CALLS}" should not include '--method put'
    The file "${PUT_BODY}" should not be exist
  End

  It 'refuses to mark a Gen1 definition as capable'
    INITIAL_DEFINITION='{"properties":{"hyperVGeneration":"V1","features":[]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be failure
    The stderr should include 'must be Gen2'
    The contents of file "${AZ_CALLS}" should not include '--method put'
  End

  Describe 'Azure failures'
    Parameters
      get 'Failed to read image definition'
      put 'Failed to update Trusted Launch support'
      wait 'Timed out waiting for Trusted Launch support'
    End

    It "stops when $1 fails"
      FAIL_OPERATION="$1"

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be failure
      The stderr should include "$2"
      The output should not include 'did not converge'
    End
  End

  It 'rejects a completed update with incorrect capability'
    UPDATED_DEFINITION='{"properties":{"provisioningState":"Succeeded","features":[{"name":"SecurityType","value":"TrustedLaunch"}]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be failure
    The output should include 'Updating image definition'
    The stderr should include 'did not converge to TrustedLaunchSupported'
  End

  Describe 'returned capability boundary'
    Parameters
      missing '{"properties":{"provisioningState":"Succeeded","features":[{"name":"SecurityType","value":"TrustedLaunchSupported"}]}}'
      older '{"properties":{"provisioningState":"Succeeded","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.260901"}]}}'
      newer '{"properties":{"provisioningState":"Succeeded","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.261007"}]}}'
    End

    It "rejects a completed update with a $1 boundary"
      UPDATED_DEFINITION="$2"

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be failure
      The output should include 'Updating image definition'
      The stderr should include 'did not converge to TrustedLaunchSupported'
    End
  End
End
