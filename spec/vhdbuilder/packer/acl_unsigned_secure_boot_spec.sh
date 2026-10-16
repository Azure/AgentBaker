#!/bin/bash
# shellcheck disable=SC2016,SC2034

Describe 'Trusted Launch Secure Boot for unsigned ACL base images'
  run_test_options() (
    set -eu
    ENABLE_TRUSTED_LAUNCH=True TARGET_COMMAND_STRING="--size Standard_D2pds_v5"
    eval "$(sed -n '/^if \[ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" \]; then$/,/^fi$/p' vhdbuilder/packer/test/run-test.sh)"
    printf '%s\n' "$TARGET_COMMAND_STRING"
  )
  vhd_scanning_options() (
    set -eu
    OS_TYPE=Linux ENABLE_TRUSTED_LAUNCH=True VM_OPTIONS="--size Standard_D8pds_v6"
    eval "$(sed -n '/^if \[ "${OS_TYPE}" = "Linux" \] && \[ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" \]; then$/,/^fi$/p' vhdbuilder/packer/vhd-scanning.sh)"
    printf '%s\n' "$VM_OPTIONS"
  )

  Parameters
    run_test_options unset 'enable-secure-boot true'
    run_test_options true 'enable-secure-boot true'
    run_test_options false 'enable-secure-boot false'
    vhd_scanning_options unset 'enable-secure-boot true'
    vhd_scanning_options true 'enable-secure-boot true'
    vhd_scanning_options false 'enable-secure-boot false'
  End
  It "$1 with ACL_BASE_IMAGE_SIGNED=$2 uses --$3"
    if [ "$2" = unset ]; then
      unset ACL_BASE_IMAGE_SIGNED
    else
      export ACL_BASE_IMAGE_SIGNED="$2"
    fi
    When run "$1"
    The status should be success
    The output should include "--security-type TrustedLaunch --$3 --enable-vtpm true"
  End
End
