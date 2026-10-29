#!/bin/bash

setup_boot_fixture() {
  BOOT_DIR="$(mktemp -d)"
  mkdir -p "${BOOT_DIR}/acl/uki-addons" "${BOOT_DIR}/loader" \
    "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d"
  printf 'default vmlinuz-test.efi\n' > "${BOOT_DIR}/loader/loader.conf"
  touch "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi"
  printf 'signed-addon-fixture\n' > "${BOOT_DIR}/acl/uki-addons/firstboot.addon.efi"
  cp "${BOOT_DIR}/acl/uki-addons/firstboot.addon.efi" \
    "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d/firstboot.addon.efi"
}
cleanup_boot_fixture() {
  rm -rf "${BOOT_DIR}"
}

Describe 'ACL workload privilege boundary'
  Skip if 'already running as root' [ "${EUID}" -eq 0 ]

  It 'reexecutes the workload with noninteractive sudo before touching the image'
    Mock sudo
      printf '%s\n' "$*"
    End
    When run bash ./vhdbuilder/scripts/acl-prefetch-driver.sh
    The status should be success
    The output should equal '-n /bin/bash ./vhdbuilder/scripts/acl-prefetch-driver.sh'
  End

  It 'propagates a denied sudo invocation'
    Mock sudo
      echo 'sudo unavailable' >&2
      exit 23
    End
    When run bash ./vhdbuilder/scripts/acl-prefetch-driver.sh
    The status should equal 23
    The stderr should equal 'sudo unavailable'
  End
End

Describe 'ACL capture phase selection'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  It 'defaults to workload execution'
    When call aclPrefetchPhase
    The status should be success
    The output should equal workload
  End

  It 'selects customization for prepare-only'
    When call aclPrefetchPhase --prepare-only
    The status should be success
    The output should equal customization
  End

  It 'rejects unknown arguments'
    When call aclPrefetchPhase --unknown
    The status should equal 2
    The stderr should include Usage
  End

  It 'rejects extra arguments'
    When call aclPrefetchPhase --prepare-only extra
    The status should equal 2
    The stderr should include Usage
  End
End

Describe 'ACL experimental AIB template'
  validate_template() {
    jq -e '
      .properties.buildTimeoutInMinutes == 60 and
      (.properties.customize | length) == 2 and
      .properties.customize[0].type == "File" and
      .properties.customize[0].sourceUri == .properties.optimize.workload.scriptUri and
      .properties.customize[0].destination == "/tmp/acl-prefetch-driver.sh" and
      .properties.customize[1].type == "Shell" and
      .properties.customize[1].inline == [
        "sudo /bin/bash /tmp/acl-prefetch-driver.sh --prepare-only && sudo rm -f /tmp/acl-prefetch-driver.sh"
      ] and .properties.vmProfile.vmSize == "Standard_D4s_v6" and
      .properties.optimize.vmBoot.state == "Enabled" and
      .properties.optimize.workload.state == "Enabled"
    ' vhdbuilder/prefetch/templates/acl-experiment.json
  }
  It 'rearms first boot at customization capture using the same pinned workload helper'
    When call validate_template
    The status should be success
    The output should equal true
  End
End

Describe 'ACL capture and workload execution'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  setup_phase() {
    setup_boot_fixture
    PREFETCH_SCRIPT="${BOOT_DIR}/prefetch.sh"
    CLEANUP_SCRIPT="${BOOT_DIR}/cleanup.sh"
    TRACE="${BOOT_DIR}/trace"
    export BOOT_DIR TRACE
    rm "${BOOT_DIR}/EFI/Linux/vmlinuz-test.efi.extra.d/firstboot.addon.efi"
    printf 'printf "warm\\n" >> "$TRACE"\n' > "${PREFETCH_SCRIPT}"
    cat > "${CLEANUP_SCRIPT}" <<'SCRIPT'
printf 'cleanup\n' >> "$TRACE"
cp "$BOOT_DIR/acl/uki-addons/firstboot.addon.efi" "$BOOT_DIR/EFI/Linux/vmlinuz-test.efi.extra.d/firstboot.addon.efi"
SCRIPT
    systemctl() { printf 'systemctl %s\n' "$*" >> "${TRACE}"; }
    sync() { printf 'sync\n' >> "${TRACE}"; }
  }
  BeforeEach 'setup_phase'
  AfterEach 'cleanup_boot_fixture'

  It 'prepares capture without warming or deleting the baked scripts'
    expected="$(printf 'cleanup\nsync')"
    When call runACLPhase customization "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}" "${BOOT_DIR}"
    The status should be success
    The contents of file "${TRACE}" should equal "${expected}"
    The path "${PREFETCH_SCRIPT}" should be exist
    The path "${CLEANUP_SCRIPT}" should be exist
    The path "${BOOT_DIR}/flatcar/first_boot" should be exist
  End

  It 'warms before cleanup, rearms, then removes the workload-only scripts'
    expected="$(printf 'systemctl start containerd\nwarm\ncleanup\nsync')"
    When call runACLPhase workload "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}" "${BOOT_DIR}"
    The status should be success
    The contents of file "${TRACE}" should equal "${expected}"
    The path "${PREFETCH_SCRIPT}" should not be exist
    The path "${CLEANUP_SCRIPT}" should not be exist
    The path "${BOOT_DIR}/flatcar/first_boot" should be exist
  End

  It 'stops when warming fails instead of reporting a successful reset'
    printf 'exit 19\n' > "${PREFETCH_SCRIPT}"
    When call runACLPhase workload "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}" "${BOOT_DIR}"
    The status should equal 19
    The path "${BOOT_DIR}/flatcar/first_boot" should not be exist
    The path "${CLEANUP_SCRIPT}" should be exist
  End

  It 'does not rearm or delete scripts after a failed cleanup'
    printf 'exit 17\n' > "${CLEANUP_SCRIPT}"
    When call runACLPhase customization "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}" "${BOOT_DIR}"
    The status should equal 17
    The path "${BOOT_DIR}/flatcar/first_boot" should not be exist
    The path "${CLEANUP_SCRIPT}" should be exist
  End
End

Describe 'ACL workload prefetch first-boot postcondition'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  BeforeEach 'setup_boot_fixture'
  AfterEach 'cleanup_boot_fixture'

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
