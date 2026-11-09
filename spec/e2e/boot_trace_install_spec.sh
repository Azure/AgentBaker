#!/bin/bash

Describe 'boot-trace image preparation guards'
  Include './e2e/boot-trace/guest/install.sh'

  sed() {
    case "$2" in
      's/^ID=//p') printf '%s\n' ubuntu ;;
      's/^VERSION_ID=//p') printf '%s\n' 26.04 ;;
      *) return 1 ;;
    esac
  }

  uname() {
    case "$1" in
      -m) printf '%s\n' x86_64 ;;
      -r) printf '%s\n' 7.0.0-1020-azure ;;
      *) return 1 ;;
    esac
  }

  It 'accepts either separately prepared backend on the pinned target'
    check_both_backends() {
      check_trace_target ftrace 26.04.20260701 test-ftrace &&
        check_trace_target lttng 26.04.20260701 test-lttng
    }
    When call check_both_backends
    The status should be success
    The output should be blank
  End

  It 'rejects latest before installing or arming anything'
    When call check_trace_target ftrace latest test-image
    The status should be failure
    The stderr should include 'exact base image version'
  End

  It 'rejects an unsupported combined backend'
    When call check_trace_target both 26.04.20260701 test-image
    The status should be failure
    The stderr should include 'Expected ftrace or lttng'
  End

  It 'requires rechecking compatibility when the kernel changes'
    uname() {
      case "$1" in
        -m) printf '%s\n' x86_64 ;;
        -r) printf '%s\n' 7.0.0-1021-azure ;;
        *) return 1 ;;
      esac
    }
    When call check_trace_target lttng 26.04.20260701 test-image
    The status should be failure
    The stderr should include 'pinned kernel'
  End

  It 'adds early tracing parameters without changing the existing boot path'
    configure_grub() {
      local GRUB_CMDLINE_LINUX='console=ttyS0 root=PARTUUID=test-root'
      local GRUB_CMDLINE_LINUX_DEFAULT='quiet'
      local GRUB_FORCE_PARTUUID='test-root'
      . ./e2e/boot-trace/guest/ftrace-grub.cfg
      printf '%s\n' "$GRUB_CMDLINE_LINUX" "$GRUB_CMDLINE_LINUX_DEFAULT" "$GRUB_FORCE_PARTUUID"
    }
    When call configure_grub
    The status should be success
    The line 1 should start with 'console=ttyS0 root=PARTUUID=test-root '
    The line 1 should include 'trace_clock=mono trace_buf_size=4M trace_event=sched:sched_switch,'
    The line 1 should include ',block:block_rq_issue,block:block_rq_complete'
    The line 1 should include ',net:net_dev_xmit,net:netif_receive_skb_entry,net:netif_receive_skb_list_entry,net:netif_rx_entry'
    The line 1 should not include ' bootconfig'
    The line 2 should equal 'quiet'
    The line 3 should equal 'test-root'
  End

  It 'rejects unverified native recorder packages before changing APT'
    curl() { : > "$4"; }
    sha512sum() {
      cat > /dev/null
      echo 'checksum mismatch' >&2
      return 1
    }
    apt-get() { echo 'Unexpected package mutation'; }
    apt-mark() { echo 'Unexpected package mutation'; }
    When run install_ftrace_recorder
    The status should be failure
    The output should be blank
    The stderr should equal 'checksum mismatch'
  End

  It 'refuses to instrument an unexpected cloud script without changing it'
    cloud_fixture="${SHELLSPEC_TMPBASE}/boot-trace-unknown-cloud.sh"
    printf 'unexpected cloud script\n' > "$cloud_fixture"
    When run python3 ./e2e/boot-trace/guest/diagnostics.py --instrument-cloud "$cloud_fixture"
    The status should be failure
    The stderr should include 'refusing to instrument'
    The contents of file "$cloud_fixture" should equal 'unexpected cloud script'
  End
End
