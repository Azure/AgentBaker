#!/bin/bash

Describe 'Linux VHD flavor naming and AMD GPU build constraints'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  setup_environment() {
    OS_SKU=Ubuntu
    OS_VERSION=24.04
    ARCHITECTURE=X86_64
    HYPERV_GENERATION=V2
    FEATURE_FLAGS=AMD_GPU
    ENABLE_FIPS=False
    ENABLE_TRUSTED_LAUNCH=False
    TRUSTED_LAUNCH_SUPPORTED=False
    SKU_NAME=''
    SIG_IMAGE_NAME=''
    SIG_GALLERY_NAME=''
    IMG_OFFER=ubuntu-24_04-lts
  }
  BeforeEach 'setup_environment'

  It 'uses a dedicated AMD GPU SKU instead of the shared Ubuntu image'
    When call get_linux_sku_name
    The status should be success
    The output should equal '2404gen2amdgpucontainerd'
  End

  derive_sig_name() {
    SKU_NAME=$(get_linux_sku_name) || return 1
    ensure_sig_image_name_linux
  }

  It 'uses the dedicated SKU as the default staging gallery image definition'
    When call derive_sig_name
    The status should be success
    The variable SIG_IMAGE_NAME should equal '2404gen2amdgpucontainerd'
    The variable SIG_GALLERY_NAME should equal 'PackerSigGalleryEastUS'
    The output should be present
  End

  Describe 'unsupported AMD configurations'
    Parameters
      OS_SKU AzureLinux
      OS_VERSION 22.04
      OS_VERSION 26.04
      ARCHITECTURE ARM64
      HYPERV_GENERATION V1
      ENABLE_FIPS True
      ENABLE_TRUSTED_LAUNCH True
      TRUSTED_LAUNCH_SUPPORTED True
      FEATURE_FLAGS 'AMD_GPU,NVIDIA_CUDA_PREBAKE'
      FEATURE_FLAGS 'AMD_GPU,NVIDIA_GB'
      FEATURE_FLAGS 'AMD_GPU,cvm'
      FEATURE_FLAGS 'AMD_GPU,minimal'
      FEATURE_FLAGS 'NOT_AMD_GPU'
    End

    It 'rejects the unsupported AMD image configuration'
      printf -v "$1" '%s' "$2"
      When call get_linux_sku_name
      The status should be failure
      The stderr should include 'AMD_GPU requires Ubuntu 24.04 x86_64 Gen2'
      The output should be blank
    End
  End

  It 'rejects reusing the existing shared SKU for an AMD build'
    SKU_NAME=2404gen2containerd
    When call validate_amd_gpu_build
    The status should be failure
    The stderr should include 'requires the dedicated SKU_NAME'
  End

  It 'accepts the generated dedicated SKU during Packer settings generation'
    SKU_NAME=2404gen2amdgpucontainerd
    When call validate_amd_gpu_build
    The status should be success
    The output should be blank
  End

  It 'rejects overriding the capture definition with the existing shared image'
    SKU_NAME=2404gen2amdgpucontainerd
    SIG_IMAGE_NAME=2404gen2containerd
    When call validate_amd_gpu_build
    The status should be failure
    The stderr should include 'requires the dedicated SIG_IMAGE_NAME'
  End

  It 'accepts an explicit dedicated capture definition'
    SKU_NAME=2404gen2amdgpucontainerd
    SIG_IMAGE_NAME=2404gen2amdgpucontainerd
    When call validate_amd_gpu_build
    The status should be success
    The output should be blank
  End

  # Preserve the existing public naming contract when extracting the pipeline logic.
  Describe 'existing image flavors'
    Parameters
      Ubuntu 22.04 X86_64 V1 None False False 2204containerd
      Ubuntu 22.04 X86_64 V2 NVIDIA_CUDA_PREBAKE False False 2204gen2containerd
      Ubuntu 24.04 X86_64 V2 NVIDIA_CUDA_PREBAKE False False 2404gen2containerd
      Ubuntu 24.04 ARM64 V2 NVIDIA_GB False False 2404gen2arm64gbcontainerd
      Ubuntu 22.04 X86_64 V2 None True True 2204gen2fipsTLcontainerd
      Ubuntu 24.04 X86_64 V2 cvm False False 2404gen2CVMcontainerd
      Ubuntu 26.04 ARM64 V2 minimal False False 2604minimalgen2arm64containerd
      AzureLinux V3kata X86_64 V2 kata False False V3katagen2
      AzureLinuxOSGuard V3 X86_64 V2 None True True V3gen2fipsTL
      CBLMariner V2 X86_64 V1 None False False V2
      Flatcar '' ARM64 V2 None False False gen2arm64
      AzureContainerLinux acl ARM64 V2 None True True aclgen2arm64fipsTL
    End

    It 'preserves the existing SKU name'
      OS_SKU="$1"
      OS_VERSION="$2"
      ARCHITECTURE="$3"
      HYPERV_GENERATION="$4"
      FEATURE_FLAGS="$5"
      ENABLE_FIPS="$6"
      ENABLE_TRUSTED_LAUNCH="$7"
      When call get_linux_sku_name
      The status should be success
      The output should equal "$8"
    End
  End
End
