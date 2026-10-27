#!/usr/bin/env shellspec

Describe 'VHD staging for ACL overlays'
  setup() {
    TEST_ROOT="$(mktemp -d)"
    COMMAND_LOG="$TEST_ROOT/commands"
    mkdir -p "$TEST_ROOT/bin"
    : > "$COMMAND_LOG"
    # Execute the pipeline's actual inline script, not a duplicate implementation.
    awk '
      /inlineScript: \|/ { script = ""; capture = 1; next }
      capture && /^        / { script = script substr($0, 9) "\n"; next }
      capture { capture = 0 }
      /^    displayName: Set Destination Storage Container to Staging$/ {
        printf "%s", script
        found = 1
        exit
      }
      END { if (!found) exit 1 }
    ' .pipelines/templates/.builder-release-template.yaml > "$TEST_ROOT/staging.sh"
    cat > "$TEST_ROOT/bin/az" <<'EOF'
#!/bin/bash
printf 'az %s\n' "$*" >> "$COMMAND_LOG"
case "$1 $2" in
  'cloud show')
    [[ "${FAIL_CLOUD:-0}" == 0 ]] || exit 1
    printf '%s\n' "${STORAGE_SUFFIX-core.windows.net}"
    ;;
  'storage container')
    case "$3" in
      exists)
        [[ "${FAIL_EXISTS:-0}" == 0 ]] || exit 1
        printf '{"exists":%s}\n' "${CONTAINER_EXISTS:-false}"
        ;;
      create)
        [[ "${FAIL_CREATE:-0}" == 0 ]] || exit 1
        ;;
      *) exit 1 ;;
    esac
    ;;
  *) exit 1 ;;
esac
EOF
    chmod +x "$TEST_ROOT/bin/az"
    export TEST_ROOT COMMAND_LOG
    export PATH="$TEST_ROOT/bin:$PATH"
    export STORAGE_ACCOUNT_NAME=teststorage VHD_STAGING_CONTAINER_NAME=vhdstaging
    unset CLASSIC_BLOB_STAGING STORAGE_SUFFIX CONTAINER_EXISTS FAIL_CLOUD FAIL_EXISTS FAIL_CREATE
  }
  cleanup() {
    rm -rf "$TEST_ROOT"
  }
  stage() {
    bash "$TEST_ROOT/staging.sh"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'derives a staging URL when CLASSIC_BLOB_STAGING is unset'
    When call stage
    The status should be success
    The output should include 'variable=DESTINATION_STORAGE_CONTAINER]https://teststorage.blob.core.windows.net/vhdstaging'
    The output should include 'variable=CLASSIC_BLOB_STAGING]https://teststorage.blob.core.windows.net/vhdstaging'
    The contents of file "$COMMAND_LOG" should include 'storage container create --account-name teststorage --name vhdstaging'
  End

  It 'derives a staging URL when CLASSIC_BLOB_STAGING is empty'
    export CLASSIC_BLOB_STAGING=''
    When call stage
    The status should be success
    The output should include 'variable=DESTINATION_STORAGE_CONTAINER]https://teststorage.blob.core.windows.net/vhdstaging'
  End

  It 'preserves an explicitly configured staging endpoint'
    export CLASSIC_BLOB_STAGING=https://custom.blob.core.windows.net/custom-staging
    export FAIL_CLOUD=1
    When call stage
    The status should be success
    The output should include 'variable=DESTINATION_STORAGE_CONTAINER]https://custom.blob.core.windows.net/custom-staging'
    The contents of file "$COMMAND_LOG" should not include 'cloud show'
  End

  It 'uses the current cloud storage suffix rather than hardcoding public Azure'
    export STORAGE_SUFFIX=core.chinacloudapi.cn
    When call stage
    The status should be success
    The output should include 'https://teststorage.blob.core.chinacloudapi.cn/vhdstaging'
  End

  It 'does not recreate an existing container'
    export CONTAINER_EXISTS=true
    When call stage
    The status should be success
    The output should include 'already exists'
    The contents of file "$COMMAND_LOG" should not include 'storage container create'
  End

  It 'propagates failure to query the Azure cloud'
    export FAIL_CLOUD=1
    When call stage
    The status should be failure
    The output should not include 'task.setvariable'
    The contents of file "$COMMAND_LOG" should not include 'storage container'
  End

  It 'rejects an empty storage suffix'
    export STORAGE_SUFFIX=''
    When call stage
    The status should be failure
    The error should include 'Unable to resolve'
    The output should not include 'task.setvariable'
  End

  It 'rejects a missing storage suffix reported as None'
    export STORAGE_SUFFIX=None
    When call stage
    The status should be failure
    The error should include 'Unable to resolve'
    The output should not include 'task.setvariable'
  End

  It 'propagates failure to query container existence'
    export FAIL_EXISTS=1
    When call stage
    The status should be failure
    The output should not include 'task.setvariable'
    The contents of file "$COMMAND_LOG" should not include 'storage container create'
  End

  It 'does not export the destination if container creation fails'
    export FAIL_CREATE=1
    When call stage
    The status should be failure
    The output should include 'Creating staging container'
    The output should not include 'task.setvariable'
  End

  It 'rejects a missing storage account before running Azure commands'
    unset STORAGE_ACCOUNT_NAME
    When call stage
    The status should be failure
    The error should include 'STORAGE_ACCOUNT_NAME'
    The contents of file "$COMMAND_LOG" should equal ''
  End
End
