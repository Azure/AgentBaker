#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'guest failure diagnostics in generate-disk-usage.sh'
  setup_commands() {
    mkdir() { :; }
    chmod() {
      echo "chmod $*"
      if [ "$1" = "644" ]; then
        return "${FINAL_CHMOD_STATUS:-0}"
      fi
    }
    df() { echo "Filesystem Size Used Avail Use% Mounted on"; }
    ctr() { :; }
    du() { :; }
    find() { :; }
    tee() { cat; }

    diagnostic_command() {
      echo "diagnostic command: $*"
      echo "diagnostic stderr: $1" >&2
      return "${DIAGNOSTIC_STATUS:-0}"
    }
    ps() { diagnostic_command ps "$@"; }
    readlink() { diagnostic_command readlink "$@"; }
    ss() { diagnostic_command ss "$@"; }
    ls() { diagnostic_command ls "$@"; }
    journalctl() { diagnostic_command journalctl "$@"; }
    dmesg() { diagnostic_command dmesg "$@"; }
  }
  BeforeEach 'setup_commands'

  run_report() {
    # shellcheck source=/dev/null
    . ./vhdbuilder/packer/generate-disk-usage.sh "$@"
  }

  It 'keeps normal disk reports free of failure diagnostics'
    When call run_report
    The status should be success
    The output should include 'DISK SPACE DIAGNOSTICS REPORT'
    The output should include 'chmod 644 /opt/azure/disk-usage.txt'
    The output should not include 'SYSTEMD FAILURE DIAGNOSTICS'
    The output should not include 'diagnostic command:'
  End

  It 'prints PID 1, socket, current-boot journal and recent kernel evidence to stdout'
    When call run_report --failure
    The status should be success
    The output should include 'diagnostic command: ps -p 1 -o pid,stat,comm,args'
    The output should include 'diagnostic command: readlink /proc/1/exe'
    The output should include 'diagnostic command: ss -xlpn'
    The output should include 'diagnostic command: ls -ld /run/systemd/private'
    The output should include 'diagnostic command: ls -ld /run/dbus/system_bus_socket'
    The output should include 'diagnostic command: journalctl --boot --since 15 minutes ago --no-pager -o short-precise'
    The output should include 'diagnostic command: dmesg --ctime --since 15 minutes ago'
    The output should include 'diagnostic stderr: ps'
    The output should include 'diagnostic stderr: readlink'
    The output should include 'diagnostic stderr: ss'
    The output should include 'diagnostic stderr: ls'
    The output should include 'diagnostic stderr: journalctl'
    The output should include 'diagnostic stderr: dmesg'
    The output should include 'DISK SPACE DIAGNOSTICS REPORT'
    The output should include 'chmod 644 /opt/azure/disk-usage.txt'
    The output should not include 'WARNING: diagnostic failed'
  End

  Describe 'when diagnostic commands fail or are unavailable'
    Parameters:value 1 127
    It 'reports every failure and still completes the disk report'
      DIAGNOSTIC_STATUS="$1"
      When call run_report --failure
      The status should be success
      The output should include "WARNING: diagnostic failed (exit $1): ps -p 1 -o pid,stat,comm,args"
      The output should include "WARNING: diagnostic failed (exit $1): readlink /proc/1/exe"
      The output should include "WARNING: diagnostic failed (exit $1): ss -xlpn"
      The output should include "WARNING: diagnostic failed (exit $1): ls -ld /run/systemd/private"
      The output should include "WARNING: diagnostic failed (exit $1): ls -ld /run/dbus/system_bus_socket"
      The output should include "WARNING: diagnostic failed (exit $1): journalctl --boot --since 15 minutes ago --no-pager -o short-precise"
      The output should include "WARNING: diagnostic failed (exit $1): dmesg --ctime --since 15 minutes ago"
      The output should include 'DISK SPACE DIAGNOSTICS REPORT'
      The output should include 'chmod 644 /opt/azure/disk-usage.txt'
    End
  End

  It 'preserves the existing final disk-report command status'
    FINAL_CHMOD_STATUS=13
    When call run_report --failure
    The status should equal 13
    The output should include 'diagnostic command: dmesg'
    The output should include 'DISK SPACE DIAGNOSTICS REPORT'
    The output should include 'chmod 644 /opt/azure/disk-usage.txt'
  End
End
