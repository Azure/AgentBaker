#!/bin/bash

# Tests for removePackageKit from vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh
#
# Background: PackageKit ships on the non-minimal Ubuntu image and installs an apt hook
# (/etc/apt/apt.conf.d/20packagekit) that runs `gdbus call --system ... StateHasChanged` after every
# `apt-get update`. During early-boot node provisioning (CSE) the system D-Bus is not yet ready, so
# that call prints a benign `Error connecting: ... Broken pipe` to apt's stderr, which the
# node-bootstrap apt error-check misreads as a failure -> CSE exit 99 -> node never joins. These tests
# assert removePackageKit() purges packagekit (and the packages that hard-depend on it) so the hook is
# gone from the shipped VHD, and that a purge failure fails the build.
#
# tool_installs_ubuntu.sh contains {{/* ... */}} template comments that are sed-stripped at VHD build
# time (see vhdbuilder/packer/pre-install-dependencies.sh). We reproduce that strip here and eval the
# result so the ERR_* constants are assigned and the functions are defined for the test shell.

# Mock functions below are invoked indirectly by the eval'd code under test, which shellcheck cannot
# see, so it wrongly flags them as never invoked.
# shellcheck disable=SC2329

Describe 'removePackageKit'
  UBUNTU_TOOL_INSTALLS="./vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh"

  setup_remove_packagekit() {
    # Strip the build-time template comments, then define the functions and ERR_* constants.
    eval "$(sed 's/{{\/\*[^*]*\*\/}}//g' "${UBUNTU_TOOL_INSTALLS}")"

    # retrycmd_if_failure <retries> <wait> <timeout> <cmd...> -> drop the first 3 args and run cmd.
    retrycmd_if_failure() { shift 3; "$@"; }
    # Mock apt-get to emit a trace line (and succeed) so assertions can observe the purge without
    # touching real packages.
    apt-get() { echo "apt-get $*"; return 0; }
  }
  BeforeEach 'setup_remove_packagekit'

  It 'purges packagekit and the packages that hard-depend on it, with --auto-remove'
    When call removePackageKit
    The status should be success
    The output should include "apt-get purge --auto-remove packagekit packagekit-tools software-properties-common -y"
  End

  It 'fails the build when the purge errors out'
    # A failing purge (after retries) must fail the VHD build via ERR_PACKAGEKIT_PURGE, not be swallowed.
    apt-get() { return 1; }
    When run removePackageKit
    The status should be failure
    The status should eq "${ERR_PACKAGEKIT_PURGE}"
  End
End
