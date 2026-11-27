#!/bin/bash

Describe 'stage_signed_windows_powershell_scripts'
  BeforeAll "eval \"\$(sed -n '/^stage_signed_windows_powershell_scripts()/,/^}/p' ./.pipelines/scripts/windows_build_vhd.sh)\""

  setup_signed_scripts() {
    ORIGINAL_DIRECTORY="$PWD"
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_TMPBASE}/signed-windows.XXXXXX")
    WINDOWS_SIGNED_POWERSHELL_SCRIPTS_DIR="$TEST_ROOT/signed artifacts"
    LOCAL_SCRIPTS="$TEST_ROOT/checkout/vhdbuilder/packer/windows"
    SIGNED_SCRIPTS="$WINDOWS_SIGNED_POWERSHELL_SCRIPTS_DIR/vhdbuilder/packer/windows"
    mkdir -p "$LOCAL_SCRIPTS" "$SIGNED_SCRIPTS"
    printf 'unsigned helper' > "$LOCAL_SCRIPTS/helper.ps1"
    printf 'unsigned startup' > "$LOCAL_SCRIPTS/expand-volume.ps1"
    printf 'test script' > "$LOCAL_SCRIPTS/helper.tests.ps1"
    printf '\357\273\277signed helper\r\n# SIG # Begin signature block\r\n' > "$SIGNED_SCRIPTS/helper.ps1"
    printf 'signed startup\r\n# SIG # Begin signature block\r\n' > "$SIGNED_SCRIPTS/expand-volume.ps1"
    cd "$TEST_ROOT/checkout" || return 1
  }

  cleanup_signed_scripts() {
    cd "$ORIGINAL_DIRECTORY" || return 1
    rm -rf "$TEST_ROOT"
  }

  BeforeEach 'setup_signed_scripts'
  AfterEach 'cleanup_signed_scripts'

  stage_and_compare() {
    stage_signed_windows_powershell_scripts &&
      cmp "$SIGNED_SCRIPTS/helper.ps1" "$LOCAL_SCRIPTS/helper.ps1" &&
      cmp "$SIGNED_SCRIPTS/expand-volume.ps1" "$LOCAL_SCRIPTS/expand-volume.ps1"
  }

  It 'uses the signed bytes without changing encoding or line endings and excludes tests'
    When call stage_and_compare
    The status should be success
    The contents of file "$LOCAL_SCRIPTS/helper.tests.ps1" should eq 'test script'
  End

  It 'rejects a missing signed file before overwriting any checkout file'
    rm "$SIGNED_SCRIPTS/helper.ps1"
    When call stage_signed_windows_powershell_scripts
    The status should be failure
    The output should include 'Missing signed Packer script'
    The contents of file "$LOCAL_SCRIPTS/expand-volume.ps1" should eq 'unsigned startup'
  End

  It 'rejects an empty signed file rather than using the unsigned checkout copy'
    : > "$SIGNED_SCRIPTS/helper.ps1"
    When call stage_signed_windows_powershell_scripts
    The status should be failure
    The output should include 'Missing signed Packer script'
    The contents of file "$LOCAL_SCRIPTS/helper.ps1" should eq 'unsigned helper'
  End

  It 'requires a downloaded signed artifact'
    unset WINDOWS_SIGNED_POWERSHELL_SCRIPTS_DIR
    When call stage_signed_windows_powershell_scripts
    The status should be failure
    The output should include 'must point to the downloaded signed scripts'
  End
End

Describe 'configure_windows_build_mode'
  WINDOWS_BUILD_SCRIPT="./.pipelines/scripts/windows_build_vhd.sh"

  BeforeAll "eval \"\$(sed -n '/^configure_windows_build_mode()/,/^}/p' \"${WINDOWS_BUILD_SCRIPT}\")\""

  setup_environment() {
    BRANCH="refs/heads/main"
    DRY_RUN="False"
    ENVIRONMENT="test"
    GENERATE_PUBLISHING_INFO="False"
    unset IS_RELEASE_PIPELINE
    unset SIG_FOR_PRODUCTION
  }

  BeforeEach 'setup_environment'

  It 'keeps normal non-release builds in dry-run mode'
    When call configure_windows_build_mode
    The status should be success
    The variable IS_RELEASE_PIPELINE should eq "False"
    The variable SIG_FOR_PRODUCTION should eq "False"
    The variable DRY_RUN should eq "True"
    The output should be present
  End

  It 'allows TME release-candidate builds to generate publishing info without deleting the builder SIG'
    ENVIRONMENT="tme"
    GENERATE_PUBLISHING_INFO="True"
    When call configure_windows_build_mode
    The status should be success
    The variable IS_RELEASE_PIPELINE should eq "False"
    The variable SIG_FOR_PRODUCTION should eq "False"
    The variable DRY_RUN should eq "False"
    The output should include "TME release-candidate build"
  End

  It 'accepts the configured TME publishing mode values case-insensitively'
    ENVIRONMENT="tme"
    GENERATE_PUBLISHING_INFO="true"
    When call configure_windows_build_mode
    The status should be success
    The variable SIG_FOR_PRODUCTION should eq "False"
    The variable DRY_RUN should eq "False"
    The output should be present
  End

  It 'keeps TME builds in dry-run mode when publishing info is disabled'
    ENVIRONMENT="tme"
    When call configure_windows_build_mode
    The status should be success
    The variable SIG_FOR_PRODUCTION should eq "False"
    The variable DRY_RUN should eq "True"
    The output should be present
  End

  It 'marks a non-dry-run release-branch build for production'
    BRANCH="refs/heads/windows/v20260818"
    When call configure_windows_build_mode
    The status should be success
    The variable IS_RELEASE_PIPELINE should eq "True"
    The variable SIG_FOR_PRODUCTION should eq "True"
    The variable DRY_RUN should eq "False"
    The output should be present
  End

  It 'rejects a non-dry-run release build from a non-release branch'
    IS_RELEASE_PIPELINE="True"
    When call configure_windows_build_mode
    The status should be failure
    The output should include "Please use a branch with the format windows/vYYYYMMDD"
  End
End
