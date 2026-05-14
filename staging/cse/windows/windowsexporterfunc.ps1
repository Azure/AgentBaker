<#
.SYNOPSIS
    Installs and starts the aks-windows-exporter service using assets baked into the VHD.

.DESCRIPTION
    Migrated from aks-vm-extension (see aks-windows-node-vm-extension/entrypoint.ps1).
    Registers windows-exporter.exe as the Windows service "aks-windows-exporter" via NSSM,
    matching the service name, port (19182), log paths, and NSSM settings the extension
    used so existing customer dashboards/alerts continue to work.

    The function is guarded so it is a no-op when running on a VHD that does not carry
    the exporter assets. In that case the aks-vm-extension install path continues to
    handle the service (dual-mode coexistence).

    Coordination with aks-vm-extension:
    - When C:\k\skip_vhd_windows_exporter exists, the extension's entrypoint.ps1
            short-circuits. The sentinel is created by the Windows VHD build once the binary
      and config are staged.
#>

$global:WindowsExporterInstallDir     = "C:\k\windows-exporter"
$global:WindowsExporterBinary         = Join-Path $global:WindowsExporterInstallDir "windows-exporter.exe"
$global:WindowsExporterConfig         = Join-Path $global:WindowsExporterInstallDir "windows-exporter-config.yml"
$global:WindowsExporterHealthScript   = Join-Path $global:WindowsExporterInstallDir "windows-exporter-health.ps1"
$global:WindowsExporterSkipFile       = "C:\k\skip_vhd_windows_exporter"
$global:WindowsExporterServiceName    = "aks-windows-exporter"
$global:WindowsExporterPort           = 19182
$global:WindowsExporterStdoutLog      = "C:\k\windows-exporter.log"
$global:WindowsExporterStderrLog      = "C:\k\windows-exporter.err.log"
$global:WindowsExporterNssm           = "C:\k\nssm.exe"

function Test-WindowsExporterHealth {
    param(
        [int]$RetryCount    = 5,
        [int]$RetryInterval = 5
    )

    if (Test-Path $global:WindowsExporterHealthScript) {
        . $global:WindowsExporterHealthScript
        for ($i = 0; $i -le $RetryCount; $i++) {
            $healthResult = Get-Health
            if ($healthResult -ne "") {
                Write-Log "aks-windows-exporter health check passed: $healthResult"
                $versionResult = Get-Version
                if ($versionResult -ne "") {
                    Write-Log "aks-windows-exporter version $versionResult"
                }
                return $true
            }
            Start-Sleep -Seconds $RetryInterval
        }

        Write-Log "aks-windows-exporter health script check failed after $($RetryCount + 1) attempts"
        return $false
    }

    Write-Log "windows-exporter health script not found at $($global:WindowsExporterHealthScript); falling back to direct health endpoint probe"
    for ($i = 0; $i -le $RetryCount; $i++) {
        $result = ""
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:$($global:WindowsExporterPort)/health" -TimeoutSec 10 -ErrorAction Stop
            $result = [string]$response.Content
        }
        catch {
            $result = ""
        }
        if ($null -ne $result -and $result.Contains("ok")) {
            Write-Log "aks-windows-exporter health check passed: $result"
            return $true
        }
        Start-Sleep -Seconds $RetryInterval
    }
    Write-Log "aks-windows-exporter health check failed after $($RetryCount + 1) attempts"
    return $false
}

function Install-WindowsExporter {
    <#
    .SYNOPSIS
        Registers and starts the aks-windows-exporter NSSM service.

    .NOTES
        No-ops when:
        - The VHD-build sentinel C:\k\skip_vhd_windows_exporter is absent
          (older VHD without baked assets - aks-vm-extension still covers it).
        - The windows-exporter binary is missing on disk (defensive).
    #>

    if (-not (Test-Path $global:WindowsExporterSkipFile)) {
        Write-Log "skip_vhd_windows_exporter not present; aks-vm-extension will manage windows-exporter on this node"
        return
    }

    if (-not (Test-Path $global:WindowsExporterBinary)) {
        Write-Log "windows-exporter binary not found at $($global:WindowsExporterBinary); skipping install (older VHD?)"
        return
    }

    if (-not (Test-Path $global:WindowsExporterConfig)) {
        Write-Log "windows-exporter config not found at $($global:WindowsExporterConfig); skipping install"
        return
    }

    if (-not (Test-Path $global:WindowsExporterHealthScript)) {
        Write-Log "windows-exporter health script not found at $($global:WindowsExporterHealthScript); health validation will use direct endpoint probe"
    }

    if (-not (Test-Path $global:WindowsExporterNssm)) {
        Write-Log "nssm.exe not found at $($global:WindowsExporterNssm); cannot install $($global:WindowsExporterServiceName)"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_WINDOWS_EXPORTER_START_FAIL -ErrorMessage "nssm.exe missing; cannot register aks-windows-exporter"
        return
    }

    Write-Log "Ensuring $($global:WindowsExporterServiceName) is installed and running"

    $appParameters = "--config.file=`"$($global:WindowsExporterConfig)`""

    # NSSM settings mirror aks-vm-extension/aks-windows-node-vm-extension/entrypoint.ps1 Install-SystemService
    # to preserve service behavior (logs, rotation, restart policy) that customers rely on.
    $existingService = Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue
    if (-not $existingService) {
        & $global:WindowsExporterNssm install $global:WindowsExporterServiceName $global:WindowsExporterBinary | Out-Null
    } else {
        Write-Log "$($global:WindowsExporterServiceName) is already registered; ensuring settings and running state"
    }
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppDirectory                 $global:WindowsExporterInstallDir | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppParameters                $appParameters                    | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName DisplayName                  $global:WindowsExporterServiceName | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName Description                  $global:WindowsExporterServiceName | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName Start                        SERVICE_AUTO_START                | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName ObjectName                   LocalSystem                       | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName Type                         SERVICE_WIN32_OWN_PROCESS         | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppRestartDelay              5000                              | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppThrottle                  1500                              | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppStdout                    $global:WindowsExporterStdoutLog  | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppStderr                    $global:WindowsExporterStderrLog  | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppStdoutCreationDisposition 4                                 | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppStderrCreationDisposition 4                                 | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppRotateFiles               1                                 | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppRotateOnline              1                                 | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppRotateSeconds             86400                             | Out-Null
    & $global:WindowsExporterNssm set $global:WindowsExporterServiceName AppRotateBytes               10485760                          | Out-Null

    & $global:WindowsExporterNssm start $global:WindowsExporterServiceName | Out-Null

    if (-not (Test-WindowsExporterHealth)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_WINDOWS_EXPORTER_START_FAIL -ErrorMessage "aks-windows-exporter failed to become healthy"
        return
    }

    Write-Log "Ensured $($global:WindowsExporterServiceName) is installed and running"
}
