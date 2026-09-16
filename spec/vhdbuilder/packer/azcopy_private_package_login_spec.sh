#!/bin/bash

# Tests for functions in vhdbuilder/packer/azcopy-private-package-login.sh, sourced by
# install-dependencies.sh to select the correct managed identity for AzCopy when downloading
# private Kubernetes packages during the Linux VHD build.

Describe 'azcopy-private-package-login.sh'
  Include './vhdbuilder/packer/azcopy-private-package-login.sh'

  setup_environment() {
    # OS family constants normally defined by install-dependencies.sh before this file is sourced.
    UBUNTU_OS_NAME="UBUNTU"
    MARINER_OS_NAME="MARINER"
    MARINER_KATA_OS_NAME="MARINERKATA"
    AZURELINUX_OS_NAME="AZURELINUX"
    AZURELINUX_KATA_OS_NAME="AZURELINUXKATA"
    OS=""

    unset AZURE_MSI_RESOURCE_STRING

    MOCK_APT_GET_INSTALL_RESULT="0"
    MOCK_DNF_INSTALL_RESULT="0"

    # shellcheck disable=SC2329
    apt_get_install() { return "$MOCK_APT_GET_INSTALL_RESULT"; }
    # shellcheck disable=SC2329
    apt_get_update() { return 0; }
    # shellcheck disable=SC2329
    dnf_install() { return "$MOCK_DNF_INSTALL_RESULT"; }
    # shellcheck disable=SC2329
    rpm() { return 0; }
    # shellcheck disable=SC2329
    write_apt_azure_cli_repo() { return 0; }
    # shellcheck disable=SC2329
    write_yum_azure_cli_repo() { return 0; }
    # azure_cli_is_present is mocked explicitly per test below, so behavior here is deterministic
    # regardless of whether the machine actually running these tests happens to have az on PATH.
    # shellcheck disable=SC2329
    azure_cli_is_present() { return 1; }
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
      # shellcheck disable=SC2329
      az() { echo "az $*"; return 0; }
      When call ensure_azure_login_for_private_packages
      The status should be success
      The output should include "az login --identity --resource-id /subscriptions/x/resourceGroups/y/providers/.../uami"
    End

    It 'installs azure-cli and logs in with the resource ID when set (Mariner/AzureLinux)'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      OS="$AZURELINUX_OS_NAME"
      # shellcheck disable=SC2329
      az() { echo "az $*"; return 0; }
      When call ensure_azure_login_for_private_packages
      The status should be success
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
      # shellcheck disable=SC2329
      azure_cli_is_present() { return 0; }
      When call install_azure_cli_for_private_packages
      The status should be success
      The output should include "already installed"
    End
  End
End
