BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSScriptRoot\networkisolatedclusterfunc.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Get-Service is a Windows-only cmdlet; stub it so Mock can override it when
    # tests run in isolation (e.g. locally on non-Windows, outside the full suite).
    function Get-Service {}

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

    BeforeAll {
        Mock Get-Disk -MockWith {
            Write-Host "Get-Disk $ErrorAction"
            $valueObj = [PSCustomObject]@{
                Size          = 1024 * 1024;
                AllocatedSize = 1024 * 1024
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
                    Size          = 512GB;
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
                    Size          = 30GB;
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
        $CredentialProviderConfPATH = [Io.path]::Combine("$global:credentialProviderConfigDir", "credential-provider-config.yaml")
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

    BeforeAll {
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
            $expectedCredentialProviderConfigPath = "c:\k\credential-provider-config.yaml"
            $expectedCredentialProviderBinDir = "c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs += "--image-credential-provider-config=" + $expectedCredentialProviderConfigPath
            $global:KubeletConfigArgs += "--image-credential-provider-bin-dir=" + $expectedCredentialProviderBinDir
            Validate-CredentialProviderConfigFlags
            Compare-Object $global:credentialProviderConfigPath $expectedCredentialProviderConfigPath | Should -Be $null
            Compare-Object $global:credentialProviderBinDir $expectedCredentialProviderBinDir | Should -Be $null
        }

        It "Should return empty config path and bin path" {
            $expectedCredentialProviderConfigPath = ""
            $expectedCredentialProviderBinDir = ""
            Validate-CredentialProviderConfigFlags
            Compare-Object $global:credentialProviderConfigPath $expectedCredentialProviderConfigPath | Should -Be $null
            Compare-Object $global:credentialProviderBinDir $expectedCredentialProviderBinDir | Should -Be $null
        }
    }

    Context 'fail' {
        It "Should call Set-ExitCode when only config path is specified" {
            $expectedCredentialProviderConfigPath = "c:\k\credential-provider_config.yaml"
            $global:KubeletConfigArgs += "--image-credential-provider-config=" + $expectedCredentialProviderConfigPath
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
        It "Should call Set-ExitCode when only bin dir is specified" {
            $expectedCredentialProviderBinDir = "c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs += "--image-credential-provider-bin-dir=" + $expectedCredentialProviderBinDir
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
        It "Should call Set-ExitCode when flag value is emtpy string" {
            $expectedCredentialProviderBinDir = "c:\var\lib\kubelet\credential-provider"
            $global:KubeletConfigArgs += "--image-credential-provider-bin-dir="
            $credentialProviderConfigs = Validate-CredentialProviderConfigFlags
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG }
        }
    }
}

Describe 'Test-GmsaPluginRegistry' {
    BeforeEach {
        $env:SystemRoot = 'C:\Windows'
        $permissionHex = '01000480440000005400000000000000140000000200300002000000000014000B000000010100000000000512000000000014000B00000001010000000000050B0000000102000000000005200000002002000001020000000000052000000020020000'
        $script:expectedPermission = [byte[]]@()
        for ($index = 0; $index -lt $permissionHex.Length; $index += 2) {
            $script:expectedPermission += [Convert]::ToByte($permissionHex.Substring($index, 2), 16)
        }

        Mock Write-Log
        Mock Test-Path -MockWith { return $true }
        Mock Get-ItemPropertyValue -MockWith {
            param($Path, $Name)

            if ($Path -like '*\ProxyStubClsid32' -and $Name -eq '(default)') {
                return '{A6FF50C0-56C0-71CA-5732-BED303A59628}'
            }
            if ($Path -like '*\Interface\{*}' -and $Name -eq '(default)') {
                return 'ICcgDomainAuthCredentials'
            }
            if ($Path -like '*\AppID\*' -and $Name -in @('AccessPermission', 'LaunchPermission')) {
                return $script:expectedPermission
            }
            if ($Path -like '*\AppID\*' -and $Name -eq 'DllSurrogate') {
                return ''
            }
            if ($Path -like '*\CLSID\*' -and $Name -eq 'AppID') {
                return '{557110E1-88BC-4583-8281-6AAC6F708584}'
            }
            if ($Path -like '*\InprocServer32' -and $Name -eq '(default)') {
                return [Io.path]::Combine($env:SystemRoot, 'System32', 'CCGAKVPlugin.dll')
            }
            if ($Path -like '*\InprocServer32' -and $Name -eq 'ThreadingModel') {
                return 'Both'
            }
            if ($Path -like '*\CCG\COMClasses\*' -and $Name -eq '(default)') {
                return ''
            }

            throw "Unexpected registry value: $Path $Name"
        }
    }

    It 'returns true when all required registry values are present' {
        Test-GmsaPluginRegistry | Should -BeTrue
    }

    It 'returns false when a required registry key is missing' {
        Mock Test-Path -MockWith {
            param($Path)
            return $Path -notlike '*\CCG\COMClasses\*'
        }

        Test-GmsaPluginRegistry | Should -BeFalse
    }

    It 'returns false when the proxy stub registration is invalid' {
        Mock Get-ItemPropertyValue -MockWith {
            return '{invalid}'
        } -ParameterFilter { $Path -like '*\ProxyStubClsid32' -and $Name -eq '(default)' }

        Test-GmsaPluginRegistry | Should -BeFalse
    }

    It 'returns false when the permission descriptor is invalid' {
        Mock Get-ItemPropertyValue -MockWith {
            return [byte[]](1)
        } -ParameterFilter { $Path -like '*\AppID\*' -and $Name -eq 'AccessPermission' }

        Test-GmsaPluginRegistry | Should -BeFalse
    }

    It 'returns false when the plugin DLL is missing' {
        Mock Test-Path -MockWith {
            return $false
        } -ParameterFilter { $Path -like '*CCGAKVPlugin.dll' -and $PathType -eq 'Leaf' }

        Test-GmsaPluginRegistry | Should -BeFalse
    }
}

Describe 'Import-GmsaPluginRegistry' {
    BeforeEach {
        $script:logMessages = @()

        Mock Write-Log -MockWith {
            param($Message)
            $script:logMessages += $Message
        }
        Mock Set-ExitCode
        Mock Test-GmsaPluginRegistry -MockWith { return $false }
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 0
            return ""
        }
    }

    It 'does not validate registry state when reg.exe succeeds' {
        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 0
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'continues when reg.exe fails but the required registry state is valid' {
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "The operation completed with errors."
        }
        Mock Test-GmsaPluginRegistry -MockWith { return $true }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
        $script:logMessages[-1] | Should -Match 'registry values are valid'
    }

    It 'fails when reg.exe fails and the required registry state is invalid' {
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "The operation failed."
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter {
            $ExitCode -eq $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES `
                -and $ErrorMessage -match 'exit code 1' `
                -and $ErrorMessage -match 'The operation failed'
        }
    }
}

Describe 'Install-OpenSSH' {
    BeforeAll {
        function Start-Service {}
        function Restart-Service {}
        function Set-Service {}
        function Get-NetFirewallRule {}
        function icacls {}
    }

    BeforeEach {
        $script:icaclsCallCount = 0

        Mock Logs-To-Event
        Mock Get-Service -MockWith { return [PSCustomObject]@{ Name = 'sshd' } }
        Mock Start-Service
        Mock Test-Path -MockWith { return $true }
        Mock Add-Content
        Mock Restart-Service
        Mock Set-Service
        Mock Get-NetFirewallRule -MockWith { return [PSCustomObject]@{ Name = 'OpenSSH-Server-In-TCP' } }
        Mock icacls -MockWith {
            $script:icaclsCallCount++
            $global:LASTEXITCODE = 0
        }
    }

    It 'configures the authorized keys permissions when icacls succeeds' {
        { Install-OpenSSH -SSHKeys @('ssh-rsa test') } | Should -Not -Throw

        $script:icaclsCallCount | Should -Be 4
    }

    It 'throws when Authenticated Users permissions cannot be removed' {
        Mock icacls -MockWith {
            $script:icaclsCallCount++
            $global:LASTEXITCODE = 5
        }

        { Install-OpenSSH -SSHKeys @('ssh-rsa test') } | Should -Throw '*remove Authenticated Users permissions*exit code 5*'

        $script:icaclsCallCount | Should -Be 1
        Assert-MockCalled -CommandName Restart-Service -Exactly -Times 0
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

Describe 'New-CsiProxyService' {
    BeforeEach {
        $global:KubeDir = 'c:\k'
        $script:scExeCallCount = 0

        Mock Logs-To-Event
        Mock DownloadFileOverHttp
        Mock tar { $global:LASTEXITCODE = 0 }
        Mock cp
        Mock del
        Mock New-TemporaryDirectory -MockWith { return 'c:\temp\csiproxy' }
        Mock Invoke-Nssm
        Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 0 }
    }

    Context 'when csi-proxy service does not exist' {
        BeforeEach {
            Mock Get-Service -MockWith { return $null }
        }

        It 'does not call sc.exe and still installs the service' {
            New-CsiProxyService -CsiProxyPackageUrl 'https://example.com/csiproxy.tar.gz' -KubeDir 'c:\k'

            $script:scExeCallCount | Should -Be 0
            Assert-MockCalled -CommandName 'Invoke-Nssm' -Exactly -Times 1 -ParameterFilter { $KubeDir -eq 'c:\k' -and $NssmArguments[0] -eq 'install' -and $NssmArguments[1] -eq 'csi-proxy' }
        }
    }

    Context 'when csi-proxy service already exists' {
        BeforeEach {
            $script:getServiceCallCount = 0
            $mockExistingSvc = [PSCustomObject]@{Name = 'csi-proxy'; Status = 'Stopped'}
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -eq 1) {
                    return $mockExistingSvc
                }
                return $null
            }
        }

        It 'calls sc.exe delete to remove the existing service before install' {
            New-CsiProxyService -CsiProxyPackageUrl 'https://example.com/csiproxy.tar.gz' -KubeDir 'c:\k'

            $script:scExeCallCount | Should -Be 1
        }

        It 'throws when sc.exe delete fails' {
            Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 1 }

            { New-CsiProxyService -CsiProxyPackageUrl 'https://example.com/csiproxy.tar.gz' -KubeDir 'c:\k' } | Should -Throw '*exit code 1*'
        }
    }
}

Describe 'New-HostsConfigService' {
    BeforeEach {
        $global:KubeDir = 'c:\k'
        $script:scExeCallCount = 0

        Mock Logs-To-Event
        Mock Invoke-Nssm
        Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 0 }
    }

    Context 'when hosts-config-agent service does not exist' {
        BeforeEach {
            Mock Get-Service -MockWith { return $null }
        }

        It 'does not call sc.exe and still installs the service' {
            New-HostsConfigService

            $script:scExeCallCount | Should -Be 0
        }
    }

    Context 'when hosts-config-agent service already exists' {
        BeforeEach {
            $script:getServiceCallCount = 0
            $mockExistingSvc = [PSCustomObject]@{Name = 'hosts-config-agent'; Status = 'Stopped'}
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -eq 1) {
                    return $mockExistingSvc
                }
                return $null
            }
        }

        It 'calls sc.exe delete to remove the existing service before install' {
            New-HostsConfigService

            $script:scExeCallCount | Should -Be 1
        }

        It 'throws when sc.exe delete fails' {
            Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 1 }

            { New-HostsConfigService } | Should -Throw '*exit code 1*'
        }
    }
}
