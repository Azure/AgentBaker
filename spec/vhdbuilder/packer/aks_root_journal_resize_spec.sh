#!/bin/bash

Describe 'AKS root journal initramfs installer'
  Include './vhdbuilder/scripts/linux/aks-root-journal/install.sh'

  accepts_ubuntu_2604_minimal_only() {
    aks_rj_is_supported ubuntu 26.04 ubuntu-26_04-lts-minimal '' false &&
      ! aks_rj_is_supported ubuntu 26.04 ubuntu-26_04-lts '' false
  }

  rejects_specialized_variants() {
    ! aks_rj_is_supported ubuntu 24.04 ubuntu-24_04-lts 'cvm' false &&
      ! aks_rj_is_supported azurelinux 3.0 AzureLinux '' true
  }

  accepts_azurelinux_3() {
    aks_rj_is_supported azurelinux 3.0 AzureLinux '' false &&
      aks_rj_is_supported azurelinux 3.0.1 AzureLinux '' false
  }

  It 'includes Ubuntu 24.04'
    When call aks_rj_is_supported ubuntu 24.04 ubuntu-24_04-lts '' false
    The status should be success
  End

  It 'includes Ubuntu 26.04 minimal only'
    When call accepts_ubuntu_2604_minimal_only
    The status should be success
  End

  It 'includes Azure Linux 3.0'
    When call accepts_azurelinux_3
    The status should be success
  End

  It 'excludes specialized image variants and FIPS'
    When call rejects_specialized_variants
    The status should be success
  End

  It 'excludes unsupported operating systems'
    When call aks_rj_is_supported ubuntu 22.04 ubuntu-22_04-lts '' false
    The status should be failure
  End
End

Describe 'AKS root journal dracut hook packaging'
  It 'uses the .sh suffix required by dracut hook discovery'
    When run grep -Fq dracut-hook.sh ./vhdbuilder/scripts/linux/aks-root-journal/dracut-module-setup.sh
    The status should be success
  End

  It 'installs the dracut hook with its discoverable .sh suffix'
    When run grep -Fq dracut-hook.sh ./vhdbuilder/scripts/linux/aks-root-journal/install.sh
    The status should be success
  End
End

Describe 'AKS root journal runtime entrypoint'
  setup_runtime_entrypoint() {
    MOUNT_PATH=$(mktemp -d)
    cp ./vhdbuilder/scripts/linux/aks-root-journal/resize-root-journal \
      "$MOUNT_PATH/aks-root-journal-resize"
    chmod +x "$MOUNT_PATH/aks-root-journal-resize"
  }

  cleanup_runtime_entrypoint() {
    rm -rf "$MOUNT_PATH"
  }

  invoke_installed_runtime_entrypoint() {
    "$MOUNT_PATH/aks-root-journal-resize"
  }

  BeforeEach 'setup_runtime_entrypoint'
  AfterEach 'cleanup_runtime_entrypoint'

  It 'runs the installed aks-root-journal-resize entrypoint'
    When call invoke_installed_runtime_entrypoint
    The status should be failure
    The output should include 'usage: resize-root-journal ROOT_DEVICE'
  End
End

Describe 'AKS root journal first-boot state'
  Include './vhdbuilder/scripts/linux/aks-root-journal/resize-root-journal'

  setup_state_context() {
    MOUNT_PATH=$(mktemp -d)
    STATE_PATH=/var/lib/aks/root-journal
    STATE_FILE="$STATE_PATH/test-instance-test-filesystem"
  }

  cleanup_state_context() {
    rm -rf "$MOUNT_PATH"
    unset -f mount umount sync
  }

  mount() {
    :
  }

  umount() {
    :
  }

  sync() {
    :
  }

  persist_state_atomically() {
    write_state in-progress &&
      [ "$(cat "$MOUNT_PATH$STATE_FILE")" = in-progress ] &&
      [ ! -e "$MOUNT_PATH$STATE_FILE.tmp" ]
  }

  BeforeEach 'setup_state_context'
  AfterEach 'cleanup_state_context'

  It 'atomically persists state without leaving a temporary file'
    When call persist_state_atomically
    The status should be success
  End
End

