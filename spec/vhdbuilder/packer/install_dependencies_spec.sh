#!/bin/bash
# shellcheck disable=SC2329

Describe 'installPatchedCloudInit'
  BeforeAll "eval \"\$(sed -n '/^installPatchedCloudInit()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')\""

  setup() {
    TEST_DIR=$(mktemp -d)
    mkdir -p "${TEST_DIR}/24.04" "${TEST_DIR}/26.04"
    MOCK_VERSION="26.2-0ubuntu1~24.04.2+aks2"
  }

  cleanup() {
    rm -rf "$TEST_DIR"
  }

  sha256sum() {
    echo "verify $PWD"
    return "${MOCK_CHECKSUM_STATUS:-0}"
  }

  apt_get_install() {
    printf 'install %s\n' "$@"
    return "${MOCK_INSTALL_STATUS:-0}"
  }

  dpkg-query() {
    printf '%s' "$MOCK_VERSION"
  }

  python3() {
    echo 'verify runtime override'
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'installs and verifies the Ubuntu 24.04 package'
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be success
    The output should include "verify ${TEST_DIR}/24.04"
    The output should include "install ${TEST_DIR}/24.04/cloud-init_26.2-0ubuntu1~24.04.2+aks2_all.deb"
    The output should include 'verify runtime override'
  End

  It 'installs the split Ubuntu 26.04 packages together'
    MOCK_VERSION="26.2-0ubuntu1~26.04.1+aks2"
    When call installPatchedCloudInit UBUNTU 26.04 "$TEST_DIR"
    The status should be success
    The output should include "install ${TEST_DIR}/26.04/cloud-init-base_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-azure_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-cloud-sigma_${MOCK_VERSION}_all.deb"
    The output should include "install ${TEST_DIR}/26.04/cloud-init-smart-os_${MOCK_VERSION}_all.deb"
    The output should include 'verify runtime override'
  End

  Describe 'unsupported releases'
    Parameters
      UBUNTU 22.04
      UBUNTU 20.04
      AZURELINUX 3.0
      AZURECONTAINERLINUX 3.0
    End

    It 'does not modify other OS releases'
      When call installPatchedCloudInit "$1" "$2" "$TEST_DIR"
      The status should be success
      The output should be blank
    End
  End

  It 'fails before installation if checksum verification fails'
    MOCK_CHECKSUM_STATUS=1
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'verify'
    The output should not include 'install'
  End

  It 'propagates installation failures'
    MOCK_INSTALL_STATUS=1
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'install'
    The output should not include 'verify runtime override'
  End

  It 'rejects an unexpected installed version'
    MOCK_VERSION="26.2-0ubuntu1~24.04.2"
    When call installPatchedCloudInit UBUNTU 24.04 "$TEST_DIR"
    The status should be failure
    The output should include 'install'
    The error should include 'Unexpected cloud-init version'
  End
End
