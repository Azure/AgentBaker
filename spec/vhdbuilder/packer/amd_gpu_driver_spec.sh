#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'CPU-only AMDGPU driver bake'
  setup_amd_driver() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    : > "${TRACE}"
    OS=UBUNTU OS_VERSION=24.04 CPU_ARCH=amd64 HYPERV_GENERATION=v2 ENABLE_FIPS=false
    AMD_COMPONENTS_FILEPATH="${PWD}/vhdbuilder/packer/amd-gpu-components.json"
    VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
    FAIL_STAGE=""
    PACKAGE_ARCH=all
    KERNEL=6.8.0-test-azure
    PACKAGE_VERSION=$(jq -r '.AMDGPUDriver.packageVersion' "${AMD_COMPONENTS_FILEPATH}")
    FIRMWARE_VERSION=$(jq -r '.AMDGPUDriver.firmwarePackageVersion' "${AMD_COMPONENTS_FILEPATH}")
    MODULE_VERSION=$(jq -r '.AMDGPUDriver.moduleVersion' "${AMD_COMPONENTS_FILEPATH}")
    DKMS_VERSION=$(jq -r '.AMDGPUDriver.dkmsVersion' "${AMD_COMPONENTS_FILEPATH}")
    mkdir -p "${TEST_DIR}/work" "${TEST_DIR}/marker" "${TEST_DIR}/modprobe" "${TEST_DIR}/vendor-modprobe" \
      "${TEST_DIR}/modules/${KERNEL}/build"
    touch "${TEST_DIR}/modules/${KERNEL}/build/Makefile"
    printf 'blacklist amdgpu\nblacklist nouveau\n# retain this comment\n' > "${TEST_DIR}/modprobe/cloud.conf"
    # Remap owned filesystem locations only. Real sed exercises the cloud image
    # blacklist edit; apt, GPG, module commands and network are all mocked below.
    eval "$(sed -n '/^installAMDGPUDriver()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/amd_gpu.sh |
      sed -e "s|/opt/azure/amd-gpu|${TEST_DIR}/marker|g" -e "s|/tmp/amd-gpu|${TEST_DIR}/work/amd-gpu|g" \
          -e "s|/etc/modprobe.d|${TEST_DIR}/modprobe|g" -e "s|/usr/lib/modprobe.d|${TEST_DIR}/vendor-modprobe|g" \
          -e "s|/lib/modules|${TEST_DIR}/modules|g")"
  }
  cleanup_amd_driver() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_driver
  AfterEach cleanup_amd_driver

  uname() { printf '%s\n' "${KERNEL}"; }
  retrycmd_if_failure() { shift 3; "$@"; }
  apt_get_update() { echo ubuntu-update >> "${TRACE}"; }
  apt_get_install() {
    shift 3
    echo "ubuntu-install $*" >> "${TRACE}"
    [ "${FAIL_STAGE}" != install ]
  }
  apt-mark() { echo "apt-mark $*" >> "${TRACE}"; }
  apt-get() {
    echo "apt-get $*" >> "${TRACE}"
    if [ "${FAIL_STAGE}" = metadata ]; then return 1; fi
    case " $* " in
      *' download '*) touch driver.deb firmware.deb ;;
    esac
  }
  curl() {
    echo key-download >> "${TRACE}"
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -o ]; then printf 'test-key\n' > "$2"; return; fi
      shift
    done
    return 99
  }
  gpg() {
    case " $* " in
      *' --show-keys '*)
        echo 'pub:::::::::'
        if [ "${FAIL_STAGE}" = key ]; then
          echo 'fpr:::::::::WRONG:'
        else
          echo 'fpr:::::::::CA8BB4727A47B4D09B4EE8969386B48A1A693C5C:'
        fi
        echo 'sub:::::::::'
        echo 'fpr:::::::::2B2AFE47A094DC2D1013777C30C07AF01A6D36BA:'
        ;;
      *)
        while [ "$#" -gt 0 ]; do
          if [ "$1" = --output ]; then touch "$2"; return; fi
          shift
        done
        return 99
        ;;
    esac
  }
  dpkg-deb() {
    case "$2:$3" in
      */driver.deb:Package) echo amdgpu-dkms ;;
      */firmware.deb:Package) echo amdgpu-dkms-firmware ;;
      *:Architecture) echo "${PACKAGE_ARCH:-all}" ;;
      */driver.deb:Version) if [ "${FAIL_STAGE}" = package ]; then echo wrong-version; else echo "${PACKAGE_VERSION}"; fi ;;
      */firmware.deb:Version) echo "${FIRMWARE_VERSION}" ;;
      *) return 99 ;;
    esac
  }
  dpkg-query() {
    case "${*: -1}" in
      amdgpu-dkms) echo "install ok installed ${PACKAGE_VERSION}" ;;
      amdgpu-dkms-firmware) echo "install ok installed ${FIRMWARE_VERSION}" ;;
      *) return 99 ;;
    esac
  }
  # ShellSpec parameters below also contain the literal command name.
  # shellcheck disable=SC2120
  dkms() {
    echo "dkms $*" >> "${TRACE}"
    [ "${FAIL_STAGE}" != dkms ] || return 1
    if [ "$1" = status ]; then
      echo "amdgpu/${DKMS_VERSION}, ${KERNEL}, x86_64: installed"
    fi
  }
  depmod() { echo "depmod $*" >> "${TRACE}"; }
  modinfo() {
    echo "modinfo $*" >> "${TRACE}"
    case "$4" in
      filename)
        if [ "${FAIL_STAGE}" = inbox ]; then
          echo "${TEST_DIR}/modules/${KERNEL}/kernel/drivers/gpu/drm/amd/amdgpu/amdgpu.ko"
        else
          echo "${TEST_DIR}/modules/${KERNEL}/updates/dkms/amdgpu.ko.zst"
        fi
        ;;
      version) if [ "${FAIL_STAGE}" = module ]; then echo wrong-version; else echo "${MODULE_VERSION}"; fi ;;
      vermagic) if [ "${FAIL_STAGE}" = kernel ]; then echo wrong-kernel; else echo "${KERNEL} SMP mod_unload modversions "; fi ;;
      *) return 99 ;;
    esac
  }
  modprobe() {
    echo "modprobe $*" >> "${TRACE}"
    [ "$*" = -c ] || return 99
    cat "${TEST_DIR}/modprobe/cloud.conf"
  }
  update-initramfs() {
    echo "update-initramfs $*" >> "${TRACE}"
    [ "${FAIL_STAGE}" != initramfs ]
  }
  run_installer() {
    local status=0
    # Conditional callers disable errexit: every mutation must still fail closed.
    installAMDGPUDriver || status=$?
    echo "remaining-work-files=$(find "${TEST_DIR}/work" -mindepth 1 | wc -l | tr -d ' ')"
    return "${status}"
  }

  It 'authenticates two pinned packages and validates a DKMS module without GPU hardware'
    When run run_installer
    The status should be success
    The output should eq 'remaining-work-files=0'
    The stderr should eq ''
    The contents of file "${TEST_DIR}/marker/driver.json" should include '"schema_version": 1'
    The contents of file "${TEST_DIR}/marker/driver.json" should include "\"module_version\": \"${MODULE_VERSION}\""
    The contents of file "${TRACE}" should include "download amdgpu-dkms=${PACKAGE_VERSION} amdgpu-dkms-firmware=${FIRMWARE_VERSION}"
    The contents of file "${TRACE}" should include 'Dir::Etc::sourceparts=-'
    The contents of file "${TRACE}" should include 'AllowUnauthenticated=false'
    The contents of file "${TRACE}" should include 'apt-mark manual ca-certificates curl gnupg build-essential dkms autoconf automake initramfs-tools'
    The contents of file "${TRACE}" should include 'modprobe -c'
    The contents of file "${TRACE}" should not include 'modprobe amdgpu'
    The contents of file "${TRACE}" should not include 'rocm-dev'
    The contents of file "${TEST_DIR}/modprobe/cloud.conf" should eq "blacklist nouveau
