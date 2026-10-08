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
        elif [ -f "${TEST_DIR}/created-definition" ]; then
          cat "${TEST_DIR}/created-definition"
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
      'sig show --resource-group')
        printf '%s\n' '{"provisioningState":"Succeeded"}'
        ;;
      'sig image-definition show')
        if [ ! -f "${TEST_DIR}/created-definition" ]; then return 1; fi
        printf '{"id":"%s"}\n' "${DEFINITION_ID}"
        ;;
      'sig image-definition create')
        local security_type="Standard"
        case "$*" in *SecurityType=TrustedLaunchSupported*) security_type="TrustedLaunchSupported" ;; esac
        jq --arg security_type "${security_type}" '
          .properties.features += [{name: "SecurityType", value: $security_type}]
        ' <<<"${INITIAL_DEFINITION}" > "${TEST_DIR}/created-definition"
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

  It 'creates a new shared Windows definition with a verified capability boundary'
    MODE="windowsVhdMode"
    ARCHITECTURE="x64"
    FEATURE_FLAGS=""
    HYPERV_GENERATION="V2"
    OS_TYPE="Windows"
    AZURE_LOCATION="eastus"
    ENABLE_TRUSTED_LAUNCH="False"
    TRUSTED_LAUNCH_SUPPORTED="True"

    When call ensure_sig_vhd_exists
    The status should be success
    The output should include 'Updating image definition'
    The stderr should equal ''
    The contents of file "${AZ_CALLS}" should include 'SecurityType=Standard'
    The file "${PUT_BODY}" should be exist
    The value "$(jq -r '.properties.features[] | select(.name == "SecurityType").startsAtVersion' "${PUT_BODY}")" should equal '26100.1.261006'
  End

  It 'does not move the boundary of an already-supported definition'
    INITIAL_DEFINITION='{"properties":{"hyperVGeneration":"V2","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.260901"}]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be success
    The output should equal ''
    The contents of file "${AZ_CALLS}" should not include '--method put'
    The file "${PUT_BODY}" should not be exist
  End

  Describe 'valid existing capability boundaries'
    Parameters
      equal '26100.1.261006'
      numeric '26100.1.9'
      minor '26100.0.999999'
    End

    It "preserves an existing $1 boundary at or below the build version"
      INITIAL_DEFINITION=$(jq --arg version "$2" '.properties.hyperVGeneration = "V2" | .properties.features[0].startsAtVersion = $version' <<<"${UPDATED_DEFINITION}")

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be success
      The output should equal ''
      The contents of file "${AZ_CALLS}" should not include '--method put'
      The file "${PUT_BODY}" should not be exist
    End
  End

  Describe 'invalid existing capability boundaries'
    Parameters
      missing ''
      malformed 'invalid'
      short '26100.1'
      nonnumeric '26100.1.bad'
      future '26100.1.261007'
      futureminor '26100.10.1'
    End

    It "rejects an existing $1 boundary without updating it"
      INITIAL_DEFINITION=$(jq --arg version "$2" '
        .properties.hyperVGeneration = "V2" |
        if $version == "" then del(.properties.features[0].startsAtVersion)
        else .properties.features[0].startsAtVersion = $version end
      ' <<<"${UPDATED_DEFINITION}")

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be failure
      The stderr should include 'cannot support build version'
      The output should equal ''
      The contents of file "${AZ_CALLS}" should not include '--method put'
      The file "${PUT_BODY}" should not be exist
    End
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
