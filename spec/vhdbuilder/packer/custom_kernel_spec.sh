#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'custom kernel packages'
  Include ./vhdbuilder/packer/custom-kernel/manage.sh

  setup() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_WORKDIR}/custom-kernel.XXXXXX")
    PACKAGE_DIR="$TEST_ROOT/packages"
    mkdir -p "$PACKAGE_DIR"
    RELEASE=7.0.0-1018-azure
    VERSION=7.0.0-1018.18
    create_package "linux-image-unsigned-$RELEASE" "$VERSION" amd64 image.deb
    create_package "linux-modules-$RELEASE" "$VERSION" amd64 modules.deb
    write_manifest
    export ENVIRONMENT=test OS_SKU=Ubuntu OS_VERSION=26.04 ARCHITECTURE=X86_64
    export HYPERV_GENERATION=V2 FEATURE_FLAGS=minimal ENABLE_FIPS=False
    export ENABLE_TRUSTED_LAUNCH=False GENERATE_PUBLISHING_INFO=False BUILDER=packer
  }

  create_package() {
    local control_dir="$TEST_ROOT/control-$4"
    mkdir -p "$control_dir/DEBIAN"
    printf 'Package: %s\nVersion: %s\nArchitecture: %s\nMaintainer: Test <test@example.invalid>\nDescription: Test fixture\n' \
      "$1" "$2" "$3" > "$control_dir/DEBIAN/control"
    dpkg-deb --build "$control_dir" "$PACKAGE_DIR/$4" >/dev/null
  }

  write_manifest() {
    printf '%s\n' "$RELEASE" > "$PACKAGE_DIR/kernel-release"
    (cd "$PACKAGE_DIR" && LC_ALL=C sha256sum -- *.deb kernel-release > SHA256SUMS)
  }

  BeforeEach 'setup'

  It 'accepts a matching image and modules with a complete checksum manifest'
    When call validate_kernel_manifest "$PACKAGE_DIR"
    The status should be success
    The output should equal "$RELEASE"
  End

  It 'accepts matching common and amd64 header packages'
    create_package "linux-azure-headers-${RELEASE%-azure}" "$VERSION" all common-headers.deb
    create_package "linux-headers-$RELEASE" "$VERSION" amd64 headers.deb
    write_manifest

    When call validate_kernel_manifest "$PACKAGE_DIR"
    The status should be success
    The output should equal "$RELEASE"
  End

  Describe 'invalid package sets'
    Parameters
      'missing modules' 'Supply linux-modules'
      'wrong architecture' 'Only amd64 packages'
      'mismatched version' 'Package versions must match'
      'changed payload' 'SHA256SUMS does not match'
      'extra unhashed package' 'SHA256SUMS does not match'
      'wrong release' 'kernel-release does not match'
    End

    It "rejects $1"
      case "$1" in
        'missing modules') rm "$PACKAGE_DIR/modules.deb" ;;
        'wrong architecture') create_package "linux-modules-$RELEASE" "$VERSION" arm64 modules.deb ;;
        'mismatched version') create_package "linux-modules-$RELEASE" 7.0.0-1018.19 amd64 modules.deb ;;
        'changed payload') printf '\n' >> "$PACKAGE_DIR/modules.deb" ;;
        'extra unhashed package') create_package "linux-modules-extra-$RELEASE" "$VERSION" amd64 extra.deb ;;
        'wrong release') echo 7.0.0-9999-azure > "$PACKAGE_DIR/kernel-release" ;;
      esac

      When call validate_kernel_manifest "$PACKAGE_DIR"
      The status should be failure
      The error should include "$2"
    End
  End

  Describe 'unsafe blob prefixes'
    Parameters
      ../escape
      /absolute
      'test/*'
      test/../escape
    End

    It "rejects $1"
      When call validate_kernel_prefix "$1"
      The status should be failure
      The error should include 'relative blob prefix'
    End
  End

  Describe 'unsupported build configurations'
    Parameters
      ENVIRONMENT prod
      GENERATE_PUBLISHING_INFO True
      ARCHITECTURE ARM64
      ENABLE_TRUSTED_LAUNCH True
      ENABLE_FIPS True
      OS_VERSION 24.04
      FEATURE_FLAGS minimal,cvm
    End

    It "rejects $1=$2 before accessing Azure"
      export "$1=$2"
      az() { echo 'unexpected Azure call'; }

      When call download_kernel_packages subscription account container test/run "$TEST_ROOT/download"
      The status should be failure
      The error should include 'Only non-publishing test builds'
      The output should not include 'unexpected Azure call'
      The path "$TEST_ROOT/download" should not be exist
    End
  End

  It 'uploads using login, private access, and no overwrite, with the manifest last'
    az() {
      if [ "$3" = show ]; then return 0; fi
      echo "az $*"
      echo "$*" > "$TEST_ROOT/last-azure-command"
    }

    When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR"
    The status should be success
    The output should include '--public-access off --auth-mode login'
    The output should include '--overwrite false --auth-mode login'
    The output should include '--name test/run/SHA256SUMS'
    The output should include "Uploaded $RELEASE"
    The contents of file "$TEST_ROOT/last-azure-command" should include '--name test/run/SHA256SUMS'
  End

  It 'stops upload if Azure rejects container access'
    az() { echo 'container access denied' >&2; return 1; }

    When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR"
    The status should be failure
    The error should equal 'container access denied'
    The output should not include 'Uploaded'
  End

  It 'refuses a previously existing public container'
    az() { if [ "$3" = show ]; then echo blob; fi; }

    When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR"
    The status should be failure
    The error should include 'Refusing to upload to a public container'
    The output should not include 'Uploaded'
  End

  It 'downloads and validates packages before staging them for Packer'
    az() {
      local destination
      while [ "$#" -gt 0 ]; do
        if [ "$1" = --destination ]; then destination="$2"; fi
        shift
      done
      mkdir -p "$destination/test/run"
      cp "$PACKAGE_DIR/"* "$destination/test/run/"
    }

    When call download_kernel_packages subscription account container test/run "$TEST_ROOT/download"
    The status should be success
    The output should include "Downloaded and verified test kernel $RELEASE"
    The contents of file "$TEST_ROOT/download/kernel-release" should equal "$RELEASE"
  End

  It 'does not install anything unless explicitly enabled'
    install_kernel_packages() { echo 'unexpected installation'; return 1; }

    When call custom_kernel_main install False
    The status should be success
    The output should equal ''
  End

  It 'stops installation before changing holds or GRUB if apt fails'
    require_test_kernel_host() { return 0; }
    apt-cache() { return 0; }
    apt-get() { echo "apt-get $*"; return 1; }
    apt-mark() { echo 'unexpected package hold'; }

    When call install_kernel_packages "$PACKAGE_DIR"
    The status should be failure
    The output should include '--reinstall --allow-downgrades'
    The output should not include 'unexpected package hold'
  End

  It 'fails before installing if the running-kernel headers would be unavailable'
    require_test_kernel_host() { return 0; }
    apt-cache() { return 1; }
    apt-get() { echo 'unexpected installation'; return 1; }

    When call install_kernel_packages "$PACKAGE_DIR"
    The status should be failure
    The error should include "Matching linux-headers-$RELEASE is unavailable"
    The output should not include 'unexpected installation'
  End

  It 'rejects a boot into the stock kernel'
    uname() { echo 7.0.0-1017-azure; }

    When call verify_kernel_boot "$PACKAGE_DIR"
    The status should be failure
    The error should include "Expected running kernel $RELEASE, got 7.0.0-1017-azure"
  End

  It 'records the running kernel and package checksums after reboot'
    uname() { echo "$RELEASE"; }
    tee() { cat; }

    When call verify_kernel_boot "$PACKAGE_DIR"
    The status should be success
    The output should include "CUSTOM_KERNEL_VERIFIED=$RELEASE"
    The output should include "$(cat "$PACKAGE_DIR/SHA256SUMS")"
  End
End
