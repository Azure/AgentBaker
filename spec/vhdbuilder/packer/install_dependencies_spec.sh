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
  rpm2cpio() {
    echo "rpm2cpio $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${RPM2CPIO_RC:-0}"
  }
  cpio() {
    echo "cpio $*" >> "$ARTIFACT_STREAMING_CALLS"
    [ "${CPIO_RC:-0}" -eq 0 ] || return "$CPIO_RC"
    mkdir -p opt/acr/bin usr/lib/systemd/system
    : > opt/acr/bin/acr
    : > usr/lib/systemd/system/acr-mirror.service
  }
  cp() {
    echo "cp $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${CP_RC:-0}"
  }
  install() {
    echo "install $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${INSTALL_RC:-0}"
  }
  env() {
    echo "env $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${ACR_INIT_RC:-0}"
  }
  rm() {
    echo "rm $*" >> "$ARTIFACT_STREAMING_CALLS"
    command rm "$@"
  }

  It 'extracts the rpm payload and initializes acr on Azure Container Linux'
    OS="AZURECONTAINERLINUX"
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "rpm2cpio"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "cpio -idm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "cp -a"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "/opt/. /opt/"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "install -m 0644"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "/usr/lib/systemd/system/acr-mirror.service /etc/systemd/system/acr-mirror.service"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "env -C /opt/acr/bin ./acr init --min-init"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'uses the arm64 package name on Azure Container Linux ARM64'
    OS="AZURECONTAINERLINUX"
    TEST_ARM64=1
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_curl_file 10 5 60 ./acr-mirror-arm64.rpm https://example/acr-mirror-arm64.rpm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "rpm2cpio"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "/acr-mirror-arm64.rpm"
    The status should be success
  End

  It 'extracts the rpm payload for the Azure Linux ACL variant'
    OS="AZURELINUX"
    OS_VARIANT="AZURECONTAINERLINUX"
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "rpm2cpio"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "cpio -idm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "env -C /opt/acr/bin ./acr init --min-init"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'fails when the ACL rpm payload cannot be extracted'
    OS="AZURECONTAINERLINUX"
    RPM2CPIO_RC=127
    When run installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "rpm2cpio"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "acr init --min-init"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
  End

  It 'fails when acr initialization fails on ACL'
    OS="AZURECONTAINERLINUX"
    ACR_INIT_RC=1
    When run installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "env -C /opt/acr/bin ./acr init --min-init"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "rm ./acr-mirror.rpm"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
  End
End
