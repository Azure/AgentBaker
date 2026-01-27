BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1

    # Define mock functions before loading the scripts
    function Write-Log {
        param($Message)
        Write-Host "LOG: $Message"
        $script:LastLogMessage = $Message
    }

    function Set-ExitCode {
        param($ExitCode, $ErrorMessage)
        Write-Host "MOCK: Exit Code would be: $ExitCode, Error: $ErrorMessage"
        # Store the exit code for verification
        $script:LastExitCode = $ExitCode
        $script:LastErrorMessage = $ErrorMessage
    }

    # Mock Test-Json function (PowerShell built-in)
    function Test-Json {
        param($Json)
        try {
            $null = ConvertFrom-Json $Json -ErrorAction Stop
            return $true
        }
        catch {
            return $false
        }
    }

    # Mock Join-Path to avoid path validation issues
    Mock Join-Path -MockWith {
        param($Path, $ChildPath)
        return "$Path\$ChildPath"
    }

    Mock Logs-To-Event

    # Load the function under test
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe 'Enable-WindowsCiliumNetworking' {
    BeforeEach {
        # Reset global variables and mocks before each test
        $global:EnableWindowsCiliumNetworking = $false
        $global:WindowsCiliumNetworkingConfiguration = ""
        $global:RebootNeeded = $false
        $global:WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED = 72
        
        # Reset tracked values
        $script:LastExitCode = $null
        $script:LastErrorMessage = $null
        $script:InstallScriptCalled = $false
        $script:InstallScriptArgs = $null
        
        # Mock the new install script function
        Mock Invoke-WindowsCiliumNetworkingInstallScript -MockWith {
            param($Arguments)
            Write-Host "MOCK: Install script called with arguments"
            $script:InstallScriptCalled = $true
            $script:InstallScriptArgs = $Arguments
            
            # Simulate successful execution - the real function would modify the ref variable
            if ($Arguments.ContainsKey('RestartRequiredOut')) {
                $Arguments['RestartRequiredOut'].Value = $false  # Default to no reboot needed
            }
        }
    }

    Context 'When Windows Cilium Networking is disabled' {
        It 'Should skip installation and return early' {
            $global:EnableWindowsCiliumNetworking = $false
            
            Enable-WindowsCiliumNetworking
            
            $script:InstallScriptCalled | Should -Be $false
            $script:LastExitCode | Should -BeNullOrEmpty
            $global:RebootNeeded | Should -Be $false
        }
    }

    Context 'When Windows Cilium Networking is enabled' {
        BeforeEach {
            $global:EnableWindowsCiliumNetworking = $true
        }

        Context 'With no configuration specified' {
            It 'Should call install script with basic arguments only' {
                $global:WindowsCiliumNetworkingConfiguration = ""
                
                Enable-WindowsCiliumNetworking
                
                $script:InstallScriptCalled | Should -Be $true
                $script:InstallScriptArgs | Should -Not -BeNullOrEmpty
                $script:InstallScriptArgs.ContainsKey('RestartRequiredOut') | Should -Be $true
                $script:InstallScriptArgs.ContainsKey('SourceDirectory') | Should -Be $true
                $script:InstallScriptArgs.ContainsKey('SkipNugetUnpack') | Should -Be $true
                $script:InstallScriptArgs.ContainsKey('ScenarioConfig') | Should -Be $false
            }
        }

        Context 'With valid JSON configuration' {
            It 'Should call install script with scenario configuration' {
                $validJson = '{"key": "value", "nested": {"prop": "test"}}'
                $global:WindowsCiliumNetworkingConfiguration = $validJson
                
                # Set up the mock to include ScenarioConfig when valid JSON is provided
                Mock Invoke-WindowsCiliumNetworkingInstallScript -MockWith {
                    param($Arguments)
                    $script:InstallScriptCalled = $true
                    $script:InstallScriptArgs = $Arguments
                    
                    if ($Arguments.ContainsKey('RestartRequiredOut')) {
                        $Arguments['RestartRequiredOut'].Value = $false
                    }
                }
                
                Enable-WindowsCiliumNetworking
                
                $script:InstallScriptCalled | Should -Be $true
                $script:InstallScriptArgs | Should -Not -BeNullOrEmpty
                $script:InstallScriptArgs.ContainsKey('ScenarioConfig') | Should -Be $true
            }
        }

        Context 'With invalid JSON configuration' {
            It 'Should call install script without scenario configuration and succeed' {
                $invalidJson = '{"key": "value", "invalid": }'
                $global:WindowsCiliumNetworkingConfiguration = $invalidJson
                
                Enable-WindowsCiliumNetworking
                
                $script:InstallScriptCalled | Should -Be $true
                $script:InstallScriptArgs | Should -Not -BeNullOrEmpty
                $script:InstallScriptArgs.ContainsKey('ScenarioConfig') | Should -Be $false
                $script:LastExitCode | Should -BeNullOrEmpty
            }
        }

        Context 'When installation requires reboot' {
            It 'Should set global reboot flag when script indicates reboot needed' {
                $global:WindowsCiliumNetworkingConfiguration = ""
                
                # Mock the install script to simulate reboot needed
                Mock Invoke-WindowsCiliumNetworkingInstallScript -MockWith {
                    param($Arguments)
                    $script:InstallScriptCalled = $true
                    $script:InstallScriptArgs = $Arguments
                    
                    # Simulate the install script setting reboot needed to true
                    if ($Arguments.ContainsKey('RestartRequiredOut')) {
                        $Arguments['RestartRequiredOut'].Value = $true
                    }
                }
                
                Enable-WindowsCiliumNetworking
                
                $script:InstallScriptCalled | Should -Be $true
                $global:RebootNeeded | Should -Be $true
            }
        }

        Context 'When installation fails' {
            It 'Should set exit code and error message on installation failure' {
                $global:WindowsCiliumNetworkingConfiguration = ""
                
                # Mock the install script to throw an exception
                Mock Invoke-WindowsCiliumNetworkingInstallScript -MockWith {
                    throw "Simulated installation failure"
                }
                
                Enable-WindowsCiliumNetworking
                
                $script:LastExitCode | Should -Be 72 # == WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED
                $script:LastErrorMessage | Should -Match "Simulated installation failure"
                $global:RebootNeeded | Should -Be $false
            }
        }
    }
}
