#!/bin/bash

# Tests for ensure_sig_image_name_linux function from produce-packer-settings-functions.sh

Describe 'ensure_sig_image_name_linux function'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  # Helper function to reset environment variables
  setup_environment() {
    SIG_GALLERY_NAME=""
    SIG_IMAGE_NAME=""
    SKU_NAME=""
    IMG_OFFER=""
    ENABLE_CGROUPV2=""
    OS_SKU=""
    FEATURE_FLAGS=""
  }

  BeforeEach 'setup_environment'

  Describe 'SIG_GALLERY_NAME scenarios'
    It 'should use default gallery name when SIG_GALLERY_NAME is empty'
      SIG_GALLERY_NAME=""
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "PackerSigGalleryEastUS"
	  The output should be present
    End

    It 'should use default gallery name when SIG_GALLERY_NAME is unset'
      unset SIG_GALLERY_NAME
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "PackerSigGalleryEastUS"
	  The output should be present
    End

    It 'should use provided SIG_GALLERY_NAME when set'
      SIG_GALLERY_NAME="MyCustomGallery"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "MyCustomGallery"
	  The output should be present
    End

    It 'should preserve spaces in provided SIG_GALLERY_NAME'
      SIG_GALLERY_NAME="Gallery With Spaces"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "Gallery With Spaces"
	  The output should be present
    End
  End

  Describe 'Basic SIG_IMAGE_NAME scenarios'
    It 'should use SKU_NAME as base when SIG_IMAGE_NAME is empty'
      SIG_IMAGE_NAME=""
      SKU_NAME="ubuntu-2004"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "ubuntu-2004"
	  The output should be present
    End

    It 'should use SKU_NAME as base when SIG_IMAGE_NAME is unset'
      unset SIG_IMAGE_NAME
      SKU_NAME="test-sku"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should use provided SIG_IMAGE_NAME when set'
      SIG_IMAGE_NAME="CustomImageName"
      SKU_NAME="ignored-sku"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CustomImageName"
	  The output should be present
    End

    It 'should handle empty SKU_NAME gracefully'
      SIG_IMAGE_NAME=""
      SKU_NAME=""
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq ""
	  The output should be present
    End
  End

  Describe 'IMG_OFFER cbl-mariner scenarios'
    It 'should add CBLMariner prefix when IMG_OFFER is cbl-mariner and ENABLE_CGROUPV2 is false'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="false"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End

    It 'should add CBLMariner prefix when IMG_OFFER is cbl-mariner and ENABLE_CGROUPV2 is empty'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2=""
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End

    It 'should add AzureLinux prefix when IMG_OFFER is cbl-mariner and ENABLE_CGROUPV2 is true (lowercase)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="true"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should add AzureLinux prefix when IMG_OFFER is cbl-mariner and ENABLE_CGROUPV2 is TRUE (uppercase)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="TRUE"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should add AzureLinux prefix when IMG_OFFER is cbl-mariner and ENABLE_CGROUPV2 is True (mixed case)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="True"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive IMG_OFFER cbl-mariner (uppercase)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="CBL-MARINER"
      ENABLE_CGROUPV2="false"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive IMG_OFFER cbl-mariner (mixed case)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="Cbl-Mariner"
      ENABLE_CGROUPV2="false"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End
  End

  Describe 'IMG_OFFER azure-linux-3 scenarios'
    It 'should add AzureLinux prefix when IMG_OFFER is azure-linux-3'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="azure-linux-3"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive IMG_OFFER azure-linux-3 (uppercase)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="AZURE-LINUX-3"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive IMG_OFFER azure-linux-3 (mixed case)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="Azure-Linux-3"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should prioritize azure-linux-3 over cbl-mariner when both conditions match'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="azure-linux-3"
      ENABLE_CGROUPV2="true"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End
  End

  Describe 'OS_SKU azurelinuxosguard scenarios'
    It 'should add AzureLinuxOSGuard prefix when OS_SKU is azurelinuxosguard'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      OS_SKU="azurelinuxosguard"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxOSGuardtest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive OS_SKU azurelinuxosguard (uppercase)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      OS_SKU="AZURELINUXOSGUARD"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxOSGuardtest-sku"
	  The output should be present
    End

    It 'should handle case-insensitive OS_SKU azurelinuxosguard (mixed case)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      OS_SKU="AzureLinuxOSGuard"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxOSGuardtest-sku"
	  The output should be present
    End

    It 'should apply cbl-mariner condition first when both azurelinuxosguard and cbl-mariner match'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      OS_SKU="azurelinuxosguard"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="true"
      When call ensure_sig_image_name_linux
      The status should be success
      # cbl-mariner is checked first in the if-elif chain, so it wins
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End
  End

  Describe 'FEATURE_FLAGS cvm scenarios'
    It 'should add Specialized suffix when FEATURE_FLAGS contains cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="gpu,cvm,networking"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End

    It 'should add Specialized suffix when FEATURE_FLAGS is only cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End

    It 'should add Specialized suffix when FEATURE_FLAGS contains cvm at the beginning'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="cvm,gpu,networking"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End

    It 'should add Specialized suffix when FEATURE_FLAGS contains cvm at the end'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="gpu,networking,cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End

    It 'should not add Specialized suffix when FEATURE_FLAGS does not contain cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="gpu,networking"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should add Specialized suffix when FEATURE_FLAGS contains cvmlike (grep finds cvm substring)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="cvmlike,networking"
      When call ensure_sig_image_name_linux
      The status should be success
      # grep -q "cvm" will match "cvmlike" because it contains "cvm" as a substring
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End

    It 'should not add Specialized suffix when FEATURE_FLAGS is empty'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS=""
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should not add Specialized suffix when FEATURE_FLAGS does not contain cvm substring'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="gpu,networking,arm64"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should handle FEATURE_FLAGS with spaces correctly'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      FEATURE_FLAGS="gpu, cvm, networking"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-skuSpecialized"
	  The output should be present
    End
  End

  Describe 'Priority and combination scenarios'
    It 'should prioritize IMG_OFFER cbl-mariner over FEATURE_FLAGS cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="false"
      FEATURE_FLAGS="cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End

    It 'should prioritize IMG_OFFER azure-linux-3 over FEATURE_FLAGS cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="azure-linux-3"
      FEATURE_FLAGS="cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End

    It 'should prioritize OS_SKU azurelinuxosguard over FEATURE_FLAGS cvm'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      OS_SKU="azurelinuxosguard"
      FEATURE_FLAGS="cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "AzureLinuxOSGuardtest-sku"
	  The output should be present
    End

    It 'should handle complex combination of all conditions correctly (cbl-mariner wins)'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="true"
      OS_SKU="azurelinuxosguard"
      FEATURE_FLAGS="cvm"
      When call ensure_sig_image_name_linux
      The status should be success
      # cbl-mariner should be processed first, so we get AzureLinux prefix due to ENABLE_CGROUPV2=true
      The variable SIG_IMAGE_NAME should eq "AzureLinuxtest-sku"
	  The output should be present
    End
  End

  Describe 'Edge cases and special scenarios'
    It 'should handle unset SKU_NAME variable'
      SIG_IMAGE_NAME=""
      unset SKU_NAME
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq ""
	  The output should be present
    End

    It 'should handle unset IMG_OFFER variable'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      unset IMG_OFFER
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should handle unset ENABLE_CGROUPV2 variable with cbl-mariner'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      IMG_OFFER="cbl-mariner"
      unset ENABLE_CGROUPV2
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinertest-sku"
	  The output should be present
    End

    It 'should handle unset OS_SKU variable'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      unset OS_SKU
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should handle unset FEATURE_FLAGS variable'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      unset FEATURE_FLAGS
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should handle SKU_NAME with special characters'
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku_with-special.chars"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "test-sku_with-special.chars"
	  The output should be present
    End

    It 'should handle complex prefixes with special characters in SKU_NAME'
      SIG_IMAGE_NAME=""
      SKU_NAME="sku_with-special.chars"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="false"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_IMAGE_NAME should eq "CBLMarinersku_with-special.chars"
	  The output should be present
    End
  End

  Describe 'Function behavior verification'
    It 'should set both variables when both are empty'
      SIG_GALLERY_NAME=""
      SIG_IMAGE_NAME=""
      SKU_NAME="test-sku"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "PackerSigGalleryEastUS"
      The variable SIG_IMAGE_NAME should eq "test-sku"
	  The output should be present
    End

    It 'should not modify variables when both are provided'
      SIG_GALLERY_NAME="ProvidedGallery"
      SIG_IMAGE_NAME="ProvidedImage"
      SKU_NAME="ignored-sku"
      IMG_OFFER="cbl-mariner"
      ENABLE_CGROUPV2="true"
      When call ensure_sig_image_name_linux
      The status should be success
      The variable SIG_GALLERY_NAME should eq "ProvidedGallery"
      The variable SIG_IMAGE_NAME should eq "ProvidedImage"
	  The output should be present
    End
  End
End
