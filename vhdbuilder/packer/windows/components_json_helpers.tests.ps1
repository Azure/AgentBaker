BeforeAll {
    . $PSScriptRoot\components_json_helpers.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')
}

Describe 'SafeReplaceString' {
    It 'given no vars are present, it returns the string' {
        SafeReplaceString "this is a string" | Should -Be "this is a string"
    }

    It 'given version var is present, it replaces version' {
        # set the str before the $version env var so that we know it's being replaced in the function.
        $str = "this is a `${version}` string"
        $version = "versioned"
        SafeReplaceString $str | Should -Be "this is a versioned string"
    }

    It 'given CPU_ARCH var is present, it replaces version' {
        # set the str before the $version env var so that we know it's being replaced in the function.
        $str = "this is an `${CPU_ARCH}` string"
        $CPU_ARCH = "architecture"
        SafeReplaceString $str | Should -Be "this is an architecture string"
    }

    It 'given BOBBY var is present, it replaces with empty string' {
        # set the str before the $version env var so that we know it's being replaced in the function.
        $str = "this is an `${BOBBY}` string"
        $BOBBY = "architecture"
        SafeReplaceString $str | Should -Be "this is an  string"
    }

}



Describe 'Tests of GetAllCachedThings ' {
    BeforeEach {
        $windowsSettingsTestString = '{
"WindowsBaseVersions": {
"2019": {
  "base_image_sku": "2019-Datacenter-Core-smalldisk",
  "windows_image_name": "windows-2019",
  "base_image_version": "17763.6893.250210",
  "patches_to_apply": [{"id": "patchid", "url": "patch_url"}]
},
 "23H2-gen2": {
  "base_image_sku": "2019-Datacenter-Core-smalldisk",
  "windows_image_name": "windows-2019",
  "base_image_version": "17763.6893.250210",
  "patches_to_apply": [{"id": "patchid", "url": "patch_url"}]
}
},
  "WindowsRegistryKeys": [
    {
      "Comment": "Enables DNS resolution of SMB shares for containerD:  # https://github.com/kubernetes-sigs/windows-gmsa/issues/30#issuecomment-802240945",
      "WindowsSkuMatch": "*",
      "Path": "HKLM:\\SYSTEM\\CurrentControlSet\\Services\\hns\\State",
      "Name": "EnableCompartmentNamespace",
      "Value": "1",
      "Type": "DWORD"
    }
  ]
}'
        $windowsSettings = echo $windowsSettingsTestString | ConvertFrom-Json

        $componentsJsonTestString = '{
        "ContainerImages": [
{
  "downloadURL": "mcr.microsoft.com/container/with/seperate/win/and/linux/versions:*",
  "amd64OnlyVersions": [],
  "multiArchVersionsV2": [],
  "windowsVersions": [
    {
      "renovateTag": "registry=https://mcr.microsoft.com, name=oss/kubernetes/pause",
      "latestVersion": "win-version"
    },{
      "renovateTag": "registry=https://mcr.microsoft.com, name=oss/kubernetes/pause",
      "latestVersion": "other-version"
    }
  ]
}],
"Packages": [
{
  "windowsDownloadLocation": "c:\\akse-cache\\",
  "downloadLocation": null,
  "downloadUris": {
    "windows": {
      "default": {
        "versionsV2": [
          {
            "renovateTag": "<DO_NOT_UPDATE>",
            "latestVersion": "0.0.50",
            "previousLatestVersion": "0.0.51"
          }
        ],
        "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
      }
    }
  }
}
],
"OCIArtifacts": [
{
  "registry": "mcr.microsoft.com/repository/path:*",
  "windowsDownloadLocation": "C:\\akse-cache\\",
  "windowsVersions": [
    {
      "latestVersion": "1.2.3-prerelease.4"
    }
  ]
}
]}'
        $componentsJson = echo $componentsJsonTestString | ConvertFrom-Json
    }

    it 'has a component in it' {
        $windowsSku = "2019-containerd"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Contain "mcr.microsoft.com/container/with/seperate/win/and/linux/versions:win-version"
    }

    it 'has a package in it' {
        $windowsSku = "2019-containerd"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Contain "c:\akse-cache\: https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.50.zip"
    }

    it 'has a reg key in it' {
        $windowsSku = "2019-containerd"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Contain "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State\EnableCompartmentNamespace=1"
    }

    it 'has an oci artifact in it' {
        $windowsSku = "2019-containerd"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Contain "c:\akse-cache\: mcr.microsoft.com/repository/path:1.2.3-prerelease.4"
    }

    it 'has the windows base version in it' {
        $windowsSku = "2019"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Contain "Windows 2019 base version: 17763.6893.250210"

    }

    it 'is sorted' {
        $windowsSku = "2019-containerd"

        $allpackages = GetAllCachedThings $componentsJson $windowsSettings

        $allpackages | Should -Be ( $allpackages | Sort-Object )
    }
}


