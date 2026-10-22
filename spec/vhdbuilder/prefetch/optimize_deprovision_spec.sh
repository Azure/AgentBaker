#!/bin/bash

# shellcheck disable=SC2329

Describe 'AIB deprovision hook delivery'
  setup_template() {
    TEST_DIR="$(mktemp -d)"
    OPTIMIZE_SCRIPT="${SHELLSPEC_PROJECT_ROOT}/vhdbuilder/prefetch/scripts/optimize.sh"
    BUILD_COMMIT=0123456789abcdef0123456789abcdef01234567
  }

  cleanup_template() {
    rm -rf "${TEST_DIR:?}"
  }

  BeforeEach 'setup_template'
  AfterEach 'cleanup_template'

  # Exercise the actual renderer, stopping at template creation. Every Azure CLI
  # invocation is mocked; no build is queued and no Azure resource is modified.
  render_template() (
    export SUBSCRIPTION_ID=test LOCATION=test SIG_GALLERY_RESOURCE_GROUP_NAME=test
    export SIG_GALLERY_NAME=test SIG_IMAGE_NAME=test SKU_NAME=test
    export STORAGE_ACCOUNT_BLOB_URL=https://example.invalid/vhds
    export STORAGE_ACCOUNT_BLOB_URL_STAGING=https://example.invalid/staging
    export VHD_NAME=test.vhd IMAGE_BUILDER_IDENTITY_ID=/identities/test
    export BUILD_RUN_NUMBER=test CAPTURED_SIG_VERSION=1.2.3 HYPERV_GENERATION=V2
    export WORKLOAD_OPTIMIZATION_SCRIPT_URL=https://example.invalid/workload.sh
    export FEATURE_FLAGS="$1" GIT_VERSION="$2" ENABLE_TRUSTED_LAUNCH="${3:-false}"

    az() {
      case "$*" in
        "storage blob exists "*) echo '{"exists":false}' ;;
        "group exists "*) echo true ;;
        "image builder show "*) echo '{"id":""}' ;;
        "image show "*) echo '{"id":"/images/test"}' ;;
        "resource create "*) return 42 ;;
        *) echo "Unexpected Azure CLI call: $*" >&2; return 99 ;;
      esac
    }
    export -f az

    cd "${TEST_DIR}" || return $?
    bash "${OPTIMIZE_SCRIPT}" > optimize.stdout 2> optimize.stderr
    local status=$?
    if [ -f input.json ]; then
      jq -er '.properties.customize[] | select(.type == "File") | .sourceUri' input.json
      jq -er '.properties.customize[] | select(.name == "InstallDeprovisionHook") | .inline[]' input.json
    else
      cat optimize.stdout
      cat optimize.stderr >&2
    fi
    return "${status}"
  )

  It 'pins every downloaded helper to the build commit and wires CVM into the final hook'
    When run render_template minimal,cvm "${BUILD_COMMIT}"
    The status should equal 42
    The output should include "https://raw.githubusercontent.com/Azure/AgentBaker/${BUILD_COMMIT}/vhdbuilder/packer/aib-deprovision.sh"
    The output should include "https://raw.githubusercontent.com/Azure/AgentBaker/${BUILD_COMMIT}/vhdbuilder/packer/post-deprovision-walinuxagent.sh"
    The output should include "https://raw.githubusercontent.com/Azure/AgentBaker/${BUILD_COMMIT}/vhdbuilder/packer/install_walinuxagent.py"
    The output should include 'exec sudo /bin/bash -eu /tmp/agentbaker-deprovision.sh true'
    The output should include 'tee /tmp/DeprovisioningScript.sh'
    The output should not include '<AGENTBAKER_SCRIPT_BASE_URL>'
    The output should not include '<CVM_BUILD>'
  End

  It 'disables CVM credential cleanup for regular builds'
    When run render_template none "${BUILD_COMMIT}"
    The status should equal 42
    The output should include 'exec sudo /bin/bash -eu /tmp/agentbaker-deprovision.sh false'
  End

  It 'does not mistake Trusted Launch for CVM'
    When run render_template none "${BUILD_COMMIT}" true
    The status should equal 42
    The output should include 'exec sudo /bin/bash -eu /tmp/agentbaker-deprovision.sh false'
  End

  It 'rejects a moving branch instead of downloading unpinned helpers'
    When run render_template cvm master
    The status should equal 1
    The error should include 'GIT_VERSION must be the full AgentBaker commit SHA'
  End

  It 'fails before Azure calls when the build commit is missing'
    When run render_template cvm ''
    The status should equal 1
    The output should include 'GIT_VERSION is not set'
    The error should not include '+ az '
  End
End
