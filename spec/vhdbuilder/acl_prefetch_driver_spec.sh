#!/bin/bash

Describe 'ACL experimental AIB template'
  validate_template() {
    jq -e '
      (.properties.customize | length) == 1 and
      .properties.customize[0].type == "Shell" and
      .properties.customize[0].inline == [
        "sudo test -s /opt/azure/containers/prefetch.sh",
        "sudo test -s /opt/azure/containers/cleanup-vhd.sh"
      ] and .properties.vmProfile.vmSize == "Standard_D4s_v6" and
      .properties.optimize.vmBoot.state == "Enabled" and
      .properties.optimize.workload.state == "Enabled"
    ' vhdbuilder/prefetch/templates/acl-experiment.json
  }
  It 'retains a nonempty preflight customizer without deleting the baked cleanup'
    When call validate_template
    The status should be success
    The output should equal true
  End
End

Describe 'ACL workload prefetch first-boot postcondition'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  setup() {
    BOOT_DIR="$(mktemp -d)"
    mkdir -p "${BOOT_DIR}/acl/uki-addons" "${BOOT_DIR}/loader" \
      "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d"
    printf 'default vmlinuz-test.efi\n' > "${BOOT_DIR}/loader/loader.conf"
    touch "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi"
    printf 'signed-addon-fixture\n' > "${BOOT_DIR}/acl/uki-addons/firstboot.addon.efi"
    cp "${BOOT_DIR}/acl/uki-addons/firstboot.addon.efi" \
      "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d/firstboot.addon.efi"
  }
  cleanup() {
    rm -rf "${BOOT_DIR}"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'rearms the legacy marker after verifying the selected UKI addon'
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be success
    The path "${BOOT_DIR}/flatcar/first_boot" should be exist
  End

  It 'is idempotent'
    verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be success
    The path "${BOOT_DIR}/flatcar/first_boot" should be exist
  End

  It 'supports the older acl.efi name'
    mv "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi" "${BOOT_DIR}/EFI/Linux/acl.efi"
    mv "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d" "${BOOT_DIR}/EFI/Linux/acl.efi.extra.d"
    printf 'default acl.efi\n' > "${BOOT_DIR}/loader/loader.conf"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be success
  End

  It 'fails if cleanup restored a different UKI'
    printf 'default vmlinuz-other.efi\n' > "${BOOT_DIR}/loader/loader.conf"
    touch "${BOOT_DIR}/EFI/Linux/vmlinuz-other.efi"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be failure
    The stderr should include 'did not restore'
    The path "${BOOT_DIR}/flatcar/first_boot" should not be exist
  End

  It 'fails if the template is missing'
    rm "${BOOT_DIR}/acl/uki-addons/firstboot.addon.efi"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be failure
    The stderr should include 'missing firstboot addon template'
  End

  It 'fails if the restored addon differs'
    printf 'wrong-addon\n' > "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d/firstboot.addon.efi"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be failure
    The stderr should include 'did not restore'
  End

  It 'fails for an ambiguous boot default'
    printf 'default vmlinuz-*.efi\n' > "${BOOT_DIR}/loader/loader.conf"
    When call verifyAndRearmACLFirstBoot "${BOOT_DIR}"
    The status should be failure
    The stderr should include 'default UKI does not exist'
  End
End
