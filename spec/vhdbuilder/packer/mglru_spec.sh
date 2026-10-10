#!/bin/bash
# shellcheck disable=SC2329

Describe 'MGLRU image default'
  load_helpers() {
    . ./parts/linux/cloud-init/artifacts/cse_helpers.sh >/dev/null
    . ./vhdbuilder/packer/packer_source.sh
  }
  BeforeAll 'load_helpers'

  Describe 'OS scope'
    Parameters
      UBUNTU     20.04 ''                  0
      UBUNTU     22.04 ''                  0
      UBUNTU     23.10 ''                  0
      UBUNTU     24.03 ''                  0
      UBUNTU     24.04 ''                  0
      UBUNTU     26.04 ''                  0
      UBUNTU     28.04 ''                  0
      MARINER    2.0   ''                  0
      MARINERKATA 2.0  ''                  0
      AZURELINUX 2.0   ''                  0
      AZURELINUX 3.0   ''                  0
      AZURELINUX 3.0   OSGUARD             0
      AZURELINUX 4.0   ''                  0
      AZURELINUXKATA 3.0 ''                0
      FLATCAR    4230.2.2 ''               1
      AZURECONTAINERLINUX 3.0 ''           1
      AZURELINUX 3.0   AZURECONTAINERLINUX 1
    End

    It "returns $4 for $1 $2 variant=$3"
      OS_VERSION="$2"
      When call isMGLRUDefaultDisabled "$1" "$3"
      The status should equal "$4"
    End
  End

  Describe 'Packer installation'
    cpAndMode() {
      printf '%s\n' "$*"
    }

    Describe 'included versions'
      Parameters
        UBUNTU 20.04
        UBUNTU 22.04
        UBUNTU 24.04
        UBUNTU 26.04
        MARINER 2.0
        MARINERKATA 2.0
        AZURELINUX 2.0
        AZURELINUX 3.0
        AZURELINUX 4.0
      End

      It "installs the rule for $1 $2 independently of the build kernel"
        OS="$1" OS_VERSION="$2" OS_VARIANT=''
        When call copyMGLRUConfig
        The status should be success
        The output should equal '/home/packer/aks-mglru.conf /etc/tmpfiles.d/aks-mglru.conf 644'
      End
    End

    Describe 'excluded OS families'
      Parameters
        FLATCAR 4230.2.2 ''
        AZURECONTAINERLINUX 3.0 ''
        AZURELINUX 3.0 AZURECONTAINERLINUX
      End

      It "leaves $1 $2 variant=$3 unchanged"
        OS="$1" OS_VERSION="$2" OS_VARIANT="$3"
        When call copyMGLRUConfig
        The status should be success
        The output should be blank
      End
    End

    It 'calls the installer when copying Packer files'
      When call sed -n '/^copyPackerFiles()/,/^}/p' vhdbuilder/packer/packer_source.sh
      The output should include '  copyMGLRUConfig'
    End

    Describe 'upload mappings'
      Parameters
        base
        cvm
        arm64-gen2
        arm64-gb
        mariner
        mariner-arm64
        mariner-cvm
      End

      It "uploads the rule in the $1 image template"
        filter='[.provisioners[] | select(.type == "file" and .source == "parts/linux/cloud-init/artifacts/aks-mglru.conf" and .destination == "/home/packer/aks-mglru.conf")] | length == 1'
        When call jq -e "$filter" "vhdbuilder/packer/vhd-image-builder-$1.json"
        The status should be success
        The output should equal true
      End
    End

    It 'installs the same rule in the OSGuard image'
      When call grep -A2 -F 'source: /AgentBaker/parts/linux/cloud-init/artifacts/aks-mglru.conf' \
        vhdbuilder/packer/imagecustomizer/azlosguard/azlosguard.yml
      The output should include 'destination: /etc/tmpfiles.d/aks-mglru.conf'
      The output should include 'permissions: 644'
    End
  End

  Describe 'post-boot VHD validation'
    setup_validation() {
      TEST_ROOT="${SHELLSPEC_TMPBASE}/mglru-validation"
      mkdir -p "$TEST_ROOT/etc/tmpfiles.d" "$TEST_ROOT/sys/kernel/mm/lru_gen"
      cp parts/linux/cloud-init/artifacts/aks-mglru.conf "$TEST_ROOT/etc/tmpfiles.d/aks-mglru.conf"
      printf '0x0000\n' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      unset OS OS_VARIANT
      OS_SKU=Ubuntu OS_VERSION=24.04 FEATURE_FLAGS=''
      eval "$(sed -n '/^err()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
      eval "$(sed -n '/^getCurrentPackageTestOS()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
      eval "$(sed -n '/^testMGLRUDisabled()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh |
        sed -e "s|\"/etc/tmpfiles.d/aks-mglru.conf\"|\"$TEST_ROOT/etc/tmpfiles.d/aks-mglru.conf\"|" \
            -e "s|\"/sys/kernel/mm/lru_gen/enabled\"|\"$TEST_ROOT/sys/kernel/mm/lru_gen/enabled\"|")"
    }
    cleanup_validation() {
      rm -rf "$TEST_ROOT"
    }
    BeforeEach 'setup_validation'
    AfterEach 'cleanup_validation'

    Describe 'independent execution on included images'
      Parameters
        Ubuntu 20.04 ''
        Ubuntu 22.04 ''
        Ubuntu 24.04 ''
        Ubuntu 26.04 ''
        CBLMariner 2.0 ''
        CBLMariner 2.0 kata
        AzureLinux 2.0 ''
        AzureLinux 3.0 ''
        AzureLinux 4.0 ''
        AzureLinux 3.0 kata
        AzureLinuxOSGuard 3.0 ''
      End

      It "validates $1 $2 features=$3 without OS globals"
        OS_SKU="$1" OS_VERSION="$2" FEATURE_FLAGS="$3"
        When call testMGLRUDisabled
        The status should be success
        The output should include 'MGLRU is disabled after boot'
        The variable OS should be undefined
        The variable OS_VARIANT should be undefined
      End
    End

    It 'ignores stale OS globals without changing them'
      OS_SKU=AzureLinux OS_VERSION=3.0
      OS=FLATCAR OS_VARIANT=AZURECONTAINERLINUX
      When call testMGLRUDisabled
      The status should be success
      The output should include 'MGLRU is disabled after boot'
      The variable OS should equal FLATCAR
      The variable OS_VARIANT should equal AZURECONTAINERLINUX
    End

    Describe 'kernels without MGLRU'
      Parameters
        Ubuntu 20.04
        Ubuntu 22.04
        Ubuntu 24.04
        CBLMariner 2.0
        AzureLinux 2.0
        AzureLinux 3.0
      End

      It "accepts an absent interface on $1 $2 while retaining the rule"
        OS_SKU="$1" OS_VERSION="$2"
        rm "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
        When call testMGLRUDisabled
        The status should be success
        The output should include 'Kernel does not expose MGLRU; boot-time override is installed'
      End
    End

    It 'rejects an enabled kernel without masking failure by applying the rule'
      printf '0x0007\n' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      When call testMGLRUDisabled
      The status should be failure
      The error should include 'MGLRU is not disabled after boot'
      The contents of file "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should equal '0x0007'
    End

    It 'rejects a missing boot-time rule'
      rm "$TEST_ROOT/etc/tmpfiles.d/aks-mglru.conf"
      When call testMGLRUDisabled
      The status should be failure
      The error should include 'Missing or incorrect boot-time MGLRU override'
    End

    It 'rejects an override on an excluded OS family'
      OS_SKU=Flatcar OS_VERSION=4230.2.2
      When call testMGLRUDisabled
      The status should be failure
      The error should include 'MGLRU override must not be installed'
    End

    Describe 'independent execution on excluded images'
      Parameters
        Flatcar 4230.2.2
        AzureContainerLinux 3.0
      End

      It "does not require MGLRU to be disabled on $1 $2"
        OS_SKU="$1" OS_VERSION="$2"
        rm "$TEST_ROOT/etc/tmpfiles.d/aks-mglru.conf"
        printf '0x0007\n' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
        When call testMGLRUDisabled
        The status should be success
        The output should include 'Skipping'
      End
    End

    It 'resolves the excluded ACL variant despite stale included-image globals'
      OS_SKU=AzureContainerLinux OS_VERSION=3.0
      OS=AZURELINUX OS_VARIANT=''
      When call testMGLRUDisabled
      The status should be failure
      The error should include 'MGLRU override must not be installed on AZURELINUX 3.0 (AZURECONTAINERLINUX)'
    End

    It 'is included in the VHD content-test run'
      When call grep -x testMGLRUDisabled vhdbuilder/packer/test/linux-vhd-content-test.sh
      The output should equal testMGLRUDisabled
    End
  End

  Describe 'systemd-tmpfiles boot behavior'
    tmpfiles_unavailable() {
      ! command -v systemd-tmpfiles >/dev/null 2>&1
    }
    Skip if 'systemd-tmpfiles is unavailable on this host' tmpfiles_unavailable

    setup_tmpfiles() {
      TEST_ROOT="${SHELLSPEC_TMPBASE}/mglru-tmpfiles"
      mkdir -p "$TEST_ROOT/sys/kernel/mm/lru_gen"
      printf '7' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
    }
    cleanup_tmpfiles() {
      rm -rf "$TEST_ROOT"
    }
    apply_tmpfiles() {
      systemd-tmpfiles --create --root="$TEST_ROOT" "$@" - \
        < parts/linux/cloud-init/artifacts/aks-mglru.conf
    }
    BeforeEach 'setup_tmpfiles'
    AfterEach 'cleanup_tmpfiles'

    It 'writes zero at boot'
      When call apply_tmpfiles --boot
      The status should be success
      The contents of file "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should equal 0
    End

    It 'reapplies the rule on subsequent boots'
      apply_tmpfiles --boot
      printf '7' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      When call apply_tmpfiles --boot
      The status should be success
      The contents of file "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should equal 0
    End

    It 'skips kernels without the interface instead of creating it'
      rm "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      When call apply_tmpfiles --boot
      The status should be success
      The path "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should not be exist
    End

    It 'disables MGLRU if a subsequent kernel introduces the interface'
      rm "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      apply_tmpfiles --boot
      printf '7' > "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      When call apply_tmpfiles --boot
      The status should be success
      The contents of file "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should equal 0
    End

    It 'does not reset settings during non-boot tmpfiles runs'
      When call apply_tmpfiles
      The status should be success
      The contents of file "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled" should equal 7
    End

    It 'reports write failures rather than ignoring them'
      rm "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      mkdir "$TEST_ROOT/sys/kernel/mm/lru_gen/enabled"
      When call apply_tmpfiles --boot
      The status should be failure
      The error should include 'enabled'
    End
  End
End