Describe 'GetWindowsDefenderInfo' {
    BeforeEach {
        $testString = '{
   "WindowsDefenderInfo": {
    "DefenderUpdateUrl": "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64",
    "DefenderUpdateInfoUrl": "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64&action=info"
  },
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    it 'returns the right info for GetDefenderUpdateUrl' {
        GetDefenderUpdateUrl $windowsSettings | Should -Be "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64"
    }

    it 'returns the right info for GetDefenderUpdateInfoUrl' {
        GetDefenderUpdateInfoUrl $windowsSettings | Should -Be "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64&action=info"
    }

}

Describe 'GetWindowsBaseVersions' {
    BeforeEach {
        $testString = '{
  "WindowsBaseVersions": {
    "2019": {
      "base_image_sku": "2019-Datacenter-Core-smalldisk",
      "windows_image_name": "windows-2019",
      "base_image_version": "17763.6893.250210",
      "patches_to_apply": [{"id": "patchid", "url": "patch_url"}]
    },
     "23H2-gen2": {
      "base_image_sku": "2019-Datacenter-Core-smalldisk",
      "windows_image_name": "windows-2019",
      "base_image_version": "17763.6893.250210",
      "patches_to_apply": [{"id": "patchid", "url": "patch_url"}]
    }
  }
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    it "returns the bsae versions" {
        $baseVersions = GetWindowsBaseVersions $windowsSettings
        $baseVersions.Length | Should -Be 2
        $baseVersions | Should -Contain "2019"
        $baseVersions | Should -Contain "23H2-gen2"
    }
}

Describe 'WindowsBaseVersions' {
    BeforeEach {
        $testString = '{
  "WindowsBaseVersions": {
    "2019": {
      "base_image_sku": "2019-Datacenter-Core-smalldisk",
      "windows_image_name": "windows-2019",
      "base_image_version": "17763.6893.250210",
      "patches_to_apply": [{"id": "patchid", "url": "patch_url"}]
    }
  }
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    it "returns an empty array for an unknown windows sku" {
        $patchurls = GetPatchInfo "12345" $windowsSettings
        $patchurls.Length | Should -Be 0
    }

    it "can extract patch urls for windows 2019" {
        $patchurls = GetPatchInfo "2019" $windowsSettings
        $patchurls[0].url | Should -Be "patch_url"
        $patchurls[0].id | Should -Be "patchid"
        $patchurls.Length | Should -Be 1
    }

    it "can extract two patch urls for windows 2019" {
        $testString = '{
  "WindowsBaseVersions": {
    "2019": {
      "base_image_sku": "2019-Datacenter-Core-smalldisk",
      "windows_image_name": "windows-2019",
      "base_image_version": "17763.6893.250210",
      "patches_to_apply": [{"id": "patchid1", "url": "patch_url1"},{"id": "patchid2", "url": "patch_url2"}]
    }
  }
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
        $patchurls = GetPatchInfo "2019" $windowsSettings
        $patchurls[0].url | Should -Be "patch_url1"
        $patchurls[0].id | Should -Be "patchid1"
        $patchurls[1].url | Should -Be "patch_url2"
        $patchurls[1].id | Should -Be "patchid2"
        $patchurls.Length | Should -Be 2
    }
}

Describe 'LogReleaseNotesForWindowsRegistryKeys' {
    BeforeEach {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "this is a comment",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json

    }

    it "creates a line for the path" {
        Mock Get-ItemProperty -MockWith { return @{ EnableCertPaddingCheck = "1" } }

        $windowsSku = "2019-containerd-gen2"
        $lines = LogReleaseNotesForWindowsRegistryKeys $windowsSettings

        $lines | Should -Contain ("`t{0}" -f "pathpath")
    }

    it "creates a line for the name" {
        Mock Get-ItemProperty -MockWith { return @{ EnableCertPaddingCheck = "1" } }

        $windowsSku = "2019-containerd-gen2"
        $lines = LogReleaseNotesForWindowsRegistryKeys $windowsSettings

        $lines | Should -Contain ("`t`t{0} : {1}" -f "EnableCertPaddingCheck", "1")
    }

    it 'given two names in the different path, it logs each path' {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath1",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    },
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath2",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1"
    }
  ]
}'

        $windowsSettings = echo $testString | ConvertFrom-Json
        Mock Get-ItemProperty -MockWith {
            return @{
                EnableCertPaddingCheck = "1"
                EnableCertPaddingCheck2 = "2"
            }
        }

        $windowsSku = "2019-containerd-gen2"
        $lines = LogReleaseNotesForWindowsRegistryKeys $windowsSettings

        $lines.Length | Should -Be 4
        $lines | Should -Contain ("`t{0}" -f "pathpath1")
        $lines | Should -Contain ("`t`t{0} : {1}" -f "EnableCertPaddingCheck", "1")
        $lines | Should -Contain ("`t{0}" -f "pathpath2")
        $lines | Should -Contain ("`t`t{0} : {1}" -f "EnableCertPaddingCheck2", "2")
    }

    it 'given two names in the same path, it only logs the path once' {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    },
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1"
    }
  ]
}'

        $windowsSettings = echo $testString | ConvertFrom-Json
        Mock Get-ItemProperty -MockWith {
            return @{
                EnableCertPaddingCheck = "1"
                EnableCertPaddingCheck2 = "2"
            }
        }

        $windowsSku = "2019-containerd-gen2"
        $lines = LogReleaseNotesForWindowsRegistryKeys $windowsSettings

        $lines.Length | Should -Be 3
        $lines | Should -Contain ("`t{0}" -f "pathpath")
        $lines | Should -Contain ("`t`t{0} : {1}" -f "EnableCertPaddingCheck", "1")
        $lines | Should -Contain ("`t`t{0} : {1}" -f "EnableCertPaddingCheck2", "2")
    }
}

