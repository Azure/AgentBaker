#!/usr/bin/env shellspec

Describe 'ACL candidate creation'
  setup() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/build-acl"
    mkdir -p "${TEST_ROOT}/bin"
    CAPTURED_TEMPLATE="${TEST_ROOT}/captured.json"
    cat > "${TEST_ROOT}/bin/packer" <<'EOF'
#!/bin/sh
for argument in "$@"; do
  template=$argument
done
cp "$template" "$CAPTURED_TEMPLATE"
exit "${PACKER_EXIT_CODE:-0}"
EOF
    chmod +x "${TEST_ROOT}/bin/packer"
    export CAPTURED_TEMPLATE
    PATH="${TEST_ROOT}/bin:${PATH}"
    export PATH
  }
  BeforeEach 'setup'

  build_tl_templates() {
    for template in vhdbuilder/packer/vhd-image-builder-acl.json vhdbuilder/packer/vhd-image-builder-acl-arm64.json; do
      cp "$template" "${TEST_ROOT}/original.json"
      bash vhdbuilder/packer/build-acl.sh "$template" || return
      cmp "$template" "${TEST_ROOT}/original.json" || return
      if [ "${ACL_CANDIDATE_IMAGE:-false}" = "true" ]; then
        jq -e '.builders[0].azure_tags["abe2e-ignore"] == "true"' "$CAPTURED_TEMPLATE" >/dev/null || return
        jq 'del(.builders[0].azure_tags["abe2e-ignore"])' "$CAPTURED_TEMPLATE" > "${TEST_ROOT}/without-tag.json"
        jq . "$template" > "${TEST_ROOT}/formatted.json"
        cmp "${TEST_ROOT}/without-tag.json" "${TEST_ROOT}/formatted.json" || return
      else
        cmp "$template" "$CAPTURED_TEMPLATE" || return
      fi
    done
  }

  It 'tags both TL architectures before Packer without changing anything else'
    ACL_CANDIDATE_IMAGE=true
    export ACL_CANDIDATE_IMAGE
    When call build_tl_templates
    The status should be success
    The output should include 'Building ACL candidate with abe2e-ignore=true at creation'
  End

  It 'passes both non-candidate TL templates byte for byte'
    unset ACL_CANDIDATE_IMAGE
    When call build_tl_templates
    The status should be success
    The output should be blank
  End

  It 'propagates a Packer failure after preparing the candidate template'
    ACL_CANDIDATE_IMAGE=true
    PACKER_EXIT_CODE=17
    export ACL_CANDIDATE_IMAGE PACKER_EXIT_CODE
    When run bash vhdbuilder/packer/build-acl.sh vhdbuilder/packer/vhd-image-builder-acl.json
    The status should equal 17
    The output should include 'Building ACL candidate with abe2e-ignore=true at creation'
  End

  verify_gallery_tag() (
    set -euo pipefail
    script=$(sed -n '/        shopt -s nocasematch/,/    condition: and(succeeded(), eq(variables.ACL_CANDIDATE_IMAGE/ {
      /    condition:/d
      p
    }' .pipelines/templates/.builder-release-template.yaml)
    eval "$script"
  )

  setup_gallery() {
    MANAGED_SIG_ID=/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/1.0.0
    export MANAGED_SIG_ID
    REPAIRED_TAG="${TEST_ROOT}/repaired-tag"
    rm -f "$REPAIRED_TAG"
    export REPAIRED_TAG
    cat > "${TEST_ROOT}/bin/az" <<'EOF'
#!/bin/sh
case "$1 $2 $3" in
  "sig image-version show")
    if [ -f "$REPAIRED_TAG" ]; then
      cat "$REPAIRED_TAG"
    else
      printf '%s\n' "${GALLERY_TAG:-}"
    fi
    ;;
  "sig image-version update")
    if [ "${GALLERY_TAG:-}" = "true" ]; then
      echo "Unexpected gallery write" >&2
      exit 99
    fi
    printf '%s\n' "${REPAIR_TAG_VALUE:-true}" > "$REPAIRED_TAG"
    ;;
  *)
    echo "Unexpected gallery command" >&2
    exit 99
    ;;
esac
EOF
    chmod +x "${TEST_ROOT}/bin/az"
  }

  It 'hides a missed creation tag but still fails the actual pipeline verification'
    setup_gallery
    unset GALLERY_TAG
    When call verify_gallery_tag
    The status should be failure
    The output should include 'was created without abe2e-ignore=true'
    The output should include 'after a missed creation tag; this job still fails'
    The contents of file "$REPAIRED_TAG" should equal "true"
  End

  It 'fails explicitly if repairing a missed creation tag does not stick'
    setup_gallery
    unset GALLERY_TAG
    REPAIR_TAG_VALUE=false
    export REPAIR_TAG_VALUE
    When call verify_gallery_tag
    The status should be failure
    The output should include 'still has no abe2e-ignore=true tag after repair'
  End

  It 'verifies an existing creation tag using only a gallery read'
    setup_gallery
    GALLERY_TAG=true
    export GALLERY_TAG
    When call verify_gallery_tag
    The status should be success
    The output should include 'was created with abe2e-ignore=true'
  End
End
