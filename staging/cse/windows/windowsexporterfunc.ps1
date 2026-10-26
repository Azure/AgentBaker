<#
.SYNOPSIS
    Registers and starts the aks-windows-exporter service using assets baked into the VHD.

.DESCRIPTION
    Migrated from aks-vm-extension (see aks-windows-node-vm-extension/entrypoint.ps1).
    Runs windows-exporter.exe as the Windows service "aks-windows-exporter" via NSSM,
    matching the service name, log paths, and NSSM settings the extension
    used so existing customer dashboards/alerts continue to work.

    Lifecycle:
    - VHD build stages the binary, config, health script, the assets marker, and
      C:\k\skip_vhd_windows_exporter (which disables the aks-vm-extension path).
    - BasePrep (Register-WindowsExporter) registers the service disabled once
      nssm.exe is available, so PIS-cached images carry a registered service that
      cannot start before the node is provisioned.
    - NodePrep (Enable-WindowsExporter) sets the service to auto start, starts it,
      and checks its health. It registers the service first if BasePrep did not.

    Both functions no-op when the VHD does not carry exporter assets; in that case
    aks-vm-extension continues to manage the exporter. Failures are returned to the
    caller, which logs them and continues provisioning without removing the baked
    skip marker.
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
            $service = Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue
            if ($service -and $service.Status -eq 'Running' -and $healthResult -ne "") {
                Write-Log "aks-windows-exporter health check passed: $healthResult"
                $versionResult = Get-Version
                if ($versionResult -ne "") {
                    Write-Log "aks-windows-exporter version $versionResult"
                }
                return $true
            }
            if ($i -lt $RetryCount) {
                Start-Sleep -Seconds $RetryInterval
            }
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
        $service = Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status -eq 'Running' -and $null -ne $result -and $result.Contains("ok")) {
            Write-Log "aks-windows-exporter health check passed: $result"
            return $true
        }
        if ($i -lt $RetryCount) {
            Start-Sleep -Seconds $RetryInterval
        }
    }
    Write-Log "aks-windows-exporter health check failed after $($RetryCount + 1) attempts"
    return $false
}

function Test-WindowsExporterAssets {
    <#
    .SYNOPSIS
        Returns 'NotBaked', 'Ready', or 'Invalid' for the VHD-baked exporter assets.
    #>
    if (-not (Test-Path $global:WindowsExporterAssetsFile)) {
        if (Test-Path $global:WindowsExporterSkipFile) {
            Write-Log "windows-exporter assets marker is missing but the baked skip marker disables the extension"
            return 'Invalid'
        }
        Write-Log "windows-exporter assets marker not present; aks-vm-extension will manage windows-exporter on this node"
        return 'NotBaked'
    }

    # Without the skip marker aks-vm-extension may still manage the service, so
    # CSE must not take ownership.
    if (-not (Test-Path $global:WindowsExporterSkipFile)) {
        Write-Log "windows-exporter assets marker is present but the extension skip marker is missing at $($global:WindowsExporterSkipFile)"
        return 'Invalid'
    }

    if (-not (Test-Path $global:WindowsExporterBinary)) {
        Write-Log "windows-exporter assets marker is present but binary is missing at $($global:WindowsExporterBinary)"
        return 'Invalid'
    }

    if (-not (Test-Path $global:WindowsExporterConfig)) {
        Write-Log "windows-exporter assets marker is present but config is missing at $($global:WindowsExporterConfig)"
        return 'Invalid'
    }

    if (-not (Test-Path $global:WindowsExporterHealthScript)) {
        Write-Log "windows-exporter health script not found at $($global:WindowsExporterHealthScript); health validation will use direct endpoint probe"
    }

    if (-not (Test-Path $global:WindowsExporterNssm)) {
        Write-Log "nssm.exe not found at $($global:WindowsExporterNssm); cannot manage $($global:WindowsExporterServiceName)"
        return 'Invalid'
    }

    return 'Ready'
}

