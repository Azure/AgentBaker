#!/bin/bash
# shellcheck disable=SC2329

Describe 'ensure_sig_trusted_launch_supported'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  setup() {
    TEST_DIR="${SHELLSPEC_WORKDIR}/trusted-launch.${SHELLSPEC_EXAMPLE_NO}"
    mkdir -p "${TEST_DIR}"
    AZ_CALLS="${TEST_DIR}/az-calls"
    UPDATE_FEATURES="${TEST_DIR}/update-features"
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
      'resource show --ids')
        if [ "${FAIL_OPERATION}" = "show" ]; then return 1; fi
        if [ -f "${TEST_DIR}/created-definition" ]; then
          cat "${TEST_DIR}/created-definition"
        else
          printf '%s\n' "${INITIAL_DEFINITION}"
        fi
        ;;
      'resource update --ids')
        if [ "${FAIL_OPERATION}" = "update" ]; then return 1; fi
        while [ "$#" -gt 0 ]; do
          case "$1" in
            properties.features=*)
              printf '%s\n' "${1#properties.features=}" > "${UPDATE_FEATURES}"
              break
              ;;
          esac
          shift
        done
        printf '%s\n' "${UPDATED_DEFINITION}"
        ;;
      'sig show --resource-group')
        printf '%s\n' '{"provisioningState":"Succeeded"}'
        ;;
      'sig image-definition show')
        if [ ! -f "${TEST_DIR}/created-definition" ]; then return 1; fi
        printf '{"id":"%s"}\n' "${DEFINITION_ID}"
        ;;
      'sig image-definition create')
        if [ "${FAIL_OPERATION}" = "create" ]; then return 23; fi
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
    The contents of file "${AZ_CALLS}" should include "resource show --ids ${DEFINITION_ID} --api-version 2024-03-03"
    The contents of file "${AZ_CALLS}" should include "resource update --ids ${DEFINITION_ID} --api-version 2024-03-03 --set properties.allowUpdateImage=true properties.features="
    The contents of file "${AZ_CALLS}" should include '--remove properties.provisioningState'
    The contents of file "${AZ_CALLS}" should not include '--no-wait'
    The contents of file "${AZ_CALLS}" should not include 'image-definition wait'
    The value "$(grep -c '^resource show ' "${AZ_CALLS}")" should equal 1
    The value "$(jq -c '.' "${UPDATE_FEATURES}")" should equal '[{"name":"DiskControllerTypes","value":"SCSI,NVMe"},{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.261006"}]'
  End

  It 'creates a new shared Windows definition capable directly without a migration'
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
    The output should include 'Creating image definition windows-2025-gen2'
    The output should not include 'Updating image definition'
    The stderr should equal ''
    The contents of file "${AZ_CALLS}" should include 'DiskControllerTypes=SCSI,NVMe SecurityType=TrustedLaunchSupported'
    The contents of file "${AZ_CALLS}" should not include 'resource update'
    The contents of file "${AZ_CALLS}" should not include 'image-definition wait'
    The value "$(grep -c '^sig image-definition create ' "${AZ_CALLS}")" should equal 1
    The value "$(grep -c '^sig image-definition show ' "${AZ_CALLS}")" should equal 1
    The file "${UPDATE_FEATURES}" should not be exist
    The value "$(jq -c '.properties.features' "${TEST_DIR}/created-definition")" should equal '[{"name":"DiskControllerTypes","value":"SCSI,NVMe"},{"name":"SecurityType","value":"TrustedLaunchSupported"}]'
  End

  It 'propagates a new shared Windows definition creation failure without migration'
    MODE="windowsVhdMode"
    ARCHITECTURE="x64"
    FEATURE_FLAGS=""
    HYPERV_GENERATION="V2"
    OS_TYPE="Windows"
    AZURE_LOCATION="eastus"
    ENABLE_TRUSTED_LAUNCH="False"
    TRUSTED_LAUNCH_SUPPORTED="True"
    FAIL_OPERATION="create"

    When call ensure_sig_vhd_exists
    The status should equal 23
    The output should include 'Creating image definition windows-2025-gen2'
    The contents of file "${AZ_CALLS}" should not include 'resource update'
    The contents of file "${AZ_CALLS}" should not include 'image-definition wait'
    The file "${UPDATE_FEATURES}" should not be exist
  End

  It 'migrates an existing Standard staging definition during build setup'
    MODE="windowsVhdMode"
    ENABLE_TRUSTED_LAUNCH="False"
    TRUSTED_LAUNCH_SUPPORTED="True"
    jq '.properties.features += [{name:"SecurityType",value:"Standard"}]' \
      <<<"${INITIAL_DEFINITION}" > "${TEST_DIR}/created-definition"

    When call ensure_sig_vhd_exists
    The status should be success
    The output should include 'Updating image definition'
    The stderr should equal ''
    The contents of file "${AZ_CALLS}" should not include 'image-definition create'
    The contents of file "${AZ_CALLS}" should include '--set properties.allowUpdateImage=true properties.features='
    The value "$(grep -c '^resource show ' "${AZ_CALLS}")" should equal 1
    The value "$(jq -c '.' "${UPDATE_FEATURES}")" should equal '[{"name":"DiskControllerTypes","value":"SCSI,NVMe"},{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.261006"}]'
  End

  It 'does not move the boundary of an already-supported definition'
    INITIAL_DEFINITION='{"properties":{"hyperVGeneration":"V2","features":[{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":"26100.1.260901"}]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be success
    The output should equal ''
    The contents of file "${AZ_CALLS}" should not include 'resource update'
    The file "${UPDATE_FEATURES}" should not be exist
  End

  Describe 'existing capability without a boundary'
    Parameters
      absent '{"name":"SecurityType","value":"TrustedLaunchSupported"}'
      null '{"name":"SecurityType","value":"TrustedLaunchSupported","startsAtVersion":null}'
    End

    It "accepts an $1 boundary without modifying the staging definition"
      INITIAL_DEFINITION=$(jq --argjson feature "$2" '
        .properties.features += [$feature]
      ' <<<"${INITIAL_DEFINITION}")

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be success
      The output should equal ''
      The stderr should equal ''
      The contents of file "${AZ_CALLS}" should not include 'resource update'
      The contents of file "${AZ_CALLS}" should not include 'image-definition wait'
      The file "${UPDATE_FEATURES}" should not be exist
    End
  End

  It 'reuses an existing supported staging definition without a boundary during build setup'
    MODE="windowsVhdMode"
    ENABLE_TRUSTED_LAUNCH="False"
    TRUSTED_LAUNCH_SUPPORTED="True"
    jq '.properties.features += [{name:"SecurityType",value:"TrustedLaunchSupported"}]' \
      <<<"${INITIAL_DEFINITION}" > "${TEST_DIR}/created-definition"

    When call ensure_sig_vhd_exists
    The status should be success
    The output should include 'Image definition windows-2025-gen2 existing'
    The stderr should equal ''
    The contents of file "${AZ_CALLS}" should not include 'resource update'
    The contents of file "${AZ_CALLS}" should not include 'image-definition wait'
    The contents of file "${AZ_CALLS}" should not include 'image-definition create'
    The file "${UPDATE_FEATURES}" should not be exist
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
      The contents of file "${AZ_CALLS}" should not include 'resource update'
      The file "${UPDATE_FEATURES}" should not be exist
    End
  End

  Describe 'invalid existing capability boundaries'
    Parameters
      empty ''
      malformed 'invalid'
      short '26100.1'
      nonnumeric '26100.1.bad'
      future '26100.1.261007'
      futureminor '26100.10.1'
      outofrange '26100.1.2147483648'
    End

    It "rejects an existing $1 boundary without updating it"
      INITIAL_DEFINITION=$(jq --arg version "$2" '
        .properties.hyperVGeneration = "V2" |
        .properties.features[0].startsAtVersion = $version
      ' <<<"${UPDATED_DEFINITION}")

      When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
      The status should be failure
      The stderr should include 'cannot support build version'
      The output should equal ''
      The contents of file "${AZ_CALLS}" should not include 'resource update'
      The file "${UPDATE_FEATURES}" should not be exist
    End
  End

  It 'refuses to mark a Gen1 definition as capable'
    INITIAL_DEFINITION='{"properties":{"hyperVGeneration":"V1","features":[]}}'

    When call ensure_sig_trusted_launch_supported "${DEFINITION_ID}"
    The status should be failure
    The stderr should include 'must be Gen2'
    The contents of file "${AZ_CALLS}" should not include 'resource update'
  End

  Describe 'Azure failures'
    Parameters
      show 'Failed to read image definition'
      update 'Failed to update Trusted Launch support'
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

  It 'rejects an update that has not succeeded'
    UPDATED_DEFINITION=$(jq '.properties.provisioningState = "Updating"' <<<"${UPDATED_DEFINITION}")

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
