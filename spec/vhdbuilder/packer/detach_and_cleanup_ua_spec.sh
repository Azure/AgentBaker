#!/bin/bash

# Tests for detachAndCleanUpUA / disableAndMaskUbuntuProUnit from
# vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh
#
# Background: the Ubuntu 20.04 / FIPS VHD must ship with Ubuntu Pro made INERT (no phone-home
# to esm.ubuntu.com or contracts.canonical.com) while preserving the installed FIPS kernel.
# These tests assert that detachAndCleanUpUA() removes the apt ESM hook, masks the Pro
# background units, and removes the baked-in machine token, without ever running 'ua detach'.
#
# tool_installs_ubuntu.sh contains {{/* ... */}} template comments that are sed-stripped at VHD
# build time (see vhdbuilder/packer/pre-install-dependencies.sh). We reproduce that strip here
# and eval the result so the ERR_* constants are assigned correctly and the functions are
# defined for the test shell.

# Mock functions below are invoked indirectly by the eval'd code under test, which shellcheck
# cannot see, so it wrongly flags them as never invoked.
# shellcheck disable=SC2329

Describe 'detachAndCleanUpUA'
  UBUNTU_TOOL_INSTALLS="./vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh"

  setup_detach_ua() {
    # Strip the build-time template comments, then define the functions and ERR_* constants.
    eval "$(sed 's/{{\/\*[^*]*\*\/}}//g' "${UBUNTU_TOOL_INSTALLS}")"

    # ERR_APT_UPDATE_TIMEOUT is defined in cse_helpers.sh, not in this file. Give it a value so
    # the final 'apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT' has something to reference.
    ERR_APT_UPDATE_TIMEOUT=25

    # Mocks. Each emits a trace line so assertions can observe the side effects without touching
    # the real filesystem, systemd or apt.
    ua() { echo "ua $*"; return 0; }
    apt_get_update() { echo "apt_get_update"; return 0; }
    # retrycmd_if_failure <retries> <wait> <timeout> <cmd...> -> drop the first 3 args and run cmd.
    retrycmd_if_failure() { shift 3; "$@"; }
    rm() { echo "rm $*"; return 0; }
  }

  Describe 'when the Ubuntu Pro background units are present'
    setup_units_present() {
      setup_detach_ua
      # 'systemctl cat <unit>' returns 0 => unit is considered present on the image.
      systemctl() { echo "systemctl $*"; return 0; }
    }
    BeforeEach 'setup_units_present'

    It 'succeeds and makes Ubuntu Pro inert without detaching'
      When call detachAndCleanUpUA
      The status should be success
      # FIPS preservation: we must NOT run 'ua detach'.
      The output should not include "ua detach"
      # esm.ubuntu.com fix: remove the apt ESM hook that restarts esm-cache on every apt update.
      The output should include "rm -f /etc/apt/apt.conf.d/20apt-esm-hook.conf"
      # Mask all four Pro background units so they cannot phone home on a customer node.
      The output should include "systemctl mask esm-cache.service"
      The output should include "systemctl mask apt-news.service"
      The output should include "systemctl mask ua-timer.timer"
      The output should include "systemctl mask ua-timer.service"
      # Each masked unit is also stopped and disabled.
      The output should include "systemctl stop esm-cache.service"
      The output should include "systemctl disable ua-timer.timer"
      # Security: remove the baked-in Ubuntu Pro machine token / private state.
      The output should include "rm -rf /var/lib/ubuntu-advantage/private"
      # The final apt update still runs (last, after the hook is removed and esm-cache masked).
      The output should include "apt_get_update"
    End

    # Ordering guarantee: the hook removal and esm-cache mask must both appear before the final
    # apt_get_update, otherwise that apt update would re-trigger esm-cache and re-establish the
    # esm.ubuntu.com traffic. We assert ordering by checking the line numbers of the trace.
    assert_hook_and_mask_precede_apt_update() {
      out="$(detachAndCleanUpUA)" || return 1
      hook_line=$(printf '%s\n' "$out" | grep -n '20apt-esm-hook.conf' | head -1 | cut -d: -f1)
      mask_line=$(printf '%s\n' "$out" | grep -n 'mask esm-cache.service' | head -1 | cut -d: -f1)
      update_line=$(printf '%s\n' "$out" | grep -n 'apt_get_update' | tail -1 | cut -d: -f1)
      [ "${hook_line}" -lt "${update_line}" ] || return 1
      [ "${mask_line}" -lt "${update_line}" ] || return 1
      echo "ordering ok"
    }

    It 'removes the apt ESM hook before masking esm-cache (so apt update cannot re-trigger it)'
      When call assert_hook_and_mask_precede_apt_update
      The status should be success
      The output should include "ordering ok"
    End
  End

  Describe 'when an Ubuntu Pro unit is absent on the image'
    setup_units_absent() {
      setup_detach_ua
      # 'systemctl cat <unit>' returns 1 => unit not present; masking must be skipped gracefully.
      systemctl() {
        if [ "$1" = "cat" ]; then
          return 1
        fi
        echo "systemctl $*"
        return 0
      }
    }
    BeforeEach 'setup_units_absent'

    It 'skips masking missing units but still succeeds and cleans up'
      When call detachAndCleanUpUA
      The status should be success
      # No unit was present, so nothing should be masked.
      The output should not include "systemctl mask"
      # Hook removal and token cleanup still happen regardless of unit presence.
      The output should include "rm -f /etc/apt/apt.conf.d/20apt-esm-hook.conf"
      The output should include "rm -rf /var/lib/ubuntu-advantage/private"
      The output should include "not present on this image, skipping"
    End
  End

  Describe 'disableAndMaskUbuntuProUnit helper'
    BeforeEach 'setup_detach_ua'

    It 'returns success and skips when the unit does not exist'
      systemctl() {
        if [ "$1" = "cat" ]; then
          return 1
        fi
        echo "systemctl $*"
        return 0
      }
      When call disableAndMaskUbuntuProUnit ua-timer.timer
      The status should be success
      The output should not include "systemctl mask"
      The output should include "not present on this image, skipping"
    End

    It 'stops, disables and masks a unit that exists'
      systemctl() { echo "systemctl $*"; return 0; }
      When call disableAndMaskUbuntuProUnit ua-timer.timer
      The status should be success
      The output should include "systemctl stop ua-timer.timer"
      The output should include "systemctl disable ua-timer.timer"
      The output should include "systemctl mask ua-timer.timer"
    End

    It 'fails when masking a present unit errors out'
      systemctl() {
        case "$1" in
          cat) return 0 ;;
          mask) return 1 ;;
          *) return 0 ;;
        esac
      }
      When call disableAndMaskUbuntuProUnit ua-timer.timer
      The status should be failure
      The output should include "stopping, disabling and masking ua-timer.timer"
    End
  End
End