function Register-WindowsExporter {
    <#
    .SYNOPSIS
        Registers and configures the aks-windows-exporter NSSM service as disabled.

    .NOTES
        Called from BasePrep after Get-KubePackage provides nssm.exe. The service
        stays disabled until Enable-WindowsExporter runs in NodePrep. Returns true
        when the VHD has no exporter assets (extension-managed node).
    #>
    $assetState = Test-WindowsExporterAssets
    if ($assetState -eq 'NotBaked') {
        return $true
    }
    if ($assetState -ne 'Ready') {
        return $false
    }

    Write-Log "Registering $($global:WindowsExporterServiceName) as a disabled service"
    $appParameters = "--config.file=`"$($global:WindowsExporterConfig)`""

    # NSSM settings mirror aks-vm-extension/aks-windows-node-vm-extension/entrypoint.ps1 Install-SystemService
    # to preserve service behavior (logs, rotation, restart policy) that customers rely on.
    try {
        if (-not (Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue)) {
            Invoke-WindowsExporterNssm -Arguments @("install", $global:WindowsExporterServiceName, $global:WindowsExporterBinary)
        } else {
            Write-Log "$($global:WindowsExporterServiceName) is already registered; updating its settings"
        }
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Application", $global:WindowsExporterBinary)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppDirectory", $global:WindowsExporterInstallDir)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "AppParameters", $appParameters)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "DisplayName", $global:WindowsExporterServiceName)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Description", $global:WindowsExporterServiceName)
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Start", "SERVICE_DISABLED")
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
    }
    catch {
        Write-Log "failed to register aks-windows-exporter: $_"
        return $false
    }

    Write-Log "Registered $($global:WindowsExporterServiceName) as a disabled service"
    return $true
}

function Enable-WindowsExporter {
    <#
    .SYNOPSIS
        Enables, starts, and health-checks the aks-windows-exporter service.

    .NOTES
        Called from NodePrep on every provisioned node. Registers the service first
        when BasePrep did not (for example, a PIS image baked by an older CSE).
        Returns true when the VHD has no exporter assets (extension-managed node).
    #>
    $assetState = Test-WindowsExporterAssets
    if ($assetState -eq 'NotBaked') {
        return $true
    }
    if ($assetState -ne 'Ready') {
        return $false
    }

    if (-not (Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue)) {
        Write-Log "$($global:WindowsExporterServiceName) is not registered; registering before enabling it"
        if (-not (Register-WindowsExporter)) {
            return $false
        }
    }

    Write-Log "Enabling and starting $($global:WindowsExporterServiceName)"
    try {
        Invoke-WindowsExporterNssm -Arguments @("set", $global:WindowsExporterServiceName, "Start", "SERVICE_AUTO_START")
        # Restart a service that is already running (for example, a CSE re-run) so
        # settings applied by Register-WindowsExporter take effect.
        $service = Get-Service $global:WindowsExporterServiceName -ErrorAction Stop
        if ($service.Status -ne 'Stopped') {
            Invoke-WindowsExporterNssm -Arguments @("stop", $global:WindowsExporterServiceName)
        }
    }
    catch {
        Write-Log "failed to enable aks-windows-exporter: $_"
        return $false
    }

    try {
        Invoke-WindowsExporterNssm -Arguments @("start", $global:WindowsExporterServiceName)
    }
    catch {
        # NSSM can return nonzero while the service is still START_PENDING.
        # The bounded health checks below also require the named service to be Running.
        Write-Log "aks-windows-exporter start did not complete immediately: $_; waiting for service health"
    }

    if (-not (Test-WindowsExporterHealth)) {
        Write-Log "aks-windows-exporter failed to become healthy"
        return $false
    }

    Write-Log "Enabled $($global:WindowsExporterServiceName) and verified it is healthy"
    return $true
}
