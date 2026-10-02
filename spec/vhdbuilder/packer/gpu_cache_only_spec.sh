#!/bin/bash
# shellcheck disable=SC2329

Describe 'shared Ubuntu GPU installer cache'
  Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
  BeforeAll "eval \"\$(sed -n '/^configureCachedGPUDriverPrerequisites()/,/^}/p; /^buildNVIDIAKernelModule()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')\""
  BeforeAll "eval \"\$(sed -n '/^testUbuntuGPUCacheOnlyImage()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  setup_cache_only() {
    TEST_ROOT=$(mktemp -d)
    OS=UBUNTU
    UBUNTU_OS_NAME=UBUNTU
    OS_SKU=Ubuntu
    OS_VERSION=24.04
    CPU_ARCH=amd64
    IMG_SKU=server
    ENABLE_FIPS=False
    FEATURE_FLAGS=None
    COMPONENTS_FILEPATH="./parts/common/components.json"
    TEST_KERNEL=6.8.0-test-azure
    mkdir -p "$TEST_ROOT/lib/modules/$TEST_KERNEL/build" "$TEST_ROOT/boot" \
      "$TEST_ROOT/etc/modprobe.d" "$TEST_ROOT/var/lib/dkms" "$TEST_ROOT/usr/bin"
    touch "$TEST_ROOT/lib/modules/$TEST_KERNEL/build/Makefile" "$TEST_ROOT/boot/initrd.img-$TEST_KERNEL"
    printf 'blacklist nouveau\noptions nouveau modeset=0\n' > "$TEST_ROOT/etc/modprobe.d/blacklist-nouveau.conf"
    DKMS_STATUS=""
    DKMS_RC=0
    INITRAMFS_RC=0
    INITRAMFS_CONTENTS="etc/modprobe.d/blacklist-nouveau.conf"
    MISSING_PACKAGE=""
    CACHE_RC=0
    CACHE_MISSING=false
  }
  cleanup_cache_only() { rm -rf "$TEST_ROOT"; }
  BeforeEach 'setup_cache_only'
  AfterEach 'cleanup_cache_only'

  getCPUArch() { echo "$CPU_ARCH"; }
  isARM64() { if [ "$CPU_ARCH" = arm64 ]; then echo 1; else echo 0; fi; }
  err() { echo "$1:Error: $2" >&2; }
  uname() { echo "$TEST_KERNEL"; }
  dkms() { printf '%s\n' "$DKMS_STATUS"; return "$DKMS_RC"; }
  dpkg-query() {
    if [ "${*: -1}" = "$MISSING_PACKAGE" ]; then return 1; fi
    printf 'install ok installed'
  }
  lsinitramfs() { printf '%s\n' "$INITRAMFS_CONTENTS"; return "$INITRAMFS_RC"; }
  ctr() {
    if [ "$CACHE_MISSING" = false ]; then printf '%s\n' "${6#name==}"; fi
    return "$CACHE_RC"
  }
  apt_get_install() { echo "apt_get_install $*"; return "${APT_RC:-0}"; }
  update-initramfs() { echo "update-initramfs $*"; return "${UPDATE_INITRAMFS_RC:-0}"; }
  retrycmd_if_failure() { echo "unexpected driver build $*" >&2; return 1; }

  Describe 'image policy'
    Parameters
      22.04 amd64 22_04-lts-gen2 False 0
      24.04 amd64 server false 0
      24.04 arm64 server false 1
      24.04 amd64 server TRUE 1
      22.04 amd64 22_04-lts-gen2 true 1
      24.04 amd64 cvm false 1
      24.04 amd64 server-gen1 false 1
      22.04 amd64 22_04-lts false 1
      26.04 amd64 server false 1
    End
    It "classifies version=$1 arch=$2 sku=$3 fips=$4"
      When call isUbuntuGPUCacheOnlyImage "$1" "$2" "$3" "$4"
      The status should equal "$5"
    End
  End

  Describe 'build prerequisites'
    It 'installs the compiler prerequisites and refreshes every initramfs without building a driver'
      When call configureCachedGPUDriverPrerequisites "$TEST_ROOT"
      The status should be success
      The output should include "apt_get_install 10 2 300 gcc make libc6-dev"
      The output should include "update-initramfs -u -k all"
      The contents of file "$TEST_ROOT/etc/modprobe.d/blacklist-nouveau.conf" should equal "blacklist nouveau
