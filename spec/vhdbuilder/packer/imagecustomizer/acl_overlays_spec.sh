#!/usr/bin/env shellspec

Describe 'ACL overlay image publication'
  setup() {
    ROOT_DIR="$(pwd)"
    TEST_ROOT="$(mktemp -d)"
    COMMAND_LOG="$TEST_ROOT/commands"
    mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/metadata"
    : > "$COMMAND_LOG"
    cat > "$TEST_ROOT/bin/az" <<'EOF'
#!/bin/bash
printf 'az %s\n' "$*" >> "$COMMAND_LOG"
if [[ "${FAIL_SIG:-0}" == 1 ]]; then exit 1; fi
echo /subscriptions/test/resourceGroups/images/providers/Microsoft.Compute/galleries/gallery/images/acl/versions/1.12345.100008
EOF
    cat > "$TEST_ROOT/bin/azcopy" <<'EOF'
#!/bin/bash
printf 'azcopy %s\n' "$*" >> "$COMMAND_LOG"
if [[ "$3" == https:* ]]; then
    [[ "${FAIL_UPLOAD:-0}" == 0 ]] || exit 1
    [[ "$(cat "$2")" == customized ]] || exit 1
else
    [[ "${FAIL_DOWNLOAD:-0}" == 0 ]] || exit 1
    printf original > "$3"
fi
EOF
    cat > "$TEST_ROOT/bin/docker" <<'EOF'
#!/bin/bash
printf 'docker %s\n' "$*" >> "$COMMAND_LOG"
if [[ "$1" == pull ]]; then
    [[ "${FAIL_PULL:-0}" == 0 || "$*" == *customizer:fallback* ]]
    exit $?
fi
case " $* " in
  *" customize "*)
    [[ "${FAIL_CUSTOMIZE:-0}" == 0 ]] || exit 1
    for arg in "$@"; do
      if [[ "$arg" == *:/customize ]]; then
        work_dir="${arg%:/customize}"
        [[ "${NO_OUTPUT:-0}" == 0 ]] && printf customized > "$work_dir/out/image.vhd"
        exit 0
      fi
    done
    exit 1
    ;;
  *) exit 1 ;;
esac
EOF
    cat > "$TEST_ROOT/bin/uname" <<'EOF'
