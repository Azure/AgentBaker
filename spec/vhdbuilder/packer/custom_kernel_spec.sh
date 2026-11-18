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
    if [ -f "$PACKAGE_DIR/kernel-target" ]; then
      (cd "$PACKAGE_DIR" && LC_ALL=C sha256sum -- *.deb kernel-release kernel-target > SHA256SUMS)
    else
      (cd "$PACKAGE_DIR" && LC_ALL=C sha256sum -- *.deb kernel-release > SHA256SUMS)
    fi
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

  It 'accepts matching tools and cloud-tools with their ABI dependencies'
    create_package "linux-tools-$RELEASE" "$VERSION" amd64 tools.deb
    create_package "linux-cloud-tools-$RELEASE" "$VERSION" amd64 cloud-tools.deb
    create_package "linux-azure-tools-${RELEASE%-azure}" "$VERSION" amd64 abi-tools.deb
    create_package "linux-azure-cloud-tools-${RELEASE%-azure}" "$VERSION" amd64 abi-cloud-tools.deb
    write_manifest

    When call validate_kernel_manifest "$PACKAGE_DIR"
    The status should be success
    The output should equal "$RELEASE"
  End

  It 'accepts series-qualified NVIDIA headers, tools and cloud-tools'
    RELEASE=6.14.0-1003-azure-nvidia
    VERSION=6.14.0-1003.3
    create_package "linux-image-$RELEASE" "$VERSION" arm64 image.deb
    create_package "linux-modules-$RELEASE" "$VERSION" arm64 modules.deb
    create_package "linux-headers-$RELEASE" "$VERSION" arm64 headers.deb
    create_package linux-azure-nvidia-6.14-headers-6.14.0-1003 "$VERSION" all common-headers.deb
    create_package linux-azure-nvidia-6.14-tools-6.14.0-1003 "$VERSION" arm64 abi-tools.deb
    create_package linux-azure-nvidia-6.14-cloud-tools-6.14.0-1003 "$VERSION" arm64 abi-cloud-tools.deb
    echo ubuntu:24.04:arm64 > "$PACKAGE_DIR/kernel-target"
    write_manifest

    When call validate_kernel_manifest "$PACKAGE_DIR"
    The status should be success
    The output should equal "$RELEASE"
  End

  Describe 'invalid package sets'
    Parameters
      'missing modules' 'Supply linux-modules'
      'wrong architecture' 'Package architecture must match'
      'mismatched version' 'Package versions must match'
      'changed payload' 'SHA256SUMS does not match'
      'extra unhashed package' 'SHA256SUMS does not match'
      'wrong release' 'kernel-release does not match'
      'wrong tools ABI' 'Unexpected package'
      'mismatched tools version' 'Package versions must match'
      'wrong series tools ABI' 'Unexpected package'
      'mismatched series tools version' 'Package versions must match'
    End

    It "rejects $1"
      case "$1" in
        'missing modules') rm "$PACKAGE_DIR/modules.deb" ;;
        'wrong architecture') create_package "linux-modules-$RELEASE" "$VERSION" arm64 modules.deb ;;
        'mismatched version') create_package "linux-modules-$RELEASE" 7.0.0-1018.19 amd64 modules.deb ;;
        'changed payload') printf '\n' >> "$PACKAGE_DIR/modules.deb" ;;
        'extra unhashed package') create_package "linux-modules-extra-$RELEASE" "$VERSION" amd64 extra.deb ;;
        'wrong release') echo 7.0.0-9999-azure > "$PACKAGE_DIR/kernel-release" ;;
        'wrong tools ABI') create_package linux-tools-7.0.0-9999-azure "$VERSION" amd64 tools.deb ;;
        'mismatched tools version') create_package "linux-tools-$RELEASE" 7.0.0-1018.19 amd64 tools.deb ;;
        'wrong series tools ABI') create_package linux-azure-7.0-tools-7.0.0-9999 "$VERSION" amd64 tools.deb ;;
        'mismatched series tools version') create_package "linux-azure-7.0-tools-${RELEASE%-azure}" 7.0.0-1018.19 amd64 tools.deb ;;
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

  Describe 'Packer build-host validation'
    BeforeEach 'setup_packer_commands'

    setup_packer_commands() {
      mkdir -p "$TEST_ROOT/bin"
      printf '#!/bin/sh\necho "packer $*"\n' > "$TEST_ROOT/bin/packer"
      printf '#!/bin/sh\nexit 0\n' > "$TEST_ROOT/bin/az"
      chmod +x "$TEST_ROOT/bin/packer" "$TEST_ROOT/bin/az"
      export PATH="$TEST_ROOT/bin:$PATH"
      export CUSTOM_KERNEL_ENABLED=true
    }

    Describe 'unsupported configurations'
      Parameters
        run-packer ENVIRONMENT prod
        run-packer GENERATE_PUBLISHING_INFO True
        build-packer ENVIRONMENT prod
        build-packer GENERATE_PUBLISHING_INFO True
      End

      It "rejects $2=$3 before $1 invokes Packer"
        export "$2=$3"

        When run make --no-print-directory -f packer.mk "$1"
        The status should be failure
        The error should include 'Only non-publishing test builds'
        The output should not include 'packer '
      End
    End

    Describe 'allowed configurations'
      Parameters
        test true
        test True
        prod false
        prod ''
      End

      It "preserves build-packer with ENVIRONMENT=$1 and CUSTOM_KERNEL_ENABLED=$2"
        export ENVIRONMENT="$1" CUSTOM_KERNEL_ENABLED="$2"

        When run make --no-print-directory -f packer.mk build-packer
        The status should be success
        The output should include 'packer build'
      End
    End
  End

  Describe 'E2E VHD metadata'
    Parameters
      2404gen2containerd
      2404gen2containerd-customkernel
    End

    It "maps $1 to the scenario name without changing the image resource ID"
      export BUILD_ARTIFACTSTAGINGDIRECTORY="$TEST_ROOT" VHD_ARTIFACT_NAME=test
      export SIG_IMAGE_NAME="$1" PACKER_BUILD_LOCATION=eastus
      export CAPTURED_SIG_VERSION=202611.13.0
      export MANAGED_SIG_ID="/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/galleries/test/images/$1/versions/$CAPTURED_SIG_VERSION"
      export GENERATE_PUBLISHING_INFO=False
      awk '
        /^[[:space:]]*metadata_file=/ { copy = 1 }
        copy && /^    condition:/ { exit }
        copy { sub(/^        /, ""); print }
      ' .pipelines/templates/.builder-release-template.yaml > "$TEST_ROOT/metadata.sh"

      When run bash -euo pipefail "$TEST_ROOT/metadata.sh"
      The status should be success
      The output should include 'Created E2E VHD metadata'
      The contents of file "$TEST_ROOT/e2e-vhd-metadata/test.json" should equal "{\"2404gen2containerd\":{\"resourceId\":\"$MANAGED_SIG_ID\",\"version\":\"$CAPTURED_SIG_VERSION\",\"regions\":[\"eastus\"]}}"
    End
  End

  Describe 'unsupported build configurations'
    Parameters
      ENVIRONMENT prod
      GENERATE_PUBLISHING_INFO True
      ARCHITECTURE RISCV64
      ENABLE_TRUSTED_LAUNCH True
      ENABLE_FIPS True
      OS_VERSION 99.04
      OS_SKU AzureContainerLinux
      OS_SKU AzureLinux
      OS_SKU Mariner
      FEATURE_FLAGS minimal,cvm
      BUILDER imagecustomizer
    End

    It "rejects $1=$2 before accessing Azure"
      export "$1=$2"
      az() { echo 'unexpected Azure call'; }

      When call download_kernel_packages subscription account container test/run "$TEST_ROOT/download"
      The status should be failure
      The error should include 'Custom kernel:'
      The output should not include 'unexpected Azure call'
      The path "$TEST_ROOT/download" should not be exist
    End
  End

  Describe 'Ubuntu targets'
    Parameters
      22.04 amd64 X86_64
      22.04 arm64 ARM64
      24.04 amd64 X86_64
      24.04 arm64 ARM64
      26.04 arm64 ARM64
    End

    It "downloads a bundle for Ubuntu $1 $2 only into its matching build"
      create_package "linux-image-unsigned-$RELEASE" "$VERSION" "$2" image.deb
      create_package "linux-modules-$RELEASE" "$VERSION" "$2" modules.deb
      echo "ubuntu:$1:$2" > "$PACKAGE_DIR/kernel-target"
      write_manifest
      export OS_VERSION="$1" ARCHITECTURE="$3"
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
      The contents of file "$TEST_ROOT/download/kernel-target" should equal "ubuntu:$1:$2"
    End
  End

  It 'rejects a valid bundle for a different build release before staging it'
    export OS_VERSION=24.04
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
    The status should be failure
    The error should include 'does not match this build'
    The path "$TEST_ROOT/download" should not be exist
  End

  It 'checksums target metadata rather than trusting a changed release declaration'
    echo ubuntu:24.04:amd64 > "$PACKAGE_DIR/kernel-target"
    write_manifest
    echo ubuntu:22.04:amd64 > "$PACKAGE_DIR/kernel-target"

    When call validate_kernel_manifest "$PACKAGE_DIR"
    The status should be failure
    The error should include 'SHA256SUMS does not match'
  End

  It 'rejects a host whose distribution or architecture differs from the bundle'
    kernel_host_target() { echo ubuntu:24.04:arm64; }

    When call require_test_kernel_host "$PACKAGE_DIR"
    The status should be failure
    The error should include 'Installation requires a host matching ubuntu:26.04:amd64'
  End

  It 'rejects Secure Boot on a host that otherwise matches the bundle'
    kernel_host_target() { echo ubuntu:26.04:amd64; }
    mokutil() { echo 'SecureBoot enabled'; }

    When call require_test_kernel_host "$PACKAGE_DIR"
    The status should be failure
    The error should include 'Secure Boot enabled'
  End

  It 'accepts a non-Azure Ubuntu flavor without inventing Azure cloud-tools dependencies'
    RELEASE=6.8.0-100-generic
    VERSION=6.8.0-100.100
    create_package "linux-image-$RELEASE" "$VERSION" arm64 image.deb
    create_package "linux-modules-$RELEASE" "$VERSION" arm64 modules.deb
    create_package "linux-headers-$RELEASE" "$VERSION" arm64 headers.deb
    echo ubuntu:24.04:arm64 > "$PACKAGE_DIR/kernel-target"
    write_manifest
    require_test_kernel_host() { return 0; }
    apt-cache() { return 1; }
    apt-get() { echo "apt-get $*"; }
    apt-mark() { return 0; }
    check_kernel_boot_files() { return 0; }
    pin_kernel_boot() { return 0; }

    When call install_kernel_packages "$PACKAGE_DIR"
    The status should be success
    The output should include "linux-headers-$RELEASE"
    The output should not include 'linux-cloud-tools'
    The output should include "Installed test kernel $RELEASE"
  End

  Describe 'protected Ubuntu kernel flavors'
    Parameters
      azure-fips
      azure-cvm
      azure-mshv
      azure-fde
    End

    It "rejects $1 before uploading anything"
      RELEASE="7.0.0-1018-$1"
      create_package "linux-image-$RELEASE" "$VERSION" amd64 image.deb
      create_package "linux-modules-$RELEASE" "$VERSION" amd64 modules.deb
      az() { echo 'unexpected Azure call'; }

      When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR"
      The status should be failure
      The error should include 'Protected kernel flavors are not supported'
      The output should not include 'unexpected Azure call'
    End
  End

  Describe 'mixed or invalid package types'
    It 'rejects a mix of Debian and RPM packages'
      touch "$PACKAGE_DIR/kernel.rpm"

      When call validate_kernel_manifest "$PACKAGE_DIR"
      The status should be failure
      The error should include 'Only Ubuntu .deb packages are supported'
    End

    It 'rejects an RPM-only bundle before accessing Azure'
      rm "$PACKAGE_DIR/"*.deb
      touch "$PACKAGE_DIR/kernel.rpm"
      az() { echo 'unexpected Azure call'; }

      When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR"
      The status should be failure
      The error should include 'Only Ubuntu .deb packages are supported'
      The output should not include 'unexpected Azure call'
    End

    It 'rejects Azure Linux target metadata on Debian packages'
      echo azurelinux:3.0:amd64 > "$PACKAGE_DIR/kernel-target"
      write_manifest

      When call validate_kernel_manifest "$PACKAGE_DIR"
      The status should be failure
      The error should include 'Unsupported kernel-target'
    End

    It 'rejects an Azure Linux upload declaration before accessing Azure'
      az() { echo 'unexpected Azure call'; }

      When call upload_kernel_packages subscription account container test/run "$PACKAGE_DIR" azurelinux 3.0
      The status should be failure
      The error should include 'Unsupported kernel-target'
      The output should not include 'unexpected Azure call'
    End
  End

  Describe 'exact boot selection'
    It 'pins the requested Ubuntu entry after the ARM64 hardware selector'
      generate_kernel_grub_config() {
        mkdir -p "$(dirname "$1")"
        printf "menuentry 'Linux %s' --id 'gnulinux-%s-advanced-testuuid' {\n}\n" "$RELEASE" "$RELEASE" > "$1"
        echo 'set default="a-different-hardware-selected-kernel"' >> "$1"
        if [ -x "$TEST_ROOT/etc/grub.d/99_aks_test_kernel" ]; then
          "$TEST_ROOT/etc/grub.d/99_aks_test_kernel" >> "$1"
        fi
      }

      When call pin_kernel_boot "$RELEASE" "$TEST_ROOT"
      The status should be success
      The contents of file "$TEST_ROOT/etc/default/grub.d/99-aks-test-kernel.cfg" should include "GRUB_DEFAULT=\"gnulinux-$RELEASE-advanced-testuuid\""
      The contents of file "$TEST_ROOT/etc/grub.d/99_aks_test_kernel" should include "set default=\"gnulinux-$RELEASE-advanced-testuuid\""
    End

    It 'fails if GRUB has no exact entry for the requested kernel'
      generate_kernel_grub_config() {
        mkdir -p "$(dirname "$1")"
        printf "menuentry 'stock kernel' --id 'gnulinux-stock-advanced-testuuid' {\n}\n" > "$1"
      }

      When call pin_kernel_boot "$RELEASE" "$TEST_ROOT"
      The status should be failure
      The error should include 'Expected unique kernel entry is absent'
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

  It 'uploads the declared Ubuntu release and package architecture in the checksummed target'
    create_package "linux-image-unsigned-$RELEASE" "$VERSION" arm64 image.deb
    create_package "linux-modules-$RELEASE" "$VERSION" arm64 modules.deb
    az() {
      if [ "$3" = show ]; then return 0; fi
      local name="" file=""
      while [ "$#" -gt 0 ]; do
        if [ "$1" = --name ]; then name="$2"; fi
        if [ "$1" = --file ]; then file="$2"; fi
        shift
      done
      if [ "$name" = test/run/kernel-target ]; then
        cp "$file" "$TEST_ROOT/uploaded-target"
      fi
      if [ "$name" = test/run/SHA256SUMS ]; then
        cp "$file" "$TEST_ROOT/uploaded-checksums"
      fi
    }

    When call custom_kernel_main upload subscription account container test/run "$PACKAGE_DIR" ubuntu 24.04
    The status should be success
    The output should include "Uploaded $RELEASE"
    The contents of file "$TEST_ROOT/uploaded-target" should equal 'ubuntu:24.04:arm64'
    The contents of file "$TEST_ROOT/uploaded-checksums" should include 'kernel-target'
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
    The output should include "linux-tools-$RELEASE"
    The output should include "linux-cloud-tools-$RELEASE"
    The output should not include 'unexpected package hold'
  End

  Describe 'unavailable matching kernel dependencies'
    Parameters
      linux-headers
      linux-tools
      linux-cloud-tools
    End

    It "fails before installing if $1 would be unavailable"
      MISSING_PACKAGE="$1-$RELEASE"
      require_test_kernel_host() { return 0; }
      apt-cache() { [ "$2" != "$MISSING_PACKAGE" ]; }
      apt-get() { echo 'unexpected installation'; return 1; }

      When call install_kernel_packages "$PACKAGE_DIR"
      The status should be failure
      The error should include "Matching $MISSING_PACKAGE is unavailable"
      The output should not include 'unexpected installation'
    End
  End

  It 'uses supplied headers and tools even when their packages are absent from apt'
    create_package "linux-azure-headers-${RELEASE%-azure}" "$VERSION" all common-headers.deb
    create_package "linux-headers-$RELEASE" "$VERSION" amd64 headers.deb
    create_package "linux-tools-$RELEASE" "$VERSION" amd64 tools.deb
    create_package "linux-cloud-tools-$RELEASE" "$VERSION" amd64 cloud-tools.deb
    create_package "linux-azure-tools-${RELEASE%-azure}" "$VERSION" amd64 abi-tools.deb
    create_package "linux-azure-cloud-tools-${RELEASE%-azure}" "$VERSION" amd64 abi-cloud-tools.deb
    write_manifest
    require_test_kernel_host() { return 0; }
    apt-cache() { return 1; }
    apt-get() { echo "apt-get $*"; return 1; }

    When call install_kernel_packages "$PACKAGE_DIR"
    The status should be failure
    The error should equal ''
    The output should include "$PACKAGE_DIR/abi-cloud-tools.deb"
    The output should include "linux-tools-$RELEASE"
    The output should include "linux-cloud-tools-$RELEASE"
  End

  It 'rejects a boot into the stock kernel'
    uname() { echo 7.0.0-1017-azure; }

    When call verify_kernel_boot "$PACKAGE_DIR"
    The status should be failure
    The error should include "Expected running kernel $RELEASE, got 7.0.0-1017-azure"
  End

  It 'records the running kernel and package checksums after reboot'
    uname() { echo "$RELEASE"; }
    systemctl() { return 0; }
    tee() { cat; }

    When call verify_kernel_boot "$PACKAGE_DIR"
    The status should be success
    The output should include "CUSTOM_KERNEL_VERIFIED=$RELEASE"
    The output should include "$(cat "$PACKAGE_DIR/SHA256SUMS")"
  End

  It 'does not record a verified kernel when the KVP daemon fails after reboot'
    uname() { echo "$RELEASE"; }
    systemctl() { return 1; }
    tee() { cat; }

    When call verify_kernel_boot "$PACKAGE_DIR"
    The status should be failure
    The error should include "hv-kvp-daemon.service is not active for kernel $RELEASE"
    The output should not include 'CUSTOM_KERNEL_VERIFIED'
  End
End
