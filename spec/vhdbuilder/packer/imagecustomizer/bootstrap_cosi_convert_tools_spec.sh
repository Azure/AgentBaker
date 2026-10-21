#!/bin/bash
# shellcheck disable=SC2034,SC2329

# Tests for vhdbuilder/packer/imagecustomizer/scripts/bootstrap-cosi-convert-tools.sh.
# Only the functions are sourced (guarded by ${__SOURCED__:+return}); every
# system command is stubbed, so nothing is installed.

Describe 'bootstrap-cosi-convert-tools.sh'
  setup() {
    # shellcheck disable=SC1090
    __SOURCED__=1 . "./vhdbuilder/packer/imagecustomizer/scripts/bootstrap-cosi-convert-tools.sh"
    stub_dir="$(mktemp -d)"
  }
  cleanup() { rm -rf "$stub_dir"; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # Creates executable stubs named $@ in stub_dir.
  make_stubs() {
    local name
    for name in "$@"; do
      printf '#!/bin/sh\nexit 0\n' > "$stub_dir/$name"
      chmod +x "$stub_dir/$name"
    done
  }

  # Links the real tools that install_azcopy needs into stub_dir.
  link_tools() {
    local name
    for name in "$@"; do
      ln -s "$(command -v "$name")" "$stub_dir/$name"
    done
  }

  Describe 'detect_package_manager'
    detect_with_path() {
      PATH="$stub_dir" detect_package_manager
      local rc=$?
      echo "PKG=$PKG"
      return $rc
    }

    It 'prefers apt when both apt-get and tdnf exist'
      make_stubs apt-get tdnf
      When call detect_with_path
      The status should be success
      The line 1 of output should equal 'Package manager: apt'
      The line 2 of output should equal 'PKG=apt'
    End

    It 'uses tdnf on Azure Linux'
      make_stubs tdnf
      When call detect_with_path
      The status should be success
      The line 2 of output should equal 'PKG=tdnf'
    End

    It 'falls back to dnf'
      make_stubs dnf
      When call detect_with_path
      The status should be success
      The line 2 of output should equal 'PKG=dnf'
    End

    It 'fails without a supported package manager'
      When call detect_with_path
      The status should be failure
      The line 1 of output should include 'No supported package manager'
    End
  End

  Describe 'require_passwordless_sudo'
    It 'fails when sudo prompts for a password'
      sudo() { return 1; }
      When call require_passwordless_sudo
      The status should be failure
      The output should include 'does not have passwordless sudo'
    End

    It 'passes when sudo -n succeeds'
      sudo() { return 0; }
      When call require_passwordless_sudo
      The status should be success
    End
  End

  Describe 'pkg_install'
    sudo() { echo "sudo $*"; }

    It 'uses tdnf on Azure Linux'
      PKG=tdnf
      When call pkg_install make jq
      The output should equal 'sudo tdnf install -y make jq'
    End

    It 'uses noninteractive apt-get without recommends'
      PKG=apt
      When call pkg_install make
      The output should equal 'sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends make'
    End

    It 'fails before a package manager is detected'
      PKG=""
      When call pkg_install make
      The status should be failure
      The output should include 'pkg_install called before detect_package_manager'
    End
  End

  Describe 'azcopy_release_for_machine'
    Parameters
      aarch64 arm64 4db9a4b48abc7775f1a5d6d928afc42361dcc57bbfcde23ac82e4c419a0dc8fc
      arm64   arm64 4db9a4b48abc7775f1a5d6d928afc42361dcc57bbfcde23ac82e4c419a0dc8fc
      x86_64  amd64 7549424d56ab2d8b4033c84c2a9bb167dc2dcbb23998acd7fffb37bc1a71a267
    End

    It "pins the $2 release for $1"
      When call azcopy_release_for_machine "$1"
      The status should be success
      The output should equal "https://github.com/Azure/azure-storage-azcopy/releases/download/v${AZCOPY_VERSION}/azcopy_linux_$2_${AZCOPY_VERSION}.tar.gz $3"
    End
  End

  It 'rejects an unsupported machine architecture'
    When call azcopy_release_for_machine riscv64
    The status should be failure
    The output should include 'Unsupported machine architecture for azcopy: riscv64'
  End

  Describe 'install_azcopy'
    It 'refuses an archive whose checksum does not match the pin'
      link_tools mktemp sha256sum rm
      uname() { echo x86_64; }
      curl() { printf 'tampered' > "${@: -1}"; }
      install_with_path() { PATH="$stub_dir" install_azcopy; }
      When call install_with_path
      The status should be failure
      The output should include 'azcopy archive checksum mismatch'
    End
  End

  Describe 'install_go'
    go() { echo 'go version go1.27.1 linux/amd64'; }

    It 'always installs the pinned Microsoft Go on Ubuntu, even if Go exists'
      PKG=apt
      mkdir -p "$stub_dir/hack"
      printf '#!/bin/sh\necho setup_golang ran\n' > "$stub_dir/hack/setup_golang.sh"
      chmod +x "$stub_dir/hack/setup_golang.sh"
      install_in_dir() { cd "$stub_dir" && install_go; }
      When call install_in_dir
      The line 1 of output should equal 'setup_golang ran'
      The line 2 of output should equal 'go version go1.27.1 linux/amd64'
    End

    It 'keeps an existing Go on Azure Linux'
      PKG=tdnf
      sudo() { echo "sudo $*"; }
      When call install_go
      The output should equal 'go version go1.27.1 linux/amd64'
    End

    It 'installs golang on Azure Linux when Go is missing'
      PKG=tdnf
      unset -f go
      sudo() { echo "sudo $*"; go() { echo 'go version go1.27.1 linux/arm64'; }; }
      install_without_go() { PATH="$stub_dir" install_go; }
      When call install_without_go
      The line 1 of output should equal 'sudo tdnf install -y golang'
      The line 2 of output should equal 'go version go1.27.1 linux/arm64'
    End
  End
End
