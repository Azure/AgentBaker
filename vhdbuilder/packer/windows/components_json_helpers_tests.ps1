BeforeAll {
    . $PSScriptRoot\components_json_helpers.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')
}

Describe 'Gets the Binaries' {
    BeforeEach {
        $testString = '{
  "Packages": [
    {
      "downloadLocation": "c:\\akse-cache\\",
      "downloadUris": {
        "windows": {
          "current": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.48",
                "previousLatestVersion": "0.0.50"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
          }
        }
      }
    },
]}'
        $componentsJson = echo $testString | ConvertFrom-Json
    }

    It 'given there are no packages, it returns an empty hashtable' {
        $componentsJson.Packages = @()

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages | ConvertTo-Json | Should -Be "{}"
    }

    It 'given there are no windows versions, it returns an empty array' {
        $componentsJson.Packages[0].downloadUris.windows = @()

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages | ConvertTo-Json | Should -Be "{}"
    }

    It 'given there is a latest version and previousLatestVersion, it returns both version' {
        $componentsJson.Packages[0].downloadUris.windows.current.versionsV2 = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
                previousLatestVersion = "1.8.21"
            }
        )
        $componentsJson.Packages[0].downloadLocation = "location"
        $componentsJson.Packages[0].downloadUris.windows.current.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.22.zip"
        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.21.zip"
    }

    It 'given there is a latest version, it returns that version' {
        $componentsJson.Packages[0].downloadUris.windows.current.versionsV2 = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )
        $componentsJson.Packages[0].downloadLocation = "location"
        $componentsJson.Packages[0].downloadUris.windows.current.downloadURL = "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v1.8.22.zip"
    }

    It 'given there are two packages in the same directory, it combines them' {
        $testString = '{
  "Packages": [
    {
      "downloadLocation": "location",
      "downloadUris": {
        "windows": {
          "current": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.48",
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
          }
        }
      }
    },
     {
      "downloadLocation": "location",
      "downloadUris": {
        "windows": {
          "current": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "0.0.49",
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v[version].zip"
          }
        }
      }
    },
]}'
        $componentsJson = echo $testString | ConvertFrom-Json

        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.48.zip"
        $packages["location"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.49.zip"
    }

}

Describe 'Gets The Versions' {
    BeforeEach {
        $testString = '{
"ContainerImages": [
{
"downloadURL": "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*",
"amd64OnlyVersions": [],
"windowsVersions": [],
},]}'
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

        $windowsSKU = "2022-containerd-gen2"
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

        $windowsSKU = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components | Should -Be @()
    }

}

Describe 'Tests of components.json' {
    BeforeEach {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json
    }

    it 'can parse components.json' {
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/pause:3.9"
    }

    It 'has the latest 2 versions of windows scripts and cgmaplugin' {
        $packages = GetPackagesFromComponentsJson $componentsJson

        $packages.Length | Should -BeGreaterThan 0

        $packages["c:\akse-cache\"] | Should -Contain "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.48.zip"
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

    It 'has specific WS2019 containers' {
        $windowsSKU = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2019"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:1809"
    }

    It 'has specific WS2022 containers' {
        $windowsSKU = "2022-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS2022-gen2 containers' {
        $windowsSKU = "2022-containerd-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }


    It 'has specific WS23H2-gen2 containers' {
        $windowsSKU = "23H2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS23H2-gen2 containers' {
        $windowsSKU = "23H2-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }
}