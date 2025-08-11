BeforeAll {
    . $PSScriptRoot\securetlsbootstrapfunc.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe "Install-SecureTLSBootstrapClient" {
    BeforeEach {
        Mock DownloadFileOverHttp -MockWith {
            Param(
                $Url,
                $DestinationPath,
                $ExitCode
            )
            Write-Host "DownloadFileOverHttp -Url $Url -DestinationPath $DestinationPath -ExitCode $ExitCode"
        } -Verifiable
        Mock Set-ExitCode -MockWith{
            Param(
                $ExitCode,
                $ErrorMessage
            )
            Write-Host "Set-ExitCode -ExitCode $ExitCode -ErrorMessage $ErrorMessage"
        } -Verifiable
        Mock Test-Path -MockWith { return $true }
        Mock New-Item
        Mock Copy-Item
        Mock Remove-Item
        Mock Expand-Archive
        Mock Logs-To-Event

        $global:EnableSecureTLSBootstrapping = $true
        $global:CacheDir = "C:\akse-cache"
    }

    Context "When secure TLS bootstrapping is disabled" {
        BeforeAll {
            $testKubeDir = "C:\k"
            $global:EnableSecureTLSBootstrapping = $false
        }

        It "Should cleanup existing installations and return early" {
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
                $Url -eq $customUrl -and $DestinationPath -eq "C:\k\aks-secure-tls-bootstrap-client-downloads\aks-secure-tls-bootstrap-client.zip" -and $ExitCode -eq $global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT
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

            Mock -CommandName "[IO.Directory]::GetFiles" -MockWith {
                return @("$cacheDir\windows-amd64.zip")
            } -ModuleName ""
        }

        It "Should handle missing cache directory gracefully" {
            Mock Test-Path -ParameterFilter { $Path -eq $global:CacheDir } -MockWith { return $false }

            { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

            Assert-MockCalled Set-ExitCode -ParameterFilter { 
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -and $ErrorMessage -eq "CacheDir is missing"
            } -Exactly 1
        }

        It "Should handle missing cached files gracefully" {
            # Mock empty search results
            Mock -CommandName "[IO.Directory]::GetFiles" -MockWith {
                return @()
            } -ModuleName ""

            { Install-SecureTLSBootstrapClient -KubeDir $testKubeDir } | Should -Not -Throw

            Assert-MockCalled Set-ExitCode -ParameterFilter { 
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -and $ErrorMessage -eq "Secure TLS bootstrap client is missing from cache"
            } -Exactly 1
        }

        It "Should verify binary exists after extraction" {
            Mock Test-Path -ParameterFilter { $Path -like "*aks-secure-tls-bootstrap-client.exe" } -MockWith { return $false }

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

        It "Should succeed when extraction and binary verification pass" {
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
    }
}