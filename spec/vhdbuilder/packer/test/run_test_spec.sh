#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2286,SC2288,SC2329

Describe 'Linux content-test Run Command retries'
  az() {
    if [ "$1 $2" = 'group delete' ]; then echo cleanup; return; fi
    printf '%s\n' "${@: -1}" > "$REPOSITORY_ARGUMENT"
    local calls
    calls=$(( $(cat "$CALLS") + 1 ))
    printf '%s' "$calls" > "$CALLS"
    printf '%s\n' "$RESPONSE"
    [ "$calls" -ge "$SUCCESS_ON" ]
  }
  run_retry_loop() (
    set -eu
    VM_NAME=test-vm TEST_VM_RESOURCE_GROUP_NAME=test-rg SCRIPT_PATH=test.sh VHD_DEBUG=False
    OS_VERSION=22.04 ENABLE_FIPS=false OS_SKU=Ubuntu GIT_BRANCH=refs/heads/main
    IMG_SKU='' FEATURE_FLAGS='' GIT_COMMIT_HASH=commit
    AGENTBAKER_REPOSITORY_URL=https://github.com/example/AgentBaker.git
    eval "$(sed -n '/^function cleanup()/,/^trap cleanup EXIT/p' vhdbuilder/packer/test/run-test.sh)"
    eval "$(sed -n '/^  for i in $(seq 1 3); do$/,/^  done$/p' vhdbuilder/packer/test/run-test.sh)"
  )

  Parameters
    1 0 1 ''
    2 0 2 ''
    3 0 3 ''
    4 1 3 ''
    4 1 3 'nonempty failure response'
  End
  It "fails only if all attempts fail: success on attempt $1"
    SUCCESS_ON=$1 RESPONSE=$4 CALLS="${SHELLSPEC_WORKDIR}/calls"
    REPOSITORY_ARGUMENT="${SHELLSPEC_WORKDIR}/repository-argument"
    printf 0 > "$CALLS"
    When run run_retry_loop
    The status should equal "$2"
    The contents of file "$CALLS" should equal "$3"
    The contents of file "$REPOSITORY_ARGUMENT" should equal "https://github.com/example/AgentBaker.git"
    The output should include cleanup
    The output should not include '3: retrying'
    if [ "$2" -eq 1 ]; then
      The error should include 'Run Command failed after 3 attempts'
    fi
  End
End

Describe 'Linux content-test Run Command parameters'
  az() {
    while [ "$#" -gt 0 ] && [ "$1" != '--parameters' ]; do shift; done
    [ "$#" -gt 0 ] || return 1
    shift
    printf 'parameter-count=%s\n' "$#"
    printf 'wire=<%s>\n' "$@"

    # Model Run Command dropping empty values before the VM script receives them.
    local parameter
    local received=()
    for parameter in "$@"; do
      if [ -n "$parameter" ]; then received+=("$parameter"); fi
    done
    set -- "${received[@]}"
    local OS_VERSION='' ENABLE_FIPS='' OS_SKU='' GIT_BRANCH=''
    local IMG_SKU='' FEATURE_FLAGS='' GIT_COMMIT_HASH='' AGENTBAKER_REPOSITORY_URL=''
    eval "$(sed -n '/^OS_VERSION=/,/^AGENTBAKER_REPOSITORY_URL=/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
    printf 'decoded=<%s>\n' "$OS_VERSION" "$ENABLE_FIPS" "$OS_SKU" "$GIT_BRANCH" \
      "$IMG_SKU" "$FEATURE_FLAGS" "$GIT_COMMIT_HASH" "$AGENTBAKER_REPOSITORY_URL"
  }

  run_parameter_round_trip() (
    set -eu
    VM_NAME=test-vm TEST_VM_RESOURCE_GROUP_NAME=test-rg SCRIPT_PATH=test.sh
    eval "$(sed -n '/^  for i in $(seq 1 3); do$/,/^  done$/p' vhdbuilder/packer/test/run-test.sh)"
    printf '%s\n' "$ret"
  )

  Parameters
    acl false AzureContainerLinux '' None
    acl true AzureContainerLinux '' None
    acl false AzureContainerLinux '' cvm
    acl false AzureContainerLinux '' ''
    22.04 false Ubuntu 22_04-lts-gen2 ''
    22.04 true Ubuntu 22_04-lts-gen2 None
    3.0 false AzureLinux azurelinux-3-gen2 kata
    26.04 false Ubuntu minimal minimal,cvm
    22.04 false Ubuntu img-sku:literal feature-flags:literal
  End

  It "preserves every field through transport for $3 with SKU '$4' and flags '$5'"
    OS_VERSION="$1" ENABLE_FIPS="$2" OS_SKU="$3" IMG_SKU="$4" FEATURE_FLAGS="$5"
    GIT_BRANCH=refs/heads/example
    GIT_COMMIT_HASH=6da980562fefc854bb3a779f2414c068600065b1
    AGENTBAKER_REPOSITORY_URL=https://github.com/example/AgentBaker.git
    expected_output=$(
      printf 'parameter-count=8\n'
      printf 'wire=<%s>\n' "$OS_VERSION" "$ENABLE_FIPS" "$OS_SKU" "$GIT_BRANCH" \
        "img-sku:$IMG_SKU" "feature-flags:$FEATURE_FLAGS" "$GIT_COMMIT_HASH" "$AGENTBAKER_REPOSITORY_URL"
      printf 'decoded=<%s>\n' "$OS_VERSION" "$ENABLE_FIPS" "$OS_SKU" "$GIT_BRANCH" \
        "$IMG_SKU" "$FEATURE_FLAGS" "$GIT_COMMIT_HASH" "$AGENTBAKER_REPOSITORY_URL"
    )

    When run run_parameter_round_trip
    The status should be success
    The output should equal "$expected_output"
  End
