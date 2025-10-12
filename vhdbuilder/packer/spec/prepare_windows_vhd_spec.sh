#shellspec shell=bash

# Tests for prepare_windows_vhd function from produce-packer-settings-functions.sh
# Note: This function has many external dependencies (jq, az, azcopy) that make comprehensive testing challenging.
# These tests focus on the core logic patterns that can be isolated and tested effectively.

Describe 'prepare_windows_vhd function'
  Include 'vhdbuilder/packer/produce-packer-settings-functions.sh'

  # Helper function to reset environment variables and create minimal mocks
  setup_environment() {
    # Input variables
    WINDOWS_SKU=""
    CDIR=""
    CREATE_TIME=""
    AZURE_MSI_RESOURCE_STRING=""
    WINDOWS_CONTAINERIMAGE_JSON_URL=""
    BUILD_ARTIFACTSTAGINGDIRECTORY=""
    WINDOWS_BASE_IMAGE_URL=""
    AZURE_RESOURCE_GROUP_NAME=""
    AZURE_LOCATION=""
    SUBSCRIPTION_ID=""
    SIG_GALLERY_NAME=""
    HYPERV_GENERATION=""
    OS_TYPE=""
    WINDOWS_NANO_IMAGE_URL=""
    WINDOWS_CORE_IMAGE_URL=""
    WINDOWS_PRIVATE_PACKAGES_URL=""

    # Output variables that the function sets
    WINDOWS_IMAGE_SKU=""
    WINDOWS_IMAGE_VERSION=""
    WINDOWS_IMAGE_NAME=""
    OS_DISK_SIZE=""
    os_disk_size_gb=""
    imported_windows_image_name=""
    windows_sigmode_source_subscription_id=""
    windows_sigmode_source_resource_group_name=""
    windows_sigmode_source_gallery_name=""
    windows_sigmode_source_image_name=""
    windows_sigmode_source_image_version=""
    windows_nanoserver_image_url=""
    windows_servercore_image_url=""
    windows_private_packages_url=""
    STORAGE_ACCOUNT_NAME=""
    IMPORTED_IMAGE_NAME=""
    IMPORTED_IMAGE_URL=""
    WINDOWS_IMAGE_URL=""
    WINDOWS_IMAGE_PUBLISHER=""
    WINDOWS_IMAGE_OFFER=""

    # Create very simple mocks to avoid external command issues
    jq() { echo "mock-value"; }
    az() { echo "success"; return 0; }
    azcopy() { return 0; }
    mkdir() { return 0; }
    sudo() { return 0; }
    basename() { echo "test.json"; }
    pwd() { echo "/mock"; }
    grep() { return 1; }
    set() { return 0; }
    shopt() { return 0; }
    cat() { return 0; }
    chmod() { return 0; }
    export() { return 0; }
  }

  BeforeEach 'setup_environment'

  Describe 'Basic function execution'
    It 'should complete successfully with minimal setup'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"

      When call prepare_windows_vhd
      The status should be success
    End

    It 'should handle empty WINDOWS_SKU without crashing'
      WINDOWS_SKU=""
      CDIR="/mock/cdir"

      When call prepare_windows_vhd
      The status should be success
    End

    It 'should set basic variables from mocked jq output'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      CREATE_TIME="20231201"
      RANDOM="12345"

      When call prepare_windows_vhd
      The status should be success
      The variable WINDOWS_IMAGE_SKU should eq "mock-value"
      The variable WINDOWS_IMAGE_VERSION should eq "mock-value"
      The variable WINDOWS_IMAGE_NAME should eq "mock-value"
    End
  End

  Describe 'Error condition testing'
    It 'should exit when WINDOWS_IMAGE_SKU is null'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"

      # Override jq to return null for the specific call that checks SKU
      jq() {
        case "$*" in
          *base_image_sku*) echo "null" ;;
          *) echo "mock-value" ;;
        esac
      }

      When run prepare_windows_vhd
      The status should equal 1
    End

    It 'should continue when WINDOWS_IMAGE_SKU is not null'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"

      # Override jq to return a valid SKU
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable WINDOWS_IMAGE_SKU should eq "valid-sku"
    End
  End

  Describe 'OS disk size logic'
    It 'should default os_disk_size_gb to 30 when OS_DISK_SIZE is null'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"

      # Override jq to return null for disk size
      jq() {
        case "$*" in
          *os_disk_size*) echo "null" ;;
          *base_image_sku*) echo "valid-sku" ;;  # Ensure we don't exit
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable OS_DISK_SIZE should eq "null"
      The variable os_disk_size_gb should eq "30"
    End

    It 'should use custom disk size when OS_DISK_SIZE is not null'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"

      # Override jq to return a custom disk size
      jq() {
        case "$*" in
          *os_disk_size*) echo "50" ;;
          *base_image_sku*) echo "valid-sku" ;;  # Ensure we don't exit
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable OS_DISK_SIZE should eq "50"
      The variable os_disk_size_gb should eq "50"
    End
  End

  Describe 'Image name generation'
    It 'should create imported_windows_image_name with CREATE_TIME and RANDOM'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      CREATE_TIME="20231201"
      RANDOM="12345"

      # Override jq to return predictable values
      jq() {
        case "$*" in
          *windows_image_name*) echo "test-image" ;;
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable WINDOWS_IMAGE_NAME should eq "test-image"
      The variable imported_windows_image_name should eq "test-image-imported-20231201-12345"
    End

    It 'should handle empty CREATE_TIME and RANDOM'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      CREATE_TIME=""
      RANDOM=""

      # Override jq to return predictable values
      jq() {
        case "$*" in
          *windows_image_name*) echo "test-image" ;;
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable imported_windows_image_name should eq "test-image-imported--"
    End
  End

  Describe 'Pipeline variable handling logic'
    It 'should set nanoserver URL from pipeline variable when not already set'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_NANO_IMAGE_URL="https://pipeline-nano.vhd"
      windows_nanoserver_image_url=""

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_nanoserver_image_url should eq "https://pipeline-nano.vhd"
    End

    It 'should not override nanoserver URL when already set'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_NANO_IMAGE_URL="https://pipeline-nano.vhd"
      windows_nanoserver_image_url="https://existing-nano.vhd"

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_nanoserver_image_url should eq "https://existing-nano.vhd"
    End

    It 'should set servercore URL from pipeline variable when not already set'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_CORE_IMAGE_URL="https://pipeline-core.vhd"
      windows_servercore_image_url=""

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_servercore_image_url should eq "https://pipeline-core.vhd"
    End

    It 'should set private packages URL from pipeline variable'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_PRIVATE_PACKAGES_URL="https://private-packages.url"

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_private_packages_url should eq "https://private-packages.url"
    End
  End

  Describe 'Variable initialization'
    It 'should initialize sig mode variables to empty strings'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_BASE_IMAGE_URL=""  # Ensure we don't process base image logic

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_sigmode_source_subscription_id should eq ""
      The variable windows_sigmode_source_resource_group_name should eq ""
      The variable windows_sigmode_source_gallery_name should eq ""
      The variable windows_sigmode_source_image_name should eq ""
      The variable windows_sigmode_source_image_version should eq ""
    End

    It 'should handle empty pipeline variables gracefully'
      WINDOWS_SKU="test-sku"
      CDIR="/mock/cdir"
      WINDOWS_NANO_IMAGE_URL=""
      WINDOWS_CORE_IMAGE_URL=""
      WINDOWS_PRIVATE_PACKAGES_URL=""

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable windows_nanoserver_image_url should eq ""
      The variable windows_servercore_image_url should eq ""
      The variable windows_private_packages_url should eq ""
    End
  End

  Describe 'Function behavior with special cases'
    It 'should handle missing environment variables gracefully'
      WINDOWS_SKU="test-sku"
      CDIR=""
      CREATE_TIME=""
      AZURE_MSI_RESOURCE_STRING=""

      # Override jq to prevent exit
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
    End

    It 'should complete successfully with complex setup'
      WINDOWS_SKU="2022-containerd"
      CDIR="/mock/cdir"
      CREATE_TIME="20231201"
      RANDOM="12345"
      AZURE_MSI_RESOURCE_STRING="mock-msi"
      WINDOWS_CONTAINERIMAGE_JSON_URL="https://example.com/images.json"
      BUILD_ARTIFACTSTAGINGDIRECTORY="/mock/staging"
      WINDOWS_NANO_IMAGE_URL="https://pipeline-nano.vhd"
      WINDOWS_CORE_IMAGE_URL="https://pipeline-core.vhd"
      WINDOWS_PRIVATE_PACKAGES_URL="https://private-packages.url"

      # Override jq to prevent exit and return consistent values
      jq() {
        case "$*" in
          *base_image_sku*) echo "valid-sku" ;;
          *windows_image_name*) echo "complex-image" ;;
          *) echo "mock-value" ;;
        esac
      }

      When call prepare_windows_vhd
      The status should be success
      The variable WINDOWS_IMAGE_SKU should eq "valid-sku"
      The variable windows_private_packages_url should eq "https://private-packages.url"
    End
  End
End
