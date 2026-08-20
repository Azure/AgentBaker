BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSScriptRoot\networkisolatedclusterfunc.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')

    $capturedContent = $null
    Mock Set-Content -MockWith {
        param($Path, $Value)
        $script:capturedContent = $Value
    } -Verifiable

    Mock Remove-Item
}

Describe 'Adjust-DynamicPortRange' {
    BeforeEach {
        Mock Invoke-Executable
    }

    Context '$global:EnableIncreaseDynamicPortRange is true' {
        It "Should call Invoke-Executable 4 times" {
            $global:EnableIncreaseDynamicPortRange = $true

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 4
        }
    }

    Context '$global:EnableIncreaseDynamicPortRange is false' {
        It "Should call Invoke-Executable 1 times" {
            $global:EnableIncreaseDynamicPortRange = $false

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 1
        }
    }
}

Describe 'Resize-OSDrive' {
    BeforeEach {
        Mock Invoke-Executable
    }

    BeforeAll{
        Mock Get-Disk -MockWith {
            Write-Host "Get-Disk $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    Size = 1024*1024;
                    AllocatedSize = 1024*1024
                }
                return $valueObj
        } -Verifiable

        Mock Set-ExitCode -MockWith {
            Param(
              $ExitCode,
              $ErrorMessage
            )
            Write-Host "Set-ExitCode $ExitCode $ErrorMessage"
        } -Verifiable

        Mock Invoke-Executable {
            Param(
                $Executable,
                $ArgList,
                $ExitCode
            )
            Write-Host "Invoke-Executable $Executable $ArgList $ExitCode"
        } -Verifiable
    }

    Context 'success' {
        It "Should call Invoke-Executable to Diskpart once" {
            Mock Get-Disk -MockWith {
                Write-Host "Get-Disk Size: 512GB, AllocatedSize: 30GB $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    Size = 512GB;
                    AllocatedSize = 30GB
                }
                return $valueObj
            } -Verifiable
            Resize-OSDrive
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }

        It "Should not call Invoke-Executable to Diskpart once" {
            Mock Get-Disk -MockWith {
                Write-Host "Get-Disk Size: 30GB, AllocatedSize: 30GB $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    Size = 30GB;
                    AllocatedSize = 30GB
                }
                return $valueObj
            } -Verifiable

            Resize-OSDrive
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }

    Context 'fail' {
        BeforeEach {
            Mock Get-Disk -MockWith {
                throw "Get-Disk $ErrorAction"
            } -Verifiable
        }

        It "Should not call Invoke-Executable" {
            Resize-OSDrive
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE }
        }
    }
}

Describe 'Config-CredentialProvider' {
    BeforeEach {
        $global:credentialProviderConfigDir = "staging\cse\windows\credentialProvider.tests.suites"
        $CredentialProviderConfPATH=[Io.path]::Combine("$global:credentialProviderConfigDir", "credential-provider-config.yaml")
        function Read-Format-Yaml ([string]$YamlFile) {
            # Read the file content directly without conversion
            return Get-Content -Path $YamlFile -Raw
        }
    }

    AfterEach {
        Remove-Item -Path $CredentialProviderConfPATH
    }

    Context 'CustomCloudContainerRegistryDNSSuffix is empty' {
        It "should match the expected config file content" {
            $expectedCredentialProviderConfig = Read-Format-Yaml ([Io.path]::Combine($credentialProviderConfigDir, "CustomCloudContainerRegistryDNSSuffixEmpty.config.yaml"))
            Config-CredentialProvider -KubeDir $credentialProviderConfigDir -CredentialProviderConfPath $CredentialProviderConfPATH -CustomCloudContainerRegistryDNSSuffix ""

            $acutalCredentialProviderConfig = Read-Format-Yaml $CredentialProviderConfPATH
            # Compare the content by normalizing whitespace and line endings
            $normalizedExpected = $expectedCredentialProviderConfig.Trim().Replace("`r`n", "`n")
            $normalizedActual = $acutalCredentialProviderConfig.Trim().Replace("`r`n", "`n")
            $normalizedActual | Should -Be $normalizedExpected
        }
    }
    Context 'CustomCloudContainerRegistryDNSSuffix is not empty' {
       It "should match the expected config file content" {
            $expectedCredentialProviderConfig = Read-Format-Yaml ([Io.path]::Combine($credentialProviderConfigDir, "CustomCloudContainerRegistryDNSSuffixNotEmpty.config.yaml"))
            Config-CredentialProvider -KubeDir $credentialProviderConfigDir -CredentialProviderConfPath $CredentialProviderConfPATH -CustomCloudContainerRegistryDNSSuffix ".azurecr.microsoft.fakecloud"
            $acutalCredentialProviderConfig = Read-Format-Yaml $CredentialProviderConfPATH

            # Compare the content by normalizing whitespace and line endings
            $normalizedExpected = $expectedCredentialProviderConfig.Trim().Replace("`r`n", "`n")
            $normalizedActual = $acutalCredentialProviderConfig.Trim().Replace("`r`n", "`n")
            $normalizedActual | Should -Be $normalizedExpected
       }
    }
}

