#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2286,SC2288,SC2329

Describe 'Linux content-test Run Command retries'
  az() {
    if [ "$1 $2" = 'group delete' ]; then echo cleanup; return; fi
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
    printf 0 > "$CALLS"
    When run run_retry_loop
    The status should equal "$2"
    The contents of file "$CALLS" should equal "$3"
    The output should include cleanup
    The output should not include '3: retrying'
    if [ "$2" -eq 1 ]; then
      The error should include 'Run Command failed after 3 attempts'
    fi
  End
End

Describe 'Test VM NIC creation'
  az() {
    printf '%s\n' "$*" >> "$AZ_CALLS"
    if [ "$1 $2 $3 $4" = 'network vnet subnet show' ]; then
      printf '{"id":"test-subnet-id"}\n'
    elif [ "$1 $2 $3" = 'network nic create' ]; then
      printf '{"NewNIC":{"id":"test-nic-id"}}\n'
    fi
  }
  create_nic() (
    set -eu
    SUBSCRIPTION_ID=test-subscription
    PACKER_VNET_RESOURCE_GROUP_NAME=test-vnet-rg
    PACKER_VNET_NAME=test-vnet
    TEST_VM_RESOURCE_GROUP_NAME=test-vm-rg
    eval "$(sed -n '/^function create_test_vm_nic()/,/^}/p' vhdbuilder/packer/test/run-test.sh)"
    create_test_vm_nic
    printf '%s\n' "$TESTING_NIC_ID"
  )

  It 'creates the test NIC on the existing packer subnet'
    AZ_CALLS="${SHELLSPEC_WORKDIR}/az-calls"
    : > "$AZ_CALLS"
    When run create_nic
    The status should be success
    The output should equal 'test-nic-id'
    The contents of file "$AZ_CALLS" should include 'network vnet subnet show --ids /subscriptions/test-subscription/resourceGroups/test-vnet-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/packer'
    The contents of file "$AZ_CALLS" should include 'network nic create --resource-group test-vm-rg'
    The contents of file "$AZ_CALLS" should include '--subnet /subscriptions/test-subscription/resourceGroups/test-vnet-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/packer'
  End
End
