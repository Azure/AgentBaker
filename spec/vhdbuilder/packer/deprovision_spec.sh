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

  Describe 'deprovisionWALinuxAgent'
    # WAAGENT_OUTPUT, WAAGENT_STATUS, WAAGENT_LEFTOVER and SYSTEMCTL_STATUS control the fakes.
    sudo() {
      case "$1" in
        systemctl)
          printf '%s\n' "$*" >> "${TEST_ROOT}/calls"
          return "${SYSTEMCTL_STATUS:-0}"
          ;;
        fake-waagent)
          printf '%s\n' "$*" >> "${TEST_ROOT}/calls"
          if [ -n "${WAAGENT_OUTPUT:-}" ]; then
            printf '%s\n' "${WAAGENT_OUTPUT}"
          fi
          if [ -n "${WAAGENT_LEFTOVER:-}" ]; then
            touch "${TEST_ROOT}/lib/${WAAGENT_LEFTOVER}"
          else
            rm -rf "${TEST_ROOT:?}/lib/"*
          fi
          return "${WAAGENT_STATUS:-0}"
          ;;
        *) "$@" ;;
      esac
    }

    seed_lib_dir() {
      mkdir -p "${TEST_ROOT}/lib/events"
      touch "${TEST_ROOT}/lib/ovf-env.xml"
    }

    It 'kills Guest Configuration and stops agent units before deprovision'
      seed_lib_dir
      WAAGENT_OUTPUT='WARNING! The waagent service will be stopped.'

      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/lib"
      The status should be success
      The output should equal 'WARNING! The waagent service will be stopped.'
      The contents of file "${TEST_ROOT}/calls" should equal "systemctl disable gcd.service
systemctl kill --signal=SIGKILL gcd.service
systemctl stop gcd.service
systemctl stop walinuxagent.service
systemctl stop waagent.service
systemctl stop azure-walinuxagent-logcollector.slice
systemctl stop azure-vmextensions.slice
fake-waagent -force -deprovision+user"
    End

    It 'succeeds when the lib dir is absent'
      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/missing"
      The status should be success
      The output should equal ''
    End

    It 'continues when gcd.service and agent units do not exist'
      seed_lib_dir
      SYSTEMCTL_STATUS=5

      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/lib"
      The status should be success
      The output should equal ''
      The lines of contents of file "${TEST_ROOT}/calls" should equal 8
    End

    It 'propagates a nonzero waagent exit code'
      seed_lib_dir
      WAAGENT_STATUS=7

      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/lib"
      The status should equal 7
      The output should equal ''
    End

    It 'fails when waagent reports a failed deprovision but exits 0'
      seed_lib_dir
      WAAGENT_OUTPUT="2026/01/01 00:00:00 ERROR Failed to run 'deprovision+user': [Errno 39] Directory not empty: '/var/lib/waagent/events'"
      WAAGENT_LEFTOVER=late-event

      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/lib"
      The status should equal 1
      The output should include "Failed to run 'deprovision+user'"
      The stderr should equal 'waagent deprovision failed'
      The contents of file "${TEST_ROOT}/calls" should include 'fake-waagent -force -deprovision+user'
      The lines of contents of file "${TEST_ROOT}/calls" should equal 8
    End

    It 'fails when the lib dir is not empty after a silent deprovision'
      seed_lib_dir
      WAAGENT_LEFTOVER=late-event

      When call deprovisionWALinuxAgent fake-waagent "${TEST_ROOT}/lib"
      The status should equal 1
      The output should equal ''
      The stderr should equal "waagent deprovision left files in ${TEST_ROOT}/lib"
    End
  End

  Describe 'Packer templates'
    check_template() {
      local template="$1" helper="$2" waagent_bin="$3"
      # Every deprovision must go through the verified helper, never waagent directly.
      ! grep -F -- '-deprovision+user' "${template}" &&
        grep -F -- "'. ${helper} && deprovisionWALinuxAgent ${waagent_bin}" "${template}" >/dev/null &&
        jq -e '[.provisioners[] | select(.type == "file" and
          .source == "vhdbuilder/packer/deprovision.sh" and
          .destination == "/home/packer/deprovision.sh")] | length == 1' "${template}" >/dev/null
    }

    Parameters
      base /opt/azure/containers/deprovision.sh /usr/sbin/waagent
      cvm /opt/azure/containers/deprovision.sh /usr/sbin/waagent
      arm64-gen2 /opt/azure/containers/deprovision.sh /usr/sbin/waagent
      arm64-gb /opt/azure/containers/deprovision.sh /usr/sbin/waagent
      mariner /opt/azure/containers/deprovision.sh waagent
      mariner-cvm /opt/azure/containers/deprovision.sh waagent
      mariner-arm64 /opt/azure/containers/deprovision.sh waagent
      acl /home/packer/deprovision.sh /usr/bin/waagent
      acl-arm64 /home/packer/deprovision.sh /usr/bin/waagent
    End

    It "deprovisions $1 through the verified helper"
      When call check_template "vhdbuilder/packer/vhd-image-builder-$1.json" "$2" "$3"
      The status should be success
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
      deprovisionWALinuxAgent() { record_step deprovisionWALinuxAgent "$@"; }
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
        The output should equal "deprovisionWALinuxAgent waagent
installPinnedWALinuxAgent /
cleanupNvPCRCredentials $1 /
rm -f /opt/azure/containers/post-deprovision-walinuxagent.sh /opt/azure/containers/install_walinuxagent.py /opt/azure/containers/deprovision.sh /opt/azure/containers/aib-deprovision.sh /tmp/DeprovisioningScript.sh
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
        deprovisionWALinuxAgent 1
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
