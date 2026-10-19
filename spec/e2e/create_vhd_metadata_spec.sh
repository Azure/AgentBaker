#!/usr/bin/env shellspec
# shellcheck shell=bash disable=SC2329

Describe 'create-e2e-vhd-metadata.sh'
  setup() {
    TEST_ROOT="$(mktemp -d "${SHELLSPEC_WORKDIR}/vhd-metadata.XXXXXX")"
    METADATA_FILE="${TEST_ROOT}/output/image.json"
    export QUERY_LOG="${TEST_ROOT}/query.log"
    : >"${QUERY_LOG}"
    export OS_SKU=Ubuntu FEATURE_FLAGS=cvm PACKER_BUILD_LOCATION=eastus
    export REPLICATIONS='westus=1, eastus2=2'
    export SKU_NAME=2404gen2CVMcontainerd SIG_IMAGE_NAME=2404gen2CVMcontainerdSpecialized
    export CAPTURED_SIG_VERSION=1.2.3
    export GALLERY_ID=/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.Compute/galleries/test-gallery
    export MANAGED_SIG_ID="${GALLERY_ID}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
    export MOCK_AZ_STATUS=0 MOCK_VERSION
    set_mock_version
    export -f az
  }
  BeforeEach 'setup'

  set_mock_version() {
    MOCK_VERSION="$(jq -nc \
      --arg id "${GALLERY_ID}/images/${SKU_NAME}/versions/${CAPTURED_SIG_VERSION}" '
      {
        id: $id,
        provisioningState: "Succeeded",
        replicationStatus: {
          aggregatedState: "Completed",
          summary: [
            {region: "westus", state: "Completed"},
            {region: "eastus", state: "Completed"},
            {region: "westus", state: "Completed"}
          ]
        }
      }')"
  }

  az() {
    printf '%s\n' "$@" >"${QUERY_LOG}"
    if [ "${MOCK_AZ_STATUS}" -ne 0 ]; then
      echo 'ResourceNotFound: generalized image version is missing' >&2
      return "${MOCK_AZ_STATUS}"
    fi
    printf '%s\n' "${MOCK_VERSION}"
  }

  create_and_check_metadata() {
    bash .pipelines/scripts/create-e2e-vhd-metadata.sh "${METADATA_FILE}" &&
      jq -e --arg generalizedId "${GALLERY_ID}/images/${SKU_NAME}/versions/${CAPTURED_SIG_VERSION}" '
        length == 2 and
        .[env.SIG_IMAGE_NAME] == {
          resourceId: env.MANAGED_SIG_ID,
          version: env.CAPTURED_SIG_VERSION,
          regions: ["eastus", "eastus2", "westus"]
        } and
        .[env.SKU_NAME] == {
          resourceId: $generalizedId,
          version: env.CAPTURED_SIG_VERSION,
          regions: ["eastus", "westus"]
        }
      ' "${METADATA_FILE}" >/dev/null &&
      grep -Fxq "${GALLERY_ID}/images/${SKU_NAME}/versions/${CAPTURED_SIG_VERSION}" "${QUERY_LOG}" &&
      grep -Fxq ReplicationStatus "${QUERY_LOG}"
  }

  Describe 'generalized CVM metadata'
    Parameters
      Ubuntu True 2404gen2CVMcontainerd 2404gen2CVMcontainerdSpecialized
      Ubuntu False 2604minimalgen2CVMcontainerd 2604minimalgen2CVMcontainerdSpecialized
      AzureLinux True V3gen2CVM AzureLinuxV3gen2CVM
      AzureLinux False V3gen2CVM AzureLinuxV3gen2CVM
    End

    It "adds the actual generalized ID for $1 with prefetch=$2 and preserves the captured entry"
      OS_SKU="$1" SKU_NAME="$3" SIG_IMAGE_NAME="$4"
      export RUN_PREFETCH_OPTIMIZATION="$2"
      MANAGED_SIG_ID="${GALLERY_ID}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
      set_mock_version
      When call create_and_check_metadata
      The status should be success
      The output should include "Added image ready for E2Es: ${SKU_NAME}/${CAPTURED_SIG_VERSION}"
      The output should include 'Created E2E VHD metadata:'
    End
  End

  Describe 'captured-image metadata'
    Parameters
      Ubuntu None 2404gen2containerd
      AzureLinux None AzureLinuxV3gen2
      AzureContainerLinux cvm aclgen2CVM
    End

    It "preserves existing behavior for $1/$2 without an Azure query"
      OS_SKU="$1" FEATURE_FLAGS="$2"
      SIG_IMAGE_NAME="$3" SKU_NAME="$3"
      MANAGED_SIG_ID="${GALLERY_ID}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
      When run bash .pipelines/scripts/create-e2e-vhd-metadata.sh "${METADATA_FILE}"
      The status should be success
      The output should include 'Created E2E VHD metadata:'
      The contents of file "${QUERY_LOG}" should equal ''
      The value "$(jq -c --arg id "${MANAGED_SIG_ID}" 'length == 1 and .[env.SIG_IMAGE_NAME].resourceId == $id and .[env.SIG_IMAGE_NAME].regions == ["eastus", "eastus2", "westus"]' "${METADATA_FILE}")" should equal true
    End
  End

  It 'advertises only confirmed replica regions'
    MOCK_VERSION="$(jq -c '.replicationStatus.summary += [{region:"westus2",state:"InProgress"}]' <<<"${MOCK_VERSION}")"
    When call create_and_check_metadata
    The status should be success
    The output should include 'replicated regions: eastus, westus'
    The output should include 'Created E2E VHD metadata:'
  End

  It 'matches the publisher gate case-insensitively'
    OS_SKU=ubuntu FEATURE_FLAGS=CVM
    When call create_and_check_metadata
    The status should be success
    The output should include 'Added image ready for E2Es:'
    The output should include 'Created E2E VHD metadata:'
  End

  Describe 'unready or incorrect generalized versions'
    Parameters
      missing-id 'del(.id)'
      captured-id '.id = env.MANAGED_SIG_ID'
      wrong-version '.id |= sub("1.2.3$"; "1.2.4")'
      updating '.provisioningState = "Updating"'
      replicating '.replicationStatus.aggregatedState = "InProgress"'
      no-regions '.replicationStatus.summary = []'
      incomplete-region '.replicationStatus.summary = [{region:"westus",state:"InProgress"}]'
      invalid-region '.replicationStatus.summary = [{region:null,state:"Completed"}]'
    End

    It "rejects $1 without writing metadata"
      MOCK_VERSION="$(jq -c "$2" <<<"${MOCK_VERSION}")"
      When run bash .pipelines/scripts/create-e2e-vhd-metadata.sh "${METADATA_FILE}"
      The status should be failure
      The output should include 'Reading generalized CVM image'
      The stderr should include 'is not ready with confirmed replica regions'
      The path "${METADATA_FILE}" should not be exist
    End
  End

  It 'fails explicitly when the generalized image lookup fails'
    MOCK_AZ_STATUS=37
    When run bash .pipelines/scripts/create-e2e-vhd-metadata.sh "${METADATA_FILE}"
    The status should equal 37
    The output should include 'Reading generalized CVM image'
    The stderr should include 'Unable to read generalized CVM image'
    The path "${METADATA_FILE}" should not be exist
  End

  It 'rejects a missing captured image ID'
    MANAGED_SIG_ID=''
    When run bash .pipelines/scripts/create-e2e-vhd-metadata.sh "${METADATA_FILE}"
    The status should be failure
    The stderr should include 'Packer did not return a managed SIG image version ID'
    The path "${METADATA_FILE}" should not be exist
  End

  check_pipeline_order() {
    awk '
      /displayName: Run Prefetch Optimization and Convert to VHD blob/ {prefetch = NR}
      /displayName: Copy to Immutable Storage Container/ {copy = NR}
      /displayName: Create E2E VHD metadata/ {create = NR}
      /displayName: Publish E2E VHD metadata/ {publish = NR}
      END {exit !(prefetch > 0 && copy > prefetch && create > copy && publish > create)}
    ' .pipelines/templates/.builder-release-template.yaml
  }

  It 'creates and publishes metadata after both VHD publication paths'
    When call check_pipeline_order
    The status should be success
  End
End
