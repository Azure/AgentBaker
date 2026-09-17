#!/bin/bash

# shellcheck disable=SC1090,SC2329

Describe 'trim-2604-cvm-packages'
  TRIM_SCRIPT="./vhdbuilder/packer/ubuntu-2604-cvm/trim-2604-cvm-packages.sh"

  setup_trim() {
    TEST_DIR="$(mktemp -d)"
    source "${TRIM_SCRIPT}"
    MARKED_FOR_REMOVAL_PACKAGES_FILE="${TEST_DIR}/marked-for-removal-packages.txt"
    REQUIRED_PACKAGES_FILE="${TEST_DIR}/required-packages.txt"
  }

  cleanup_trim() {
    rm -rf "${TEST_DIR}"
  }

  BeforeEach 'setup_trim'
  AfterEach 'cleanup_trim'

  It 'filters comments and absent packages before purging'
    printf '%s\n' '# comment' remove-me absent-package '' > "${MARKED_FOR_REMOVAL_PACKAGES_FILE}"
    printf '%s\n' required-package > "${REQUIRED_PACKAGES_FILE}"

    dpkg-query() {
      for argument in "$@"; do
        package="${argument}"
      done
      case "${package}" in
        remove-me|required-package)
          echo installed
          ;;
        *)
          return 1
          ;;
      esac
    }
    apt-get() { echo "apt-get $*"; }
    apt-mark() { echo "apt-mark $*"; }

    When call main
    The status should be success
    The output should include "Purging 1 installed server-cvm packages marked for removal"
    The output should include "apt-get -o DPkg::Lock::Timeout=300 purge -y --no-auto-remove --allow-remove-essential remove-me"
    The output should not include "allow-remove-essential absent-package"
    The output should include "apt-mark manual curl gpg jq logrotate rsyslog sudo xfsprogs"
  End

  It 'fails when the removal list is empty'
    : > "${MARKED_FOR_REMOVAL_PACKAGES_FILE}"
    printf '%s\n' required-package > "${REQUIRED_PACKAGES_FILE}"

    When call main
    The status should be failure
    The error should include "Marked-for-removal package list is missing or empty"
  End

  It 'fails when a required package is missing after trimming'
    printf '%s\n' absent-package > "${MARKED_FOR_REMOVAL_PACKAGES_FILE}"
    printf '%s\n' required-package > "${REQUIRED_PACKAGES_FILE}"

    dpkg-query() { return 1; }
    apt-mark() { return 0; }

    When call main
    The status should be failure
    The output should include "No installed server-cvm packages marked for removal were found"
    The error should include "Required CVM package pattern is not installed: required-package"
  End

  It 'supports final verification without running another purge'
    printf '%s\n' remove-me > "${MARKED_FOR_REMOVAL_PACKAGES_FILE}"
    printf '%s\n' required-package > "${REQUIRED_PACKAGES_FILE}"

    dpkg-query() {
      echo installed
    }
    apt-get() {
      echo "unexpected apt-get"
      return 1
    }
    apt-mark() {
      echo "unexpected apt-mark"
      return 1
    }

    When call main --verify-only
    The status should be success
    The output should not include "unexpected"
  End

  It 'fails final verification when the required package list is empty'
    : > "${REQUIRED_PACKAGES_FILE}"

    When call main --verify-only
    The status should be failure
    The error should include "Required package list is missing or empty"
  End

  It 'rejects unknown arguments'
    When call main --unknown
    The status should be failure
    The error should include "Unknown argument: --unknown"
  End
End
