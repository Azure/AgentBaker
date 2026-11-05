#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'VHD deprovision'
  Include ./vhdbuilder/packer/deprovision.sh

  setup() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_WORKDIR}/deprovision.XXXXXX")
    mkdir -p "${TEST_ROOT}/etc"
    printf 'ID=ubuntu\nVERSION_ID=26.04\n' > "${TEST_ROOT}/etc/os-release"
  }
  BeforeEach 'setup'

  seed_credentials() {
    mkdir -p "${TEST_ROOT}/var/lib/systemd/nvpcr" \
      "${TEST_ROOT}/run/systemd/nvpcr" \
      "${TEST_ROOT}/run/credentials/@encrypted" \
      "${TEST_ROOT}/boot/loader/credentials/nested" \
      "${TEST_ROOT}/boot/efi/loader/credentials" \
      "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.directory.cred"
    touch "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_ROOT}/run/systemd/nvpcr/nvpcr-anchor.cred" \
      "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" \
      "${TEST_ROOT}/boot/efi/loader/credentials/NvPCR-Anchor.BUILD.CRED" \
      "${TEST_ROOT}/var/lib/systemd/tpm2-srk-public-key.tpm2b_public" \
      "${TEST_ROOT}/var/lib/systemd/nvpcr/nv-index" \
      "${TEST_ROOT}/boot/loader/credentials/disk-encryption.cred" \
      "${TEST_ROOT}/boot/efi/loader/credentials/unrelated.cred" \
      "${TEST_ROOT}/boot/loader/credentials/nested/nvpcr-anchor.build.cred" \
      "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.directory.cred/marker"
    printf 'boot import\n' > "${TEST_ROOT}/run/credentials/@encrypted/nvpcr-anchor"
    printf 'symlink target\n' > "${TEST_ROOT}/anchor-target"
    ln -s "${TEST_ROOT}/anchor-target" \
      "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.link.cred"
    ln -s "${TEST_ROOT}/missing-target" \
      "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.dangling.cred"
  }

  Describe 'cleanupNvPCRCredentials'
    Describe 'OS and CVM guards'
      Parameters
        ubuntu 26.04 false
        ubuntu 24.04 true
        azurelinux 3.0 true
      End

      It "preserves credentials for $1 $2 with CVM=$3"
        seed_credentials
        printf 'ID=%s\nVERSION_ID=%s\n' "$1" "$2" > "${TEST_ROOT}/etc/os-release"

        When call cleanupNvPCRCredentials "$3" "${TEST_ROOT}"
        The status should be success
        The output should equal ""
        The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be file
        The path "${TEST_ROOT}/run/systemd/nvpcr/nvpcr-anchor.cred" should be file
        The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" should be file
        The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.link.cred" should be symlink
      End
    End

    It 'removes only top-level anchor files and symlinks on Ubuntu 26.04 CVMs'
      seed_credentials

      When call cleanupNvPCRCredentials true "${TEST_ROOT}/"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/run/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" should not be exist
      The path "${TEST_ROOT}/boot/efi/loader/credentials/NvPCR-Anchor.BUILD.CRED" should not be exist
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.link.cred" should not be symlink
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.dangling.cred" should not be symlink
      The contents of file "${TEST_ROOT}/anchor-target" should equal 'symlink target'
      The path "${TEST_ROOT}/var/lib/systemd/tpm2-srk-public-key.tpm2b_public" should be file
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nv-index" should be file
      The path "${TEST_ROOT}/boot/loader/credentials/disk-encryption.cred" should be file
      The path "${TEST_ROOT}/boot/efi/loader/credentials/unrelated.cred" should be file
      The path "${TEST_ROOT}/boot/loader/credentials/nested/nvpcr-anchor.build.cred" should be file
      The path "${TEST_ROOT}/efi/loader/credentials/nvpcr-anchor.directory.cred/marker" should be file
      The contents of file "${TEST_ROOT}/run/credentials/@encrypted/nvpcr-anchor" should equal 'boot import'
    End

    It 'succeeds when the credential directories are absent'
      When call cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
    End

    cleanup_twice() {
      cleanupNvPCRCredentials true "${TEST_ROOT}" || return $?
      cleanupNvPCRCredentials true "${TEST_ROOT}"
    }

    It 'succeeds when cleanup is repeated'
      seed_credentials

      When call cleanup_twice
      The status should be success
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" should not be exist
      The contents of file "${TEST_ROOT}/anchor-target" should equal 'symlink target'
    End

    It 'propagates persisted-anchor deletion failures before deleting boot credentials'
      seed_credentials
      rm() { echo 'anchor deletion failed' >&2; return 37; }

      When run cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should equal 37
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The stderr should equal 'anchor deletion failed'
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should be file
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" should be file
    End

    It 'propagates boot-credential traversal or deletion failures'
      seed_credentials
      find() { echo 'boot credential deletion failed' >&2; return 38; }

      When run cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should equal 38
      The output should include 'Removing build-VM NvPCR anchor credentials'
      The stderr should equal 'boot credential deletion failed'
      The path "${TEST_ROOT}/var/lib/systemd/nvpcr/nvpcr-anchor.cred" should not be exist
      The path "${TEST_ROOT}/boot/loader/credentials/nvpcr-anchor.build.cred" should be file
    End

    It 'fails visibly when OS metadata cannot be read'
      rm "${TEST_ROOT}/etc/os-release"

      When run cleanupNvPCRCredentials true "${TEST_ROOT}"
      The status should be failure
      The stderr should include '/etc/os-release'
    End
  End

  Describe 'installPinnedWALinuxAgent'
    It 'invokes the shared installer through Bash'
      sudo() { printf '%s\n' "$*"; }

      When call installPinnedWALinuxAgent "${TEST_ROOT}"
      The status should be success
      The output should equal '/bin/bash -eu /opt/azure/containers/post-deprovision-walinuxagent.sh'
    End

    It 'skips Flatcar without invoking the installer'
      printf 'ID=flatcar\n' > "${TEST_ROOT}/etc/os-release"
      sudo() { echo 'unexpected installer invocation' >&2; return 99; }

      When call installPinnedWALinuxAgent "${TEST_ROOT}"
      The status should be success
      The output should equal 'Skipping WALinuxAgent manifest install (ID=flatcar)'
    End

    It 'propagates installer failures'
      sudo() { echo 'agent installation failed' >&2; return 39; }

      When run installPinnedWALinuxAgent "${TEST_ROOT}"
      The status should equal 39
      The stderr should equal 'agent installation failed'
    End
  End

  Describe 'final deprovision hook'
    run_hook() (
      local failed_step="$1"
      shift
      record_step() {
        printf '%s\n' "$*"
        if [ "$1" = "${failed_step}" ]; then
          return 42
        fi
      }
      sudo() { record_step sudo "$@"; }
      installPinnedWALinuxAgent() { record_step installPinnedWALinuxAgent "$@"; }
      cleanupNvPCRCredentials() { record_step cleanupNvPCRCredentials "$@"; }
      rm() { record_step rm "$@"; }
      sync() { record_step sync "$@"; }
      main "$@"
    )

    Describe 'successful lifecycle'
      Parameters
        true
        false
      End

      It "generalizes, reinstalls the agent, cleans credentials and removes helpers with CVM=$1"
        When run run_hook "" "$1"
        The status should be success
        The output should equal "sudo waagent -force -deprovision+user
installPinnedWALinuxAgent /
cleanupNvPCRCredentials $1 /
rm -f /opt/azure/containers/post-deprovision-walinuxagent.sh /opt/azure/containers/install_walinuxagent.py /opt/azure/containers/deprovision.sh /tmp/DeprovisioningScript.sh
sync"
      End
    End

    It 'rejects a missing CVM argument before doing any work'
      When run run_hook ""
      The status should equal 1
      The stderr should equal 'Expected CVM build argument: true or false'
      The output should equal ""
    End

    It 'rejects an invalid CVM argument before doing any work'
      When run run_hook "" invalid
      The status should equal 1
      The stderr should equal 'Expected CVM build argument: true or false'
      The output should equal ""
    End

    Describe 'failure propagation'
      Parameters
        sudo 1
        installPinnedWALinuxAgent 2
        cleanupNvPCRCredentials 3
        rm 4
        sync 5
      End

      It "propagates $1 failure without running later steps"
        When run run_hook "$1" true
        The status should equal 42
        The lines of output should equal "$2"
        The line "$2" of output should start with "$1"
      End
    End
  End
End
