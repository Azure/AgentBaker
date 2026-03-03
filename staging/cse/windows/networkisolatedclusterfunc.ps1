# functions for network isolated cluster

# unpackage and install oras from cache
# Oras is used for pulling windows binaries, e.g. windowszip, from private container registry when it is network isolated cluster.
function Ensure-Oras {
    # If oras already installed, directly return
    if (Test-Path $global:OrasPath) {
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
