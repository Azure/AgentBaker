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
}
