#!/bin/bash

Describe 'Kata UVM version validation'
  setup() {
    export IMG_OFFER=azure-linux-3 FEATURE_FLAGS=kata
    export HOST_VERSION=4.1.0.kata0-1.azl3 RPM_STATUS=0
    export SYSTEM_COLLECTIONURI=https://example.com/organization/
    export SYSTEM_TEAMPROJECTID=test-project
    version_file="$SHELLSPEC_TMPBASE/kata-containers-version.txt"
    printf '%s\n' "$HOST_VERSION" > "$version_file"
  }
  BeforeEach 'setup'

  Mock rpm
    printf '%s\n' "$HOST_VERSION"
    exit "$RPM_STATUS"
  End

  It 'accepts matching host and guest packages'
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file"
    The status should be success
    The output should include 'Kata host RPM: 4.1.0.kata0-1.azl3; UVM tools RPM: 4.1.0.kata0-1.azl3'
  End

  Describe 'mismatched packages'
    Parameters
      '3.32.0.kata0-6.azl3'
      '4.1.0.kata0-2.azl3'
    End
    It 'rejects mismatched versions or package releases'
      printf '%s\n' "$1" > "$version_file"
      When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file"
      The status should be failure
      The output should include 'Kata host RPM:'
      The stderr should include 'Kata host/UVM version mismatch'
      The stderr should include 'https://example.com/organization/test-project/_build?definitionId=417429'
    End
  End

  It 'rejects a missing version file'
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file.missing"
    The status should be failure
    The stderr should include 'Missing Kata UVM version file'
  End

  It 'rejects an empty version file'
    : > "$version_file"
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file"
    The status should be failure
    The stderr should include 'Missing Kata UVM version file'
  End

  It 'rejects multiple version entries'
    printf '%s\n' '4.1.0.kata0-1.azl3' '3.32.0.kata0-6.azl3' > "$version_file"
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file"
    The status should be failure
    The stderr should include 'Invalid Kata UVM version'
  End

  It 'fails when the host RPM cannot be queried'
    RPM_STATUS=1
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file"
    The status should be failure
    The stderr should include 'Cannot determine installed kata-containers host RPM version'
  End

  It 'skips non-Kata Azure Linux images'
    FEATURE_FLAGS=None
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file.missing"
    The status should be success
  End

  It 'skips Azure Linux 2 Kata images'
    IMG_OFFER=cbl-mariner
    When run bash vhdbuilder/packer/validate-kata-uvm-version.sh "$version_file.missing"
    The status should be success
  End
End
