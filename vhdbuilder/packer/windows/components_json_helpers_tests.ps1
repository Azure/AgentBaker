BeforeAll {
    . $PSScriptRoot\components_json_helpers.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')
}

Describe 'Gets The Versions' {
    BeforeEach {
        # randomized path, to get fresh file for each test
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

    it 'can parse components.json' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/pause:3.9"
    }

    It 'has specific WS2019 containers' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $windowsSKU = "2019-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2019"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:1809"
    }

    It 'has specific WS2022 containers' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $windowsSKU = "2022-containerd"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS2022-gen2 containers' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $windowsSKU = "2022-containerd-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }


    It 'has specific WS23H2-gen2 containers' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $windowsSKU = "23H2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }

    It 'has specific WS23H2-gen2 containers' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $windowsSKU = "23H2-gen2"
        $components = GetComponentsFromComponentsJson $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/windows/servercore:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/windows/nanoserver:ltsc2022"
        $components | Should -Contain "mcr.microsoft.com/containernetworking/azure-npm:v1.5.5"
    }
}