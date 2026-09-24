#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'AMD-only Packer template preparation'
  Include './vhdbuilder/packer/amd-gpu-build-settings.sh'

  setup_template() {
    TEST_DIR=$(mktemp -d)
    TEMPLATE="${TEST_DIR}/template.json"
    MAPPINGS="${PWD}/vhdbuilder/packer/amd-gpu-packer-files.json"
    cp vhdbuilder/packer/vhd-image-builder-base.json "${TEMPLATE}"
    OS_SKU=Ubuntu OS_VERSION=24.04 ARCHITECTURE=X86_64 HYPERV_GENERATION=V2
    FEATURE_FLAGS=AMD_GPU ENABLE_FIPS=False ENABLE_TRUSTED_LAUNCH=False TRUSTED_LAUNCH_SUPPORTED=False
    SKU_NAME='' SIG_IMAGE_NAME=''
  }
  cleanup_template() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_template
  AfterEach cleanup_template

  verify_upload_order() {
    prepare_amd_gpu_packer_template "${TEMPLATE}" "${MAPPINGS}" || return 1
    jq -e --slurpfile mappings "${MAPPINGS}" --slurpfile original vhdbuilder/packer/vhd-image-builder-base.json '
      [.provisioners | to_entries[] | select(.value.type == "shell") |
        select(any(.value.inline[]?; contains(" /home/packer/install-dependencies.sh"))) | .key][0] as $install |
      .provisioners[$install - 3:$install] == $mappings[0] and
      (.provisioners | map(. as $entry | select(all($mappings[0][]; . != $entry)))) == $original[0].provisioners and
      del(.provisioners) == ($original[0] | del(.provisioners))
    ' "${TEMPLATE}"
  }

  It 'uploads all three AMD files before installation and preserves the original template content'
    When call verify_upload_order
    The status should be success
    The output should equal true
  End

  verify_idempotency() {
    prepare_amd_gpu_packer_template "${TEMPLATE}" "${MAPPINGS}" || return 1
    cp "${TEMPLATE}" "${TEST_DIR}/once.json"
    prepare_amd_gpu_packer_template "${TEMPLATE}" "${MAPPINGS}" || return 1
    cmp "${TEMPLATE}" "${TEST_DIR}/once.json"
  }

  It 'can prepare the worker checkout repeatedly without duplicate uploads'
    When call verify_idempotency
    The status should be success
    The output should be blank
  End

  break_template() {
    case "$1" in
      missing)
        jq '.provisioners |= map(select(.type != "shell" or
          (any(.inline[]?; contains(" /home/packer/install-dependencies.sh")) | not)))' "${TEMPLATE}" ;;
      duplicate)
        jq '.provisioners += [.provisioners[] | select(.type == "shell") |
          select(any(.inline[]?; contains(" /home/packer/install-dependencies.sh")))]' "${TEMPLATE}" ;;
      conflict)
        jq '.provisioners += [{"type":"file", "source":"unexpected.sh", "destination":"/home/packer/amd_gpu.sh"}]' "${TEMPLATE}" ;;
    esac > "${TEST_DIR}/broken.json"
    mv "${TEST_DIR}/broken.json" "${TEMPLATE}"
  }

  reject_without_truncation() {
    local status=0
    cp "${TEMPLATE}" "${TEST_DIR}/before.json"
    prepare_amd_gpu_packer_template "${TEMPLATE}" "${MAPPINGS}" || status=$?
    cmp "${TEMPLATE}" "${TEST_DIR}/before.json" || return 99
    return "${status}"
  }

  Describe 'invalid base templates'
    Parameters
      missing 'expected exactly one install-dependencies.sh provisioner'
      duplicate 'expected exactly one install-dependencies.sh provisioner'
      conflict 'conflicting AMD Packer file destination'
    End

    It 'rejects an invalid template without replacing or truncating it'
      break_template "$1"
      When call reject_without_truncation
      The status should equal 5
      The stderr should include "$2"
      The output should be blank
    End
  End

  It 'rejects duplicate AMD destinations without changing the worker template'
    jq '. + [.[0]]' "${MAPPINGS}" > "${TEST_DIR}/duplicates.json"
    MAPPINGS="${TEST_DIR}/duplicates.json"
    When call reject_without_truncation
    The status should equal 5
    The stderr should include 'invalid AMD Packer file mappings'
    The output should be blank
  End

  It 'rejects preparation for an ordinary CPU image'
    FEATURE_FLAGS=None
    When call reject_without_truncation
    The status should equal 1
    The stderr should include 'AMD_GPU requires Ubuntu 24.04 x86_64 Gen2'
    The output should be blank
  End
