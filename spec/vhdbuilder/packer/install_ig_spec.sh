#!/bin/bash

Describe 'ig_extract_upstream_version function'
  Include './vhdbuilder/packer/install-ig.sh'

  It 'returns the upstream version on success'
    When call ig_extract_upstream_version "0.51.0-4.azl3"
    The status should be success
    The output should eq "0.51.0"
    The stderr should eq ""
  End

  It 'writes parse failures to stderr'
    When run ig_extract_upstream_version "not-a-version"
    The status should equal 1
    The output should eq ""
    The stderr should include "[ig] Could not parse upstream version from 'not-a-version'"
  End
End

Describe 'ig_validate_version_compatibility function'
  Include './vhdbuilder/packer/install-ig.sh'

  It 'writes version mismatches to stderr'
    OS="AZURELINUX"
    AZURELINUX_OS_NAME="AZURELINUX"
    IG_VERSION="0.51.1-4.azl3"

    When run ig_validate_version_compatibility
    The status should equal 1
    The output should eq ""
    The stderr should include "[ig] ig (0.51.1-4.azl3) and ig-gadgets (0.51.0-1.azl3) must share upstream version, found 0.51.1 vs 0.51.0"
  End
End
