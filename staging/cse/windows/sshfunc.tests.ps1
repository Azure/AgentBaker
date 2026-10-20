BeforeAll {
    . "$PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1"
    . "$PSScriptRoot\sshfunc.ps1"

    function Write-Log {}
    function Get-Service { [CmdletBinding()] param() }
    function Set-Service { [CmdletBinding()] param($Name, $StartupType) }
    function Stop-Service { [CmdletBinding()] param($Name) }
    function Start-Service { [CmdletBinding()] param($Name) }
    function Restart-Service { [CmdletBinding()] param($Name) }
    function Get-WindowsCapability { [CmdletBinding()] param([switch]$Online) }
    function Add-WindowsCapability { [CmdletBinding()] param([switch]$Online, $Name) }
    function Get-NetFirewallRule { [CmdletBinding()] param($Name) }
    function icacls {}
}

Describe 'Install-OpenSSH' {
    BeforeEach {
        $script:sshActions = @()
        $script:icaclsCallCount = 0
        Mock Logs-To-Event
        Mock Get-Service { [PSCustomObject]@{ Name = 'sshd'; Status = 'Running'; StartType = 'Automatic' } }
        Mock Set-Service { $script:sshActions += "Set:$StartupType" }
        Mock Stop-Service { $script:sshActions += 'Stop' }
        Mock Start-Service { $script:sshActions += 'Start' }
        Mock Restart-Service { $script:sshActions += 'Restart' }
        Mock Get-WindowsCapability { [PSCustomObject]@{ Name = 'OpenSSH.Server~~~~0.0.1.0' } }
        Mock Add-WindowsCapability { $script:sshActions += 'Install' }
        Mock Test-Path { $true }
        Mock Clear-Content { $script:sshActions += 'ClearKeys' }
        Mock Add-Content { $script:sshActions += 'AddKey' }
        Mock Get-NetFirewallRule { [PSCustomObject]@{ Name = 'OpenSSH-Server-In-TCP' } }
        Mock icacls {
            $script:icaclsCallCount++
            $global:LASTEXITCODE = 0
        }
        Mock Set-ExitCode { throw "Set-ExitCode:${ExitCode}:${ErrorMessage}" }
    }

    It 'disables and stops the service, including on repeated calls' {
        Install-OpenSSH -Enabled $false
        Mock Get-Service { [PSCustomObject]@{ Name = 'sshd'; Status = 'Stopped'; StartType = 'Disabled' } }
        Install-OpenSSH -Enabled $false

        $script:sshActions | Should -Be @('Set:Disabled', 'Stop', 'Set:Disabled', 'Stop')
        Assert-MockCalled Set-Service -Times 2 -Exactly -ParameterFilter { $Name -eq 'sshd' -and $StartupType -eq 'Disabled' }
        Assert-MockCalled Stop-Service -Times 2 -Exactly -ParameterFilter { $Name -eq 'sshd' }
        Assert-MockCalled Add-WindowsCapability -Times 0 -Exactly
        Assert-MockCalled Clear-Content -Times 0 -Exactly
    }

    It 'does not install SSH or change unrelated services when disabled and absent' {
        Mock Get-Service { [PSCustomObject]@{ Name = 'kubelet' } }
        Install-OpenSSH -Enabled $false

        $script:sshActions | Should -HaveCount 0
        Assert-MockCalled Get-WindowsCapability -Times 0 -Exactly
    }

    It 're-enables a cached disabled service without reinstalling the capability' {
        Mock Get-Service { [PSCustomObject]@{ Name = 'sshd'; Status = 'Stopped'; StartType = 'Disabled' } }
        Install-OpenSSH -Enabled $true -SSHKeys @('ssh-rsa live-key')

        $script:sshActions | Should -Be @('Set:Automatic', 'Start', 'ClearKeys', 'AddKey', 'Restart')
        Assert-MockCalled Add-WindowsCapability -Times 0 -Exactly
        Assert-MockCalled Add-Content -Times 1 -Exactly -ParameterFilter {
            $LiteralPath -eq 'C:\ProgramData\ssh\administrators_authorized_keys' -and $Value -eq 'ssh-rsa live-key'
        }
    }

    It 'installs and starts the capability before writing keys when absent' {
        Mock Get-Service {}
        Mock Test-Path { $false }
        Install-OpenSSH -Enabled $true -SSHKeys @('ssh-rsa live-key')

        $script:sshActions | Should -Be @('Install', 'Set:Automatic', 'Start', 'AddKey', 'Restart')
        Assert-MockCalled Add-WindowsCapability -Times 1 -Exactly -ParameterFilter {
            $Online -and $Name -eq 'OpenSSH.Server~~~~0.0.1.0' -and $ErrorAction -eq 'Stop'
        }
    }

    It 'replaces inherited keys on repeated calls without deleting the file' {
        $script:keysFile = Join-Path $TestDrive 'administrators_authorized_keys'
        [System.IO.File]::WriteAllText($script:keysFile, "ssh-rsa baked-key`n")
        Mock Clear-Content {
            [System.IO.File]::WriteAllText($script:keysFile, '')
        } -ParameterFilter { $LiteralPath -eq 'C:\ProgramData\ssh\administrators_authorized_keys' }
        Mock Add-Content {
            param($Value)
            [System.IO.File]::AppendAllText($script:keysFile, "$Value`n")
        } -ParameterFilter { $LiteralPath -eq 'C:\ProgramData\ssh\administrators_authorized_keys' }
        Mock Remove-Item { throw 'must preserve the existing file and ACL' }

        1..2 | ForEach-Object {
            Install-OpenSSH -Enabled $true -SSHKeys @('ssh-rsa live-key-1', 'ssh-rsa live-key-2')
            @(Get-Content -LiteralPath $script:keysFile) | Should -Be @('ssh-rsa live-key-1', 'ssh-rsa live-key-2')
        }
        Assert-MockCalled Clear-Content -Times 2 -Exactly -ParameterFilter {
            $LiteralPath -eq 'C:\ProgramData\ssh\administrators_authorized_keys' -and $Force -and $ErrorAction -eq 'Stop'
        }
    }

    It 'supports the old enabled-only caller through configfunc' {
        . "$PSScriptRoot\configfunc.ps1"
        Install-OpenSSH -SSHKeys @('ssh-rsa live-key')
        $script:icaclsCallCount | Should -Be 4
        Assert-MockCalled Start-Service -Times 1 -Exactly
    }

    It 'overrides an old cached function using the inline load from the template' {
        function Install-OpenSSH { param($SSHKeys) throw 'old cached implementation' }
        $inlinePath = Join-Path $TestDrive 'windowsssh.ps1'
        Copy-Item -LiteralPath "$PSScriptRoot\sshfunc.ps1" -Destination $inlinePath
        $template = Get-Content "$PSScriptRoot\..\..\..\parts\windows\kuberneteswindowssetup.ps1.template" -Raw
        $load = [regex]::Match($template, '(?m)^\s*\. c:\\AzureData\\windows\\windowsssh\.ps1').Value
        $load | Should -Not -BeNullOrEmpty
        . ([scriptblock]::Create($load.Replace('c:\AzureData\windows\windowsssh.ps1', "'$inlinePath'")))

        Install-OpenSSH -Enabled $false
        $script:sshActions | Should -Be @('Set:Disabled', 'Stop')
    }

    It 'requires keys for the enabled-only caller' -TestCases @(
        @{ Keys = $null }
        @{ Keys = @() }
    ) {
        param($Keys)
        { Install-OpenSSH -SSHKeys $Keys } | Should -Throw '*SSH public keys are required*'
        $script:sshActions | Should -HaveCount 0
    }

    It 'fails when the capability is unavailable' {
        Mock Get-Service {}
        Mock Get-WindowsCapability {}
        { Install-OpenSSH -SSHKeys @('ssh-rsa live-key') } | Should -Throw '*OpenSSH is not available*'
        Assert-MockCalled Add-WindowsCapability -Times 0 -Exactly
    }

    It 'fails when the SSH firewall rule is absent' {
        Mock Get-NetFirewallRule {}
        { Install-OpenSSH -SSHKeys @('ssh-rsa live-key') } | Should -Throw "*firewall is not configured*"
    }

    It 'fails before restarting the service when ACL operation <FailedCall> fails' -TestCases @(
        @{ FailedCall = 1 }
        @{ FailedCall = 2 }
        @{ FailedCall = 3 }
        @{ FailedCall = 4 }
    ) {
        param($FailedCall)
        Mock icacls {
            $script:icaclsCallCount++
            $global:LASTEXITCODE = if ($script:icaclsCallCount -eq $FailedCall) { 5 } else { 0 }
        }
        { Install-OpenSSH -SSHKeys @('ssh-rsa live-key') } | Should -Throw '*icacls failed*exit code 5*'
        $script:icaclsCallCount | Should -Be $FailedCall
        Assert-MockCalled Restart-Service -Times 0 -Exactly
    }

    It 'propagates non-terminating errors from <CommandName>' -TestCases @(
        @{ CommandName = 'Get-Service'; Enabled = $false; Absent = $false }
        @{ CommandName = 'Set-Service'; Enabled = $false; Absent = $false }
        @{ CommandName = 'Stop-Service'; Enabled = $false; Absent = $false }
        @{ CommandName = 'Set-Service'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Get-WindowsCapability'; Enabled = $true; Absent = $true }
        @{ CommandName = 'Add-WindowsCapability'; Enabled = $true; Absent = $true }
        @{ CommandName = 'Test-Path'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Clear-Content'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Add-Content'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Start-Service'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Restart-Service'; Enabled = $true; Absent = $false }
        @{ CommandName = 'Get-NetFirewallRule'; Enabled = $true; Absent = $false }
    ) {
        param($CommandName, $Enabled, $Absent)
        if ($Absent) { Mock Get-Service {} }
        Mock $CommandName { Write-Error 'SSH command failed' }
        { Install-OpenSSH -Enabled $Enabled -SSHKeys @('ssh-rsa live-key') } | Should -Throw '*SSH command failed*'
    }
}
