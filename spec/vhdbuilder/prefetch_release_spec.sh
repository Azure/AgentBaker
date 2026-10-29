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
