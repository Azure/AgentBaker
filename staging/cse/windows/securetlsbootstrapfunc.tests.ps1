BeforeAll {
    # Define mock functions before loading the scripts
    function Write-Log {
        param($Message)
        Write-Host "$Message"
    }

    function Set-ExitCode {
        param($ExitCode, $ErrorMessage)
        Write-Host "MOCK: Exit Code would be: $ExitCode, Error: $ErrorMessage"
        # Don't actually exit in tests
    }

    function Logs-To-Event {
        param($TaskName, $TaskMessage)
        Write-Host "MOCK: Event Log - Task: $TaskName, Message: $TaskMessage"
    }

    function DownloadFileOverHttp {
        param($Url, $DestinationPath, $ExitCode)
        Write-Host "MOCK: DownloadFileOverHttp - URL: $Url, Dest: $DestinationPath, ExitCode: $ExitCode"
    }

    # Mock file system operations to avoid actual file I/O
    Mock New-Item -MockWith {
        param($ItemType, $Force, $Path)
        Write-Host "MOCK: New-Item - Type: $ItemType, Path: $Path"
        return @{ FullName = $Path }
    }

    Mock Remove-Item -MockWith {
        param($Path, $Force, $Recurse)
        Write-Host "MOCK: Remove-Item - Path: $Path, Force: $Force, Recurse: $Recurse"
    }

    Mock Test-Path -MockWith {
        param($Path)
        # Default to true unless overridden in specific tests
        return $true
    }

    Mock Copy-Item -MockWith {
        param($Path, $Destination, $Force)
        Write-Host "MOCK: Copy-Item - Source: $Path, Dest: $Destination"
    }

    Mock Expand-Archive -MockWith {
        param($path, $DestinationPath)
        Write-Host "MOCK: Expand-Archive - Source: $path, Dest: $DestinationPath"
    }

    # Load the function under test
    . $PSScriptRoot\securetlsbootstrapfunc.ps1
}

