#!/bin/bash
<<<<<<< HEAD
# shellcheck disable=SC2329

Describe 'installPatchedCloudInit'
  BeforeAll "eval \"\$(sed -n '/^installPatchedCloudInit()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')\""

  setup() {
    TEST_DIR=$(mktemp -d)
    mkdir -p "${TEST_DIR}/24.04" "${TEST_DIR}/26.04"
    MOCK_VERSION="26.2-0ubuntu1~24.04.2+aks3"
  }

  cleanup() {
    rm -rf "$TEST_DIR"
  }

  sha256sum() {
    echo "verify $PWD"
    return "${MOCK_CHECKSUM_STATUS:-0}"
  }

  apt_get_install() {
    printf 'install %s\n' "$@"
    return "${MOCK_INSTALL_STATUS:-0}"
  }

  dpkg-query() {
    printf '%s' "$MOCK_VERSION"
  }

  python3() {
    echo 'verify runtime override'
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'installs and verifies the Ubuntu 24.04 package'
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be success
    The output should include "verify ${TEST_DIR}/24.04"
    The output should include "install ${TEST_DIR}/24.04/cloud-init_26.2-0ubuntu1~24.04.2+aks3_all.deb"
    The output should include 'verify runtime override'
  End

  It 'installs the split Ubuntu 26.04 packages together'
    MOCK_VERSION="26.2-0ubuntu1~26.04.1+aks3"
    When call installPatchedCloudInit UBUNTU 26.04 "$TEST_DIR"
    The status should be success
    The output should include "install ${TEST_DIR}/26.04/cloud-init-base_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-azure_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-cloud-sigma_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-smart-os_${MOCK_VERSION}_all.deb"
    The output should include 'verify runtime override'
  End

  Describe 'unsupported releases'
    Parameters
      UBUNTU 22.04
      UBUNTU 20.04
      AZURELINUX 3.0
      AZURECONTAINERLINUX 3.0
    End

    It 'does not modify other OS releases'
      When call installPatchedCloudInit "$1" "$2" "$TEST_DIR"
      The status should be success
      The output should be blank
    End
  End

  It 'fails before installation if checksum verification fails'
    MOCK_CHECKSUM_STATUS=1
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'verify'
    The output should not include 'install'
  End

  It 'propagates installation failures'
    MOCK_INSTALL_STATUS=1
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'install'
    The output should not include 'verify runtime override'
  End

  It 'rejects an unexpected installed version'
    MOCK_VERSION="26.2-0ubuntu1~24.04.2"
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'install'
    The error should include 'Unexpected cloud-init version'
=======
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
      The contents of file "$ARTIFACT_STREAMING_CALLS" should equal "downloadSysextFromVersion artifact-streaming mcr.microsoft.com/azurecontainerregistry/v2/artifact-streaming-sysext:${version}-$3"
      The status should be success
    End
  End

  It 'fails when the combined sysext cannot be cached'
    OS="AZURECONTAINERLINUX"
    DOWNLOAD_RC=1
    When run installAndConfigureArtifactStreaming "mcr.microsoft.com/azurecontainerregistry/v2/artifact-streaming-sysext:v1.0.0-1-azlinux3-x86-64" "v1.0.0-1-azlinux3"
    The status should equal "$ERR_ARTIFACT_STREAMING_DOWNLOAD"
>>>>>>> dfb97cf2685f2fa0f6723e0842a62c925c6a6ed0
  End
End
