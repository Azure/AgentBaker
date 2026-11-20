#!/bin/bash

Describe 'Kata-CC UVM version validation'
  setup() {
    export FEATURE_FLAGS=kata,kata-cc
    export HOST_VERSION=4.1.0.kata2-1.azl3 RPM_STATUS=0
    version_file="$SHELLSPEC_TMPBASE/kata-containers-cc-version.txt"
    printf '%s\n' "$HOST_VERSION" > "$version_file"
  }
  BeforeEach 'setup'

  Mock rpm
    printf '%s\n' "$HOST_VERSION"
    exit "$RPM_STATUS"
  End

  It 'accepts matching host and guest packages'
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file"
    The status should be success
    The output should include 'Kata-CC host RPM: 4.1.0.kata2-1.azl3; UVM tools RPM: 4.1.0.kata2-1.azl3'
  End

  It 'rejects mismatched versions'
    printf '%s\n' '4.1.0.kata1-1.azl3' > "$version_file"
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file"
    The status should be failure
    The output should include 'Kata-CC host RPM:'
    The stderr should include 'Kata-CC host/UVM version mismatch'
  End

  It 'rejects a missing version file'
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file.missing"
    The status should be failure
    The stderr should include 'Missing Kata-CC UVM version file'
  End

  It 'rejects multiple version entries'
    printf '%s\n' '4.1.0.kata2-1.azl3' '4.1.0.kata1-1.azl3' > "$version_file"
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file"
    The status should be failure
    The stderr should include 'Invalid Kata-CC UVM version'
  End

  It 'fails when the host RPM cannot be queried'
    RPM_STATUS=1
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file"
    The status should be failure
    The stderr should include 'Cannot determine installed kata-containers-cc host RPM version'
  End

  It 'skips plain Kata images'
    FEATURE_FLAGS=kata
    When run bash vhdbuilder/packer/validate-kata-cc-uvm-version.sh "$version_file.missing"
    The status should be success
  End
End
