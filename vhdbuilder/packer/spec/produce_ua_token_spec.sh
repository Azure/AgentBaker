#shellcheck shell=bash

# Tests for produce_ua_token function from produce-packer-settings-functions.sh

Describe 'produce_ua_token function'
  Include '/home/tim/git/AgentBaker/vhdbuilder/packer/produce-packer-settings-functions.sh'

  # Helper function to reset environment variables
  setup_environment() {
    MODE=""
    OS_SKU=""
    OS_VERSION=""
    ENABLE_FIPS=""
    UA_TOKEN=""
  }

  BeforeEach 'setup_environment'

  Describe 'Ubuntu 18.04 scenarios'
    It 'should succeed with valid UA_TOKEN for Ubuntu 18.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN="test-token-123"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "test-token-123"
      The stdout should include "will use token for UA attachment"
    End

    It 'should succeed with mixed case Ubuntu for 18.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="Ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN="mixed-case-token"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "mixed-case-token"
      The stdout should include "will use token for UA attachment"
    End

    It 'should succeed with uppercase UBUNTU for 18.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="UBUNTU"
      OS_VERSION="18.04"
      UA_TOKEN="uppercase-token"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "uppercase-token"
      The stdout should include "will use token for UA attachment"
    End

    It 'should fail without UA_TOKEN for Ubuntu 18.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN=""
      When run produce_ua_token
      The status should equal 1
      The output should include "UA_TOKEN must be provided when building SKUs which require ESM"
    End

    It 'should preserve existing environment UA_TOKEN for Ubuntu 18.04'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      export UA_TOKEN="env-token-123"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "env-token-123"
      The stdout should include "will use token for UA attachment"
    End
  End

  Describe 'Ubuntu 20.04 scenarios'
    It 'should succeed with valid UA_TOKEN for Ubuntu 20.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="20.04"
      UA_TOKEN="test-token-456"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "test-token-456"
      The stdout should include "will use token for UA attachment"
    End

    It 'should fail without UA_TOKEN for Ubuntu 20.04 in linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="20.04"
      UA_TOKEN=""
      When run produce_ua_token
      The status should equal 1
      The output should include "UA_TOKEN must be provided when building SKUs which require ESM"
    End
  End

  Describe 'Ubuntu versions that do not require UA_TOKEN'
    It 'should set UA_TOKEN to "notused" for Ubuntu 22.04 without FIPS'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="false"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for Ubuntu 24.04'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="24.04"
      ENABLE_FIPS="false"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for empty OS_VERSION'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION=""
      ENABLE_FIPS="false"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End
  End

  Describe 'ENABLE_FIPS=true scenarios'
    It 'should succeed with valid UA_TOKEN when FIPS is enabled (lowercase true)'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="true"
      UA_TOKEN="fips-token-789"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "fips-token-789"
      The stdout should include "will use token for UA attachment"
    End

    It 'should succeed with valid UA_TOKEN when FIPS is enabled (uppercase TRUE)'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="TRUE"
      UA_TOKEN="fips-token-uppercase"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "fips-token-uppercase"
      The stdout should include "will use token for UA attachment"
    End

    It 'should succeed with valid UA_TOKEN when FIPS is enabled (mixed case True)'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="True"
      UA_TOKEN="fips-token-mixed"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "fips-token-mixed"
      The stdout should include "will use token for UA attachment"
    End

    It 'should fail without UA_TOKEN when FIPS is enabled'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="true"
      UA_TOKEN=""
      When run produce_ua_token
      The status should equal 1
      The output should include "UA_TOKEN must be provided when building SKUs which require ESM"
    End

    It 'should set UA_TOKEN to "notused" when FIPS is disabled'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS="false"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" when FIPS is empty'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      ENABLE_FIPS=""
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End
  End

  Describe 'Non-Ubuntu OS scenarios'
    It 'should set UA_TOKEN to "notused" for CentOS with linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="centos"
      OS_VERSION="8"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for RHEL with linuxVhdMode'
      MODE="linuxVhdMode"
      OS_SKU="rhel"
      OS_VERSION="8"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for empty OS_SKU'
      MODE="linuxVhdMode"
      OS_SKU=""
      OS_VERSION="18.04"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End
  End

  Describe 'Non-linuxVhdMode scenarios'
    It 'should set UA_TOKEN to "notused" for windowsVhdMode with Ubuntu 18.04'
      MODE="windowsVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for unknown mode with Ubuntu 18.04'
      MODE="unknownMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should set UA_TOKEN to "notused" for empty MODE with Ubuntu 18.04'
      MODE=""
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End
  End

  Describe 'Edge cases and special scenarios'
    It 'should handle unset ENABLE_FIPS variable'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="22.04"
      unset ENABLE_FIPS
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should log correct OS_VERSION and ENABLE_FIPS values'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      ENABLE_FIPS="false"
      UA_TOKEN="logging-test-token"
      When call produce_ua_token
      The status should be success
      The stdout should include "OS_VERSION: 18.04"
      The stdout should include "ENABLE_FIPS: false"
    End

    It 'should handle complex version strings'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04.6"
      UA_TOKEN=""
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "notused"
    End

    It 'should be case-insensitive for both OS_SKU and ENABLE_FIPS'
      MODE="linuxVhdMode"
      OS_SKU="UBUNTU"
      OS_VERSION="22.04"
      ENABLE_FIPS="TRUE"
      UA_TOKEN="case-insensitive-token"
      When call produce_ua_token
      The status should be success
      The variable UA_TOKEN should eq "case-insensitive-token"
      The stdout should include "will use token for UA attachment"
    End
  End

  Describe 'Function behavior verification'
    It 'should output logging messages to stdout'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN="stdout-test-token"
      When call produce_ua_token
      The status should be success
      The stdout should not be blank
      The stdout should include "will use token for UA attachment"
    End

    It 'should preserve set +x directive (no debug output when sourced)'
      MODE="linuxVhdMode"
      OS_SKU="ubuntu"
      OS_VERSION="18.04"
      UA_TOKEN="debug-test-token"
      When call produce_ua_token
      The status should be success
      # Should not contain bash debug output like "+ MODE=linuxVhdMode"
      The stdout should not include "+ MODE="
      The stdout should not include "+ OS_SKU="
    End
  End
End