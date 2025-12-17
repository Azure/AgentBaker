#!/bin/bash

Describe 'cleanup_apt_artifacts'
  Include './vhdbuilder/packer/apt-cleanup.sh'

  setup_fixture() {
    FIXTURE_ROOT=$(mktemp -d)
  }

  teardown_fixture() {
    rm -rf "${FIXTURE_ROOT}"
    unset APT_CACHE_DIR
    unset APT_LISTS_DIR
    unset UBUNTU_OS_NAME
  }

  BeforeEach 'setup_fixture'
  AfterEach 'teardown_fixture'

  It 'removes cached apt files for Ubuntu builds'
    UBUNTU_OS_NAME="UBUNTU"
    APT_CACHE_DIR="${FIXTURE_ROOT}/cache"
    APT_LISTS_DIR="${FIXTURE_ROOT}/lists"
    mkdir -p "${APT_CACHE_DIR}/archives" "${APT_LISTS_DIR}"
    touch "${APT_CACHE_DIR}/archives/pkg.deb"
    touch "${APT_LISTS_DIR}/Packages"

    When call cleanup_apt_artifacts "UBUNTU"

    The status should be success
    The stdout should match pattern "Trimming apt caches under ${APT_CACHE_DIR} and ${APT_LISTS_DIR}"
    The file "${APT_CACHE_DIR}/archives/pkg.deb" should not be exist
    The file "${APT_LISTS_DIR}/Packages" should not be exist
  End

  It 'preserves apt caches when the image is not Ubuntu'
    UBUNTU_OS_NAME="UBUNTU"
    APT_CACHE_DIR="${FIXTURE_ROOT}/cache"
    APT_LISTS_DIR="${FIXTURE_ROOT}/lists"
    mkdir -p "${APT_CACHE_DIR}/archives" "${APT_LISTS_DIR}"
    touch "${APT_CACHE_DIR}/archives/pkg.deb"
    touch "${APT_LISTS_DIR}/Packages"

    When call cleanup_apt_artifacts "MARINER"

    The status should be success
    The file "${APT_CACHE_DIR}/archives/pkg.deb" should be exist
    The file "${APT_LISTS_DIR}/Packages" should be exist
  End

  It 'succeeds when apt directories do not exist'
    UBUNTU_OS_NAME="UBUNTU"
    APT_CACHE_DIR="${FIXTURE_ROOT}/missing-cache"
    APT_LISTS_DIR="${FIXTURE_ROOT}/missing-lists"

    When call cleanup_apt_artifacts "UBUNTU"

    The status should be success
    The stdout should match pattern "Trimming apt caches under ${APT_CACHE_DIR} and ${APT_LISTS_DIR}"
  End
End
