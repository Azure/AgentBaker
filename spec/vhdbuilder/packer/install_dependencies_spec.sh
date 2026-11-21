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

Describe 'configureNetworkdNetlinkTimeout'
  load_networkd_helpers() {
    . ./parts/linux/cloud-init/artifacts/cse_helpers.sh >/dev/null
    eval "$(sed -n '/^configureNetworkdNetlinkTimeout()/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"
  }
  BeforeAll 'load_networkd_helpers'

  setup_networkd_timeout() {
    TEST_DIR=$(mktemp -d)
    TEST_DROP_IN="$TEST_DIR/10-netlink-timeout.conf"
    TEST_CALLS="$TEST_DIR/calls"
    : > "$TEST_CALLS"
    OS="UBUNTU"
    OS_VERSION="24.04"
    MKDIR_STATUS=0
    TEE_STATUS=0
    CHMOD_STATUS=0
  }
  cleanup_networkd_timeout() {
    rm -rf "$TEST_DIR"
  }
  BeforeEach 'setup_networkd_timeout'
  AfterEach 'cleanup_networkd_timeout'

  mkdir() {
    printf 'mkdir %s\n' "$*" >> "$TEST_CALLS"
    return "$MKDIR_STATUS"
  }
  tee() {
    printf 'tee %s\n' "$*" >> "$TEST_CALLS"
    [ "$TEE_STATUS" -eq 0 ] || return "$TEE_STATUS"
    command tee "$TEST_DROP_IN"
  }
  chmod() {
    printf 'chmod %s\n' "$*" >> "$TEST_CALLS"
    [ "$CHMOD_STATUS" -eq 0 ] || return "$CHMOD_STATUS"
    command chmod "$1" "$TEST_DROP_IN"
  }

  It 'installs the Ubuntu 24.04 service drop-in with mode 0644'
    expected_drop_in=$(printf '%s\n' '[Service]' 'Environment=SYSTEMD_NETLINK_DEFAULT_TIMEOUT=250s')
    expected_calls=$(printf '%s\n' \
      'mkdir -p /etc/systemd/system/systemd-networkd.service.d' \
      'tee /etc/systemd/system/systemd-networkd.service.d/10-netlink-timeout.conf' \
      'chmod 644 /etc/systemd/system/systemd-networkd.service.d/10-netlink-timeout.conf')
    When call configureNetworkdNetlinkTimeout
    The status should be success
    The output should be blank
    The contents of file "$TEST_DROP_IN" should equal "$expected_drop_in"
    The contents of file "$TEST_CALLS" should equal "$expected_calls"
  End

  Describe 'excluded images'
    Parameters
      UBUNTU 20.04
      UBUNTU 22.04
      UBUNTU 26.04
      AZURELINUX 3.0
      MARINER 2.0
      FLATCAR 4230.2.2
      AZURECONTAINERLINUX 3.0
    End

    It "leaves $1 $2 unchanged"
      OS="$1" OS_VERSION="$2"
      When call configureNetworkdNetlinkTimeout
      The status should be success
      The output should be blank
      The contents of file "$TEST_CALLS" should be blank
      The path "$TEST_DROP_IN" should not be exist
    End
  End

  Describe 'installation failures'
    Parameters
      "directory creation"
      "configuration write"
      "permission setting"
    End

    It "fails the build when $1 fails"
      case "$1" in
        "directory creation") MKDIR_STATUS=1 ;;
        "configuration write") TEE_STATUS=1 ;;
        "permission setting") CHMOD_STATUS=1 ;;
      esac
      When call configureNetworkdNetlinkTimeout
      The status should be failure
      The output should be blank
    End
  End
End
