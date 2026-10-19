BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSScriptRoot\networkisolatedclusterfunc.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Write-Log must not be mocked because mock output can become part of a function's return value.
    function Write-Log {}

    # Always shadow these host commands so tests cannot pass, fail, or modify the machine
    # based on its installed services, service configuration, or registry state.
    function Get-Service {}
    function sc.exe {}
    function reg.exe {}

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

Describe 'Update-ServiceFailureActions' {
    BeforeEach {
        $script:scExeCallCount = 0
        $script:failedScExeCall = 0

        Mock Logs-To-Event
        Mock sc.exe -MockWith {
            $script:scExeCallCount++
            if ($script:scExeCallCount -eq $script:failedScExeCall) {
                $global:LASTEXITCODE = 1
                return
            }

            $global:LASTEXITCODE = 0
        }
    }

    It 'configures failure actions for all services when sc.exe succeeds' {
        { Update-ServiceFailureActions } | Should -Not -Throw

        $script:scExeCallCount | Should -Be 3
    }

    It 'throws when configuring <Service>' -TestCases @(
        @{ FailedCall = 1; Service = 'kubelet' }
        @{ FailedCall = 2; Service = 'kubeproxy' }
        @{ FailedCall = 3; Service = 'containerd' }
    ) {
        param($FailedCall, $Service)
        $script:failedScExeCall = $FailedCall

        { Update-ServiceFailureActions } | Should -Throw "*$Service*exit code 1*"

        $script:scExeCallCount | Should -Be $FailedCall
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
        Mock Set-ExitCode
        Mock Test-GmsaPluginRegistry -MockWith { return $false }
        Mock Repair-GmsaPluginRegistryPermissions
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 0
            return ""
        }
    }

    It 'continues when reg.exe succeeds and the required registry state is valid' {
        Mock Test-GmsaPluginRegistry -MockWith { return $true }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 0
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'repairs and retries when reg.exe succeeds but the registry state is invalid' {
        $script:validationCallCount = 0
        Mock Test-GmsaPluginRegistry -MockWith {
            $script:validationCallCount++
            return $script:validationCallCount -eq 2
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 1
        Assert-MockCalled -CommandName 'reg.exe' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'fails when reg.exe succeeds but the registry state remains invalid after retry' {
        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 1
        Assert-MockCalled -CommandName 'reg.exe' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter {
            $ExitCode -eq $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES `
                -and $ErrorMessage -match 'returned exit code 0' `
                -and $ErrorMessage -match 'retry returned exit code 0'
        }
    }

    It 'continues when reg.exe fails but the required registry state is valid' {
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "The operation completed with errors."
        }
        Mock Test-GmsaPluginRegistry -MockWith { return $true }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'
        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 0
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'repairs registry permissions and retries when the registry state is invalid' {
        $script:regExeCallCount = 0
        $script:validationCallCount = 0
        Mock reg.exe -MockWith {
            $script:regExeCallCount++
            if ($script:regExeCallCount -eq 1) {
                $global:LASTEXITCODE = 1
                return "Access is denied."
            }

            $global:LASTEXITCODE = 0
            return ""
        }
        Mock Test-GmsaPluginRegistry -MockWith {
            $script:validationCallCount++
            return $script:validationCallCount -eq 2
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 1
        Assert-MockCalled -CommandName 'reg.exe' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'continues when the retry fails but the repaired registry state is valid' {
        $script:validationCallCount = 0
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "Access is denied."
        }
        Mock Test-GmsaPluginRegistry -MockWith {
            $script:validationCallCount++
            return $script:validationCallCount -eq 2
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
    }

    It 'fails when the retry fails and the repaired registry state remains invalid' {
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "Access is denied."
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'Repair-GmsaPluginRegistryPermissions' -Exactly -Times 1
        Assert-MockCalled -CommandName 'Test-GmsaPluginRegistry' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter {
            $ExitCode -eq $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES `
                -and $ErrorMessage -match 'exit code 1' `
                -and $ErrorMessage -match 'retry returned exit code 1' `
                -and $ErrorMessage -match 'Access is denied'
        }
    }

    It 'retries and reports an unexpected permission repair failure' {
        Mock reg.exe -MockWith {
            $global:LASTEXITCODE = 1
            return "Access is denied."
        }
        Mock Repair-GmsaPluginRegistryPermissions -MockWith {
            throw "permission repair failed"
        }

        Import-GmsaPluginRegistry -RegistryFilePath 'c:\temp\registerplugin.reg'

        Assert-MockCalled -CommandName 'reg.exe' -Exactly -Times 2
        Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter {
            $ExitCode -eq $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES `
                -and $ErrorMessage -match 'permission repair failed'
        }
    }
}

Describe 'Repair-GmsaPluginRegistryPermissions' {
    BeforeEach {
        Mock Set-GmsaPluginRegistryKeyPermission
    }

    It 'repairs only the GMSA plugin registry subtrees' {
        Repair-GmsaPluginRegistryPermissions

        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 4
        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 1 -ParameterFilter {
            $RegistryKeyPath -eq 'SOFTWARE\Classes\Interface\{6ECDA518-2010-4437-8BC3-46E752B7B172}'
        }
        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 1 -ParameterFilter {
            $RegistryKeyPath -eq 'SOFTWARE\Classes\AppID\{557110E1-88BC-4583-8281-6AAC6F708584}'
        }
        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 1 -ParameterFilter {
            $RegistryKeyPath -eq 'SOFTWARE\Classes\CLSID\{CCC2A336-D7F3-4818-A213-272B7924213E}'
        }
        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 1 -ParameterFilter {
            $RegistryKeyPath -eq 'SYSTEM\CurrentControlSet\Control\CCG\COMClasses'
        }
    }

    It 'continues repairing the remaining keys when one repair fails' {
        Mock Set-GmsaPluginRegistryKeyPermission -MockWith {
            param($RegistryKeyPath)
            if ($RegistryKeyPath -like 'SOFTWARE\Classes\Interface\*') {
                throw "Access is denied."
            }
        }

        $repairFailures = @(Repair-GmsaPluginRegistryPermissions)

        Assert-MockCalled -CommandName 'Set-GmsaPluginRegistryKeyPermission' -Exactly -Times 4
        $repairFailures | Should -HaveCount 1
        $repairFailures[0] | Should -Match 'SOFTWARE\\Classes\\Interface'
        $repairFailures[0] | Should -Match 'Access is denied'
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
    BeforeAll {
        $script:savedCacheDir = $global:CacheDir
        $script:savedArchitecture = $env:PROCESSOR_ARCHITECTURE
        $script:savedNativeArchitecture = $env:PROCESSOR_ARCHITEW6432
        $script:savedPackageDownloadFqdn = $global:PackageDownloadFqdn
        $script:downloadCredentialProvider = ${function:DownloadFileOverHttp}
        function Write-Log {
            param([string]$Message)
            $script:credentialProviderLogs.Add($Message)
        }
    }
    AfterAll {
        $global:CacheDir = $script:savedCacheDir
        $env:PROCESSOR_ARCHITECTURE = $script:savedArchitecture
        $env:PROCESSOR_ARCHITEW6432 = $script:savedNativeArchitecture
        $global:PackageDownloadFqdn = $script:savedPackageDownloadFqdn
    }

    BeforeEach {
        $script:credentialProviderLogs = [System.Collections.Generic.List[string]]::new()
        $script:expectedMetadataReads = 0
        $script:expectedDalecDownloads = 0
        $env:PROCESSOR_ARCHITECTURE = 'AMD64'
        $env:PROCESSOR_ARCHITEW6432 = $null
        $global:CacheDir = Join-Path $TestDrive ([guid]::NewGuid().ToString())
        $script:dalecCacheDir = Join-Path $global:CacheDir 'azure-acr-credential-provider'
        New-Item -ItemType Directory -Path $script:dalecCacheDir -Force | Out-Null
        $script:cachedDalecFile = (New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.6-1_amd64.zip')).FullName
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
        Mock Invoke-RestMethod -MockWith { throw 'Unexpected Dalec download' }
        Mock Test-Path -MockWith { $false } -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
        Mock Get-Content -MockWith { throw 'Unexpected metadata read' }
        Mock Expand-DalecCredentialProviderPackage
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
        Assert-MockCalled Invoke-RestMethod -Times $script:expectedDalecDownloads -Exactly
        Assert-MockCalled Get-Content -Times $script:expectedMetadataReads -Exactly
        Assert-MockCalled del -Times 0 -ParameterFilter { $Path -like "$global:CacheDir*" }
        Assert-MockCalled cp -Times 0 -ParameterFilter { $Destination -like "$global:CacheDir*" }
        $global:BootstrapProfileContainerRegistryServer = $null
    }

    Context 'when a Dalec download is present in metadata' {
        BeforeEach {
            $global:BootstrapProfileContainerRegistryServer = ''
            $global:KubeBinariesVersion = '1.34.0'
            $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.34.0/binaries/azure-acr-credential-provider-windows-amd64-v1.34.0.tar.gz'
            $global:PackageDownloadFqdn = 'packages.aks.azure.com'
            $script:expectedMetadataReads = 1
            $script:expectedDalecDownloads = 1
            Mock Test-Path -MockWith { $true } -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
            Mock Get-Content -MockWith {
                @'
{"Packages":[{"name":"azure-acr-credential-provider-pmc","downloadURIs":{"windows":{"default":{
    "versionsV2":[{"latestVersion":"1.34.19-1"}],
    "downloadURL":"https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split('-')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip"
}}}}]}
'@
            }
            Mock Invoke-RestMethod
            Mock Resolve-PackagesDownloadFqdn -MockWith { throw 'Unexpected public endpoint probe' }
        }

        It 'uses an older matching cached ZIP without reading or downloading newer metadata' {
            $file = New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.34.3-1_amd64.zip')
            $script:expectedMetadataReads = 0
            $script:expectedDalecDownloads = 0
            Mock Get-Content -MockWith { throw 'A cache hit must not initialize metadata' }

            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 1 -ParameterFilter { $Path -eq $file.FullName }
            Assert-MockCalled DownloadFileOverHttp -Times 0
            Assert-MockCalled Test-Path -Times 0 -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
        }

        It 'downloads the pinned Dalec ZIP into temporary storage after a cache miss' {
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Invoke-RestMethod -Times 1 -Exactly -ParameterFilter {
                $Uri -eq 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.34.19/windows/amd64/azure-acr-credential-provider_1.34.19-1_amd64.zip' -and
                $OutFile -eq 'C:\temp\credprovider\credentialprovider.zip'
            }
            Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 1 -ParameterFilter {
                $Path -eq 'C:\temp\credprovider\credentialprovider.zip' -and $DestinationPath -eq 'C:\temp\credprovider'
            }
            Assert-MockCalled DownloadFileOverHttp -Times 0
            $script:credentialProviderLogs | Should -Contain 'Using dalec credential provider v1.34.19-1'
            ($script:credentialProviderLogs -join "`n") | Should -Not -Match 'Failed to install dalec credential provider'
        }

        It 'still checks metadata when the entire cache directory is missing' {
            $global:CacheDir = Join-Path $TestDrive 'missing-cache'
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 1
            Assert-MockCalled DownloadFileOverHttp -Times 0
        }

        It 'downloads from metadata when the RP URL is empty' {
            $global:CredentialProviderURL = ''
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 1
            Assert-MockCalled DownloadFileOverHttp -Times 0
        }

        It 'preserves sovereign and public mirror mapping for <downloadHost>' -TestCases @(
            @{ rpHost = 'packages.aks.azure.us'; preferredHost = 'packages.aks.azure.com'; downloadHost = 'packages.aks.azure.us' },
            @{ rpHost = 'packages.aks.azure.com'; preferredHost = 'acs-mirror.azureedge.net'; downloadHost = 'acs-mirror.azureedge.net' }
        ) {
            param($rpHost, $preferredHost, $downloadHost)
            $global:CredentialProviderURL = $global:CredentialProviderURL.Replace('packages.aks.azure.com', $rpHost)
            $global:PackageDownloadFqdn = $preferredHost
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Invoke-RestMethod -Times 1 -Exactly -ParameterFilter {
                $Uri -eq "https://$downloadHost/dalec-packages/azure-acr-credential-provider/1.34.19/windows/amd64/azure-acr-credential-provider_1.34.19-1_amd64.zip"
            }
            Assert-MockCalled Resolve-PackagesDownloadFqdn -Times 0
            Assert-MockCalled DownloadFileOverHttp -Times 0
        }

        It 'retries failed Dalec downloads before falling back to the RP path' {
            $script:expectedDalecDownloads = 5
            Mock Invoke-RestMethod -MockWith { throw 'Dalec download failed' }
            Mock Start-Sleep

            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled DownloadFileOverHttp -Times 1 -Exactly -ParameterFilter { $Url -eq $global:CredentialProviderURL }
            Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 0
            Assert-MockCalled tar -Times 1
            ($script:credentialProviderLogs -join "`n") | Should -Match 'Failed to install dalec credential provider, falling back to RP URL.'
        }

        It 'attempts the metadata download before reusing an available RP archive' {
            $script:expectedDalecDownloads = 0
            $script:dalecAttempted = $false
            $legacyFile = New-Item -ItemType File -Path (Join-Path $global:CacheDir 'azure-acr-credential-provider-windows-amd64-v1.34.0.tar.gz')
            Mock Get-DalecCredentialProviderPackage -MockWith {
                $script:dalecAttempted = $true
                throw 'Dalec download failed'
            }
            Mock DownloadFileOverHttp -MockWith {
                param($Url, $DestinationPath, $ExitCode)
                $script:dalecAttempted | Should -Be $true
                & $script:downloadCredentialProvider -Url $Url -DestinationPath $DestinationPath -ExitCode $ExitCode
            }
            Mock Copy-Item

            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled Get-DalecCredentialProviderPackage -Times 1 -Exactly
            Assert-MockCalled Copy-Item -Times 1 -Exactly -ParameterFilter {
                $Path -eq $legacyFile.FullName -and $Destination -eq 'C:\temp\credprovider\credentialprovider.tar.gz'
            }
        }

        It 'falls back to the RP path if the downloaded Dalec ZIP cannot be extracted' {
            Mock Expand-DalecCredentialProviderPackage -MockWith { throw 'Invalid downloaded ZIP' }
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled DownloadFileOverHttp -Times 1 -Exactly -ParameterFilter { $Url -eq $global:CredentialProviderURL }
            Assert-MockCalled tar -Times 1
        }

        It 'fails with the download error when no RP URL is available' {
            $global:CredentialProviderURL = ''
            $script:expectedDalecDownloads = 0
            Mock Get-DalecCredentialProviderPackage -MockWith { throw 'Dalec download failed' }
            { Install-CredentialProvider -KubeDir 'c:\k' } | Should -Throw '*Dalec download failed*'
            Assert-MockCalled DownloadFileOverHttp -Times 0
        }

        It 'uses the RP path when metadata does not list the requested minor' {
            $global:KubeBinariesVersion = '1.35.0'
            $script:expectedDalecDownloads = 0
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled DownloadFileOverHttp -Times 1 -Exactly -ParameterFilter { $Url -eq $global:CredentialProviderURL }
        }

        It 'uses the RP path when metadata is malformed' {
            $script:expectedDalecDownloads = 0
            Mock Get-Content -MockWith { '{malformed' }
            Install-CredentialProvider -KubeDir 'c:\k'
            Assert-MockCalled DownloadFileOverHttp -Times 1 -Exactly -ParameterFilter { $Url -eq $global:CredentialProviderURL }
            ($script:credentialProviderLogs -join "`n") | Should -Match 'Invalid Dalec credential provider metadata.*falling back to RP URL'
        }
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

    It 'extracts the cached Dalec ZIP directly into a temporary directory with a stock legacy RP URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 1 -Exactly -ParameterFilter {
            $Path -eq $script:cachedDalecFile -and $DestinationPath -eq 'C:\temp\credprovider'
        }
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 0
        Assert-MockCalled -CommandName 'del' -Times 1 -Exactly -ParameterFilter { $Path -eq 'C:\temp\credprovider' }
        $script:credentialProviderLogs | Should -Contain 'Using dalec credential provider v1.33.6-1'
        ($script:credentialProviderLogs -join "`n") | Should -Not -Match 'Failed to install dalec credential provider, falling back to RP URL.'
    }

    It 'uses dalec path for k8s >= 1.33 when the RP URL is empty' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = ""

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 1 -ParameterFilter { $Path -eq $script:cachedDalecFile }
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 0
    }

    It 'uses cached Dalec or the sovereign RP URL with cache hit <cacheHit>' -TestCases @(
        @{ cacheHit = $true },
        @{ cacheHit = $false }
    ) {
        param($cacheHit)
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        if (-not $cacheHit) {
            $global:KubeBinariesVersion = "1.34.0"
        }
        Mock Resolve-PackagesDownloadFqdn -MockWith { throw 'Sovereign downloads must not probe public endpoints' }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        if ($cacheHit) {
            Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 1 -ParameterFilter {
                $Path -eq $script:cachedDalecFile
            }
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
            Assert-MockCalled -CommandName 'tar' -Times 0
        } else {
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -Exactly -ParameterFilter {
                $Url -eq $global:CredentialProviderURL
            }
            Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 0
            Assert-MockCalled -CommandName 'tar' -Times 1
        }
        Assert-MockCalled -CommandName 'Resolve-PackagesDownloadFqdn' -Times 0
    }

    It 'uses dalec path for k8s >= 1.33 with the stock legacy mirror URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://acs-mirror.azureedge.net/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 1
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 0
    }

    It 'falls back to RP legacy URL when the cache directory is missing' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        $global:CacheDir = Join-Path $TestDrive 'missing-cache'

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
        }
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 1
        ($script:credentialProviderLogs -join "`n") | Should -Match 'cache directory .* is missing; checking baked metadata'
        ($script:credentialProviderLogs -join "`n") | Should -Match 'metadata not found.*falling back to RP URL'
    }

    It 'reuses a legacy archive through the unchanged shared helper on a Dalec cache miss' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.34.0"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.34.0/binaries/azure-acr-credential-provider-windows-amd64-v1.34.0.tar.gz'
        $legacyFile = New-Item -ItemType File -Path (Join-Path $global:CacheDir 'azure-acr-credential-provider-windows-amd64-v1.34.0.tar.gz')
        Mock DownloadFileOverHttp -MockWith {
            param($Url, $DestinationPath, $ExitCode)
            & $script:downloadCredentialProvider -Url $Url -DestinationPath $DestinationPath -ExitCode $ExitCode
        }
        Mock Copy-Item

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled Copy-Item -Times 1 -Exactly -ParameterFilter {
            $Path -eq $legacyFile.FullName -and $Destination -eq 'C:\temp\credprovider\credentialprovider.tar.gz'
        }
        Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 0
        Assert-MockCalled tar -Times 1
    }

    It 'falls back to RP legacy URL when dalec extraction fails' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.us/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Expand-DalecCredentialProviderPackage -MockWith {
            throw 'dalec extraction failed'
        }

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
        }
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 1
        Assert-MockCalled -CommandName 'tar' -Times 1
        ($script:credentialProviderLogs -join "`n") | Should -Match 'Failed to install dalec credential provider, falling back to RP URL. Error: dalec extraction failed'
    }

    It 'fails with the extraction error when a cached archive is unusable and the RP URL is empty' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = ""
        Mock Expand-DalecCredentialProviderPackage -MockWith { throw 'dalec extraction failed' }

        { Install-CredentialProvider -KubeDir 'c:\k' } | Should -Throw '*dalec extraction failed*'
        Assert-MockCalled DownloadFileOverHttp -Times 0
        Assert-MockCalled tar -Times 0
        Assert-MockCalled Set-ExitCode -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER }
    }

    It 'surfaces a cache access error instead of silently falling back to the RP URL' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'
        Mock Get-ChildItem -MockWith { Write-Error 'Cache access denied' }

        { Install-CredentialProvider -KubeDir 'c:\k' } | Should -Throw '*Failed to inspect Dalec credential provider cache*Cache access denied*'
        Assert-MockCalled DownloadFileOverHttp -Times 0
        Assert-MockCalled Expand-DalecCredentialProviderPackage -Times 0
        Assert-MockCalled Set-ExitCode -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER }
    }

    It 'fails clearly when the RP URL, matching cached Dalec ZIP and metadata download are unavailable' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.35.7"
        $global:CredentialProviderURL = ""

        { Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix '' } | Should -Throw '*No credential provider source*neither a cached Dalec package nor a matching download in baked metadata*'
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 0
        Assert-MockCalled -CommandName 'Set-ExitCode' -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER }
    }

    It 'falls back to the RP URL when no cached Dalec ZIP matches the Kubernetes minor' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.35.0"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.35.0/binaries/azure-acr-credential-provider-windows-amd64-v1.35.0.tar.gz'

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
        }
        Assert-MockCalled -CommandName 'tar' -Times 1
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 0
        ($script:credentialProviderLogs -join "`n") | Should -Match 'No cached Dalec credential provider for Kubernetes 1.35.*checking baked metadata'
        ($script:credentialProviderLogs -join "`n") | Should -Match 'metadata not found.*falling back to RP URL'
    }

    It 'uses legacy tar path for k8s 1.32 even with a matching cached Dalec ZIP' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.32.8"
        $global:CredentialProviderURL = 'https://packages.aks.azure.com/cloud-provider-azure/v1.32.8/binaries/azure-acr-credential-provider-windows-amd64-v1.32.8.tar.gz'

        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.32.19-1_amd64.zip') | Out-Null

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'tar' -Times 1
        Assert-MockCalled -CommandName 'Expand-DalecCredentialProviderPackage' -Times 0
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq 'https://packages.aks.azure.com/cloud-provider-azure/v1.32.8/binaries/azure-acr-credential-provider-windows-amd64-v1.32.8.tar.gz'
        }
        ($script:credentialProviderLogs -join "`n") | Should -Not -Match 'Using dalec credential provider|Failed to install dalec credential provider'
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

    It 'honors a custom mirror URL that uses the stock legacy path' {
        $global:BootstrapProfileContainerRegistryServer = ""
        $global:KubeBinariesVersion = "1.33.3"
        $global:CredentialProviderURL = 'https://custom-mirror.example.com/cloud-provider-azure/v1.33.3/binaries/azure-acr-credential-provider-windows-amd64-v1.33.3.tar.gz'

        Mock Resolve-DalecCredentialProviderPackage

        Install-CredentialProvider -KubeDir 'c:\k' -CustomCloudContainerRegistryDNSSuffix ''
        Assert-MockCalled -CommandName 'Resolve-DalecCredentialProviderPackage' -Times 0
        Assert-MockCalled -CommandName 'tar' -Times 1
        Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Times 1 -ParameterFilter {
            $Url -eq $global:CredentialProviderURL
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

Describe 'New-CsiProxyService' {
    BeforeEach {
        $global:KubeDir = 'c:\k'
        $script:scExeCallCount = 0

        Mock Logs-To-Event
        Mock DownloadFileOverHttp
        Mock tar { $global:LASTEXITCODE = 0 }
        Mock Copy-Item
        Mock Move-Item
        Mock del
        Mock New-TemporaryDirectory -MockWith { return 'c:\temp\csiproxy' }
        Mock Invoke-Nssm
        Mock Start-Sleep
        Mock sc.exe -MockWith {
            $script:scExeCallCount++
            $global:LASTEXITCODE = if ($args[0] -eq 'query') { 1060 } else { 0 }
        }
    }

    It 'does not remove the existing service when staging the replacement binary fails' {
        Mock Copy-Item -MockWith { throw 'The destination disk is full.' }
        Mock Remove-ServiceIfExists

        { New-CsiProxyService -CsiProxyPackageUrl 'https://example.com/csiproxy.tar.gz' -KubeDir 'c:\k' } | Should -Throw '*destination disk is full*'

        Assert-MockCalled -CommandName 'Copy-Item' -Exactly -Times 1 -ParameterFilter {
            $Path -eq 'c:\temp\csiproxy\bin\csi-proxy.exe' `
                -and $Destination -eq 'c:\k\csi-proxy.exe.new' `
                -and $ErrorAction -eq 'Stop'
        }
        Assert-MockCalled -CommandName 'Remove-ServiceIfExists' -Exactly -Times 0
        Assert-MockCalled -CommandName 'Move-Item' -Exactly -Times 0
        Assert-MockCalled -CommandName 'Invoke-Nssm' -Exactly -Times 0
    }

    Context 'when csi-proxy service does not exist' {
        BeforeEach {
            Mock Get-Service -MockWith { return $null }
        }

        It 'confirms the service is absent and still installs it' {
            New-CsiProxyService -CsiProxyPackageUrl 'https://example.com/csiproxy.tar.gz' -KubeDir 'c:\k'

            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'query' }
            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 0 -ParameterFilter { $args[0] -eq 'delete' }
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

            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'delete' }
            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'query' }
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
        Mock Start-Sleep
        Mock sc.exe -MockWith {
            $script:scExeCallCount++
            $global:LASTEXITCODE = if ($args[0] -eq 'query') { 1060 } else { 0 }
        }
    }

    Context 'when hosts-config-agent service does not exist' {
        BeforeEach {
            Mock Get-Service -MockWith { return $null }
        }

        It 'confirms the service is absent and still installs it' {
            New-HostsConfigService

            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'query' }
            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 0 -ParameterFilter { $args[0] -eq 'delete' }
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

            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'delete' }
            Assert-MockCalled -CommandName 'sc.exe' -Exactly -Times 1 -ParameterFilter { $args[0] -eq 'query' }
        }

        It 'throws when sc.exe delete fails' {
            Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 1 }

            { New-HostsConfigService } | Should -Throw '*exit code 1*'
        }
    }
}

