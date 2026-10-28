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
  downloadSysextFromVersion() {
    echo "downloadSysextFromVersion $*" >> "$ARTIFACT_STREAMING_CALLS"
    return "${DOWNLOAD_RC:-0}"
  }
  rm() {
    echo "rm $*" >> "$ARTIFACT_STREAMING_CALLS"
    command rm "$@"
  }

  Describe 'caches the combined sysext without installing RPMs or rewriting OCI tags'
    Parameters
      "AZURECONTAINERLINUX" "" "x86-64" 0
      "AZURECONTAINERLINUX" "" "arm64" 1
      "AZURELINUX" "AZURECONTAINERLINUX" "x86-64" 0
      "AZURELINUX" "AZURECONTAINERLINUX" "arm64" 1
    End
    It "caches the $3 artifact for $1 $2"
      OS="$1"
      OS_VARIANT="$2"
      TEST_ARM64="$4"
      version=$(jq -r '.Packages[] | select(.name == "acr-mirror") | .downloadURIs.azurecontainerlinux.current.versionsV2[0].latestVersion' parts/common/components.json)
      url=$(jq -r '.Packages[] | select(.name == "acr-mirror") | .downloadURIs.azurecontainerlinux.current.downloadURL' parts/common/components.json)
      url=${url//\$\{version\}/$version}
      url=${url//\$\{SYSTEMD_ARCH\}/$3}
      When call installAndConfigureArtifactStreaming "$url" "$version"
      The contents of file "$ARTIFACT_STREAMING_CALLS" should equal "downloadSysextFromVersion artifact-streaming mcr.microsoft.com/acr/v2/artifact-streaming-sysext:${version}-$3"
      The status should be success
    End
  End

  It 'fails when the combined sysext cannot be cached'
    OS="AZURECONTAINERLINUX"
    DOWNLOAD_RC=1
    When run installAndConfigureArtifactStreaming "mcr.microsoft.com/acr/v2/artifact-streaming-sysext:v1.0.0-2-azlinux3-x86-64" "v1.0.0-2-azlinux3"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
  End
End
