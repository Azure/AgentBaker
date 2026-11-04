BeforeAll {
    # Load only function definitions: the provisioning entry point also cleans host temp files.
    $ast = [System.Management.Automation.Language.Parser]::ParseFile(
        (Join-Path $PSScriptRoot 'configure-windows-vhd.ps1'), [ref]$null, [ref]$null)
    foreach ($statement in $ast.EndBlock.Statements) {
        if ($statement -is [System.Management.Automation.Language.FunctionDefinitionAst]) {
            Invoke-Expression $statement.Extent.Text
        }
    }
}

Describe 'Download-File' {
    BeforeAll {
        function global:curl.exe {}
    }

    AfterAll {
        $ExecutionContext.SessionState.InvokeProvider.Item.Remove("Function:\curl.exe", $false)
    }

    BeforeEach {
        $script:originalCacheDir = $global:cacheDir
        $global:cacheDir = $null
        $script:downloadAttempts = 0
        $script:curlExitCodes = @(0)

        Mock Write-Log {}
        Mock Log-VHDFreeSize {}
        Mock Get-ChildItem {}
        Mock Start-Sleep {}
        Mock curl.exe {
            if ($args -contains '--version') {
                $global:LASTEXITCODE = 0
                return
            }
            $global:LASTEXITCODE = $script:curlExitCodes[$script:downloadAttempts]
            $script:downloadAttempts++
        }
    }

    AfterEach {
        $global:cacheDir = $script:originalCacheDir
    }

    It 'retries transport error <ExitCode> and returns after a successful download' -ForEach @(
        @{ ExitCode = 5 }
        @{ ExitCode = 6 }
        @{ ExitCode = 7 }
        @{ ExitCode = 16 }
        @{ ExitCode = 18 }
        @{ ExitCode = 35 }
        @{ ExitCode = 52 }
        @{ ExitCode = 55 }
        @{ ExitCode = 56 }
        @{ ExitCode = 92 }
        @{ ExitCode = 95 }
        @{ ExitCode = 96 }
    ) {
        $script:curlExitCodes = @($ExitCode, 0)

        { Download-File -URL 'https://example.com/file.zip' -Dest 'out.zip' } | Should -Not -Throw

        $script:downloadAttempts | Should -Be 2
        Should -Invoke Start-Sleep -Times 1 -Exactly -ParameterFilter { $Seconds -eq 1 }
        Should -Invoke Get-ChildItem -Times 1 -Exactly
    }

    It 'stops after the transport retry budget and preserves the download exit code' {
        $script:curlExitCodes = @(18, 18, 18)

        { Download-File -URL 'https://example.com/file.zip' -Dest 'out.zip' -retryCount 2 } | Should -Throw "*Curl exited with '18'*"

        $script:downloadAttempts | Should -Be 3
        Should -Invoke Start-Sleep -Times 1 -Exactly -ParameterFilter { $Seconds -eq 1 }
        Should -Invoke Start-Sleep -Times 1 -Exactly -ParameterFilter { $Seconds -eq 2 }
        Should -Invoke Get-ChildItem -Times 0 -Exactly
    }

    It 'uses the configured delay between transport retries' {
        $script:curlExitCodes = @(18, 56, 0)

        Download-File -URL 'https://example.com/file.zip' -Dest 'out.zip' -retryDelay 3

        $script:downloadAttempts | Should -Be 3
        Should -Invoke Start-Sleep -Times 2 -Exactly -ParameterFilter { $Seconds -eq 3 }
    }

    It 'does not retry transport errors when retries are disabled' {
        $script:curlExitCodes = @(18)

        { Download-File -URL 'https://example.com/file.zip' -Dest 'out.zip' -retryCount 0 } | Should -Throw "*Curl exited with '18'*"

        $script:downloadAttempts | Should -Be 1
        Should -Invoke Start-Sleep -Times 0 -Exactly
    }

    It 'does not add retries for permanent errors or exhausted native retries (exit <ExitCode>)' -ForEach @(
        @{ ExitCode = 3 }
        @{ ExitCode = 22 }
        @{ ExitCode = 23 }
        @{ ExitCode = 28 }
        @{ ExitCode = 60 }
    ) {
        $script:curlExitCodes = @($ExitCode)

        { Download-File -URL 'https://example.com/file.zip' -Dest 'out.zip' } | Should -Throw "*Curl exited with '$ExitCode'*"

        $script:downloadAttempts | Should -Be 1
        Should -Invoke Start-Sleep -Times 0 -Exactly
    }

    It 'retains the out-of-space diagnostic and redacts the final error URL' {
        $script:curlExitCodes = @(23)

        { Download-File -URL 'https://example.com/file.zip?token=secret' -Dest 'out.zip' -redactUrl } |
            Should -Throw "Curl exited with '23' while attempting to download 'https://example.com/file.zip' to 'out.zip'. This often means VHD out of space."
    }
}