Describe 'Resolve-DalecCredentialProviderPackageFromMetadata' {
    BeforeEach {
        $script:metadata = @{
            Packages = @(@{
                name = 'azure-acr-credential-provider-pmc'
                downloadURIs = @{
                    windows = @{
                        default = @{
                            versionsV2 = @(@{ latestVersion = '1.33.19-1' })
                            downloadURL = 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/$($version.Split(''-'')[0])/windows/amd64/azure-acr-credential-provider_${version}_amd64.zip'
                        }
                    }
                }
            })
        }
        Mock Test-Path -MockWith { $true } -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
        Mock Get-Content -MockWith { $script:metadata | ConvertTo-Json -Depth 10 -Compress }
    }

    It 'resolves the pinned version and URL from older PMC-hosted Windows metadata' {
        $result = Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3'
        $result.Version | Should -Be '1.33.19-1'
        $result.Url | Should -Be 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.19/windows/amd64/azure-acr-credential-provider_1.33.19-1_amd64.zip'
        $result.CachedFile | Should -Be $null
        Assert-MockCalled Get-Content -Times 1 -Exactly -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
    }

    It 'orders numeric patches and revisions and excludes other minors, prereleases and malformed versions' {
        $script:metadata.Packages[0].downloadURIs.windows.default.versionsV2 = @(
            foreach ($version in @('1.33.9-99', '1.33.19-10', '1.33.19-2', '1.34.99-1', '2.33.99-1', '1.33.99-rc.1-1', '1.33.99-01', '1.33.99', 'invalid')) {
                @{ latestVersion = $version }
            }
        )
        $result = Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.0'
        $result.Version | Should -Be '1.33.19-10'
        $result.Url | Should -Be 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.19/windows/amd64/azure-acr-credential-provider_1.33.19-10_amd64.zip'
    }

    It 'supports consolidated Windows Dalec metadata alongside Linux-only PMC metadata' {
        $script:metadata.Packages[0].name = 'windows credential provider dalec'
        $script:metadata.Packages[0].downloadURIs.windows.default.versionsV2[0].previousLatestVersion = '1.33.6-1'
        $script:metadata.Packages += @{
            name = 'azure-acr-credential-provider-pmc'
            downloadURIs = @{
                ubuntu = @{ r2204 = @{ versionsV2 = @(@{ latestVersion = '1.33.99' }) } }
            }
        }
        $result = Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3'
        $result.Version | Should -Be '1.33.19-1'
        $result.Url | Should -Be 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.19/windows/amd64/azure-acr-credential-provider_1.33.19-1_amd64.zip'
    }

    It 'retains PMC Windows precedence when older metadata contains both Dalec blocks' {
        $script:metadata.Packages += @{
            name = 'windows credential provider dalec'
            downloadURIs = @{
                windows = @{
                    default = @{
                        versionsV2 = @(@{ latestVersion = '1.33.6-1' })
                        downloadURL = 'https://example.invalid/azure-acr-credential-provider_${version}_amd64.zip'
                    }
                }
            }
        }
        $result = Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3'
        $result.Version | Should -Be '1.33.19-1'
        $result.Url | Should -Be 'https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/1.33.19/windows/amd64/azure-acr-credential-provider_1.33.19-1_amd64.zip'
    }

    It 'does not use matching versions from the legacy component <name>' -TestCases @(
        @{ name = 'windows credential provider' },
        @{ name = 'azure-acr-credential-provider' }
    ) {
        param($name)
        $script:metadata.Packages[0].name = $name
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
    }

    It 'returns no download for a missing minor, package or URL' {
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.34.0' | Should -Be $null
        $script:metadata.Packages[0].downloadURIs.windows.default.downloadURL = ''
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
        $script:metadata.Packages = @()
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
    }

    It 'rejects an unsupported or unresolved download template <template>' -TestCases @(
        @{ template = 'https://example.invalid/provider_${version}_arm64.zip' },
        @{ template = 'https://example.invalid/${unknown}/azure-acr-credential-provider_${version}_amd64.zip' },
        @{ template = 'http://example.invalid/azure-acr-credential-provider_${version}_amd64.zip' },
        @{ template = 'not a URL' },
        @{ template = 123 }
    ) {
        param($template)
        $script:metadata.Packages[0].downloadURIs.windows.default.downloadURL = $template
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
    }

    It 'returns no download when metadata is absent without trying to read it' {
        Mock Test-Path -MockWith { $false } -ParameterFilter { $LiteralPath -eq 'c:\k\components.json' }
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
        Assert-MockCalled Get-Content -Times 0
    }

    It 'returns no download for malformed JSON' {
        Mock Get-Content -MockWith { '{malformed' }
        Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' | Should -Be $null
    }

    It 'does not hide unexpected metadata access errors' {
        Mock Get-Content -MockWith { Write-Error 'Metadata access denied' }
        { Resolve-DalecCredentialProviderPackageFromMetadata -KubeVersion '1.33.3' } | Should -Throw '*Metadata access denied*'
    }
}

