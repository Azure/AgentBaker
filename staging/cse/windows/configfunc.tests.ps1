BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
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
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 2
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
            $yaml = Get-Content $YamlFile | ConvertFrom-Yaml
            $yaml = $yaml | ConvertTo-Yaml
            return $yaml
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
            $diffence = Compare-Object $acutalCredentialProviderConfig $expectedCredentialProviderConfig
            $diffence | Should -Be $null
        }
    }
   Context 'CustomCloudContainerRegistryDNSSuffix is not empty' {
       It "should match the expected config file content" {
            $expectedCredentialProviderConfig = Read-Format-Yaml ([Io.path]::Combine($credentialProviderConfigDir, "CustomCloudContainerRegistryDNSSuffixNotEmpty.config.yaml"))
            Config-CredentialProvider -KubeDir $credentialProviderConfigDir -CredentialProviderConfPath $CredentialProviderConfPATH -CustomCloudContainerRegistryDNSSuffix ".azurecr.microsoft.fakecloud"
            $acutalCredentialProviderConfig = Read-Format-Yaml $CredentialProviderConfPATH
            $diffence = Compare-Object $acutalCredentialProviderConfig $expectedCredentialProviderConfig
            $diffence | Should -Be $null
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