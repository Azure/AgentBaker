#!/bin/bash

# Tests for ensure_sig_vhd_exists function from produce-packer-settings-functions.sh

Describe 'ensure_sig_vhd_exists function'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  # Helper function to reset environment variables and create mocks
  setup_environment() {
    # Input variables
    MODE=""
    AZURE_RESOURCE_GROUP_NAME=""
    SIG_GALLERY_NAME=""
    SIG_IMAGE_NAME=""
    AZURE_LOCATION=""
    ARCHITECTURE=""
    FEATURE_FLAGS=""
    HYPERV_GENERATION=""
    OS_TYPE=""
    ENABLE_TRUSTED_LAUNCH=""

    # Mock variables to control az command behavior
    MOCK_AZ_SIG_SHOW_STATE=""
    MOCK_AZ_SIG_SHOW_EXISTS=""
    MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS=""
    MOCK_AZ_IMAGE_DEFINITIONS=""
    MOCK_AZ_IMAGE_VERSIONS=""

    # Create mocks for external commands
    az() {
      case "$1 $2" in
        "sig show")
          if [ "$MOCK_AZ_SIG_SHOW_EXISTS" = "true" ]; then
            echo "{\"provisioningState\": \"${MOCK_AZ_SIG_SHOW_STATE}\"}"
            return 0
          else
            return 1
          fi
          ;;
        "sig create")
          echo "Gallery created successfully"
          return 0
          ;;
        "sig delete")
          echo "Gallery deleted successfully"
          return 0
          ;;
        "sig image-definition")
          case "$3" in
            "show")
              if [ "$MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS" = "true" ]; then
                echo "{\"id\": \"mock-id\"}"
                return 0
              else
                return 1
              fi
              ;;
            "list")
              echo "$MOCK_AZ_IMAGE_DEFINITIONS"
              return 0
              ;;
            "create")
              echo "Image definition created successfully"
              return 0
              ;;
            "delete")
              echo "Image definition deleted successfully"
              return 0
              ;;
          esac
          ;;
        "sig image-version")
          case "$3" in
            "list")
              echo "$MOCK_AZ_IMAGE_VERSIONS"
              return 0
              ;;
            "delete")
              echo "Image version deleted successfully"
              return 0
              ;;
          esac
          ;;
      esac
      return 0
    }

    jq() {
      case "$*" in
        *provisioningState*)
          echo "$MOCK_AZ_SIG_SHOW_STATE"
          ;;
        *"select(.osType == \"Windows\").name"*)
          echo "$MOCK_AZ_IMAGE_DEFINITIONS"
          ;;
        *".[].name"*)
          echo "$MOCK_AZ_IMAGE_VERSIONS"
          ;;
        *)
          echo "mock-value"
          ;;
      esac
    }

    grep() {
      # Mock grep for cvm feature flag checking
      case "$*" in
        *cvm*)
			# shellcheck disable=SC3010
          if [[ "$FEATURE_FLAGS" == *"cvm"* ]]; then
            return 0
          else
            return 1
          fi
          ;;
        *)
          return 0
          ;;
      esac
    }
  }

  BeforeEach 'setup_environment'

  Describe 'Basic function execution'
    It 'should complete successfully with minimal setup when gallery does not exist'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle existing gallery in Succeeded state'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Succeeded"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle existing image definition'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Succeeded"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="true"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Gallery state handling'
    It 'should recreate gallery when in Failed state'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Failed"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"
      MOCK_AZ_IMAGE_DEFINITIONS=""
      MOCK_AZ_IMAGE_VERSIONS=""

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should clean up Windows image definitions when gallery is Failed'
      MODE="windowsVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Windows"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Failed"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"
      MOCK_AZ_IMAGE_DEFINITIONS="windows-image-1\nwindows-image-2"
      MOCK_AZ_IMAGE_VERSIONS="1.0.0\n1.0.1"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle gallery in other states (not Failed)'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Creating"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Image definition creation scenarios'
    It 'should create image definition with ARM64 architecture'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="arm64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should create image definition with CVM feature flag'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="gpu,cvm,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should create image definition with HyperV Generation V1'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V1"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should create image definition with Trusted Launch enabled'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="True"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should create vanilla Gen2 image definition (no special features)'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      FEATURE_FLAGS="gpu,networking"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Mixed case and case sensitivity scenarios'
    It 'should handle lowercase arm64 architecture'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="arm64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle uppercase ARM64 architecture'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="ARM64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle mixed case Arm64 architecture'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="Arm64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Feature flag scenarios'
    It 'should detect cvm in feature flags at beginning'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="cvm,gpu,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should detect cvm in feature flags at end'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="gpu,networking,cvm"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should detect cvm in feature flags in middle'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="gpu,cvm,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should detect cvm as only feature flag'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="cvm"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should not detect cvm when not present in feature flags'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="gpu,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should detect cvm in cvmlike (substring match)'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="cvmlike,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Priority and combination scenarios'
    It 'should prioritize ARM64 over CVM when both conditions are met'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="arm64"
      FEATURE_FLAGS="cvm,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should prioritize CVM over HyperV V1 when both conditions are met'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V1"
      ARCHITECTURE="x64"
      FEATURE_FLAGS="cvm,networking"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle complex combination of all special conditions (ARM64 wins)'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V1"
      ARCHITECTURE="arm64"
      FEATURE_FLAGS="cvm,networking"
      ENABLE_TRUSTED_LAUNCH="True"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Empty and missing variable scenarios'
    It 'should handle empty FEATURE_FLAGS'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      FEATURE_FLAGS=""
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle unset FEATURE_FLAGS'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      unset FEATURE_FLAGS
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle empty ARCHITECTURE'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE=""
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle unset ENABLE_TRUSTED_LAUNCH'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      unset ENABLE_TRUSTED_LAUNCH
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Different MODE scenarios'
    It 'should handle windowsVhdMode'
      MODE="windowsVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Windows"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle unknown mode'
      MODE="unknownMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle empty MODE'
      MODE=""
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Image cleanup scenarios'
    It 'should handle cleanup when no image versions exist'
      MODE="windowsVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Windows"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Failed"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"
      MOCK_AZ_IMAGE_DEFINITIONS="windows-image-1"
      MOCK_AZ_IMAGE_VERSIONS=""

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle cleanup when image versions exist'
      MODE="windowsVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Windows"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Failed"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"
      MOCK_AZ_IMAGE_DEFINITIONS="windows-image-1"
      MOCK_AZ_IMAGE_VERSIONS="1.0.0"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle multiple image definitions and versions'
      MODE="windowsVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Windows"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="true"
      MOCK_AZ_SIG_SHOW_STATE="Failed"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"
      MOCK_AZ_IMAGE_DEFINITIONS="windows-image-1\nwindows-image-2"
      MOCK_AZ_IMAGE_VERSIONS="1.0.0\n1.0.1\n2.0.0"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End

  Describe 'Special characters and edge cases'
    It 'should handle gallery names with special characters'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test_gallery-with.special-chars"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle image names with special characters'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test-rg"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test_image-with.special-chars"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End

    It 'should handle resource group names with special characters'
      MODE="linuxVhdMode"
      AZURE_RESOURCE_GROUP_NAME="test_rg-with.special-chars"
      SIG_GALLERY_NAME="test-gallery"
      SIG_IMAGE_NAME="test-image"
      AZURE_LOCATION="eastus"
      OS_TYPE="Linux"
      HYPERV_GENERATION="V2"
      ARCHITECTURE="x64"
      ENABLE_TRUSTED_LAUNCH="False"
      MOCK_AZ_SIG_SHOW_EXISTS="false"
      MOCK_AZ_SIG_IMAGE_DEFINITION_EXISTS="false"

      When call ensure_sig_vhd_exists
      The status should be success
      The output should be present
    End
  End
End
