# functions for network isolated cluster

# Initialize-Oras will install oras and login the registry if anonymous access is disabled. This is required for network isolated cluster to pull windowszip from private container registry.
function Initialize-Oras {
    Install-Oras
    # reserve for Invoke-OrasLogin to avoid frequent code changes in parts/windows/
}

# unpackage and install oras from cache
# Oras is used for pulling windows binaries, e.g. windowszip, from private container registry when it is network isolated cluster.
function Install-Oras {
    # Check if OrasPath variable exists to avoid latest cached cse in vhd with possible old ab svc
    $orasPathVarExists = Test-Path variable:global:OrasPath
    if (-not $orasPathVarExists) {
        Write-Log "OrasPath variable does not exist. Setting OrasPath to default value C:\aks-tools\oras\oras.exe"
        $global:OrasPath = "C:\aks-tools\oras\oras.exe"
    }

    if (Test-Path -Path $global:OrasPath) {
        # oras already installed, skip
        Write-Log "Oras already installed at $($global:OrasPath)"
        return
    }
    # Ensure cache directory exists before checking for archives or downloading
    if (-Not (Test-Path $global:OrasCacheDir)) {
        New-Item -ItemType Directory -Path $global:OrasCacheDir -Force | Out-Null
    }

    if (-Not (Test-Path $global:OrasCacheDir)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras cache directory not found at $($global:OrasCacheDir)"
    }

    # Look for a cached oras archive (.tar.gz or .zip) in the oras cache directory
    $orasArchive = Get-ChildItem -Path $global:OrasCacheDir -File |
        Where-Object { $_.Name -like "*.tar.gz" -or $_.Name -like "*.zip" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-Not $orasArchive) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "No oras archive (.tar.gz or .zip) found in $($global:OrasCacheDir)"
    }

    # Extract the archive to the oras install directory
    $orasInstallDir = [IO.Path]::GetDirectoryName($global:OrasPath)
    if (-Not (Test-Path $orasInstallDir)) {
        New-Item -ItemType Directory -Path $orasInstallDir -Force | Out-Null
    }

    Write-Log "Extracting oras from $($orasArchive.FullName) to $orasInstallDir"
    if ($orasArchive.Name -like "*.zip") {
        AKS-Expand-Archive -Path $orasArchive.FullName -DestinationPath $orasInstallDir
    } elseif ($orasArchive.Name -like "*.tar.gz") {
        try {
            tar -xzf $orasArchive.FullName -C $orasInstallDir
            if ($LASTEXITCODE -ne 0) {
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Failed to extract oras archive $($orasArchive.FullName) (tar exit code $LASTEXITCODE)"
            }
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Exception while extracting oras archive $($orasArchive.FullName): $($_.Exception.Message)"
        }
    }

    if (-Not (Test-Path $global:OrasPath)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras executable not found at $($global:OrasPath) after extraction"
    }

    Write-Log "Oras installed successfully at $($global:OrasPath)"
}

function DownloadFileWithOras {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Reference,
        [Parameter(Mandatory = $true)][string]
        $DestinationPath,
        [Parameter(Mandatory = $true)][int]
        $ExitCode,
        [Parameter(Mandatory = $false)][string]
        $Platform = "windows/amd64"
    )

    Write-Log "Downloading $Reference to $DestinationPath via oras pull (platform=$Platform)"

    # oras pull --output specifies the output directory, not the filename.
    # Download to a temp directory first, then move the file to DestinationPath.
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    $downloadTimer = [System.Diagnostics.Stopwatch]::StartNew()
    $orasArgs = @(
        "pull",
        $Reference,
        "--platform=$Platform",
        "--registry-config=$($global:OrasRegistryConfigFile)",
        "--output", $tempDir
    )
    & $global:OrasPath @orasArgs
    if ($LASTEXITCODE -ne 0) {
        $downloadTimer.Stop()
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Set-ExitCode -ExitCode $ExitCode -ErrorMessage "oras pull failed with exit code $LASTEXITCODE for $Reference"
    }
    $downloadTimer.Stop()
    $elapsedMs = $downloadTimer.ElapsedMilliseconds

    # Find the downloaded file in the temp directory and move it to the desired path
    $downloadedFile = Get-ChildItem -Path $tempDir -File | Select-Object -First 1
    if (-not $downloadedFile) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        Set-ExitCode -ExitCode $ExitCode -ErrorMessage "oras pull succeeded but no file found in temp directory for $Reference"
    }

    Write-Log "Downloaded file name: $($downloadedFile.Name) (size: $($downloadedFile.Length) bytes) in temp directory $tempDir"

    # Ensure the destination parent directory exists
    $destDir = [System.IO.Path]::GetDirectoryName($DestinationPath)
    if ($destDir -and -not (Test-Path $destDir)) {
        New-Item -ItemType Directory -Path $destDir -Force | Out-Null
    }

    # Remove existing destination if present, then move the downloaded file
    if (Test-Path $DestinationPath) {
        Remove-Item -Path $DestinationPath -Force
    }
    Write-Log "Moving $($downloadedFile.FullName) to $DestinationPath"
    Move-Item -Path $downloadedFile.FullName -Destination $DestinationPath -Force
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue

    if ($global:AppInsightsClient -ne $null) {
        $event = New-Object "Microsoft.ApplicationInsights.DataContracts.EventTelemetry"
        $event.Name = "FileDownload"
        $event.Properties["FileName"] = $Reference
        $event.Properties["Method"] = "oras"
        $event.Metrics["DurationMs"] = $elapsedMs
        $global:AppInsightsClient.TrackEvent($event)
    }

    Write-Log "Downloaded $Reference to $DestinationPath via oras in $elapsedMs ms"
    Get-Item $DestinationPath -ErrorAction Continue | Format-List | Out-String | Write-Log
}
