#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'AZL3 Kata-CC signed payload'
  Include ./vhdbuilder/packer/packer_source.sh
  setup() {
    TEST_DIR=$(mktemp -d)
    mkdir -p "$TEST_DIR/source/nested"
    for name in kata-containers.img kata-containers-cc.img kata-containers-igvm.img igvm-measurement.cose reference-info-base64; do
      printf 'test-only %s\n' "$name" > "$TEST_DIR/source/nested/$name"
    done
    PREPARE_SCRIPT="$PWD/vhdbuilder/packer/prepare-kata-cc-artifacts.sh"
  }
  cleanup() { rm -rf "$TEST_DIR"; }
  prepare() {
    (cd "$TEST_DIR" && KATA_CC_SIGNING_BUILD_ID=123 bash "$PREPARE_SCRIPT" source payload)
  }
  BeforeEach setup
  AfterEach cleanup

  It 'normalizes nested signed artifacts and records the pinned run'
    When call prepare
    The status should be success
    The contents of file "$TEST_DIR/payload/signing-build-id" should eq '123'
    The contents of file "$TEST_DIR/payload/kata-containers-igvm.img" should eq 'test-only kata-containers-igvm.img'
  End

  It 'fails on missing endorsement rather than using an unsigned fallback'
    rm "$TEST_DIR/source/nested/igvm-measurement.cose"
    When run prepare
    The status should be failure
    The stderr should include 'Expected one nonempty signed payload file: igvm-measurement.cose'
  End

  It 'rejects ambiguous artifacts'
    printf duplicate > "$TEST_DIR/source/kata-containers.img"
    When run prepare
    The status should be failure
    The stderr should include 'Expected one nonempty signed payload file: kata-containers.img'
  End

  It 'installs the complete payload at the current package location'
    prepare
    When call installKataCCPayload "$TEST_DIR/payload" "$TEST_DIR/installed"
    The status should be success
    The contents of file "$TEST_DIR/installed/igvm-measurement.cose" should eq 'test-only igvm-measurement.cose'
    The contents of file "$TEST_DIR/installed/kata-containers-cc.img" should eq 'test-only kata-containers-cc.img'
    The contents of file "$TEST_DIR/installed/signing-build-id" should eq '123'
  End
End
