#!/bin/bash

# Tests for compute_msi_resource_strings function from produce-packer-settings-functions.sh

Describe 'compute_msi_resource_strings function'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  setup_environment() {
    unset AZURE_MSI_RESOURCE_STRING
    unset PRIVATE_PACKAGES_URL
    unset WINDOWS_PRIVATE_PACKAGES_URL
    unset WINDOWS_BASE_IMAGE_URL
    unset WINDOWS_CONTAINERIMAGE_JSON_URL
    unset COMPONENTS_JSON
    unset msi_resource_strings
  }

  BeforeEach 'setup_environment'

  AfterAll 'rm -rf ./.shellspec-scratch'

  write_components_json() {
    # $1: "true" to write a package with windowsDownloadRequiresAzCopy=true, anything else for none
    # Written under the repo tree (not /tmp) so this works even with a snap-confined jq install
    # that can't read outside the home/repo tree.
    mkdir -p "./.shellspec-scratch"
    COMPONENTS_JSON="./.shellspec-scratch/components-$$-${RANDOM}.json"
    if [ "$1" = "true" ]; then
      cat > "$COMPONENTS_JSON" <<'EOF'
{
  "Packages": [
    {
      "windowsDownloadLocation": "c:\\akse-cache\\private\\",
      "downloadURIs": {
        "windows": {
          "default": {
            "versionsV2": [{"latestVersion": "1.0.0"}],
            "downloadURL": "https://privatestorageaccount.blob.core.windows.net/c/f-v${version}.zip",
            "windowsDownloadRequiresAzCopy": true
          }
        }
      }
    }
  ]
}
EOF
    else
      cat > "$COMPONENTS_JSON" <<'EOF'
{
  "Packages": [
    {
      "windowsDownloadLocation": "c:\\akse-cache\\public\\",
      "downloadURIs": {
        "windows": {
          "default": {
            "versionsV2": [{"latestVersion": "1.0.0"}],
            "downloadURL": "https://acs-mirror.azureedge.net/f-v${version}.zip"
          }
        }
      }
    }
  ]
}
EOF
    fi
  }

  Describe 'no components.json signal, no legacy env vars'
    It 'leaves msi_resource_strings empty even when AZURE_MSI_RESOURCE_STRING is set'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      write_components_json "false"
      When call compute_msi_resource_strings
      The status should be success
      The value "${#msi_resource_strings[@]}" should eq "0"
      The output should include "Skipping UAMI assignment"
    End

    It 'leaves msi_resource_strings empty when AZURE_MSI_RESOURCE_STRING is unset, regardless of the flag'
      unset AZURE_MSI_RESOURCE_STRING
      write_components_json "true"
      When call compute_msi_resource_strings
      The status should be success
      The value "${#msi_resource_strings[@]}" should eq "0"
      The output should include "Skipping UAMI assignment"
    End
  End

  Describe 'windowsDownloadRequiresAzCopy=true in components.json'
    It 'populates msi_resource_strings when AZURE_MSI_RESOURCE_STRING is set'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      write_components_json "true"
      When call compute_msi_resource_strings
      The status should be success
      The variable msi_resource_strings[0] should eq "$AZURE_MSI_RESOURCE_STRING"
      The output should be present
      The output should include "Assigning UAMI to Packer VM"
    End
  End

  Describe 'legacy env vars still work on their own (regression check)'
    It 'populates msi_resource_strings for PRIVATE_PACKAGES_URL alone'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      PRIVATE_PACKAGES_URL="https://example.com/private.tar"
      write_components_json "false"
      When call compute_msi_resource_strings
      The status should be success
      The variable msi_resource_strings[0] should eq "$AZURE_MSI_RESOURCE_STRING"
      The output should be present
    End

    It 'populates msi_resource_strings for WINDOWS_PRIVATE_PACKAGES_URL alone'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      WINDOWS_PRIVATE_PACKAGES_URL="https://example.com/win-private.zip"
      write_components_json "false"
      When call compute_msi_resource_strings
      The status should be success
      The variable msi_resource_strings[0] should eq "$AZURE_MSI_RESOURCE_STRING"
      The output should be present
    End

    It 'populates msi_resource_strings for WINDOWS_BASE_IMAGE_URL alone'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      WINDOWS_BASE_IMAGE_URL="https://example.com/base.vhd"
      write_components_json "false"
      When call compute_msi_resource_strings
      The status should be success
      The variable msi_resource_strings[0] should eq "$AZURE_MSI_RESOURCE_STRING"
      The output should be present
    End

    It 'populates msi_resource_strings for WINDOWS_CONTAINERIMAGE_JSON_URL alone'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      WINDOWS_CONTAINERIMAGE_JSON_URL="https://example.com/images.json"
      write_components_json "false"
      When call compute_msi_resource_strings
      The status should be success
      The variable msi_resource_strings[0] should eq "$AZURE_MSI_RESOURCE_STRING"
      The output should be present
    End
  End

  Describe 'missing components.json file'
    It 'does not error and treats it as no AzCopy signal present'
      AZURE_MSI_RESOURCE_STRING="/subscriptions/x/resourceGroups/y/providers/.../uami"
      COMPONENTS_JSON="./.shellspec-scratch/does-not-exist-$$-${RANDOM}.json"
      When call compute_msi_resource_strings
      The status should be success
      The value "${#msi_resource_strings[@]}" should eq "0"
      The output should include "Skipping UAMI assignment"
    End
  End
End