Describe 'Validate-CredentialProviderConfigFlags' {
    BeforeEach {
        $global:KubeletConfigArgs = @( "--address=0.0.0.0" )
        $global:credentialProviderConfigPath = ""
        $global:credentialProviderBinDir = ""
    }

    BeforeAll{
        Mock Set-ExitCode -MockWith {
            Param(
              $ExitCode,
              $ErrorMessage
            )
            Write-Host "Set-ExitCode $ExitCode $ErrorMessage"
        } -Verifiable
    }

    Context 'success' {
        It "Should return expected config path and bin path" {
            $expectedCredentialProviderConfigPath="c:\k\credential-provider-config.yaml"
            $expectedCredentialProviderBinDir="c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs+="--image-credential-provider-config="+$expectedCredentialProviderConfigPath
            $global:KubeletConfigArgs+="--image-credential-provider-bin-dir="+$expectedCredentialProviderBinDir
            Validate-CredentialProviderConfigFlags
            Compare-Object $global:credentialProviderConfigPath $expectedCredentialProviderConfigPath | Should -Be $null
            Compare-Object $global:credentialProviderBinDir $expectedCredentialProviderBinDir | Should -Be $null
        }

        It "Should return empty config path and bin path" {
            $expectedCredentialProviderConfigPath=""
            $expectedCredentialProviderBinDir=""
            Validate-CredentialProviderConfigFlags
            Compare-Object $global:credentialProviderConfigPath $expectedCredentialProviderConfigPath | Should -Be $null
            Compare-Object $global:credentialProviderBinDir $expectedCredentialProviderBinDir | Should -Be $null
        }
    }

    Context 'fail' {
        It "Should call Set-ExitCode when only config path is specified" {
            $expectedCredentialProviderConfigPath="c:\k\credential-provider_config.yaml"
            $global:KubeletConfigArgs+="--image-credential-provider-config="+$expectedCredentialProviderConfigPath
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
        It "Should call Set-ExitCode when only bin dir is specified" {
            $expectedCredentialProviderBinDir="c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs+="--image-credential-provider-bin-dir="+$expectedCredentialProviderBinDir
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
        It "Should call Set-ExitCode when flag value is emtpy string" {
            $expectedCredentialProviderBinDir="c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs+="--image-credential-provider-bin-dir="
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
    }
}

Describe 'Install-CredentialProvider' {
    BeforeEach {
        $global:credentialProviderConfigPath = ""
        $global:credentialProviderBinDir = ""
        $global:KubeletConfigArgs = @(
            "--image-credential-provider-config=c:\k\credential-provider-config.yaml",
            "--image-credential-provider-bin-dir=c:\var\lib\kubelet\credential-provider"
        )
        $global:CredentialProviderURL = "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.34.0/windows/amd64/azure-acr-credential-provider_1.34.0-1_amd64.zip"
        $global:BootstrapProfileContainerRegistryServer = "myregistry.azurecr.io"
        $global:KubeBinariesVersion = "1.31.9"
        $script:lastDownloadReference = ""

        Mock Config-CredentialProvider
        Mock New-TemporaryDirectory -MockWith { "C:\temp\credprovider" }
        Mock DownloadFileOverHttp
        Mock DownloadFileWithOras -MockWith {
            param(
                [string]$Reference,
                [string]$DestinationPath,
                [string]$Platform
            )
            $script:lastDownloadReference = $Reference
        }
        Mock AKS-Expand-Archive
        Mock Create-Directory
        Mock cp
        Mock del
        Mock tar -MockWith { $global:LASTEXITCODE = 0 }
        Mock Get-Command -MockWith {
            [pscustomobject]@{ Name = "DownloadFileWithOras" }
        } -ParameterFilter { $Name -eq 'DownloadFileWithOras' }
        Mock Set-ExitCode -MockWith {
            Param($ExitCode, $ErrorMessage)
            throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
            return
        }
    }
    AfterEach {
        $global:BootstrapProfileContainerRegistryServer = $null
    }

    It 'returns early when out-of-tree credential provider flags are not configured' {
        $global:KubeletConfigArgs = @("--address=0.0.0.0")

        { Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix '' } | Should -Not -Throw
        Assert-MockCalled -CommandName 'Config-CredentialProvider' -Times 0
        $script:lastDownloadReference | Should -Be ""
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
    }

    It 'uses legacy binaries URL for non-ni cluster' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.34.0/binaries/azure-acr-credential-provider-linux-amd64-v1.34.0.tar.gz'
        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1
    }

    It 'uses version parsed from dalec URL for ORAS reference' {
        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        $script:lastDownloadReference | Should -Be 'myregistry.azurecr.io/aks/packages/kubernetes/azure-acr-credential-provider:v1.34.0'
    }

    It 'uses version parsed from legacy binaries URL for ORAS reference' {
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.34.0/binaries/azure-acr-credential-provider-linux-amd64-v1.34.0.tar.gz'
        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        $script:lastDownloadReference | Should -Be 'myregistry.azurecr.io/aks/packages/kubernetes/azure-acr-credential-provider:v1.34.0'
    }

    It 'falls back to KubeBinariesVersion when URL contains no parseable version' {
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/invalid/credential-provider.zip'
        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        $script:lastDownloadReference | Should -Be 'myregistry.azurecr.io/aks/packages/kubernetes/azure-acr-credential-provider:v1.31.9'
    }

    It 'uses dalec path via AKS-Expand-Archive for k8s >= 1.33 with stock legacy RP URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage -MockWith {
            return @{
                Url = 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.6/windows/amd64/azure-acr-credential-provider_1.33.6-1_amd64.zip'
                Version = '1.33.6-1'
                CachedFile = $null
                IsDalec = $true
            }
        }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 1
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.6/windows/amd64/azure-acr-credential-provider_1.33.6-1_amd64.zip'
        }
        Assert-MockCalled -CommandName 'tar' -Times 0
    }

    It 'uses dalec path for k8s >= 1.33 with sovereign stock legacy RP URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage -MockWith {
            return @{
                Url = 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.6/windows/amd64/azure-acr-credential-provider_1.33.6-1_amd64.zip'
                Version = '1.33.6-1'
                CachedFile = 'c:\akse-cache\azure-acr-credential-provider\azure-acr-credential-provider_1.33.6-1_amd64.zip'
                IsDalec = $true
            }
        }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'Resolve-DalecCredentialProviderPackage' -Times 1
        Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 1 -ParameterFilter {
            $Path -eq 'c:\akse-cache\azure-acr-credential-provider\azure-acr-credential-provider_1.33.6-1_amd64.zip'
        }
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 0
    }

    It 'falls back to RP legacy URL when dalec download fails' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage -MockWith {
            return @{
                Url = 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.6/windows/amd64/azure-acr-credential-provider_1.33.6-1_amd64.zip'
                Version = '1.33.6-1'
                CachedFile = $null
                IsDalec = $true
            }
        }
        Mock DownloadFileOverHttp -MockWith {
            param($Url)
            if ($Url -match '/dalec-packages/') {
                throw 'dalec download failed'
            }
        }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
        }
        Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 1
    }

    It 'falls back to RP legacy URL when dalec extraction fails' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage -MockWith {
            return @{
                Url = 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.6/windows/amd64/azure-acr-credential-provider_1.33.6-1_amd64.zip'
                Version = '1.33.6-1'
                CachedFile = 'c:\akse-cache\azure-acr-credential-provider\azure-acr-credential-provider_1.33.6-1_amd64.zip'
                IsDalec = $true
            }
        }
        Mock AKS-Expand-Archive -MockWith { throw 'dalec extraction failed' }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
        }
        Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 1
        Assert-MockCalled -CommandName 'tar' -Times 1
    }

    It 'uses legacy tar path for k8s < 1.33 (below dalec gate)' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.32.8"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.32.8/binaries/azure-acr-credential-provider-windows-amd64-v1.32.8.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage -MockWith { return $null }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'tar' -Times 1
        Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 0
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq 'https://packages.aks.azure.com/cloud-provider-azure/v1.32.8/binaries/azure-acr-credential-provider-windows-amd64-v1.32.8.tar.gz'
        }
    }

    It 'honors custom URL and skips dalec for k8s >= 1.33 with non-stock URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://custom-mirror.example.com/hotfix/azure-acr-credential-provider-1.33.3-hotfix.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        # Custom URL does not match stock pattern, so dalec resolver should NOT be called
        Assert-MockCalled -CommandName 'Resolve-DalecCredentialProviderPackage' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 1
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq 'https://custom-mirror.example.com/hotfix/azure-acr-credential-provider-1.33.3-hotfix.tar.gz'
        }
    }

    It 'uses NI ORAS path unchanged when BootstrapProfileContainerRegistryServer is set' {
        $global:BootstrapProfileContainerRegistryServer = "myregistry.azurecr.io"
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        # NI path should use ORAS, not dalec resolve
        Assert-MockCalled -CommandName 'Resolve-DalecCredentialProviderPackage' -Times 0
        $script:lastDownloadReference | Should -Be 'myregistry.azurecr.io/aks/packages/kubernetes/azure-acr-credential-provider:v1.33.3'
    }
}

