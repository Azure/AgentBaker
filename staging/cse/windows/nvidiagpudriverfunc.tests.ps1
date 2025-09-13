BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    Mock Set-Content
    Mock Get-Item -MockWith { return New-Object -TypeName PSObject -Property @{ FullName = $DestinationPath } }

}

Describe 'Start-InstallGPUDriver' {

    BeforeEach {
        $global:RebootNeeded = $false

        Mock Set-ExitCode -MockWith {
            Param(
                [Parameter(Mandatory = $true)][int]
                $ExitCode,
                [Parameter(Mandatory = $true)][string]
                $ErrorMessage
            )
            Write-Host "Set ExitCode to $ExitCode and exit. Error: $ErrorMessage"
            throw $ExitCode
        } -Verifiable

        Mock DownloadFileOverHttp -MockWith {
            param (
                [string]$Url,
                [string]$DestinationPath,
                [int]$ExitCode
            )
            $DestinationPath | Should -Be $TargetPath
        } -Verifiable

        Mock VerifySignature -MockWith {
            param (
                [string]$targetFile
            )
            $targetFile | Should -Be $TargetPath
        } -Verifiable

        Mock Start-Process -MockWith {
            param (
                [string]$FilePath,
                [string]$ArgumentList
            )
            $FilePath | Should -Be $TargetPath
            $process = New-Object System.Diagnostics.Process
            return @($process)
        } -Verifiable

        Mock Wait-Process -MockWith {
            param (
                [System.Diagnostics.Process[]]$InputObject,
                [int]$Timeout
            )
            return
        } -Verifiable

        Mock Get-VmData -MockWith {
            return @{ vmSize = "Standard_NV" }
        } -Verifiable

        Mock Remove-InstallerFile
    }

    Context 'When EnableInstall is false' {
        It "Should skip installation" {
            try {
                Start-InstallGPUDriver -EnableInstall $false
            }
            catch {
                Throw "Unexpected exception during UT: $_"
            }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 0
        }
    }

    Context 'When EnableInstall is true' {
        It "Should skip installation if GpuDriverURL is empty" {
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL ''
            }
            catch {
                Write-Host "Expected exception: $_"
            }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET }
        }

        It "Should skip installation if GpuDriverURL does not point to an exe file" {
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL 'https://example.com/somefile.zip'
            }
            catch {
                Write-Host "Expected exception: $_"
            }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_EXE }
        }

        It "Should exit when the signature is not valid" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            Mock VerifySignature -MockWith {
                param (
                    [string]$targetFile
                )
                $targetFile | Should -Be $TargetPath
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE -ErrorMessage "Signature embedded in $($Target) is not valid."
            } -Verifiable
            
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Write-Host "Expected exception: $_"
            }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE }
        }

        It "Should exit when Start-Process does not return a valid output for Wait-Process to use as a valid argument." {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            Mock Start-Process -MockWith {
                param (
                    [string]$FilePath,
                    [string]$ArgumentList
                )
                return
            } -Verifiable
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Write-Host "Expected exception: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_EXCEPTION }
        }

        It "Should run Wait-Process when Start-Process does not return a hashtable mock output" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Throw "Unexpected exception during UT: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Wait-Process' -Exactly -Times 1
        }

        It "Should exit when installation code is not 0 or 1" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            # The ExitCode of System.Diagnostics.Process is readonly so we have to create a custom hashtable.
            # However, Wait-Process does not accept hastable as a parameter so we'll need to skip it in the code.
            Mock Start-Process -MockWith {
                param (
                    [string]$FilePath,
                    [string]$ArgumentList
                )
                $FilePath | Should -Be $TargetPath
                $process = @{ ExitCode = 9 }
                return $process
            } -Verifiable
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Write-Host "Expected exception: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Wait-Process' -Exactly -Times 0
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED }
        }

        It "Should set RebootNeeded to be true when installation code is 1" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            Mock Start-Process -MockWith {
                param (
                    [string]$FilePath,
                    [string]$ArgumentList
                )
                $FilePath | Should -Be $TargetPath
                $process = @{ ExitCode = 1 }
                return $process
            } -Verifiable
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Write-Host "Expected exception: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Wait-Process' -Exactly -Times 0
            Assert-MockCalled -CommandName 'Remove-InstallerFile' -Exactly -Times 1
            $global:RebootNeeded | Should -Be $true
        }

        It "Should run set RebootNeeded to be true when installation code is 0 and vm size is nv series" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            Mock Start-Process -MockWith {
                param (
                    [string]$FilePath,
                    [string]$ArgumentList
                )
                $FilePath | Should -Be $TargetPath
                $process = @{ ExitCode = 0 }
                return $process
            } -Verifiable

            Mock Get-VmData -MockWith {
                return @{ vmSize = "Standard_NV" }
            } -Verifiable
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Throw "Unexpected exception during UT: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Wait-Process' -Exactly -Times 0
            Assert-MockCalled -CommandName 'Remove-InstallerFile' -Exactly -Times 1
            $global:RebootNeeded | Should -Be $true
        }

        It "Should run set RebootNeeded to be false when installation code is 0 and vm size is nc series" {
            $GpuDriverURL = 'https://example.com/gpudriver.exe'
            $TargetPath = "C:\AzureData\gpudriver.exe"

            Mock Start-Process -MockWith {
                param (
                    [string]$FilePath,
                    [string]$ArgumentList
                )
                $FilePath | Should -Be $TargetPath
                $process = @{ ExitCode = 0 }
                return $process
            } -Verifiable

            Mock Get-VmData -MockWith {
                return @{ vmSize = "Standard_NC" }
            } -Verifiable
            try {
                Start-InstallGPUDriver -EnableInstall $true -GpuDriverURL $GpuDriverURL
            }
            catch {
                Throw "Unexpected exception during UT: $_"
            }

            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq $GpuDriverURL }
            Assert-MockCalled -CommandName 'VerifySignature' -Exactly -Times 1 -ParameterFilter { $targetFile -eq $TargetPath }
            Assert-MockCalled -CommandName 'Start-Process' -Exactly -Times 1 -ParameterFilter { $FilePath -eq $TargetPath }
            Assert-MockCalled -CommandName 'Wait-Process' -Exactly -Times 0
            Assert-MockCalled -CommandName 'Remove-InstallerFile' -Exactly -Times 1
            $global:RebootNeeded | Should -Be $false
        }
    }
}