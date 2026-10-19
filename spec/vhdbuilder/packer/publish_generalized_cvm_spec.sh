#!/usr/bin/env shellspec
# shellcheck shell=bash disable=SC2329

Describe 'publish-generalized-cvm-if-required.sh'
  setup() {
    export FEATURE_FLAGS=cvm OS_SKU=Ubuntu
    export AZURE_RESOURCE_GROUP_NAME=test-rg SIG_GALLERY_NAME=test-gallery
    export SKU_NAME=2404gen2CVMcontainerd CAPTURED_SIG_VERSION=1.2.3
    export PACKER_BUILD_LOCATION=eastus BUILD_BUILDID=123
    export BUILD_DEFINITION_NAME='AKS Linux VHD Build - TME Tenant'
    export BUILD_NUMBER=20261019.1 BRANCH=refs/heads/zb/CVMABE2E IMG_SKU=server
    export TAGS_FILE="${SHELLSPEC_WORKDIR}/cvm-version-tags"
    export MOCK_DEFINITION_ID=/test/image-definition
    export MOCK_FAIL_COMMAND='' MOCK_STATE=Succeeded
    export MOCK_REPLICATION=Completed MOCK_REGION=westus MOCK_REGION_STATE=Completed
    export -f az
  }
  BeforeEach 'setup'

  az() {
    local command="${1} ${2} ${3}"
    if [[ "${command}" = "${MOCK_FAIL_COMMAND}" ]]; then
      echo "Mock Azure operation failed: ${command}" >&2
      return 37
    fi
    case "${command}" in
      'storage account list')
        echo '{"id":"/test/storage-account","location":"eastus"}'
        ;;
      'sig image-definition list')
        echo "${MOCK_DEFINITION_ID}"
        ;;
      'sig image-definition create')
        echo /test/new-image-definition
        ;;
      'resource update --ids')
        ;;
      'sig image-version create')
        while [[ "$#" -gt 0 ]]; do
          if [[ "$1" = --tags ]]; then
            shift
            printf '%s\n' "$@" >"${TAGS_FILE}"
            break
          fi
          shift
        done
        ;;
      'sig image-version wait')
        echo '{}'
        ;;
      'sig image-version show')
        jq -nc --arg state "${MOCK_STATE}" --arg replication "${MOCK_REPLICATION}" \
          --arg region "${MOCK_REGION}" --arg regionState "${MOCK_REGION_STATE}" \
          '{state:$state,replication:{aggregatedState:$replication,summary:[{region:$region,state:$regionState}]}}'
        ;;
      *)
        echo "Unexpected Azure command: ${command}" >&2
        return 99
        ;;
    esac
  }

  publish_and_check_tags() {
    bash ./vhdbuilder/packer/publish-generalized-cvm-if-required.sh \
      https://teststorage.blob.core.windows.net/vhds/test.vhd &&
      jq -Rsc '
        split("\n") | map(select(length > 0) | capture("^(?<key>[^=]+)=(?<value>.*)$")) | from_entries |
        .now as $now |
        . == {
          buildDefinitionName: env.BUILD_DEFINITION_NAME,
          buildNumber: env.BUILD_NUMBER,
          buildId: env.BUILD_BUILDID,
          SkipLinuxAzSecPack: "true",
          os: "Linux",
          now: $now,
          createdBy: "aks-vhd-pipeline",
          image_sku: env.IMG_SKU,
          branch: env.BRANCH
        } and ($now | test("^[0-9]+$")) and
        (($now | tonumber) <= now) and (($now | tonumber) > now - 60)
      ' "${TAGS_FILE}" | grep -qx true
  }

  Describe 'ready images'
    Parameters
      Ubuntu /test/image-definition server
      AzureLinux None azure-linux-3-cvm
    End

    It "publishes $1 with all standard version tags and verified readiness"
      OS_SKU="$1" MOCK_DEFINITION_ID="$2" IMG_SKU="$3"
      When call publish_and_check_tags
      The status should be success
      The output should include 'Image version creation completed'
      The output should include 'replicated successfully to westus'
      The output should include 'image ready in westus'
      The output should not include 'provisioned'
    End
  End

  Describe 'unready images after a zero-exit wait'
    Parameters
      Updating Completed westus Completed
      Succeeded InProgress westus InProgress
      Succeeded Completed eastus Completed
      Succeeded Completed westus InProgress
      Failed Failed westus Failed
    End

    It "rejects state=$1, replication=$2, region=$3, regional state=$4"
      MOCK_STATE="$1" MOCK_REPLICATION="$2" MOCK_REGION="$3" MOCK_REGION_STATE="$4"
      When run bash ./vhdbuilder/packer/publish-generalized-cvm-if-required.sh \
        https://teststorage.blob.core.windows.net/vhds/test.vhd
      The status should be failure
      The output should include 'Verifying image version'
      The output should not include 'image ready in westus'
      The output should not include 'replicated successfully'
      The stderr should include 'is not ready in westus after waiting for replication'
    End
  End

  Describe 'Azure operation failures'
    Parameters
      'storage account list' 'resolving source storage account'
      'sig image-definition list' 'looking up image definition'
      'sig image-definition create' 'creating image definition'
      'resource update --ids' 'enabling image definition updates'
      'sig image-version create' 'creating image version'
      'sig image-version wait' 'waiting for image version replication'
      'sig image-version show' 'verifying image version readiness'
    End

    It "reports failure during $2 without declaring readiness"
      MOCK_DEFINITION_ID=None MOCK_FAIL_COMMAND="$1"
      When run bash ./vhdbuilder/packer/publish-generalized-cvm-if-required.sh \
        https://teststorage.blob.core.windows.net/vhds/test.vhd
      The status should equal 37
      The output should include 'Publishing generalized CVM image'
      The output should not include 'image ready in westus'
      The output should not include 'replicated successfully'
      The stderr should include "failed during $2"
    End
  End

  It 'skips images that do not need generalized CVM publication'
    OS_SKU=AzureContainerLinux
    When run bash ./vhdbuilder/packer/publish-generalized-cvm-if-required.sh \
      https://teststorage.blob.core.windows.net/vhds/test.vhd
    The status should be success
    The output should include 'Generalized CVM publication is not required'
  End
End