Describe 'tests of windows_settings' {
    BeforeEach {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "this is a comment",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    It 'given windows sku matches, it returns the key' {
        $windowsSku = "2019-containerd-gen2"
        $regKeysToApplyt1 = GetRegKeysToApply $windowsSettings
        $regKeysToApplyt1.Length | Should -Be 1
    }

    It 'given windows sku does not match, it does not returns the key' {
        $windowsSku = "2022-containerd-gen2"
        $regKeysToApplyt2 = GetRegKeysToApply $windowsSettings
        $regKeysToApplyt2.Length | Should -Be 0
    }

    It 'given two windows keys match, it returns both' {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    },
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1"
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
        $windowsSku = "2019-containerd-gen2"
        $regKeysToApplyt3 = GetRegKeysToApply $windowsSettings
        $regKeysToApplyt3.Length | Should -Be 2
    }
}


Describe "Gets the paths and names for release notes" {
    BeforeEach {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    It 'given windows sku matches, the names for key should contain the name' {
        $windowsSku = "2019-containerd-gen2"
        $items = GetKeyMapForReleaseNotes $windowsSettings
        $namesForKey = $items["pathpath"]
        $namesForKey.Length | Should -Be 1
        $namesForKey | Should -Contain "EnableCertPaddingCheck"
    }

    It 'given windows sku does not match, the names for key should not contain the name' {
        $windowsSku = "2022-containerd-gen2"
        $items = GetKeyMapForReleaseNotes $windowsSettings
        $namesForKey = $items["pathpath"]
        $namesForKey | Should -Be $null
    }

    It 'given two items with the same path match, it combines them' {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck",
      "Value": "1"
    },
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1"
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
        $windowsSku = "2019-containerd-gen2"
        $items = GetKeyMapForReleaseNotes $windowsSettings
        $namesForKey = $items["pathpath"]
        $namesForKey.Length | Should -Be 2
        $namesForKey | Should -Contain "EnableCertPaddingCheck"
        $namesForKey | Should -Contain "EnableCertPaddingCheck2"
    }
}

