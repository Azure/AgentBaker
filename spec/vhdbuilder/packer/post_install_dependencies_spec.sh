#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2329

Describe 'ACL image version stamp'
  setup_image_version() {
    TEST_ROOT="$(mktemp -d)"
    TEST_SCRIPT="${TEST_ROOT}/image-version.sh"
    export TEST_IMAGE_VERSION_FILE="${TEST_ROOT}/containers/image-version"
    export VHD_LOGS_FILEPATH="${TEST_ROOT}/vhd-install.complete"
    export OS=AZURELINUX OS_VARIANT=AZURECONTAINERLINUX
    unset IMAGE_VERSION

    # Run only the stamp block, redirecting its output path away from the host.
    sed -n '/^if isACL "$OS" "$OS_VARIANT"; then$/,/^fi$/p' \
      ./vhdbuilder/packer/post-install-dependencies.sh |
      sed 's|^  IMAGE_VERSION_FILEPATH=/opt/azure/containers/image-version$|  IMAGE_VERSION_FILEPATH="${TEST_IMAGE_VERSION_FILE}"|' \
        > "$TEST_SCRIPT"

    isACL() {
      [ "$1" = AZURELINUX ] && [ "$2" = AZURECONTAINERLINUX ]
    }
    export -f isACL
  }

  cleanup_image_version() {
    rm -rf "$TEST_ROOT"
  }

  BeforeEach 'setup_image_version'
  AfterEach 'cleanup_image_version'

  Describe 'required input'
    Parameters
      unset
      empty
    End

    It "rejects an $1 IMAGE_VERSION without creating a stamp"
      if [ "$1" = empty ]; then
        export IMAGE_VERSION=""
      fi

      When run bash -eu "$TEST_SCRIPT"
      The status should be failure
      The error should include 'ERROR: IMAGE_VERSION must be set for ACL image builds'
      The output should be blank
      The path "$TEST_IMAGE_VERSION_FILE" should not be exist
      The path "$VHD_LOGS_FILEPATH" should not be exist
    End
  End

  Describe 'supplied input'
    Parameters
      202610.26.0
      202608.06.0
    End

    It "records the supplied version $1 unchanged"
      export IMAGE_VERSION="$1"

      When run bash -eu "$TEST_SCRIPT"
      The status should be success
      The output should be blank
      The contents of file "$TEST_IMAGE_VERSION_FILE" should equal "IMAGE_VERSION=$1"
      The contents of file "$VHD_LOGS_FILEPATH" should include "IMAGE_VERSION=$1"
    End
  End

  It 'preserves an existing stamp when IMAGE_VERSION is missing'
    mkdir -p "$(dirname "$TEST_IMAGE_VERSION_FILE")"
    printf '%s\n' 'IMAGE_VERSION=existing' > "$TEST_IMAGE_VERSION_FILE"

    When run bash -eu "$TEST_SCRIPT"
    The status should be failure
    The error should include 'ERROR: IMAGE_VERSION must be set for ACL image builds'
    The output should be blank
    The contents of file "$TEST_IMAGE_VERSION_FILE" should equal 'IMAGE_VERSION=existing'
    The path "$VHD_LOGS_FILEPATH" should not be exist
  End

  It 'does not require IMAGE_VERSION for a non-ACL image'
    export OS=UBUNTU OS_VARIANT=""

    When run bash -eu "$TEST_SCRIPT"
    The status should be success
    The output should be blank
    The path "$TEST_IMAGE_VERSION_FILE" should not be exist
    The path "$VHD_LOGS_FILEPATH" should not be exist
  End
End
