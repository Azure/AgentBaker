#!/usr/bin/env shellspec
# shellcheck shell=bash disable=SC2329

Describe 'prefetch publication and cleanup'
  setup() {
    export SUBSCRIPTION_ID=test LOCATION=eastus SIG_GALLERY_RESOURCE_GROUP_NAME=test-rg
    export SIG_GALLERY_NAME=test-gallery SIG_IMAGE_NAME=specialized SKU_NAME=generalized
    export STORAGE_ACCOUNT_BLOB_URL=https://test.blob.core.windows.net/vhd
    export STORAGE_ACCOUNT_BLOB_URL_STAGING=https://test.blob.core.windows.net/staging
    export VHD_NAME=test.vhd IMAGE_BUILDER_IDENTITY_ID=test-identity BUILD_RUN_NUMBER=test
    export CAPTURED_SIG_VERSION=1.2.3 HYPERV_GENERATION=V2 FEATURE_FLAGS=cvm OS_SKU=Ubuntu
    export ENABLE_TRUSTED_LAUNCH=false WORKLOAD_OPTIMIZATION_SCRIPT_URL=https://example.test/optimize.sh
    export MOCK_EXISTS=false MOCK_COPY_STATUS=success MOCK_GALLERY_READY=false
    export MOCK_GALLERY_REGION=eastus MOCK_GALLERY_REGION_STATE=Completed
    export MOCK_STORAGE_STATUS=0 MOCK_GALLERY_STATUS=0 MOCK_ARTIFACT_STATUS=0
    export MOCK_ARTIFACT_URI=https://source.blob.core.windows.net/vhds/test.vhd
    export MOCK_WAIT_STATUS=0
    export CALL_LOG="${SHELLSPEC_WORKDIR}/prefetch-calls"
    : >"${CALL_LOG}"
  }
  BeforeEach 'setup'

  az() {
    case "${1} ${2} ${3}" in
      'storage blob exists')
        printf '{"exists":%s}\n' "${MOCK_EXISTS}"
        ;;
      'storage blob show')
        printf '{"properties":{"copy":{"status":"%s"}}}\n' "${MOCK_COPY_STATUS}"
        ;;
      'sig image-version show')
        echo gallery-check >>"${CALL_LOG}"
        if [ "${MOCK_GALLERY_READY}" = true ]; then
          jq -nc --arg region "${MOCK_GALLERY_REGION}" --arg state "${MOCK_GALLERY_REGION_STATE}" \
            '{provisioningState:"Succeeded",replicationStatus:{aggregatedState:"Completed",summary:[{region:$region,state:$state}]}}'
        else
          echo '{"provisioningState":"Updating","replicationStatus":{"aggregatedState":"InProgress"}}'
        fi
        ;;
      'resource show --ids')
        echo artifact-read >>"${CALL_LOG}"
        printf '%s\n' "${MOCK_ARTIFACT_URI}"
        return "${MOCK_ARTIFACT_STATUS}"
        ;;
      *)
        echo "Unexpected Azure command: $*" >&2
        return 99
        ;;
    esac
  }

  run_optimizer() (
    # Load the real functions without executing main, then replace resource operations.
    # shellcheck disable=SC1090
    source <(sed '$d' vhdbuilder/prefetch/scripts/optimize.sh) 2>/dev/null
    { set +x; } 2>/dev/null
    ensure_image_builder_rg() { echo ensure-builder >>"${CALL_LOG}"; }
    run_image_builder_template() { echo run-builder >>"${CALL_LOG}"; }
    copy_optimized_vhd() {
      echo storage-copy >>"${CALL_LOG}"
      return "${MOCK_STORAGE_STATUS}"
    }
    copy_optimized_vhd_to_packer_gallery() {
      printf 'gallery-publish %s\n' "$1" >>"${CALL_LOG}"
      return "${MOCK_GALLERY_STATUS}"
    }
    wait_for_vhd_copy() {
      echo storage-wait >>"${CALL_LOG}"
      return "${MOCK_WAIT_STATUS}"
    }
    main
  )

  It 'allows cleanup only after both publications succeed'
    When run run_optimizer
    The status should be success
    The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]false'
    The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
    The contents of file "${CALL_LOG}" should include 'storage-copy'
    The contents of file "${CALL_LOG}" should include "gallery-publish ${MOCK_ARTIFACT_URI}"
  End

  It 'keeps gallery failure warning-only while blocking cleanup'
    MOCK_GALLERY_STATUS=37
    When run run_optimizer
    The status should be success
    The output should include 'Generalized CVM gallery publication failed (exit 37)'
    The output should include 'preserving prefetch resources for retry'
    The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]false'
    The output should not include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
  End

  It 'still fails storage delivery and blocks cleanup even if gallery publication succeeds'
    MOCK_STORAGE_STATUS=23
    When run run_optimizer
    The status should equal 23
    The output should not include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
    The contents of file "${CALL_LOG}" should include 'gallery-publish'
  End

  Describe 'gallery recovery with an existing immutable VHD'
    Parameters
      success
      pending
    End

    It "retries gallery publication after storage state $1 without rebuilding or recopying"
      MOCK_EXISTS=true MOCK_COPY_STATUS="$1"
      When run run_optimizer
      The status should be success
      The output should include 'Retrying generalized CVM publication'
      The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
      The contents of file "${CALL_LOG}" should include "gallery-publish ${MOCK_ARTIFACT_URI}"
      The contents of file "${CALL_LOG}" should not include 'storage-copy'
      The contents of file "${CALL_LOG}" should not include 'run-builder'
      The contents of file "${CALL_LOG}" should not include 'ensure-builder'
    End
  End

  It 'allows an already-complete CVM rerun without needing the deleted source'
    LOCATION=northeurope MOCK_GALLERY_REGION=northeurope
    MOCK_EXISTS=true MOCK_GALLERY_READY=true MOCK_ARTIFACT_STATUS=99
    When run run_optimizer
    The status should be success
    The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
    The contents of file "${CALL_LOG}" should equal 'gallery-check'
  End

  Describe 'publication is not required'
    Parameters
      None Ubuntu
      cvm ACL
    End

    It "preserves the existing early return for feature flags $1 and OS $2"
      MOCK_EXISTS=true FEATURE_FLAGS="$1" OS_SKU="$2"
      When run run_optimizer
      The status should be success
      The output should include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
      The contents of file "${CALL_LOG}" should equal ''
    End
  End

  Describe 'gallery replication must be complete in the build location'
    Parameters
      eastus InProgress
      westus Completed
    End

    It "retries publication when region $1 has state $2"
      MOCK_EXISTS=true MOCK_GALLERY_READY=true
      MOCK_GALLERY_REGION="$1" MOCK_GALLERY_REGION_STATE="$2"
      When run run_optimizer
      The status should be success
      The output should include 'Retrying generalized CVM publication'
      The contents of file "${CALL_LOG}" should include "gallery-publish ${MOCK_ARTIFACT_URI}"
    End
  End

  Describe 'unavailable retained output'
    Parameters
      37 https://source.blob.core.windows.net/vhds/test.vhd
      0 null
    End

    It "keeps an already-delivered VHD successful with source status $1 and URI $2 but blocks cleanup"
      MOCK_EXISTS=true MOCK_ARTIFACT_STATUS="$1" MOCK_ARTIFACT_URI="$2"
      When run run_optimizer
      The status should be success
      The output should include 'Unable to read retained Image Builder output'
      The output should not include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
      The contents of file "${CALL_LOG}" should not include 'gallery-publish'
    End
  End

  It 'still fails a fresh build if Image Builder returns an empty URI'
    MOCK_ARTIFACT_URI=''
    When run run_optimizer
    The status should equal 1
    The stderr should include 'unable to determine artifactUri'
    The output should not include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
  End

  It 'does not treat a failed immutable copy as recoverable'
    MOCK_EXISTS=true MOCK_COPY_STATUS=failed
    When run run_optimizer
    The status should equal 1
    The output should include 'unrecoverable copy state'
    The output should not include 'variable=PREFETCH_READY_FOR_CLEANUP]true'
    The contents of file "${CALL_LOG}" should equal ''
  End

  cleanup_requires_both_publications() {
    sed -n '/make -f packer.mk cleanup-prefetch/,/displayName: Cleanup Prefetch Optimization Resources/p' \
      .pipelines/templates/.builder-release-template.yaml |
      grep -Fq "eq(variables.PREFETCH_READY_FOR_CLEANUP, 'true')"
  }

  It 'gates the pipeline cleanup step on successful publication'
    When call cleanup_requires_both_publications
    The status should be success
  End
End