Describe 'Resolve-DalecCredentialProviderPackage' {
    BeforeEach {
        Mock Write-Log
    }

    It 'returns $null for k8s < 1.33' {
        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.32.8'
        $result | Should -Be $null
    }

    It 'returns $null for a malformed k8s version' {
        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'invalid-version'
        $result | Should -Be $null
    }

    It 'returns $null when components.json does not exist' {
        Mock Test-Path -MockWith { $false } -ParameterFilter { $Path -eq 'c:\k\components.json' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.33.3'
        $result | Should -Be $null
    }

    It 'returns $null when dalec entry is missing from components.json' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        Mock Get-Content -MockWith {
            return '{"Packages": [{"name": "windows credential provider"}]}'
        } -ParameterFilter { $Path -eq 'c:\k\components.json' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.33.3'
        $result | Should -Be $null
    }

    It 'returns $null when PMC has no matching k8s minor' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [{
        "name": "azure-acr-credential-provider-pmc",
        "downloadURIs": {
            "windows": {
                "default": {
                    "versionsV2": [{"latestVersion": "1.33.6-1"}],
                    "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                }
            }
        }
    }]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.34.0'
        $result | Should -Be $null
    }

    It 'does not use a matching version from the legacy tarball component' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [
        {
            "name": "azure-acr-credential-provider-pmc",
            "downloadURIs": {
                "windows": {
                    "default": {
                        "versionsV2": [{"latestVersion": "1.33.6-1"}],
                        "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                    }
                }
            }
        },
        {
            "name": "windows credential provider",
            "downloadURIs": {
                "windows": {
                    "default": {
                        "versionsV2": [{"latestVersion": "1.34.0"}],
                        "downloadURL": "https://packages.aks.azure.com/cloud-provider-azure/v${version}/binaries/azure-acr-credential-provider-windows-amd64-v${version}.tar.gz"
                    }
                }
            }
        }
    ]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.34.0'
        $result | Should -Be $null
    }

    It 'resolves correct URL and version for matching k8s minor' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [{
        "name": "azure-acr-credential-provider-pmc",
        "windowsDownloadLocation": "c:\\akse-cache\\azure-acr-credential-provider\\",
        "downloadURIs": {
            "windows": {
                "default": {
                    "versionsV2": [
                        {"latestVersion": "1.32.11-1"},
                        {"latestVersion": "1.33.6-1"},
                        {"latestVersion": "1.34.3-1"},
                        {"latestVersion": "1.35.6-2"},
                        {"latestVersion": "1.36.2-1"}
                    ],
                    "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                }
            }
        }
    }]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        Mock Get-ChildItem -MockWith { $null } -ParameterFilter { $Path -like '*akse-cache*' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.36.0'
        $result | Should -Not -Be $null
        $result.Version | Should -Be '1.36.2-1'
        $result.Url | Should -Be 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.36.2/windows/amd64/azure-acr-credential-provider_1.36.2-1_amd64.zip'
        $result.IsDalec | Should -Be $true
        $result.CachedFile | Should -Be $null
    }

    It 'returns cached file path when VHD cache hit exists' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [{
        "name": "azure-acr-credential-provider-pmc",
        "windowsDownloadLocation": "c:\\akse-cache\\azure-acr-credential-provider\\",
        "downloadURIs": {
            "windows": {
                "default": {
                    "versionsV2": [{"latestVersion": "1.33.6-1"}],
                    "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                }
            }
        }
    }]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        Mock Get-ChildItem -MockWith {
            [PSCustomObject]@{ FullName = 'c:\akse-cache\azure-acr-credential-provider\azure-acr-credential-provider_1.33.6-1_amd64.zip' }
        } -ParameterFilter { $Path -like '*akse-cache*' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3'
        $result.CachedFile | Should -Be 'c:\akse-cache\azure-acr-credential-provider\azure-acr-credential-provider_1.33.6-1_amd64.zip'
        $result.Version | Should -Be '1.33.6-1'
    }

    It 'handles version without v prefix' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [{
        "name": "azure-acr-credential-provider-pmc",
        "windowsDownloadLocation": "c:\\akse-cache\\azure-acr-credential-provider\\",
        "downloadURIs": {
            "windows": {
                "default": {
                    "versionsV2": [{"latestVersion": "1.34.3-1"}],
                    "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                }
            }
        }
    }]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        Mock Get-ChildItem -MockWith { $null } -ParameterFilter { $Path -like '*akse-cache*' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.34.1'
        $result | Should -Not -Be $null
        $result.Version | Should -Be '1.34.3-1'
    }

    It 'falls back to the legacy Dalec entry for older VHD metadata' {
        Mock Test-Path -MockWith { $true } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        $componentsContent = @'
{
    "Packages": [
        {
            "name": "azure-acr-credential-provider-pmc",
            "windowsDownloadLocation": "c:\\akse-cache\\azure-acr-credential-provider\\",
            "downloadURIs": {}
        },
        {
            "name": "windows credential provider dalec",
            "windowsDownloadLocation": "c:\\akse-cache\\azure-acr-credential-provider\\",
            "downloadURIs": {
                "windows": {
                    "default": {
                        "versionsV2": [{"latestVersion": "1.33.6-1"}],
                        "downloadURL": "https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
                    }
                }
            }
        }
    ]
}
'@
        Mock Get-Content -MockWith { return $componentsContent } -ParameterFilter { $Path -eq 'c:\k\components.json' }
        Mock Get-ChildItem -MockWith { $null } -ParameterFilter { $Path -like '*akse-cache*' }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3'
        $result.Version | Should -Be '1.33.6-1'
    }
}
