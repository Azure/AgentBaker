#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'guest failure diagnostics in generate-disk-usage.sh'
  setup_commands() {
    export DIAGNOSTIC_STATUS=0 WATCH_MODE=recovered WATCH_SLEEP_STEP=30 WATCH_TIMEOUT_STATUS=0
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
    ss() {
      if [ "$1" != "-H" ] || [ "$DIAGNOSTIC_STATUS" -ne 0 ]; then
        diagnostic_command ss "$@"
        return $?
      fi
      if [ "$WATCH_MODE" = "recovering" ] && [ "${WATCH_ATTEMPT:-0}" -eq 0 ]; then
        echo 'u_str LISTEN 0 4096 /run/user/0/systemd/private 1 * 0'
        echo 'u_str LISTEN 0 4096 /run/systemd/private-other 2 * 0'
        echo 'u_str ESTAB 0 0 /run/systemd/private 3 * 0'
      else
        echo 'u_str LISTEN 0 4096 /run/systemd/private 4 * 0 users:(("systemd",pid=1,fd=5))'
      fi
    }
    ls() { diagnostic_command ls "$@"; }
    journalctl() { diagnostic_command journalctl "$@"; }
    dmesg() { diagnostic_command dmesg "$@"; }
    systemctl() {
      if [ "$DIAGNOSTIC_STATUS" -ne 0 ]; then
        diagnostic_command systemctl "$@"
        return $?
      fi
      case "$WATCH_MODE:${WATCH_ATTEMPT:-0}" in
        unresponsive:*|recovering:1)
          echo 'mock manager query timed out' >&2
          return 124
          ;;
      esac
      echo '259'
    }
    timeout() {
      if [ "$1" = "--foreground" ]; then
        shift
      else
        echo "timeout command: $*"
      fi
      if [ "$2" = "90s" ] && [ "$WATCH_TIMEOUT_STATUS" -ne 0 ]; then
        return "$WATCH_TIMEOUT_STATUS"
      fi
      shift 2
      if [ "$1" = "bash" ]; then
        "$1" "$2" "unset SECONDS; SECONDS=0; $3"
      else
        "$@"
      fi
    }
    sleep() {
      WATCH_ATTEMPT=$((${WATCH_ATTEMPT:-0} + 1))
      SECONDS=$((SECONDS + WATCH_SLEEP_STEP))
    }
    date() {
      if [ "$1" = "-u" ]; then
        echo '2026-11-13T07:17:40Z'
      else
        echo '0'
      fi
    }
    export -f diagnostic_command ss systemctl timeout sleep date
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
    The output should not include 'timeout command:'
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

  It 'observes recovery only when the exact private socket listens and the manager responds'
    WATCH_MODE=recovering
    WATCH_SLEEP_STEP=1
    When call run_report --failure
    The status should be success
    The output should include '2026-11-13T07:17:40Z elapsed=0s'
    The output should include '2026-11-13T07:17:40Z elapsed=1s'
    The output should include '2026-11-13T07:17:40Z elapsed=2s'
    The output should include '/run/systemd/private listener: not-listening'
    The output should include '/run/systemd/private listener: listening'
    The output should include 'WARNING: diagnostic failed (exit 124): timeout --foreground --signal=KILL 2s systemctl show --property=Version --value'
    The output should include 'SYSTEMD RECOVERY OBSERVED after 2 seconds; original provisioning failure is unchanged.'
    The output should include 'SYSTEMD RECOVERY JOURNAL'
    The output should include 'timeout command: --signal=KILL 5s journalctl --boot --since 15 minutes ago --no-pager -o short-precise'
    The output should match pattern '*dmesg*SYSTEMD RECOVERY WATCH*SYSTEMD RECOVERY JOURNAL*DISK SPACE DIAGNOSTICS REPORT*'
    The output should not include 'elapsed=3s'
    The output should not include 'SYSTEMD RECOVERY NOT OBSERVED'
  End

  It 'expires with an unresponsive manager and preserves the final disk-report failure'
    WATCH_MODE=unresponsive
    FINAL_CHMOD_STATUS=13
    When call run_report --failure
    The status should equal 13
    The output should include 'elapsed=60s'
    The output should not include 'elapsed=90s'
    The output should include '/run/systemd/private listener: listening'
    The output should include 'mock manager query timed out'
    The output should include 'SYSTEMD RECOVERY NOT OBSERVED WITHIN 90-SECOND WINDOW; eventual recovery is unknown.'
    The output should not include 'SYSTEMD RECOVERY OBSERVED after'
    The output should include 'SYSTEMD RECOVERY JOURNAL'
    The output should include 'DISK SPACE DIAGNOSTICS REPORT'
    The output should include 'chmod 644 /opt/azure/disk-usage.txt'
  End

  It 'continues with journal and disk diagnostics when the overall timeout kills the watch'
    WATCH_TIMEOUT_STATUS=137
    When call run_report --failure
    The status should be success
    The output should include 'WARNING: diagnostic failed (exit 137): timeout --signal=KILL 90s bash -c observe_systemd_recovery'
    The output should include 'SYSTEMD RECOVERY NOT OBSERVED WITHIN 90-SECOND WINDOW; eventual recovery is unknown.'
    The output should include 'SYSTEMD RECOVERY JOURNAL'
    The output should include 'DISK SPACE DIAGNOSTICS REPORT'
    The output should include 'chmod 644 /opt/azure/disk-usage.txt'
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
