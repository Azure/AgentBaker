#!/bin/bash
# shellcheck disable=SC2329,SC2317

# ShellSpec tests for pruneStaleWALinuxAgentDirs (defined in
# vhdbuilder/packer/post-deprovision-walinuxagent.sh).
#
# Background: 'waagent -force -deprovision+user' at the end of a packer bake
# does NOT remove /var/lib/waagent/WALinuxAgent-*/ directories.
# A newer agent version the daemon fetched from wireserver at bake-VM boot
# (e.g. WALinuxAgent-2.15.2.1) survives into the captured VHD alongside the pinned version.
# The customer-node daemon then picks the highest on-disk version regardless of
# AutoUpdate.UpdateToLatestVersion=n (which only gates network fetches).
# pruneStaleWALinuxAgentDirs ensures that only the version specified in components.json remains.

Describe 'pruneStaleWALinuxAgentDirs'
  POST_DEPROV_SCRIPT="./vhdbuilder/packer/post-deprovision-walinuxagent.sh"

  # Extract just the function definition and eval it into the test shell.
  # Mirrors the pattern used by linux_vhd_content_test_helpers_spec.sh.
  BeforeAll "eval \"\$(sed -n '/^pruneStaleWALinuxAgentDirs()/,/^}$/p' \"${POST_DEPROV_SCRIPT}\")\""

  setup_waagent_dir() {
    WAAGENT_DIR="$(mktemp -d)"
  }

  cleanup_waagent_dir() {
    if [ -n "${WAAGENT_DIR:-}" ] && [ -d "${WAAGENT_DIR}" ]; then
      rm -rf "${WAAGENT_DIR}"
    fi
  }

  BeforeEach 'setup_waagent_dir'
  AfterEach 'cleanup_waagent_dir'

  It 'errors and exits non-zero when pinned version arg is empty'
    When run pruneStaleWALinuxAgentDirs "" "${WAAGENT_DIR}"
    The status should be failure
    The stderr should include "pinned version arg required"
  End

  It 'is a no-op when the base dir does not exist'
    rm -rf "${WAAGENT_DIR}"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
  End

  It 'is a no-op when the base dir is empty'
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}" should be directory
  End

  It 'preserves the pinned agent dir when nothing else is present'
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1"
    touch "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1/waagent"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1" should be directory
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1/waagent" should be file
  End

  It 'removes the stale higher version left behind by a racy deprovision'
    # Reproduces the exact TL bake state: pinned 2.15.0.1 is present, but the
    # bake VM's daemon auto-updated from wireserver to 2.15.2.1 and that dir
    # survived the failed 'waagent -deprovision+user'.
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1"
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1"
    touch "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1/waagent"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1" should be directory
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1" should not be exist
  End

  It 'removes multiple stale versions but keeps the pinned one'
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1"
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1"
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.16.0.0"
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.2.46"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1" should be directory
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1" should not be exist
    The path "${WAAGENT_DIR}/WALinuxAgent-2.16.0.0" should not be exist
    The path "${WAAGENT_DIR}/WALinuxAgent-2.2.46" should not be exist
  End

  It 'does not touch non-versioned state files, cert files, or extension dirs'
    # These are the sibling entries under /var/lib/waagent that carry runtime
    # state, certs, and extensions. None of them match the WALinuxAgent-*
    # glob, so the sweep must leave them alone even when the pinned dir is
    # also missing.
    mkdir -p "${WAAGENT_DIR}/events"
    mkdir -p "${WAAGENT_DIR}/history"
    mkdir -p "${WAAGENT_DIR}/Microsoft.Azure.Extensions.CustomScript-2.1.10"
    touch "${WAAGENT_DIR}/Certificates.pem"
    touch "${WAAGENT_DIR}/TransportPrivate.pem"
    touch "${WAAGENT_DIR}/waagent_status.json"
    touch "${WAAGENT_DIR}/partition"

    # Also drop a stale versioned dir so the sweep has to do real work while
    # ignoring everything else.
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1"

    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/events" should be directory
    The path "${WAAGENT_DIR}/history" should be directory
    The path "${WAAGENT_DIR}/Microsoft.Azure.Extensions.CustomScript-2.1.10" should be directory
    The path "${WAAGENT_DIR}/Certificates.pem" should be file
    The path "${WAAGENT_DIR}/TransportPrivate.pem" should be file
    The path "${WAAGENT_DIR}/waagent_status.json" should be file
    The path "${WAAGENT_DIR}/partition" should be file
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1" should not be exist
  End

  It 'does not recurse into subdirs that happen to contain WALinuxAgent-* names'
    # -mindepth/-maxdepth 1 guarantees only top-level entries are considered.
    # A nested WALinuxAgent-2.15.2.1 under an extension dir must NOT be touched.
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1"
    mkdir -p "${WAAGENT_DIR}/Extension.Something/WALinuxAgent-2.15.2.1"
    touch "${WAAGENT_DIR}/Extension.Something/WALinuxAgent-2.15.2.1/marker"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1" should be directory
    The path "${WAAGENT_DIR}/Extension.Something/WALinuxAgent-2.15.2.1/marker" should be file
  End

  It 'ignores a plain file named WALinuxAgent-* (only matches directories)'
    # -type d guarantees we never rm a plain file even if someone or something
    # dropped one whose name matches the glob.
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1"
    touch "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1.zip"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.0.1" should be directory
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1.zip" should be file
  End

  It 'purges every WALinuxAgent-* dir if the pinned version is not on disk'
    # If the installer had failed we would never reach this call thanks to
    # 'set -e' in the caller, but if someone invokes the helper directly and
    # the pinned version simply is not present, the correct behavior is still
    # to remove everything else — leaving zero agent dirs is a loud, visible
    # failure mode rather than a silent stale-version regression.
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1"
    mkdir -p "${WAAGENT_DIR}/WALinuxAgent-2.16.0.0"
    When call pruneStaleWALinuxAgentDirs "2.15.0.1" "${WAAGENT_DIR}"
    The status should be success
    The path "${WAAGENT_DIR}/WALinuxAgent-2.15.2.1" should not be exist
    The path "${WAAGENT_DIR}/WALinuxAgent-2.16.0.0" should not be exist
  End
End