End

Describe 'AMD build resource group cleanup'
  Include './vhdbuilder/packer/cleanup-amd-gpu-build.sh'

  setup_cleanup() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    SETTINGS="${TEST_DIR}/settings.json"
    RESOURCE_GROUP=image-builder-recorded BUILD_ID=1234
    GROUP_JSON='{"name":"image-builder-recorded","id":"/subscriptions/subscription-build/resourceGroups/image-builder-recorded","tags":{"buildId":"1234","createdBy":"aks-vhd-pipeline"}}'
    SHOW_STATUS=0
    printf '{"subscription_id":"subscription-build"}\n' > "${SETTINGS}"
    : > "${TRACE}"
  }
  cleanup_test() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_cleanup
  AfterEach cleanup_test

  az() {
    printf '%s\n' "$*" >> "${TRACE}"
    case "$1 $2" in
      'group show') printf '%s\n' "${GROUP_JSON}"; return "${SHOW_STATUS}" ;;
      'group delete') return 0 ;;
      *) return 99 ;;
    esac
  }
  cleanup_recorded_group() {
    cleanup_amd_gpu_build_resource_group "${SETTINGS}" "${RESOURCE_GROUP}" "${BUILD_ID}"
  }

  It 'deletes only the recorded group with matching build ownership in the recorded subscription'
    When call cleanup_recorded_group
    The status should be success
    The output should include 'Deleting verified Packer resource group image-builder-recorded for build 1234'
    The contents of file "${TRACE}" should equal 'group show --name image-builder-recorded --subscription subscription-build --output json
group delete --name image-builder-recorded --subscription subscription-build --yes --only-show-errors'
  End

  Describe 'unverified resource ownership'
    Parameters
      build_id '.tags.buildId = "another-build"'
      missing_tag 'del(.tags.buildId)'
      owner '.tags.createdBy = "another-owner"'
      name '.name = "another-resource-group"'
      subscription '.id = "/subscriptions/another-subscription/resourceGroups/image-builder-recorded"'
    End

    It 'does not delete a group without exact recorded ownership'
      GROUP_JSON=$(jq "$2" <<< "${GROUP_JSON}")
      When call cleanup_recorded_group
      The status should be success
      The output should include 'ownership does not match this build'
      The contents of file "${TRACE}" should include 'group show '
      The contents of file "${TRACE}" should not include 'group delete '
    End
  End

  Describe 'missing pipeline identity'
    Parameters
      RESOURCE_GROUP ''
      RESOURCE_GROUP "\$(PKR_RG_NAME)"
      BUILD_ID ''
      BUILD_ID "\$(Build.BuildId)"
    End

    It 'does not call Azure when the recorded group or build ID is unavailable'
      printf -v "$1" '%s' "$2"
      When call cleanup_recorded_group
      The status should be success
      The output should include 'skipping AMD build cleanup'
      The contents of file "${TRACE}" should be blank
    End
  End

  It 'does not delete anything when the recorded subscription is missing'
    printf '{}\n' > "${SETTINGS}"
    When call cleanup_recorded_group
    The status should be success
    The output should include 'No recorded build subscription'
    The contents of file "${TRACE}" should be blank
  End

  It 'does not delete anything when the group cannot be read'
    SHOW_STATUS=1
    When call cleanup_recorded_group
    The status should be success
    The output should include 'absent or could not be read'
    The contents of file "${TRACE}" should not include 'group delete '
  End
End
