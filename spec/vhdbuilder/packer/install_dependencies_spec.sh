#!/bin/bash
set -o pipefail

eval "$(sed -n '/^installAndConfigureArtifactStreaming() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"
eval "$(sed -n '/^getNvidiaContainerToolkitVersion() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"
eval "$(sed -n '/^cacheGPUContainerImageComponents() {/,/^}/p' vhdbuilder/packer/install-dependencies.sh)"

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

Describe 'getNvidiaContainerToolkitVersion'
  # Build small image layers; no container runtime or valid .deb payload is needed.
  setup_gpu_layers() {
    mkdir -p "$PWD/tmp"
    GPU_TEST_DIR=$(mktemp -d "$PWD/tmp/nvidia-toolkit.XXXXXX")
    GPU_TEST_IMAGE="mcr.microsoft.com/aks/aks-gpu-cuda-lts:test"
    GPU_TEST_DIGEST="sha256:index"
    GPU_TEST_FAIL_DIGEST=""
    CPU_ARCH="amd64"
    GPU_TEST_LAYERS='[
      {"digest":"sha256:old","mediaType":"application/vnd.oci.image.layer.v1.tar+gzip"},
      {"digest":"sha256:toolkit","mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip"},
      {"digest":"sha256:empty","mediaType":"application/vnd.oci.image.layer.v1.tar"}
    ]'
    mkdir -p "$GPU_TEST_DIR/old/opt/gpu" "$GPU_TEST_DIR/toolkit/opt/gpu" "$GPU_TEST_DIR/empty"
    touch "$GPU_TEST_DIR/old/opt/gpu/nvidia-container-toolkit_1.19.1-1_amd64.deb"
    touch "$GPU_TEST_DIR/toolkit/opt/gpu/nvidia-container-toolkit_1.20.1-1_amd64.deb"
    touch "$GPU_TEST_DIR/toolkit/opt/gpu/nvidia-container-toolkit_1.20.2-1_arm64.deb"
    touch "$GPU_TEST_DIR/toolkit/opt/gpu/nvidia-container-toolkit-base_9.9.9-1_amd64.deb"
    touch "$GPU_TEST_DIR/toolkit/opt/gpu/libnvidia-container1_9.9.9-1_amd64.deb"
    touch "$GPU_TEST_DIR/empty/entrypoint.sh"
    tar -czf "$GPU_TEST_DIR/old.tar.gz" -C "$GPU_TEST_DIR/old" opt
    tar -czf "$GPU_TEST_DIR/toolkit.tar.gz" -C "$GPU_TEST_DIR/toolkit" ./opt
    tar -cf "$GPU_TEST_DIR/empty.tar" -C "$GPU_TEST_DIR/empty" entrypoint.sh
    touch "$GPU_TEST_DIR/calls"
  }
  BeforeEach setup_gpu_layers

  # Serve cached manifests and real tar streams, and reject runtime or pull commands.
  ctr() {
    printf '%s\n' "$*" >> "$GPU_TEST_DIR/calls"
    if [ "$3 $4" = "content get" ] && [ "$5" = "$GPU_TEST_FAIL_DIGEST" ]; then
      echo "Cannot read cached content: $5" >&2
      return 23
    fi
    case "$*" in
      "-n k8s.io images list name==$GPU_TEST_IMAGE")
        echo 'REF TYPE DIGEST'
        if [ -n "$GPU_TEST_DIGEST" ]; then
          printf '%s index %s\n' "$GPU_TEST_IMAGE" "$GPU_TEST_DIGEST"
        fi
        ;;
      '-n k8s.io content get sha256:index')
        echo '{"manifests":[
          {"digest":"sha256:amd64","platform":{"os":"linux","architecture":"amd64"}},
          {"digest":"sha256:arm64","platform":{"os":"linux","architecture":"arm64"}}
        ]}'
        ;;
      '-n k8s.io content get sha256:amd64'|'-n k8s.io content get sha256:arm64')
        printf '{"layers":%s}\n' "$GPU_TEST_LAYERS"
        ;;
      '-n k8s.io content get sha256:old') cat "$GPU_TEST_DIR/old.tar.gz" ;;
      '-n k8s.io content get sha256:toolkit') cat "$GPU_TEST_DIR/toolkit.tar.gz" ;;
      '-n k8s.io content get sha256:empty') cat "$GPU_TEST_DIR/empty.tar" ;;
      *) echo "Unexpected ctr command: $*" >&2; return 1 ;;
    esac
  }

  Describe 'multi-platform images'
    Parameters
      amd64 1.20.1-1
      arm64 1.20.2-1
    End
    It "reads the actual $1 package version from the matching manifest"
      CPU_ARCH="$1"
      When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
      The status should be success
      The output should equal "$2"
      The contents of file "$GPU_TEST_DIR/calls" should include "content get sha256:$1"
      The contents of file "$GPU_TEST_DIR/calls" should not include 'content get sha256:old'
    End
  End

  It 'also reads an image with a single-platform manifest'
    GPU_TEST_DIGEST="sha256:amd64"
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be success
    The output should equal '1.20.1-1'
  End

  It 'reads toolkit packages from uncompressed layers'
    GPU_TEST_LAYERS='[{"digest":"sha256:empty","mediaType":"application/vnd.oci.image.layer.v1.tar"}]'
    tar -cf "$GPU_TEST_DIR/empty.tar" -C "$GPU_TEST_DIR/old" opt
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be success
    The output should equal '1.19.1-1'
  End

  It 'fails when the image is not cached'
    GPU_TEST_DIGEST=""
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
    The contents of file "$GPU_TEST_DIR/calls" should not include 'content get'
  End

  It 'fails when the image has no manifest for the build architecture'
    CPU_ARCH="s390x"
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
  End

  It 'fails when the cached manifest cannot be read'
    GPU_TEST_FAIL_DIGEST="sha256:amd64"
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
    The error should include 'Cannot read cached content'
  End

  It 'fails when the toolkit package is absent'
    GPU_TEST_LAYERS='[{"digest":"sha256:empty","mediaType":"application/vnd.oci.image.layer.v1.tar"}]'
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
  End

  It 'fails when a newer layer cannot be read instead of reporting an older version'
    GPU_TEST_FAIL_DIGEST="sha256:toolkit"
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
    The error should include 'Cannot read cached content'
    The contents of file "$GPU_TEST_DIR/calls" should not include 'content get sha256:old'
  End

  It 'rejects multiple toolkit versions in one layer'
    touch "$GPU_TEST_DIR/toolkit/opt/gpu/nvidia-container-toolkit_1.20.3-1_amd64.deb"
    tar -czf "$GPU_TEST_DIR/toolkit.tar.gz" -C "$GPU_TEST_DIR/toolkit" opt
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
  End

  It 'rejects unsupported layer compression instead of reporting an older version'
    GPU_TEST_LAYERS='[{"digest":"sha256:toolkit","mediaType":"application/vnd.oci.image.layer.v1.tar+unknown"}]'
    When call getNvidiaContainerToolkitVersion "$GPU_TEST_IMAGE"
    The status should be failure
    The output should be blank
    The error should include 'Unsupported GPU image layer type'
  End
