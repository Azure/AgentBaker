#!/bin/bash

Describe 'Release prefetch eligibility'
  Include ./vhdbuilder/prefetch/scripts/set-prefetch-variables.sh

  setup_release() {
    OS_SKU=AzureContainerLinux
    BUILDER=packer
    ARCHITECTURE=X86_64
    ENABLE_FIPS=False
    ENABLE_TRUSTED_LAUNCH=True
    HYPERV_GENERATION=V2
    FEATURE_FLAGS=None
    git() { echo 0123456789abcdef0123456789abcdef01234567; }
  }
  BeforeEach 'setup_release'

  It 'enables supported ACL release builds with a pinned helper and ACL template'
    When call set_prefetch_variables
    The status should be success
    The output should include 'PREFETCH_COMPATIBLE]True'
    The output should include '/templates/acl.json'
    The output should include '0123456789abcdef0123456789abcdef01234567/vhdbuilder/scripts/acl-prefetch-driver.sh'
  End

  Describe 'Prefetch source selection'
    setup_source() {
      SOURCE_TEST_ROOT="$(mktemp -d)"
      export SOURCE_TEST_ROOT
      export OPTIMIZER="${SHELLSPEC_PROJECT_ROOT}/vhdbuilder/prefetch/scripts/optimize.sh"
      export IMAGE_BUILDER_TEMPLATE_PATH="${SHELLSPEC_PROJECT_ROOT}/vhdbuilder/prefetch/templates/acl.json"
      export SUBSCRIPTION_ID=subscription LOCATION=eastus2
      export SIG_GALLERY_RESOURCE_GROUP_NAME=rg SIG_GALLERY_NAME=gallery SIG_IMAGE_NAME=acl
      export SKU_NAME=aclgen2TL VHD_NAME=candidate.vhd
      export STORAGE_ACCOUNT_BLOB_URL=https://storage.example/vhd
      export STORAGE_ACCOUNT_BLOB_URL_STAGING=https://storage.example/staging
      export IMAGE_BUILDER_IDENTITY_ID=/identities/builder BUILD_RUN_NUMBER=test
      export CAPTURED_SIG_VERSION=1.2.3 HYPERV_GENERATION=V2 FEATURE_FLAGS=None
      export ENABLE_TRUSTED_LAUNCH=True OS_SKU=AzureContainerLinux
      export WORKLOAD_OPTIMIZATION_SCRIPT_URL=https://example.test/driver.sh
      export SOURCE_DEFINITION='{"osState":"Generalized","features":[{"name":"SecurityType","value":"TrustedLaunchSupported"}]}'
      export SOURCE_LOOKUP_STATUS=0
    }
    cleanup_source() {
      rm -rf "${SOURCE_TEST_ROOT}"
    }
    BeforeEach 'setup_source'
    AfterEach 'cleanup_source'

    Mock az
      case "$1 $2 $3" in
        "storage blob exists") echo '{"exists":false}' ;;
        "group exists -g") echo true ;;
        "image builder show") ;;
        "sig image-definition show")
          printf '%s\n' "${SOURCE_DEFINITION}"
          exit "${SOURCE_LOOKUP_STATUS}"
          ;;
        "image show -g") echo '{"id":"/images/legacy-managed"}' ;;
        "resource create -n") exit 42 ;;
        *) echo "Unexpected Azure command: $*" >&2; exit 99 ;;
      esac
    End

    run_source_selection() {
      cd "${SOURCE_TEST_ROOT}" || return
      bash "${OPTIMIZER}"
    }

    It 'keeps ACL gallery trust even when the Trusted Launch flavor flag is enabled'
      When run run_source_selection
      The status should equal 42
      The output should include 'without losing its firmware trust'
      The stderr should be present
      The contents of file "${SOURCE_TEST_ROOT}/input.json" should include '"type": "SharedImageVersion"'
      The contents of file "${SOURCE_TEST_ROOT}/input.json" should include '/images/acl/versions/1.2.3'
    End

    Parameters
      '{"osState":"Generalized","features":[{"name":"SecurityType","value":"TrustedLaunch"}]}'
      '{"osState":"Specialized","features":[{"name":"SecurityType","value":"TrustedLaunchSupported"}]}'
      '{"osState":"Generalized","features":[]}'
    End
    It 'rejects incompatible ACL sources before creating an AIB template'
      SOURCE_DEFINITION="$1"
      When run run_source_selection
      The status should equal 1
      The stderr should include 'ACL prefetch requires a generalized TrustedLaunchSupported'
      The output should be present
      The path "${SOURCE_TEST_ROOT}/input.json" should not be exist
    End

    It 'propagates ACL source lookup failure'
      SOURCE_LOOKUP_STATUS=17
      When run run_source_selection
      The status should equal 17
      The stderr should be present
      The output should be present
      The path "${SOURCE_TEST_ROOT}/input.json" should not be exist
    End

    It 'preserves managed-image conversion for other Trusted Launch operating systems'
      OS_SKU=Ubuntu
      When run run_source_selection
      The status should equal 42
      The output should include 'will create managed image source'
      The stderr should be present
      The contents of file "${SOURCE_TEST_ROOT}/input.json" should include '"type": "ManagedImage"'
      The contents of file "${SOURCE_TEST_ROOT}/input.json" should include '/images/legacy-managed'
    End
  End

  Parameters
    imagecustomizer X86_64 False True V2 None
    packer ARM64 False True V2 None
    packer X86_64 True True V2 None
    packer X86_64 False False V2 None
    packer X86_64 False True V1 None
    packer X86_64 False True V2 cvm
  End
  It 'keeps unvalidated ACL variants excluded'
    BUILDER="$1" ARCHITECTURE="$2" ENABLE_FIPS="$3"
    ENABLE_TRUSTED_LAUNCH="$4" HYPERV_GENERATION="$5" FEATURE_FLAGS="$6"
    When call set_prefetch_variables
    The status should be success
    The output should include 'PREFETCH_COMPATIBLE]False'
    The output should not include '/templates/acl.json'
  End

  It 'preserves the default template and workload URL for Ubuntu'
    OS_SKU=Ubuntu
    When call set_prefetch_variables
    The status should be success
    The output should include 'PREFETCH_COMPATIBLE]True'
    The output should include '/templates/optimize.json'
    The output should not include 'PREFETCH_WORKLOAD_OPTIMIZATION_SCRIPT_URL'
  End

  It 'preserves the ARM64 CVM exclusion for other operating systems'
    OS_SKU=Ubuntu ARCHITECTURE=ARM64 FEATURE_FLAGS=cvm
    When call set_prefetch_variables
    The status should be success
    The output should include 'PREFETCH_COMPATIBLE]False'
  End

  It 'fails instead of emitting an unpinned workload URL'
    git() { return 22; }
    When call set_prefetch_variables
    The status should be failure
    The output should not include 'PREFETCH_COMPATIBLE]True'
  End
End
