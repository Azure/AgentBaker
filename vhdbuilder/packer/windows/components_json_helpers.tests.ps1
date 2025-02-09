BeforeAll {
    . $PSScriptRoot\components_json_helpers.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')
}

Describe 'tests of windows_settings' {
    BeforeEach {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "HKLM:\\Software\\Microsoft\\Cryptography\\Wintrust\\Config",
      "Name": "EnableCertPaddingCheck",
      "Value": "1",
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    It 'given windows sku matches, it returns the key' {
        $windowsSku = "2019-containerd-gen2"
        $keys = GetRegKeysToApply $windowsSettings
        $keys.Length | Should -Be 1
    }

    It 'given windows sku does not match, it does not returns the key' {
        $windowsSku = "2022-containerd-gen2"
        $keys = GetRegKeysToApply $windowsSettings
        $keys.Length | Should -Be 0
    }

    It 'given two windows keys match, it returns both' {
        $testString = '{
  "WindowsRegistryKeys": [
    {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "HKLM:\\Software\\Microsoft\\Cryptography\\Wintrust\\Config",
      "Name": "EnableCertPaddingCheck",
      "Value": "1",
    }, {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "HKLM:\\Software\\Microsoft\\Cryptography\\Wintrust\\Config",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1",
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
        $windowsSku = "2019-containerd-gen2"
        $keys = GetRegKeysToApply $windowsSettings
        $keys.Length | Should -Be 2
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
      "Value": "1",
    }
  ]
}'
        $windowsSettings = echo $testString | ConvertFrom-Json
    }

    It 'given windows sku matches, it returns the key' {
        $windowsSku = "2019-containerd-gen2"
        $items = GetKeyMapForReleaseNotes $windowsSettings
        $namesForKey = $items["pathpath"]
        $namesForKey.Length | Should -Be 1
        $namesForKey | Should -Contain "EnableCertPaddingCheck"
    }

    It 'given windows sku does not match, it does not returns the key' {
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
      "Value": "1",
    }, {
      "Comment": "https://msrc.microsoft.com/update-guide/vulnerability/CVE-2013-3900",
      "WindowsSkuMatch": "2019*",
      "Path": "pathpath",
      "Name": "EnableCertPaddingCheck2",
      "Value": "1",
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
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
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
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"

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
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"

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
        $componentsJson.Packages[0].downloadUris.windows.default.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"

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
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
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
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
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
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
          },
          "default": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.48"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
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

# note that we might remove some of these as we change the versions. Most of them were written to ensure current versions were
# migrated successfully
Describe 'Tests of components.json ' {
    BeforeEach {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json
    }

    it 'can parse components.json' {
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/pause:3.9"
    }

    it 'has the right version of ciprod' {
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Contain "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-3.1.24"
    }

    It 'has the latest 2 versions of windows scripts and cgmaplugin' {
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.51.zip"
        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.50.zip"
        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/ccgakvplugin/v1.1.5/binaries/windows-gmsa-ccgakvplugin-v1.1.5.zip"
    }

    it 'has csi proxy' {
        $packages = GetPackagesFromComponentsJson $componentsJson
        $packages["c:\akse-cache\csi-proxy\"] | Should -Contain "https://acs-mirror.azureedge.net/csi-proxy/v1.1.2-hotfix.20230807/binaries/csi-proxy-v1.1.2-hotfix.20230807.tar.gz"
    }

    it 'has credential provider' {
        $packages = GetPackagesFromComponentsJson $componentsJson
        $packages["c:\akse-cache\credential-provider\"] | Should -Contain "https://acs-mirror.azureedge.net/cloud-provider-azure/v1.29.2/binaries/azure-acr-credential-provider-windows-amd64-v1.29.2.tar.gz"
        $packages["c:\akse-cache\credential-provider\"] | Should -Contain "https://acs-mirror.azureedge.net/cloud-provider-azure/v1.30.0/binaries/azure-acr-credential-provider-windows-amd64-v1.30.0.tar.gz"
    }

    it 'has calico' {
        $packages = GetPackagesFromComponentsJson $componentsJson
        $packages["c:\akse-cache\calico\"] | Should -Contain "https://acs-mirror.azureedge.net/calico-node/v3.24.0/binaries/calico-windows-v3.24.0.zip"
    }

    it 'has vnet-cni' {
        $packages = GetPackagesFromComponentsJson $componentsJson
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain  "https://acs-mirror.azureedge.net/azure-cni/v1.5.38/binaries/azure-vnet-cni-windows-amd64-v1.5.38.zip"
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain "https://acs-mirror.azureedge.net/azure-cni/v1.6.18/binaries/azure-vnet-cni-windows-amd64-v1.6.18.zip"
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain "https://acs-mirror.azureedge.net/azure-cni/v1.4.58/binaries/azure-vnet-cni-swift-windows-amd64-v1.4.58.zip"
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain "https://acs-mirror.azureedge.net/azure-cni/v1.4.59/binaries/azure-vnet-cni-swift-windows-amd64-v1.4.59.zip"
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain "https://acs-mirror.azureedge.net/azure-cni/v1.4.58/binaries/azure-vnet-cni-overlay-windows-amd64-v1.4.58.zip"
        $packages["c:\akse-cache\win-vnet-cni\"] | Should -Contain "https://acs-mirror.azureedge.net/azure-cni/v1.4.59/binaries/azure-vnet-cni-overlay-windows-amd64-v1.4.59.zip"
    }

    it 'has kubenetes binaries' {
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.27.102-akslts/windowszip/v1.27.102-akslts-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.27.101-akslts/windowszip/v1.27.101-akslts-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.28.15/windowszip/v1.28.15-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.28.100-akslts/windowszip/v1.28.100-akslts-1int.zip"
        #        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.29.11/windowszip/v1.29.11-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.29.12/windowszip/v1.29.12-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.29.13/windowszip/v1.29.13-1int.zip"
        #        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.30.7/windowszip/v1.30.7-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.30.8/windowszip/v1.30.8-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.30.9/windowszip/v1.30.9-1int.zip"
        #        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.31.3/windowszip/v1.31.3-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.31.4/windowszip/v1.31.4-1int.zip"
        $packages["c:\akse-cache\win-k8s\"] | Should -Contain "https://acs-mirror.azureedge.net/kubernetes/v1.31.5/windowszip/v1.31.5-1int.zip"
    }

    It 'has specific WS2019 containers' {
        $windowsSku = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
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


    It 'has specific WS23H2-gen2 containers' {
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
}