Describe 'Expand-DalecCredentialProviderPackage' {
    It 'extracts into a separate directory without altering the cached archive' {
        $sourceDir = New-Item -ItemType Directory -Path (Join-Path $TestDrive 'source')
        $cacheDir = New-Item -ItemType Directory -Path (Join-Path $TestDrive 'cache')
        $sourceFile = Join-Path $sourceDir.FullName 'azure-acr-credential-provider.exe'
        [IO.File]::WriteAllText($sourceFile, 'test credential provider')
        $archive = Join-Path $cacheDir.FullName 'azure-acr-credential-provider_1.33.19-1_amd64.zip'
        Compress-Archive -Path $sourceFile -DestinationPath $archive
        $originalHash = (Get-FileHash -LiteralPath $archive).Hash
        $destination = Join-Path $TestDrive 'extract'

        Expand-DalecCredentialProviderPackage -Path $archive -DestinationPath $destination

        [IO.File]::ReadAllText((Join-Path $destination 'azure-acr-credential-provider.exe')) | Should -Be 'test credential provider'
        (Get-FileHash -LiteralPath $archive).Hash | Should -Be $originalHash
        @(Get-ChildItem -LiteralPath $cacheDir.FullName).Count | Should -Be 1
    }

    It 'throws when the cached archive cannot be extracted' {
        $archive = Join-Path $TestDrive 'invalid.zip'
        [IO.File]::WriteAllText($archive, 'invalid ZIP')
        { Expand-DalecCredentialProviderPackage -Path $archive -DestinationPath (Join-Path $TestDrive 'extract-invalid') } | Should -Throw
        Test-Path -LiteralPath $archive | Should -Be $true
    }
}