#!/bin/bash
printf '%s\n' "${NATIVE_ARCH:-x86_64}"
EOF
    chmod +x "$TEST_ROOT/bin/az" "$TEST_ROOT/bin/azcopy" "$TEST_ROOT/bin/docker" "$TEST_ROOT/bin/uname"
    export ROOT_DIR TEST_ROOT COMMAND_LOG
    export PATH="$TEST_ROOT/bin:$PATH"
    export OS_SKU=AzureContainerLinux CAPTURED_SIG_VERSION=1.12345.8
    export SIG_IMAGE_NAME=acl SIG_GALLERY_NAME=gallery RESOURCE_GROUP_NAME=images
    export PACKER_BUILD_LOCATION=eastus REPLICATIONS=westus=2,eastus
    export DESTINATION_STORAGE_CONTAINER=https://storage/staging STORAGE_ACCOUNT_NAME=storage
    export VHD_ARTIFACT_NAME=acl-tl-gen2 E2E_METADATA_DIR="$TEST_ROOT/metadata"
    export IMG_CUSTOMIZER_CONTAINER=customizer:configured ARCHITECTURE=X86_64
    export GENERATE_PUBLISHING_INFO=false
  }
  cleanup() {
    rm -rf "$TEST_ROOT"
  }
  publish() {
    bash "$ROOT_DIR/vhdbuilder/packer/imagecustomizer/scripts/publish-acl-image.sh"
  }
  customize() {
    cd "$TEST_ROOT" || return
    bash "$ROOT_DIR/vhdbuilder/packer/imagecustomizer/scripts/customize-acl-overlays.sh"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'publishes a distinct customized SIG and points E2E to it'
    When call publish
    The status should be success
    The contents of file "$COMMAND_LOG" should include '--gallery-image-version 1.12345.100008'
    The contents of file "$COMMAND_LOG" should include '--os-vhd-uri https://storage/staging/1.12345.8.vhd'
    The contents of file "$TEST_ROOT/metadata/acl-tl-gen2.json" should include '"version": "1.12345.100008"'
    The contents of file "$TEST_ROOT/metadata/acl-tl-gen2.json" should include '"westus"'
  End

  It 'does not publish an intermediate SIG for another OS'
    OS_SKU=Ubuntu
    When call publish
    The status should be failure
    The contents of file "$COMMAND_LOG" should equal ''
  End

  It 'rejects versions that overlap the customized version range'
    CAPTURED_SIG_VERSION=1.12345.100000
    When call publish
    The status should be failure
    The error should include 'patch component below 100000'
    The contents of file "$COMMAND_LOG" should equal ''
  End

  It 'does not write E2E metadata when SIG creation fails'
    export FAIL_SIG=1
    When call publish
    The status should be failure
    The path "$TEST_ROOT/metadata/acl-tl-gen2.json" should not be exist
  End

  It 'replaces the captured SIG in VHD publishing info'
    GENERATE_PUBLISHING_INFO=true
    export ACL_PUBLISHING_INPUT="$TEST_ROOT/input.json"
    printf '{"vhd_url":"https://storage/published/1.12345.8.vhd","captured_sig_resource_id":"original"}' > "$ACL_PUBLISHING_INPUT"
    publish_in_workdir() { cd "$TEST_ROOT" && publish; }
    When call publish_in_workdir
    The status should be success
    The contents of file "$TEST_ROOT/vhd-publishing-info.json" should include '/versions/1.12345.100008'
    The contents of file "$TEST_ROOT/vhd-publishing-info.json" should include 'https://storage/published/1.12345.8.vhd'
  End

  It 'customizes the staged VHD without COSI or PMC configuration'
    When call customize
    The status should be success
    The contents of file "$COMMAND_LOG" should include 'customizer:configured customize'
    The contents of file "$COMMAND_LOG" should include '--blob-type=PageBlob --overwrite=true'
    The contents of file "$COMMAND_LOG" should not include 'customizer:configured convert'
  End

  It 'does not replace the staged VHD if customization fails'
    export FAIL_CUSTOMIZE=1
    When call customize
    The status should be failure
    The contents of file "$COMMAND_LOG" should not include '--blob-type=PageBlob'
  End

  It 'does not customize another OS'
    OS_SKU=Ubuntu
    When call customize
    The status should be failure
    The error should include 'must not run for other OS SKUs'
    The contents of file "$COMMAND_LOG" should equal ''
  End

  It 'rejects a worker with the wrong architecture before downloading'
    ARCHITECTURE=ARM64
    When call customize
    The status should be failure
    The error should include 'requires a worker matching'
    The contents of file "$COMMAND_LOG" should equal ''
  End

  It 'uses the native ARM64 container for ARM64 input'
    export NATIVE_ARCH=aarch64
    ARCHITECTURE=ARM64
    When call customize
    The status should be success
    The contents of file "$COMMAND_LOG" should include '--platform linux/arm64'
  End

  It 'uses the configured fallback if the primary container is unavailable'
    export FAIL_PULL=1 IMG_CUSTOMIZER_CONTAINER_FALLBACK=customizer:fallback
    When call customize
    The status should be success
    The contents of file "$COMMAND_LOG" should include 'customizer:fallback customize'
  End

  It 'does not upload when no configured container can be pulled'
    export FAIL_PULL=1
    unset IMG_CUSTOMIZER_CONTAINER_FALLBACK
    When call customize
    The status should be failure
    The error should include 'no fallback is configured'
    The contents of file "$COMMAND_LOG" should not include '--blob-type=PageBlob'
  End

  It 'does not run ImageCustomizer when the download fails'
    export FAIL_DOWNLOAD=1
    When call customize
    The status should be failure
    The contents of file "$COMMAND_LOG" should not include 'docker'
  End

  It 'does not upload a missing ImageCustomizer output'
    export NO_OUTPUT=1
    When call customize
    The status should be failure
    The contents of file "$COMMAND_LOG" should not include '--blob-type=PageBlob'
  End

  It 'propagates failure to upload the customized VHD'
    export FAIL_UPLOAD=1
    When call customize
    The status should be failure
    The contents of file "$COMMAND_LOG" should include '--blob-type=PageBlob'
  End
End
