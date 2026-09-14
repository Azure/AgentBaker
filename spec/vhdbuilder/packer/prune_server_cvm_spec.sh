#!/bin/bash

Describe 'Ubuntu 26.04 server-cvm package pruning'
  Include './vhdbuilder/packer/ubuntu-2604-cvm/prune-server-cvm.sh'

  setup() {
    TEST_DIR="$(mktemp -d)"
    OS_RELEASE_FILE="${TEST_DIR}/os-release"
    CANDIDATE_PACKAGES_FILE="${TEST_DIR}/candidate-packages.txt"
    REQUIRED_PACKAGES_FILE="${TEST_DIR}/required-packages.txt"
    printf 'ID=ubuntu\nVERSION_ID=\"26.04\"\n' > "${OS_RELEASE_FILE}"
    printf 'candidate-one\ncandidate-two\n' > "${CANDIDATE_PACKAGES_FILE}"
    printf 'required-one\nlinux-image-*-azure-fde\n' > "${REQUIRED_PACKAGES_FILE}"
    IMG_SKU="server-cvm"
    FEATURE_FLAGS="cvm"
  }

  cleanup() {
    rm -rf "${TEST_DIR}"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'gates pruning to Ubuntu 26.04 server-cvm with the cvm feature'
    When call should_prune_ubuntu_2604_server_cvm
    The status should be success
  End

  It 'does not prune a legacy CVM source'
    IMG_SKU="22_04-lts-cvm"
    When call should_prune_ubuntu_2604_server_cvm
    The status should be failure
  End

  It 'does not prune when cvm is only a substring of another feature'
    FEATURE_FLAGS="notcvm"
    When call should_prune_ubuntu_2604_server_cvm
    The status should be failure
  End

  It 'recognizes cvm as an exact item in a comma-separated feature list'
    FEATURE_FLAGS="preview,cvm,other"
    When call should_prune_ubuntu_2604_server_cvm
    The status should be success
  End

  It 'parses both apt removal record types'
    Data
      #|NOTE: This is only a simulation!
      #|Remv candidate-one [1.0]
      #|Purg candidate-two:amd64 [2.0]
      #|Conf unrelated (3.0 Ubuntu:26.04/resolute [amd64])
    End
    When call parse_simulated_removals
    The output should eq "$(printf 'candidate-one\ncandidate-two:amd64')"
    The status should be success
  End

  It 'accepts a removal plan containing only candidates'
    removal_file="${TEST_DIR}/removals.txt"
    essential_file="${TEST_DIR}/essential.txt"
    printf 'candidate-one\ncandidate-two:amd64\n' > "${removal_file}"
    : > "${essential_file}"
    When call validate_removal_plan "${removal_file}" "${essential_file}"
    The status should be success
  End

  It 'rejects a non-candidate removal'
    removal_file="${TEST_DIR}/removals.txt"
    essential_file="${TEST_DIR}/essential.txt"
    printf 'candidate-one\nunrelated\n' > "${removal_file}"
    : > "${essential_file}"
    When run validate_removal_plan "${removal_file}" "${essential_file}"
    The status should be failure
    The stderr should include 'non-candidate package: unrelated'
  End

  It 'rejects a required package matched by a version-independent pattern'
    printf 'linux-image-7.0.0-1011-azure-fde\n' >> "${CANDIDATE_PACKAGES_FILE}"
    removal_file="${TEST_DIR}/removals.txt"
    essential_file="${TEST_DIR}/essential.txt"
    printf 'linux-image-7.0.0-1011-azure-fde\n' > "${removal_file}"
    : > "${essential_file}"
    When run validate_removal_plan "${removal_file}" "${essential_file}"
    The status should be failure
    The stderr should include 'required CVM package'
  End

  It 'rejects an Essential package'
    removal_file="${TEST_DIR}/removals.txt"
    essential_file="${TEST_DIR}/essential.txt"
    printf 'candidate-one\n' > "${removal_file}"
    printf 'candidate-one\n' > "${essential_file}"
    When run validate_removal_plan "${removal_file}" "${essential_file}"
    The status should be failure
    The stderr should include 'Essential package'
  End
End