Describe 'Resolve-DalecCredentialProviderPackage' {
    BeforeAll {
        $script:savedCacheDir = $global:CacheDir
        $script:savedArchitecture = $env:PROCESSOR_ARCHITECTURE
        $script:savedNativeArchitecture = $env:PROCESSOR_ARCHITEW6432
    }
    AfterAll {
        $global:CacheDir = $script:savedCacheDir
        $env:PROCESSOR_ARCHITECTURE = $script:savedArchitecture
        $env:PROCESSOR_ARCHITEW6432 = $script:savedNativeArchitecture
    }

    BeforeEach {
        $env:PROCESSOR_ARCHITECTURE = 'AMD64'
        $env:PROCESSOR_ARCHITEW6432 = $null
        $global:CacheDir = Join-Path $TestDrive ([guid]::NewGuid().ToString())
        $script:dalecCacheDir = Join-Path $global:CacheDir 'azure-acr-credential-provider'
        New-Item -ItemType Directory -Path $script:dalecCacheDir -Force | Out-Null
        Mock Get-Content -MockWith { throw 'Resolver must not read components.json' }
        Mock ConvertFrom-Json -MockWith { throw 'Resolver must not parse components.json' }
        Mock Resolve-DalecCredentialProviderPackageFromMetadata -MockWith { $null }
    }
    AfterEach {
        Assert-MockCalled Get-Content -Times 0
        Assert-MockCalled ConvertFrom-Json -Times 0
    }

    It 'selects a cached ZIP without consulting <state> components.json' -TestCases @(
        @{ state = 'absent'; exists = $false; content = $null },
        @{ state = 'malformed'; exists = $true; content = '{malformed' }
    ) {
        param($state, $exists, $content)
        Mock Test-Path -MockWith { $exists } -ParameterFilter { $Path -eq 'c:\k\components.json' -or $LiteralPath -eq 'c:\k\components.json' }
        Mock Get-Content -MockWith { $content }
        $file = New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.19-1_amd64.zip')
        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.33.3'
        $result.Version | Should -Be '1.33.19-1'
        $result.CachedFile | Should -Be $file.FullName
        $result.ContainsKey('Url') | Should -Be $false
        Assert-MockCalled Test-Path -Times 0 -ParameterFilter { $Path -eq 'c:\k\components.json' -or $LiteralPath -eq 'c:\k\components.json' }
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 0
    }

    It 'selects only the requested major and minor from mixed cached packages for <kubeVersion>' -TestCases @(
        @{ kubeVersion = 'v1.33.3'; expected = '1.33.19-1' },
        @{ kubeVersion = '1.34.1'; expected = '1.34.9-2' }
    ) {
        param($kubeVersion, $expected)
        foreach ($version in @('1.32.19-1', '1.33.19-1', '1.34.9-2', '2.33.99-1')) {
            New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir "azure-acr-credential-provider_${version}_amd64.zip") | Out-Null
        }
        foreach ($architecture in @('arm64', 'x86', 'amd64extra')) {
            New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir "azure-acr-credential-provider_1.33.99-99_${architecture}.zip") | Out-Null
        }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion $kubeVersion
        $result.Version | Should -Be $expected
        $result.CachedFile | Should -Be (Join-Path $script:dalecCacheDir "azure-acr-credential-provider_${expected}_amd64.zip")
    }

    It 'orders patch and package revision numerically regardless of enumeration order <reverse>' -TestCases @(
        @{ reverse = $false },
        @{ reverse = $true }
    ) {
        param($reverse)
        $script:cachedFiles = @(
            foreach ($version in @('1.33.9-99', '1.33.19-2', '1.33.19-10')) {
                New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir "azure-acr-credential-provider_${version}_amd64.zip")
            }
        )
        # The oldest file is deliberately the highest numeric version.
        $script:cachedFiles[2].LastWriteTime = [datetime]'2000-01-01'
        if ($reverse) {
            [array]::Reverse($script:cachedFiles)
        }
        Mock Get-ChildItem -MockWith { $script:cachedFiles }

        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.0'
        $result.Version | Should -Be '1.33.19-10'
    }

    It 'ignores malformed archives, prereleases, unrelated packages and directories' {
        foreach ($filename in @(
            'azure-acr-credential-provider_1.33.99-rc.1-1_amd64.zip',
            'azure-acr-credential-provider_1.33.99-1_amd64.zip.backup',
            'azure-acr-credential-provider_1.33.99-1_amd64.tar.gz',
            'azure-acr-credential-provider_1.33.99_amd64.zip',
            'azure-acr-credential-provider_v1.33.99-1_amd64.zip',
            'azure-acr-credential-provider_1.33.99-01_amd64.zip',
            'azure-acr-credential-provider_1.33.099-1_amd64.zip',
            'azure-acr-credential-provider_1.33.99.0-1_amd64.zip',
            'azure-acr-credential-provider_1.33.999999999999999999999-1_amd64.zip',
            'azure-acr-credential-provider_1.33.99-999999999999999999999_amd64.zip',
            'unrelated-azure-acr-credential-provider_1.33.99-1_amd64.zip',
            'unrelated_1.33.99-1_amd64.zip'
        )) {
            New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir $filename) | Out-Null
        }
        $directory = New-Item -ItemType Directory -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.99-1_amd64.zip')
        New-Item -ItemType File -Path (Join-Path $directory.FullName 'azure-acr-credential-provider_1.33.99-1_amd64.zip') | Out-Null
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
    }

    It 'returns no package for unsupported node architecture <architecture>' -TestCases @(
        @{ architecture = 'ARM64' },
        @{ architecture = 'x86' },
        @{ architecture = '' }
    ) {
        param($architecture)
        $env:PROCESSOR_ARCHITECTURE = $architecture
        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.19-1_amd64.zip') | Out-Null
        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.19-1_arm64.zip') | Out-Null
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 0
    }

    It 'uses the native AMD64 architecture when running under WOW64' {
        $env:PROCESSOR_ARCHITECTURE = 'x86'
        $env:PROCESSOR_ARCHITEW6432 = 'AMD64'
        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.33.19-1_amd64.zip') | Out-Null
        $result = Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3'
        $result.Version | Should -Be '1.33.19-1'
    }

    It 'returns no package below the gate even when a matching Dalec ZIP exists' {
        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.32.19-1_amd64.zip') | Out-Null
        Mock Get-ChildItem -MockWith { throw 'Must not enumerate cache below the gate' }
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'v1.32.8' | Should -Be $null
        Assert-MockCalled Get-ChildItem -Times 0
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 0
    }

    It 'returns no package for malformed Kubernetes versions' {
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion 'invalid-version' | Should -Be $null
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 0
    }

    It 'returns no package for an empty cache or an unmatched minor' {
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
        New-Item -ItemType File -Path (Join-Path $script:dalecCacheDir 'azure-acr-credential-provider_1.34.19-1_amd64.zip') | Out-Null
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 2 -Exactly -ParameterFilter { $KubeVersion -eq [version]'1.33.3' }
    }

    It 'returns no package when the cache directory is missing or unconfigured' {
        $global:CacheDir = Join-Path $TestDrive 'absent'
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
        $global:CacheDir = ''
        Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' | Should -Be $null
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 2 -Exactly
    }

    It 'surfaces unexpected cache access errors instead of treating them as cache misses' {
        Mock Get-ChildItem -MockWith { Write-Error 'Access denied to archive directory' }
        { Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' } | Should -Throw '*Failed to inspect Dalec credential provider cache*Access denied*'
        Assert-MockCalled Resolve-DalecCredentialProviderPackageFromMetadata -Times 0
    }

    It 'surfaces errors probing the cache directory' {
        Mock Test-Path -MockWith { Write-Error 'Access denied to cache root' }
        { Resolve-DalecCredentialProviderPackage -KubeBinariesVersion '1.33.3' } | Should -Throw '*Failed to inspect Dalec credential provider cache*Access denied*'
    }
}
