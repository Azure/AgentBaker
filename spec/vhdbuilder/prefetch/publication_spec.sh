#!/bin/bash
# shellcheck disable=SC2329

Describe 'Post-prefetch CVM publication'
  setup_environment() {
    TEST_ROOT="$(mktemp -d "${SHELLSPEC_TMPBASE}/publication.XXXXXX")"
    export TEST_ROOT
    export AZ_MOCK_LOG="${TEST_ROOT}/az.log"
    : > "${AZ_MOCK_LOG}"
    export SUBSCRIPTION_ID=test-subscription LOCATION=westus PACKER_BUILD_LOCATION=westus
    export SIG_GALLERY_RESOURCE_GROUP_NAME=gallery-rg AZURE_RESOURCE_GROUP_NAME=gallery-rg
    export SIG_GALLERY_NAME=gallery SIG_IMAGE_NAME=specialized SKU_NAME=generalized
    export STORAGE_ACCOUNT_BLOB_URL=https://destination.blob.core.windows.net/vhd
    export STORAGE_ACCOUNT_BLOB_URL_STAGING=https://destination.blob.core.windows.net/vhdstaging
    export VHD_NAME=test.vhd IMAGE_BUILDER_IDENTITY_ID=identity BUILD_RUN_NUMBER=run
    export CAPTURED_SIG_VERSION=1.2.3 HYPERV_GENERATION=V2 FEATURE_FLAGS=cvm
    export ENABLE_TRUSTED_LAUNCH=False WORKLOAD_OPTIMIZATION_SCRIPT_URL=https://example.test/optimize.sh
    export OS_SKU=Ubuntu BUILD_BUILDID=123
    export SOURCE_URI=https://sourceaccount.blob.core.windows.net/vhds/original.vhd
    export MOCK_SOURCE_LOCATION=westus MOCK_STORAGE_ACCOUNT_MISSING=false
    export MOCK_FINAL_EXISTS=false MOCK_STAGING_COMPLETE=false MOCK_VERSION_STATE=missing
    export MOCK_STORAGE_FAILURE=false MOCK_GALLERY_FAILURE=false MOCK_REQUIRE_OVERLAP=false
    export MOCK_GALLERY_WAIT_FOR_STORAGE=false MOCK_REPLICATION_PENDING_ONCE=false
    export MOCK_ARTIFACT_MISSING=false
    export MOCK_DEFINITION_EXISTS=true MOCK_VERSION_READ_FAILURE=false
    export MOCK_REPLICATION_FAILURE=false
    export MOCK_COPY_PENDING_ONCE=false MOCK_COPY_STATUS=success

    wait_for_marker() {
      local attempt
      for ((attempt=0; attempt<200; attempt++)); do
        [ -f "${TEST_ROOT}/$1" ] && return 0
        command sleep 0.01
      done
      echo "Timed out waiting for mocked operation: $1" >&2
      return 90
    }

    az() {
      printf '%s\n' "$*" >> "${AZ_MOCK_LOG}"
      case "$1 $2 $3" in
        'sig image-definition list')
          if [ "${MOCK_DEFINITION_EXISTS}" = false ] && [ ! -f "${TEST_ROOT}/definition-created" ]; then
            return 0
          fi
          echo definition
          ;;
        'sig image-definition create')
          touch "${TEST_ROOT}/definition-created"
          echo definition
          ;;
        'resource update --ids')
          ;;
        'sig image-version show')
          if [ "${MOCK_VERSION_READ_FAILURE}" = true ]; then
            echo '(AuthorizationFailed) Access denied' >&2
            return 4
          fi
          if [ "${MOCK_VERSION_STATE}" = succeeded ] || [ -f "${TEST_ROOT}/version-created" ]; then
            if [ "${MOCK_REPLICATION_FAILURE}" = true ]; then
              echo '{"provisioningState":"Succeeded","replicationStatus":{"aggregatedState":"Failed"}}'
            elif [ "${MOCK_REPLICATION_PENDING_ONCE}" = true ] && [ ! -f "${TEST_ROOT}/replication-ready" ]; then
              echo '{"provisioningState":"Succeeded","replicationStatus":{"aggregatedState":"InProgress"}}'
            else
              echo '{"provisioningState":"Succeeded","replicationStatus":{"aggregatedState":"Completed"}}'
            fi
          elif [ "${MOCK_VERSION_STATE}" = failed ]; then
            echo '{"provisioningState":"Failed","replicationStatus":{"aggregatedState":"Failed"}}'
          else
            echo '(ResourceNotFound) Image version not found' >&2
            return 3
          fi
          ;;
        'sig image-version create')
          touch "${TEST_ROOT}/gallery-started"
          if [ "${MOCK_REQUIRE_OVERLAP}" = true ]; then
            wait_for_marker storage-started || return $?
          fi
          if [ "${MOCK_GALLERY_WAIT_FOR_STORAGE}" = true ]; then
            wait_for_marker storage-finished || return $?
          fi
          touch "${TEST_ROOT}/gallery-finished"
          [ "${MOCK_GALLERY_FAILURE}" = true ] && return 7
          touch "${TEST_ROOT}/version-created"
          ;;
        'sig image-version wait')
          if [ "${MOCK_REPLICATION_PENDING_ONCE}" = true ]; then
            sleep 60
          fi
          if [ "${MOCK_REPLICATION_FAILURE}" = true ]; then
            echo 'Gallery replication did not complete' >&2
            return 8
          fi
          touch "${TEST_ROOT}/replication-ready"
          ;;
        'storage account list')
          if [ "${MOCK_STORAGE_ACCOUNT_MISSING}" = true ]; then
            echo null
          else
            printf '{"id":"/subscriptions/test-subscription/resourceGroups/IT_staging/providers/Microsoft.Storage/storageAccounts/sourceaccount","location":"%s"}\n' "${MOCK_SOURCE_LOCATION}"
          fi
          ;;
        'storage blob exists')
          printf '{"exists":%s}\n' "${MOCK_FINAL_EXISTS}"
          ;;
        'storage blob show')
          if [[ "$*" = *'/vhdstaging/'* ]]; then
            if [ "${MOCK_STAGING_COMPLETE}" = true ]; then
              echo '{"name":"test.vhd","metadata":{"prefetched":"true"}}'
            else
              echo '{}'
            fi
          else
            if [ "${MOCK_COPY_PENDING_ONCE}" = true ] && [ ! -f "${TEST_ROOT}/copy-pending-checked" ]; then
              touch "${TEST_ROOT}/copy-pending-checked"
              echo '{"properties":{"copy":{"status":"pending"}}}'
            else
              touch "${TEST_ROOT}/storage-finished"
              printf '{"properties":{"copy":{"status":"%s"}}}\n' "${MOCK_COPY_STATUS}"
            fi
          fi
          ;;
        'storage blob metadata'|'storage blob copy')
          ;;
        'storage blob delete')
          ;;
        'resource show --ids')
          if [ "${MOCK_ARTIFACT_MISSING}" = true ]; then
            echo null
          else
            echo "${SOURCE_URI}"
          fi
          ;;
        'group exists -g')
          echo true
          ;;
        'image builder show')
          echo '{"provisioningState":"Succeeded","lastRunStatus":{"runState":"Succeeded"}}'
          ;;
        'image builder wait')
          ;;
        'group delete -g')
          ;;
        *)
          echo "Unexpected Azure call: $*" >&2
          return 99
          ;;
      esac
    }

    azcopy() {
      printf 'azcopy %s\n' "$*" >> "${AZ_MOCK_LOG}"
      touch "${TEST_ROOT}/storage-started"
      if [ "${MOCK_REQUIRE_OVERLAP}" = true ]; then
        wait_for_marker gallery-started || return $?
      fi
      if [ "${MOCK_STORAGE_FAILURE}" = true ]; then
        touch "${TEST_ROOT}/storage-finished"
        return 6
      fi
    }

    sleep() {
      printf 'sleep %s\n' "$*" >> "${AZ_MOCK_LOG}"
      touch "${TEST_ROOT}/replication-ready"
    }

    export -f az azcopy sleep wait_for_marker
  }
  BeforeEach 'setup_environment'

  run_script() {
    local status=0
    bash "$@" 2>&1 || status=$?
    cat "${AZ_MOCK_LOG}"
    return "${status}"
  }

  Describe 'Direct VHD gallery versions'
    It 'uses the original VHD, account ID and actual account region without a managed image'
      MOCK_SOURCE_LOCATION=northeurope
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include "--os-vhd-uri ${SOURCE_URI}"
      The output should include '--os-vhd-storage-account /subscriptions/test-subscription/resourceGroups/IT_staging/providers/Microsoft.Storage/storageAccounts/sourceaccount'
      The output should include '--location northeurope'
      The output should include '--target-regions westus --tags'
      The output should include 'buildId=123'
      The output should not include '--managed-image'
      The output should not include 'image create'
      The output should not include 'image delete'
    End

    It 'replicates only to West US'
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include '--target-regions westus --tags'
      The output should not include 'eastus'
      The output should not include 'westeurope'
    End

    It 'creates a generalized Gen2 confidential-VM-supported definition when needed'
      MOCK_DEFINITION_EXISTS=false
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include '--os-state Generalized'
      The output should include '--hyper-v-generation V2'
      The output should include 'SecurityType=ConfidentialVMSupported'
    End

    It 'waits for replication even when provisioning has succeeded'
      MOCK_REPLICATION_PENDING_ONCE=true
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include 'sig image-version wait'
      The output should include '--expand ReplicationStatus --custom'
      The output should include "provisioningState=='Succeeded' && replicationStatus.aggregatedState=='Completed'"
      The output should include 'sleep 60'
    End

    It 'fails explicitly if the source storage metadata is unavailable'
      MOCK_STORAGE_ACCOUNT_MISSING=true
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be failure
      The output should include 'Unable to resolve source storage account'
      The output should not include 'image-version create'
    End

    It 'does not inspect or delete failed versions before publishing'
      MOCK_VERSION_STATE=failed
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include 'sig image-version create'
      The output should not include 'sig image-version show'
      The output should not include 'sig image-version delete'
      The output should not include '--deleted'
    End

    It 'does not succeed when gallery replication fails'
      MOCK_REPLICATION_FAILURE=true
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be failure
      The output should include 'Gallery replication did not complete'
      The output should not include 'provisioned successfully'
    End

    It 'skips unsupported OS flavors without storage lookups'
      OS_SKU=AzureContainerLinux
      When call run_script vhdbuilder/packer/publish-generalized-cvm-if-required.sh "${SOURCE_URI}"
      The status should be success
      The output should include 'publication is not required'
      The output should not include 'storage account list'
    End
  End

  Describe 'Parallel delivery and retries'
    It 'reads the original blob concurrently and joins both operations before returning'
      MOCK_REQUIRE_OVERLAP=true MOCK_GALLERY_WAIT_FOR_STORAGE=true MOCK_COPY_PENDING_ONCE=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include "azcopy copy ${SOURCE_URI} ${STORAGE_ACCOUNT_BLOB_URL_STAGING}/${VHD_NAME}"
      The output should include "--os-vhd-uri ${SOURCE_URI}"
      The output should include "storage blob copy start"
      The output should include 'sleep 30s'
      The output should include 'provisioned successfully'
      The output should include 'storage blob delete'
    End

    It 'waits for gallery publication even if the storage copy fails'
      MOCK_STORAGE_FAILURE=true MOCK_GALLERY_WAIT_FOR_STORAGE=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should equal 6
      The output should include 'failed to copy optimized VHD to staging location'
      The output should include 'provisioned successfully'
      The output should not include 'storage blob delete'
    End

    It 'warns without failing and waits for immutable storage delivery if gallery publication fails'
      MOCK_GALLERY_FAILURE=true MOCK_REQUIRE_OVERLAP=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include 'server-side copy to https://destination.blob.core.windows.net/vhd/test.vhd completed successfully'
      The output should include '##vso[task.logissue type=warning]Generalized CVM gallery publication failed (exit 7)'
      The output should include 'storage blob delete'
    End

    It 'blocks cleanup if the server-side immutable copy fails'
      MOCK_COPY_STATUS=failed
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be failure
      The output should include 'finished with unexpected status: failed'
      The output should include 'provisioned successfully'
      The output should not include 'storage blob delete'
    End

    It 'preserves the existing wait for a pending immutable copy on retry'
      MOCK_FINAL_EXISTS=true MOCK_VERSION_STATE=succeeded MOCK_COPY_PENDING_ONCE=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include 'is currently being copied'
      The output should include 'completed successfully'
      The output should not include 'storage blob copy start'
      The output should not include 'resource show'
    End

    It 'preserves the existing early exit when an immutable VHD already exists'
      MOCK_FINAL_EXISTS=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include 'nothing to do'
      The output should not include 'image-version create'
      The output should not include 'azcopy copy'
      The output should not include 'storage blob copy start'
      The output should not include 'image builder show'
    End

    It 'resolves the original blob even when the destination staging copy is reusable'
      MOCK_STAGING_COMPLETE=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include "--os-vhd-uri ${SOURCE_URI}"
      The output should include 'storage blob copy start'
      The output should not include 'azcopy copy'
    End

    It 'allows an already-completed retry after Image Builder resources have been removed'
      MOCK_FINAL_EXISTS=true MOCK_VERSION_STATE=succeeded MOCK_ARTIFACT_MISSING=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be success
      The output should include 'nothing to do'
      The output should not include 'resource show'
    End

    It 'fails before starting either delivery if the original source URI is missing'
      MOCK_ARTIFACT_MISSING=true
      When call run_script vhdbuilder/prefetch/scripts/optimize.sh
      The status should be failure
      The output should include 'unable to determine artifactUri'
      The output should not include 'image-version create'
      The output should not include 'azcopy copy'
    End
  End

  Describe 'Image Builder cleanup'
    It 'retains resource group cleanup without additional prerequisites'
      When call run_script vhdbuilder/prefetch/scripts/cleanup.sh
      The status should be success
      The output should include 'group delete -g image-builder-1.2.3-run --yes --no-wait'
      The output should not include 'image builder delete'
    End
  End
End