# retain this comment"
  End

  It 'rejects a key mismatch before querying the AMD repository and removes stale success state'
    FAIL_STAGE=key
    echo stale > "${TEST_DIR}/marker/driver.json"
    When run run_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'signing key fingerprint mismatch'
    The contents of file "${TRACE}" should not include 'apt-get '
    The path "${TEST_DIR}/marker/driver.json" should not be exist
  End

  Describe 'installation and validation failures'
    Parameters
      install
      metadata
      package
      dkms
      module
      kernel
      initramfs
    End
    It 'cleans temporary repository state and never writes a driver marker'
      FAIL_STAGE="$1"
      When run run_installer
      The status should be failure
      The output should eq 'remaining-work-files=0'
      The stderr should eq ''
      The path "${TEST_DIR}/marker/driver.json" should not be exist
    End
  End

  It 'rejects the inbox kernel module even when its version looks correct'
    FAIL_STAGE=inbox
    When run run_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'unexpected module'
    The path "${TEST_DIR}/marker/driver.json" should not be exist
  End

  It 'rejects an archive for the wrong architecture before installation'
    PACKAGE_ARCH=arm64
    When run run_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'Unexpected AMDGPU package architecture'
    The path "${TEST_DIR}/marker/driver.json" should not be exist
  End

  It 'rejects an effective install deny rule without removing that rule'
    printf 'install amdgpu /bin/false\n' >> "${TEST_DIR}/modprobe/cloud.conf"
    When run run_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'remains disabled'
    The contents of file "${TEST_DIR}/modprobe/cloud.conf" should include 'install amdgpu /bin/false'
    The path "${TEST_DIR}/marker/driver.json" should not be exist
  End

  Describe 'unsupported bake configurations'
    Parameters
      MARINER 24.04 amd64 v2 false
      UBUNTU 22.04 amd64 v2 false
      UBUNTU 24.04 arm64 v2 false
      UBUNTU 24.04 amd64 v1 false
      UBUNTU 24.04 amd64 v2 true
      UBUNTU 24.04 amd64 v2 True
    End
    It 'fails before package or filesystem work'
      OS="$1" OS_VERSION="$2" CPU_ARCH="$3" HYPERV_GENERATION="$4" ENABLE_FIPS="$5"
      When run run_installer
      The status should be failure
      The output should eq 'remaining-work-files=0'
      The stderr should include 'requires Ubuntu 24.04 amd64 Gen2 without FIPS'
      The contents of file "${TRACE}" should eq ''
    End
  End
