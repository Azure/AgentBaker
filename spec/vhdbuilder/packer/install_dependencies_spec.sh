#!/bin/bash
set -o pipefail

eval "$(sed -n '/^installAndConfigureArtifactStreaming() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"

Describe 'installAndConfigureArtifactStreaming'
  setup_artifact_streaming() {
    ARTIFACT_STREAMING_CALLS="$(mktemp)"
    VHD_LOGS_FILEPATH="$(mktemp)"
    ERR_ARTIFACT_STREAMING_DOWNLOAD=149
    OS="UBUNTU"
    OS_VARIANT=""
  }
  cleanup_artifact_streaming() {
    command rm -f ./acr-mirror.rpm ./acr-mirror-arm64.rpm
    rm -f "$ARTIFACT_STREAMING_CALLS" "$VHD_LOGS_FILEPATH"
  }
  BeforeEach 'setup_artifact_streaming'
  AfterEach 'cleanup_artifact_streaming'

  isARM64() {
    echo "${TEST_ARM64:-0}"
  }
  isACL() {
    [ "$OS" = "AZURECONTAINERLINUX" ] ||
      { [ "$OS" = "AZURELINUX" ] && [ "$OS_VARIANT" = "AZURECONTAINERLINUX" ]; }
  }
  retrycmd_curl_file() {
    echo "retrycmd_curl_file $*" >> "$ARTIFACT_STREAMING_CALLS"
    : > "$4"
  }
  apt_get_install() {
    echo "apt_get_install $*" >> "$ARTIFACT_STREAMING_CALLS"
  }
  dnf_install() {
    echo "dnf_install $*" >> "$ARTIFACT_STREAMING_CALLS"
  }
  retrycmd_if_failure() {
    echo "retrycmd_if_failure $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${DNF_RC:-0}"
  }
  rm() {
    echo "rm $*" >> "$ARTIFACT_STREAMING_CALLS"
    command rm "$@"
  }

  It 'installs the rpm with dnf on Azure Container Linux'
    OS="AZURECONTAINERLINUX"
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_if_failure 10 2 120 dnf install -y ./acr-mirror.rpm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'uses the arm64 package name on Azure Container Linux ARM64'
    OS="AZURECONTAINERLINUX"
    TEST_ARM64=1
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_curl_file 10 5 60 ./acr-mirror-arm64.rpm https://example/acr-mirror-arm64.rpm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_if_failure 10 2 120 dnf install -y ./acr-mirror-arm64.rpm"
    The status should be success
  End

  It 'installs the rpm with dnf for the Azure Linux ACL variant'
    OS="AZURELINUX"
    OS_VARIANT="AZURECONTAINERLINUX"
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_if_failure 10 2 120 dnf install -y ./acr-mirror.rpm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'fails when dnf cannot install the ACL rpm'
    OS="AZURECONTAINERLINUX"
    DNF_RC=1
    When run installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_if_failure 10 2 120 dnf install -y ./acr-mirror.rpm"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
  End
End