Describe 'AKS root journal sizing oracle'
  Include './vhdbuilder/scripts/linux/aks-root-journal/resize-root-journal'

  setup_oracle_context() {
    MOUNT_PATH=$(mktemp -d)
    INSTANCE_ID=00000000-0000-0000-0000-000000000001
    FILESYSTEM_BLOCKS=268435456
    FILESYSTEM_BLOCK_SIZE=4096
    FILESYSTEM_FEATURES=""
    MOCK_MKE2FS_ARGS=""
    [ -n "$INSTANCE_ID" ] && [ "$FILESYSTEM_BLOCKS" -eq 268435456 ] &&
      [ "$FILESYSTEM_BLOCK_SIZE" -eq 4096 ] && [ -z "$FILESYSTEM_FEATURES" ]
  }

  cleanup_oracle_context() {
    rm -rf "$MOUNT_PATH"
  }

  dd() {
    :
  }

  mke2fs() {
    MOCK_MKE2FS_ARGS=$*
  }

  tune2fs() {
    printf '%s\n' 'Journal inode: 8'
  }

  debugfs() {
    printf '%s\n' 'Inode: 8 Type: regular Mode: 0600 Size: 1073741824'
  }

  run_one_tib_oracle() {
    get_default_journal_size &&
      [ "$DEFAULT_JOURNAL_SIZE" -eq 1073741824 ] &&
      case "$MOCK_MKE2FS_ARGS" in
        *268435456*) return 0 ;;
        *) return 1 ;;
      esac
  }

  run_one_thousand_gib_oracle() {
    FILESYSTEM_BLOCKS=262144000
    get_default_journal_size &&
      [ "$DEFAULT_JOURNAL_SIZE" -eq 1073741824 ] &&
      case "$MOCK_MKE2FS_ARGS" in
        *262144000*) return 0 ;;
        *) return 1 ;;
      esac
  }

  run_fast_commit_oracle() {
    FILESYSTEM_FEATURES="has_journal fast_commit"
    get_default_journal_size &&
      case "$MOCK_MKE2FS_ARGS" in
        *"-O fast_commit"*) return 0 ;;
        *) return 1 ;;
      esac
  }

  BeforeEach 'setup_oracle_context'
  AfterEach 'cleanup_oracle_context'

  It 'uses the target e2fsprogs toolchain to calculate a 1 TiB default journal'
    When call run_one_tib_oracle
    The status should be success
  End

  It 'uses the requested 1000-GiB deployment geometry'
    When call run_one_thousand_gib_oracle
    The status should be success
  End

  It 'preserves the filesystem fast-commit feature in the sizing oracle'
    When call run_fast_commit_oracle
    The status should be success
  End
End

Describe 'AKS root journal completed-state boot'
  Include './vhdbuilder/scripts/linux/aks-root-journal/resize-root-journal'

  setup_completed_state_context() {
    FSCK_CALLS=0
    METADATA_CALLS=0
    READ_STATE_CALLS=0
    STATE_READ_FAILURES=0
    STATE_VALUE=complete
  }

  cleanup_completed_state_context() {
    unset -f resolve_root_device blkid awk cat read_state check_filesystem read_journal_metadata
  }

  resolve_root_device() {
    ROOT_DEVICE=$1
  }

  blkid() {
    [ "$ROOT_DEVICE" = /dev/test-root ] || return 1
    case "$*" in
      *'-s TYPE'*) printf '%s\n' ext4 ;;
      *'-s UUID'*) printf '%s\n' 11111111-1111-1111-1111-111111111111 ;;
      *) return 1 ;;
    esac
  }

  awk() {
    return 1
  }

  cat() {
    printf '%s\n' 00000000-0000-0000-0000-000000000001
  }

  read_state() {
    READ_STATE_CALLS=$((READ_STATE_CALLS + 1))
    if [ "$READ_STATE_CALLS" -le "$STATE_READ_FAILURES" ]; then
      return 1
    fi
    STATE=$STATE_VALUE
  }

  check_filesystem() {
    FSCK_CALLS=$((FSCK_CALLS + 1))
  }

  read_journal_metadata() {
    METADATA_CALLS=$((METADATA_CALLS + 1))
  }

  completed_boot_skips_filesystem_work() {
    aks_root_journal_main /dev/test-root &&
      [ "$READ_STATE_CALLS" -eq 1 ] &&
      [ "$STATE" = complete ] &&
      [ "$FSCK_CALLS" -eq 0 ] &&
      [ "$METADATA_CALLS" -eq 0 ]
  }

  recovered_state_read_skips_journal_work() {
    STATE_READ_FAILURES=1
    aks_root_journal_main /dev/test-root &&
      [ "$READ_STATE_CALLS" -eq 2 ] &&
      [ "$FSCK_CALLS" -eq 1 ] &&
      [ "$METADATA_CALLS" -eq 0 ]
  }

  BeforeEach 'setup_completed_state_context'
  AfterEach 'cleanup_completed_state_context'

  It 'skips filesystem checks and journal inspection after completion'
    When call completed_boot_skips_filesystem_work
    The status should be success
    The output should include 'journal sizing already completed'
  End

  It 'verifies the filesystem once when it must retry reading completion state'
    When call recovered_state_read_skips_journal_work
    The status should be success
    The output should include 'checking before retry'
    The output should include 'journal sizing already completed'
  End
End
