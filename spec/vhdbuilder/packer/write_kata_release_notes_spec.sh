#!/bin/bash

Describe 'Kata default release notes'
  setup() {
    export OS=AZURELINUX FEATURE_FLAGS=kata
    kernel_file="$SHELLSPEC_TMPBASE/kata-uvm-kernel-version.txt"
    notes_file="$SHELLSPEC_TMPBASE/kata-release-notes.txt"
    printf '%s\n' '6.18.52.mshv1-2.azl3' > "$kernel_file"
    : > "$notes_file"
  }
  BeforeEach 'setup'

  Mock rpm
    printf '%s\n' '4.1.0.kata1-2.azl3'
  End
  Mock uname
    printf '%s\n' '6.18.34.mshv3-1.azl3'
  End

  It 'publishes defaults from the artifact and the booted host'
    When run bash vhdbuilder/packer/write-kata-release-notes.sh "$kernel_file" "$notes_file"
    The status should be success
    The contents of file "$notes_file" should include '=== Kata Default Versions Begin'
    The contents of file "$notes_file" should include 'Kata host kernel: 6.18.34.mshv3-1.azl3'
    The contents of file "$notes_file" should include 'Kata UVM kernel: 6.18.52.mshv1-2.azl3'
    The contents of file "$notes_file" should include 'Kata runtime: 4.1.0.kata1-2.azl3'
    The contents of file "$notes_file" should include 'not standard Azure Linux or Kata-CC'
    The contents of file "$notes_file" should include '=== Kata Default Versions End'
  End

  It 'rejects missing artifact metadata instead of guessing from installed packages'
    When run bash vhdbuilder/packer/write-kata-release-notes.sh "$kernel_file.missing" "$notes_file"
    The status should be failure
    The stderr should include 'Missing Kata UVM kernel metadata'
    The contents of file "$notes_file" should equal ''
  End

  It 'rejects multi-line metadata'
    printf '%s\n' '6.18.52.mshv1-2.azl3' 'other' > "$kernel_file"
    When run bash vhdbuilder/packer/write-kata-release-notes.sh "$kernel_file" "$notes_file"
    The status should be failure
    The stderr should include 'Invalid Kata UVM kernel metadata'
  End

  Describe 'invalid kernel values'
    Parameters
      ''
      '6.18.52 mshv1'
      '6.18.52;other'
    End
    It 'rejects empty or unsafe values without appending notes'
      printf '%s\n' "$1" > "$kernel_file"
      When run bash vhdbuilder/packer/write-kata-release-notes.sh "$kernel_file" "$notes_file"
      The status should be failure
      The stderr should include 'Invalid Kata UVM kernel metadata'
      The contents of file "$notes_file" should equal ''
    End
  End

  Describe 'other image variants'
    Parameters
      'AZURELINUX' 'None'
      'AZURELINUX' 'kata,kata-cc'
      'MARINER' 'kata'
      'UBUNTU' 'kata'
    End
    It 'does not publish ordinary Azure Linux 3 Kata defaults'
      OS=$1 FEATURE_FLAGS=$2
      When run bash vhdbuilder/packer/write-kata-release-notes.sh "$kernel_file.missing" "$notes_file"
      The status should be success
      The contents of file "$notes_file" should equal ''
    End
  End
End
