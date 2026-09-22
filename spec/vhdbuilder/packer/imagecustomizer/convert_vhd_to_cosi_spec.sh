#!/bin/bash

# Tests for generate_cosi_package_version in
# vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh.
#
# Nebraska validates COSI package versions as strict SemVer, which rejects
# leading zeros in numeric components (e.g. 202608.06.0). These tests pin the
# normalization: leading zeros stripped, -fips appended only for FIPS builds,
# and never duplicated.

Describe 'generate_cosi_package_version'
  setup() {
    # Source only the functions (guarded by ${__SOURCED__:+return}), not the
    # main conversion flow.
    # shellcheck disable=SC1090
    __SOURCED__=1 . "./vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh"
  }
  BeforeEach 'setup'

  Describe 'strips per-component leading zeros'
    It 'normalizes day 01 (non-FIPS)'
      When call generate_cosi_package_version "202608.01.0" "false"
      The status should be success
      The output should equal "202608.1.0"
    End

    It 'normalizes day 06 (non-FIPS)'
      When call generate_cosi_package_version "202608.06.0" "false"
      The status should be success
      The output should equal "202608.6.0"
    End

    It 'leaves day 10 unchanged (non-FIPS)'
      When call generate_cosi_package_version "202608.10.0" "false"
      The status should be success
      The output should equal "202608.10.0"
    End
  End

  Describe 'appends -fips only for FIPS builds'
    It 'emits -fips when ENABLE_FIPS is true'
      When call generate_cosi_package_version "202608.06.0" "true"
      The status should be success
      The output should equal "202608.6.0-fips"
    End

    It 'omits -fips when ENABLE_FIPS is false'
      When call generate_cosi_package_version "202608.06.0" "false"
      The status should be success
      The output should equal "202608.6.0"
    End

    It 'treats ENABLE_FIPS case-insensitively (True)'
      When call generate_cosi_package_version "202608.10.2" "True"
      The status should be success
      The output should equal "202608.10.2-fips"
    End

    It 'does not duplicate -fips when the version already contains it'
      When call generate_cosi_package_version "202608.06.0-fips" "true"
      The status should be success
      The output should equal "202608.6.0-fips"
    End
  End

  Describe 'rejects a core that is not exactly three numeric components'
    It 'fails with too few components'
      When call generate_cosi_package_version "202608.06" "false"
      The status should be failure
      The error should be present
    End

    It 'fails with too many components'
      When call generate_cosi_package_version "202608.06.0.1" "false"
      The status should be failure
      The error should be present
    End

    It 'fails with a non-numeric component'
      When call generate_cosi_package_version "202608.aug.0" "false"
      The status should be failure
      The error should be present
    End
  End
End

Describe 'VHD SAS handoff'
  setup_handoff() {
    export DESTINATION_STORAGE_CONTAINER=https://storage.invalid/staging
    export CAPTURED_SIG_VERSION=test
    export STORAGE_ACCOUNT_NAME=mock
    export VHD_CONTAINER_NAME=immutable
    export MOCK_COPY_STATUS=success
    export MOCK_COMPLETE_AFTER_WAIT=0
    export MOCK_START_STATUS=0
    export MOCK_SHOW_STATUS=0
    export MOCK_SAS_STATUS=0

    az() {
      case "$*" in
        'storage blob copy start '*)
          echo COPY_STARTED
          return "$MOCK_START_STATUS"
          ;;
        'storage blob show '*)
          printf '%s\n' "$MOCK_COPY_STATUS"
          return "$MOCK_SHOW_STATUS"
          ;;
        'storage blob generate-sas '*)
          echo MOCK_SAS
          return "$MOCK_SAS_STATUS"
          ;;
        *) return 1 ;;
      esac
    }
    azcopy() { echo SOURCE_REMOVED; }
    sleep() {
      [ "$1" = 15 ] || return 1
      echo COPY_WAITED
      if [ "$MOCK_COMPLETE_AFTER_WAIT" = 1 ]; then
        MOCK_COPY_STATUS=success
      fi
    }
    export -f az azcopy sleep
  }
  BeforeEach 'setup_handoff'

  It 'waits for a successful copy before exporting the secret SAS and deleting the source'
    MOCK_COPY_STATUS=pending
    MOCK_COMPLETE_AFTER_WAIT=1
    When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
    The status should be success
    The line 3 of output should equal COPY_WAITED
    The line 4 of output should equal '##vso[task.setvariable variable=VHD_SAS_URL;isOutput=true;issecret=true]MOCK_SAS'
    The line 6 of output should equal SOURCE_REMOVED
  End

  Describe 'unsuccessful copy states'
    Parameters
      failed
      aborted
      # shellcheck disable=SC2286
      ''
    End
    It 'preserves the source when the copy is not successful'
      MOCK_COPY_STATUS="$1"
      When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
      The status should be failure
      The output should include 'VHD copy to immutable container finished with status'
      The output should not include MOCK_SAS
      The output should not include SOURCE_REMOVED
    End
  End

  It 'preserves the source when the copy stays pending until timeout'
    MOCK_COPY_STATUS=pending
    When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
    The status should be failure
    The output should include 'Timed out waiting for the immutable VHD copy'
    The output should not include MOCK_SAS
    The output should not include SOURCE_REMOVED
  End

  It 'stops when starting the copy fails'
    MOCK_START_STATUS=1
    When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
    The status should be failure
    The output should include COPY_STARTED
    The output should not include COPY_WAITED
    The output should not include SOURCE_REMOVED
  End

  It 'preserves the source when reading the copy status fails'
    MOCK_SHOW_STATUS=1
    When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
    The status should be failure
    The output should include COPY_STARTED
    The output should not include MOCK_SAS
    The output should not include SOURCE_REMOVED
  End

  It 'preserves the source when SAS minting fails'
    MOCK_SAS_STATUS=1
    When run bash ./vhdbuilder/packer/imagecustomizer/scripts/export-vhd-sas.sh
    The status should be failure
    The output should include COPY_STARTED
    The output should not include MOCK_SAS
    The output should not include SOURCE_REMOVED
  End
