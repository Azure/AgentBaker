#!/bin/bash

# shellcheck disable=SC1090,SC2329

Describe 'AIB final deprovisioning'
  Include ./vhdbuilder/packer/aib-deprovision.sh

  setup_deprovision() {
    TEST_DIR="$(mktemp -d)"
    mkdir -p "${TEST_DIR}/etc" "${TEST_DIR}/var/lib/systemd/nvpcr" \
      "${TEST_DIR}/run/systemd/nvpcr" "${TEST_DIR}/run/credentials/@encrypted" \
      "${TEST_DIR}/boot/loader/credentials" "${TEST_DIR}/boot/efi/loader/credentials" \
      "${TEST_DIR}/efi/loader/credentials"
    printf '%s\n' 'ID=ubuntu' 'VERSION_ID="26.04"' > "${TEST_DIR}/etc/os-release"
    touch "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_DIR}/run/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_DIR}/run/credentials/@encrypted/nvpcr-anchor.build" \
      "${TEST_DIR}/boot/loader/credentials/nvpcr-anchor.build.cred" \
      "${TEST_DIR}/boot/efi/loader/credentials/nvpcr-anchor.build.cred" \
      "${TEST_DIR}/efi/loader/credentials/NVPCR-ANCHOR.BUILD.CRED"
  }

  cleanup_deprovision() {
    /bin/rm -rf "${TEST_DIR:?}"
  }

  BeforeEach 'setup_deprovision'
  AfterEach 'cleanup_deprovision'

  Describe 'NvPCR credentials'
    It 'removes only anchor credentials on Ubuntu 26.04 CVM'
      touch "${TEST_DIR}/var/lib/systemd/nvpcr/hardware.auth" \
        "${TEST_DIR}/var/lib/systemd/nvpcr/hardware.anchor" \
        "${TEST_DIR}/boot/efi/loader/credentials/disk-key.cred" \
        "${TEST_DIR}/run/credentials/@encrypted/unrelated.cred"
      mkdir -p "${TEST_DIR}/var/lib/systemd/tpm2" \
        "${TEST_DIR}/boot/loader/credentials/nested"
      touch "${TEST_DIR}/var/lib/systemd/tpm2/srk-public.pem" \
        "${TEST_DIR}/boot/loader/credentials/nested/nvpcr-anchor.build.cred"

      When call cleanupNvPCRCredentials true "${TEST_DIR}"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_DIR}/run/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_DIR}/run/credentials/@encrypted/nvpcr-anchor.build" should be exist
      The path "${TEST_DIR}/boot/loader/credentials/nvpcr-anchor.build.cred" should not be exist
      The path "${TEST_DIR}/boot/efi/loader/credentials/nvpcr-anchor.build.cred" should not be exist
      The path "${TEST_DIR}/efi/loader/credentials/NVPCR-ANCHOR.BUILD.CRED" should not be exist
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/hardware.auth" should be exist
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/hardware.anchor" should be exist
      The path "${TEST_DIR}/var/lib/systemd/tpm2/srk-public.pem" should be exist
      The path "${TEST_DIR}/boot/efi/loader/credentials/disk-key.cred" should be exist
      The path "${TEST_DIR}/run/credentials/@encrypted/unrelated.cred" should be exist
      The path "${TEST_DIR}/boot/loader/credentials/nested/nvpcr-anchor.build.cred" should be exist
    End

    It 'leaves non-CVM Ubuntu 26.04 untouched'
      When call cleanupNvPCRCredentials false "${TEST_DIR}"
      The status should be success
      The output should be blank
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be exist
      The path "${TEST_DIR}/boot/efi/loader/credentials/nvpcr-anchor.build.cred" should be exist
    End

    It 'leaves older Ubuntu CVM images untouched'
      printf '%s\n' 'ID=ubuntu' 'VERSION_ID="24.04"' > "${TEST_DIR}/etc/os-release"
      When call cleanupNvPCRCredentials true "${TEST_DIR}"
      The status should be success
      The output should be blank
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be exist
      The path "${TEST_DIR}/boot/efi/loader/credentials/nvpcr-anchor.build.cred" should be exist
    End

    It 'leaves Azure Linux CVM images untouched'
      printf '%s\n' 'ID=azurelinux' 'VERSION_ID="3.0"' > "${TEST_DIR}/etc/os-release"
      When call cleanupNvPCRCredentials true "${TEST_DIR}"
      The status should be success
      The output should be blank
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be exist
    End

    It 'succeeds when no anchor credentials exist'
      /bin/rm -rf "${TEST_DIR:?}/var" "${TEST_DIR:?}/run" "${TEST_DIR:?}/boot" "${TEST_DIR:?}/efi"
      When call cleanupNvPCRCredentials true "${TEST_DIR}"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
    End

    It 'propagates credential removal failures'
      rm() { echo 'credential removal failed' >&2; return 1; }
      When call cleanupNvPCRCredentials true "${TEST_DIR}"
      The status should be failure
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The error should include 'credential removal failed'
      The path "${TEST_DIR}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be exist
    End
  End

  Describe 'agent installation'
    sudo() { echo "$*"; }

    It 'reuses the existing post-deprovision installer without tracing'
      When call installPinnedWALinuxAgent "${TEST_DIR}"
      The status should be success
      The output should equal '/bin/bash -eu /opt/azure/containers/post-deprovision-walinuxagent.sh'
    End

    It 'retains the Flatcar exclusion for the shared AIB template'
      printf '%s\n' 'ID=flatcar' 'VERSION_ID="4459.2.0"' > "${TEST_DIR}/etc/os-release"
      When call installPinnedWALinuxAgent "${TEST_DIR}"
      The status should be success
      The output should equal 'Skipping WALinuxAgent manifest install (ID=flatcar)'
    End

    It 'fails if the existing installer fails'
      sudo() { return 9; }
      When call installPinnedWALinuxAgent "${TEST_DIR}"
      The status should equal 9
    End
  End

  Describe 'finalization ordering'
    sudo() { echo "$*"; }
    installPinnedWALinuxAgent() { echo 'install pinned agent'; }
    cleanupNvPCRCredentials() { echo "cleanup NvPCR $1"; }
    rm() { echo 'remove build scripts'; }
    sync() { echo 'sync'; }

    It 'generalizes before repinning and cleans credentials before syncing'
      When call main true
      The status should be success
      The output should equal "$(printf '%s\n' \
        '/usr/sbin/waagent -force -deprovision+user' \
        'install pinned agent' 'cleanup NvPCR true' 'remove build scripts' 'sync')"
      The variable HISTSIZE should equal 0
    End

    It 'does not repin or clean when waagent deprovisioning fails'
      sudo() { echo 'deprovision failed' >&2; return 7; }
      When call main true
      The status should equal 7
      The output should be blank
      The error should equal 'deprovision failed'
    End

    It 'does not capture success when repinning fails'
      installPinnedWALinuxAgent() { echo 'pin install failed' >&2; return 9; }
      When call main true
      The status should equal 9
      The output should equal '/usr/sbin/waagent -force -deprovision+user'
      The error should equal 'pin install failed'
    End

    It 'does not capture success when credential cleanup fails'
      cleanupNvPCRCredentials() { echo 'credential cleanup failed' >&2; return 8; }
      When call main true
      The status should equal 8
      The output should equal "$(printf '%s\n' \
        '/usr/sbin/waagent -force -deprovision+user' 'install pinned agent')"
      The error should equal 'credential cleanup failed'
    End

    It 'rejects a missing CVM build argument before deprovisioning'
      When call main
      The status should be failure
      The output should be blank
      The error should equal 'Expected CVM build argument: true or false'
    End
  End
End