Describe 'Download-FileWithAzCopy' {
    BeforeEach {
        $script:tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-configure-windows-vhd-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:tempDir -Force | Out-Null

        $global:aksTempDir = (Join-Path $script:tempDir "aksTemp")
        New-Item -ItemType Directory -Path $global:aksTempDir -Force | Out-Null
        New-Item -ItemType File -Path "$global:aksTempDir\azcopy.exe" -Force | Out-Null

        Mock Write-Log {}
        Mock dir {}
        Mock Get-Content {} -ParameterFilter { $Path -like "*azcopy*.log*" }
    }

    AfterEach {
        Remove-Item -Path $script:tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    Context 'without -RequireMSILogin (legacy callers: Get-PrivatePackagesToCacheOnVHD, Get-ContainerImages base image override)' {
        It 'falls through to copy when login fails, and succeeds if copy succeeds (e.g. a SAS-bearing URL)' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip?sv=sas" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Not -Throw

            Should -Invoke Invoke-AzCopyLogin -Times 1
            Should -Invoke Invoke-AzCopyCopy -Times 1
        }

        It 'still throws if copy itself fails, even though login failure alone was tolerated' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 1 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip?sv=sas" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Throw "*azcopy copy*failed*"
        }

        It 'succeeds normally when both login and copy succeed' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Not -Throw
        }
    }

    Context 'with -RequireMSILogin (the new windowsDownloadRequiresAzCopy component path, via Invoke-PackageDownload)' {
        It 'throws immediately when login fails, before copy is ever attempted' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Throw "*MSI-only*"

            Should -Invoke Invoke-AzCopyCopy -Times 0
        }

        It 'succeeds when login succeeds and copy succeeds' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Not -Throw
        }

        It 'throws if copy fails even when login succeeded' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 1 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Throw "*azcopy copy*failed*"
        }
    }
}

Describe 'Get-OrasDownloadUrl' {
    It 'uses the specified DALEC revision <Revision>' -TestCases @(
        @{ Revision = '1' }
        @{ Revision = '12' }
    ) {
        param($Revision)
        Get-OrasDownloadUrl -Version "1.3.6-${Revision}" | Should -Be "https://packages.aks.azure.com/dalec-packages/oras/1.3.6/windows/amd64/oras_1.3.6-${Revision}_amd64.zip"
    }

    It 'rejects a version without a package revision' {
        { Get-OrasDownloadUrl -Version '1.3.6' } | Should -Throw '*including the DALEC revision*'
    }

    It 'matches the Windows component URL and package version' {
        . "$PSScriptRoot\components_json_helpers.ps1"
        $components = Get-Content "$PSScriptRoot\..\..\..\parts\common\components.json" -Raw | ConvertFrom-Json
        $oras = $components.Packages | Where-Object { $_.name -eq 'oras' }
        $part = $oras.downloadURIs.windows.default
        $version = $part.versionsV2[0].latestVersion
        $expectedUrl = ReplaceVarsInUrl -versionString $version -stringToReplace $part.downloadURL
        Get-OrasDownloadUrl -Version $version | Should -Be $expectedUrl
    }
}

Describe 'Invoke-PackageDownload' {
    BeforeEach {
        $script:tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-configure-windows-vhd-$(New-Guid)"
        Mock Write-Log {}
        Mock Download-File {}
        Mock Download-FileWithAzCopy {}
    }

    It 'dispatches to Download-FileWithAzCopy with -RequireMSILogin when the URL is in the AzCopy set' {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/f.zip" = $true }

        Invoke-PackageDownload -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-FileWithAzCopy -Times 1 -ParameterFilter { $RequireMSILogin -eq $true }
        Should -Invoke Download-File -Times 0
    }

    It 'dispatches to Download-File when the URL is not in the AzCopy set' {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/f.zip" = $true }

        Invoke-PackageDownload -URL "https://acs-mirror.azureedge.net/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-File -Times 1
        Should -Invoke Download-FileWithAzCopy -Times 0
    }

    It 'dispatches to Download-File when the AzCopy set is empty/unset' {
        $global:azCopyUrls = @{ }

        Invoke-PackageDownload -URL "https://acs-mirror.azureedge.net/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-File -Times 1
        Should -Invoke Download-FileWithAzCopy -Times 0
    }
}

Describe 'Get-ContainerImages with base image overrides' {
    BeforeEach {
        $script:originalCoreUrl = $env:WindowsServerCoreImageURL
        $script:originalNanoUrl = $env:WindowsNanoServerImageURL
        $env:WindowsServerCoreImageURL = "https://example.com/core2022.tar,https://example.com/core2025.tar"
        $env:WindowsNanoServerImageURL = ""
        $script:originalImagesToPull = $global:imagesToPull
        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:10.0.26100.33438",
            "mcr.microsoft.com/windows/servercore:ltsc2022",
            "mcr.microsoft.com/windows/servercore:ltsc2025"
        )
        $script:pulled = [System.Collections.Generic.List[string]]::new()
        $script:downloaded = [System.Collections.Generic.List[string]]::new()

        Mock Write-Log {}
        Mock Write-Host {}
        Mock Get-Command { [pscustomobject]@{ Path = (Join-Path ([System.IO.Path]::GetTempPath()) "crictl.exe") } } -ParameterFilter { $Name -eq "crictl.exe" }
        Mock Download-FileWithAzCopy { $script:downloaded.Add($URL) }
        Mock Remove-Item {}
        Mock Stop-Job {}
        Mock Remove-Job {}
        Mock Retry-Command {
            $text = $ScriptBlock.ToString()
            if ($text -match "pull") { $script:pulled.Add($image) }
        }
        function global:crictl { }
    }

    AfterEach {
        $env:WindowsServerCoreImageURL = $script:originalCoreUrl
        $env:WindowsNanoServerImageURL = $script:originalNanoUrl
        $global:imagesToPull = $script:originalImagesToPull
        # Pester intercepts every Remove-Item call here, so remove the stub through the provider.
        $ExecutionContext.SessionState.InvokeProvider.Item.Remove("Function:\crictl", $false)
    }

    It 'imports each tarball once instead of pulling base images' {
        Get-ContainerImages

        $script:downloaded | Should -Be @("https://example.com/core2022.tar", "https://example.com/core2025.tar")
        $script:pulled.Count | Should -Be 0
    }
}

Describe 'Get-ContainerImages test isolation' {
    It 'does not leak the crictl stub' {
        Test-Path Function:\crictl | Should -BeFalse
    }
}
