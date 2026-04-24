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
