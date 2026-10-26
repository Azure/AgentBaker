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
    [[ "$(cat "$2")" == customized ]] || exit 1
else
    printf original > "$3"
fi
EOF
    cat > "$TEST_ROOT/bin/docker" <<'EOF'
#!/bin/bash
printf 'docker %s\n' "$*" >> "$COMMAND_LOG"
[[ "$1" == pull ]] && exit 0
case " $* " in
  *" customize "*)
    [[ "${FAIL_CUSTOMIZE:-0}" == 0 ]] || exit 1
    printf customized > "$TEST_ROOT/cosi-convert/out/1.12345.8.vhd"
    ;;
  *" convert "*)
    [[ "${FAIL_CONVERT:-0}" == 0 ]] || exit 1
    [[ "$(cat "$TEST_ROOT/cosi-convert/1.12345.8.vhd")" == customized ]] || exit 1
    printf '{}' > "$TEST_ROOT/metadata.json"
    tar -cf "$TEST_ROOT/cosi-convert/out/1.12345.8.cosi" -C "$TEST_ROOT" metadata.json
    ;;
  *) exit 1 ;;
esac
EOF
    chmod +x "$TEST_ROOT/bin/az" "$TEST_ROOT/bin/azcopy" "$TEST_ROOT/bin/docker"
    export ROOT_DIR TEST_ROOT COMMAND_LOG
    export PATH="$TEST_ROOT/bin:$PATH"
    export OS_SKU=AzureContainerLinux CAPTURED_SIG_VERSION=1.12345.8
    export SIG_IMAGE_NAME=acl SIG_GALLERY_NAME=gallery RESOURCE_GROUP_NAME=images
    export PACKER_BUILD_LOCATION=eastus REPLICATIONS=westus=2,eastus
    export DESTINATION_STORAGE_CONTAINER=https://storage/staging STORAGE_ACCOUNT_NAME=storage
    export VHD_ARTIFACT_NAME=acl-tl-gen2 E2E_METADATA_DIR="$TEST_ROOT/metadata"
    export IMG_CUSTOMIZER_CONTAINER=customizer:configured AFD_DOWNLOAD_HOSTNAME=download COSI_CONTAINER=cosi
    export IMAGE_VERSION=202610.26.0 GENERATE_PUBLISHING_INFO=false
  }
  cleanup() {
    rm -rf "$TEST_ROOT"
  }
  publish() {
    bash "$ROOT_DIR/vhdbuilder/packer/imagecustomizer/scripts/publish-acl-image.sh"
  }
  convert() {
    cd "$TEST_ROOT" || return
    bash "$ROOT_DIR/vhdbuilder/packer/imagecustomizer/scripts/convert-vhd-to-cosi.sh"
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

  It 'customizes before COSI conversion and replaces the staged VHD'
    When call convert
    The status should be success
    The output should be present
    The contents of file "$COMMAND_LOG" should include 'customizer:configured customize'
    The contents of file "$COMMAND_LOG" should include 'customizer:configured convert'
    The contents of file "$COMMAND_LOG" should include '--blob-type=PageBlob --overwrite=true'
    The path "$TEST_ROOT/1.12345.8.cosi" should be exist
  End

  It 'customizes a PR VHD without COSI or PMC configuration'
    export GENERATE_COSI=false
    unset AFD_DOWNLOAD_HOSTNAME COSI_CONTAINER
    When call convert
    The status should be success
    The output should be present
    The contents of file "$COMMAND_LOG" should include '--blob-type=PageBlob --overwrite=true'
    The contents of file "$COMMAND_LOG" should not include 'customizer:configured convert'
  End

  It 'does not replace the staged VHD if customization fails'
    export FAIL_CUSTOMIZE=1
    When call convert
    The status should be failure
    The output should be present
    The contents of file "$COMMAND_LOG" should not include '--blob-type=PageBlob'
    The contents of file "$COMMAND_LOG" should not include 'customizer:configured convert'
  End

  It 'does not replace the staged VHD if COSI conversion fails'
    export FAIL_CONVERT=1
    When call convert
    The status should be failure
    The output should be present
    The contents of file "$COMMAND_LOG" should not include '--blob-type=PageBlob'
  End
End