Describe 'Gets the Binaries' {
    BeforeEach {
        $testString = '{
  "Packages": [
    {
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "downloadLocation": null,
      "downloadUris": {
        "windows": {
          "default": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.50",
                "previousLatestVersion": "0.0.51"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
          }
        }
      }
    }
]}'
        $componentsJson = echo $testString | ConvertFrom-Json
    }

    It 'given there are no packages, it returns an empty hashtable' {
        $componentsJson.Packages = @()

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'given the windows block is missing, it returns an empty array' {
        $componentsJson.Packages[0].downloadUris.windows = $null

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'given there are no windows versions, it returns an empty array' {
        $componentsJson.Packages[0].downloadUris.windows = @()

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'given there is a latest version and previousLatestVersion, it returns both version' {
        $componentsJson.Packages[0].downloadUris.windows.default.versionsV2 = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
                previousLatestVersion = "1.8.21"
            }
        )
        $componentsJson.Packages[0].windowsDownloadLocation = "location"
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v`${version}.zip"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.22.zip"
        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.21.zip"
    }

    It 'skips a package if windowsDownloadLocation is not set' {
        $componentsJson.Packages[0].downloadLocation = "linuxLocation"
        $componentsJson.Packages[0].windowsDownloadLocation = $null
        $componentsJson.Packages[0].downloadUris.windows.default.versionsV2 = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
                previousLatestVersion = "1.8.21"
            }
        )
        $componentsJson.Packages[0].windowsDownloadLocation = "location"
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v`${version}.zip"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["other_location"] | Should -Not -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.22.zip"
        $packages["other_location"] | Should -Not -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.21.zip"
    }

    It 'given there is a latest version, it returns that version' {
        $componentsJson.Packages[0].downloadUris.windows.default.versionsV2 = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )
        $componentsJson.Packages[0].windowsDownloadLocation = "location"
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v`${version}.zip"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.22.zip"
    }

    It 'given there are two packages in the same directory, it combines them' {
        $testString = '{
  "Packages": [
    {
      "windowsDownloadLocation": "location",
      "downloadUris": {
        "windows": {
          "default": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.48"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
          }
        }
      }
    },
    {
      "windowsDownloadLocation": "location",
      "downloadUris": {
        "windows": {
          "default": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.49"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
          }
        }
      }
    }
  ]
}'
        $componentsJson = echo $testString | ConvertFrom-Json

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.48.zip"
        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.49.zip"
    }

    It 'given there is an override for ws2022 and ws2022 sku is being built, it uses the override' {
        $testString = '{
  "Packages": [
    {
      "windowsDownloadLocation": "location",
      "downloadUris": {
        "windows": {
          "ws2022": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.49"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
          },
          "default": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.48"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
          }
        }
      }
    }
  ]
}'
        $componentsJson = echo $testString | ConvertFrom-Json
        $windowsSku = "2022-containerd-gen2"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.49.zip"
        $packages["location"] | Should -Not -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.48.zip"
    }
}

Describe 'Gets the OCI Artifacts' {
    BeforeEach {
        $testString = '{
  "OCIArtifacts": [
    {
      "registry": "mcr.microsoft.com/aks/oci-artifact:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "1.0.0",
          "previousLatestVersion": "0.9.0"
        }
      ]
    }
  ]}'
        $componentsJson = echo $testString | ConvertFrom-Json
    }

    It 'given there are no OCI artifacts, it returns an empty hashtable' {
        $componentsJson.OCIArtifacts = @()

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'given the windowsVersions block is missing, it returns an empty hashtable' {
        $componentsJson.OCIArtifacts[0].windowsVersions = $null

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'skips an artifact if windowsDownloadLocation is not set' {
        $componentsJson.OCIArtifacts[0].windowsDownloadLocation = $null

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'given there is a latest version and previousLatestVersion, it returns both versions' {
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact:1.0.0"
        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact:0.9.0"
    }

    It 'given there is only a latest version, it returns just that version' {
        $componentsJson.OCIArtifacts[0].windowsVersions[0].previousLatestVersion = $null

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact:1.0.0"
        $artifacts["c:\akse-cache\"] | Should -HaveCount 1
    }

    It 'given there are two artifacts in the same directory, it combines them' {
        $testString = '{
  "OCIArtifacts": [
    {
      "registry": "mcr.microsoft.com/aks/oci-artifact1:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "1.0.0"
        }
      ]
    },
    {
      "registry": "mcr.microsoft.com/aks/oci-artifact2:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "2.0.0"
        }
      ]
    }
  ]}'
        $componentsJson = echo $testString | ConvertFrom-Json

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact1:1.0.0"
        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact2:2.0.0"
    }

    It 'given there is a windowsSkuMatch that matches the current sku, it returns the version' {
        $testString = '{
  "OCIArtifacts": [
    {
      "registry": "mcr.microsoft.com/aks/oci-artifact:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "1.0.0",
          "windowsSkuMatch": "2022-containerd*"
        }
      ]
    }
  ]}'
        $componentsJson = echo $testString | ConvertFrom-Json
        $windowsSku = "2022-containerd-gen2"

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/oci-artifact:1.0.0"
    }

    It 'given there is a windowsSkuMatch that does not match the current sku, it does not return the version' {
        $testString = '{
  "OCIArtifacts": [
    {
      "registry": "mcr.microsoft.com/aks/oci-artifact:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "1.0.0",
          "windowsSkuMatch": "2022-containerd*"
        }
      ]
    }
  ]}'
        $componentsJson = echo $testString | ConvertFrom-Json
        $windowsSku = "2019-containerd"

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts | ConvertTo-Json -Compress | Should -Be "{}"
    }

    It 'replaces CPU_ARCH in the string' {
        $testString = '{
  "OCIArtifacts": [
    {
      "registry": "mcr.microsoft.com/aks/${CPU_ARCH}/oci-artifact:*",
      "windowsDownloadLocation": "c:\\akse-cache\\",
      "windowsVersions": [
        {
          "renovateTag": "<DO_NOT_UPDATE>",
          "latestVersion": "1.0.0"
        }
      ]
    }
  ]}'
        $componentsJson = echo $testString | ConvertFrom-Json
        $CPU_ARCH = "amd64"

        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\"] | Should -Contain "mcr.microsoft.com/aks/amd64/oci-artifact:1.0.0"
    }
}

Describe 'Gets The Versions' {
    BeforeEach {
        $testString = '{
"ContainerImages": [
{
"downloadURL": "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*",
"amd64OnlyVersions": [],
"windowsVersions": []
}]}'
        $componentsJson = echo $testString | ConvertFrom-Json
    }

    It 'replaces CPU_ARCH in the string' {
        $componentsJson.ContainerImages[0].windowsVersions = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )
        $componentsJson.ContainerImages[0].downloadURL = "mcr.microsoft.com/oss/kubernetes/autoscaler/`${CPU_ARCH}/addon-resizer:*"

        $CPU_ARCH = "x86"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/x86/addon-resizer:1.8.22"
    }

    It 'does not replace varvarvar in the string' {
        $componentsJson.ContainerImages[0].windowsVersions = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )
        $componentsJson.ContainerImages[0].downloadURL = "mcr.microsoft.com/oss/kubernetes/autoscaler/`${varvarvar}/addon-resizer:*"

        $varvarvar = "x86"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler//addon-resizer:1.8.22"
    }

    It 'given there are no container images, it returns an empty array' {
        $componentsJson.ContainerImages = @()

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Be @()
    }

    It 'given there are no windows versions, it returns an empty array' {
        $componentsJson.ContainerImages[0].windowsVersions = @()

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Be @()
    }

    It 'given the windowsVersions block is missing, it returns an empty array' {
        $componentsJson.ContainerImages[0].windowsVersions = $null

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Be @()
    }

    It 'given there is a latest version, it returns that version' {
        $componentsJson.ContainerImages[0].windowsVersions = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
    }


    It 'given there are two windows versions, it returns both images' {
        $componentsJson.ContainerImages = @(
            [PSCustomObject]@{
                downloadUrl = "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
                windowsVersions = @(
                    [PSCustomObject]@{
                        latestVersion = "1.8.22"
                    }
                    [PSCustomObject]@{
                        latestVersion = "1.8.44"
                    }
                )
            }
        )

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 2
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.44"
    }

    It 'given there is a previous latest version, it returns both images' {
        $componentsJson.ContainerImages = @(
            [PSCustomObject]@{
                downloadUrl = "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
                windowsVersions = @(
                    [PSCustomObject]@{
                        latestVersion = "1.8.22"
                        previousLatestVersion = "1.8.44"
                    }
                )
            }
        )

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 2
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.44"
    }


    It 'given there is not a previous latest version, it returns just the latest' {
        $componentsJson.ContainerImages = @(
            [PSCustomObject]@{
                downloadUrl = "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
                windowsVersions = @(
                    [PSCustomObject]@{
                        latestVersion = "1.8.22"
                        previousLatestVersion = ""
                    }
                )
            }
        )

        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
    }

    It 'given there is a sku match that matches the current sku, it returns the version' {
        $componentsJson.ContainerImages = @(
            [PSCustomObject]@{
                downloadUrl = "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
                windowsVersions = @(
                    [PSCustomObject]@{
                        latestVersion = "1.8.22"
                        windowsSkuMatch = "2022-containerd*"
                    }
                )
            }
        )

        $windowsSku = "2022-containerd-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
    }

    It 'given there is a sku match that does not matches the current sku, it returns the version' {
        $componentsJson.ContainerImages = @(
            [PSCustomObject]@{
                downloadUrl = "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
                windowsVersions = @(
                    [PSCustomObject]@{
                        latestVersion = "1.8.22"
                        windowsSkuMatch = "2022-containerd*"
                    }
                )
            }
        )

        $windowsSku = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Be @()
    }
}

