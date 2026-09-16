#!/bin/bash

# Tests for functions in vhdbuilder/packer/azcopy-private-package-login.sh, sourced by
# install-dependencies.sh to select the correct managed identity for AzCopy when downloading
# private Kubernetes packages during the Linux VHD build.

Describe 'azcopy-private-package-login.sh'
  Include './vhdbuilder/packer/azcopy-private-package-login.sh'

  AfterAll 'rm -rf ./.shellspec-scratch-home-*'

  setup_environment() {
    # OS family constants normally defined by install-dependencies.sh before this file is sourced.
    UBUNTU_OS_NAME="UBUNTU"
    MARINER_OS_NAME="MARINER"
    MARINER_KATA_OS_NAME="MARINERKATA"
    AZURELINUX_OS_NAME="AZURELINUX"
    AZURELINUX_KATA_OS_NAME="AZURELINUXKATA"
    ACL_OS_NAME="AZURECONTAINERLINUX"
    ACL_OS_VARIANT="AZURECONTAINERLINUX"
    OS=""
    OS_VARIANT=""

    unset AZURE_MSI_RESOURCE_STRING

    MOCK_APT_GET_INSTALL_RESULT="0"
    MOCK_DNF_INSTALL_RESULT="0"
    MOCK_PYTHON3_RESULT="0"
    OS_VERSION=""
    # Tracks whether a mocked package-manager call has "installed" azure-cli, so
    # azure_cli_is_present reflects install_azure_cli_for_private_packages's actual effect instead
    # of the real state of whatever machine happens to run these tests.
    AZURE_CLI_INSTALLED="false"

    # isACL mirrors the real implementation in parts/linux/cloud-init/artifacts/cse_helpers.sh -
    # redefined here (rather than sourcing that file) to keep this spec focused on
    # azcopy-private-package-login.sh's own logic.
    # shellcheck disable=SC2329
    isACL() {
      local os=${1-$OS}
      local os_variant=${2-$OS_VARIANT}
      [ "$os" = "$ACL_OS_NAME" ] || { [ "$os" = "$AZURELINUX_OS_NAME" ] && [ "$os_variant" = "$ACL_OS_VARIANT" ]; }
    }

    # shellcheck disable=SC2329
    apt_get_install() {
      [ "$MOCK_APT_GET_INSTALL_RESULT" = "0" ] || return "$MOCK_APT_GET_INSTALL_RESULT"
      case " $* " in *" azure-cli "*) AZURE_CLI_INSTALLED="true" ;; esac
      return 0
    }
    # shellcheck disable=SC2329
    apt_get_update() { return 0; }
    # shellcheck disable=SC2329
    dnf_install() {
      [ "$MOCK_DNF_INSTALL_RESULT" = "0" ] || return "$MOCK_DNF_INSTALL_RESULT"
      case " $* " in *" azure-cli "*) AZURE_CLI_INSTALLED="true" ;; esac
      return 0
    }
    # shellcheck disable=SC2329
    rpm() { return 0; }
    # shellcheck disable=SC2329
    python3() {
      echo "python3 $*"
      [ "$MOCK_PYTHON3_RESULT" = "0" ] || return "$MOCK_PYTHON3_RESULT"
      case " $* " in *" azure-cli "*) AZURE_CLI_INSTALLED="true" ;; esac
      return 0
    }
    # shellcheck disable=SC2329
    write_apt_azure_cli_repo() { echo "write_apt_azure_cli_repo called"; return 0; }
    # shellcheck disable=SC2329
    write_yum_azure_cli_repo() { return 0; }
    # shellcheck disable=SC2329
    azure_cli_is_present() { [ "$AZURE_CLI_INSTALLED" = "true" ]; }
    # shellcheck disable=SC2329
    az() { echo "az $*"; return 0; }
  }

  BeforeEach 'setup_environment'

  Describe 'ensure_azure_login_for_private_packages'
    It 'does nothing when AZURE_MSI_RESOURCE_STRING is not set'
      unset AZURE_MSI_RESOURCE_STRING
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "AZURE_MSI_RESOURCE_STRING is not set"
      The output should not include "az login"
    End

    It 'installs azure-cli and logs in with the resource ID when set (Ubuntu)'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$UBUNTU_OS_NAME"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "az login --identity --resource-id /subscriptions/x/resourceGroups/y/providers/.../uami"
    End

    It 'installs azure-cli and logs in with the resource ID when set (Mariner/AzureLinux)'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$AZURELINUX_OS_NAME"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "az login --identity --resource-id /subscriptions/x/resourceGroups/y/providers/.../uami"
    End

    It 'installs azure-cli and logs in with the resource ID when set (Ubuntu 26.04, pip fallback)'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$UBUNTU_OS_NAME"
      OS_VERSION="26.04"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "python3 -m pip install azure-cli --break-system-packages"
      The output should not include "write_apt_azure_cli_repo called"
      The output should include "az login --identity --resource-id /subscriptions/x/resourceGroups/y/providers/.../uami"
    End

    It 'skips login gracefully when azure-cli cannot be installed for this OS'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="SOME_UNSUPPORTED_OS"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "Could not install azure-cli"
      The output should not include "az login"
    End

    It 'skips login gracefully for Azure Container Linux (ACL), old-style OS name'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$ACL_OS_NAME"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "Could not install azure-cli"
      The output should not include "az login"
    End

    It 'skips login gracefully for Azure Container Linux (ACL), new-style OS=AZURELINUX + VARIANT'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$AZURELINUX_OS_NAME"
      OS_VARIANT="$ACL_OS_VARIANT"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "no azure-cli install recipe for Azure Container Linux (ACL)"
      The output should not include "dnf_install"
      The output should not include "az login"
    End

    It 'skips login gracefully when the package manager install fails'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$UBUNTU_OS_NAME"
      MOCK_APT_GET_INSTALL_RESULT="1"
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "Could not install azure-cli"
      The output should not include "az login"
    End
  End

  Describe 'install_azure_cli_for_private_packages'
    It 'skips installation when az is already available'
      OS="$UBUNTU_OS_NAME"
      AZURE_CLI_INSTALLED="true"
      When call install_azure_cli_for_private_packages
      The status should be success
      The output should include "already installed"
    End
  End

  Describe 'clear_azure_cli_login_state'
    It 'removes the token cache directory (az account clear output is intentionally suppressed)'
      # shellcheck disable=SC2329
      az() { return 0; }
      HOME="./.shellspec-scratch-home-$$-${RANDOM}"
      mkdir -p "$HOME/.azure"
      When call clear_azure_cli_login_state
      The status should be success
      The path "$HOME/.azure" should not be exist
    End

    It 'does not fail when there is nothing to clean up'
      # shellcheck disable=SC2329
      az() { return 1; }
      HOME="./.shellspec-scratch-home-$$-${RANDOM}"
      When call clear_azure_cli_login_state
      The status should be success
      The path "$HOME/.azure" should not be exist
    End
  End
End

# Regression coverage for the OSGuard ImageCustomizer staging gap this file's addition originally
# broke: install-dependencies.sh unconditionally sources azcopy-private-package-login.sh, but the
# OSGuard build stages install-dependencies.sh (and siblings) into /opt/azure/containers via a
# separate `additionalFiles` mechanism from the Packer JSON templates, not this Include-based test.
Describe 'OSGuard ImageCustomizer staging'
  osguard_yml="./vhdbuilder/packer/imagecustomizer/azlosguard/azlosguard.yml"
  osguard_postinstall="./vhdbuilder/packer/imagecustomizer/azlosguard/scripts/azlosguard-postinstall.sh"

  It 'stages azcopy-private-package-login.sh alongside install-dependencies.sh'
    When run grep -A2 "azcopy-private-package-login.sh$" "$osguard_yml"
    The status should be success
    The output should include "destination: /opt/azure/containers/azcopy-private-package-login.sh"
  End

  It 'cleans up the staged azcopy-private-package-login.sh after the build'
    When run grep -F "rm /home/packer/azcopy-private-package-login.sh" "$osguard_postinstall"
    The status should be success
    The output should include "rm /home/packer/azcopy-private-package-login.sh"
  End
End
