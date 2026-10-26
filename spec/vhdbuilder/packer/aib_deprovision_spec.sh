#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'AIB deprovision hook'
  Include ./vhdbuilder/packer/aib-deprovision.sh

  setup_image() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/image"
    mkdir -p "${TEST_ROOT}/etc" \
      "${TEST_ROOT}/var/lib/systemd/nvpcr" \
      "${TEST_ROOT}/run/systemd/nvpcr" \
      "${TEST_ROOT}/boot/loader/credentials" \
      "${TEST_ROOT}/boot/efi/loader/credentials" \
      "${TEST_ROOT}/efi/loader/credentials"
    printf 'ID=ubuntu\nVERSION_ID=26.04\n' > "${TEST_ROOT}/etc/os-release"
    touch "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_ROOT}/run/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_ROOT}/var/lib/systemd/tpm2-srk-public-key.pem"
    for directory in boot boot/efi efi; do
      touch "${TEST_ROOT}/${directory}/loader/credentials/nvpcr-anchor.test.cred" \
        "${TEST_ROOT}/${directory}/loader/credentials/unrelated.cred"
    done
  }
  BeforeEach 'setup_image'

  Describe 'cleanupNvPCRCredentials'
    It 'removes only Ubuntu 26.04 CVM anchor credentials from persistent, runtime and boot locations'
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/run/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.test.cred" should not be exist
      The path "${TEST_ROOT}/boot/efi/loader/credentials/nvpcr-anchor.test.cred" should not be exist
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.test.cred" should not be exist
      The path "${TEST_ROOT}/boot/loader/credentials/unrelated.cred" should be file
      The path "${TEST_ROOT}/boot/efi/loader/credentials/unrelated.cred" should be file
      The path "${TEST_ROOT}/efi/loader/credentials/unrelated.cred" should be file
      The path "${TEST_ROOT}/var/lib/systemd/tpm2-srk-public-key.pem" should be file
    End

    It 'removes an anchor symlink without modifying its target'
      touch "${TEST_ROOT}/unrelated-secret"
      ln -s "${TEST_ROOT}/unrelated-secret" "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.link.cred"
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.link.cred" should not be exist
      The path "${TEST_ROOT}/unrelated-secret" should be file
    End

    It 'does not remove anchors from non-CVM builds'
      When call cleanupNvPCRCredentials false "${TEST_ROOT}"
      The status should be success
      The output should eq ''
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be file
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.test.cred" should be file
    End

    It 'does not remove anchors from Ubuntu 24.04'
      printf 'ID=ubuntu\nVERSION_ID=24.04\n' > "${TEST_ROOT}/etc/os-release"
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be success
      The output should eq ''
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be file
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.test.cred" should be file
    End

    It 'does not remove anchors from other operating systems'
      printf 'ID=azurelinux\nVERSION_ID=3.0\n' > "${TEST_ROOT}/etc/os-release"
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be success
      The output should eq ''
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be file
    End

    It 'fails if an anchor cannot be removed'
      rm() { return 17; }
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should eq 17
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.test.cred" should be file
    End
  End

  Describe 'installPinnedWALinuxAgent'
    sudo() { printf '%s\n' "$*"; }

    It 'runs the existing installer without tracing'
      When call installPinnedWALinuxAgent "${TEST_ROOT}"
      The status should be success
      The output should eq '/bin/bash -eu /opt/azure/containers/post-deprovision-walinuxagent.sh'
    End

    It 'skips Flatcar'
      printf 'ID=flatcar\nVERSION_ID=1\n' > "${TEST_ROOT}/etc/os-release"
      When call installPinnedWALinuxAgent "${TEST_ROOT}"
      The status should be success
      The output should eq 'Skipping WALinuxAgent manifest install (ID=flatcar)'
    End
  End

  Describe 'main'
    sudo() { echo "deprovision: $*"; return "${DEPROVISION_STATUS:-0}"; }
    installPinnedWALinuxAgent() { echo 'install'; return "${INSTALL_STATUS:-0}"; }
    cleanupNvPCRCredentials() { echo "cleanup: $*"; }
    rm() { echo 'remove scripts'; }
    sync() { echo 'sync'; }

    It 'generalizes before restoring the agent and cleaning anchors'
      When call main true
      The status should be success
      The output should eq "deprovision: /usr/sbin/waagent -force -deprovision+user
install
cleanup: true /
remove scripts
sync"
    End

    It 'passes the non-CVM flag through to cleanup'
      When call main false
      The status should be success
      The output should include 'cleanup: false /'
    End

    It 'stops if deprovisioning fails'
      DEPROVISION_STATUS=23
      When call main true
      The status should eq 23
      The output should eq 'deprovision: /usr/sbin/waagent -force -deprovision+user'
    End

    It 'stops if restoring the pinned agent fails'
      INSTALL_STATUS=19
      When call main true
      The status should eq 19
      The output should eq "deprovision: /usr/sbin/waagent -force -deprovision+user
install"
    End

    It 'rejects an invalid CVM argument before making changes'
      When call main invalid
      The status should be failure
      The stderr should eq 'Expected CVM build argument: true or false'
      The output should eq ''
    End
  End
End
