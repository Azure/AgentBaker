#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2329

Describe 'Azure NVMe read-ahead reconciliation'
  setup() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/nvme-readahead"
    mkdir -p "$TEST_ROOT/sys/block"
    TEST_SCRIPT="$TEST_ROOT/reconcile.sh"
    sed "s|/sys/block/|$TEST_ROOT/sys/block/|g" \
      parts/linux/cloud-init/artifacts/azure-nvme-readahead.sh > "$TEST_SCRIPT"
  }
  cleanup() { rm -rf "$TEST_ROOT"; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  device() {
    mkdir -p "$TEST_ROOT/sys/block/$1/device" "$TEST_ROOT/sys/block/$1/queue"
    printf '%s\n' "$2" > "$TEST_ROOT/sys/block/$1/device/model"
    printf '%s\n' "$3" > "$TEST_ROOT/sys/block/$1/queue/read_ahead_kb"
  }

  It 'corrects the known-bad value and trims padded sysfs model names'
    device nvme0n1 'MSFT NVMe Accelerator v1.0              ' 65536
    When run sh "$TEST_SCRIPT"
    The status should be success
    The output should include 'nvme0n1 65536 -> 256 KiB'
    The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 256
  End

  It 'reconciles all affected namespaces, not just a newly attached disk'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    device nvme0n2 'MSFT NVMe Accelerator v1.0' 65536
    When run sh "$TEST_SCRIPT"
    The status should be success
    The lines of output should equal 2
    The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 256
    The contents of file "$TEST_ROOT/sys/block/nvme0n2/queue/read_ahead_kb" should equal 256
  End

  Describe 'unaffected values'
    Parameters
      0
      128
      256
      4096
      32768
      131072
    End
    It "preserves $1 KiB without writes or log noise"
      device nvme0n1 'MSFT NVMe Accelerator v1.0' "$1"
      When run sh "$TEST_SCRIPT"
      The status should be success
      The output should be blank
      The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal "$1"
    End
  End

  Describe 'unaffected models'
    Parameters
      'Microsoft NVMe Direct Disk'
      'Microsoft NVMe Direct Disk v2'
      'MSFT NVMe Accelerator v2.0'
      'Virtual_Disk'
    End
    It "leaves $1 untouched"
      device nvme0n1 "$1" 65536
      When run sh "$TEST_SCRIPT"
      The status should be success
      The output should be blank
      The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 65536
    End
  End

  It 'does nothing on a host without NVMe'
    device sda 'MSFT NVMe Accelerator v1.0' 65536
    When run sh "$TEST_SCRIPT"
    The status should be success
    The output should be blank
    The contents of file "$TEST_ROOT/sys/block/sda/queue/read_ahead_kb" should equal 65536
  End

  It 'is quiet on subsequent invocations'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    When run sh -c 'sh "$1" && sh "$1"' _ "$TEST_SCRIPT"
    The status should be success
    The lines of output should equal 1
    The output should include '65536 -> 256 KiB'
  End

  It 'reports unreadable metadata while still correcting other devices'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    device nvme1n1 'MSFT NVMe Accelerator v1.0' 65536
    rm "$TEST_ROOT/sys/block/nvme0n1/device/model"
    When run sh "$TEST_SCRIPT"
    The status should be failure
    The error should include 'Cannot read NVMe model'
    The output should include 'nvme1n1 65536 -> 256 KiB'
    The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 65536
    The contents of file "$TEST_ROOT/sys/block/nvme1n1/queue/read_ahead_kb" should equal 256
  End

  It 'reports an unreadable queue'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    rm "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb"
    When run sh "$TEST_SCRIPT"
    The status should be failure
    The error should include 'Cannot read NVMe read-ahead'
    The output should be blank
  End

  It 'propagates a failed write'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    When run bash -c 'printf() { return 1; }; . "$1"' _ "$TEST_SCRIPT"
    The status should be failure
    The error should include 'Cannot set NVMe read-ahead'
    The output should be blank
  End

  It 'does not claim success when read-back differs'
    device nvme0n1 'MSFT NVMe Accelerator v1.0' 65536
    When run bash -c 'printf() { command printf "%s\n" 65536; }; . "$1"' _ "$TEST_SCRIPT"
    The status should be failure
    The error should include 'NVMe read-ahead verification failed'
    The output should be blank
  End
End