Describe "Install-SecureTLSBootstrapClient" {
    BeforeEach {
        # Reset global variables for each test
        $global:EnableSecureTLSBootstrapping = $true
        $global:CacheDir = "C:\akse-cache"
        
        # Reset all mocks to default behavior
        Mock Test-Path -MockWith { return $true }
    }

    Context "When secure TLS bootstrapping is disabled" {
        BeforeAll {
            $testKubeDir = "C:\k"
        }

        It "Should cleanup existing installations and return early" {
            $global:EnableSecureTLSBootstrapping = $false

            { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

            # Verify cleanup operations were called
            Assert-MockCalled Remove-Item -ParameterFilter { 
                $Path -eq [Io.path]::Combine($testKubeDir, "aks-secure-tls-bootstrap-client.exe") 
            } -Exactly 1

            Assert-MockCalled Remove-Item -ParameterFilter { 
                $Path -eq [Io.path]::Combine($testKubeDir, "aks-secure-tls-bootstrap-client-downloads") -and $Recurse -eq $true 
            } -Exactly 1

            # Should not attempt any downloads or installations
            Assert-MockCalled New-Item -Exactly 0
        }
    }

    Context "when using custom download URL" {
        BeforeAll {
            $testKubeDir = "C:\k"
            $customUrl = "https://xxx.blob.core.windows.net/aks-secure-tls-bootstrap-client/custom.zip"
        }

        It "Should successfully download and install a custom client version if a custom URL is specified" {
            { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir -CustomSecureTLSBootstrapClientDownloadUrl $customUrl } | Should -Not -Throw

            # Verify cache was cleared
            Assert-MockCalled Remove-Item -ParameterFilter { 
                $Path -eq [Io.path]::Combine($global:CacheDir, "aks-secure-tls-bootstrap-client") -and $Recurse -eq $true 
            } -Exactly 1

            # Verify download directory was created
            Assert-MockCalled New-Item -ParameterFilter { 
                $ItemType -eq "Directory" -and $Path -eq [Io.path]::Combine($testKubeDir, "aks-secure-tls-bootstrap-client-downloads")
            } -Exactly 1

            # Verify custom download was called
            Assert-MockCalled DownloadFileOverHttp -ParameterFilter { 
                $Url -eq $customUrl -and $ExitCode -eq $global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT
            } -Exactly 1

            # Verify event logging
            Assert-MockCalled Logs-To-Event -ParameterFilter { 
                $TaskName -eq "AKS.WindowsCSE.DownloadSecureTLSBootstrapClient" -and $TaskMessage -like "*$customUrl*"
            } -Exactly 1

            # Verify archive extraction
            Assert-MockCalled Expand-Archive -ParameterFilter { 
                $DestinationPath -eq $testKubeDir
            } -Exactly 1

            # Verify download directory cleanup
            Assert-MockCalled Remove-Item -ParameterFilter { 
                $Path -eq [Io.path]::Combine($testKubeDir, "aks-secure-tls-bootstrap-client-downloads") -and $Recurse -eq $true
            } -Exactly 1
        }
    }

    Context "When using cached version" {
        BeforeAll {
            $testKubeDir = "C:\k"
            $cacheDir = [Io.path]::Combine($global:CacheDir, "aks-secure-tls-bootstrap-client")
        }

        It "Should handle missing cache directory gracefully" {
            Mock Test-Path -ParameterFilter { $Path -eq $global:CacheDir } -MockWith { return $false }
            Mock Set-ExitCode -MockWith { Write-Host "MOCK: Set-ExitCode called with missing cache" }

            { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

            # Verify error handling was called
            Assert-MockCalled Set-ExitCode -ParameterFilter { 
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -and $ErrorMessage -eq "CacheDir is missing"
            } -Exactly 1
        }

        It "Should handle missing cached files gracefully" {
            # Mock empty search results
            $originalGetFiles = [IO.Directory]::GetFiles
            [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value {
                return @()  # Empty array simulates no cached files found
            }

            Mock Set-ExitCode -MockWith { Write-Host "MOCK: Set-ExitCode called with missing cache files" }

            try {
                { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

                # Verify error handling was called
                Assert-MockCalled Set-ExitCode -ParameterFilter { 
                    $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -and $ErrorMessage -eq "Secure TLS bootstrap client is missing from cache"
                } -Exactly 1
            }
            finally {
                # Restore original method
                [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value $originalGetFiles
            }
        }

        It "Should verify binary exists after extraction" {
            Mock Test-Path -ParameterFilter { $Path -like "*aks-secure-tls-bootstrap-client.exe" } -MockWith { return $false }
            Mock Set-ExitCode -MockWith { Write-Host "MOCK: Set-ExitCode called for missing binary" }

            # Mock [IO.Directory]::GetFiles to simulate finding cached file
            Mock -CommandName "[IO.Directory]::GetFiles" -MockWith {
                return @("$cacheDir\windows-amd64.zip")
            } -ModuleName ""

            # Override the static method call with a script block
            $originalGetFiles = [IO.Directory]::GetFiles
            [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value {
                param($path, $searchPattern, $searchOption)
                if ($path -like "*aks-secure-tls-bootstrap-client*") {
                    return @("$cacheDir\windows-amd64.zip")
                }
                return $originalGetFiles.Invoke($path, $searchPattern, $searchOption)
            }

            try {
                { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

                # Verify cached file was copied
                Assert-MockCalled Copy-Item -ParameterFilter { 
                    $Path -eq "$cacheDir\windows-amd64.zip" -and $Force -eq $true
                } -Exactly 1

                # Should not call download function
                Assert-MockCalled DownloadFileOverHttp -Exactly 0

                # Verify error handling was called
                Assert-MockCalled Set-ExitCode -ParameterFilter { 
                    $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -and $ErrorMessage -eq "Secure TLS bootstrap client is missing from KubeDir after zip extraction"
                } -Exactly 1
            }
            finally {
                # Restore original method
                [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value $originalGetFiles
            }
        }

        It "Should succeed when extraction and binary verification pass" {
            Mock Test-Path -ParameterFilter { $Path -like "*aks-secure-tls-bootstrap-client.exe" } -MockWith { return $true }

            # Mock [IO.Directory]::GetFiles to simulate finding cached file
            Mock -CommandName "[IO.Directory]::GetFiles" -MockWith {
                return @("$cacheDir\windows-amd64.zip")
            } -ModuleName ""

            # Override the static method call with a script block
            $originalGetFiles = [IO.Directory]::GetFiles
            [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value {
                param($path, $searchPattern, $searchOption)
                if ($path -like "*aks-secure-tls-bootstrap-client*") {
                    return @("$cacheDir\windows-amd64.zip")
                }
                return $originalGetFiles.Invoke($path, $searchPattern, $searchOption)
            }

            try {
                { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

                # Verify cached file was copied
                Assert-MockCalled Copy-Item -ParameterFilter { 
                    $Path -eq "$cacheDir\windows-amd64.zip" -and $Force -eq $true
                } -Exactly 1

                # Should not call download function
                Assert-MockCalled DownloadFileOverHttp -Exactly 0

                # Should not call Set-ExitCode for errors
                Assert-MockCalled Set-ExitCode -Exactly 0

                # Verify successful extraction
                Assert-MockCalled Expand-Archive -Exactly 1
                Assert-MockCalled Test-Path -ParameterFilter { $Path -like "*aks-secure-tls-bootstrap-client.exe" } -Exactly 1
            }
            finally {
                # Restore original method
                [IO.Directory] | Add-Member -Force -MemberType ScriptMethod -Name GetFiles -Value $originalGetFiles
            }
        }
    }
}