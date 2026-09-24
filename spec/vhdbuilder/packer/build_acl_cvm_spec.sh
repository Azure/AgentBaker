#!/usr/bin/env shellspec

Describe 'build-acl-cvm.sh'
  setup() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/build-acl-cvm"
    MOCK_BIN="${TEST_ROOT}/bin"
    BASE_TEMPLATE="${TEST_ROOT}/acl.json"
    CAPTURED_TEMPLATE="${TEST_ROOT}/acl-cvm.json"
    rm -rf "$TEST_ROOT"
    mkdir -p "$MOCK_BIN"
    cat > "$BASE_TEMPLATE" <<'EOF'
{"builders":[{"type":"azure-arm","shared_image_gallery_destination":{"image_name":"acl"}}],"provisioners":[{"type":"shell","inline":["true"]}]}
EOF
    cat > "${MOCK_BIN}/packer" <<'EOF'
#!/bin/sh
for argument in "$@"; do
  template=$argument
done
cp "$template" "$CAPTURED_TEMPLATE"
EOF
    chmod +x "${MOCK_BIN}/packer"
    export CAPTURED_TEMPLATE
    PATH="${MOCK_BIN}:${PATH}"
    export PATH
  }

  BeforeEach 'setup'

  build_and_validate_cvm_template() {
    ./vhdbuilder/packer/build-acl-cvm.sh &&
      jq -e '
        (.builders[0].managed_image_name == "{{user `sig_image_name`}}-{{user `captured_sig_version`}}") and
        (.builders[0].managed_image_resource_group_name == "{{user `resource_group_name`}}") and
        (.builders[0] | has("secure_boot_enabled") | not) and
        (.builders[0] | has("vtpm_enabled") | not) and
        (.builders[0] | has("security_type") | not) and
        (.builders[0] | has("security_encryption_type") | not) and
        (.builders[0].shared_image_gallery_destination | has("confidential_vm_image_encryption_type") | not) and
        (.builders[0].shared_image_gallery_destination | has("specialized") | not) and
        .provisioners == [{"type":"shell","inline":["true"]}]
      ' "$CAPTURED_TEMPLATE" >/dev/null
  }

  It 'publishes via a managed image without changing security settings or provisioners'
    ACL_PACKER_TEMPLATE="$BASE_TEMPLATE"
    export ACL_PACKER_TEMPLATE
    When call build_and_validate_cvm_template
    The status should be success
    The output should include "Using pre-CPS ACL image settings derived from $BASE_TEMPLATE"
  End

  It 'keeps the production Packer destination generalized and uses generalized ACL CVM deployment flags'
    # These assertions intentionally match literal Packer and shell expressions.
    # shellcheck disable=SC2016
    When call sh -c '
      jq -e '\''
        .variables.sig_image_name == "{{env `SIG_IMAGE_NAME`}}" and
        .builders[0].shared_image_gallery_destination.image_name == "{{user `sig_image_name`}}" and
        (.builders[0].shared_image_gallery_destination | has("specialized") | not)
      '\'' vhdbuilder/packer/vhd-image-builder-acl.json >/dev/null &&
      grep -F -- '\''--security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly'\'' vhdbuilder/packer/test/run-test.sh >/dev/null &&
      grep -F '\''if [ "${OS_SKU:-}" != "AzureContainerLinux" ]; then'\'' vhdbuilder/packer/test/run-test.sh >/dev/null &&
      grep -F '\''TARGET_COMMAND_STRING+=" --specialized true"'\'' vhdbuilder/packer/test/run-test.sh >/dev/null &&
      grep -F '\''TEST_VM_USER_DATA_ARGS=(--user-data "@./vhdbuilder/packer/acl-customdata.json")'\'' vhdbuilder/packer/test/run-test.sh >/dev/null &&
      grep -F -- '\''--security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly'\'' vhdbuilder/packer/vhd-scanning.sh >/dev/null &&
      grep -F '\''if [ "${OS_SKU:-}" != "AzureContainerLinux" ]; then'\'' vhdbuilder/packer/vhd-scanning.sh >/dev/null &&
      grep -F '\''VM_OPTIONS+=" --specialized true"'\'' vhdbuilder/packer/vhd-scanning.sh >/dev/null
    '
    The status should be success
  End
End