End

Describe 'AMD-specific build dispatch'
  setup_amd_dispatch() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    : > "${TRACE}"
    FEATURE_FLAGS=AMD_GPU OS=UBUNTU OS_VERSION=24.04 OS_VARIANT='' IS_KATA=false
    COMPONENTS_FILEPATH="${TEST_DIR}/components.json"
    jq '{Packages: ([.Packages[] | select(.name | test("nvidia|datacenter-gpu-manager|dcgm-exporter"))] + [{name: "retained-package"}])}' \
      parts/common/components.json > "${COMPONENTS_FILEPATH}"
    eval "$(sed -n '/^isAMDGPUSkippedPackage()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/amd_gpu.sh)"
    eval "$(sed -n '/^cachePackageAndBinaryComponents()/,/^}$/p' vhdbuilder/packer/install-dependencies.sh)"
  }
  cleanup_amd_dispatch() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_dispatch
  AfterEach cleanup_amd_dispatch
  isMariner() { return 1; }
  isAzureLinux() { return 1; }
  updatePackageVersions() { jq -r .name <<< "$1" >> "${TRACE}"; PACKAGE_VERSIONS=(); }
  updatePackageDownloadURL() { PACKAGE_DOWNLOAD_URL=''; }

  It 'excludes all declared NVIDIA payloads before resolving package versions or repositories'
    When run cachePackageAndBinaryComponents
    The status should be success
    The output should include 'Skipping NVIDIA package'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq 'retained-package'
  End

  It 'continues processing NVIDIA packages on existing image SKUs'
    FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
    unset -f isAMDGPUSkippedPackage
    When run cachePackageAndBinaryComponents
    The status should be success
    The output should include 'processing components.packages'
    The stderr should eq ''
    The contents of file "${TRACE}" should include 'nvidia-device-plugin'
    The contents of file "${TRACE}" should include 'datacenter-gpu-manager-4-core'
    The contents of file "${TRACE}" should include 'retained-package'
  End
End

