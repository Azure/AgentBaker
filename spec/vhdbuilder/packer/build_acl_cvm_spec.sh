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
        (.builders[0] | has("managed_image_name") | not) and
        (.builders[0] | has("managed_image_resource_group_name") | not) and
        (.builders[0] | has("secure_boot_enabled") | not) and
        (.builders[0] | has("vtpm_enabled") | not) and
        (.builders[0] | has("security_type") | not) and
        (.builders[0] | has("security_encryption_type") | not) and
        (.builders[0].shared_image_gallery_destination | has("confidential_vm_image_encryption_type") | not) and
        (.builders[0].shared_image_gallery_destination | has("specialized") | not) and
        .provisioners == [{"type":"shell","inline":["true"]}]
      ' "$CAPTURED_TEMPLATE" >/dev/null
  }

  It 'reuses the ACL template without changing the builder or provisioners'
    ACL_PACKER_TEMPLATE="$BASE_TEMPLATE"
    export ACL_PACKER_TEMPLATE
    When call build_and_validate_cvm_template
    The status should be success
    The output should include "Using pre-CPS ACL image settings derived from $BASE_TEMPLATE"
  End
End