End

Describe 'Ubuntu GPU release notes'
  # Keep all build output in a private directory and stub host-changing commands.
  setup_gpu_release_notes() {
    command mkdir -p "$PWD/tmp"
    GPU_TEST_DIR=$(mktemp -d "$PWD/tmp/gpu-release-notes.XXXXXX")
    VHD_LOGS_FILEPATH="$GPU_TEST_DIR/release-notes.txt"
    COMPONENTS_FILEPATH="parts/common/components.json"
    UBUNTU_OS_NAME="UBUNTU"
    OS="UBUNTU"
    UBUNTU_RELEASE="24.04"
    TEST_ARM64=0
    TEST_TOOLKIT_STATUS=0
    touch "$VHD_LOGS_FILEPATH" "$GPU_TEST_DIR/calls"
  }
  BeforeEach setup_gpu_release_notes

  isARM64() { echo "$TEST_ARM64"; }
  mkdir() { printf 'mkdir %s\n' "$*" >> "$GPU_TEST_DIR/calls"; }
  function /opt/azure/containers/image-fetcher() {
    printf 'image-fetcher %s\n' "$*" >> "$GPU_TEST_DIR/calls"
  }
  getNvidiaContainerToolkitVersion() {
    printf 'toolkit image %s\n' "$1" >> "$GPU_TEST_DIR/calls"
    [ "$TEST_TOOLKIT_STATUS" -eq 0 ] || return "$TEST_TOOLKIT_STATUS"
    echo '1.20.1-1'
  }

  Describe 'images with a GPU cache'
    Parameters
      0 22.04
      0 24.04
      0 26.04
      1 24.04
    End
    It "records the toolkit at the same indentation as CUDA-LTS for Ubuntu $2 (ARM64=$1)"
      TEST_ARM64="$1"
      UBUNTU_RELEASE="$2"
      When call cacheGPUContainerImageComponents
      The status should be success
      The contents of file "$VHD_LOGS_FILEPATH" should equal "  - nvidia-cuda-driver=$NVIDIA_DRIVER_IMAGE_TAG
  - nvidia-container-toolkit=1.20.1-1
Components installed at node provisioning time (CSE) for supported GPU VM sizes (example A10 family):
  - nvidia-grid-driver=$NVIDIA_GRID_DRIVER_VERSION"
      The variable NVIDIA_DRIVER_IMAGE should equal 'mcr.microsoft.com/aks/aks-gpu-cuda-lts'
      The contents of file "$GPU_TEST_DIR/calls" should equal "mkdir -p /opt/actions /opt/gpu
image-fetcher cache-image --image $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
toolkit image $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG"
    End
  End

  Describe 'images without a GPU cache'
    Parameters
      AZURELINUX 0 3.0
      MARINER 0 2.0
      AZURECONTAINERLINUX 0 3.0
      FLATCAR 0 4152.2.3
      UBUNTU 1 22.04
      UBUNTU 1 26.04
    End
    It "does not read toolkit metadata for $1 $3 (ARM64=$2)"
      OS="$1"
      TEST_ARM64="$2"
      UBUNTU_RELEASE="$3"
      When call cacheGPUContainerImageComponents
      The status should be success
      The contents of file "$VHD_LOGS_FILEPATH" should not include 'nvidia-container-toolkit'
      The contents of file "$GPU_TEST_DIR/calls" should be blank
    End
  End

  It 'fails instead of publishing a missing toolkit version'
    TEST_TOOLKIT_STATUS=1
    When call cacheGPUContainerImageComponents
    The status should be failure
    The error should include 'Unable to read NVIDIA Container Toolkit version'
    The contents of file "$VHD_LOGS_FILEPATH" should not include 'nvidia-container-toolkit'
  End
End
