<#
.SYNOPSIS
    Registers and starts the aks-windows-exporter service using assets baked into the VHD.

.DESCRIPTION
    Migrated from aks-vm-extension (see aks-windows-node-vm-extension/entrypoint.ps1).
    Runs windows-exporter.exe as the Windows service "aks-windows-exporter" via NSSM,
    matching the service name, log paths, and NSSM settings the extension
    used so existing customer dashboards/alerts continue to work.

    Lifecycle:
    - VHD build stages the binary, config, the assets marker, and
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
$global:WindowsExporterAssetsFile     = Join-Path $global:WindowsExporterInstallDir "windows-exporter-assets.complete"
$global:WindowsExporterSkipFile       = "C:\k\skip_vhd_windows_exporter"
$global:WindowsExporterServiceName    = "aks-windows-exporter"
$global:WindowsExporterPort           = 19100
$global:WindowsExporterStdoutLog      = "C:\k\windows-exporter.log"
$global:WindowsExporterStderrLog      = "C:\k\windows-exporter.err.log"

function Invoke-WindowsExporterRequest {
    param([Parameter(Mandatory=$true)][string]$Path)

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:$($global:WindowsExporterPort)/$Path" -TimeoutSec 10 -ErrorAction Stop
        return [string]$response.Content
    }
    catch {
        return ""
    }
}

function Test-WindowsExporterHealth {
    param(
        [int]$RetryCount    = 5,
        [int]$RetryInterval = 5
    )

    for ($i = 0; $i -le $RetryCount; $i++) {
        $result = Invoke-WindowsExporterRequest -Path "health"
        $service = Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status -eq 'Running' -and $null -ne $result -and $result.Contains("ok")) {
            Write-Log "aks-windows-exporter health check passed: $result"
            Write-WindowsExporterVersion
            return $true
        }
        if ($i -lt $RetryCount) {
            Start-Sleep -Seconds $RetryInterval
        }
    }
    Write-Log "aks-windows-exporter health check failed after $($RetryCount + 1) attempts"
    return $false
}

function Write-WindowsExporterVersion {
    # Best effort: the version is logged for diagnostics only.
    $result = Invoke-WindowsExporterRequest -Path "version"
    if ([string]::IsNullOrEmpty($result)) {
        return
    }
    try {
        $versionInfo = $result | ConvertFrom-Json -ErrorAction Stop
        if (-not [string]::IsNullOrEmpty($versionInfo.version)) {
            Write-Log "aks-windows-exporter version $($versionInfo.version)"
        }
    }
    catch {
    }
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

    $nssm = Join-Path $global:KubeDir "nssm.exe"
    if (-not (Test-Path $nssm)) {
        Write-Log "nssm.exe not found at $nssm; cannot manage $($global:WindowsExporterServiceName)"
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

    $svc = $global:WindowsExporterServiceName
    Write-Log "Registering $svc as a disabled service"
    $appParameters = "--config.file=`"$($global:WindowsExporterConfig)`""

    # NSSM settings mirror aks-vm-extension/aks-windows-node-vm-extension/entrypoint.ps1 Install-SystemService
    # to preserve service behavior (logs, rotation, restart policy) that customers rely on.
    try {
        if (-not (Get-Service $global:WindowsExporterServiceName -ErrorAction SilentlyContinue)) {
            $null = Invoke-Nssm -KubeDir $global:KubeDir install $svc $global:WindowsExporterBinary
        } else {
            Write-Log "$($global:WindowsExporterServiceName) is already registered; updating its settings"
        }
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc Application $global:WindowsExporterBinary
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppDirectory $global:WindowsExporterInstallDir
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppParameters $appParameters
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc DisplayName $svc
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc Description $svc
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc Start SERVICE_DISABLED
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc ObjectName LocalSystem
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc Type SERVICE_WIN32_OWN_PROCESS
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppRestartDelay 5000
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppThrottle 1500
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppStdout $global:WindowsExporterStdoutLog
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppStderr $global:WindowsExporterStderrLog
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppStdoutCreationDisposition 4
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppStderrCreationDisposition 4
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppRotateFiles 1
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppRotateOnline 1
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppRotateSeconds 86400
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc AppRotateBytes 10485760
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

    $svc = $global:WindowsExporterServiceName
    Write-Log "Enabling and starting $svc"
    try {
        $null = Invoke-Nssm -KubeDir $global:KubeDir set $svc Start SERVICE_AUTO_START
        # Restart a service that is already running (for example, a CSE re-run) so
        # settings applied by Register-WindowsExporter take effect.
        $service = Get-Service $global:WindowsExporterServiceName -ErrorAction Stop
        if ($service.Status -ne 'Stopped') {
            $null = Invoke-Nssm -KubeDir $global:KubeDir stop $svc
        }
    }
    catch {
        Write-Log "failed to enable aks-windows-exporter: $_"
        return $false
    }

    try {
        $null = Invoke-Nssm -KubeDir $global:KubeDir start $svc
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