End

Describe 'Linux content-test failure handling'
  run_failure_handling() (
    set -eu
    ret='{}'
    errMsg="$ERR_MSG"
    eval "$(sed -n '/^case "${OS_VERSION:-}" in$/,/^esac$/p' vhdbuilder/packer/test/run-test.sh)"
    eval "$(sed -n '/^function reportTestFailuresAsWarnings()/,/^}/p' vhdbuilder/packer/test/run-test.sh)"
    eval "$(sed -n '/^  if \[\[ "\$errMsg" == \*":Error:"\* \]\]; then$/,/^  fi$/p' vhdbuilder/packer/test/run-test.sh)"
    echo 'run-test continued'
  )
  collect_warnings() { run_failure_handling | grep '^##vso\[task.logissue type=warning\]'; }

  Context 'exit status'
    Parameters
      22.04 '' 1
      V3 '' 1
      V4 '' 0
      4.0 '' 0
      V4 false 1
      V4 FALSE 1
      22.04 true 0
      22.04 TRUE 0
    End

    It "exits $3 for OS_VERSION '$1' with VHD_TEST_FAILURES_AS_WARNINGS '$2'"
      OS_VERSION="$1" VHD_TEST_FAILURES_AS_WARNINGS="$2"
      ERR_MSG=$'testPam:Error: pam failed\ntestCloudInit:Error: cloud-init errors'
      When run run_failure_handling
      The status should equal "$3"
      The output should include 'Tests failed.'
      if [ "$3" -eq 0 ]; then
        The output should include 'run-test continued'
        The output should include '##vso[task.logissue type=warning]2 VHD content test(s) failed'
      else
        The output should not include '##vso[task.logissue'
      fi
    End
  End

  It 'logs a summary plus one warning per failing test and skips continuation lines'
    OS_VERSION=V4 VHD_TEST_FAILURES_AS_WARNINGS=''
    ERR_MSG=$'testPam:Error: pam failed\r\nPAM output line\ntestCloudInit:Error: cloud-init errors'
    expected=$(printf '%s\n' \
      '##vso[task.logissue type=warning]2 VHD content test(s) failed; reporting as warnings because VHD_TEST_FAILURES_AS_WARNINGS=true' \
      '##vso[task.logissue type=warning]VHD content test failed (non-fatal): testPam:Error: pam failed' \
      '##vso[task.logissue type=warning]VHD content test failed (non-fatal): testCloudInit:Error: cloud-init errors')
    When call collect_warnings
    The status should be success
    The output should equal "$expected"
  End
End

Describe 'Test VM credential log hygiene'
  openssl() { printf 'deadbeefcafebabe1234567890abcdef\n'; }
  az() { printf '%s\n' "$@" > "$AZ_ARGS"; }

  run_vm_create() (
    set -eux
    VM_NAME=test-vm TEST_VM_RESOURCE_GROUP_NAME=test-rg MANAGED_SIG_ID=sig-id
    TEST_VM_ADMIN_USERNAME=azureuser TESTING_NIC_ID=nic-id
    TARGET_COMMAND_STRING='--size Standard_D2ds_v5'
    TEST_VM_USER_DATA_ARGS=()
    eval "$(sed -n '/^set +x$/,/^set -x$/p' vhdbuilder/packer/test/run-test.sh)"
    eval "$(sed -n "/^$1set +x$/,/^$1set -x$/p" vhdbuilder/packer/test/run-test.sh)"
    eval "$(grep '^echo "VHD test VM username:' vhdbuilder/packer/test/run-test.sh)"
  )

  Parameters
    linux '    '
    windows '  '
  End

  It "passes the generated password to az vm create on $1 without logging it"
    AZ_ARGS="${SHELLSPEC_WORKDIR}/az-args-$1"
    When run run_vm_create "$2"
    The status should be success
    The contents of file "$AZ_ARGS" should include 'vm'
    The contents of file "$AZ_ARGS" should include '--admin-password'
    The contents of file "$AZ_ARGS" should include 'TestVM@deadbeefcafebabe1234567890abcdef'
    The contents of file "$AZ_ARGS" should not include '--debug'
    The output should include 'VHD test VM username: azureuser'
    The output should not include 'deadbeefcafebabe'
    The error should include "+ echo 'VHD test VM username: azureuser'"
    The error should not include 'deadbeefcafebabe'
  End
End
