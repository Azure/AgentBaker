#!/bin/bash

# Tests for generate_cosi_package_version in
# vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh.
#
# Nebraska validates COSI package versions as strict SemVer, which rejects
# leading zeros in numeric components (e.g. 202608.06.0). These tests pin the
# normalization: leading zeros stripped, -fips appended only for FIPS builds,
# and never duplicated.

Describe 'generate_cosi_package_version'
  setup() {
    # Source only the functions (guarded by ${__SOURCED__:+return}), not the
    # main conversion flow.
    # shellcheck disable=SC1090
    __SOURCED__=1 . "./vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh"
  }
  BeforeEach 'setup'

  Describe 'strips per-component leading zeros'
    It 'normalizes day 01 (non-FIPS)'
      When call generate_cosi_package_version "202608.01.0" "false"
      The status should be success
      The output should equal "202608.1.0"
    End

    It 'normalizes day 06 (non-FIPS)'
      When call generate_cosi_package_version "202608.06.0" "false"
      The status should be success
      The output should equal "202608.6.0"
    End

    It 'leaves day 10 unchanged (non-FIPS)'
      When call generate_cosi_package_version "202608.10.0" "false"
      The status should be success
      The output should equal "202608.10.0"
    End
  End

  Describe 'appends -fips only for FIPS builds'
    It 'emits -fips when ENABLE_FIPS is true'
      When call generate_cosi_package_version "202608.06.0" "true"
      The status should be success
      The output should equal "202608.6.0-fips"
    End

    It 'omits -fips when ENABLE_FIPS is false'
      When call generate_cosi_package_version "202608.06.0" "false"
      The status should be success
      The output should equal "202608.6.0"
    End

    It 'treats ENABLE_FIPS case-insensitively (True)'
      When call generate_cosi_package_version "202608.10.2" "True"
      The status should be success
      The output should equal "202608.10.2-fips"
    End

    It 'does not duplicate -fips when the version already contains it'
      When call generate_cosi_package_version "202608.06.0-fips" "true"
      The status should be success
      The output should equal "202608.6.0-fips"
    End
  End

  Describe 'rejects a core that is not exactly three numeric components'
    It 'fails with too few components'
      When call generate_cosi_package_version "202608.06" "false"
      The status should be failure
      The error should be present
    End

    It 'fails with too many components'
      When call generate_cosi_package_version "202608.06.0.1" "false"
      The status should be failure
      The error should be present
    End

    It 'fails with a non-numeric component'
      When call generate_cosi_package_version "202608.aug.0" "false"
      The status should be failure
      The error should be present
    End
  End
End