Describe 'Tests of components-test.json ' {
    BeforeEach {
        # Note that we use a test components.json file - this file is validated to match our components.cue file during builds.
        $componentsJson = Get-Content 'vhdbuilder/packer/windows/components-test.json' | Out-String | ConvertFrom-Json
    }

    it 'can get the right version of a container that has different windows and linux versions' {
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/container/with/seperate/win/and/linux/versions:win-version"
        $components | Should -Not -Contain "mcr.microsoft.com/container/with/seperate/win/and/linux/versions:linux-version"
    }

    it 'can get a specific version of a windows container for win 2019 where there is a different windows download url' {
        $windowsSku = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Contain "mcr.microsoft.com/windows-url/win:v-for-2019"
        $components | Should -Not -Contain "mcr.microsoft.com/windows-url/win:v-for-2022"
        $components | Should -Not -Contain "mcr.microsoft.com/linux-url/lin:v-for-linux"
    }

    it 'can get a specific version of a windows container for win 2022 where there is a different windows download url' {
        $windowsSku = "2022-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Not -Contain "mcr.microsoft.com/windows-url/win:v-for-2019"
        $components | Should -Contain "mcr.microsoft.com/windows-url/win:v-for-2022"
        $components | Should -Not -Contain "mcr.microsoft.com/linux-url/lin:v-for-linux"
    }

    it 'can ignore windows 23H2 when there is a version for 2019 and 2022' {
        $windowsSku = "23H2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Not -Contain "mcr.microsoft.com/windows-url/win:v-for-2019"
        $components | Should -Not -Contain "mcr.microsoft.com/windows-url/win:v-for-2022"
        $components | Should -Not -Contain "mcr.microsoft.com/linux-url/lin:v-for-linux"
    }

    it 'has the no version of ciprod for win 2025' {
        $windowsSku = "2025"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Not -Contain "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-3.1.25"
    }

    It 'has the latest 2 versions of a package' {
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/win-cse-scripts-vLatest.zip"
        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/win-cse-scripts-vPrev.zip"
    }

    it 'can get packages from the default when there is no windows override set' {
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.27.102-akslts/windowszip/v1.27.102-akslts-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.27.101-akslts/windowszip/v1.27.101-akslts-1int.zip"
    }

    It 'has specific WS2019 containers' {
        $windowsSku = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # core images shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2019"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:1809"
    }

    It 'has specific WS2022 containers' {
        $windowsSku = "2022-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS2022-gen2 containers' {
        $windowsSku = "2022-containerd-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }


    It 'has specific WS23H2 containers' {
        $windowsSku = "23H2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS23H2-gen2 containers' {
        $windowsSku = "23H2-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS2025 containers' {
        $windowsSku = "2025"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2025"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2025"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS2025-gen2 containers' {
        $windowsSku = "2025-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2025"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2025"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has containerd versions for 2019' {
        $windowsSku = "2019-containerd"
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 2022' {
        $windowsSku = "2022-containerd"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 2022-gen2' {
        $windowsSku = "2022-containerd-gen2"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 23H2' {
        $windowsSku = "23H2"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Not -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 23H2-gen2' {
        $windowsSku = "23H2-gen2"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Not -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 2025' {
        $windowsSku = "2025"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Not -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    It 'has containerd versions for 2025-gen2' {
        $windowsSku = "2025-gen2"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
        $packages["c:\akse-cache\containerd\"] | Should -Not -Contain "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    it 'has the right default containerd for ws2019' {
        $windowsSku = "2019-containerd"

        $containerDUrl = GetDefaultContainerDFromComponentsJson $componentsJson

        $containerDUrl | Should -Be "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    it 'has the right default containerd for ws2022' {
        $windowsSku = "2022-containerd"

        $containerDUrl = GetDefaultContainerDFromComponentsJson $componentsJson

        $containerDUrl | Should -Be "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
    }

    it 'has the right default containerd for ws23H2' {
        $windowsSku = "23H2"

        $containerDUrl = GetDefaultContainerDFromComponentsJson $componentsJson

        $containerDUrl | Should -Be "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
    }

    it 'has the right default containerd for ws2025' {
        $windowsSku = "2025"

        $containerDUrl = GetDefaultContainerDFromComponentsJson $componentsJson

        $containerDUrl | Should -Be "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
    }

    it 'can get the right OCI artifacts with latest and previous versions' {
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry:1.0.0"
        $artifacts["c:\akse-cache\oci-test\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry:0.9.0"
    }

    it 'can get OCI artifacts with CPU_ARCH variable replacement' {
        $CPU_ARCH = "amd64"
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test-arch\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry-arch-amd64:2.0.0"
    }
    
    it 'can get OCI artifacts with CPU_ARCH and version variable replacement' {
        $CPU_ARCH = "amd64"
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test-arch-and-version\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry-arch-and-version:2.0.0-amd64"
    }

    it 'can get OCI artifacts with Windows 2019 SKU matching' {
        $windowsSku = "2019-containerd"
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2019-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2022-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2025-version"
    }

    it 'can get OCI artifacts with Windows 2022 SKU matching' {
        $windowsSku = "2022-containerd"
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2019-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2022-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2025-version"
    }

    it 'can get OCI artifacts with Windows 2025 SKU matching' {
        $windowsSku = "2025"
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson

        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2019-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2022-version"
        $artifacts["c:\akse-cache\oci-test-sku\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry-sku:2025-version"
    }

    it 'skips OCI artifacts with missing windowsDownloadLocation' {
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJson
        
        # No entry should exist for the artifact without a download location
        $artifacts.Keys | ForEach-Object { $_ } | Should -Not -Contain "mcr.microsoft.com/aks/oci-test-registry-no-location:3.0.0"
    }

    it 'can get OCI artifacts from the default when there is no windows override set' {
        # Modify a copy of the components.json to test default behavior
        $componentsJsonCopy = $componentsJson | ConvertTo-Json -Depth 10 | ConvertFrom-Json
        
        # Find the oci-test-registry artifact and set windowsVersions to null to test default behavior
        foreach ($artifact in $componentsJsonCopy.OCIArtifacts) {
            if ($artifact.registry -like "*oci-test-registry*" -and $artifact.repository -like "*oci-test-registry*") {
                $artifact.windowsVersions = $null
                break
            }
        }
        
        $artifacts = GetOCIArtifactsFromComponentsJson $componentsJsonCopy
        
        # Should still get the artifact using the default version
        $artifacts["c:\akse-cache\oci-test\"] | Should -Contain "mcr.microsoft.com/aks/oci-test-registry:1.0.0"
    }
}