options nouveau modeset=0"
    End

    It 'propagates package installation failure without rebuilding initramfs'
      APT_RC=1
      When call configureCachedGPUDriverPrerequisites "$TEST_ROOT"
      The status should be failure
      The output should include "apt_get_install"
      The output should not include "update-initramfs"
    End

    It 'propagates initramfs update failure'
      UPDATE_INITRAMFS_RC=1
      When call configureCachedGPUDriverPrerequisites "$TEST_ROOT"
      The status should be failure
      The output should include "update-initramfs -u -k all"
    End

    It 'does not execute host build-only with the shared-image default'
      When call buildNVIDIAKernelModule
      The status should be success
      The output should equal ""
      The stderr should equal ""
    End
  End

  Describe 'final-image invariant'
    It 'accepts the cache-only image with prebaking disabled'
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be success
      The output should include "testUbuntuGPUCacheOnlyImage:Finish"
    End

    It 'allows unrelated DKMS registrations and the in-tree framebuffer module'
      DKMS_STATUS="other/1.0, $TEST_KERNEL, x86_64: installed"
      touch "$TEST_ROOT/lib/modules/$TEST_KERNEL/nvidiafb.ko"
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be success
      The output should include ":Finish"
    End

    Describe 'stock backlight modules'
      Parameters
        nvidia-wmi-ec-backlight.ko
        nvidia-wmi-ec-backlight.ko.gz
        nvidia-wmi-ec-backlight.ko.xz
        nvidia-wmi-ec-backlight.ko.zst
      End
      It "allows $1 in the stock kernel path on disk and in initramfs"
        mkdir -p "$TEST_ROOT/lib/modules/$TEST_KERNEL/kernel/drivers/platform/x86"
        touch "$TEST_ROOT/lib/modules/$TEST_KERNEL/kernel/drivers/platform/x86/$1"
        INITRAMFS_CONTENTS="$INITRAMFS_CONTENTS
usr/lib/modules/$TEST_KERNEL/kernel/drivers/platform/x86/$1"
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be success
        The output should include ":Finish"
      End
    End

    Describe 'effective registrations'
      Parameters
        "nvidia/580.1, 6.8.0, x86_64: installed"
        "nvidia, 580.1, 6.8.0, x86_64: installed"
        "nvidia/580.1: added"
        "nvidia-current/580.1: added"
      End
      It "rejects '$1' even without a marker"
        DKMS_STATUS="$1"
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be failure
        The output should not include ":Finish"
        The stderr should include "NVIDIA DKMS registration"
      End
    End

    It 'fails closed when DKMS inspection fails'
      DKMS_RC=1
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be failure
      The output should not include ":Finish"
      The stderr should include "Cannot inspect DKMS registration"
    End

    It 'rejects dangling DKMS links even when dkms status is empty'
      ln -s "$TEST_ROOT/nonexistent" "$TEST_ROOT/var/lib/dkms/nvidia"
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be failure
      The output should not include ":Finish"
      The stderr should include "Unexpected host NVIDIA artifact"
      The path "$TEST_ROOT/var/lib/dkms/nvidia" should be symlink
    End

    Describe 'host residue'
      Parameters
        usr/bin/nvidia-modprobe
        usr/bin/lib64/libnvidia-ml.so
        usr/lib/x86_64-linux-gnu/libcuda.so.1
        opt/azure/aks-gpu/dkms-marker
        usr/src/nvidia-580.1/dkms.conf
        sys/module/nvidia/refcnt
      End
      It "rejects $1 without deleting it"
        mkdir -p "$(dirname "$TEST_ROOT/$1")"
        touch "$TEST_ROOT/$1"
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be failure
        The output should not include ":Finish"
        The stderr should include "Unexpected host NVIDIA artifact"
        The path "$TEST_ROOT/$1" should be exist
      End
    End

    Describe 'installed modules'
      Parameters
        nvidia.ko
        nvidia-modeset.ko.zst
        nvidia_uvm.ko.xz
        updates/dkms/nvidia-wmi-ec-backlight.ko
        kernel/drivers/platform/x86/nvidia.ko.zst
      End
      It "rejects $1"
        mkdir -p "$(dirname "$TEST_ROOT/lib/modules/$TEST_KERNEL/$1")"
        touch "$TEST_ROOT/lib/modules/$TEST_KERNEL/$1"
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be failure
        The output should not include ":Finish"
        The stderr should include "Unexpected host NVIDIA modules"
      End
    End

    Describe 'initramfs containing stock backlight and driver residue'
      Parameters
        nvidia.ko
        nvidia.ko.gz
        nvidia_uvm.ko.xz
        nvidia-peermem.ko.zst
        nvidia-wmi-ec-backlight.ko.zst
      End
      It "still rejects updates/dkms/$1"
        INITRAMFS_CONTENTS="$INITRAMFS_CONTENTS
