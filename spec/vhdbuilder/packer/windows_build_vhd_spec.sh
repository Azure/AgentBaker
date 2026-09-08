#!/bin/bash

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
