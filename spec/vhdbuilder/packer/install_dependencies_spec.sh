#!/bin/bash
set -o pipefail

eval "$(sed -n '/^installAndConfigureArtifactStreaming() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"
eval "$(sed -n '/^cachePackageAndBinaryComponents() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"

Describe 'credential provider VHD caching'
  Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
  Include "./vhdbuilder/scripts/linux/tool_installs.sh"

  setup_provider_caching() {
    COMPONENTS_FILEPATH=$(mktemp)
    VHD_LOGS_FILEPATH=$(mktemp)
    jq '{Packages: [.Packages[] | select(
      .name == "azure-acr-credential-provider" or
      .name == "azure-acr-credential-provider-legacy"
    )]}' parts/common/components.json > "$COMPONENTS_FILEPATH"
    IS_KATA=false
    CPU_ARCH=amd64
  }
  cleanup_provider_caching() {
    rm -f "$COMPONENTS_FILEPATH" "$VHD_LOGS_FILEPATH"
  }
  BeforeEach 'setup_provider_caching'
  AfterEach 'cleanup_provider_caching'

  downloadCredentialProvider() { echo "legacy: $1 $2"; }
  downloadPkgFromVersion() { echo "native: $1 $3"; }
  downloadSysextFromVersion() { echo "sysext: $1 $3 $2"; }
  capture_benchmark() { :; }

  Parameters
    "UBUNTU" "22.04" "" "x86-64" native yes
    "UBUNTU" "26.04" "" "arm64" native no
    "AZURELINUX" "3.0" "" "x86-64" native yes
    "AZURELINUX" "3.0" "OSGUARD" "x86-64" native yes
    "MARINER" "2.0" "" "x86-64" none yes
    "FLATCAR" "current" "" "x86-64" sysext no
    "FLATCAR" "current" "" "arm64" sysext no
    "AZURECONTAINERLINUX" "3.0" "" "x86-64" sysext no
    "AZURELINUX" "3.0" "AZURECONTAINERLINUX" "arm64" sysext no
  End

  It "keeps the correct download paths and formats for $1 $2 $3 $4"
    OS="$1" OS_VERSION="$2" OS_VARIANT="$3" SYSTEMD_ARCH="$4"
    if [ "$4" = arm64 ]; then CPU_ARCH=arm64; fi

    When call cachePackageAndBinaryComponents

    The status should be success
    if [ "$5" = native ]; then
      The output should include 'native: azure-acr-credential-provider /opt/azure-acr-credential-provider/downloads'
      The output should not include 'sysext:'
    elif [ "$5" = sysext ]; then
      The output should include 'sysext: azure-acr-credential-provider /opt/azure-acr-credential-provider/downloads mcr.microsoft.com/oss/v2/kubernetes/azure-acr-credential-provider-sysext:'
      The output should include "-$4"
      The output should not include 'native:'
    else
      The output should not include 'native:'
      The output should not include 'sysext:'
    fi
    if [ "$6" = yes ]; then
      The output should include 'legacy: /opt/credentialprovider/downloads mcr.microsoft.com/oss/binaries/kubernetes/azure-acr-credential-provider:'
    else
      The output should not include 'legacy:'
    fi
  End
End

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
      The contents of file "$ARTIFACT_STREAMING_CALLS" should equal "downloadSysextFromVersion artifact-streaming mcr.microsoft.com/azurecontainerregistry/v2/artifact-streaming-sysext:${version}-$3"
      The status should be success
    End
  End

  It 'fails when the combined sysext cannot be cached'
    OS="AZURECONTAINERLINUX"
    DOWNLOAD_RC=1
    When run installAndConfigureArtifactStreaming "mcr.microsoft.com/azurecontainerregistry/v2/artifact-streaming-sysext:v1.0.0-1-azlinux3-x86-64" "v1.0.0-1-azlinux3"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
  End
End
