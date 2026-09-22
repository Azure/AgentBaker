#!/bin/bash

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
  bsdtar() {
    echo "bsdtar $*" >> "$ARTIFACT_STREAMING_CALLS"
    [ "${BSDTAR_RC:-0}" -eq 0 ] || return "$BSDTAR_RC"
    if [ "$1" = "-Oxf" ]; then
      echo "[Unit]"
    fi
  }
  install() {
    cat > /dev/null
    echo "install $*" >> "$ARTIFACT_STREAMING_CALLS"
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
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "bsdtar -C / -xf ./acr-mirror.rpm opt/"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "bsdtar -Oxf ./acr-mirror.rpm usr/lib/systemd/system/acr-mirror.service"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "install -m 0644 /dev/stdin /etc/systemd/system/acr-mirror.service"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "env -C /opt/acr/bin ./acr init --min-init"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'uses the arm64 package name on Azure Container Linux ARM64'
    OS="AZURECONTAINERLINUX"
    TEST_ARM64=1
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "retrycmd_curl_file 10 5 60 ./acr-mirror-arm64.rpm https://example/acr-mirror-arm64.rpm"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "bsdtar -C / -xf ./acr-mirror-arm64.rpm opt/"
    The status should be success
  End

  It 'extracts the rpm payload for the Azure Linux ACL variant'
    OS="AZURELINUX"
    OS_VARIANT="AZURECONTAINERLINUX"
    When call installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "bsdtar -C / -xf ./acr-mirror.rpm opt/"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "env -C /opt/acr/bin ./acr init --min-init"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should not include "dnf_install"
    The status should be success
  End

  It 'fails when the ACL rpm payload cannot be extracted'
    OS="AZURECONTAINERLINUX"
    BSDTAR_RC=127
    When run installAndConfigureArtifactStreaming "https://example/acr-mirror.rpm" "1.0.0"
    The contents of file "$ARTIFACT_STREAMING_CALLS" should include "bsdtar -C / -xf ./acr-mirror.rpm opt/"
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
