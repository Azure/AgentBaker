#!/bin/bash
# shellcheck disable=SC2329

Describe 'Packer settings source logging'
  setup() {
    SCRIPT_PATH="${PWD}/vhdbuilder/packer/produce-packer-settings.sh"
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_WORKDIR}/settings-logging.XXXXXX")
    mkdir -p "${TEST_ROOT}/vhdbuilder/packer"
    MODE=linuxVhdMode OS_SKU=AzureContainerLinux FEATURE_FLAGS=None ENVIRONMENT=tme
    PACKER_BUILD_LOCATION=westus3 CVM_PACKER_BUILD_LOCATION=westeurope AZURE_LOCATION=eastus
    SIG_SOURCE_GALLERY_UNIQUE_NAME=shared-acl-gallery
    SIG_SOURCE_IMAGE_NAME=acl-3.0-amd64 SIG_SOURCE_IMAGE_VERSION=20260923.1209152.2
  }
  BeforeEach 'setup'

  resolve_security_type_feature() {
    SECURITY_TYPE_FEATURE=TrustedLaunch
  }

  produce_ua_token() {
    UA_TOKEN=test-only-token
  }

  log_settings() (
    set -e
    cd "$TEST_ROOT"
    # Exercise the real CVM override and final settings generation without Azure setup.
    eval "$(sed -n '/^if grep -q "cvm" /,/^fi$/p' "$SCRIPT_PATH")"
    eval "$(sed -n '/^# set PACKER_BUILD_LOCATION /,$p' "$SCRIPT_PATH")"
  )

  Describe 'ACL builds'
    Parameters
      acl-3.0-amd64 20260923.1209152.2 None tme westus3
      acl-3.0-amd64 20260923.1209152.2 cvm tme westeurope
      acl-3.0-arm64 20260923.1209110.1 None tme westus3
      acl-3.0-amd64 20260923.1209152.2 cvm prod eastus
    End

    It "logs $1/$2 and the final $5 region before the settings dump"
      SIG_SOURCE_IMAGE_NAME="$1" SIG_SOURCE_IMAGE_VERSION="$2" FEATURE_FLAGS="$3" ENVIRONMENT="$4"
      expected=$(printf 'ACL base image: shared-acl-gallery/%s/%s\nPacker build region: %s\npacker settings:' "$1" "$2" "$5")

      When call log_settings
      The status should be success
      The output should include "$expected"
      The output should not include test-only-token
      The value "$(jq -r .location "${TEST_ROOT}/vhdbuilder/packer/settings.json")" should equal "$5"
    End
  End

  Describe 'other builds'
    Parameters
      linuxVhdMode Ubuntu
      linuxVhdMode AzureLinux
      linuxVhdMode AzureLinuxOSGuard
      windowsVhdMode Windows
      windowsVhdMode AzureContainerLinux
    End

    It "does not add ACL source messages for $1/$2"
      MODE="$1" OS_SKU="$2"

      When call log_settings
      The status should be success
      The output should include 'packer settings:'
      The output should not include 'ACL base image:'
      The output should not include 'Packer build region:'
      The output should not include test-only-token
    End
  End
End
