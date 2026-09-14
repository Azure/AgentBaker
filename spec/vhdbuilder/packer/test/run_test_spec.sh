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

Describe 'Content VM FIPS encryption contract'
  az() {
    case "$1 $2 $3" in
      'network vnet subnet') printf '{"id":"subnet"}\n' ;;
      'network nic create') printf '{"NewNIC":{"id":"nic"}}\n' ;;
      'vm create --debug')
        printf '%s\n' "$*" > "$VM_REQUEST"
        ;;
      'sig image-definition show')
        [ "$5" = image ] || return 1
        jq -n --arg generation "${TEST_GENERATION:-V2}" --arg feature "${TEST_IMAGE_FEATURE:-}" \
          '{hyperVGeneration: $generation, features: [{name: "SecurityType", value: $feature}]}'
        return "${IMAGE_STATUS:-0}"
        ;;
      'rest --method put')
        shift 3
        while [ "$#" -gt 0 ]; do
          case "$1" in
            --body) printf '%s\n' "$2" > "$VM_REQUEST" ;;
            --url) printf '%s\n' "$2" > "$VM_REQUEST.url" ;;
          esac
          shift 2
        done
        printf 'create\n' > "$VM_REQUEST.events"
        return "${CREATE_STATUS:-0}"
        ;;
      'vm wait -g')
        printf 'wait\n' >> "$VM_REQUEST.events"
        return 0
        ;;
      'vm run-command invoke')
        [ "$(cat "$VM_REQUEST.events")" = "$(printf 'create\nwait')" ] || return 1
        jq -e --argjson fips "$EXPECT_FIPS" --arg security "${EXPECT_SECURITY:-}" '
          .location == "eastus"
          and .properties.storageProfile.imageReference.id == "image/versions/1"
          and .properties.storageProfile.osDisk.createOption == "FromImage"
          and .properties.storageProfile.osDisk.caching == "ReadWrite"
          and .properties.storageProfile.osDisk.diskSizeGB == null
          and .properties.storageProfile.osDisk.managedDisk.storageAccountType == null
          and .properties.networkProfile.networkInterfaces == [{id: "nic"}]
          and .identity == null
          and (if $fips then .properties.additionalCapabilities.enableFips1403Encryption == true
               else .properties.additionalCapabilities == null end)
          and (if $security == "" then .properties.securityProfile == null
               else .properties.securityProfile == {
                 securityType: $security, uefiSettings: {secureBootEnabled: true, vTpmEnabled: true}
               } end)
          and (if $security == "ConfidentialVM" then
                 .properties.osProfile == null
                 and .properties.storageProfile.osDisk.managedDisk.securityProfile.securityEncryptionType == "VMGuestStateOnly"
               else .properties.osProfile == {
                 computerName: "test-vm", adminUsername: "test-user", adminPassword: "unused"
               } end)' "$VM_REQUEST" > /dev/null || return 1
        printf 'content-test-passed\n'
        ;;
      'group delete --name') printf 'cleanup\n' ;;
      *) echo "Unexpected Azure operation: $1 $2" >&2; return 1 ;;
    esac
  }

  run_content_vm() (
    set -eu
    TIMEFORMAT=''
    ret=''
    MANAGED_SIG_ID=image/versions/1 OS_TYPE="${TEST_OS_TYPE:-Linux}" OS_SKU="$1" OS_VERSION="$2" ENABLE_FIPS="$3"
    ENABLE_TRUSTED_LAUNCH="$4" ARCHITECTURE="${TEST_ARCH:-amd64}" FEATURE_FLAGS="${TEST_FEATURE_FLAGS:-}"
    TEST_VM_RESOURCE_GROUP_NAME=test-rg VM_NAME=test-vm TEST_VM_ADMIN_USERNAME=test-user
    TEST_VM_ADMIN_PASSWORD=unused PACKER_BUILD_LOCATION=eastus AZURE_LOCATION=eastus
    SUBSCRIPTION_ID=subscription PACKER_VNET_RESOURCE_GROUP_NAME=network-rg PACKER_VNET_NAME=network
    VHD_DEBUG=False SCRIPT_PATH=test.sh GIT_BRANCH=refs/heads/main IMG_SKU='' GIT_COMMIT_HASH=commit
    eval "$(sed -n '/^function cleanup()/,/^trap cleanup EXIT/p' vhdbuilder/packer/test/run-test.sh)"
    eval "$(sed -n '/^TEST_VM_SIZE=/,/^time az vm wait/p' vhdbuilder/packer/test/run-test.sh)"
    if [ "$OS_TYPE" = Windows ]; then exit 0; fi
    eval "$(sed -n '/^  for i in $(seq 1 3); do$/,/^  done$/p' vhdbuilder/packer/test/run-test.sh)"
    printf '%s\n' "$ret"
  )

  Context 'VM configurations'
    Parameters
      Ubuntu 22.04 true False true amd64 Standard_D2ds_v5 '' V2 '' ''
      Ubuntu 22.04 True True true amd64 Standard_D2ds_v5 '' V2 '' TrustedLaunch
      Ubuntu 22.04 TRUE true true amd64 Standard_D2ds_v5 '' V2 '' TrustedLaunch
      Ubuntu 22.04 true False true arm64 Standard_D2pds_v5 '' V2 '' ''
      Ubuntu 22.04 true true true arm64 Standard_D2pds_v6 '' V2 '' TrustedLaunch
      Ubuntu 22.04 false False false amd64 Standard_D2ds_v5 '' V2 '' ''
      Ubuntu 22.04 false True false arm64 Standard_D2pds_v6 '' V2 '' TrustedLaunch
      Ubuntu 20.04 true False false amd64 Standard_D2ds_v5 '' V1 '' ''
      AzureLinux 3.0 true False false amd64 Standard_D2ds_v5 '' V2 '' ''
      Ubuntu 22.04 true False true amd64 Standard_D2ds_v5 TrustedLaunchSupported V2 '' TrustedLaunch
      Ubuntu 22.04 false False false amd64 Standard_D2ds_v5 TrustedLaunchSupported V2 '' TrustedLaunch
      Ubuntu 22.04 false False false amd64 Standard_D2ds_v5 TrustedLaunchAndConfidentialVmSupported V2 '' TrustedLaunch
      Ubuntu 22.04 true False true arm64 Standard_D2pds_v6 TrustedLaunchSupported V2 '' TrustedLaunch
      AzureLinux V3 True False false arm64 Standard_D2pds_v6 TrustedLaunchSupported V2 '' TrustedLaunch
      Ubuntu 22.04 false False false amd64 Standard_DC8ads_v5 '' V2 cvm ConfidentialVM
      AzureLinux 3.0 false True false amd64 Standard_DC8ads_v5 '' V2 cvm ConfidentialVM
    End
    It "enables extension encryption only for $1 $2 FIPS=$3 TL=$4"
      VM_REQUEST="${SHELLSPEC_WORKDIR}/request" EXPECT_FIPS="$5" TEST_ARCH="$6"
      TEST_IMAGE_FEATURE="$8" TEST_GENERATION="$9" TEST_FEATURE_FLAGS="${10}" EXPECT_SECURITY="${11}"
      When run run_content_vm "$1" "$2" "$3" "$4"
      The status should be success
      The output should include content-test-passed
      The output should include cleanup
      The contents of file "$VM_REQUEST" should include "\"vmSize\": \"$7\""
      The contents of file "$VM_REQUEST.url" should equal '/subscriptions/subscription/resourceGroups/test-rg/providers/Microsoft.Compute/virtualMachines/test-vm?api-version=2024-11-01'
    End
  End

  Context 'Windows configurations'
    Parameters
      False
      True
    End
    It "keeps Windows on CLI creation with TL=$1"
      VM_REQUEST="${SHELLSPEC_WORKDIR}/request" TEST_OS_TYPE=Windows
      When run run_content_vm Windows 2022 false "$1"
      The status should be success
      The output should include cleanup
      The contents of file "$VM_REQUEST" should include 'vm create --debug'
      The contents of file "$VM_REQUEST" should include '--public-ip-address  --size Standard_D2ds_v5'
      if [ "$1" = True ]; then
        The contents of file "$VM_REQUEST" should include '--security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true'
      else
        The contents of file "$VM_REQUEST" should not include '--security-type'
      fi
    End
  End

  Parameters
    9 0 9
    0 7 7
  End
  It "stops before content tests when creation=$1 or image lookup=$2 fails"
    VM_REQUEST="${SHELLSPEC_WORKDIR}/request"
    EXPECT_FIPS=true CREATE_STATUS="$1" IMAGE_STATUS="$2"
    When run run_content_vm Ubuntu 22.04 true False
    The status should equal "$3"
    The output should include cleanup
    The output should not include content-test-passed
  End
End
