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

        $components = GetComponentsFromComponentsJson2 $componentsJson

        $components | Should -Be @()
    }

    It 'given there are no windows versions, it returns an empty array' {
        $componentsJson.ContainerImages[0].windowsVersions = @()

        $components = GetComponentsFromComponentsJson2 $componentsJson

        $components | Should -Be @()
    }

    It 'given there is a latest version, it returns that version' {
        $componentsJson.ContainerImages[0].windowsVersions = @(
            [PSCustomObject]@{
                latestVersion = "1.8.22"
            }
        )

        $components = GetComponentsFromComponentsJson2 $componentsJson

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

        $components = GetComponentsFromComponentsJson2 $componentsJson

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

        $components = GetComponentsFromComponentsJson2 $componentsJson

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

        $components = GetComponentsFromComponentsJson2 $componentsJson

        $components | Should -HaveCount 1
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:1.8.22"
    }

    it 'can parse components.json' {
        $componentsJson = Get-Content 'parts/linux/cloud-init/artifacts/components.json' | Out-String | ConvertFrom-Json

        $components = GetComponentsFromComponentsJson2 $componentsJson

        $components.Length | Should -BeGreaterThan 0

        # Pause image shouldn't change too often, so let's check that is in there.
        $components | Should -Contain "mcr.microsoft.com/oss/kubernetes/pause:3.9"
    }
}