Describe 'Dedicated AMD image installation hook'
  setup_amd_image() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    : > "${TRACE}"
    FEATURE_FLAGS=AMD_GPU OS=UBUNTU OS_VERSION=24.04 CPU_ARCH=amd64 HYPERV_GENERATION=v2 ENABLE_FIPS=false
    SCRIPT_NAME=test-build FAIL_STAGE=''
    AMD_COMPONENTS_FILEPATH="${TEST_DIR}/amd-gpu/components.json"
    eval "$(sed -n '/^validateAMDGPUImageConfiguration()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/amd_gpu.sh)"
    eval "$(sed -n '/^installAMDGPUImage()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/amd_gpu.sh |
      sed 's/^  install /  amd_test_install /')"
  }
  cleanup_amd_image() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_image
  AfterEach cleanup_amd_image
  amd_test_install() {
    case "$2" in
      /home/packer/amd-gpu-components.json)
        echo "metadata $*" >> "${TRACE}"
        [ "${FAIL_STAGE}" != metadata ]
        ;;
      /home/packer/amd-gpu-validate.sh)
        echo "validator $*" >> "${TRACE}"
        [ "${FAIL_STAGE}" != validator ]
        ;;
      *) return 99 ;;
    esac
  }
  installAMDGPUDriver() { echo driver >> "${TRACE}"; [ "${FAIL_STAGE}" != driver ]; }
  installAMDGPUDiagnostics() { echo diagnostics >> "${TRACE}"; [ "${FAIL_STAGE}" != diagnostics ]; }
  capture_benchmark() { echo "benchmark $1" >> "${TRACE}"; [ "${FAIL_STAGE}" != benchmark ]; }

  It 'copies only dedicated metadata and the baked validator around the AMD installation'
    When run installAMDGPUImage
    The status should be success
    The output should eq ''
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "metadata -Dm0644 /home/packer/amd-gpu-components.json ${AMD_COMPONENTS_FILEPATH}
driver
benchmark test-build_build_amd_gpu_kernel_module
diagnostics
benchmark test-build_install_amd_gpu_diagnostics
validator -Dm0755 /home/packer/amd-gpu-validate.sh /opt/azure/containers/amd-gpu-validate.sh"
  End

  Describe 'incomplete image installation'
    Parameters
      metadata
      driver
      diagnostics
      benchmark
      validator
    End
    It 'propagates each installation failure'
      FAIL_STAGE="$1"
      When run installAMDGPUImage
      The status should be failure
      The output should eq ''
      The stderr should eq ''
    End
  End

  Describe 'unsupported image configuration'
    Parameters
      NVIDIA_CUDA_PREBAKE UBUNTU 24.04 amd64 v2 false
      AMD_GPU,NVIDIA_CUDA_PREBAKE UBUNTU 24.04 amd64 v2 false
      AMD_GPU UBUNTU 22.04 amd64 v2 false
      AMD_GPU MARINER 24.04 amd64 v2 false
      AMD_GPU UBUNTU 24.04 arm64 v2 false
      AMD_GPU UBUNTU 24.04 amd64 v1 false
      AMD_GPU UBUNTU 24.04 amd64 v2 True
    End
    It 'fails before changing image content'
      FEATURE_FLAGS="$1" OS="$2" OS_VERSION="$3" CPU_ARCH="$4" HYPERV_GENERATION="$5" ENABLE_FIPS="$6"
      When run installAMDGPUImage
      The status should be failure
      The output should eq ''
      The stderr should include 'requires the dedicated Ubuntu 24.04 amd64 Gen2 non-FIPS image'
      The contents of file "${TRACE}" should eq ''
    End
  End
End

Describe 'AMD installer loading does not affect existing images'
  setup_shared_bake_hook() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    : > "${TRACE}"
    AMD_SUPPORT_FILE="${TEST_DIR}/amd_gpu.sh"
    # Match the literal feature flag expression in the shared source hook.
    # shellcheck disable=SC2016
    SHARED_HOOK=$(sed -n '/^case "${FEATURE_FLAGS:-}" in$/,/^esac$/p' vhdbuilder/packer/install-dependencies.sh |
      sed "s|/home/packer/amd_gpu.sh|${AMD_SUPPORT_FILE}|g")
    [ -n "${SHARED_HOOK}" ]
  }
  cleanup_shared_bake_hook() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_shared_bake_hook
  AfterEach cleanup_shared_bake_hook
  run_shared_bake_hook() { eval "${SHARED_HOOK}"; }

  Parameters
    None missing
    None broken
    NVIDIA_CUDA_PREBAKE missing
    NVIDIA_CUDA_PREBAKE broken
  End
  It 'does not load a missing or broken AMD support file on non-AMD images'
    FEATURE_FLAGS="$1"
    if [ "$2" = broken ]; then
      printf 'echo AMD-file-was-sourced >> "%s"\nreturn 99\n' "${TRACE}" > "${AMD_SUPPORT_FILE}"
    fi
    When run run_shared_bake_hook
    The status should be success
    The output should eq ''
    The stderr should eq ''
    The contents of file "${TRACE}" should eq ''
  End
End