usr/lib/modules/$TEST_KERNEL/kernel/drivers/platform/x86/nvidia-wmi-ec-backlight.ko.zst
usr/lib/modules/$TEST_KERNEL/updates/dkms/$1"
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be failure
        The output should not include ":Finish"
        The stderr should include "Unexpected NVIDIA module"
      End
    End

    It 'rejects NVIDIA embedded in a different installed kernel initramfs'
      touch "$TEST_ROOT/boot/initrd.img-another-kernel"
      lsinitramfs() {
        echo "etc/modprobe.d/blacklist-nouveau.conf"
        case "$1" in *another-kernel) echo "usr/lib/modules/old/updates/dkms/nvidia.ko.zst" ;; esac
      }
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be failure
      The output should not include ":Finish"
      The stderr should include "Unexpected NVIDIA module"
    End

    It 'rejects driver residue before a large initramfs listing with pipefail enabled'
      INITRAMFS_CONTENTS="$INITRAMFS_CONTENTS
usr/lib/modules/$TEST_KERNEL/updates/dkms/nvidia.ko
$(awk 'BEGIN { for (i = 0; i < 4096; i++) print "usr/lib/modules/kernel/other-module-" i ".ko" }')"
      check_with_pipefail() {
        set -o pipefail
        testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      }
      When run check_with_pipefail
      The status should be failure
      The output should not include ":Finish"
      The stderr should include "Unexpected NVIDIA module"
    End

    Describe 'missing or unverifiable prerequisites'
      Parameters
        "missing libc6-dev" "Missing GPU installation prerequisite"
        "missing headers" "Missing usable headers"
        "missing host blacklist" "Missing nouveau boot configuration"
        "missing initramfs" "Missing initramfs"
        "unreadable initramfs" "Cannot inspect initramfs"
        "missing initramfs blacklist" "Missing nouveau boot configuration in"
        "missing cache" "Missing cached GPU installer"
        "failed cache inspection" "Missing cached GPU installer"
        "invalid components" "Cannot resolve cached GPU installer reference"
      End
      It "rejects $1"
        case "$1" in
          "missing libc6-dev") MISSING_PACKAGE=libc6-dev ;;
          "missing headers") rm "$TEST_ROOT/lib/modules/$TEST_KERNEL/build/Makefile" ;;
          "missing host blacklist") printf '# no blacklist\n' > "$TEST_ROOT/etc/modprobe.d/blacklist-nouveau.conf" ;;
          "missing initramfs") rm "$TEST_ROOT/boot/initrd.img-$TEST_KERNEL" ;;
          "unreadable initramfs") INITRAMFS_RC=1 ;;
          "missing initramfs blacklist") INITRAMFS_CONTENTS="etc/modprobe.d/other.conf" ;;
          "missing cache") CACHE_MISSING=true ;;
          "failed cache inspection") CACHE_RC=1 ;;
          "invalid components") COMPONENTS_FILEPATH="$TEST_ROOT/components.json"; printf '{}\n' > "$COMPONENTS_FILEPATH" ;;
        esac
        When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
        The status should be failure
        The output should not include ":Finish"
        The stderr should include "$2"
      End
    End

    It 'cannot bypass the invariant by accidentally restoring the prebake flag'
      FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
      DKMS_STATUS="nvidia/580.1: added"
      When call testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      The status should be failure
      The output should not include ":Finish"
      The stderr should include "NVIDIA DKMS registration"
    End
  End

  Describe 'unaffected image families'
    Parameters
      UBUNTU Ubuntu 24.04 arm64 server false NVIDIA_GB
      UBUNTU Ubuntu 24.04 amd64 cvm false cvm
      UBUNTU Ubuntu 22.04 amd64 22_04-lts-gen2 True None
      AZURELINUX AzureLinux 3.0 amd64 core false None
    End
    It "does not configure or inspect OS=$1 arch=$4 sku=$5 fips=$6 flags=$7"
      OS=$1 OS_SKU=$2 OS_VERSION=$3 CPU_ARCH=$4 IMG_SKU=$5 ENABLE_FIPS=$6 FEATURE_FLAGS=$7
      DKMS_RC=1
      unaffected() {
        configureCachedGPUDriverPrerequisites "$TEST_ROOT" &&
          testUbuntuGPUCacheOnlyImage "$TEST_ROOT"
      }
      When call unaffected
      The status should be success
      The output should equal ""
      The stderr should equal ""
    End
  End

  Describe 'pipeline defaults'
    Parameters
      .pipelines/.vsts-vhd-builder.yaml build2204gen2containerd
      .pipelines/.vsts-vhd-builder.yaml build2404gen2containerd
      .pipelines/.vsts-vhd-builder-release.yaml build2204gen2containerd
      .pipelines/.vsts-vhd-builder-release.yaml build2404gen2containerd
    End
    It "keeps $2 cache-only in $1"
      build_variables() {
        awk -v job="$2" '
          $1 == "-" && $2 == "job:" { if (found) exit; found = ($3 == job); next }
          found { print }
        ' "$1"
      }
      When call build_variables "$1" "$2"
      The status should be success
      The output should include 'variable=FEATURE_FLAGS]None'
      The output should not include 'NVIDIA_CUDA_PREBAKE'
    End
  End
End
