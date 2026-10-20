function Install-OpenSSH {
    [CmdletBinding()]
    Param(
        [string[]]
        $SSHKeys,
        [bool]
        $Enabled = $true
    )

    $sshdService = Get-Service -ErrorAction Stop | Where-Object { $_.Name -eq 'sshd' }
    if (-not $Enabled) {
        if ($sshdService) {
            Write-Log "Stopping and disabling OpenSSH Server"
            Set-Service -Name sshd -StartupType Disabled -ErrorAction Stop
            Stop-Service -Name sshd -ErrorAction Stop
        }
        else {
            Write-Log "OpenSSH Server is not installed; SSH access is disabled"
        }
        return
    }

    if (-not $SSHKeys -or @($SSHKeys | Where-Object { [string]::IsNullOrWhiteSpace($_) }).Count -gt 0) {
        throw "SSH public keys are required when SSH access is enabled"
    }

    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallOpenSSH" -TaskMessage "Start to install OpenSSH"
    if (-not $sshdService) {
        Write-Log "Installing OpenSSH"
        $isAvailable = Get-WindowsCapability -Online -ErrorAction Stop | Where-Object { $_.Name -like 'OpenSSH*' }
        if (-not $isAvailable) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED -ErrorMessage "OpenSSH is not available on this machine"
        }
        Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 -ErrorAction Stop
    }
    else {
        Write-Log "OpenSSH Server service detected - skipping online install..."
    }

    # A cached image may have the service disabled and keys from an earlier policy.
    Set-Service -Name sshd -StartupType Automatic -ErrorAction Stop
    $adminpath = 'C:\ProgramData\ssh'
    $adminfile = 'administrators_authorized_keys'
    $authorizedKeysPath = "$adminpath\$adminfile"
    if (Test-Path -LiteralPath $authorizedKeysPath -ErrorAction Stop) {
        Clear-Content -LiteralPath $authorizedKeysPath -Force -ErrorAction Stop
    }
    if (-not (Test-Path -LiteralPath $adminpath -ErrorAction Stop)) {
        New-Item -Path $adminpath -ItemType Directory -Force -ErrorAction Stop | Out-Null
    }
    $SSHKeys | ForEach-Object {
        Add-Content -LiteralPath $authorizedKeysPath -Value $_ -ErrorAction Stop
    }

    Write-Log "Setting required permissions..."
    icacls $authorizedKeysPath /remove "NT AUTHORITY\Authenticated Users"
    if ($LASTEXITCODE -ne 0) { throw "icacls failed to remove Authenticated Users permissions from $authorizedKeysPath (exit code $LASTEXITCODE)" }
    icacls $authorizedKeysPath /inheritance:r
    if ($LASTEXITCODE -ne 0) { throw "icacls failed to set inheritance on $authorizedKeysPath (exit code $LASTEXITCODE)" }
    icacls $authorizedKeysPath /grant SYSTEM:`(F`)
    if ($LASTEXITCODE -ne 0) { throw "icacls failed to grant SYSTEM permissions on $authorizedKeysPath (exit code $LASTEXITCODE)" }
    icacls $authorizedKeysPath /grant BUILTIN\Administrators:`(F`)
    if ($LASTEXITCODE -ne 0) { throw "icacls failed to grant Administrators permissions on $authorizedKeysPath (exit code $LASTEXITCODE)" }

    Start-Service -Name sshd -ErrorAction Stop
    Restart-Service -Name sshd -ErrorAction Stop

    $firewall = Get-NetFirewallRule -Name *ssh* -ErrorAction Stop
    if (-not $firewall) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED -ErrorMessage "OpenSSH's firewall is not configured properly"
    }
    Write-Log "OpenSSH installed and configured successfully"
}