End

Describe 'COSI artifact names'
  setup_artifacts() {
    convert_script="$(pwd)/vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh"
    upload_script="$(pwd)/vhdbuilder/packer/imagecustomizer/scripts/upload-cosi-to-pmc.sh"
    test_dir="$(mktemp -d)"
    mkdir -p "$test_dir/bin"
    ln -s /bin/echo "$test_dir/bin/cosi-upload"
    export CAPTURED_SIG_VERSION=202609.21.0
    export IMAGE_VERSION=202609.21.0
    export DESTINATION_STORAGE_CONTAINER=https://storage.invalid/vhds
    export IMG_CUSTOMIZER_CONTAINER=mock
    export AFD_DOWNLOAD_HOSTNAME=download.invalid
    export AFD_UPLOAD_ENDPOINT=https://upload.invalid
    export COSI_CONTAINER=cosi

    azcopy() { return 0; }
    docker() {
      if [ "$1" = run ]; then
        printf 'mock COSI\n' > "$PWD/cosi-convert/out/${CAPTURED_SIG_VERSION}.cosi"
      fi
    }
    export -f azcopy docker
  }
  cleanup_artifacts() { rm -rf "$test_dir"; }
  BeforeEach 'setup_artifacts'
  AfterEach 'cleanup_artifacts'

  check_artifact_name() {
    cd "$test_dir" || return 1
    bash "$convert_script" >/dev/null || return $?
    jq -r .cosi_url cosi-publishing-info.json
    bash "$upload_script"
  }

  Describe 'variants with a shared capture version'
    Parameters
      aclgen2TL X86_64 false
      aclgen2fipsTL X86_64 true
      aclgen2arm64TL ARM64 false
      aclgen2arm64fipsTL ARM64 true
    End
    It 'uses the SKU in both the metadata URL and upload destination'
      export SKU_NAME="$1" ARCHITECTURE="$2" ENABLE_FIPS="$3"
      When call check_artifact_name
      The status should be success
      The line 1 of output should equal "https://download.invalid/cosi/${SKU_NAME}-${CAPTURED_SIG_VERSION}.cosi"
      The line 2 of output should include "--blob ${SKU_NAME}-${CAPTURED_SIG_VERSION}.cosi --file ${test_dir}/${SKU_NAME}-${CAPTURED_SIG_VERSION}.cosi"
    End
  End

  It 'rejects conversion without a SKU'
    unset SKU_NAME
    When run bash "$convert_script"
    The status should be failure
    The output should equal 'SKU_NAME was not set!'
  End

  It 'rejects upload without a SKU'
    unset SKU_NAME
    When run bash "$upload_script"
    The status should be failure
    The output should equal 'SKU_NAME was not set!'
  End
End
