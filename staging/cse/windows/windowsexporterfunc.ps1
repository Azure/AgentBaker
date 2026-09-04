<#
.SYNOPSIS
    Installs and starts the aks-windows-exporter service using assets baked into the VHD.

.DESCRIPTION
    Migrated from aks-vm-extension (see aks-windows-node-vm-extension/entrypoint.ps1).
    Registers windows-exporter.exe as the Windows service "aks-windows-exporter" via NSSM,
    matching the service name, log paths, and NSSM settings the extension
    used so existing customer dashboards/alerts continue to work.

    The function is guarded so it is a no-op when running on a VHD that does not carry
    the exporter assets. In that case the aks-vm-extension install path continues to
    handle the service (dual-mode coexistence).

    Coordination with aks-vm-extension:
    - The VHD build creates windows-exporter-assets.complete after staging assets.
    - CSE creates C:\k\skip_vhd_windows_exporter only after the service is healthy.
      Older CSE versions leave it absent, so aks-vm-extension remains the owner.
#>

$global:WindowsExporterInstallDir     = "C:\k\windows-exporter"
$global:WindowsExporterBinary         = Join-Path $global:WindowsExporterInstallDir "windows-exporter.exe"
$global:WindowsExporterConfig         = Join-Path $global:WindowsExporterInstallDir "windows-exporter-config.yml"
$global:WindowsExporterHealthScript   = Join-Path $global:WindowsExporterInstallDir "windows-exporter-health.ps1"
$global:WindowsExporterAssetsFile     = Join-Path $global:WindowsExporterInstallDir "windows-exporter-assets.complete"
$global:WindowsExporterSkipFile       = "C:\k\skip_vhd_windows_exporter"
$global:WindowsExporterServiceName    = "aks-windows-exporter"
$global:WindowsExporterPort           = 19100
$global:WindowsExporterStdoutLog      = "C:\k\windows-exporter.log"
$global:WindowsExporterStderrLog      = "C:\k\windows-exporter.err.log"
$global:WindowsExporterNssm           = "C:\k\nssm.exe"

function Invoke-WindowsExporterNssm {
    param([Parameter(Mandatory=$true)][string[]]$Arguments)

    & $global:WindowsExporterNssm @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "nssm.exe $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

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
        - The VHD assets marker is absent (older VHD without baked assets;
          aks-vm-extension still covers it).

        Returns false without creating the extension skip marker when takeover fails,
        allowing node provisioning and the extension fallback to continue.
    #>

    if (-not (Test-Path $global:WindowsExporterAssetsFile)) {
        Write-Log "windows-exporter assets marker not present; aks-vm-extension will manage windows-exporter on this node"
        return $true
    }

    if (-not (Test-Path $global:WindowsExporterBinary)) {
        Write-Log "windows-exporter assets marker is present but binary is missing at $($global:WindowsExporterBinary); leaving ownership with aks-vm-extension"
        return $false
    }

    if (-not (Test-Path $global:WindowsExporterConfig)) {
        Write-Log "windows-exporter assets marker is present but config is missing at $($global:WindowsExporterConfig); leaving ownership with aks-vm-extension"
        return $false
    }

    if (-not (Test-Path $global:WindowsExporterHealthScript)) {
        Write-Log "windows-exporter health script not found at $($global:WindowsExporterHealthScript); health validation will use direct endpoint probe"
    }

    if (-not (Test-Path $global:WindowsExporterNssm)) {
        Write-Log "nssm.exe not found at $($global:WindowsExporterNssm); cannot install $($global:WindowsExporterServiceName)"
        return $false
    }

    Write-Log "Ensuring $($global:WindowsExporterServiceName) is installed and running"

    $appParameters = "--config.file=`"$($global:WindowsExporterConfig)`""

    # NSSM settings mirror aks-vm-extension/aks-windows-node-vm-extension/entrypoint.ps1 Install-SystemService
    # to preserve service behavior (logs, rotation, restart policy) that customers rely on.
    try {
        $existingService = Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue
        if (-not $existingService) {
            Invoke-WindowsExporterNssm -Arguments @("install", $global:WindowsExporterServiceName, $global:WindowsExporterBinary)
        } else {
            Write-Log "$($global:WindowsExporterServiceName) is already registered; taking ownership of its settings and running state"
            if ($existingService.Status -ne 'Stopped') {
                Invoke-WindowsExporterNssm -Arguments @("stop", $global:WindowsExporterServiceName)
            }
        }
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Application", $global:WindowsExporterBinary)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppDirectory", $global:WindowsExporterInstallDir)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppParameters", $appParameters)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "DisplayName", $global:WindowsExporterServiceName)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Description", $global:WindowsExporterServiceName)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Start", "SERVICE_AUTO_START")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "ObjectName", "LocalSystem")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Type", "SERVICE_WIN32_OWN_PROCESS")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppRestartDelay", "5000")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppThrottle", "1500")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppStdout", $global:WindowsExporterStdoutLog)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppStderr", $global:WindowsExporterStderrLog)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppStdoutCreationDisposition", "4")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppStderrCreationDisposition", "4")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppRotateFiles", "1")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppRotateOnline", "1")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppRotateSeconds", "86400")
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppRotateBytes", "10485760")
        Invoke-WindowsExporterNssm -Arguments @("start", $global:WindowsExporterServiceName)
    }
    catch {
        Write-Log "failed to configure aks-windows-exporter: $_; leaving ownership with aks-vm-extension"
        return $false
    }

    if (-not (Test-WindowsExporterHealth)) {
        Write-Log "aks-windows-exporter failed to become healthy; leaving ownership with aks-vm-extension"
        return $false
    }

    # Commit ownership only after the service is healthy. Old CSE versions never
    # create this marker, so the extension remains responsible on new VHDs.
    New-Item -ItemType File -Path $global:WindowsExporterSkipFile -Force | Out-Null
    Write-Log "Ensured $($global:WindowsExporterServiceName) is installed and running"
    return $true
}
